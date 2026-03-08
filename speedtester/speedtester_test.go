package speedtester

import (
	"testing"
	"time"
)

func TestTransferSummaryAdd(t *testing.T) {
	summary := newTransferSummary()
	summary.add(nil)

	errorMessage := "download request to https://example.com/__down?bytes=1 failed: boom"
	summary.add(&downloadResult{error: errorMessage})
	if summary.successCount != 0 {
		t.Fatalf("expected successCount to remain 0, got %d", summary.successCount)
	}
	if len(summary.errors) != 1 {
		t.Fatalf("expected 1 error message, got %d", len(summary.errors))
	}
	if summary.errors[0] != errorMessage {
		t.Fatalf("expected error message %q, got %q", errorMessage, summary.errors[0])
	}

	summary.add(&downloadResult{error: errorMessage})
	if len(summary.errors) != 1 {
		t.Fatalf("expected duplicate errors to be deduplicated, got %d", len(summary.errors))
	}

	summary.add(&downloadResult{bytes: 100, duration: time.Second})
	summary.add(&downloadResult{bytes: 50, duration: 2 * time.Second})

	if summary.successCount != 2 {
		t.Fatalf("expected successCount to be 2, got %d", summary.successCount)
	}
	if summary.totalBytes != 150 {
		t.Fatalf("expected totalBytes to be 150, got %d", summary.totalBytes)
	}
	if summary.totalDuration != 3*time.Second {
		t.Fatalf("expected totalDuration to be 3s, got %v", summary.totalDuration)
	}
	if summary.averageDuration() != 1500*time.Millisecond {
		t.Fatalf("expected averageDuration to be 1.5s, got %v", summary.averageDuration())
	}
}

func TestResultFormatErrors(t *testing.T) {
	result := &Result{}
	if result.FormatDownloadError() != "N/A" {
		t.Fatalf("expected empty download error to format as N/A, got %q", result.FormatDownloadError())
	}
	if result.FormatUploadError() != "N/A" {
		t.Fatalf("expected empty upload error to format as N/A, got %q", result.FormatUploadError())
	}

	result.DownloadError = "download failed: timeout"
	result.UploadError = "upload failed: status 500"
	if result.FormatDownloadError() != result.DownloadError {
		t.Fatalf("expected download error to pass through, got %q", result.FormatDownloadError())
	}
	if result.FormatUploadError() != result.UploadError {
		t.Fatalf("expected upload error to pass through, got %q", result.FormatUploadError())
	}

	result.DownloadSpeed = 1024
	result.UploadSpeed = 2048
	if result.FormatDownloadSpeed() != result.DownloadError {
		t.Fatalf("expected download speed to prefer error string, got %q", result.FormatDownloadSpeed())
	}
	if result.FormatUploadSpeed() != result.UploadError {
		t.Fatalf("expected upload speed to prefer error string, got %q", result.FormatUploadSpeed())
	}
	if result.FormatDownloadSpeedValue() == result.DownloadError {
		t.Fatalf("expected download speed value to ignore error string")
	}
	if result.FormatUploadSpeedValue() == result.UploadError {
		t.Fatalf("expected upload speed value to ignore error string")
	}
}

func TestDeduplicateProxies(t *testing.T) {
	t.Run("deduplicate by server and port", func(t *testing.T) {
		proxies := map[string]*CProxy{
			"proxy-a": {Config: map[string]any{"server": "1.1.1.1", "port": 443}},
			"proxy-b": {Config: map[string]any{"server": "1.1.1.1", "port": "443"}},
			"proxy-c": {Config: map[string]any{"server": "2.2.2.2", "port": float64(443)}},
		}

		results := deduplicateProxiesByServerPort(proxies)
		if len(results) != 2 {
			t.Fatalf("expected 2 proxies after deduplication, got %d", len(results))
		}
		if _, ok := results["proxy-c"]; !ok {
			t.Fatalf("expected proxy-c to remain")
		}
		duplicateCount := 0
		if _, ok := results["proxy-a"]; ok {
			duplicateCount++
		}
		if _, ok := results["proxy-b"]; ok {
			duplicateCount++
		}
		if duplicateCount != 1 {
			t.Fatalf("expected exactly one duplicate proxy to remain, got %d", duplicateCount)
		}
	})

	t.Run("mapped ipv6 and ipv4 are deduplicated after normalization", func(t *testing.T) {
		proxies := map[string]*CProxy{
			"proxy-a": {Config: map[string]any{"server": convertMappedIPv6ToIPv4("::ffff:1.1.1.1"), "port": 443}},
			"proxy-b": {Config: map[string]any{"server": "1.1.1.1", "port": 443}},
		}

		results := deduplicateProxiesByServerPort(proxies)
		if len(results) != 1 {
			t.Fatalf("expected 1 proxy after deduplication, got %d", len(results))
		}
	})

	t.Run("missing server or port is not deduplicated", func(t *testing.T) {
		proxies := map[string]*CProxy{
			"proxy-a": {Config: map[string]any{"server": "1.1.1.1"}},
			"proxy-b": {Config: map[string]any{"server": "1.1.1.1", "port": 80}},
			"proxy-c": {Config: map[string]any{"port": 80}},
		}

		results := deduplicateProxiesByServerPort(proxies)
		if len(results) != 3 {
			t.Fatalf("expected proxies to remain when server or port missing, got %d", len(results))
		}
	})
}
