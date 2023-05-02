package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Dreamacro/clash/adapter"
	"github.com/Dreamacro/clash/adapter/provider"
	C "github.com/Dreamacro/clash/constant"
	providerTypes "github.com/Dreamacro/clash/constant/provider"
	"github.com/Dreamacro/clash/log"
	"gopkg.in/yaml.v3"
)

var (
	configPathConfig   = flag.String("c", "", "specify configuration file")
	filterRegexConfig  = flag.String("f", ".*", "filter proxies by name, use regexp")
	downloadSizeConfig = flag.Int("s", 1024*1024*100, "download size for testing proxies")
	timeoutConfig      = flag.Duration("t", time.Second*5, "timeout for testing proxies")
)

func main() {
	flag.Parse()

	if *configPathConfig == "" {
		log.Fatalln("Please specify the configuration file")
	}
	if !filepath.IsAbs(*configPathConfig) {
		currentDir, _ := os.Getwd()
		*configPathConfig = filepath.Join(currentDir, *configPathConfig)
	}
	filterRe := regexp.MustCompile(*filterRegexConfig)

	C.SetHomeDir(os.TempDir())

	C.SetConfig(*configPathConfig)

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalln("Failed to load config: %s", err)
	}

	fmt.Printf("%-30s\t%-10s\t%-12s\n", "ProxyName", "Bandwidth", "ResponseTime")

	for _, proxy := range cfg.Proxies {
		switch proxy.Type() {
		case C.Shadowsocks, C.ShadowsocksR, C.Snell, C.Socks5, C.Http, C.Vmess, C.Trojan:
			if filterRe.MatchString(proxy.Name()) {
				name := formatName(proxy.Name())
				result := TestProxy(proxy, *downloadSizeConfig, *timeoutConfig)
				fmt.Printf("%-30s\t%-10s\t%-12s\n", name, formatBandwidth(result.Bandwidth), formatMillseconds(result.ResponseTime))
			}
		case C.Direct, C.Reject, C.Relay, C.Selector, C.Fallback, C.URLTest, C.LoadBalance:
			continue
		default:
			log.Fatalln("Unsupported proxy type: %s", proxy.Type())
		}
	}
	for _, provider := range cfg.Providers {
		for _, proxy := range provider.Proxies() {
			switch proxy.Type() {
			case C.Shadowsocks, C.ShadowsocksR, C.Snell, C.Socks5, C.Http, C.Vmess, C.Trojan:
				if filterRe.MatchString(proxy.Name()) {
					name := fmt.Sprintf("[%s] %s", provider.Name(), formatName(proxy.Name()))
					result := TestProxy(proxy, *downloadSizeConfig, *timeoutConfig)
					fmt.Printf("%-30s\t%-10s\t%-12s\n", name, formatBandwidth(result.Bandwidth), formatMillseconds(result.ResponseTime))
				}
			case C.Direct, C.Reject, C.Relay, C.Selector, C.Fallback, C.URLTest, C.LoadBalance:
				continue
			default:
				log.Fatalln("Unsupported proxy type: %s", proxy.Type())
			}
		}
	}
}

type Config struct {
	Proxies   map[string]C.Proxy
	Providers map[string]providerTypes.ProxyProvider
}

type RawConfig struct {
	Providers map[string]map[string]any `yaml:"proxy-providers"`
	Proxies   []map[string]any          `yaml:"proxies"`
}

func LoadConfig() (*Config, error) {
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
	providers := make(map[string]providerTypes.ProxyProvider)
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
		providers[name] = pd
	}
	return &Config{proxies, providers}, nil
}

type Result struct {
	Bandwidth    float64
	ResponseTime time.Duration
}

func TestProxy(proxy C.Proxy, downloadSize int, timeout time.Duration) *Result {
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
		return &Result{-1, 0}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &Result{-1, 0}
	}
	responseTime := time.Since(start)

	io.Copy(io.Discard, resp.Body)
	downloadTime := time.Since(start) - responseTime
	bandwidth := float64(downloadSize) / downloadTime.Seconds()

	return &Result{bandwidth, responseTime}
}

var (
	emojiRegex = regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{1F1E0}-\x{1F1FF}]`)
	pipeRegex  = regexp.MustCompile(`\|.*`)
	spaceRegex = regexp.MustCompile(`\s{2,}`)
)

func formatName(name string) string {
	noEmoji := emojiRegex.ReplaceAllString(name, "")
	noPipe := pipeRegex.ReplaceAllString(noEmoji, "")
	mergedSpaces := spaceRegex.ReplaceAllString(noPipe, " ")
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
	return fmt.Sprintf("%.02fms", float64(v.Milliseconds()))
}
