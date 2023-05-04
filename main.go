package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/Dreamacro/clash/adapter/provider"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"gopkg.in/yaml.v3"
)

var (
	configPathConfig   = flag.String("c", "", "specify configuration file")
	filterRegexConfig  = flag.String("f", ".*", "filter proxies by name, use regexp")
	downloadSizeConfig = flag.Int("size", 1024*1024*100, "download size for testing proxies")
	timeoutConfig      = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	sortField          = flag.String("sort", "", "sort field for testing proxies, b for bandwidth, t for TTFB")
	output             = flag.String("output", "", "output result to csv file")
)

type Result struct {
	Name      string
	Bandwidth float64
	TTFB      time.Duration
}

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

func main() {
	flag.Parse()

	if *configPathConfig == "" {
		log.Fatalln("Please specify the configuration file")
	}
	if !filepath.IsAbs(*configPathConfig) {
		currentDir, _ := os.Getwd()
		*configPathConfig = filepath.Join(currentDir, *configPathConfig)
	}
	C.SetHomeDir(os.TempDir())
	C.SetConfig(*configPathConfig)

	proxies, err := loadProxies()
	if err != nil {
		log.Fatalln("Failed to load config: %s", err)
	}

	filteredProxies := filterProxies(*filterRegexConfig, proxies)
	results := make([]Result, 0, len(filteredProxies))

	format := fmt.Sprintf("%%-32s\t%%-12s\t%%-12s\n")

	fmt.Printf(format, "节点", "带宽", "延迟")
	for _, name := range filteredProxies {
		proxy := proxies[name]
		switch proxy.Type() {
		case C.Shadowsocks, C.ShadowsocksR, C.Snell, C.Socks5, C.Http, C.Vmess, C.Trojan:
			result := TestProxy(name, proxy, *downloadSizeConfig, *timeoutConfig)
			fmt.Printf(format, formatName(name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB))
			results = append(results, *result)
		case C.Direct, C.Reject, C.Relay, C.Selector, C.Fallback, C.URLTest, C.LoadBalance:
			continue
		default:
			log.Fatalln("Unsupported proxy type: %s", proxy.Type())
		}
	}

	if *sortField != "" {
		switch *sortField {
		case "b", "bandwidth":
			sort.Slice(results, func(i, j int) bool {
				return results[i].Bandwidth > results[j].Bandwidth
			})
			fmt.Println("\n\n===结果按照带宽排序===")
		case "t", "ttfb":
			sort.Slice(results, func(i, j int) bool {
				return results[i].TTFB < results[j].TTFB
			})
			fmt.Println("\n\n===结果按照延迟排序===")
		default:
			log.Fatalln("Unsupported sort field: %s", *sortField)
		}
		fmt.Printf(format, "节点", "带宽", "延迟")
		for _, result := range results {
			fmt.Printf(format, formatName(result.Name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB))
		}
	}

	if *output != "" {
		writeToCSV(*output, results)
	}
}

func filterProxies(filter string, proxies map[string]C.Proxy) []string {
	filterRegexp := regexp.MustCompile(filter)
	filteredProxies := make([]string, 0, len(proxies))
	for name := range proxies {
		if filterRegexp.MatchString(name) {
			filteredProxies = append(filteredProxies, name)
		}
	}
	sort.Strings(filteredProxies)
	return filteredProxies
}

func loadProxies() (map[string]C.Proxy, error) {
	buf, err := os.ReadFile(C.Path.Config())
	if err != nil {
		return nil, err
	}
	rawCfg := &RawConfig{
		Proxies: []map[string]any{},
	}
	if err := yaml.Unmarshal(buf, rawCfg); err != nil {
		return nil, err
	}
	proxies := make(map[string]C.Proxy)
	proxiesConfig := rawCfg.Proxies
	providersConfig := rawCfg.Providers

	for i, config := range proxiesConfig {
		proxy, err := adapter.ParseProxy(config)
		if err != nil {
			return nil, fmt.Errorf("proxy %d: %w", i, err)
		}

		if _, exist := proxies[proxy.Name()]; exist {
			return nil, fmt.Errorf("proxy %s is the duplicate name", proxy.Name())
		}
		proxies[proxy.Name()] = proxy
	}
	for name, config := range providersConfig {
		if name == provider.ReservedName {
			return nil, fmt.Errorf("can not defined a provider called `%s`", provider.ReservedName)
		}
		pd, err := provider.ParseProxyProvider(name, config)
		if err != nil {
			return nil, fmt.Errorf("parse proxy provider %s error: %w", name, err)
		}
		if err := pd.Initial(); err != nil {
			return nil, fmt.Errorf("initial proxy provider %s error: %w", pd.Name(), err)
		}
		for _, proxy := range pd.Proxies() {
			proxies[fmt.Sprintf("[%s] %s", name, proxy.Name())] = proxy
		}
	}
	return proxies, nil
}

func TestProxy(name string, proxy C.Proxy, downloadSize int, timeout time.Duration) *Result {
	client := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return proxy.DialContext(ctx, &C.Metadata{
					Host:    host,
					DstPort: port,
				})
			},
		},
	}

	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("https://speed.cloudflare.com/__down?bytes=%d", downloadSize))
	if err != nil {
		return &Result{name, -1, -1}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Result{name, -1, -1}
	}
	ttfb := time.Since(start)

	written, _ := io.Copy(io.Discard, resp.Body)
	if written == 0 {
		return &Result{name, -1, -1}
	}
	downloadTime := time.Since(start) - ttfb
	bandwidth := float64(written) / downloadTime.Seconds()

	return &Result{name, bandwidth, ttfb}
}

var (
	emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{1F1E0}-\x{1F1FF}]`)
	spaceRegex = regexp.MustCompile(`\s{2,}`)
)

func formatName(name string) string {
	noEmoji := emojiRegex.ReplaceAllString(name, "")
	mergedSpaces := spaceRegex.ReplaceAllString(noEmoji, " ")
	return strings.TrimSpace(mergedSpaces)
}

func formatBandwidth(v float64) string {
	if v <= 0 {
		return "N/A"
	}
	if v < 1024 {
		return fmt.Sprintf("%.02fB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fKB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fMB/s", v)
	}
	v /= 1024
	if v < 1024 {
		return fmt.Sprintf("%.02fGB/s", v)
	}
	v /= 1024
	return fmt.Sprintf("%.02fTB/s", v)
}

func formatMillseconds(v time.Duration) string {
	if v <= 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.02fms", float64(v.Milliseconds()))
}

func writeToCSV(filePath string, results []Result) error {
	csvFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	// 写入 UTF-8 BOM 头
	csvFile.WriteString("\xEF\xBB\xBF")

	csvWriter := csv.NewWriter(csvFile)
	err = csvWriter.Write([]string{"节点", "带宽 (MB/s)", "延迟 (ms)"})
	if err != nil {
		return err
	}
	for _, result := range results {
		line := []string{
			result.Name,
			fmt.Sprintf("%.2f", result.Bandwidth/1024/1024),
			strconv.FormatInt(result.TTFB.Milliseconds(), 10),
		}
		err := csvWriter.Write(line)
		if err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return nil
}
