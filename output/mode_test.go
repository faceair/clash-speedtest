package output

import (
	"os"
	"testing"
)

func TestDetermineOutputMode(t *testing.T) {
	tests := []struct {
		name        string
		isTerminal  IsTerminal
		expected    OutputMode
		description string
	}{
		{
			name:        "non-terminal stdout returns TSV",
			isTerminal:  func(*os.File) bool { return false },
			expected:    OutputModeTSV,
			description: "When stdout is not a terminal (e.g., piped), should use TSV mode",
		},
		{
			name:        "terminal stdout returns Interactive",
			isTerminal:  func(*os.File) bool { return true },
			expected:    OutputModeInteractive,
			description: "When stdout is a terminal, should use Interactive mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineOutputMode(tt.isTerminal)
			if result != tt.expected {
				t.Errorf("%s: DetermineOutputMode() = %v, want %v", tt.description, result, tt.expected)
			}
		})
	}
}

func TestIsTerminalFile(t *testing.T) {
	// Test with stdout (may or may not be a terminal depending on test environment)
	// This test mainly ensures the function doesn't panic
	_ = IsTerminalFile(os.Stdout)
	_ = IsTerminalFile(os.Stderr)
	_ = IsTerminalFile(os.Stdin)
}

func TestOutputModeValues(t *testing.T) {
	// Ensure OutputMode enum values are as expected
	if OutputModeTSV != 0 {
		t.Errorf("OutputModeTSV should be 0, got %d", OutputModeTSV)
	}
	if OutputModeInteractive != 1 {
		t.Errorf("OutputModeInteractive should be 1, got %d", OutputModeInteractive)
	}
}
