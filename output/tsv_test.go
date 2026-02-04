package output

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

func TestNewTSVWriter(t *testing.T) {
	tests := []struct {
		name           string
		mode           speedtester.SpeedMode
		expectedHeader string
	}{
		{
			name:           "fast mode header",
			mode:           speedtester.SpeedModeFast,
			expectedHeader: "序号\t节点名称\t类型\t延迟\n",
		},
		{
			name:           "download-only mode header",
			mode:           speedtester.SpeedModeDownload,
			expectedHeader: "序号\t节点名称\t类型\t延迟\t抖动\t丢包率\t下载速度\n",
		},
		{
			name:           "upload-enabled mode header",
			mode:           speedtester.SpeedModeFull,
			expectedHeader: "序号\t节点名称\t类型\t延迟\t抖动\t丢包率\t下载速度\t上传速度\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			writer, err := NewTSVWriter(&output, tt.mode)

			if err != nil {
				t.Fatalf("NewTSVWriter failed: %v", err)
			}
			if writer == nil {
				t.Fatal("NewTSVWriter returned nil")
			}

			result := output.String()
			if result != tt.expectedHeader {
				t.Errorf("Expected header %q, got %q", tt.expectedHeader, result)
			}
		})
	}
}

func TestTSVWriter_WriteRow(t *testing.T) {
	tests := []struct {
		name        string
		mode        speedtester.SpeedMode
		result      *speedtester.Result
		index       int
		expectedRow string
	}{
		{
			name: "fast mode row",
			mode: speedtester.SpeedModeFast,
			result: &speedtester.Result{
				ProxyName: "Test Proxy",
				ProxyType: "Trojan",
				Latency:   500 * time.Millisecond,
			},
			index:       0,
			expectedRow: "1.\tTest Proxy\tTrojan\t500ms\n",
		},
		{
			name: "download-only mode row",
			mode: speedtester.SpeedModeDownload,
			result: &speedtester.Result{
				ProxyName:     "Test Proxy",
				ProxyType:     "Trojan",
				Latency:       500 * time.Millisecond,
				Jitter:        50 * time.Millisecond,
				PacketLoss:    5.0,
				DownloadSpeed: 10 * 1024 * 1024,
				UploadSpeed:   5 * 1024 * 1024,
			},
			index:       1,
			expectedRow: "2.\tTest Proxy\tTrojan\t500ms\t50ms\t5.0%\t10.00MB/s\n",
		},
		{
			name: "row with N/A values",
			mode: speedtester.SpeedModeDownload,
			result: &speedtester.Result{
				ProxyName:     "Failed Proxy",
				ProxyType:     "Shadowsocks",
				Latency:       0,
				Jitter:        0,
				PacketLoss:    100.0,
				DownloadSpeed: 0,
				UploadSpeed:   0,
			},
			index:       2,
			expectedRow: "3.\tFailed Proxy\tShadowsocks\tN/A\tN/A\t100.0%\t0.00B/s\n",
		},
		{
			name: "upload-enabled row with errors",
			mode: speedtester.SpeedModeFull,
			result: &speedtester.Result{
				ProxyName:     "Error Proxy",
				ProxyType:     "Vmess",
				Latency:       300 * time.Millisecond,
				Jitter:        10 * time.Millisecond,
				PacketLoss:    2.0,
				DownloadError: "download failed: timeout",
				UploadError:   "upload failed: 500",
			},
			index:       3,
			expectedRow: "4.\tError Proxy\tVmess\t300ms\t10ms\t2.0%\tdownload failed: timeout\tupload failed: 500\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			writer, err := NewTSVWriter(&output, tt.mode)
			if err != nil {
				t.Fatalf("NewTSVWriter failed: %v", err)
			}

			// Clear the header from output
			output.Reset()

			err = writer.WriteRow(tt.result, tt.index)
			if err != nil {
				t.Fatalf("WriteRow failed: %v", err)
			}

			result := output.String()
			if result != tt.expectedRow {
				t.Errorf("Expected row %q, got %q", tt.expectedRow, result)
			}
		})
	}
}

func TestTSVWriter_WriteRows(t *testing.T) {
	tests := []struct {
		name         string
		mode         speedtester.SpeedMode
		results      []*speedtester.Result
		expectedRows string
	}{
		{
			name: "fast mode multiple rows",
			mode: speedtester.SpeedModeFast,
			results: []*speedtester.Result{
				{
					ProxyName: "Proxy 1",
					ProxyType: "Trojan",
					Latency:   100 * time.Millisecond,
				},
				{
					ProxyName: "Proxy 2",
					ProxyType: "Shadowsocks",
					Latency:   200 * time.Millisecond,
				},
			},
			expectedRows: "1.\tProxy 1\tTrojan\t100ms\n2.\tProxy 2\tShadowsocks\t200ms\n",
		},
		{
			name: "upload-enabled mode multiple rows",
			mode: speedtester.SpeedModeFull,
			results: []*speedtester.Result{
				{
					ProxyName:     "Proxy 1",
					ProxyType:     "Trojan",
					Latency:       100 * time.Millisecond,
					Jitter:        10 * time.Millisecond,
					PacketLoss:    0.0,
					DownloadSpeed: 20 * 1024 * 1024,
					UploadSpeed:   10 * 1024 * 1024,
				},
				{
					ProxyName:     "Proxy 2",
					ProxyType:     "Shadowsocks",
					Latency:       200 * time.Millisecond,
					Jitter:        20 * time.Millisecond,
					PacketLoss:    5.0,
					DownloadSpeed: 15 * 1024 * 1024,
					UploadSpeed:   8 * 1024 * 1024,
				},
			},
			expectedRows: "1.\tProxy 1\tTrojan\t100ms\t10ms\t0.0%\t20.00MB/s\t10.00MB/s\n2.\tProxy 2\tShadowsocks\t200ms\t20ms\t5.0%\t15.00MB/s\t8.00MB/s\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output strings.Builder
			writer, err := NewTSVWriter(&output, tt.mode)
			if err != nil {
				t.Fatalf("NewTSVWriter failed: %v", err)
			}

			// Clear the header from output
			output.Reset()

			err = writer.WriteRows(tt.results)
			if err != nil {
				t.Fatalf("WriteRows failed: %v", err)
			}

			result := output.String()
			if result != tt.expectedRows {
				t.Errorf("Expected rows %q, got %q", tt.expectedRows, result)
			}
		})
	}
}

func TestTSVWriter_NoANSIColors(t *testing.T) {
	// Ensure TSV output does not contain ANSI color codes
	var output strings.Builder
	writer, err := NewTSVWriter(&output, speedtester.SpeedModeFull)
	if err != nil {
		t.Fatalf("NewTSVWriter failed: %v", err)
	}

	result := &speedtester.Result{
		ProxyName:     "Test Proxy",
		ProxyType:     "Trojan",
		Latency:       500 * time.Millisecond,
		Jitter:        50 * time.Millisecond,
		PacketLoss:    5.0,
		DownloadSpeed: 10 * 1024 * 1024,
		UploadSpeed:   5 * 1024 * 1024,
	}

	output.Reset()
	err = writer.WriteRow(result, 0)
	if err != nil {
		t.Fatalf("WriteRow failed: %v", err)
	}

	outputStr := output.String()
	// Check for ANSI escape sequences
	if strings.Contains(outputStr, "\033[") || strings.Contains(outputStr, "\x1b[") {
		t.Errorf("TSV output should not contain ANSI color codes, got: %q", outputStr)
	}
}

func TestTSVWriter_HeaderWrittenOnce(t *testing.T) {
	var output strings.Builder
	writer, err := NewTSVWriter(&output, speedtester.SpeedModeFast)
	if err != nil {
		t.Fatalf("NewTSVWriter failed: %v", err)
	}

	// Write header is called in NewTSVWriter
	initialOutput := output.String()

	// Call writeHeader again
	writer.writeHeader()

	// Output should not change
	if output.String() != initialOutput {
		t.Errorf("Header should only be written once, expected %q, got %q", initialOutput, output.String())
	}
}

func TestTSVWriter_ErrorContext(t *testing.T) {
	var output strings.Builder
	writer, err := NewTSVWriter(&output, speedtester.SpeedModeFast)
	if err != nil {
		t.Fatalf("NewTSVWriter failed: %v", err)
	}

	err = writer.WriteRow(nil, 0)
	if err == nil {
		t.Fatal("WriteRow should return error for nil result")
	}
	expectedErrMsg := "cannot write nil result"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}

	result := &speedtester.Result{
		ProxyName: "Test Proxy",
		ProxyType: "Trojan",
		Latency:   100 * time.Millisecond,
	}

	failWriter := &TSVWriter{
		output: &errorWriter{},
		mode:   speedtester.SpeedModeFast,
	}
	err = failWriter.WriteRow(result, 0)
	if err == nil {
		t.Fatal("WriteRow should return error for write failure")
	}
	if !strings.Contains(err.Error(), "write row for proxy") {
		t.Errorf("Error should contain proxy name context, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Test Proxy") {
		t.Errorf("Error should contain proxy name 'Test Proxy', got: %v", err)
	}
}

type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("mock write error")
}
