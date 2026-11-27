package i18n

import (
	"testing"
)

func TestGuideTranslationsExist(t *testing.T) {
	Init()
	SetLanguageOverride("zh-CN")
	if got := T("guide.title"); got == "guide.title" {
		t.Errorf("expected zh-CN guide.title translation, got fallback key")
	}
	SetLanguageOverride("en-US")
	if got := T("guide.title"); got == "guide.title" {
		t.Errorf("expected en-US guide.title translation, got fallback key")
	}
}

func TestUninstallTranslationsExist(t *testing.T) {
	Init()
	keys := []string{
		"cli.uninstall.short",
		"cli.uninstall.long",
		"cli.uninstall.flag_keep_config",
		"cli.uninstall.flag_yes",
		"cli.uninstall.flag_dry_run",
		"cli.uninstall.flag_output",
		"cli.uninstall.err_yes_required_json",
		"cli.uninstall.err_non_interactive",
		"cli.uninstall.preview_title",
		"cli.uninstall.prompt_confirm",
		"cli.uninstall.prompt_keep_config",
		"cli.uninstall.cancelled",
		"cli.uninstall.col_item",
		"cli.uninstall.col_status",
		"cli.uninstall.col_detail",
		"cli.uninstall.status_removed",
		"cli.uninstall.status_preserved",
		"cli.uninstall.status_stopped",
		"cli.uninstall.status_recovered",
		"cli.uninstall.status_skipped",
		"cli.uninstall.status_manual",
		"cli.uninstall.status_deferred",
		"cli.uninstall.status_planned",
		"uninstall.summary.header_dry_run",
		"uninstall.summary.header_partial",
		"uninstall.summary.header_complete",
		"uninstall.summary.core_stopped",
		"uninstall.summary.service_removed",
		"uninstall.summary.network_recovered",
		"uninstall.summary.shell_cleaned",
		"uninstall.summary.shell_reload",
		"uninstall.summary.proxy_script_removed",
		"uninstall.summary.service_binary_removed",
		"uninstall.summary.config_removed",
		"uninstall.summary.config_preserved",
		"uninstall.summary.binary_removed",
		"uninstall.summary.binary_deferred",
		"uninstall.summary.binary_homebrew",
		"uninstall.summary.binary_manual",
		"uninstall.summary.warning",
		"help.settings.uninstall",
		"help.settings.hint_uninstall",
		"settings.uninstall.modal_title",
		"settings.uninstall.modal_subtitle",
		"settings.uninstall.loading",
		"settings.uninstall.running",
		"settings.uninstall.label_binary",
		"settings.uninstall.label_service",
		"settings.uninstall.label_shell",
		"settings.uninstall.label_network",
		"settings.uninstall.label_config",
		"settings.uninstall.badge_homebrew",
		"settings.uninstall.badge_sudo",
		"settings.uninstall.value_service_remove",
		"settings.uninstall.value_shell_remove",
		"settings.uninstall.value_network_recover",
		"settings.uninstall.value_config_keep",
		"settings.uninstall.value_config_delete",
		"settings.uninstall.value_config_missing",
		"settings.uninstall.toggle_keep_config",
		"settings.uninstall.toggle_keep_config_warn",
		"settings.uninstall.btn_confirm",
		"settings.uninstall.btn_cancel",
		"settings.uninstall.hint_toggle",
		"settings.uninstall.hint_switch",
		"settings.uninstall.hint_confirm",
		"settings.uninstall.hint_cancel",
	}
	for _, lang := range []string{"zh-CN", "en-US"} {
		SetLanguageOverride(lang)
		for _, k := range keys {
			if got := T(k); got == k {
				t.Errorf("missing %s translation for %q", lang, k)
			}
		}
	}
	SetLanguageOverride("auto")
}
