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
	model := NewTUIModel(false, 1, resultChannel)
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

	detail := updatedModel.detailPanelView(updatedModel.table.View())
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
