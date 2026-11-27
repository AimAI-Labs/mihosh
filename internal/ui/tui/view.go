package tui

import (
	"fmt"
	"strings"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/common"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/dialog"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/layout"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/components/notice"
	"github.com/AimAI-Labs/mihosh/internal/ui/tui/features/help"
	"github.com/AimAI-Labs/mihosh/pkg/i18n"
	"github.com/charmbracelet/lipgloss"
)

// View 渲染视图
func (m Model) View() string {
	if m.width == 0 {
		return i18n.T("status.initializing")
	}

	// ── 布局参数 ──
	page := m.mainPageLayout()

	// ── 顶部导航 ──
	topNav := layout.RenderTopNav(m.currentPage, m.width, layout.TopNavRefreshStatus{
		Enabled:          m.autoRefreshInterval() > 0,
		SecondsRemaining: m.autoRefreshRemaining,
		Interval:         m.autoRefreshInterval(),
		Synced:           m.autoRefreshSynced,
	})

	// ── 渲染当前页面内容（固定高度，确保状态栏不崩塌）──
	pageContent := clampToHeight(m.renderPage(page.width, page.height), page.height)

	// ── 底部状态栏 ──
	var uploadTotal, downloadTotal int64
	if m.connsState.Connections != nil {
		uploadTotal = m.connsState.Connections.UploadTotal
		downloadTotal = m.connsState.Connections.DownloadTotal
	}

	// 提取当前活动节点信息（复用 GetActiveProxyAndDelay 统一逻辑）
	var groupName, nodeName string
	var delay int
	nodeName, delay, _ = m.nodesState.GetActiveProxyAndDelay()
	if nodeName != "" {
		// 根据模式反推所属策略组名
		groupName = m.nodesState.ResolveActiveGroupName()
	}

	statusBar := layout.RenderStatusBar(
		m.width,
		m.chartData,
		uploadTotal,
		downloadTotal,
		m.nodesState.Mode,
		groupName,
		nodeName,
		delay,
		m.hasUpdate,
	)

	footer := strings.LastIndexByte(pageContent, '\n')
	bodyContent, footerContent := pageContent, ""
	if footer >= 0 {
		bodyContent = pageContent[:footer]
		footerContent = pageContent[footer+1:]
	}
	fullPage := lipgloss.JoinVertical(lipgloss.Left, topNav, bodyContent,
		notice.Render(m.visibleNotice(), m.width), page.renderFooter(footerContent), statusBar)

	// ── 报错弹窗叠加 ──
	if m.notice.DetailsOpen {
		fullPage = overlayErrorPopup(fullPage, m.width, m.height, m.notice.Message.Detail)
	}

	// ── 更新弹窗叠加 ──
	if m.showUpdateDialog && m.updateInfo != nil {
		fullPage = overlayUpdateDialog(fullPage, m.width, m.height, m.updateInfo, m.isUpdating, m.updateError)
	}

	// ── 引导弹窗叠加 ──
	if m.guideState.Active {
		return m.guideState.Render(m.width, m.height)
	}

	// ── 帮助弹窗叠加（lazygit 风格，叠加在完整页面之上）──
	// 节点页始终显示右下角内联帮助提示；按 ? 可叠加完整帮助弹窗
	if m.showHelp {
		return help.OverlayHelpPopup(fullPage, m.width, m.height, m.buildHelpContext())
	}

	return fullPage
}

// buildHelpContext 根据当前模型状态构建帮助弹窗上下文
func (m *Model) buildHelpContext() help.HelpContext {
	platform, asset := m.coreState.ReleaseHelpDetails()
	return help.HelpContext{
		CurrentPage:        int(m.currentPage),
		CoreReleases:       m.coreState.BrowsingReleases(),
		CorePlatform:       platform,
		CoreAsset:          asset,
		CoreReleaseNotes:   m.coreState.ReleaseHelpNotes(),
		ScrollOffset:       m.helpState.Offset(),
		ConnViewMode:       m.connsState.ViewMode(),
		ConnDetail:         m.connsState.DetailMode(),
		ConnTopN:           m.connsState.TopNModalMode(),
		ConnIPModal:        m.connsState.CurrentIPModalOpen(),
		ConnFilter:         m.connsState.FilterMode(),
		NodesTestDetail:    m.nodesState.ShowTestDetail,
		NodesFilterMode:    m.nodesState.NodeFilterMode,
		NodesChainModal:    m.nodesState.ChainModal.Open,
		LogsDetail:         m.logsState.DetailMode(),
		LogsFilter:         m.logsState.FilterMode(),
		RulesTypeFilter:    m.rulesState.ShowTypeFilter(),
		RulesFilter:        m.rulesState.FilterMode(),
		RulesDeleteConfirm: m.rulesState.ShowDeleteConfirm(),
		RulesEditForm:      m.rulesState.ShowEditForm(),
		SubMode:            m.subState.Mode(),
		SettingsEdit:       m.settingsState.IsEditing(),
		SettingsLanguage:   m.settingsState.IsLanguageSelected(),
		SettingsTheme:      m.settingsState.IsThemeSelected(),
	}
}

// clampToHeight 将内容字符串精确约束为 h 行：
// 不足时在末尾补空行，超出时截断多余行。
// 这保证了 lipgloss.JoinVertical 后状态栏始终位于终端最底部。
func clampToHeight(content string, h int) string {
	if h <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	if len(lines) < h {
		// 补空行
		for len(lines) < h {
			lines = append(lines, "")
		}
	} else if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// overlayErrorPopup 将报错信息完整弹窗居中叠加在 base 页面之上
func overlayErrorPopup(base string, width, height int, errText string) string {
	// ── 1. 弹窗尺寸计算 ──
	popupWidth := width * 60 / 100
	if popupWidth < 40 {
		popupWidth = 40
	}
	if popupWidth > 80 {
		popupWidth = 80
	}
	if popupWidth > width {
		popupWidth = width
	}

	// ── 2. 内容换行与截断 ──
	contentWidth := popupWidth - 4 // border 2 + padding 2
	wrappedErr := lipgloss.NewStyle().Width(contentWidth).Render(errText)
	errLines := strings.Split(wrappedErr, "\n")

	maxLines := height * 60 / 100
	if maxLines < 5 {
		maxLines = 5
	}
	if len(errLines) > maxLines {
		errLines = errLines[:maxLines]
		errLines = append(errLines, lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render("..."))
	}

	body := strings.Join(errLines, "\n")

	title := i18n.T("status.err.detail")
	popup := common.RenderBorderedPanel(title, body, popupWidth, common.TokyoRed(), common.TokyoRed())

	// ── 3. 弹窗居中合并 ──
	return dialog.New(popup, width, height).Overlay(base, true)
}

// overlayUpdateDialog 将更新弹窗完整弹窗居中叠加在 base 页面之上
func overlayUpdateDialog(base string, width, height int, info *model.UpdateInfo, isUpdating bool, err error) string {
	// ── 1. 弹窗尺寸计算 ──
	popupWidth := width * 60 / 100
	if popupWidth < 40 {
		popupWidth = 40
	}
	if popupWidth > 80 {
		popupWidth = 80
	}
	if popupWidth > width {
		popupWidth = width
	}

	// ── 2. 构建弹窗内容 ──
	contentWidth := popupWidth - 4 // border 2 + padding 2

	var body string
	if isUpdating {
		body = "\n" + lipgloss.NewStyle().Foreground(common.TokyoCyan()).Render("  "+i18n.T("update.downloading")) + "\n\n"
	} else if err != nil {
		body = "\n" + lipgloss.NewStyle().Foreground(common.TokyoRed()).Render("  "+i18n.Tf("update.failed", err.Error())) + "\n\n"
		body += lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render("  " + i18n.T("update.close"))
	} else {
		verStr := fmt.Sprintf("%s  →  %s", info.CurrentVersion, info.Version)
		body = lipgloss.NewStyle().Foreground(common.TokyoCyan()).Bold(true).Render(verStr) + "\n\n"
		body += lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(i18n.T("update.release_page")+"\n"+info.ReleaseURL) + "\n\n"
		if info.HomebrewManaged {
			body += i18n.T("update.homebrew_install")
		} else if info.DownloadURL == "" {
			body += i18n.T("update.manual_install")
		} else {

			leftBtn := lipgloss.NewStyle().Foreground(common.TokyoGreen()).Render(i18n.T("update.apply_now"))
			rightBtn := lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render(i18n.T("update.later"))

			halfWidth := contentWidth / 2
			leftHalf := lipgloss.NewStyle().Width(halfWidth).Align(lipgloss.Right).Render(leftBtn + "  ")
			rightHalf := lipgloss.NewStyle().Width(contentWidth - halfWidth).Align(lipgloss.Left).Render("  " + rightBtn)
			buttons := lipgloss.JoinHorizontal(lipgloss.Top, leftHalf, rightHalf)

			body += buttons
		}
	}
	wrappedBody := lipgloss.NewStyle().Width(contentWidth).Render(body)
	bodyLines := strings.Split(wrappedBody, "\n")

	maxLines := height * 70 / 100
	if maxLines < 8 {
		maxLines = 8
	}
	if len(bodyLines) > maxLines {
		bodyLines = bodyLines[:maxLines]
		bodyLines = append(bodyLines, lipgloss.NewStyle().Foreground(common.TokyoMuted()).Render("..."))
	}

	finalBody := strings.Join(bodyLines, "\n")

	title := i18n.T("update.title")
	borderColor := common.TokyoCyan()
	if err != nil {
		borderColor = common.TokyoRed()
	}
	popup := common.RenderBorderedPanel(title, finalBody, popupWidth, borderColor, borderColor)

	// ── 3. 弹窗居中合并 ──
	return dialog.New(popup, width, height).Overlay(base, true)
}
