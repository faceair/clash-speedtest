package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/faceair/clash-speedtest/speedtester"
)

func (m *tuiModel) toggleDetail(result *speedtester.Result) {
	if result == nil {
		return
	}
	if m.detailVisible && m.detailResult == result {
		m.detailVisible = false
		m.updateTableLayout()
		return
	}
	m.detailResult = result
	m.detailVisible = true
	m.updateTableLayout()
}

func (m tuiModel) detailPanelView() string {
	if !m.detailVisible || m.detailResult == nil {
		return ""
	}
	panelWidth := m.detailPanelWidth()
	contentWidth := max(10, panelWidth-2)
	content := buildDetailContent(m.detailResult, contentWidth, m.mode)
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Width(panelWidth).Render(content)
}

func (m tuiModel) detailPanelWidth() int {
	if m.windowWidth == 0 {
		return defaultDetailWidth
	}
	width := max(detailPanelMinWidth, m.windowWidth-8)
	maxWidth := m.windowWidth - 2
	if maxWidth < 20 {
		maxWidth = m.windowWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	if width < 20 {
		width = 20
	}
	return width
}

func (m tuiModel) detailPanelHeight() int {
	if !m.detailVisible || m.detailResult == nil {
		return 0
	}
	panelWidth := m.detailPanelWidth()
	contentWidth := max(10, panelWidth-2)
	content := buildDetailContent(m.detailResult, contentWidth, m.mode)
	return lipgloss.Height(lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Width(panelWidth).Render(content))
}

func buildDetailContent(result *speedtester.Result, width int, mode speedtester.SpeedMode) string {
	lines := []string{
		fmt.Sprintf("Node: %s", result.ProxyName),
		fmt.Sprintf("Type: %s", result.ProxyType),
		"",
		fmt.Sprintf("Latency: %s", result.FormatLatency()),
	}
	if !mode.IsFast() {
		lines = append(lines,
			fmt.Sprintf("Jitter: %s", result.FormatJitter()),
			fmt.Sprintf("Packet Loss: %s", result.FormatPacketLoss()),
			"",
			fmt.Sprintf("Download: %s", result.FormatDownloadSpeedValue()),
		)
		lines = appendWrappedValue(lines, "Download Error:", result.FormatDownloadError(), width)
		if mode.UploadEnabled() {
			lines = append(lines, "", fmt.Sprintf("Upload: %s", result.FormatUploadSpeedValue()))
			lines = appendWrappedValue(lines, "Upload Error:", result.FormatUploadError(), width)
		}
	}
	lines = append(lines, "", "Press ESC to close details.")
	return strings.Join(lines, "\n")
}

func appendWrappedValue(lines []string, label, value string, width int) []string {
	if value == "" {
		value = "N/A"
	}
	prefix := label + " "
	wrapWidth := max(width-lipgloss.Width(prefix), 10)
	wrapped := wrapText(value, wrapWidth)
	for i, line := range wrapped {
		if i == 0 {
			lines = append(lines, prefix+line)
			continue
		}
		lines = append(lines, strings.Repeat(" ", lipgloss.Width(prefix))+line)
	}
	return lines
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	for _, rawLine := range strings.Split(text, "\n") {
		words := strings.Fields(rawLine)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			if lipgloss.Width(current)+1+lipgloss.Width(word) > width {
				lines = append(lines, current)
				current = word
				continue
			}
			current += " " + word
		}
		lines = append(lines, current)
	}
	return lines
}
