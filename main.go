package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"github.com/faceair/clash-speedtest/speedtester"
	"github.com/metacubex/mihomo/log"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
)

var (
    configPathsConfig   = flag.String("c", "", "config file path, also support http(s) url")
    filterRegexConfig   = flag.String("f", ".+", "filter proxies by name, use regexp")
    blockKeywords       = flag.String("b", "", "block proxies by keywords, use | to separate multiple keywords (example: -b 'rate|x1|1x')")
    serverURL           = flag.String("server-url", "https://speed.cloudflare.com", "server url")
    downloadSize        = flag.Int("download-size", 50*1024*1024, "download size for testing proxies")
    uploadSize          = flag.Int("upload-size", 20*1024*1024, "upload size for testing proxies")
    timeout             = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
    concurrent          = flag.Int("concurrent", 4, "download concurrent size")
    outputPath          = flag.String("output", "", "output config file path")
    otherOutputPath     = flag.String("other-output", "other.yaml", "output config file path for other proxies")
    stashCompatible     = flag.Bool("stash-compatible", false, "enable stash compatible mode")
    maxLatency          = flag.Duration("max-latency", 800*time.Millisecond, "filter latency greater than this value")
    minDownloadSpeed    = flag.Float64("min-download-speed", 0, "filter download speed less than this value(unit: MB/s)") // 修改默认值为0
    minUploadSpeed      = flag.Float64("min-upload-speed", 0, "filter upload speed less than this value(unit: MB/s)")   // 修改默认值为0
    renameNodes         = flag.Bool("rename", false, "rename nodes with IP location and speed")
    fastMode            = flag.Bool("fast", false, "fast mode, only test latency")
)


const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func main() {
	flag.Parse()
	log.SetLevel(log.SILENT)
	if *configPathsConfig == "" {
		log.Fatalln("please specify the configuration file")
	}
	speedTester := speedtester.New(&speedtester.Config{
		ConfigPaths:      *configPathsConfig,
		FilterRegex:      *filterRegexConfig,
		BlockRegex:       *blockKeywords,
		ServerURL:        *serverURL,
		DownloadSize:     *downloadSize,
		UploadSize:       *uploadSize,
		Timeout:          *timeout,
		Concurrent:       *concurrent,
		MaxLatency:       *maxLatency,
		MinDownloadSpeed: *minDownloadSpeed * 1024 * 1024,
		MinUploadSpeed:   *minUploadSpeed * 1024 * 1024,
		FastMode:         *fastMode,
	})
	allProxies, err := speedTester.LoadProxies(*stashCompatible)
	if err != nil {
		log.Fatalln("load proxies failed: %v", err)
	}
	bar := progressbar.Default(int64(len(allProxies)), "测试中...")
	results := make([]*speedtester.Result, 0)
	speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
		bar.Add(1)
		bar.Describe(result.ProxyName)
		results = append(results, result)
	})
	sort.Slice(results, func(i, j int) bool {
		return results[i].DownloadSpeed > results[j].DownloadSpeed
	})
	printResults(results)
	if *outputPath != "" {
		// 分类节点
		var filteredProxies, otherProxies []map[string]any
		for _, result := range results {
			if (*maxLatency > 0 && result.Latency > *maxLatency) ||
			   (*downloadSize > 0 && *minDownloadSpeed > 0 && result.DownloadSpeed < *minDownloadSpeed*1024*1024) ||
			   (*uploadSize > 0 && *minUploadSpeed > 0 && result.UploadSpeed < *minUploadSpeed*1024*1024) {
				proxyConfig := result.ProxyConfig
				if *renameNodes {
					location, err := getIPLocation(proxyConfig["server"].(string))
					if err != nil || location.CountryCode == "" {
						otherProxies = append(otherProxies, proxyConfig)
						continue
					}
					proxyConfig["name"] = generateNodeName(location.CountryCode, result.DownloadSpeed)
				}
				otherProxies = append(otherProxies, proxyConfig)
			} else {
				proxyConfig := result.ProxyConfig
				if *renameNodes {
					location, err := getIPLocation(proxyConfig["server"].(string))
					if err != nil || location.CountryCode == "" {
						filteredProxies = append(filteredProxies, proxyConfig)
						continue
					}
					proxyConfig["name"] = generateNodeName(location.CountryCode, result.DownloadSpeed)
				}
				filteredProxies = append(filteredProxies, proxyConfig)
			}
		}
		// 保存符合条件的节点
		err = saveProxiesToFile(filteredProxies, *outputPath)
		if err != nil {
			log.Fatalln("save config file failed: %v", err)
		}
		fmt.Printf("\nsave config file to: %s\n", *outputPath)
		// 保存不符合条件的节点
		err = saveProxiesToFile(otherProxies, *otherOutputPath)
		if err != nil {
			log.Fatalln("save other config file failed: %v", err)
		}
		fmt.Printf("save other config file to: %s\n", *otherOutputPath)
	}
}

func saveProxiesToFile(proxies []map[string]any, filepath string) error {
	fixedKeyOrder := []string{
		"name", "server", "port", "client-fingerprint", "type",
		"password", "auth", "sni", "skip-cert-verify", "obfs", "obfs-password",
	}
	var yamlContent strings.Builder
	for _, proxy := range proxies {
		var items []string
		for _, key := range fixedKeyOrder {
			if value, exists := proxy[key]; exists {
				var formattedValue string
				switch v := value.(type) {
				case string:
					// 检查是否需要引号
					needsQuotes := false
					for _, c := range v {
						if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '\'' {
							needsQuotes = true
							break
						}
					}
					// 移除换行符
					v = strings.ReplaceAll(v, "\n", "")
					v = strings.ReplaceAll(v, "\r", "")
					if needsQuotes {
						// 转义双引号
						v = strings.ReplaceAll(v, `"`, `\"`)
						formattedValue = fmt.Sprintf(`"%s"`, v)
					} else {
						formattedValue = v
					}
				case bool:
					formattedValue = fmt.Sprintf("%t", v)
				case int, int8, int16, int32, int64:
					formattedValue = fmt.Sprintf("%d", v)
				case uint, uint8, uint16, uint32, uint64:
					formattedValue = fmt.Sprintf("%d", v)
				case float32, float64:
					formattedValue = fmt.Sprintf("%v", v)
				default:
					formattedValue = fmt.Sprintf("%v", v)
				}
				items = append(items, fmt.Sprintf("%s: %s", key, formattedValue))
			}
		}
		yamlContent.WriteString("  -\n")
		for _, item := range items {
			yamlContent.WriteString(fmt.Sprintf("    %s\n", item))
		}
	}
	return os.WriteFile(filepath, []byte(yamlContent.String()), 0o644)
}

func printResults(results []*speedtester.Result) {
	table := tablewriter.NewWriter(os.Stdout)
	var headers []string
	if *fastMode {
		headers = []string{
			"序号",
			"节点名称",
			"类型",
			"延迟",
		}
	} else {
		headers = []string{
			"序号",
			"节点名称",
			"类型",
			"延迟",
			"抖动",
			"丢包率",
			"下载速度",
			"上传速度",
		}
	}
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.SetColMinWidth(0, 4)  // 序号
	table.SetColMinWidth(1, 20) // 节点名称
	table.SetColMinWidth(2, 8)  // 类型
	table.SetColMinWidth(3, 8)  // 延迟
	if !*fastMode {
		table.SetColMinWidth(4, 8)  // 抖动
		table.SetColMinWidth(5, 8)  // 丢包率
		table.SetColMinWidth(6, 12) // 下载速度
		table.SetColMinWidth(7, 12) // 上传速度
	}
	for i, result := range results {
		idStr := fmt.Sprintf("%d.", i+1)
		// 延迟颜色
		latencyStr := result.FormatLatency()
		if result.Latency > 0 {
			if result.Latency < 800*time.Millisecond {
				latencyStr = colorGreen + latencyStr + colorReset
			} else if result.Latency < 1500*time.Millisecond {
				latencyStr = colorYellow + latencyStr + colorReset
			} else {
				latencyStr = colorRed + latencyStr + colorReset
			}
		} else {
			latencyStr = colorRed + latencyStr + colorReset
		}
		jitterStr := result.FormatJitter()
		if result.Jitter > 0 {
			if result.Jitter < 800*time.Millisecond {
				jitterStr = colorGreen + jitterStr + colorReset
			} else if result.Jitter < 1500*time.Millisecond {
				jitterStr = colorYellow + jitterStr + colorReset
			} else {
				jitterStr = colorRed + jitterStr + colorReset
			}
		} else {
			jitterStr = colorRed + jitterStr + colorReset
		}
		// 丢包率颜色
		packetLossStr := result.FormatPacketLoss()
		if result.PacketLoss < 10 {
			packetLossStr = colorGreen + packetLossStr + colorReset
		} else if result.PacketLoss < 20 {
			packetLossStr = colorYellow + packetLossStr + colorReset
		} else {
			packetLossStr = colorRed + packetLossStr + colorReset
		}
		// 下载速度颜色 (以MB/s为单位判断)
		downloadSpeed := result.DownloadSpeed / (1024 * 1024)
		downloadSpeedStr := result.FormatDownloadSpeed()
		if downloadSpeed >= 10 {
			downloadSpeedStr = colorGreen + downloadSpeedStr + colorReset
		} else if downloadSpeed >= 5 {
			downloadSpeedStr = colorYellow + downloadSpeedStr + colorReset
		} else {
			downloadSpeedStr = colorRed + downloadSpeedStr + colorReset
		}
		// 上传速度颜色
		uploadSpeed := result.UploadSpeed / (1024 * 1024)
		uploadSpeedStr := result.FormatUploadSpeed()
		if uploadSpeed >= 5 {
			uploadSpeedStr = colorGreen + uploadSpeedStr + colorReset
		} else if uploadSpeed >= 2 {
			uploadSpeedStr = colorYellow + uploadSpeedStr + colorReset
		} else {
			uploadSpeedStr = colorRed + uploadSpeedStr + colorReset
		}
		var row []string
		if *fastMode {
			row = []string{
				idStr,
				result.ProxyName,
				result.ProxyType,
				latencyStr,
			}
		} else {
			row = []string{
				idStr,
				result.ProxyName,
				result.ProxyType,
				latencyStr,
				jitterStr,
				packetLossStr,
				downloadSpeedStr,
				uploadSpeedStr,
			}
		}
		table.Append(row)
	}
	fmt.Println()
	table.Render()
	fmt.Println()
}

func getIPLocation(ip string) (*IPLocation, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=country,countryCode", ip))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get location for IP %s", ip)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var location IPLocation
	if err := json.Unmarshal(body, &location); err != nil {
		return nil, err
	}
	return &location, nil
}

func generateNodeName(countryCode string, downloadSpeed float64) string {
	flag, exists := countryFlags[strings.ToUpper(countryCode)]
	if !exists {
		flag = "🏳️"
	}
	speedMBps := downloadSpeed / (1024 * 1024)
	return fmt.Sprintf("%s %s | ⬇️ %.2f MB/s", flag, strings.ToUpper(countryCode), speedMBps)
}

type IPLocation struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

var countryFlags = map[string]string{
	"US": "🇺🇸", "CN": "🇨🇳", "GB": "🇬🇧", "UK": "🇬🇧", "JP": "🇯🇵", "DE": "🇩🇪", "FR": "🇫🇷", "RU": "🇷🇺",
	"SG": "🇸🇬", "HK": "🇭🇰", "TW": "🇹🇼", "KR": "🇰🇷", "CA": "🇨🇦", "AU": "🇦🇺", "NL": "🇳🇱", "IT": "🇮🇹",
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
