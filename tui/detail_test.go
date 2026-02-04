package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/speedtester"
)

func TestTUIModelDetailPanelToggle(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeFull, 1, resultChannel)
	model.windowWidth = 120
	model.windowHeight = 40

	result := &speedtester.Result{
		ProxyName:     "Error Proxy",
		ProxyType:     "Trojan",
		Latency:       200 * time.Millisecond,
		Jitter:        20 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadError: "download failed: timeout",
		UploadError:   "upload failed: 500",
	}
	model.results = append(model.results, result)
	model.updateTableRows()
	model.updateTableLayout()

	rowY := model.tableHeaderY() + tableHeaderLines
	click := tea.MouseMsg{
		X:      1,
		Y:      rowY,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}

	updated, _ := model.Update(click)
	updatedModel := updated.(tuiModel)
	if !updatedModel.detailVisible {
		t.Fatalf("expected detail panel to be visible after click")
	}
	if updatedModel.detailResult != result {
		t.Fatalf("expected detail result to match clicked row")
	}

	detail := updatedModel.detailPanelView()
	if !strings.Contains(detail, result.DownloadError) {
		t.Fatalf("expected detail view to include download error, got %q", detail)
	}
	if !strings.Contains(detail, result.UploadError) {
		t.Fatalf("expected detail view to include upload error, got %q", detail)
	}
	if updatedModel.table.Height() <= 0 {
		t.Fatalf("expected table height to remain positive when detail is visible")
	}

	closed, _ := updatedModel.Update(click)
	closedModel := closed.(tuiModel)
	if closedModel.detailVisible {
		t.Fatalf("expected detail panel to close on second click")
	}
}

func TestTUIModelDetailPanelSwitchOnClick(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeFull, 2, resultChannel)
	model.windowWidth = 120
	model.windowHeight = 40

	first := &speedtester.Result{
		ProxyName:  "First Proxy",
		ProxyType:  "SS",
		Latency:    120 * time.Millisecond,
		PacketLoss: 2.0,
	}
	second := &speedtester.Result{
		ProxyName:  "Second Proxy",
		ProxyType:  "Trojan",
		Latency:    180 * time.Millisecond,
		PacketLoss: 4.0,
	}
	model.results = []*speedtester.Result{first, second}
	model.updateTableRows()
	model.updateTableLayout()

	rowY := model.tableHeaderY() + tableHeaderLines
	clickFirst := tea.MouseMsg{
		X:      1,
		Y:      rowY,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}
	opened, _ := model.Update(clickFirst)
	openedModel := opened.(tuiModel)
	if !openedModel.detailVisible {
		t.Fatalf("expected detail panel to be visible after first click")
	}
	if openedModel.detailResult != first {
		t.Fatalf("expected detail result to match first row")
	}

	clickSecond := tea.MouseMsg{
		X:      1,
		Y:      rowY + 1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}
	updated, _ := openedModel.Update(clickSecond)
	updatedModel := updated.(tuiModel)
	if !updatedModel.detailVisible {
		t.Fatalf("expected detail panel to remain visible after switching rows")
	}
	if updatedModel.detailResult != second {
		t.Fatalf("expected detail result to switch to second row")
	}
}

func TestTUIModelDetailPanelEscRestoresLayout(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel)
	model.windowWidth = 120
	model.windowHeight = 40

	result := &speedtester.Result{
		ProxyName: "Error Proxy",
		ProxyType: "Trojan",
		Latency:   200 * time.Millisecond,
	}
	model.results = append(model.results, result)
	model.updateTableRows()
	model.updateTableLayout()
	closedHeight := model.table.Height()

	rowY := model.tableHeaderY() + tableHeaderLines
	click := tea.MouseMsg{
		X:      1,
		Y:      rowY,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}

	opened, _ := model.Update(click)
	openedModel := opened.(tuiModel)
	if !openedModel.detailVisible {
		t.Fatalf("expected detail panel to be visible after click")
	}
	openedHeight := openedModel.table.Height()
	if openedHeight >= closedHeight {
		t.Fatalf("expected table height to shrink when detail is visible: closed=%d opened=%d", closedHeight, openedHeight)
	}

	closed, _ := openedModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	closedModel := closed.(tuiModel)
	if closedModel.detailVisible {
		t.Fatalf("expected detail panel to close on ESC")
	}
	if closedModel.table.Height() != closedHeight {
		t.Fatalf("expected table height to restore after ESC close: want=%d got=%d", closedHeight, closedModel.table.Height())
	}
}

func TestBuildDetailContentDownloadOnly(t *testing.T) {
	result := &speedtester.Result{
		ProxyName:     "Error Proxy",
		ProxyType:     "Trojan",
		Latency:       200 * time.Millisecond,
		Jitter:        20 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadError: "download failed: timeout",
		UploadError:   "upload failed: 500",
	}
	content := buildDetailContent(result, 80, speedtester.SpeedModeDownload)
	if strings.Contains(content, "Upload") {
		t.Fatalf("expected download-only detail to omit upload section, got %q", content)
	}
	if strings.Contains(content, result.UploadError) {
		t.Fatalf("expected download-only detail to omit upload error, got %q", content)
	}
}

func TestDetailPanelHeightUpdatesOnSelectionChange(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(speedtester.SpeedModeFull, 2, resultChannel)
	model.windowWidth = 80
	model.windowHeight = 30

	shortResult := &speedtester.Result{
		ProxyName:     "Short",
		ProxyType:     "SS",
		Latency:       120 * time.Millisecond,
		Jitter:        20 * time.Millisecond,
		PacketLoss:    1.0,
		DownloadError: "timeout",
		UploadError:   "timeout",
	}
	longResult := &speedtester.Result{
		ProxyName:     "Long",
		ProxyType:     "Trojan",
		Latency:       180 * time.Millisecond,
		Jitter:        30 * time.Millisecond,
		PacketLoss:    2.0,
		DownloadError: strings.Repeat("download error ", 8),
		UploadError:   strings.Repeat("upload error ", 8),
	}

	model.results = []*speedtester.Result{shortResult, longResult}
	model.updateTableRows()
	model.detailVisible = true
	model.detailResult = shortResult
	model.updateTableLayout()
	initialHeight := model.table.Height()

	model.table.SetCursor(1)
	model.syncSelectionFromCursor()
	updatedHeight := model.table.Height()
	if model.detailResult != longResult {
		t.Fatalf("expected detail result to update to selected row, got %v", model.detailResult)
	}
	if initialHeight == updatedHeight {
		t.Fatalf("expected table height to change after detail content update: initial=%d updated=%d", initialHeight, updatedHeight)
	}
}
