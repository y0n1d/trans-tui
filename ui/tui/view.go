package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const recordBorderPadding = 2

func (m Model) renderView() string {
	if !m.ready {
		return "Initializing..."
	}
	th := m.themeOr()

	var sections []string

	// Disabled sections are omitted entirely (not rendered as ""), so the
	// heights from headerHeight/statusBarHeight and the joined view can
	// never disagree: lipgloss.JoinVertical counts every section as a row.
	if th.HeaderEnabled {
		sections = append(sections, m.renderHeader())
	}
	sections = append(sections, m.viewport.View())

	if m.inputMode {
		sections = append(sections, m.renderInputPanel())
	}

	if m.Error != "" {
		sections = append(sections, m.renderErrorPanel(m.terminalWidth))
	}

	if th.StatusBarEnabled {
		sections = append(sections, m.renderStatusBar())
	}

	if m.Loading {
		sections = append(sections, m.renderLoading())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	th := m.themeOr()
	title := th.Header.Render(th.HeaderTitle)
	if !th.HeaderShowRecordCount {
		return title
	}
	recordCount := th.HeaderRecordCount.Render(fmt.Sprintf("Records: %d", len(m.Records)))
	return lipgloss.JoinHorizontal(lipgloss.Top, title, " ", recordCount)
}

// renderStatusBar joins the enabled indicators with " | ". Every flag is
// checked, so a fully disabled bar renders an empty (one-row) line while
// headerHeight/statusBarHeight keep the layout arithmetic exact.
func (m Model) renderStatusBar() string {
	th := m.themeOr()

	var parts []string
	if th.ShowRecordCount {
		parts = append(parts, fmt.Sprintf("Records: %d", len(m.Records)))
	}
	if th.ShowScrollPosition {
		parts = append(parts, fmt.Sprintf("Scroll: %d (%.0f%%)", m.viewport.YOffset(), m.viewport.ScrollPercent()*100))
	}
	if m.inputMode {
		if th.ShowCancelHint {
			parts = append(parts, firstKey(m.keyMap.CancelInput)+": cancel")
		}
	} else {
		if th.ShowInputHint {
			parts = append(parts, firstKey(m.keyMap.ManualInput)+": input")
		}
		if th.ShowQuitHint {
			parts = append(parts, firstKey(m.keyMap.Quit)+": quit")
		}
	}
	return th.StatusBar.Render(strings.Join(parts, " | "))
}

func (m Model) renderErrorPanel(width int) string {
	if m.Error == "" {
		return ""
	}
	content := fmt.Sprintf("⚠ %s", m.Error)
	if width < 1 {
		width = 1
	}
	return m.themeOr().ErrorPanel.Width(width).Render(content)
}

func (m Model) renderLoading() string {
	th := m.themeOr()
	return th.Loading.Render(th.LoadingText)
}

func (m Model) renderInputPanel() string {
	// The textarea renders its own prompt internally.
	content := m.textArea.View()
	return m.themeOr().InputPanel.Width(m.inputPanelOuterWidth()).Render(content)
}
