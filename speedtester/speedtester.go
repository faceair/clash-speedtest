package speedtester

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/provider"
	"github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v2"
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
	MaxPacketLoss    float64
	MinDownloadSpeed float64
	MinUploadSpeed   float64
	Mode             SpeedMode
	OutputPath       string
	UserAgent        string // optional; empty means use default (mihomo kernel UA)
}

type serverMode int

const (
	serverModeDownloadServer serverMode = iota
	serverModeDirectDownload
)

// defaultFetchConfigUA returns the default User-Agent (mihomo kernel format) when none is set.
func defaultFetchConfigUA() string {
	return constant.MihomoName + "/" + constant.Version
}

func (st *SpeedTester) fetchConfigUA() string {
	if st.config.UserAgent != "" {
		return st.config.UserAgent
	}
	return defaultFetchConfigUA()
}

func (st *SpeedTester) fetchHTTPConfig(targetURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", st.fetchConfigUA())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

type serverTarget struct {
	mode        serverMode
	baseURL     string
	downloadURL string
}

type SpeedTester struct {
	config           *Config
	blockedNodes     []string
	blockedNodeCount int
	serverMode       serverMode
	serverBaseURL    string
	downloadURL      string
	mode             SpeedMode
}

func New(config *Config) (*SpeedTester, error) {
	if config.Concurrent <= 0 {
		config.Concurrent = 1
	}
	if config.DownloadSize < 0 {
		config.DownloadSize = 100 * 1024 * 1024
	}
	if config.UploadSize < 0 {
		config.UploadSize = 10 * 1024 * 1024
	}
	mode := config.Mode
	if mode == "" {
		mode = SpeedModeDownload
	}
	target, err := resolveServerTarget(config.ServerURL)
	if err != nil {
		return nil, err
	}
	if mode == SpeedModeFull && config.UploadSize <= 0 {
		return nil, fmt.Errorf("upload size must be positive when speed mode is %s", mode)
	}
	if target.mode == serverModeDirectDownload && mode == SpeedModeFull {
		mode = SpeedModeDownload
	}
	config.Mode = mode
	return &SpeedTester{
		config:        config,
		serverMode:    target.mode,
		serverBaseURL: target.baseURL,
		downloadURL:   target.downloadURL,
		mode:          mode,
	}, nil
}

func (st *SpeedTester) Mode() SpeedMode {
	return st.mode
}

func resolveServerTarget(rawURL string) (*serverTarget, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("server url is empty")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse server url %q failed: %w", rawURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("server url %q must include scheme and host", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server url %q must use http or https scheme, got %q", rawURL, parsed.Scheme)
	}
	path := strings.TrimSpace(parsed.Path)
	hasPath := strings.Trim(path, "/") != ""
	hasQuery := parsed.RawQuery != ""
	hasFragment := parsed.Fragment != ""
	if !hasPath && !hasQuery && !hasFragment {
		return &serverTarget{
			mode:    serverModeDownloadServer,
			baseURL: strings.TrimRight(trimmed, "/"),
		}, nil
	}
	return &serverTarget{
		mode:        serverModeDirectDownload,
		downloadURL: trimmed,
	}, nil
}

type CProxy struct {
	constant.Proxy
	Config map[string]any
}

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

// sanitizeYAMLBody removes or replaces control characters that go-yaml rejects
// (e.g. "yaml: control characters are not allowed"). Tabs are replaced with spaces;
// other control chars (except \n, \r) are removed; \r is normalized to \n.
func sanitizeYAMLBody(body []byte) []byte {
	var b bytes.Buffer
	b.Grow(len(body))
	for _, r := range string(body) {
		switch {
		case r == '\t':
			b.WriteString("  ") // YAML indentation must be spaces
		case r == '\r':
			b.WriteRune('\n')
		case r == '\n':
			b.WriteRune(r)
		case unicode.IsControl(r):
			// drop other control characters
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.Bytes()
}

func (st *SpeedTester) LoadProxies() (map[string]*CProxy, error) {
	allProxies := make(map[string]*CProxy)
	st.blockedNodes = make([]string, 0)
	st.blockedNodeCount = 0

	for configPath := range strings.SplitSeq(st.config.ConfigPaths, ",") {
		var body []byte
		var err error
		if strings.HasPrefix(configPath, "http") {
			body, err = st.fetchHTTPConfig(strings.TrimSpace(configPath))
			if err != nil {
				log.Printf("failed to fetch config: %s", err)
				continue
			}
		} else {
			body, err = os.ReadFile(configPath)
		}
		if err != nil {
			log.Printf("failed to read config: %s", err)
			continue
		}

		body = sanitizeYAMLBody(body)
		rawCfg := &RawConfig{
			Proxies: []map[string]any{},
		}
		if err := yaml.Unmarshal(body, rawCfg); err != nil {
			return nil, fmt.Errorf("unable to parse config at path %s: %w, body: %s", configPath, err, body)
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
				log.Printf("initial proxy provider %s error: %v", pd.Name(), err)
				continue
			}

			body, err = st.fetchHTTPConfig(config["url"].(string))
			if err != nil {
				log.Printf("failed to fetch config: %s", err)
				continue
			}
			body = sanitizeYAMLBody(body)
			pdRawCfg := &RawConfig{
				Proxies: []map[string]any{},
			}
			if err := yaml.Unmarshal(body, pdRawCfg); err != nil {
				return nil, fmt.Errorf("unable to parse config: %w, body: %s", err, body)
			}
			pdProxies := make(map[string]map[string]any)
			for _, pdProxy := range pdRawCfg.Proxies {
				if pdProxy["name"] == nil || pdProxy["server"] == nil {
					continue
				}
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
				constant.WireGuard, constant.Tuic, constant.Ssh, constant.Mieru, constant.AnyTLS, constant.Sudoku:
			default:
				continue
			}
			if server, ok := p.Config["server"]; ok {
				p.Config["server"] = convertMappedIPv6ToIPv4(server.(string))
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

func (st *SpeedTester) TestProxies(proxies map[string]*CProxy, tester func(result *Result)) {
	for name, proxy := range proxies {
		tester(st.testProxy(name, proxy))
	}
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
	DownloadError string         `json:"download_error"`
	UploadSize    float64        `json:"upload_size"`
	UploadTime    time.Duration  `json:"upload_time"`
	UploadSpeed   float64        `json:"upload_speed"`
	UploadError   string         `json:"upload_error"`
}

func (r *Result) FormatDownloadSpeed() string {
	if r.DownloadError != "" {
		return r.DownloadError
	}
	return formatSpeed(r.DownloadSpeed)
}

func (r *Result) FormatDownloadSpeedValue() string {
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
	if r.UploadError != "" {
		return r.UploadError
	}
	return formatSpeed(r.UploadSpeed)
}

func (r *Result) FormatUploadSpeedValue() string {
	return formatSpeed(r.UploadSpeed)
}

func (r *Result) FormatDownloadError() string {
	if r.DownloadError == "" {
		return "N/A"
	}
	return r.DownloadError
}

func (r *Result) FormatUploadError() string {
	if r.UploadError == "" {
		return "N/A"
	}
	return r.UploadError
}

func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond == 0 {
		return "N/A"
	}
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
	result.Jitter = latencyResult.jitter
	result.PacketLoss = latencyResult.packetLoss

	if st.mode.IsFast() || result.PacketLoss == 100 {
		return result
	}
	if st.config.OutputPath != "" && st.config.MaxPacketLoss < 100 && latencyResult.packetLoss > st.config.MaxPacketLoss {
		return result
	}
	if st.config.OutputPath != "" && st.config.MaxLatency > 0 && latencyResult.avgLatency > st.config.MaxLatency {
		return result
	}

	// 2. 并发进行下载测试，按需进行上传测试

	var wg sync.WaitGroup

	downloadSummary := newTransferSummary()
	var uploadSummary *transferSummary
	if st.mode.UploadEnabled() {
		uploadSummary = newTransferSummary()
	}

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
				downloadSummary.add(dr)
			}
		}
		close(downloadResults)

		result.DownloadSize, result.DownloadTime, result.DownloadSpeed, result.DownloadError = applyTransferSummary(downloadSummary)

		if st.config.OutputPath != "" && st.config.MinDownloadSpeed > 0 && result.DownloadSpeed < st.config.MinDownloadSpeed {
			return result
		}
	}

	if st.mode.UploadEnabled() {
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
					uploadSummary.add(ur)
				}
			}
			close(uploadResults)

			result.UploadSize, result.UploadTime, result.UploadSpeed, result.UploadError = applyTransferSummary(uploadSummary)
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
	defer client.CloseIdleConnections()

	latencies := make([]time.Duration, 0, 6)
	failedPings := 0

	for range 6 {
		time.Sleep(100 * time.Millisecond)

		start := time.Now()
		req, err := http.NewRequest(http.MethodHead, st.downloadURL, nil)
		if err != nil {
			failedPings++
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			failedPings++
			continue
		}
		resp.Body.Close()
		latencies = append(latencies, time.Since(start))
	}

	return calculateLatencyStats(latencies, failedPings)
}

type downloadResult struct {
	error    string
	bytes    int64
	duration time.Duration
}

type transferSummary struct {
	totalBytes    int64
	totalDuration time.Duration
	successCount  int
	errors        []string
	errorSeen     map[string]struct{}
}

func applyTransferSummary(summary *transferSummary) (float64, time.Duration, float64, string) {
	if summary == nil {
		return 0, 0, 0, ""
	}
	var size float64
	var duration time.Duration
	var speed float64
	var errorMessage string
	if summary.successCount > 0 {
		size = float64(summary.totalBytes)
		duration = summary.averageDuration()
		if duration > 0 {
			speed = float64(summary.totalBytes) / duration.Seconds()
		}
	}
	if len(summary.errors) > 0 {
		errorMessage = strings.Join(summary.errors, "; ")
		// If any transfer error is reported, treat the speed as zero.
		speed = 0
	}
	return size, duration, speed, errorMessage
}

func newTransferSummary() *transferSummary {
	return &transferSummary{
		errorSeen: make(map[string]struct{}),
	}
}

func (s *transferSummary) add(result *downloadResult) {
	if result == nil {
		return
	}
	if result.error != "" {
		s.appendError(result.error)
		return
	}
	s.totalBytes += result.bytes
	s.totalDuration += result.duration
	s.successCount++
}

func (s *transferSummary) appendError(message string) {
	if message == "" {
		return
	}
	if s.errorSeen == nil {
		s.errorSeen = make(map[string]struct{})
	}
	if _, exists := s.errorSeen[message]; exists {
		return
	}
	s.errorSeen[message] = struct{}{}
	s.errors = append(s.errors, message)
}

func (s *transferSummary) averageDuration() time.Duration {
	if s.successCount == 0 {
		return 0
	}
	return s.totalDuration / time.Duration(s.successCount)
}

func (st *SpeedTester) testDownload(proxy constant.Proxy, size int, timeout time.Duration) *downloadResult {
	client := st.createClient(proxy, timeout)
	defer client.CloseIdleConnections()

	start := time.Now()
	var downloadURL string
	if st.serverMode == serverModeDirectDownload {
		downloadURL = st.downloadURL
	} else {
		downloadURL = fmt.Sprintf("%s/__down?bytes=%d", st.serverBaseURL, size)
	}

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return &downloadResult{
			error: fmt.Sprintf("create download request for %s failed: %v", downloadURL, err),
		}
	}
	if st.serverMode == serverModeDirectDownload && size > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", size-1))
	}
	resp, err := client.Do(req)
	if err != nil {
		return &downloadResult{
			error: fmt.Sprintf("download request to %s failed: %v, spent %s", downloadURL, err, time.Since(start)),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return &downloadResult{
			error: fmt.Sprintf("download response from %s returned %s, spent %s", downloadURL, resp.Status, time.Since(start)),
		}
	}

	downloadBytes, _ := io.Copy(io.Discard, resp.Body)
	return &downloadResult{
		bytes:    downloadBytes,
		duration: time.Since(start),
	}
}

func (st *SpeedTester) testUpload(proxy constant.Proxy, size int, timeout time.Duration) *downloadResult {
	client := st.createClient(proxy, timeout)
	defer client.CloseIdleConnections()

	reader := NewZeroReader(size)
	uploadURL := fmt.Sprintf("%s/__up", st.serverBaseURL)

	start := time.Now()
	resp, err := client.Post(
		uploadURL,
		"application/octet-stream",
		reader,
	)
	if err != nil {
		return &downloadResult{
			error: fmt.Sprintf("upload request to %s failed: %v, spent %s", uploadURL, err, time.Since(start)),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &downloadResult{
			error: fmt.Sprintf("upload response from %s returned %s, spent %s", uploadURL, resp.Status, time.Since(start)),
		}
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
