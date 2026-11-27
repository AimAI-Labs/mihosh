package connections

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestDNSDialogRefreshesTranslationOnReopen(t *testing.T) {
	i18n.Init()
	t.Cleanup(func() { i18n.SetLanguageOverride("auto") })
	i18n.SetLanguageOverride("en-US")
	var s State
	_ = s.openDNSQuery()
	s.dns.input.Placeholder = "stale.invalid"
	s.closeDNSQuery()
	i18n.SetLanguageOverride("zh-CN")
	_ = s.openDNSQuery()
	require.Equal(t, i18n.T("conns.dns.domain_placeholder"), s.dns.input.Placeholder)
	dlg, _ := s.dns.popup(80, 24)
	plain := ansi.Strip(dlg.View())
	require.Contains(t, plain, i18n.T("conns.dns.title"))
	require.Contains(t, plain, i18n.T("conns.dns.domain_label"))
}
