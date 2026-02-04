package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *tuiModel) updateTableLayout() {
	if m.windowWidth == 0 || m.windowHeight == 0 {
		return
	}
	columns := buildColumns(addSortIndicators(m.baseHeaders, m.sortColumn, m.sortAscending), m.windowWidth, m.mode)
	m.table.SetColumns(columns)
	m.table.SetWidth(m.windowWidth)
	reserved := 2
	if m.detailVisible && m.detailResult != nil {
		detailHeight := m.detailPanelHeight()
		if detailHeight > 0 {
			reserved += detailHeight + 1
		}
	}
	tableHeight := max(6, m.windowHeight-reserved)
	m.table.SetHeight(tableHeight)
}

func (m tuiModel) progressLine() string {
	elapsed := time.Since(m.startTime)
	eta := formatETA(elapsed, m.currentProxy, m.totalProxies)
	state := "Testing"
	if !m.testing {
		state = "Completed"
	}
	info := fmt.Sprintf("%s %d/%d", state, m.currentProxy, m.totalProxies)
	metrics := fmt.Sprintf("Elapsed %s | ETA %s", formatDuration(elapsed), eta)
	barWidth := 40
	if m.windowWidth > 0 {
		available := min(max(m.windowWidth-lipgloss.Width(info)-lipgloss.Width(metrics)-lipgloss.Width(" | ")-1, 10), 40)
		barWidth = available
	}
	progressModel := m.progress
	progressModel.Width = barWidth
	bar := progressModel.View()
	return fmt.Sprintf("%s %s | %s", info, bar, metrics)
}

func formatDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	seconds := int(value.Seconds())
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, remainingSeconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, remainingSeconds)
}

func formatETA(elapsed time.Duration, current, total int) string {
	if current <= 0 || total <= 0 {
		return "N/A"
	}
	progress := float64(current) / float64(total)
	if progress <= 0 {
		return "N/A"
	}
	estimatedTotal := time.Duration(float64(elapsed) / progress)
	remaining := max(estimatedTotal-elapsed, 0)
	return formatDuration(remaining)
}

func timerTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return timerTickMsg{}
	})
}
