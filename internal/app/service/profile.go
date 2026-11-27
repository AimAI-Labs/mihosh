package service

// profile.go — 订阅管理业务逻辑。
//
// ProfileService 负责订阅元数据变更（config.yaml 的 subs/active_sub）、
// 拉取（profile.Fetch）、覆写编辑（profile.WriteMerge）、以及激活流程
// （生成 → 备份 → 写入 mihomo 配置 → 热重载核心）。
//
// 设计要点：
//   - 本服务不返回 tea.Cmd（保持与 ProxyService/ConfigService 一致的纯服务风格）；
//     tea.Cmd 封装放在 features/sub/commands.go。
//   - Activate 流程用 sync.Mutex 串行化，避免两次切换交叉写文件。
//   - 元数据持久化委托 config 包（subs/active_sub 字段）。

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"gopkg.in/yaml.v3"
)

// ErrSubNotFound 表示给定 UID 的订阅不存在。
var ErrSubNotFound = errors.New("订阅不存在")

// ErrMihomoPathMissing 表示无法解析 mihomo 配置路径（激活所需）。
var ErrMihomoPathMissing = errors.New("未找到 mihomo 配置文件路径")

// ProfileService 订阅管理服务。
type ProfileService struct {
	core    *CoreService
	client  *api.Client
	mu      sync.Mutex // Activate 流程串行化
	applied *confirmedProfileApply
}

// NewProfileService 创建订阅管理服务。
// client 用于激活后的核心热重载；为 nil 时激活流程会在重载阶段报错。
func NewProfileService(client *api.Client, cores ...*CoreService) *ProfileService {
	s := &ProfileService{client: client}
	if len(cores) > 0 {
		s.core = cores[0]
	}
	return s
}

// ListProfiles 返回当前所有订阅元数据与激活 UID。
func (s *ProfileService) ListProfiles() ([]profile.Profile, string, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrConfigNotFound) {
		return []profile.Profile{}, "", nil
	} else if err != nil {
		return nil, "", err
	}
	if cfg.Subs == nil {
		return []profile.Profile{}, cfg.ActiveSub, nil
	}
	return cfg.Subs, cfg.ActiveSub, nil
}

// AddProfile 新增订阅：生成 UID + 创建 profile 目录 + 写入元数据。
// 返回新建的 Profile。
func (s *ProfileService) AddProfile(name string, src profile.SubSource) (profile.Profile, error) {
	name = trimSpace(name)
	if name == "" {
		return profile.Profile{}, errors.New("订阅名称不能为空")
	}
	if trimSpace(src.Display()) == "" {
		return profile.Profile{}, errors.New("订阅来源不能为空")
	}
	// 规范化来源字段：去除首尾空白，并按内容推断 Kind（统一用户输入）。
	src.URL = trimSpace(src.URL)
	src.Path = trimSpace(src.Path)
	src.UserAgent = trimSpace(src.UserAgent)
	src.Kind = profile.ParseSourceKind(src.Display())
	if src.Kind != profile.SourceRemote {
		src.UseProxy = false
		src.UserAgent = ""
		src.InsecureSkipVerify = false
		src.UpdateInterval = 0
	} else if src.UpdateInterval < 0 {
		src.UpdateInterval = 0
	}

	uid, err := newUID()
	if err != nil {
		return profile.Profile{}, fmt.Errorf("生成订阅 ID 失败: %w", err)
	}

	// 预创建目录，避免后续 fetch/merge 时才发现权限问题。
	if _, err := profile.RawPath(uid); err != nil {
		return profile.Profile{}, err
	}

	p := profile.Profile{
		UID:    uid,
		Name:   name,
		Source: src,
	}

	if err := s.persistMutation(func(cfg *config.Config) {
		cfg.Subs = append(cfg.Subs, p)
	}); err != nil {
		_ = profile.DeleteProfileDir(uid) // 回滚：清理空目录
		return profile.Profile{}, err
	}
	return p, nil
}

// DeleteProfile 删除订阅：移除元数据 + 删除 raw/merge 目录。
// 若删除的是当前激活订阅，同时清空 active_sub。
func (s *ProfileService) DeleteProfile(uid string) error {
	exists, err := s.uidExists(uid)
	if err != nil {
		return err
	}
	if !exists {
		return ErrSubNotFound
	}

	if err := s.persistMutation(func(cfg *config.Config) {
		filtered := cfg.Subs[:0]
		for _, p := range cfg.Subs {
			if p.UID != uid {
				filtered = append(filtered, p)
			}
		}
		cfg.Subs = filtered
		if cfg.ActiveSub == uid {
			cfg.ActiveSub = ""
		}
	}); err != nil {
		return err
	}
	return profile.DeleteProfileDir(uid)
}

// RenameProfile 重命名订阅。
func (s *ProfileService) RenameProfile(uid, newName string) error {
	newName = trimSpace(newName)
	if newName == "" {
		return errors.New("订阅名称不能为空")
	}
	return s.mutateProfile(uid, func(p *profile.Profile) error {
		p.Name = newName
		return nil
	})
}

// EditProfile 编辑订阅元数据（名称 + 来源），就地校验并持久化。
// 校验逻辑与 AddProfile 对齐：名称非空、来源非空，并按内容规范化 Kind。
func (s *ProfileService) EditProfile(uid, name string, src profile.SubSource) error {
	name = trimSpace(name)
	if name == "" {
		return errors.New("订阅名称不能为空")
	}
	if trimSpace(src.Display()) == "" {
		return errors.New("订阅来源不能为空")
	}
	// 规范化来源字段：去除首尾空白，并按内容推断 Kind（与 AddProfile 一致）。
	src.URL = trimSpace(src.URL)
	src.Path = trimSpace(src.Path)
	src.UserAgent = trimSpace(src.UserAgent)
	src.Kind = profile.ParseSourceKind(src.Display())
	if src.Kind != profile.SourceRemote {
		src.UseProxy = false
		src.UserAgent = ""
		src.InsecureSkipVerify = false
		src.UpdateInterval = 0
	} else if src.UpdateInterval < 0 {
		src.UpdateInterval = 0
	}
	return s.mutateProfile(uid, func(p *profile.Profile) error {
		p.Name = name
		if !p.Source.SameOrigin(src) {
			if err := profile.BindLegacyRawSource(*p); err != nil {
				return err
			}
			p.UpdatedAt = 0
			p.Usage = nil
		}
		p.Source = src
		return nil
	})
}

// SetActiveSub 设置激活订阅 UID（仅改元数据，不触发生成/重载）。
// 传空字符串表示取消激活。
func (s *ProfileService) SetActiveSub(uid string) error {
	if uid != "" {
		exists, err := s.uidExists(uid)
		if err != nil {
			return err
		}
		if !exists {
			return ErrSubNotFound
		}
	}
	return s.persistMutation(func(cfg *config.Config) {
		cfg.ActiveSub = uid
	})
}

// FetchProfile 拉取订阅原始配置并刷新 UpdatedAt。
func (s *ProfileService) FetchProfile(uid string) error {
	return s.FetchProfileContext(context.Background(), uid)
}

func (s *ProfileService) FetchProfileContext(ctx context.Context, uid string) error {
	p, err := s.findProfile(uid)
	if err != nil {
		return err
	}
	return fetchProfile(ctx, p)
}

func fetchProfile(ctx context.Context, p profile.Profile) error {
	var opts profile.DownloadOptions
	if p.Source.UseProxy {
		opts.ProxyURL = subscriptionProxyURL(config.ResolveMihomoEndpoint())
	}
	raw, usage, err := profile.DownloadWithOptions(ctx, p.Source, opts)
	if err != nil {
		return err
	}
	return config.Update(func(cfg *config.Config) error {
		for i := range cfg.Subs {
			current := &cfg.Subs[i]
			if current.UID != p.UID {
				continue
			}
			if !current.Source.SameOrigin(p.Source) {
				return errors.New(i18n.T("sub.err.source_changed"))
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := profile.WriteRawForSource(p, raw); err != nil {
				return err
			}
			current.UpdatedAt = time.Now().Unix()
			if usage != nil {
				current.Usage = usage
			}
			return nil
		}
		return ErrSubNotFound
	})
}

func subscriptionProxyURL(endpoint config.MihomoEndpoint) string {
	local := config.MixedPortToProxyURL(endpoint.MixedPort)
	controller, err := url.Parse(api.ControllerHTTPURL(endpoint.ExternalController))
	if err != nil {
		return local
	}
	host := controller.Hostname()
	ip := net.ParseIP(host)
	if host == "" || strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") || ip.IsLoopback() || ip.IsUnspecified() {
		return local
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(endpoint.MixedPort))}).String()
}

func snapshotForActivation(ctx context.Context, p profile.Profile) (*yaml.Node, error) {
	raw, err := profile.ReadRawForProfile(p)
	missing := errors.Is(err, profile.ErrRawNotFound)
	if errors.Is(err, profile.ErrStaleRaw) || missing {
		if fetchErr := fetchProfile(ctx, p); fetchErr != nil {
			if missing && p.Source.Kind == profile.SourceLocal {
				return nil, errors.Join(err, fetchErr)
			}
			return nil, fetchErr
		}
		return profile.ReadRawForProfile(p)
	}
	return raw, err
}

// BatchUpdateResult summarizes a concurrent batch update of remote subscriptions.
type BatchUpdateResult struct {
	Total   int
	Updated int
	Failed  int
	Errors  []string
}

// BatchUpdateRemoteProfiles fetches the specified remote profiles reusing FetchProfileContext
// with bounded concurrency.
func (s *ProfileService) BatchUpdateRemoteProfiles(ctx context.Context, uids []string) BatchUpdateResult {
	if ctx == nil {
		ctx = context.Background()
	}
	res := BatchUpdateResult{Total: len(uids)}
	if len(uids) == 0 {
		return res
	}
	limit := min(model.TestConcurrency, 4)
	jobs := make(chan string)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for range min(limit, len(uids)) {
		wg.Go(func() {
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				err := s.FetchProfileContext(ctx, id)
				mu.Lock()
				if err != nil {
					res.Failed++
					res.Errors = append(res.Errors, err.Error())
				} else {
					res.Updated++
				}
				mu.Unlock()
			}
		})
	}
dispatch:
	for _, uid := range uids {
		select {
		case jobs <- uid:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	wg.Wait()
	if skipped := res.Total - res.Updated - res.Failed; skipped > 0 {
		res.Failed += skipped
		res.Errors = append(res.Errors, ctx.Err().Error())
	}
	return res
}

// UpdateAllRemoteProfiles updates every configured remote subscription.
func (s *ProfileService) UpdateAllRemoteProfiles(ctx context.Context) (BatchUpdateResult, error) {
	subs, _, err := s.ListProfiles()
	if err != nil {
		return BatchUpdateResult{}, err
	}
	var uids []string
	for _, p := range subs {
		if p.Source.Kind == profile.SourceRemote {
			uids = append(uids, p.UID)
		}
	}
	return s.BatchUpdateRemoteProfiles(ctx, uids), nil
}

// UpdateDueRemoteProfiles checks and updates remote subscriptions whose configured
// auto-update interval has elapsed as of now.
func (s *ProfileService) UpdateDueRemoteProfiles(ctx context.Context, now time.Time) (BatchUpdateResult, error) {
	subs, _, err := s.ListProfiles()
	if err != nil {
		return BatchUpdateResult{}, err
	}
	var uids []string
	for _, p := range subs {
		if p.IsDueForUpdate(now) {
			uids = append(uids, p.UID)
		}
	}
	return s.BatchUpdateRemoteProfiles(ctx, uids), nil
}

// LoadMerge 读取全局 merge.yaml 内容（不存在返回 nil）。
func (s *ProfileService) LoadMerge() ([]byte, error) {
	return profile.ReadMerge()
}

// SaveMerge 写入全局 merge.yaml（语法校验在 profile.WriteMerge 内）。
func (s *ProfileService) SaveMerge(content []byte) error {
	return profile.WriteMerge(content)
}

// ActivateResult 激活流程的结果。
type ActivateResult struct {
	BackupName    string // 产生的备份文件名（首次生成时为空）
	MergeErr      error  // merge.yaml 解析错误（非致命，已降级忽略覆写）
	Restarted     bool   // external-controller 或 secret 变更后已重启内核
	NeedsRestart  bool   // 外部/远程内核需要手动重启才能应用控制器地址或密钥变更
	Remote        bool   // 仅通过远程控制器内存重载应用（未写入远端磁盘）
	ConfigWritten bool   // 本地目标已包含生成的配置
	CoreApplied   bool   // 内核确认重载，或已用新配置启动
	ActiveSaved   bool   // 激活 UID 已持久化
}

type profileApplyTarget struct {
	path       string
	digest     [sha256.Size]byte
	generation uint64
}

type confirmedProfileApply struct {
	target profileApplyTarget
	result ActivateResult
}

// Activate 激活订阅。若某一步失败，结果报告已完成的步骤供调用方提示及重试。
func (s *ProfileService) Activate(uid string) (ActivateResult, error) {
	return s.activate(uid, nil)
}

// ApplyActiveMerge 在编辑全局 merge.yaml 后重新生成并应用当前激活订阅。
func (s *ProfileService) ApplyActiveMerge(uid string, cleared []string) (ActivateResult, error) {
	return s.activate(uid, cleared)
}

func (s *ProfileService) activate(uid string, cleared []string) (ActivateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activateLocked(uid, cleared)
}

func (s *ProfileService) activateLocked(uid string, cleared []string) (ActivateResult, error) {
	p, err := s.findProfile(uid)
	if err != nil {
		return ActivateResult{}, err
	}
	cfg, cfgErr := config.Load()
	if cfgErr == nil && cfg.Core.Mode == "managed" {
		if s.core != nil {
			return s.core.ApplyProfileCleared(context.Background(), uid, true, cleared)
		}
		runtime := NewCoreService()
		defer runtime.Close()
		return runtime.ApplyProfileCleared(context.Background(), uid, false, cleared)
	}

	if cfgErr != nil {
		return ActivateResult{}, cfgErr
	}
	return s.activateExternal(p, cfg, cleared)
}

func (s *ProfileService) activateExternal(p profile.Profile, cfg *config.Config, cleared []string) (ActivateResult, error) {
	path, err := config.GetMihomoConfigPath()
	remote := errors.Is(err, config.ErrExternalConfig)
	if err != nil && !remote {
		return ActivateResult{}, ErrMihomoPathMissing
	}
	ctx := context.Background()
	if s.core != nil {
		ctx = s.core.lifetime
	}
	raw, err := snapshotForActivation(ctx, p)
	if err != nil {
		return ActivateResult{}, err
	}
	generated, err := profile.GenerateFromSnapshot(raw, path, cleared, false)
	if err != nil {
		return ActivateResult{}, err
	}
	result := ActivateResult{MergeErr: generated.MergeErr, Remote: remote}
	var client *api.Client
	var generation uint64
	if s.client != nil {
		client = s.client.Snapshot()
		generation = client.Generation()
	}
	target := profileApplyTarget{path: path, digest: sha256.Sum256(generated.Data), generation: generation}
	identityChanged := false
	unchanged := remote
	if remote {
		if overrides, err := profile.ReadMihomoOverrides(); err == nil {
			result.NeedsRestart = identityRequestedForRemote(overrides, cfg.Core.ExternalController, cfg.Core.ExternalSecret)
		}
	} else {
		existing, readErr := os.ReadFile(path)
		identityChanged = readErr == nil && localControllerIdentityChanged(existing, generated.Data, client)
		unchanged = readErr == nil && bytes.Equal(existing, generated.Data)
		if !unchanged {
			s.applied = nil
			backup, err := profile.BackupConfig(path)
			if err != nil {
				return result, fmt.Errorf("备份配置失败: %w", err)
			}
			result.BackupName = backup
			if err := profile.WriteFinal(path, generated.Data); err != nil {
				return result, fmt.Errorf("写入最终配置失败: %w", err)
			}
		}
		result.ConfigWritten = true
	}
	if client == nil {
		return result, errors.New(i18n.T("sub.err.activation_no_client"))
	}
	if unchanged && s.applied != nil && s.applied.target == target {
		result.Restarted = s.applied.result.Restarted && !s.applied.result.ActiveSaved
		result.NeedsRestart = s.applied.result.NeedsRestart
	} else {
		s.applied = nil
		var reloadErr error
		switch {
		case remote:
			reloadErr = client.ReloadConfigPayload(generated.Data)
		case identityChanged && config.RestartMihomoService() == nil:
			result.Restarted = true
		default:
			reloadErr = client.ReloadConfig(path)
			result.NeedsRestart = identityChanged && reloadErr == nil
		}
		if reloadErr != nil {
			key := "sub.err.activation_reload"
			if result.ConfigWritten {
				key = "sub.err.activation_reload_written"
			}
			return result, fmt.Errorf("%s: %w", i18n.T(key), reloadErr)
		}
	}
	result.CoreApplied = true
	if s.client.Generation() != generation {
		return result, errors.New(i18n.T("sub.err.activation_target_changed"))
	}
	s.applied = &confirmedProfileApply{target: target, result: result}
	if err := config.Update(func(latest *config.Config) error { return setActiveProfile(latest, p) }); err != nil {
		return result, fmt.Errorf("%s: %w", i18n.T("sub.err.activation_metadata"), err)
	}
	result.ActiveSaved = true
	if remote {
		s.applied = nil
	} else {
		s.applied.result.ActiveSaved = true
	}
	return result, nil
}

// ===== 内部辅助 =====

// AutoImportLocalSub 首次启动时自动将当前运行的 mihomo 配置文件复制为本地订阅。
// 若已有 SourceLocal 类型的订阅，或无法解析 mihomo 配置路径，则静默跳过。
// 返回导入的 Profile（成功时）或 nil（跳过/失败时）。
func (s *ProfileService) AutoImportLocalSub() (*profile.Profile, error) {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			defaultCopy := config.DefaultConfig
			cfg = &defaultCopy
			cfg.Subs = []profile.Profile{}
			cfg.ActiveSub = ""
		} else {
			return nil, nil // 配置异常，静默跳过
		}
	}

	if cfg.Core.Mode == "managed" {
		return nil, nil
	}
	// 已有本地订阅，跳过
	for _, p := range cfg.Subs {
		if p.Source.Kind == profile.SourceLocal {
			return nil, nil
		}
	}

	// 解析 mihomo 运行配置路径（已通过 -d 参数优先解析）
	mihomoPath, err := config.GetMihomoConfigPath()
	if err != nil {
		return nil, nil // 无本地内核配置，静默跳过
	}

	// 读取 mihomo 配置文件内容
	data, err := os.ReadFile(mihomoPath)
	if err != nil || len(data) == 0 {
		return nil, nil // 文件不可读或为空，静默跳过
	}

	// 生成 UID 并创建目录
	uid, err := newUID()
	if err != nil {
		return nil, fmt.Errorf("生成订阅 ID 失败: %w", err)
	}
	rawPath, err := profile.RawPath(uid)
	if err != nil {
		return nil, err
	}
	// 构建 Profile 元数据（Source.Path 指向复制后的 raw.yaml，而非原始 mihomo 配置路径）
	p := profile.Profile{
		UID:       uid,
		Name:      i18nLocalSubName,
		Source:    profile.SubSource{Kind: profile.SourceLocal, Path: rawPath},
		UpdatedAt: time.Now().Unix(),
	}
	if err := profile.WriteRawForSource(p, data); err != nil {
		_ = profile.DeleteProfileDir(uid)
		return nil, fmt.Errorf("写入本地订阅配置失败: %w", err)
	}

	// 持久化元数据
	if err := s.persistMutation(func(cfg *config.Config) {
		cfg.Subs = append(cfg.Subs, p)
	}); err != nil {
		_ = profile.DeleteProfileDir(uid)
		return nil, err
	}

	return &p, nil
}

// i18nLocalSubName 本地订阅默认名称（硬编码，不依赖 i18n 初始化时序）。
const i18nLocalSubName = "Local Config"

// findProfile 按 UID 查找订阅元数据。
func (s *ProfileService) findProfile(uid string) (profile.Profile, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrConfigNotFound) {
		return profile.Profile{}, ErrSubNotFound
	} else if err != nil {
		return profile.Profile{}, err
	}
	return profileByUID(cfg, uid)
}

func profileByUID(cfg *config.Config, uid string) (profile.Profile, error) {
	for _, p := range cfg.Subs {
		if p.UID == uid {
			return p, nil
		}
	}
	return profile.Profile{}, ErrSubNotFound
}

func setActiveProfile(cfg *config.Config, p profile.Profile) error {
	if p.UID != "" {
		current, err := profileByUID(cfg, p.UID)
		if err != nil {
			return err
		}
		if !current.Source.SameOrigin(p.Source) {
			return errors.New(i18n.T("sub.err.source_changed"))
		}
	}
	cfg.ActiveSub = p.UID
	return nil
}

// uidExists 判断 UID 是否存在。
func (s *ProfileService) uidExists(uid string) (bool, error) {
	_, err := s.findProfile(uid)
	if errors.Is(err, ErrSubNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// mutateProfile 修改单个订阅元数据后持久化。
func (s *ProfileService) mutateProfile(uid string, fn func(*profile.Profile) error) error {
	return config.Update(func(cfg *config.Config) error {
		for i := range cfg.Subs {
			if cfg.Subs[i].UID == uid {
				return fn(&cfg.Subs[i])
			}
		}
		return ErrSubNotFound
	})
}

// persistMutation 加载配置、应用变更函数、保存。
// 保存采用读-改-写，调用方在 fn 内完成对 cfg 的就地修改。
func (s *ProfileService) persistMutation(fn func(*config.Config)) error {
	return config.Update(func(cfg *config.Config) error { fn(cfg); return nil })
}

// newUID 生成 8 字符 hex 订阅 ID（碰撞概率极低，足够本场景）。
func newUID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// trimSpace 去除首尾空白。
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
