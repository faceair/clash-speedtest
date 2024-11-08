package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
	"github.com/metacubex/mihomo/log"
	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v3"
)

var (
	configPathsConfig = flag.String("c", "", "config file path, also support http(s) url")
	filterRegexConfig = flag.String("f", ".+", "filter proxies by name, use regexp")
	downloadURL       = flag.String("download-url", "https://speed.cloudflare.com", "download url")
	downloadSize      = flag.Int("download-size", 100*1024*1024, "download size for testing proxies")
	timeout           = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	concurrent        = flag.Int("concurrent", 4, "download concurrent size")
	outputPath        = flag.String("output", "", "output config file path")
	maxLatency        = flag.Duration("max-latency", 800*time.Millisecond, "filter latency greater than this value")
	minSpeed          = flag.Float64("min-speed", 5, "filter speed less than this value(unit: MB/s)")
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
		ConfigPaths:  *configPathsConfig,
		FilterRegex:  *filterRegexConfig,
		DownloadURL:  *downloadURL,
		DownloadSize: *downloadSize,
		Timeout:      *timeout,
		Concurrent:   *concurrent,
	})

	allProxies, err := speedTester.LoadProxies()
	if err != nil {
		log.Fatalln("load proxies failed: %v", err)
	}

	bar := progressbar.Default(int64(len(allProxies)), "测试中...")
	results := make([]*speedtester.Result, 0)
	err = speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
		bar.Add(1)
		bar.Describe(result.ProxyName)
		results = append(results, result)
	})
	if err != nil {
		log.Fatalln("test proxies failed: %v", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DownloadSpeed > results[j].DownloadSpeed
	})

	printResults(results)

	if *outputPath != "" {
		err = saveConfig(results)
		if err != nil {
			log.Fatalln("save config file failed: %v", err)
		}
		fmt.Printf("\nsave config file to: %s\n", *outputPath)
	}
}

func printResults(results []*speedtester.Result) {
	fmt.Printf("\n%-4s %-40s %-15s %-12s %-12s\n", "", "节点名称", "节点类型", "下载速度", "延迟")
	fmt.Printf("%-4s %s\n", "", strings.Repeat("-", 100))

	for i, result := range results {
		speedStr := result.FormatDownloadSpeed()

		proxyName := fmt.Sprintf("%-40s", result.ProxyName)

		color := colorGreen
		if speedStr == "0.00B/s" {
			color = colorRed
		} else if strings.HasPrefix(speedStr, "0") {
			color = colorYellow
		}

		index := fmt.Sprintf("%d.", i+1)
		fmt.Printf("%-4s %s%-40s %-15s %-12s %-12s%s\n",
			index,
			color,
			proxyName,
			result.ProxyType,
			speedStr,
			result.FormatLatency(),
			colorReset,
		)
	}
}

func saveConfig(results []*speedtester.Result) error {
	filteredResults := make([]*speedtester.Result, 0)
	for _, result := range results {
		if *maxLatency > 0 && result.Latency > *maxLatency {
			continue
		}
		if *minSpeed > 0 && float64(result.DownloadSpeed)/(1024*1024) < *minSpeed {
			continue
		}
		filteredResults = append(filteredResults, result)
	}

	proxies := make([]map[string]any, 0)
	for _, result := range filteredResults {
		proxies = append(proxies, result.ProxyConfig)
	}

	config := &speedtester.RawConfig{
		Proxies: proxies,
	}
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(*outputPath, yamlData, 0o644)
}
