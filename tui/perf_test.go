package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/speedtester"
)

func TestPerfTrackerRecordsSortAndRowsOnResultUpdate(t *testing.T) {
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF", "1")
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF_LOG_EVERY", "0")

	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 20, resultChannel)

	updates := 20
	for i := 0; i < updates; i++ {
		result := &speedtester.Result{
			ProxyName:     fmt.Sprintf("Proxy %d", i),
			ProxyType:     "SS",
			DownloadSpeed: float64(i),
			ProxyConfig:   map[string]any{},
		}
		updated, _ := model.Update(resultMsg{result: result})
		model = updated.(tuiModel)
	}
	updated, _ := model.Update(flushResultsMsg{})
	model = updated.(tuiModel)

	sortStats := model.perf.snapshot(perfEventSort)
	if sortStats.Count != 1 {
		t.Fatalf("expected sort count %d, got %d", 1, sortStats.Count)
	}
	rowsStats := model.perf.snapshot(perfEventRows)
	if rowsStats.Count != 1 {
		t.Fatalf("expected row rebuild count %d, got %d", 1, rowsStats.Count)
	}
	if rowsStats.ItemsMax != updates {
		t.Fatalf("expected max items %d, got %d", updates, rowsStats.ItemsMax)
	}
}

func TestPerfTrackerRecordsLayoutOnDetailScroll(t *testing.T) {
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF", "1")
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF_LOG_EVERY", "0")

	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 5, resultChannel)
	model.results = []*speedtester.Result{
		{ProxyName: "Proxy 1", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{ProxyName: "Proxy 2", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{ProxyName: "Proxy 3", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{ProxyName: "Proxy 4", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{ProxyName: "Proxy 5", ProxyType: "SS", ProxyConfig: map[string]any{}},
	}
	model.updateTableRows()
	model.windowWidth = 120
	model.windowHeight = 40
	model.table.SetHeight(3)
	model.detailVisible = true
	model.detailResult = model.results[0]
	model.table.SetCursor(0)
	model.refreshDetailHeight()
	model.updateTableLayout()

	before := model.perf.snapshot(perfEventLayout).Count
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updatedModel := updated.(tuiModel)
	after := updatedModel.perf.snapshot(perfEventLayout).Count

	if after != before {
		t.Fatalf("expected layout count to stay the same, got %d -> %d", before, after)
	}
}

func TestPerfTrackerUpdatesLayoutWhenDetailHeightChanges(t *testing.T) {
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF", "1")
	t.Setenv("CLASH_SPEEDTEST_TUI_PERF_LOG_EVERY", "0")

	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 2, resultChannel)
	model.results = []*speedtester.Result{
		{ProxyName: "Proxy 1", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{
			ProxyName:     "Proxy 2",
			ProxyType:     "SS",
			DownloadError: strings.Repeat("error ", 30),
			ProxyConfig:   map[string]any{},
		},
	}
	model.updateTableRows()
	model.windowWidth = 50
	model.windowHeight = 40
	model.table.SetHeight(3)
	model.detailVisible = true
	model.detailResult = model.results[0]
	model.table.SetCursor(0)
	model.refreshDetailHeight()
	model.updateTableLayout()

	before := model.perf.snapshot(perfEventLayout).Count
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updatedModel := updated.(tuiModel)
	after := updatedModel.perf.snapshot(perfEventLayout).Count

	if after != before+1 {
		t.Fatalf("expected layout count to increase by 1, got %d -> %d", before, after)
	}
}
