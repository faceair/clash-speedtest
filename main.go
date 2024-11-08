package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
	"github.com/metacubex/mihomo/log"
	"github.com/schollz/progressbar/v3"
)

var (
	configPathsConfig = flag.String("c", "", "configuration file path, also support http(s) url")
	filterRegexConfig = flag.String("f", ".*", "filter proxies by name, use regexp")
	downloadURL       = flag.String("download-url", "https://speed.cloudflare.com/__down?bytes=%d", "download url")
	downloadSize      = flag.Int("download-size", 1024*1024*100, "download size for testing proxies")
	timeout           = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	concurrent        = flag.Int("concurrent", 4, "download concurrent size")
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
		log.Fatalln("请指定配置文件")
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
		log.Fatalln("加载代理失败: %v", err)
	}

	bar := progressbar.Default(int64(len(allProxies)), "测试中...")
	results := make([]*speedtester.Result, 0)
	err = speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
		bar.Add(1)
		bar.Describe(fmt.Sprintf("测试 %s 完成", result.ProxyName))
		results = append(results, result)
	})
	if err != nil {
		log.Fatalln("测试代理失败: %v", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DownloadSpeed > results[j].DownloadSpeed
	})

	fmt.Printf("\n%-4s %-42s %-15s %-12s\n", "", "节点名称", "下载速度", "延迟")
	fmt.Printf("%-4s %s\n", "", strings.Repeat("-", 75))

	for i, result := range results {
		speedStr := result.FormatDownloadSpeed()

		color := colorGreen
		if speedStr == "0.00B/s" {
			color = colorRed
		} else if strings.HasPrefix(speedStr, "0") {
			color = colorYellow
		}

		index := fmt.Sprintf("%d.", i+1)
		fmt.Printf("%-4s %s%-42s %-15s %-12s%s\n",
			index,
			color,
			result.ProxyName,
			speedStr,
			result.FormatLatency(),
			colorReset,
		)
	}
}
