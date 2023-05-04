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
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/Dreamacro/clash/adapter/provider"
	ClashConfig "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"gopkg.in/yaml.v3"
)

var (
	configPathConfig   = flag.String("c", "", "specify configuration file")
	filterRegexConfig  = flag.String("f", ".*", "filter proxies by name, use regexp")
	downloadSizeConfig = flag.Int("s", 1024*1024*100, "download size for testing proxies")
	timeoutConfig      = flag.Duration("t", time.Second*5, "timeout for testing proxies")
	sortField          = flag.String("S", "b", "sort field for testing proxies,b for bandwidth,t for TTFB")

	emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{1F1E0}-\x{1F1FF}]`)
	spaceRegex = regexp.MustCompile(`\s{2,}`)
)

type Result struct {
	Bandwidth float64
	TTFB      time.Duration
}

type Result4Order struct {
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
	ClashConfig.SetHomeDir(os.TempDir())
	ClashConfig.SetConfig(*configPathConfig)

	proxies, err := loadProxies()
	if err != nil {
		log.Fatalln("Failed to load config: %s", err)
	}
	//根据正则表达式过滤代理节点
	filteredProxies := filterProxies(proxies)

	format := fmt.Sprintf("%%-32s\t%%-12s\t%%-12s\n")
	var speedTestSlice []Result4Order

	fmt.Printf(format, "节点", "带宽", "延迟")
	for _, name := range filteredProxies {
		proxy := proxies[name]
		switch proxy.Type() {
		case ClashConfig.Shadowsocks, ClashConfig.ShadowsocksR, ClashConfig.Snell, ClashConfig.Socks5, ClashConfig.Http, ClashConfig.Vmess, ClashConfig.Trojan:
			result := TestProxy(proxy, *downloadSizeConfig, *timeoutConfig)
			speedTestSlice = generateResultList(name, result, speedTestSlice)

			fmt.Printf(format, formatName(name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB))
		case ClashConfig.Direct, ClashConfig.Reject, ClashConfig.Relay, ClashConfig.Selector, ClashConfig.Fallback, ClashConfig.URLTest, ClashConfig.LoadBalance:
			continue
		default:
			log.Fatalln("Unsupported proxy type: %s", proxy.Type())
		}
	}

	if *sortField == "t" {
		sort.Slice(speedTestSlice, sortByTTFB(speedTestSlice))
	} else {
		sort.Slice(speedTestSlice, sortByBandWidth(speedTestSlice))
	}

	fmt.Println("===输出排序结果===")
	fmt.Printf(format, "节点", "带宽", "延迟")
	for _, result := range speedTestSlice {
		fmt.Printf(format, formatName(result.Name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB))
	}

	writeToCsv(speedTestSlice)
}

func generateResultList(name string, result *Result, speedTestSlice []Result4Order) []Result4Order {
	result4Order := Result4Order{
		Name:      name,
		Bandwidth: result.Bandwidth,
		TTFB:      result.TTFB,
	}
	speedTestSlice = append(speedTestSlice, result4Order)
	return speedTestSlice
}

func sortAndPrintResult(speedTestSlice []Result4Order, format string) {
	if *sortField == "t" {
		sort.Slice(speedTestSlice, sortByTTFB(speedTestSlice))
	} else {
		sort.Slice(speedTestSlice, sortByBandWidth(speedTestSlice))
	}

	fmt.Println("===输出排序结果===")
	fmt.Printf(format, "节点", "带宽", "延迟")
	for _, result := range speedTestSlice {
		fmt.Printf(format, formatName(result.Name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB))
	}
}

func filterProxies(proxies map[string]ClashConfig.Proxy) []string {
	filterRegexp := regexp.MustCompile(*filterRegexConfig)
	filteredProxies := make([]string, 0, len(proxies))
	for name := range proxies {
		if filterRegexp.MatchString(name) {
			filteredProxies = append(filteredProxies, name)
		}
	}
	sort.Strings(filteredProxies)
	return filteredProxies
}

func loadProxies() (map[string]ClashConfig.Proxy, error) {
	buf, err := os.ReadFile(ClashConfig.Path.Config())
	if err != nil {
		return nil, err
	}
	rawCfg := &RawConfig{
		Proxies: []map[string]any{},
	}
	if err := yaml.Unmarshal(buf, rawCfg); err != nil {
		return nil, err
	}
	proxies := make(map[string]ClashConfig.Proxy)
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

func TestProxy(proxy ClashConfig.Proxy, downloadSize int, timeout time.Duration) *Result {
	client := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				return proxy.DialContext(ctx, &ClashConfig.Metadata{
					Host:    host,
					DstPort: port,
				})
			},
		},
	}

	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("https://speed.cloudflare.com/__down?bytes=%d", downloadSize))
	if err != nil {
		return &Result{-1, -1}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Result{-1, -1}
	}
	ttfb := time.Since(start)

	written, _ := io.Copy(io.Discard, resp.Body)
	if written == 0 {
		return &Result{-1, -1}
	}
	downloadSize = int(written)
	downloadTime := time.Since(start) - ttfb
	bandwidth := float64(downloadSize) / downloadTime.Seconds()

	return &Result{bandwidth, ttfb}
}

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

func sortByBandWidth(speedTestSlice []Result4Order) func(i int, j int) bool {
	return func(i, j int) bool {
		return speedTestSlice[i].Bandwidth >= speedTestSlice[j].Bandwidth
	}
}

func sortByTTFB(speedTestSlice []Result4Order) func(i int, j int) bool {
	return func(i, j int) bool {
		return speedTestSlice[i].TTFB <= speedTestSlice[j].TTFB
	}
}

func writeToCsv(slice []Result4Order) {
	fileName := "./result.csv"
	os.Remove(fileName)
	csvFile, err := os.Create(fileName)
	if err != nil {
		log.Infoln("create csv file error:%v", err)
	}
	defer csvFile.Close()
	//写入UTF-8 BOM头
	csvFile.WriteString("\xEF\xBB\xBF")

	csvWriter := csv.NewWriter(csvFile)
	err = csvWriter.Write([]string{"节点", "带宽", "延迟"})
	if err != nil {
		log.Infoln("write error:%v", err)
		return
	}
	for _, result := range slice {
		csvWriter.Write([]string{formatName(result.Name), formatBandwidth(result.Bandwidth), formatMillseconds(result.TTFB)})
		if err != nil {
			log.Infoln("write data error:%v", err)
		}
	}
	csvWriter.Flush()
}
