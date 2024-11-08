package speedtester

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/provider"
	"github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ConfigPaths  string
	FilterRegex  string
	DownloadURL  string
	DownloadSize int
	Timeout      time.Duration
	Concurrent   int
}

type SpeedTester struct {
	config *Config
}

func New(config *Config) *SpeedTester {
	if config.Concurrent <= 0 {
		config.Concurrent = 1
	}
	return &SpeedTester{config: config}
}

type CProxy struct {
	constant.Proxy
	Config map[string]any
}

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

func (st *SpeedTester) LoadProxies() (map[string]*CProxy, error) {
	allProxies := make(map[string]*CProxy)

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
			for _, proxy := range pd.Proxies() {
				proxies[fmt.Sprintf("[%s] %s", name, proxy.Name())] = &CProxy{Proxy: proxy}
			}
		}
		for k, p := range proxies {
			switch p.Type() {
			case constant.Shadowsocks, constant.ShadowsocksR, constant.Snell, constant.Socks5, constant.Http,
				constant.Vmess, constant.Vless, constant.Trojan, constant.Hysteria, constant.Hysteria2,
				constant.WireGuard, constant.Tuic, constant.Ssh:
			default:
				continue
			}
			if _, ok := allProxies[k]; !ok {
				allProxies[k] = p
			}
		}
	}

	filterRegexp := regexp.MustCompile(st.config.FilterRegex)
	filteredProxies := make(map[string]*CProxy)
	for name := range allProxies {
		if filterRegexp.MatchString(name) {
			filteredProxies[name] = allProxies[name]
		}
	}
	return filteredProxies, nil
}

func (st *SpeedTester) TestProxies(proxies map[string]*CProxy, fn func(result *Result)) error {
	for name, proxy := range proxies {
		fn(st.testProxy(name, proxy))
	}
	return nil
}

type Result struct {
	ProxyName     string         `json:"proxy_name"`
	ProxyType     string         `json:"proxy_type"`
	ProxyConfig   map[string]any `json:"proxy_config"`
	Latency       time.Duration  `json:"latency"`
	DownloadSize  float64        `json:"download_size"`
	DownloadTime  time.Duration  `json:"download_time"`
	DownloadSpeed float64        `json:"download_speed"`
}

func (r *Result) FormatDownloadSpeed() string {
	speed := r.DownloadSpeed
	if speed <= 0 {
		return "0.00B/s"
	}

	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	for speed >= 1024 && unit < len(units)-1 {
		speed /= 1024
		unit++
	}
	return fmt.Sprintf("%.2f%s", speed, units[unit])
}

func (r *Result) FormatLatency() string {
	if r.Latency < 0 {
		return "超时"
	}
	return fmt.Sprintf("%dms", r.Latency.Milliseconds())
}

func (st *SpeedTester) testProxy(name string, proxy *CProxy) *Result {
	chunkSize := st.config.DownloadSize / st.config.Concurrent

	var totalCount int64
	var totalLatency int64
	var totalDownloadSize int64

	startTime := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < st.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			result := st.testSingleConnection(name, proxy, chunkSize)
			if result.DownloadSize != 0 {
				atomic.AddInt64(&totalCount, 1)
				atomic.AddInt64(&totalLatency, int64(result.Latency))
				atomic.AddInt64(&totalDownloadSize, int64(result.DownloadSize))
			}
		}()
	}
	wg.Wait()
	totalDownloadTime := time.Since(startTime)

	return &Result{
		ProxyName:     name,
		ProxyType:     proxy.Type().String(),
		ProxyConfig:   proxy.Config,
		Latency:       time.Duration(totalLatency / max(totalCount, 1)),
		DownloadSize:  float64(totalDownloadSize),
		DownloadTime:  totalDownloadTime,
		DownloadSpeed: float64(totalDownloadSize) / totalDownloadTime.Seconds(),
	}
}

func (st *SpeedTester) testSingleConnection(name string, proxy constant.Proxy, downloadSize int) *Result {
	client := &http.Client{
		Timeout: st.config.Timeout,
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

	start := time.Now()
	resp, err := client.Get(fmt.Sprintf("%s/__down?bytes=%d", st.config.DownloadURL, downloadSize))
	if err != nil {
		return &Result{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &Result{}
	}
	latency := time.Since(start)

	downloadBytes, _ := io.Copy(io.Discard, resp.Body)
	downloadTime := time.Since(start) - latency

	return &Result{
		ProxyName:     name,
		Latency:       latency,
		DownloadSize:  float64(downloadBytes),
		DownloadTime:  downloadTime,
		DownloadSpeed: float64(downloadBytes) / downloadTime.Seconds(),
	}
}
