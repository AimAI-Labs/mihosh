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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	if err != nil {
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

// LoadMerge 读取某订阅的 merge.yaml 内容（不存在返回 nil）。
func (s *ProfileService) LoadMerge(uid string) ([]byte, error) {
	if _, err := s.findProfile(uid); err != nil {
		return nil, err
	}
	return profile.ReadMerge(uid)
}

// SaveMerge 写入 merge.yaml（语法校验在 profile.WriteMerge 内）。
func (s *ProfileService) SaveMerge(uid string, content []byte) error {
	if _, err := s.findProfile(uid); err != nil {
		return err
	}
	return profile.WriteMerge(uid, content)
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

	// 1. 生成（merge 损坏时降级；data 为空表示 raw 缺失或序列化失败）
	data, mergeErr := profile.GenerateAndWriteForUIDIgnoreMergeError(uid)
	if len(data) == 0 {
		// raw.yaml 尚未拉取（最常见），给出可操作的提示。
		return ActivateResult{}, profile.ErrRawNotFound
	}

	// 2. 解析 mihomo 配置路径
	mihomoPath, err := config.GetMihomoConfigPath()
	if err != nil {
		return ActivateResult{}, ErrMihomoPathMissing
	}

	// 3. 备份现有 config.yaml
	backupName, err := profile.BackupConfig(mihomoPath)
	if err != nil {
		return ActivateResult{}, fmt.Errorf("备份配置失败: %w", err)
	}

	// 4. 原子写入最终配置
	if err := profile.WriteFinal(mihomoPath, data); err != nil {
		return ActivateResult{}, fmt.Errorf("写入最终配置失败: %w", err)
	}

	// 5. 热重载核心
	if s.client == nil {
		return ActivateResult{}, errors.New("未配置 mihomo 客户端，无法重载核心")
	}
	if err := s.client.ReloadConfig(mihomoPath); err != nil {
		// 重载失败：文件已写入，不回滚。返回错误供调用方提示。
		return ActivateResult{}, fmt.Errorf("核心重载失败（配置已写入）: %w", err)
	}

	// 6. 更新激活 UID
	if err := s.SetActiveSub(uid); err != nil {
		// 元数据更新失败不致命（核心已重载），仅返回结果。
		_ = err
	}

	return ActivateResult{BackupName: backupName, MergeErr: mergeErr}, nil
}

// ===== 内部辅助 =====

// findProfile 按 UID 查找订阅元数据。
func (s *ProfileService) findProfile(uid string) (profile.Profile, error) {
	cfg, err := config.Load()
	if err != nil {
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
	if err != nil {
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
