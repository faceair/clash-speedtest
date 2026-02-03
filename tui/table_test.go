package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

// TestTUIModelColorizeRow tests the colorizeRow function
func TestTUIModelColorizeRow(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(false, 1, resultChannel)

	// Test latency coloring
	t.Run("LatencyGreen", func(t *testing.T) {
		result := &speedtester.Result{
			Latency: 500 * time.Millisecond,
		}
		row := []string{"1.", "Proxy", "SS", "500ms", "N/A", "0.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify latency contains the original value
		if !strings.Contains(coloredRow[3], "500ms") {
			t.Errorf("Expected latency to contain '500ms', got %s", coloredRow[3])
		}
	})

	t.Run("LatencyYellow", func(t *testing.T) {
		result := &speedtester.Result{
			Latency: 1000 * time.Millisecond,
		}
		row := []string{"1.", "Proxy", "SS", "1000ms", "N/A", "0.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify latency contains the original value
		if !strings.Contains(coloredRow[3], "1000ms") {
			t.Errorf("Expected latency to contain '1000ms', got %s", coloredRow[3])
		}
	})

	t.Run("LatencyRed", func(t *testing.T) {
		result := &speedtester.Result{
			Latency: 2000 * time.Millisecond,
		}
		row := []string{"1.", "Proxy", "SS", "2000ms", "N/A", "0.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify latency contains the original value
		if !strings.Contains(coloredRow[3], "2000ms") {
			t.Errorf("Expected latency to contain '2000ms', got %s", coloredRow[3])
		}
	})

	// Test packet loss coloring
	t.Run("PacketLossGreen", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:    100 * time.Millisecond,
			Jitter:     50 * time.Millisecond,
			PacketLoss: 5.0,
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify packet loss contains the original value
		if !strings.Contains(coloredRow[5], "5.0%") {
			t.Errorf("Expected packet loss to contain '5.0%%', got %s", coloredRow[5])
		}
	})

	t.Run("PacketLossYellow", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:    100 * time.Millisecond,
			Jitter:     50 * time.Millisecond,
			PacketLoss: 15.0,
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "15.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify packet loss contains the original value
		if !strings.Contains(coloredRow[5], "15.0%") {
			t.Errorf("Expected packet loss to contain '15.0%%', got %s", coloredRow[5])
		}
	})

	t.Run("PacketLossRed", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:    100 * time.Millisecond,
			Jitter:     50 * time.Millisecond,
			PacketLoss: 25.0,
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "25.0%", "10.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify packet loss contains the original value
		if !strings.Contains(coloredRow[5], "25.0%") {
			t.Errorf("Expected packet loss to contain '25.0%%', got %s", coloredRow[5])
		}
	})

	// Test download speed coloring
	t.Run("DownloadSpeedGreen", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 15 * 1024 * 1024, // 15 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "15.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify download speed contains the original value
		if !strings.Contains(coloredRow[6], "15.00MB/s") {
			t.Errorf("Expected download speed to contain '15.00MB/s', got %s", coloredRow[6])
		}
	})

	t.Run("DownloadSpeedYellow", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 7 * 1024 * 1024, // 7 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "7.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify download speed contains the original value
		if !strings.Contains(coloredRow[6], "7.00MB/s") {
			t.Errorf("Expected download speed to contain '7.00MB/s', got %s", coloredRow[6])
		}
	})

	t.Run("DownloadSpeedRed", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 3 * 1024 * 1024, // 3 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "3.00MB/s", "5.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify download speed contains the original value
		if !strings.Contains(coloredRow[6], "3.00MB/s") {
			t.Errorf("Expected download speed to contain '3.00MB/s', got %s", coloredRow[6])
		}
	})

	// Test upload speed coloring
	t.Run("UploadSpeedGreen", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 10 * 1024 * 1024,
			UploadSpeed:   8 * 1024 * 1024, // 8 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "10.00MB/s", "8.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify upload speed contains the original value
		if !strings.Contains(coloredRow[7], "8.00MB/s") {
			t.Errorf("Expected upload speed to contain '8.00MB/s', got %s", coloredRow[7])
		}
	})

	t.Run("UploadSpeedYellow", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 10 * 1024 * 1024,
			UploadSpeed:   3 * 1024 * 1024, // 3 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "10.00MB/s", "3.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify upload speed contains the original value
		if !strings.Contains(coloredRow[7], "3.00MB/s") {
			t.Errorf("Expected upload speed to contain '3.00MB/s', got %s", coloredRow[7])
		}
	})

	t.Run("UploadSpeedRed", func(t *testing.T) {
		result := &speedtester.Result{
			Latency:       100 * time.Millisecond,
			Jitter:        50 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadSpeed: 10 * 1024 * 1024,
			UploadSpeed:   1 * 1024 * 1024, // 1 MB/s
		}
		row := []string{"1.", "Proxy", "SS", "100ms", "50ms", "5.0%", "10.00MB/s", "1.00MB/s"}
		coloredRow := model.colorizeRow(row, result)
		// Verify upload speed contains the original value
		if !strings.Contains(coloredRow[7], "1.00MB/s") {
			t.Errorf("Expected upload speed to contain '1.00MB/s', got %s", coloredRow[7])
		}
	})
}

// TestTUIModelUpdateTableRows tests the updateTableRows function
func TestTUIModelUpdateTableRows(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(false, 2, resultChannel)

	// Add a result
	result := &speedtester.Result{
		ProxyName:     "Test Proxy",
		ProxyType:     "SS",
		Latency:       100 * time.Millisecond,
		Jitter:        50 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadSpeed: 10 * 1024 * 1024,
		UploadSpeed:   5 * 1024 * 1024,
		ProxyConfig:   map[string]any{},
	}

	model.results = append(model.results, result)
	model.updateTableRows()

	// Verify table has one row
	rows := model.table.Rows()
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 8 {
		t.Errorf("Expected 8 columns in normal mode, got %d", len(rows[0]))
	}
}

// TestTUIModelUpdateTableRowsFastMode tests the updateTableRows function in fast mode
func TestTUIModelUpdateTableRowsFastMode(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 10)
	model := NewTUIModel(true, 2, resultChannel)

	// Add a result
	result := &speedtester.Result{
		ProxyName:   "Test Proxy",
		ProxyType:   "SS",
		Latency:     100 * time.Millisecond,
		ProxyConfig: map[string]any{},
	}

	model.results = append(model.results, result)
	model.updateTableRows()

	// Verify table has one row
	rows := model.table.Rows()
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != 4 {
		t.Errorf("Expected 4 columns in fast mode, got %d", len(rows[0]))
	}
}

func TestCalculateColumnWidthsFitsWindow(t *testing.T) {
	width := 100
	widths := calculateColumnWidths(width, false)
	if len(widths) != 8 {
		t.Fatalf("expected 8 columns, got %d", len(widths))
	}
	total := 0
	for _, value := range widths {
		total += value
	}
	padding := 2 * len(widths)
	if total+padding > width {
		t.Fatalf("expected total width to fit window: columns=%d padding=%d window=%d", total, padding, width)
	}
}
