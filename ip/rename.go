package ip

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

var countryFlags = map[string]string{
	"US": "🇺🇸", "CN": "🇨🇳", "GB": "🇬🇧", "UK": "🇬🇧", "JP": "🇯🇵", "DE": "🇩🇪", "FR": "🇫🇷", "RU": "🇷🇺",
	"SG": "🇸🇬", "HK": "🇭🇰", "TW": "🇨🇳", "KR": "🇰🇷", "CA": "🇨🇦", "AU": "🇦🇺", "NL": "🇳🇱", "IT": "🇮🇹",
	"ES": "🇪🇸", "SE": "🇸🇪", "NO": "🇳🇴", "DK": "🇩🇰", "FI": "🇫🇮", "CH": "🇨🇭", "AT": "🇦🇹", "BE": "🇧🇪",
	"BR": "🇧🇷", "IN": "🇮🇳", "TH": "🇹🇭", "MY": "🇲🇾", "VN": "🇻🇳", "PH": "🇵🇭", "ID": "🇮🇩", "UA": "🇺🇦",
	"TR": "🇹🇷", "IL": "🇮🇱", "AE": "🇦🇪", "SA": "🇸🇦", "EG": "🇪🇬", "ZA": "🇿🇦", "NG": "🇳🇬", "KE": "🇰🇪",
	"RO": "🇷🇴", "PL": "🇵🇱", "CZ": "🇨🇿", "HU": "🇭🇺", "BG": "🇧🇬", "HR": "🇭🇷", "SI": "🇸🇮", "SK": "🇸🇰",
	"LT": "🇱🇹", "LV": "🇱🇻", "EE": "🇪🇪", "PT": "🇵🇹", "GR": "🇬🇷", "IE": "🇮🇪", "LU": "🇱🇺", "MT": "🇲🇹",
	"CY": "🇨🇾", "IS": "🇮🇸", "MX": "🇲🇽", "AR": "🇦🇷", "CL": "🇨🇱", "CO": "🇨🇴", "PE": "🇵🇪", "VE": "🇻🇪",
	"EC": "🇪🇨", "UY": "🇺🇾", "PY": "🇵🇾", "BO": "🇧🇴", "CR": "🇨🇷", "PA": "🇵🇦", "GT": "🇬🇹", "HN": "🇭🇳",
	"SV": "🇸🇻", "NI": "🇳🇮", "BZ": "🇧🇿", "JM": "🇯🇲", "TT": "🇹🇹", "BB": "🇧🇧", "GD": "🇬🇩", "LC": "🇱🇨",
	"VC": "🇻🇨", "AG": "🇦🇬", "DM": "🇩🇲", "KN": "🇰🇳", "BS": "🇧🇸", "CU": "🇨🇺", "DO": "🇩🇴", "HT": "🇭🇹",
	"PR": "🇵🇷", "VI": "🇻🇮", "GU": "🇬🇺", "AS": "🇦🇸", "MP": "🇲🇵", "PW": "🇵🇼", "FM": "🇫🇲", "MH": "🇲🇭",
	"KI": "🇰🇮", "TV": "🇹🇻", "NR": "🇳🇷", "WS": "🇼🇸", "TO": "🇹🇴", "FJ": "🇫🇯", "VU": "🇻🇺", "SB": "🇸🇧",
	"PG": "🇵🇬", "NC": "🇳🇨", "PF": "🇵🇫", "WF": "🇼🇫", "CK": "🇨🇰", "NU": "🇳🇺", "TK": "🇹🇰", "SC": "🇸🇨",
}

// DefaultNameTemplate is the built-in format when -rename-template is not set.
const DefaultNameTemplate = `{{.Flag}} {{.CountryCode}} {{.Index}} | {{.Direction}} {{.Speed}}MB/s`

// NodeNameData is the data passed to the rename template.
type NodeNameData struct {
	Flag               string // country flag emoji
	CountryCode        string // e.g. US, HK
	Index              string // padded number, e.g. 001
	Direction          string // ⬇️ or ⬆️
	Speed              string // main speed in MB/s (e.g. 12.34)
	DownloadSpeedMBps  string // download MB/s
	UploadSpeedMBps    string // upload MB/s
}

// GenerateNodeNameFromTemplate renders name from a text/template. Placeholders:
// {{.Flag}}, {{.CountryCode}}, {{.Index}}, {{.Direction}}, {{.Speed}}, {{.DownloadSpeedMBps}}, {{.UploadSpeedMBps}}.
// If template is empty, DefaultNameTemplate is used. On execute error, falls back to default format.
func GenerateNodeNameFromTemplate(tmpl string, countryCode string, downloadSpeed, uploadSpeed float64, nameCount map[string]int) (string, error) {
	if tmpl == "" {
		tmpl = DefaultNameTemplate
	}
	t, err := template.New("name").Parse(tmpl)
	if err != nil {
		return "", err
	}
	data := buildNodeNameData(countryCode, downloadSpeed, uploadSpeed, nameCount)
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		// fallback to default format so caller does not double-increment nameCount
		return fmt.Sprintf("%s %s %s | %s %sMB/s", data.Flag, data.CountryCode, data.Index, data.Direction, data.Speed), nil
	}
	return buf.String(), nil
}

func buildNodeNameData(countryCode string, downloadSpeed, uploadSpeed float64, nameCount map[string]int) NodeNameData {
	flag, exists := countryFlags[strings.ToUpper(countryCode)]
	if !exists {
		flag = "🏳️"
	}
	upperCountryCode := strings.ToUpper(countryCode)
	speed := downloadSpeed
	direction := "⬇️"
	if downloadSpeed <= 0 {
		speed = uploadSpeed
		direction = "⬆️"
	}
	speedMBps := speed / (1024 * 1024)
	count := nameCount[upperCountryCode] + 1
	nameCount[upperCountryCode] = count
	dlMBps := downloadSpeed / (1024 * 1024)
	ulMBps := uploadSpeed / (1024 * 1024)
	return NodeNameData{
		Flag:              flag,
		CountryCode:       upperCountryCode,
		Index:             fmt.Sprintf("%03d", count),
		Direction:         direction,
		Speed:             fmt.Sprintf("%.2f", speedMBps),
		DownloadSpeedMBps: fmt.Sprintf("%.2f", dlMBps),
		UploadSpeedMBps:   fmt.Sprintf("%.2f", ulMBps),
	}
}

func GenerateNodeName(countryCode string, downloadSpeed float64, uploadSpeed float64, nameCount map[string]int) string {
	name, _ := GenerateNodeNameFromTemplate("", countryCode, downloadSpeed, uploadSpeed, nameCount)
	return name
}
