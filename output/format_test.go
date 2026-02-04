package output

import (
	"testing"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

func TestGetHeaders(t *testing.T) {
	t.Run("fast mode", func(t *testing.T) {
		headers := GetHeaders(speedtester.SpeedModeFast)
		expected := []string{"序号", "节点名称", "类型", "延迟"}
		if len(headers) != len(expected) {
			t.Errorf("expected %d headers, got %d", len(expected), len(headers))
		}
		for i, h := range headers {
			if h != expected[i] {
				t.Errorf("header %d: expected %q, got %q", i, expected[i], h)
			}
		}
	})

	t.Run("download-only mode", func(t *testing.T) {
		headers := GetHeaders(speedtester.SpeedModeDownload)
		expected := []string{"序号", "节点名称", "类型", "延迟", "抖动", "丢包率", "下载速度"}
		if len(headers) != len(expected) {
			t.Errorf("expected %d headers, got %d", len(expected), len(headers))
		}
		for i, h := range headers {
			if h != expected[i] {
				t.Errorf("header %d: expected %q, got %q", i, expected[i], h)
			}
		}
	})

	t.Run("upload-enabled mode", func(t *testing.T) {
		headers := GetHeaders(speedtester.SpeedModeFull)
		expected := []string{"序号", "节点名称", "类型", "延迟", "抖动", "丢包率", "下载速度", "上传速度"}
		if len(headers) != len(expected) {
			t.Errorf("expected %d headers, got %d", len(expected), len(headers))
		}
		for i, h := range headers {
			if h != expected[i] {
				t.Errorf("header %d: expected %q, got %q", i, expected[i], h)
			}
		}
	})
}

func TestFormatRow(t *testing.T) {
	result := &speedtester.Result{
		ProxyName:     "Test Proxy",
		ProxyType:     "Trojan",
		Latency:       500 * time.Millisecond,
		Jitter:        100 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadSpeed: 10 * 1024 * 1024, // 10 MB/s
		UploadSpeed:   5 * 1024 * 1024,  // 5 MB/s
	}

	t.Run("fast mode", func(t *testing.T) {
		row := FormatRow(result, speedtester.SpeedModeFast, 0)
		if len(row) != 4 {
			t.Errorf("expected 4 columns in fast mode, got %d", len(row))
		}
		if row[0] != "1." {
			t.Errorf("expected index '1.', got %q", row[0])
		}
		if row[1] != "Test Proxy" {
			t.Errorf("expected proxy name 'Test Proxy', got %q", row[1])
		}
		if row[2] != "Trojan" {
			t.Errorf("expected proxy type 'Trojan', got %q", row[2])
		}
		if row[3] != "500ms" {
			t.Errorf("expected latency '500ms', got %q", row[3])
		}
	})

	t.Run("download-only mode", func(t *testing.T) {
		row := FormatRow(result, speedtester.SpeedModeDownload, 0)
		if len(row) != 7 {
			t.Errorf("expected 7 columns in download-only mode, got %d", len(row))
		}
		if row[0] != "1." {
			t.Errorf("expected index '1.', got %q", row[0])
		}
		if row[1] != "Test Proxy" {
			t.Errorf("expected proxy name 'Test Proxy', got %q", row[1])
		}
		if row[2] != "Trojan" {
			t.Errorf("expected proxy type 'Trojan', got %q", row[2])
		}
		if row[3] != "500ms" {
			t.Errorf("expected latency '500ms', got %q", row[3])
		}
		if row[4] != "100ms" {
			t.Errorf("expected jitter '100ms', got %q", row[4])
		}
		if row[5] != "5.0%" {
			t.Errorf("expected packet loss '5.0%%', got %q", row[5])
		}
		if row[6] != "10.00MB/s" {
			t.Errorf("expected download speed '10.00MB/s', got %q", row[6])
		}
	})

	t.Run("upload-enabled mode", func(t *testing.T) {
		row := FormatRow(result, speedtester.SpeedModeFull, 0)
		if len(row) != 8 {
			t.Errorf("expected 8 columns in upload-enabled mode, got %d", len(row))
		}
		if row[0] != "1." {
			t.Errorf("expected index '1.', got %q", row[0])
		}
		if row[1] != "Test Proxy" {
			t.Errorf("expected proxy name 'Test Proxy', got %q", row[1])
		}
		if row[2] != "Trojan" {
			t.Errorf("expected proxy type 'Trojan', got %q", row[2])
		}
		if row[3] != "500ms" {
			t.Errorf("expected latency '500ms', got %q", row[3])
		}
		if row[4] != "100ms" {
			t.Errorf("expected jitter '100ms', got %q", row[4])
		}
		if row[5] != "5.0%" {
			t.Errorf("expected packet loss '5.0%%', got %q", row[5])
		}
		if row[6] != "10.00MB/s" {
			t.Errorf("expected download speed '10.00MB/s', got %q", row[6])
		}
		if row[7] != "5.00MB/s" {
			t.Errorf("expected upload speed '5.00MB/s', got %q", row[7])
		}
	})

	t.Run("upload-enabled mode with errors", func(t *testing.T) {
		errorResult := &speedtester.Result{
			ProxyName:     "Error Proxy",
			ProxyType:     "Trojan",
			Latency:       100 * time.Millisecond,
			Jitter:        20 * time.Millisecond,
			PacketLoss:    5.0,
			DownloadError: "download failed: timeout",
			UploadError:   "upload failed: 500",
		}
		row := FormatRow(errorResult, speedtester.SpeedModeFull, 0)
		if row[6] != errorResult.DownloadError {
			t.Errorf("expected download error in speed cell, got %q", row[6])
		}
		if row[7] != errorResult.UploadError {
			t.Errorf("expected upload error in speed cell, got %q", row[7])
		}
	})

	t.Run("index increment", func(t *testing.T) {
		row1 := FormatRow(result, speedtester.SpeedModeFast, 0)
		row2 := FormatRow(result, speedtester.SpeedModeFast, 1)
		if row1[0] != "1." {
			t.Errorf("expected first row index '1.', got %q", row1[0])
		}
		if row2[0] != "2." {
			t.Errorf("expected second row index '2.', got %q", row2[0])
		}
	})
}

func TestSortResults(t *testing.T) {
	t.Run("fast mode - latency ascending", func(t *testing.T) {
		results := []*speedtester.Result{
			{Latency: 500 * time.Millisecond},
			{Latency: 100 * time.Millisecond},
			{Latency: 300 * time.Millisecond},
		}
		results = SortResults(results, speedtester.SpeedModeFast)
		if results[0].Latency != 100*time.Millisecond {
			t.Errorf("expected first result latency 100ms, got %v", results[0].Latency)
		}
		if results[1].Latency != 300*time.Millisecond {
			t.Errorf("expected second result latency 300ms, got %v", results[1].Latency)
		}
		if results[2].Latency != 500*time.Millisecond {
			t.Errorf("expected third result latency 500ms, got %v", results[2].Latency)
		}
	})

	t.Run("normal mode - download speed descending", func(t *testing.T) {
		results := []*speedtester.Result{
			{DownloadSpeed: 5 * 1024 * 1024},
			{DownloadSpeed: 20 * 1024 * 1024},
			{DownloadSpeed: 10 * 1024 * 1024},
		}
		results = SortResults(results, speedtester.SpeedModeDownload)
		if results[0].DownloadSpeed != 20*1024*1024 {
			t.Errorf("expected first result download speed 20MB/s, got %v", results[0].DownloadSpeed)
		}
		if results[1].DownloadSpeed != 10*1024*1024 {
			t.Errorf("expected second result download speed 10MB/s, got %v", results[1].DownloadSpeed)
		}
		if results[2].DownloadSpeed != 5*1024*1024 {
			t.Errorf("expected third result download speed 5MB/s, got %v", results[2].DownloadSpeed)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		results := []*speedtester.Result{}
		results = SortResults(results, speedtester.SpeedModeFast)
		if len(results) != 0 {
			t.Errorf("expected empty results, got %d items", len(results))
		}
	})

	t.Run("single result", func(t *testing.T) {
		results := []*speedtester.Result{
			{Latency: 500 * time.Millisecond},
		}
		results = SortResults(results, speedtester.SpeedModeFast)
		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
		if results[0].Latency != 500*time.Millisecond {
			t.Errorf("expected latency 500ms, got %v", results[0].Latency)
		}
	})

	t.Run("deduplicate by server and port", func(t *testing.T) {
		results := []*speedtester.Result{
			{
				Latency:     200 * time.Millisecond,
				ProxyConfig: map[string]any{"server": "1.1.1.1", "port": 443},
			},
			{
				Latency:     100 * time.Millisecond,
				ProxyConfig: map[string]any{"server": "1.1.1.1", "port": "443"},
			},
			{
				Latency:     150 * time.Millisecond,
				ProxyConfig: map[string]any{"server": "2.2.2.2", "port": float64(443)},
			},
		}
		results = SortResults(results, speedtester.SpeedModeFast)
		if len(results) != 2 {
			t.Errorf("expected 2 results after deduplication, got %d", len(results))
		}
		if results[0].Latency != 100*time.Millisecond {
			t.Errorf("expected lowest latency to remain after deduplication, got %v", results[0].Latency)
		}
		if results[1].ProxyConfig["server"] != "2.2.2.2" {
			t.Errorf("expected different server to remain, got %v", results[1].ProxyConfig["server"])
		}
	})

	t.Run("missing server or port is not deduplicated", func(t *testing.T) {
		results := []*speedtester.Result{
			{
				Latency:     100 * time.Millisecond,
				ProxyConfig: map[string]any{"server": "1.1.1.1"},
			},
			{
				Latency:     200 * time.Millisecond,
				ProxyConfig: map[string]any{"server": "1.1.1.1", "port": 80},
			},
			{
				Latency:     300 * time.Millisecond,
				ProxyConfig: map[string]any{"port": 80},
			},
		}
		results = SortResults(results, speedtester.SpeedModeFast)
		if len(results) != 3 {
			t.Errorf("expected results to remain when server or port missing, got %d", len(results))
		}
	})
}

func TestResultFormatting(t *testing.T) {
	// Integration test to verify formatting works end-to-end
	t.Run("complete flow", func(t *testing.T) {
		results := []*speedtester.Result{
			{
				ProxyName:     "Proxy A",
				ProxyType:     "Trojan",
				Latency:       200 * time.Millisecond,
				Jitter:        50 * time.Millisecond,
				PacketLoss:    2.5,
				DownloadSpeed: 15 * 1024 * 1024,
				UploadSpeed:   8 * 1024 * 1024,
			},
			{
				ProxyName:     "Proxy B",
				ProxyType:     "Vmess",
				Latency:       100 * time.Millisecond,
				Jitter:        30 * time.Millisecond,
				PacketLoss:    1.0,
				DownloadSpeed: 20 * 1024 * 1024,
				UploadSpeed:   10 * 1024 * 1024,
			},
		}

		// Test upload-enabled mode
		headers := GetHeaders(speedtester.SpeedModeFull)
		if len(headers) != 8 {
			t.Errorf("expected 8 headers in upload-enabled mode, got %d", len(headers))
		}

		results = SortResults(results, speedtester.SpeedModeFull)
		if results[0].ProxyName != "Proxy B" {
			t.Errorf("expected Proxy B first (higher download speed), got %s", results[0].ProxyName)
		}

		row := FormatRow(results[0], speedtester.SpeedModeFull, 0)
		if len(row) != 8 {
			t.Errorf("expected 8 columns, got %d", len(row))
		}
		if row[1] != "Proxy B" {
			t.Errorf("expected proxy name 'Proxy B', got %q", row[1])
		}

		// Test fast mode
		results = SortResults(results, speedtester.SpeedModeFast)
		if results[0].ProxyName != "Proxy B" {
			t.Errorf("expected Proxy B first (lower latency), got %s", results[0].ProxyName)
		}

		headers = GetHeaders(speedtester.SpeedModeFast)
		if len(headers) != 4 {
			t.Errorf("expected 4 headers in fast mode, got %d", len(headers))
		}

		row = FormatRow(results[0], speedtester.SpeedModeFast, 0)
		if len(row) != 4 {
			t.Errorf("expected 4 columns, got %d", len(row))
		}
		if row[1] != "Proxy B" {
			t.Errorf("expected proxy name 'Proxy B', got %q", row[1])
		}
	})
}
