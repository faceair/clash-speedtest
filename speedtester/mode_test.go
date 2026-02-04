package speedtester

import "testing"

func TestParseSpeedMode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  SpeedMode
		wantError bool
	}{
		{
			name:     "fast",
			input:    "fast",
			expected: SpeedModeFast,
		},
		{
			name:     "download",
			input:    "download",
			expected: SpeedModeDownload,
		},
		{
			name:     "full",
			input:    "full",
			expected: SpeedModeFull,
		},
		{
			name:      "invalid",
			input:     "slow",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, err := ParseSpeedMode(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpeedMode(%q) failed: %v", tt.input, err)
			}
			if mode != tt.expected {
				t.Fatalf("expected mode %s, got %s", tt.expected, mode)
			}
		})
	}
}

func TestSpeedModeHelpers(t *testing.T) {
	if !SpeedModeFast.IsFast() {
		t.Fatalf("expected fast mode to be fast")
	}
	if SpeedModeDownload.IsFast() {
		t.Fatalf("expected download mode to be non-fast")
	}
	if SpeedModeFull.IsFast() {
		t.Fatalf("expected full mode to be non-fast")
	}

	if SpeedModeFast.UploadEnabled() {
		t.Fatalf("expected fast mode to disable upload")
	}
	if SpeedModeDownload.UploadEnabled() {
		t.Fatalf("expected download mode to disable upload")
	}
	if !SpeedModeFull.UploadEnabled() {
		t.Fatalf("expected full mode to enable upload")
	}
}
