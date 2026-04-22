package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/speedtester"
)

// TestTUIModelUpdate tests the TUI model update logic
func TestTUIModelUpdate(t *testing.T) {
	// Create a result channel
	resultChannel := make(chan *speedtester.Result, 10)

	// Create a new TUI model
	model := NewTUIModel(speedtester.SpeedModeDownload, 3, resultChannel)

	// Verify initial state
	if model.mode != speedtester.SpeedModeDownload {
		t.Errorf("Expected mode to be %v, got %v", speedtester.SpeedModeDownload, model.mode)
	}
	if model.totalProxies != 3 {
		t.Errorf("Expected totalProxies to be 3, got %d", model.totalProxies)
	}
	if model.currentProxy != 0 {
		t.Errorf("Expected currentProxy to be 0, got %d", model.currentProxy)
	}
	if len(model.results) != 0 {
		t.Errorf("Expected results length to be 0, got %d", len(model.results))
	}
	if model.testing != true {
		t.Errorf("Expected testing to be true, got %v", model.testing)
	}
	if model.quitting != false {
		t.Errorf("Expected quitting to be false, got %v", model.quitting)
	}

	// Create test results
	result1 := &speedtester.Result{
		ProxyName:     "Proxy 1",
		ProxyType:     "SS",
		Latency:       100 * time.Millisecond,
		Jitter:        50 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadSpeed: 15 * 1024 * 1024, // 15 MB/s
		UploadSpeed:   8 * 1024 * 1024,  // 8 MB/s
		ProxyConfig:   map[string]any{},
	}

	result2 := &speedtester.Result{
		ProxyName:     "Proxy 2",
		ProxyType:     "Trojan",
		Latency:       200 * time.Millisecond,
		Jitter:        100 * time.Millisecond,
		PacketLoss:    15.0,
		DownloadSpeed: 8 * 1024 * 1024, // 8 MB/s
		UploadSpeed:   3 * 1024 * 1024, // 3 MB/s
		ProxyConfig:   map[string]any{},
	}

	result3 := &speedtester.Result{
		ProxyName:     "Proxy 3",
		ProxyType:     "Vmess",
		Latency:       300 * time.Millisecond,
		Jitter:        200 * time.Millisecond,
		PacketLoss:    25.0,
		DownloadSpeed: 3 * 1024 * 1024, // 3 MB/s
		UploadSpeed:   1 * 1024 * 1024, // 1 MB/s
		ProxyConfig:   map[string]any{},
	}

	// Send first result
	resultChannel <- result1
	updatedModel, cmd := model.Update(resultMsg{result: result1})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 1 {
		t.Errorf("Expected currentProxy to be 1, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 1 {
		t.Errorf("Expected results length to be 1, got %d", len(updatedModel.(tuiModel).results))
	}
	if updatedModel.(tuiModel).results[0] != result1 {
		t.Error("Expected first result to be result1")
	}

	// Send second result
	resultChannel <- result2
	updatedModel, cmd = updatedModel.(tuiModel).Update(resultMsg{result: result2})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 2 {
		t.Errorf("Expected currentProxy to be 2, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 2 {
		t.Errorf("Expected results length to be 2, got %d", len(updatedModel.(tuiModel).results))
	}

	// Send third result
	resultChannel <- result3
	updatedModel, cmd = updatedModel.(tuiModel).Update(resultMsg{result: result3})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}
	if updatedModel.(tuiModel).currentProxy != 3 {
		t.Errorf("Expected currentProxy to be 3, got %d", updatedModel.(tuiModel).currentProxy)
	}
	if len(updatedModel.(tuiModel).results) != 3 {
		t.Errorf("Expected results length to be 3, got %d", len(updatedModel.(tuiModel).results))
	}

	// Flush updates to apply sorting and table refresh.
	updatedModel, _ = updatedModel.(tuiModel).Update(flushResultsMsg{})

	// Verify results are sorted by download speed (descending)
	// result1 (15 MB/s) > result2 (8 MB/s) > result3 (3 MB/s)
	if updatedModel.(tuiModel).results[0] != result1 {
		t.Error("Expected first result to be result1")
	}
	if updatedModel.(tuiModel).results[1] != result2 {
		t.Error("Expected second result to be result2")
	}
	if updatedModel.(tuiModel).results[2] != result3 {
		t.Error("Expected third result to be result3")
	}

	// Send done message
	updatedModel, cmd = updatedModel.(tuiModel).Update(doneMsg{})
	if updatedModel == nil {
		t.Error("Expected updatedModel to be non-nil")
	}
	if cmd == nil {
		t.Error("Expected cmd to be non-nil (progress update command)")
	}
	if updatedModel.(tuiModel).testing != false {
		t.Errorf("Expected testing to be false, got %v", updatedModel.(tuiModel).testing)
	}
	// Verify progress is complete by checking the percent
	if updatedModel.(tuiModel).progress.Percent() != 1.0 {
		t.Errorf("Expected progress percent to be 1.0, got %f", updatedModel.(tuiModel).progress.Percent())
	}
}

// TestTUIModelUpdateFastMode tests the TUI model update logic in fast mode
func TestTUIModelUpdateFastMode(t *testing.T) {
	// Create a result channel
	resultChannel := make(chan *speedtester.Result, 10)

	// Create a new TUI model in fast mode
	model := NewTUIModel(speedtester.SpeedModeFast, 3, resultChannel)

	// Verify initial state
	if model.mode != speedtester.SpeedModeFast {
		t.Errorf("Expected mode to be %v, got %v", speedtester.SpeedModeFast, model.mode)
	}

	// Create test results (only latency matters in fast mode)
	result1 := &speedtester.Result{
		ProxyName:   "Proxy 1",
		ProxyType:   "SS",
		Latency:     300 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	result2 := &speedtester.Result{
		ProxyName:   "Proxy 2",
		ProxyType:   "Trojan",
		Latency:     100 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	result3 := &speedtester.Result{
		ProxyName:   "Proxy 3",
		ProxyType:   "Vmess",
		Latency:     200 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	// Send results
	resultChannel <- result1
	updatedModel, _ := model.Update(resultMsg{result: result1})

	resultChannel <- result2
	updatedModel, _ = updatedModel.(tuiModel).Update(resultMsg{result: result2})

	resultChannel <- result3
	updatedModel, _ = updatedModel.(tuiModel).Update(resultMsg{result: result3})

	updatedModel, _ = updatedModel.(tuiModel).Update(flushResultsMsg{})

	// Verify results are sorted by latency (ascending)
	// result2 (100ms) < result3 (200ms) < result1 (300ms)
	if updatedModel.(tuiModel).results[0] != result2 {
		t.Error("Expected first result to be result2")
	}
	if updatedModel.(tuiModel).results[1] != result3 {
		t.Error("Expected second result to be result3")
	}
	if updatedModel.(tuiModel).results[2] != result1 {
		t.Error("Expected third result to be result1")
	}
}

func TestTUIModelRetestAllResetsState(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 4)
	model := NewTUIModel(speedtester.SpeedModeDownload, 2, resultChannel)
	model.testing = false
	model.currentProxy = 2
	model.progress.SetPercent(1.0)
	model.results = []*speedtester.Result{
		{ProxyName: "Proxy 1", ProxyType: "SS", ProxyConfig: map[string]any{}},
		{ProxyName: "Proxy 2", ProxyType: "Trojan", ProxyConfig: map[string]any{}},
	}
	model.updateTableRows()
	called := false
	model.SetRetestCallbacks(
		func(out chan<- *speedtester.Result) {
			called = true
			out <- nil
		},
		nil,
	)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updatedModel := updated.(tuiModel)
	if cmd == nil {
		t.Fatalf("expected retest-all to return a command")
	}
	progressCmd := updatedModel.progress.SetPercent(0)
	progressUpdated, _ := updatedModel.progress.Update(progressCmd())
	updatedModel.progress = progressUpdated.(progress.Model)
	updatedModel.runRetestAllCmd()()
	if !called {
		t.Fatalf("expected retest-all callback to run")
	}
	if !updatedModel.testing {
		t.Fatalf("expected model to enter testing state")
	}
	if updatedModel.currentProxy != 0 {
		t.Fatalf("expected currentProxy reset, got %d", updatedModel.currentProxy)
	}
	if len(updatedModel.results) != 0 {
		t.Fatalf("expected results to clear, got %d", len(updatedModel.results))
	}
	if updatedModel.selectedIndex != -1 {
		t.Fatalf("expected selection to reset, got %d", updatedModel.selectedIndex)
	}
	if updatedModel.progress.Percent() != 0 {
		t.Fatalf("expected progress percent reset to 0, got %f", updatedModel.progress.Percent())
	}
}

func TestTUIModelRetestOneReplacesExistingResult(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 4)
	model := NewTUIModel(speedtester.SpeedModeDownload, 2, resultChannel)
	oldSlow := &speedtester.Result{
		ProxyName:     "Slow",
		ProxyType:     "SS",
		Latency:       100 * time.Millisecond,
		DownloadSpeed: 2 * 1024 * 1024,
		ProxyConfig:   map[string]any{},
	}
	oldFast := &speedtester.Result{
		ProxyName:     "Fast",
		ProxyType:     "Trojan",
		Latency:       200 * time.Millisecond,
		DownloadSpeed: 10 * 1024 * 1024,
		ProxyConfig:   map[string]any{},
	}
	model.results = []*speedtester.Result{oldFast, oldSlow}
	model.recordSequence(oldFast)
	model.recordSequence(oldSlow)
	model.testing = false
	model.detailVisible = true
	model.detailResult = oldSlow
	model.selectedIndex = 1
	model.updateTableRows()

	newSlow := &speedtester.Result{
		ProxyName:     "Slow",
		ProxyType:     "SS",
		Latency:       90 * time.Millisecond,
		DownloadSpeed: 20 * 1024 * 1024,
		ProxyConfig:   map[string]any{},
	}
	calledName := ""
	model.SetRetestCallbacks(nil, func(name string, out chan<- *speedtester.Result) {
		calledName = name
		out <- newSlow
		out <- nil
	})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updatedModel := updated.(tuiModel)
	if cmd == nil {
		t.Fatalf("expected retest-one to return a command")
	}
	updatedModel.runRetestOneCmd(updatedModel.retestingName)()
	if calledName != "Slow" {
		t.Fatalf("expected retest-one callback to receive selected node, got %q", calledName)
	}

	updated, _ = updatedModel.Update(resultMsg{result: newSlow})
	updatedModel = updated.(tuiModel)
	updated, _ = updatedModel.Update(flushResultsMsg{})
	updatedModel = updated.(tuiModel)

	if len(updatedModel.results) != 2 {
		t.Fatalf("expected result count to stay the same, got %d", len(updatedModel.results))
	}
	if updatedModel.results[0] != newSlow {
		t.Fatalf("expected retested node to be resorted to first position")
	}
	if updatedModel.detailResult != newSlow {
		t.Fatalf("expected detail result to follow replacement")
	}
	if updatedModel.selectedIndex != 0 {
		t.Fatalf("expected selection to follow resorted node, got %d", updatedModel.selectedIndex)
	}
	if updatedModel.currentProxy != 0 {
		t.Fatalf("expected currentProxy to remain unchanged during single retest, got %d", updatedModel.currentProxy)
	}
}

func TestTUIModelIgnoresRetestWhileTesting(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel)
	called := false
	model.SetRetestCallbacks(
		func(chan<- *speedtester.Result) { called = true },
		func(string, chan<- *speedtester.Result) { called = true },
	)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if called {
		t.Fatalf("expected retest callbacks not to run while testing")
	}
	if cmd != nil {
		t.Fatalf("expected no command when retest is ignored")
	}
	if !updated.(tuiModel).testing {
		t.Fatalf("expected testing state to remain true")
	}
}
