package speedtester

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/provider"
	"github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ConfigPaths      string
	FilterRegex      string
	BlockRegex       string
	ServerURL        string
	DownloadSize     int
	UploadSize       int
	Timeout          time.Duration
	Concurrent       int
	MaxLatency       time.Duration
	MinDownloadSpeed float64
	MinUploadSpeed   float64
	FastMode         bool
}

type SpeedTester struct {
	config           *Config
	blockedNodes     []string
	blockedNodeCount int
}

func New(config *Config) *SpeedTester {
	if config.Concurrent <= 0 {
		config.Concurrent = 1
	}
	if config.DownloadSize < 0 {
		config.DownloadSize = 100 * 1024 * 1024
	}
	if config.UploadSize < 0 {
		config.UploadSize = 10 * 1024 * 1024
	}
	return &SpeedTester{
		config: config,
	}
}

type CProxy struct {
	constant.Proxy
	Config map[string]any
}

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

func (st *SpeedTester) LoadProxies(stashCompatible bool) (map[string]*CProxy, error) {
	allProxies := make(map[string]*CProxy)
	st.blockedNodes = make([]string, 0)
	st.blockedNodeCount = 0

	for _, configPath := range strings.Split(st.config.ConfigPaths, ",") {
		var body []byte
		var err error
		if strings.HasPrefix(configPath, "http") {
			var resp *http.Response
			resp, err = http.Get(configPath)
			if err != nil {
				log.Warnln("failed to fetch config: %s", err)
				continue
			}
			body, err = io.ReadAll(resp.Body)
		} else {
			body, err = os.ReadFile(configPath)
		}
		if err != nil {
			log.Warnln("failed to read config: %s", err)
			continue
		}

		rawCfg := &RawConfig{
			Proxies: []map[string]any{},
		}
		if err := yaml.Unmarshal(body, rawCfg); err != nil {
			return nil, err
		}
		proxies := make(map[string]*CProxy)
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
			proxies[proxy.Name()] = &CProxy{Proxy: proxy, Config: config}
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

			resp, err := http.Get(config["url"].(string))
			if err != nil {
				log.Warnln("failed to fetch config: %s", err)
				continue
			}
			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			pdRawCfg := &RawConfig{
				Proxies: []map[string]any{},
			}
			if err := yaml.Unmarshal(body, pdRawCfg); err != nil {
				return nil, err
			}
			pdProxies := make(map[string]map[string]any)
			for _, pdProxy := range pdRawCfg.Proxies {
				pdProxies[pdProxy["name"].(string)] = pdProxy
			}
			for _, proxy := range pd.Proxies() {
				proxies[fmt.Sprintf("[%s] %s", name, proxy.Name())] = &CProxy{
					Proxy:  proxy,
					Config: pdProxies[proxy.Name()],
				}
			}
		}
		for k, p := range proxies {
			switch p.Type() {
			case constant.Shadowsocks, constant.ShadowsocksR, constant.Snell, constant.Socks5, constant.Http,
				constant.Vmess, constant.Vless, constant.Trojan, constant.Hysteria, constant.Hysteria2,
				constant.WireGuard, constant.Tuic, constant.Ssh, constant.Mieru, constant.AnyTLS:
			default:
				continue
			}
			if server, ok := p.Config["server"]; ok {
				p.Config["server"] = convertMappedIPv6ToIPv4(server.(string))
			}
			if stashCompatible && !isStashCompatible(p) {
				continue
			}
			if _, ok := allProxies[k]; !ok {
				allProxies[k] = p
			}
		}
	}

	filterRegexp := regexp.MustCompile(st.config.FilterRegex)
	var blockKeywords []string
	if st.config.BlockRegex != "" {
		for _, keyword := range strings.Split(st.config.BlockRegex, "|") {
			keyword = strings.TrimSpace(keyword)
			if keyword != "" {
				blockKeywords = append(blockKeywords, strings.ToLower(keyword))
			}
		}
	}

	filteredProxies := make(map[string]*CProxy)
	for name := range allProxies {
		shouldBlock := false
		if len(blockKeywords) > 0 {
			lowerName := strings.ToLower(name)
			for _, keyword := range blockKeywords {
				if strings.Contains(lowerName, keyword) {
					shouldBlock = true
					break
				}
			}
		}

		if shouldBlock {
			continue
		}
		if filterRegexp.MatchString(name) {
			filteredProxies[name] = allProxies[name]
		}
	}
	return filteredProxies, nil
}

func isStashCompatible(proxy *CProxy) bool {
	switch proxy.Type() {
	case constant.Shadowsocks:
		cipher, ok := proxy.Config["cipher"]
		if ok {
			switch cipher {
			case "aes-128-gcm", "aes-192-gcm", "aes-256-gcm",
				"aes-128-cfb", "aes-192-cfb", "aes-256-cfb",
				"aes-128-ctr", "aes-192-ctr", "aes-256-ctr",
				"rc4-md5", "chacha20", "chacha20-ietf", "xchacha20",
				"chacha20-ietf-poly1305", "xchacha20-ietf-poly1305",
				"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm":
			default:
				return false
			}
		}
	case constant.ShadowsocksR:
		if obfs, ok := proxy.Config["obfs"]; ok {
			switch obfs {
			case "plain", "http_simple", "http_post", "random_head",
				"tls1.2_ticket_auth", "tls1.2_ticket_fastauth":
			default:
				return false
			}
		}
		if protocol, ok := proxy.Config["protocol"]; ok {
			switch protocol {
			case "origin", "auth_sha1_v4", "auth_aes128_md5",
				"auth_aes128_sha1", "auth_chain_a", "auth_chain_b":
			default:
				return false
			}
		}
	case constant.Snell:
		if obfsOpts, ok := proxy.Config["obfs-opts"]; ok {
			if obfsOptsMap, ok := obfsOpts.(map[string]any); ok {
				if mode, ok := obfsOptsMap["mode"]; ok {
					switch mode {
					case "http", "tls":
					default:
						return false
					}
				}
			}
		}
	case constant.Socks5, constant.Http:
	case constant.Vmess:
		if cipher, ok := proxy.Config["cipher"]; ok {
			switch cipher {
			case "auto", "aes-128-gcm", "chacha20-poly1305", "none":
			default:
				return false
			}
		}
		if network, ok := proxy.Config["network"]; ok {
			switch network {
			case "ws", "h2", "http", "grpc":
			default:
				return false
			}
		}
	case constant.Vless:
		if flow, ok := proxy.Config["flow"]; ok {
			switch flow {
			case "xtls-rprx-origin", "xtls-rprx-direct", "xtls-rprx-splice", "xtls-rprx-vision":
			default:
				return false
			}
		}
	case constant.Trojan:
		if network, ok := proxy.Config["network"]; ok {
			switch network {
			case "ws", "grpc":
			default:
				return false
			}
		}
	case constant.Hysteria, constant.Hysteria2:
	case constant.WireGuard:
	case constant.Tuic:
	case constant.Ssh:
	default:
		return false
	}
	return true
}

func (st *SpeedTester) TestProxies(proxies map[string]*CProxy, tester func(result *Result)) {
	for name, proxy := range proxies {
		tester(st.testProxy(name, proxy))
	}
}

type testJob struct {
	name  string
	proxy *CProxy
}

type Result struct {
	ProxyName     string         `json:"proxy_name"`
	ProxyType     string         `json:"proxy_type"`
	ProxyConfig   map[string]any `json:"proxy_config"`
	Latency       time.Duration  `json:"latency"`
	Jitter        time.Duration  `json:"jitter"`
	PacketLoss    float64        `json:"packet_loss"`
	DownloadSize  float64        `json:"download_size"`
	DownloadTime  time.Duration  `json:"download_time"`
	DownloadSpeed float64        `json:"download_speed"`
	UploadSize    float64        `json:"upload_size"`
	UploadTime    time.Duration  `json:"upload_time"`
	UploadSpeed   float64        `json:"upload_speed"`
}

func (r *Result) FormatDownloadSpeed() string {
	return formatSpeed(r.DownloadSpeed)
}

func (r *Result) FormatLatency() string {
	if r.Latency == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%dms", r.Latency.Milliseconds())
}

func (r *Result) FormatJitter() string {
	if r.Jitter == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%dms", r.Jitter.Milliseconds())
}

func (r *Result) FormatPacketLoss() string {
	return fmt.Sprintf("%.1f%%", r.PacketLoss)
}

func (r *Result) FormatUploadSpeed() string {
	return formatSpeed(r.UploadSpeed)
}

func formatSpeed(bytesPerSecond float64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	speed := bytesPerSecond
	for speed >= 1024 && unit < len(units)-1 {
		speed /= 1024
		unit++
	}
	return fmt.Sprintf("%.2f%s", speed, units[unit])
}

func (st *SpeedTester) testProxy(name string, proxy *CProxy) *Result {
	result := &Result{
		ProxyName:   name,
		ProxyType:   proxy.Type().String(),
		ProxyConfig: proxy.Config,
	}

	// 1. 首先进行延迟测试
	latencyResult := st.testLatency(proxy, st.config.MaxLatency)
	result.Latency = latencyResult.avgLatency
	if st.config.FastMode {
		return result
	} else {
		result.Jitter = latencyResult.jitter
		result.PacketLoss = latencyResult.packetLoss
	}

	if result.PacketLoss == 100 || result.Latency > st.config.MaxLatency {
		return result
	}

	// 2. 并发进行下载和上传测试

	var wg sync.WaitGroup

	var totalDownloadBytes, totalUploadBytes int64
	var totalDownloadTime, totalUploadTime time.Duration
	var downloadCount, uploadCount int

	downloadChunkSize := st.config.DownloadSize / st.config.Concurrent
	if downloadChunkSize > 0 {
		downloadResults := make(chan *downloadResult, st.config.Concurrent)

		for i := 0; i < st.config.Concurrent; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				downloadResults <- st.testDownload(proxy, downloadChunkSize, st.config.Timeout)
			}()
		}
		wg.Wait()

		for range st.config.Concurrent {
			if dr := <-downloadResults; dr != nil {
				totalDownloadBytes += dr.bytes
				totalDownloadTime += dr.duration
				downloadCount++
			}
		}
		close(downloadResults)

		if downloadCount > 0 {
			result.DownloadSize = float64(totalDownloadBytes)
			result.DownloadTime = totalDownloadTime / time.Duration(downloadCount)
			result.DownloadSpeed = float64(totalDownloadBytes) / result.DownloadTime.Seconds()
		}

		if result.DownloadSpeed < st.config.MinDownloadSpeed {
			return result
		}
	}

	uploadChunkSize := st.config.UploadSize / st.config.Concurrent
	if uploadChunkSize > 0 {
		uploadResults := make(chan *downloadResult, st.config.Concurrent)

		for i := 0; i < st.config.Concurrent; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				uploadResults <- st.testUpload(proxy, uploadChunkSize, st.config.Timeout)
			}()
		}
		wg.Wait()

		for i := 0; i < st.config.Concurrent; i++ {
			if ur := <-uploadResults; ur != nil {
				totalUploadBytes += ur.bytes
				totalUploadTime += ur.duration
				uploadCount++
			}
		}
		close(uploadResults)

		if uploadCount > 0 {
			result.UploadSize = float64(totalUploadBytes)
			result.UploadTime = totalUploadTime / time.Duration(uploadCount)
			result.UploadSpeed = float64(totalUploadBytes) / result.UploadTime.Seconds()
		}

		if result.UploadSpeed < st.config.MinUploadSpeed {
			return result
		}
	}

	return result
}

type latencyResult struct {
	avgLatency time.Duration
	jitter     time.Duration
	packetLoss float64
}

func (st *SpeedTester) testLatency(proxy constant.Proxy, minLatency time.Duration) *latencyResult {
	client := st.createClient(proxy, minLatency)
	latencies := make([]time.Duration, 0, 6)
	failedPings := 0

	for i := 0; i < 6; i++ {
		time.Sleep(100 * time.Millisecond)

		start := time.Now()
		resp, err := client.Get(fmt.Sprintf("%s/__down?bytes=0", st.config.ServerURL))
		if err != nil {
			failedPings++
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			latencies = append(latencies, time.Since(start))
		} else {
			failedPings++
		}
	}

	return calculateLatencyStats(latencies, failedPings)
}

type downloadResult struct {
	bytes    int64
	duration time.Duration
}

func (st *SpeedTester) testDownload(proxy constant.Proxy, size int, timeout time.Duration) *downloadResult {
	client := st.createClient(proxy, timeout)
	start := time.Now()

	resp, err := client.Get(fmt.Sprintf("%s/__down?bytes=%d", st.config.ServerURL, size))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	downloadBytes, _ := io.Copy(io.Discard, resp.Body)

	return &downloadResult{
		bytes:    downloadBytes,
		duration: time.Since(start),
	}
}

func (st *SpeedTester) testUpload(proxy constant.Proxy, size int, timeout time.Duration) *downloadResult {
	client := st.createClient(proxy, timeout)
	reader := NewZeroReader(size)

	start := time.Now()
	resp, err := client.Post(
		fmt.Sprintf("%s/__up", st.config.ServerURL),
		"application/octet-stream",
		reader,
	)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	return &downloadResult{
		bytes:    reader.WrittenBytes(),
		duration: time.Since(start),
	}
}

func (st *SpeedTester) createClient(proxy constant.Proxy, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				var u16Port uint16
				if port, err := strconv.ParseUint(port, 10, 16); err == nil {
					u16Port = uint16(port)
				}
				return proxy.DialContext(ctx, &constant.Metadata{
					Host:    host,
					DstPort: u16Port,
				})
			},
		},
	}
}

func calculateLatencyStats(latencies []time.Duration, failedPings int) *latencyResult {
	result := &latencyResult{
		packetLoss: float64(failedPings) / 6.0 * 100,
	}

	if len(latencies) == 0 {
		return result
	}

	// 计算平均延迟
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	result.avgLatency = total / time.Duration(len(latencies))

	// 计算抖动
	var variance float64
	for _, l := range latencies {
		diff := float64(l - result.avgLatency)
		variance += diff * diff
	}
	variance /= float64(len(latencies))
	result.jitter = time.Duration(math.Sqrt(variance))

	return result
}

func convertMappedIPv6ToIPv4(server string) string {
	ip := net.ParseIP(server)
	if ip == nil {
		return server
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return server
}
