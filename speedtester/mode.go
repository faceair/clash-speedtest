package speedtester

import (
	"fmt"
	"strings"
)

type SpeedMode string

const (
	SpeedModeFast     SpeedMode = "fast"
	SpeedModeDownload SpeedMode = "download"
	SpeedModeFull     SpeedMode = "full"
)

func ParseSpeedMode(value string) (SpeedMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch SpeedMode(normalized) {
	case SpeedModeFast:
		return SpeedModeFast, nil
	case SpeedModeDownload:
		return SpeedModeDownload, nil
	case SpeedModeFull:
		return SpeedModeFull, nil
	default:
		return "", fmt.Errorf("unsupported speed mode %q", value)
	}
}

func (m SpeedMode) IsFast() bool {
	return m == SpeedModeFast
}

func (m SpeedMode) UploadEnabled() bool {
	return m == SpeedModeFull
}
