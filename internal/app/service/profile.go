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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/api"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/profile"
)

// ErrSubNotFound 表示给定 UID 的订阅不存在。
var ErrSubNotFound = errors.New("订阅不存在")

// ErrMihomoPathMissing 表示无法解析 mihomo 配置路径（激活所需）。
var ErrMihomoPathMissing = errors.New("未找到 mihomo 配置文件路径")

// ProfileService 订阅管理服务。
type ProfileService struct {
	client *api.Client // 仅为 ReloadConfig
	mu     sync.Mutex  // Activate 流程串行化
}

// NewProfileService 创建订阅管理服务。
// client 用于激活后的核心热重载；为 nil 时激活流程会在重载阶段报错。
func NewProfileService(client *api.Client) *ProfileService {
	return &ProfileService{client: client}
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
	src.Kind = profile.ParseSourceKind(src.Display())

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
	return s.mutateProfile(uid, func(p *profile.Profile) {
		p.Name = newName
	})
}

// EditProfile 编辑订阅元数据（名称 + 来源），就地校验并持久化。
// 校验逻辑与 AddProfile 对齐：名称非空、来源非空，并按内容规范化 Kind。
func (s *ProfileService) EditProfile(uid, name string, src profile.SubSource) error {
	exists, err := s.uidExists(uid)
	if err != nil {
		return err
	}
	if !exists {
		return ErrSubNotFound
	}
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
	src.Kind = profile.ParseSourceKind(src.Display())
	return s.mutateProfile(uid, func(p *profile.Profile) {
		p.Name = name
		p.Source = src
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
	p, err := s.findProfile(uid)
	if err != nil {
		return err
	}
	if err := profile.Fetch(p); err != nil {
		return err
	}
	// 更新 UpdatedAt
	return s.mutateProfile(uid, func(p *profile.Profile) {
		p.UpdatedAt = time.Now().Unix()
	})
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
	BackupName string // 产生的备份文件名（首次生成时为空）
	MergeErr   error  // merge.yaml 解析错误（非致命，已降级忽略覆写）
}

// Activate 激活订阅：生成 → 备份 → 写入 mihomo 配置 → 热重载。
//
// 返回 ActivateResult（含备份名与可能的 merge 降级错误）。
// 任一致命步骤失败（生成/备份/写入/重载）返回 error，此时 ActivateResult 零值。
//
// 失败回滚策略（与现有 rules 功能一致）：
//   - 写入成功但重载失败时不回滚文件，由调用方提示「配置已写入但重载失败」；
//   - 重载成功后同步把 active_sub 更新为 uid。
func (s *ProfileService) Activate(uid string) (ActivateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.findProfile(uid)
	if err != nil {
		return ActivateResult{}, err
	}
	_ = p // UID 已通过 findProfile 校验存在

	// 1. 解析 mihomo 配置路径（提前获取用于保留字段）
	mihomoPath, err := config.GetMihomoConfigPath()
	if err != nil {
		return ActivateResult{}, ErrMihomoPathMissing
	}

	// 2. 生成（merge 损坏时降级；保留现有 mihomo 配置中的托管字段）
	data, mergeErr := profile.GenerateAndWriteForUIDIgnoreMergeErrorWithPreserve(uid, mihomoPath)
	if len(data) == 0 {
		// raw.yaml 尚未拉取（最常见），给出可操作的提示。
		return ActivateResult{}, profile.ErrRawNotFound
	}

	// 3. 配置未变更时跳过写盘与重载。
	// yaml.v3 序列化确定性保证：相同 raw+merge 生成字节一致。
	// 跳过可避免重复触发 mihomo ApplyConfig 重建 proxies/rules/providers
	// （其固有内存峰值是用户感知"切换订阅内存升高"的直接来源）。
	if existing, rerr := os.ReadFile(mihomoPath); rerr == nil && bytes.Equal(existing, data) {
		// 仅同步激活 UID 后返回（BackupName 为空表示未产生新备份）。
		if err := s.SetActiveSub(uid); err != nil {
			_ = err
		}
		return ActivateResult{MergeErr: mergeErr}, nil
	}

	// 4. 备份现有 config.yaml
	backupName, err := profile.BackupConfig(mihomoPath)
	if err != nil {
		return ActivateResult{}, fmt.Errorf("备份配置失败: %w", err)
	}

	// 5. 原子写入最终配置
	if err := profile.WriteFinal(mihomoPath, data); err != nil {
		return ActivateResult{}, fmt.Errorf("写入最终配置失败: %w", err)
	}

	// 6. 热重载核心
	if s.client == nil {
		return ActivateResult{}, errors.New("未配置 mihomo 客户端，无法重载核心")
	}
	if err := s.client.ReloadConfig(mihomoPath); err != nil {
		// 重载失败：文件已写入，不回滚。返回错误供调用方提示。
		return ActivateResult{}, fmt.Errorf("核心重载失败（配置已写入）: %w", err)
	}

	// 7. 更新激活 UID
	if err := s.SetActiveSub(uid); err != nil {
		// 元数据更新失败不致命（核心已重载），仅返回结果。
		_ = err
	}

	return ActivateResult{BackupName: backupName, MergeErr: mergeErr}, nil
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
	if _, err := profile.RawPath(uid); err != nil {
		return nil, err
	}

	// 写入 raw.yaml
	if err := profile.WriteRaw(uid, data); err != nil {
		_ = profile.DeleteProfileDir(uid)
		return nil, fmt.Errorf("写入本地订阅配置失败: %w", err)
	}

	// 构建 Profile 元数据（Source.Path 指向复制后的 raw.yaml，而非原始 mihomo 配置路径）
	rawPath, _ := profile.RawPath(uid)
	p := profile.Profile{
		UID:       uid,
		Name:      i18nLocalSubName,
		Source:    profile.SubSource{Kind: profile.SourceLocal, Path: rawPath},
		UpdatedAt: time.Now().Unix(),
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
	for _, p := range cfg.Subs {
		if p.UID == uid {
			return p, nil
		}
	}
	return profile.Profile{}, ErrSubNotFound
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
func (s *ProfileService) mutateProfile(uid string, fn func(*profile.Profile)) error {
	return s.persistMutation(func(cfg *config.Config) {
		for i := range cfg.Subs {
			if cfg.Subs[i].UID == uid {
				fn(&cfg.Subs[i])
				return
			}
		}
	})
}

// persistMutation 加载配置、应用变更函数、保存。
// 保存采用读-改-写，调用方在 fn 内完成对 cfg 的就地修改。
func (s *ProfileService) persistMutation(fn func(*config.Config)) error {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrConfigNotFound) {
		defaultCopy := config.DefaultConfig
		cfg = &defaultCopy
		cfg.Subs = []profile.Profile{}
		cfg.ActiveSub = ""
	} else if err != nil {
		return err
	}
	fn(cfg)
	return config.Save(cfg)
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
