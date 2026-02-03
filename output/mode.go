package output

import (
	"os"

	"golang.org/x/term"
)

// OutputMode represents the output format mode
type OutputMode int

const (
	// OutputModeTSV outputs tab-separated values without ANSI colors
	OutputModeTSV OutputMode = iota
	// OutputModeInteractive outputs interactive UI (currently tablewriter, future TUI)
	OutputModeInteractive
)

// IsTerminal is a function type for terminal detection to enable testability
type IsTerminal func(file *os.File) bool

// DetermineOutputMode determines the output mode based on terminal detection.
// Returns TSV mode when stdout is not a terminal (for example, piped output).
// Otherwise returns Interactive mode. File output is handled separately by the caller.
func DetermineOutputMode(isTerminal IsTerminal) OutputMode {
	if !isTerminal(os.Stdout) {
		return OutputModeTSV
	}
	return OutputModeInteractive
}

// IsTerminalFile checks if the given file is a terminal
// This is the default implementation used in production
func IsTerminalFile(file *os.File) bool {
	return term.IsTerminal(int(file.Fd()))
}
