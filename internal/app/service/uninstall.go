package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AimAI-Labs/mihosh/internal/infrastructure/config"
	coreinfra "github.com/AimAI-Labs/mihosh/internal/infrastructure/core"
	"github.com/AimAI-Labs/mihosh/internal/infrastructure/host"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
)

// UninstallOptions controls Mihosh uninstallation behavior.
type UninstallOptions struct {
	KeepConfig           bool `json:"keep_config"`
	DryRun               bool `json:"dry_run"`
	AllowInteractiveSudo bool `json:"-"`
}

// UninstallPlan describes what an uninstall operation will clean, preserve, or leave for manual removal.
type UninstallPlan struct {
	ExecutablePath    string   `json:"executable_path"`
	ResolvedPath      string   `json:"resolved_path,omitempty"`
	BinaryHomebrew    bool     `json:"binary_homebrew"`
	BinaryWritable    bool     `json:"binary_writable"`
	ServiceBinaryPath string   `json:"service_binary_path,omitempty"`
	ConfigDir         string   `json:"config_dir"`
	ConfigExists      bool     `json:"config_exists"`
	KeepConfig        bool     `json:"keep_config"`
	CoreRunning       bool     `json:"core_running"`
	ServiceInstalled  bool     `json:"service_installed"`
	ServiceRunning    bool     `json:"service_running"`
	ServiceUnit       string   `json:"service_unit,omitempty"`
	NetworkRecovery   bool     `json:"network_recovery"`
	ShellFiles        []string `json:"shell_files,omitempty"`
	ProxyScriptExists bool     `json:"proxy_script_exists"`
}

// UninstallResult summarizes the executed (or simulated, when DryRun is true) uninstall steps.
type UninstallResult struct {
	DryRun                  bool     `json:"dry_run"`
	KeepConfig              bool     `json:"keep_config"`
	CoreStopped             bool     `json:"core_stopped"`
	ServiceRemoved          bool     `json:"service_removed"`
	ServiceUnit             string   `json:"service_unit,omitempty"`
	NetworkRecovered        bool     `json:"network_recovered"`
	ShellFilesCleaned       []string `json:"shell_files_cleaned,omitempty"`
	ProxyScriptRemoved      bool     `json:"proxy_script_removed"`
	ConfigDir               string   `json:"config_dir"`
	ConfigRemoved           bool     `json:"config_removed"`
	ConfigPreserved         bool     `json:"config_preserved"`
	ServiceBinaryRemoved    bool     `json:"service_binary_removed"`
	ExecutablePath          string   `json:"executable_path,omitempty"`
	BinaryRemoved           bool     `json:"binary_removed"`
	BinaryDeferred          bool     `json:"binary_deferred"`
	BinaryHomebrew          bool     `json:"binary_homebrew"`
	BinaryNeedsManualRemove bool     `json:"binary_needs_manual_removal"`
	ManualCommand           string   `json:"manual_command,omitempty"`
	Warnings                []string `json:"warnings,omitempty"`
}

// UninstallService orchestrates complete or config-preserving removal of Mihosh.
type UninstallService struct {
	coreSvc          *CoreService
	executableLookup func() (string, error)
	homeDirLookup    func() (string, error)
	servicesFactory  func(root string) host.Services
	removeBinaryFn   func(ctx context.Context, path string, allowInteractiveSudo bool) binaryRemoveOutcome
	goos             string
}

// NewUninstallService creates an UninstallService optionally reusing an active CoreService.
func NewUninstallService(coreSvc *CoreService) *UninstallService {
	return &UninstallService{
		coreSvc:          coreSvc,
		executableLookup: os.Executable,
		homeDirLookup:    os.UserHomeDir,
		servicesFactory:  host.NewServices,
		removeBinaryFn:   removeExecutableFile,
		goos:             runtime.GOOS,
	}
}

// SetTargets overrides the detected executable and home directory (primarily for tests).
func (s *UninstallService) SetTargets(executable, home string) *UninstallService {
	s.executableLookup = func() (string, error) { return executable, nil }
	s.homeDirLookup = func() (string, error) { return home, nil }
	return s
}

func (s *UninstallService) currentGOOS() string {
	if s.goos != "" {
		return s.goos
	}
	return runtime.GOOS
}

func (s *UninstallService) lookupExecutable() (exe string, resolved string) {
	if s.executableLookup == nil {
		return "", ""
	}
	raw, err := s.executableLookup()
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", ""
	}
	if abs, err := filepath.Abs(raw); err == nil {
		raw = abs
	}
	res := raw
	if evaluated, err := filepath.EvalSymlinks(raw); err == nil && evaluated != "" {
		res = evaluated
	}
	return raw, res
}

func (s *UninstallService) lookupHome() string {
	if s.homeDirLookup == nil {
		return ""
	}
	home, err := s.homeDirLookup()
	if err != nil {
		return ""
	}
	return home
}

func (s *UninstallService) serviceManager(root string) host.Services {
	if s.servicesFactory != nil {
		return s.servicesFactory(root)
	}
	return host.NewServices(root)
}

// Plan inspects local state read-only and returns what uninstall would touch.
func (s *UninstallService) Plan(ctx context.Context, keepConfig bool) (UninstallPlan, error) {
	root, err := config.GetConfigDir()
	if err != nil {
		return UninstallPlan{}, err
	}
	exe, resolved := s.lookupExecutable()
	homebrew := (exe != "" && isHomebrewUpdateTarget(exe)) || (resolved != "" && isHomebrewUpdateTarget(resolved))
	writable := exe != "" && isDirWritable(filepath.Dir(exe))

	plan := UninstallPlan{
		ExecutablePath: exe,
		ResolvedPath:   resolved,
		BinaryHomebrew: homebrew,
		BinaryWritable: writable,
		ConfigDir:      root,
		KeepConfig:     keepConfig,
	}

	if info, err := os.Stat(root); err == nil && info.IsDir() {
		plan.ConfigExists = true
	}
	svcBin := filepath.Join(root, "bin", "mihosh")
	if _, err := os.Lstat(svcBin); err == nil {
		plan.ServiceBinaryPath = svcBin
	}
	if _, err := os.Stat(filepath.Join(root, "runtime", "network-recovery.json")); err == nil {
		plan.NetworkRecovery = true
	}
	if _, err := os.Lstat(filepath.Join(root, "proxy.sh")); err == nil {
		plan.ProxyScriptExists = true
	}
	if home := s.lookupHome(); home != "" {
		plan.ShellFiles = FindShellProxyStartupFiles(home)
	}

	if s.currentGOOS() == "linux" {
		manager := s.serviceManager(root)
		statusCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		svcState, _ := manager.Status(statusCtx)
		cancel()
		if svcState.Installed {
			plan.ServiceInstalled = true
			plan.ServiceRunning = svcState.Running
			plan.ServiceUnit = host.ServiceName(root) + ".service"
		}
	}

	if plan.ServiceRunning {
		plan.CoreRunning = true
	} else {
		coreSvc := s.coreSvc
		if coreSvc == nil {
			coreSvc = NewCoreService()
			defer coreSvc.Close()
		}
		if st, err := coreSvc.Status(); err == nil && st.Mode == "managed" && st.Running {
			plan.CoreRunning = true
		}
	}

	return plan, nil
}

// Uninstall stops managed runtime artifacts, removes shell/service integrations,
// optionally deletes the config directory, and removes the executable when permitted.
func (s *UninstallService) Uninstall(ctx context.Context, opts UninstallOptions) (UninstallResult, error) {
	plan, err := s.Plan(ctx, opts.KeepConfig)
	if err != nil {
		return UninstallResult{}, err
	}

	result := UninstallResult{
		DryRun:         opts.DryRun,
		KeepConfig:     opts.KeepConfig,
		ConfigDir:      plan.ConfigDir,
		ExecutablePath: plan.ExecutablePath,
	}

	if opts.DryRun {
		result.CoreStopped = plan.CoreRunning
		result.ServiceRemoved = plan.ServiceInstalled
		result.ServiceUnit = plan.ServiceUnit
		result.NetworkRecovered = plan.NetworkRecovery
		result.ShellFilesCleaned = append([]string(nil), plan.ShellFiles...)
		result.ProxyScriptRemoved = plan.ProxyScriptExists
		result.ServiceBinaryRemoved = plan.ServiceBinaryPath != ""
		if plan.ConfigExists {
			if opts.KeepConfig {
				result.ConfigPreserved = true
			} else {
				result.ConfigRemoved = true
			}
		}
		if plan.ExecutablePath != "" {
			if plan.BinaryHomebrew {
				result.BinaryHomebrew = true
				result.ManualCommand = "brew uninstall mihosh"
			} else if !plan.BinaryWritable {
				result.BinaryNeedsManualRemove = true
				if s.currentGOOS() == "windows" {
					result.ManualCommand = fmt.Sprintf("del /F /Q %q", plan.ExecutablePath)
				} else {
					result.ManualCommand = "sudo rm -f " + quoteShellProxy(plan.ExecutablePath)
				}
			} else {
				result.BinaryRemoved = true
			}
		}
		return result, nil
	}

	root := plan.ConfigDir
	home := s.lookupHome()

	// 1. Stop and uninstall Linux systemd user service before deleting files.
	if s.currentGOOS() == "linux" && plan.ServiceInstalled {
		manager := s.serviceManager(root)
		svcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		svcErr := manager.Action(svcCtx, "uninstall")
		cancel()
		if svcErr != nil {
			if plan.ServiceRunning {
				return result, fmt.Errorf("uninstall running background service: %w", svcErr)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to remove background service %s: %v", plan.ServiceUnit, svcErr))
		} else {
			result.ServiceRemoved = true
			result.ServiceUnit = plan.ServiceUnit
			if plan.ServiceRunning {
				result.CoreStopped = true
			}
		}
	}

	// 2. Stop managed session core (if running in this process or local owner).
	coreSvc := s.coreSvc
	ownsTempCore := false
	if coreSvc == nil {
		coreSvc = NewCoreService()
		ownsTempCore = true
	}
	defer func() {
		if ownsTempCore && coreSvc != nil {
			coreSvc.Close()
		}
	}()
	stopped, stopErr := coreSvc.StopManagedForUninstall(ctx)
	if stopErr != nil {
		if plan.CoreRunning {
			return result, fmt.Errorf("stop managed core: %w", stopErr)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("stop managed core: %v", stopErr))
	} else if stopped {
		result.CoreStopped = true
	}

	// 3. Recover any lingering system proxy or TUN state recorded by Mihosh.
	if _, statErr := os.Stat(filepath.Join(root, "runtime", "network-recovery.json")); statErr == nil {
		if recErr := coreSvc.RecoverNetwork(ctx); recErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("recover network settings: %v", recErr))
		} else {
			result.NetworkRecovered = true
		}
	}

	// 4. Clean shell startup hooks (~/.bashrc, ~/.zshrc, $ZDOTDIR/.zshrc) and proxy.sh.
	if home != "" {
		cleaned, shellErr := RemoveShellProxyStartupInHome(home)
		result.ShellFilesCleaned = cleaned
		if shellErr != nil {
			result.Warnings = append(result.Warnings, shellErr.Error())
		}
	}
	proxyScript := filepath.Join(root, "proxy.sh")
	if _, statErr := os.Lstat(proxyScript); statErr == nil {
		if rmErr := os.Remove(proxyScript); rmErr != nil && !os.IsNotExist(rmErr) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("remove %s: %v", proxyScript, rmErr))
		} else {
			result.ProxyScriptRemoved = true
		}
	}

	// 5. Wait briefly for any exiting network-guard process to release runtime/guard.lock.
	waitForNetworkGuardExit(filepath.Join(root, "runtime", "guard.lock"))

	// 6. Keep or delete the Mihosh config directory.
	if opts.KeepConfig {
		svcBin := filepath.Join(root, "bin", "mihosh")
		if _, statErr := os.Lstat(svcBin); statErr == nil {
			if rmErr := os.Remove(svcBin); rmErr != nil && !os.IsNotExist(rmErr) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("remove service binary %s: %v", svcBin, rmErr))
			} else {
				result.ServiceBinaryRemoved = true
				_ = os.Remove(filepath.Join(root, "bin"))
			}
		}
		if _, statErr := os.Stat(filepath.Join(root, "config.yaml")); statErr == nil &&
			(result.ServiceRemoved || len(result.ShellFilesCleaned) > 0 || result.ProxyScriptRemoved) {
			if updErr := config.Update(func(cfg *config.Config) error {
				if result.ServiceRemoved && cfg.Core.Runtime == "service" {
					cfg.Core.Runtime = "session"
				}
				if len(result.ShellFilesCleaned) > 0 || result.ProxyScriptRemoved {
					cfg.ShellProxy = false
				}
				return nil
			}); updErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("update preserved config.yaml: %v", updErr))
			}
		}
		result.ConfigPreserved = plan.ConfigExists
	} else {
		// Release control server & owner.json before removing the config directory.
		coreSvc.Close()
		if ownsTempCore {
			ownsTempCore = false
		}
		if plan.ConfigExists {
			if !isSafeConfigDirForRemoval(root, home) {
				return result, fmt.Errorf("refusing to remove unsafe config directory path: %s", root)
			}
			if rmErr := os.RemoveAll(root); rmErr != nil {
				return result, fmt.Errorf("remove config directory %s: %w", root, rmErr)
			}
			result.ConfigRemoved = true
			if plan.ServiceBinaryPath != "" {
				result.ServiceBinaryRemoved = true
			}
		}
	}

	// 7. Remove the Mihosh executable (skipping Homebrew-managed binaries).
	if plan.ExecutablePath != "" {
		if plan.BinaryHomebrew {
			result.BinaryHomebrew = true
			result.ManualCommand = "brew uninstall mihosh"
		} else if _, statErr := os.Lstat(plan.ExecutablePath); os.IsNotExist(statErr) {
			result.BinaryRemoved = true
		} else {
			removeFn := s.removeBinaryFn
			if removeFn == nil {
				removeFn = removeExecutableFile
			}
			outcome := removeFn(ctx, plan.ExecutablePath, opts.AllowInteractiveSudo)
			if outcome.Err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("remove executable %s: %v", plan.ExecutablePath, outcome.Err))
			}
			result.BinaryRemoved = outcome.Removed
			result.BinaryDeferred = outcome.Deferred
			result.BinaryNeedsManualRemove = outcome.NeedsManual
			result.ManualCommand = outcome.ManualCommand
			if outcome.Removed && plan.ResolvedPath != "" && plan.ResolvedPath != plan.ExecutablePath && !isHomebrewUpdateTarget(plan.ResolvedPath) {
				if _, statErr := os.Lstat(plan.ResolvedPath); statErr == nil {
					extra := removeFn(ctx, plan.ResolvedPath, opts.AllowInteractiveSudo)
					if extra.Err != nil {
						result.Warnings = append(result.Warnings, fmt.Sprintf("remove resolved executable %s: %v", plan.ResolvedPath, extra.Err))
					} else if extra.NeedsManual {
						result.BinaryRemoved = false
						result.BinaryNeedsManualRemove = true
						result.ManualCommand = extra.ManualCommand
					}
				}
			}
		}
	}

	return result, nil
}

func waitForNetworkGuardExit(lockPath string) {
	if _, err := os.Stat(lockPath); err != nil {
		return
	}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for {
		unlock, err := coreinfra.LockRuntime(lockPath)
		if err == nil {
			unlock()
			return
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func isSafeConfigDirForRemoval(root, home string) bool {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return false
	}
	cleaned := filepath.Clean(root)
	vol := filepath.VolumeName(cleaned)
	if cleaned == "/" || cleaned == `\` || (vol != "" && cleaned == vol+`\`) || cleaned == vol {
		return false
	}
	if home != "" && cleaned == filepath.Clean(home) {
		return false
	}
	trimmed := strings.TrimPrefix(cleaned, vol)
	trimmed = strings.TrimLeft(trimmed, `/\`)
	if trimmed == "" {
		return false
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool { return r == '/' || r == '\\' })
	return len(parts) >= 2
}

// FormatUninstallSummary renders a localized human-readable summary shared by CLI and TUI exit notices.
func FormatUninstallSummary(res UninstallResult) string {
	var lines []string
	switch {
	case res.DryRun:
		lines = append(lines, i18n.T("uninstall.summary.header_dry_run"))
	case res.BinaryNeedsManualRemove || res.BinaryHomebrew || len(res.Warnings) > 0:
		lines = append(lines, i18n.T("uninstall.summary.header_partial"))
	default:
		lines = append(lines, i18n.T("uninstall.summary.header_complete"))
	}

	if res.CoreStopped {
		lines = append(lines, "- "+i18n.T("uninstall.summary.core_stopped"))
	}
	if res.ServiceRemoved {
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.service_removed", res.ServiceUnit))
	}
	if res.NetworkRecovered {
		lines = append(lines, "- "+i18n.T("uninstall.summary.network_recovered"))
	}
	if len(res.ShellFilesCleaned) > 0 {
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.shell_cleaned", strings.Join(res.ShellFilesCleaned, ", ")))
		lines = append(lines, "  "+i18n.T("uninstall.summary.shell_reload"))
	}
	if res.ProxyScriptRemoved {
		lines = append(lines, "- "+i18n.T("uninstall.summary.proxy_script_removed"))
	}
	if res.ServiceBinaryRemoved && res.ConfigPreserved {
		lines = append(lines, "- "+i18n.T("uninstall.summary.service_binary_removed"))
	}
	if res.ConfigRemoved {
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.config_removed", res.ConfigDir))
	} else if res.ConfigPreserved {
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.config_preserved", res.ConfigDir))
	}
	switch {
	case res.BinaryRemoved:
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.binary_removed", res.ExecutablePath))
	case res.BinaryDeferred:
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.binary_deferred", res.ExecutablePath))
	case res.BinaryHomebrew:
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.binary_homebrew", res.ManualCommand))
	case res.BinaryNeedsManualRemove:
		lines = append(lines, "- "+i18n.Tf("uninstall.summary.binary_manual", res.ExecutablePath, res.ManualCommand))
	}
	for _, w := range res.Warnings {
		lines = append(lines, "! "+i18n.Tf("uninstall.summary.warning", w))
	}
	return strings.Join(lines, "\n")
}
