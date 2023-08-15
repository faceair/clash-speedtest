package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/Dreamacro/clash/adapter/outbound"
	"github.com/Dreamacro/clash/common/structure"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/Dreamacro/clash/adapter/provider"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
	"gopkg.in/yaml.v3"
)

var (
	livenessObject     = flag.String("l", "https://speed.cloudflare.com/__down?bytes=%d", "liveness object, support http(s) url, support payload too")
	configPathConfig   = flag.String("c", "", "configuration file path, also support http(s) url")
	filterRegexConfig  = flag.String("f", ".*", "filter proxies by name, use regexp")
	downloadSizeConfig = flag.Int("size", 1024*1024*100, "download size for testing proxies")
	timeoutConfig      = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	sortField          = flag.String("sort", "b", "sort field for testing proxies, b for bandwidth, t for TTFB")
	output             = flag.String("output", "", "output result to csv/yaml file")
	concurrent         = flag.Int("concurrent", 4, "download concurrent size")
)

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

func DifferentTypesOfParsing(pv string, isUriType bool) any {
	defer func() {
		// 防止解析过程中出现获取文件句柄出现致命性错误
		if err := recover(); err != nil {
			log.Warnln("There is an error url being ignored : %s", err)
		}
	}()
	var err error
	if isUriType {
		var u *url.URL
		if u, err = url.Parse(pv); err != nil {
			return nil
		}

		if u.Host == "" {
			return nil
		}
		return u
	} else {
		if _, err = os.Stat(pv); err != nil {
			return DifferentTypesOfParsing(pv, true)
		}
		return os.File{}
	}
}

func main() {
	flag.Parse()

	if *configPathConfig == "" {
		log.Fatalln("Please specify the configuration file")
	}

	var allProxies = make(map[string]C.Proxy)
	arURL := strings.Split(*configPathConfig, ",")
	for _, v := range arURL {
		var u any
		var body []byte
		var err error

		// 前置最基本的判断，如果错了也可以通过在方法内纠正回来
		u = DifferentTypesOfParsing(v, strings.HasPrefix(v, "http"))

		// 避免因为多层 if 产生的嵌套造成代码维护性下降
		switch u.(type) {
		case os.File:
			if body, err = os.ReadFile(v); err != nil {
				log.Warnln("Failed to decode config: %s", err)
				continue
			}
		case *url.URL:
			resp, err := http.Get(u.(*url.URL).String())
			if err != nil {
				log.Warnln("Failed to fetch config: %s", err)
			}
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				log.Warnln("Failed to read config: %s", err)
			}
		}

		lps, err := loadProxies(body)
		if err != nil {
			log.Fatalln("Failed to convert : %s", err)
		}

		for k, p := range lps {
			if _, ok := allProxies[k]; !ok {
				allProxies[k] = p
			}
		}
	}

	filteredProxies := filterProxies(*filterRegexConfig, allProxies)
	results := make([]Result, 0, len(filteredProxies))

	format := "%s%-42s\t%-12s\t%-12s\033[0m\n"

	fmt.Printf(format, "", "节点", "带宽", "延迟")
	for _, name := range filteredProxies {
		proxy := allProxies[name]
		switch proxy.Type() {
		case C.Shadowsocks, C.ShadowsocksR, C.Snell, C.Socks5, C.Http, C.Vmess, C.Trojan:
			result := TestProxyConcurrent(name, proxy, *downloadSizeConfig, *timeoutConfig, *concurrent)
			result.Printf(format)
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
		fmt.Printf(format, "", "节点", "带宽", "延迟")
		for _, result := range results {
			result.Printf(format)
		}
	}

	if strings.EqualFold(*output, "yaml") {
		if err := writeNodeConfigurationToYAML("result.yaml", results, allProxies); err != nil {
			log.Fatalln("Failed to write yaml: %s", err)
		}
	} else if strings.EqualFold(*output, "csv") {
		if err := writeToCSV("result.csv", results); err != nil {
			log.Fatalln("Failed to write csv: %s", err)
		}
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

func loadProxies(buf []byte) (map[string]C.Proxy, error) {
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

type Result struct {
	Name      string
	Bandwidth float64
	TTFB      time.Duration
}

var (
	red   = "\033[31m"
	green = "\033[32m"
)

func (r *Result) Printf(format string) {
	color := ""
	if r.Bandwidth < 1024*1024 {
		color = red
	} else if r.Bandwidth > 1024*1024*10 {
		color = green
	}
	fmt.Printf(format, color, formatName(r.Name), formatBandwidth(r.Bandwidth), formatMilliseconds(r.TTFB))
}

func TestProxyConcurrent(name string, proxy C.Proxy, downloadSize int, timeout time.Duration, concurrentCount int) *Result {
	if concurrentCount <= 0 {
		concurrentCount = 1
	}

	chunkSize := downloadSize / concurrentCount
	totalTTFB := int64(0)
	downloaded := int64(0)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(i int) {
			result, w := TestProxy(name, proxy, chunkSize, timeout)
			if w != 0 {
				atomic.AddInt64(&downloaded, w)
				atomic.AddInt64(&totalTTFB, int64(result.TTFB))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	downloadTime := time.Since(start)

	result := &Result{
		Name:      name,
		Bandwidth: float64(downloaded) / downloadTime.Seconds(),
		TTFB:      time.Duration(totalTTFB / int64(concurrentCount)),
	}

	return result
}

func TestProxy(name string, proxy C.Proxy, downloadSize int, timeout time.Duration) (*Result, int64) {
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
	resp, err := client.Get(fmt.Sprintf(*livenessObject, downloadSize))
	if err != nil {
		return &Result{name, -1, -1}, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode-http.StatusOK > 100 {
		return &Result{name, -1, -1}, 0
	}
	ttfb := time.Since(start)

	written, _ := io.Copy(io.Discard, resp.Body)
	if written == 0 {
		return &Result{name, -1, -1}, 0
	}
	downloadTime := time.Since(start) - ttfb
	bandwidth := float64(written) / downloadTime.Seconds()

	return &Result{name, bandwidth, ttfb}, written
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

func formatMilliseconds(v time.Duration) string {
	if v <= 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.02fms", float64(v.Milliseconds()))
}

func writeNodeConfigurationToYAML(filePath string, results []Result, proxies map[string]C.Proxy) error {
	fp, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer fp.Close()

	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true})
	var sortedProxies []any
	for _, result := range results {
		if v, ok := proxies[result.Name]; ok {
			dst := &outbound.TrojanOption{}
			err = decoder.Decode(map[string]any{result.Name: v}, dst)
			if err != nil {
				log.Warnln("%s", err)
			}
			sortedProxies = append(sortedProxies, dst)
		}
	}

	bytes, err := yaml.Marshal(sortedProxies)
	if err != nil {
		return err
	}

	fmt.Printf("%s", bytes)
	return nil
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
