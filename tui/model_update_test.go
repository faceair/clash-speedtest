package tui

import (
	"testing"
	"time"

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

	// Verify results are sorted by download speed (descending)
	// result1 (15 MB/s) should be first, result2 (8 MB/s) should be second
	if updatedModel.(tuiModel).results[0] != result1 {
		t.Error("Expected first result to be result1")
	}
	if updatedModel.(tuiModel).results[1] != result2 {
		t.Error("Expected second result to be result2")
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
