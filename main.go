package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/gist"
	"github.com/faceair/clash-speedtest/ip"
	"github.com/faceair/clash-speedtest/output"
	"github.com/faceair/clash-speedtest/speedtester"
	"github.com/faceair/clash-speedtest/tui"
	mihomolog "github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v2"
)

// Version information injected via ldflags during build
var (
	version = "dev"
	commit  = "unknown"
)

var (
	configPathsConfig = flag.String("c", "", "config file path, also support http(s) url")
	filterRegexConfig = flag.String("f", ".+", "filter proxies by name, use regexp")
	blockKeywords     = flag.String("b", "", "block proxies by keywords, use | to separate multiple keywords (example: -b 'rate|x1|1x')")
	serverURL         = flag.String("server-url", "https://dl.google.com/chrome/mac/universal/stable/GGRO/googlechrome.dmg", "server url or direct download url")
	speedMode         = flag.String("speed-mode", "download", "speed test mode: fast, download, full")
	downloadSize      = flag.Int("download-size", 50*1024*1024, "download size for testing proxies")
	uploadSize        = flag.Int("upload-size", 20*1024*1024, "upload size for testing proxies (full mode only)")
	timeout           = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	concurrent        = flag.Int("concurrent", 4, "download concurrent size")
	outputPath        = flag.String("output", "", "output config file path")
	gistToken         = flag.String("gist-token", "", "github gist token for updating output")
	gistAddress       = flag.String("gist-address", "", "github gist address or id for updating output (filename uses output basename)")
	repoToken         = flag.String("repo-token", "", "github token for updating repository file")
	repoAddress       = flag.String("repo-address", "", "github repository address or owner/repo for updating output")
	repoFilePath      = flag.String("repo-file-path", "", "repository file path for uploading output (default: output basename)")
	repoBranch        = flag.String("repo-branch", "", "repository branch for uploading output (default: repository default branch)")
	maxLatency        = flag.Duration("max-latency", time.Second, "filter latency greater than this value")
	maxPacketLoss     = flag.Float64("max-packet-loss", 100, "filter packet loss greater than this value(unit: %)")
	minDownloadSpeed  = flag.Float64("min-download-speed", 5, "filter download speed less than this value(unit: MB/s)")
	minUploadSpeed    = flag.Float64("min-upload-speed", 2, "filter upload speed less than this value(unit: MB/s, full mode only)")
	renameNodes       = flag.Bool("rename", true, "rename nodes with IP location and speed")
	fastMode          = flag.Bool("fast", false, "fast mode (alias for --speed-mode fast)")
	versionFlag       = flag.Bool("v", false, "show version information")
	userAgent         = flag.String("ua", "", "User-Agent for fetching config from http(s) URL (default: mihomo kernel UA, e.g. mihomo/1.10.0)")
)

func main() {
	flag.Parse()
	mihomolog.SetLevel(mihomolog.SILENT)

	// Handle version flag
	if *versionFlag {
		fmt.Printf("clash-speedtest version %s (commit %s)\n", version, commit)
		os.Exit(0)
	}

	if *configPathsConfig == "" {
		log.Fatalln("please specify the configuration file")
	}

	var err error
	requestedMode := speedtester.SpeedModeFast
	if !*fastMode {
		requestedMode, err = speedtester.ParseSpeedMode(*speedMode)
		if err != nil {
			log.Fatalln("parse speed mode failed: %v", err)
		}
	}

	speedTester, err := speedtester.New(&speedtester.Config{
		ConfigPaths:      *configPathsConfig,
		FilterRegex:      *filterRegexConfig,
		BlockRegex:       *blockKeywords,
		ServerURL:        *serverURL,
		DownloadSize:     *downloadSize,
		UploadSize:       *uploadSize,
		Timeout:          *timeout,
		Concurrent:       *concurrent,
		MaxPacketLoss:    *maxPacketLoss,
		MaxLatency:       *maxLatency,
		MinDownloadSpeed: *minDownloadSpeed * 1024 * 1024,
		MinUploadSpeed:   *minUploadSpeed * 1024 * 1024,
		Mode:             requestedMode,
		OutputPath:       *outputPath,
		UserAgent:        *userAgent,
	})
	if err != nil {
		log.Fatalln("create speed tester failed: %v", err)
	}
	effectiveMode := speedTester.Mode()

	allProxies, err := speedTester.LoadProxies()
	if err != nil {
		log.Fatalln("load proxies failed: %v", err)
	}

	outputMode := output.DetermineOutputMode(output.IsTerminalFile)

	var tsvWriter *output.TSVWriter
	if outputMode == output.OutputModeTSV {
		var err error
		tsvWriter, err = output.NewTSVWriter(os.Stdout, effectiveMode)
		if err != nil {
			log.Fatalln("create TSV writer failed: %v", err)
		}
	}

	results := make([]*speedtester.Result, 0, len(allProxies))

	if outputMode == output.OutputModeInteractive {
		collectResults := *outputPath != ""
		// Run TUI for Interactive mode
		resultChannel := make(chan *speedtester.Result, len(allProxies))
		resultsDone := make(chan struct{})
		saveResult := make(chan error, 1)

		// Start testing in goroutine to send results to channel
		go func() {
			speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
				if collectResults {
					results = append(results, result)
				}
				resultChannel <- result
			})
			close(resultChannel)
			close(resultsDone)
		}()

		if collectResults {
			// Save results once all tests finish, without blocking the TUI loop.
			go func() {
				<-resultsDone
				results = output.SortResults(results, effectiveMode)
				saveResult <- saveConfig(results, effectiveMode)
			}()
		}

		// Create and run TUI
		p := tea.NewProgram(
			tui.NewTUIModel(effectiveMode, len(allProxies), resultChannel),
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		)
		if _, err := p.Run(); err != nil {
			log.Fatalln("TUI failed: %v", err)
		}

		if !collectResults {
			return
		}

		err = <-saveResult
		if err != nil {
			log.Fatalln("save config file failed: %v", err)
		}
		fmt.Printf("\nsave config file to: %s\n", *outputPath)
		return
	}

	// TSV mode: collect results synchronously
	speedTester.TestProxies(allProxies, func(result *speedtester.Result) {
		results = append(results, result)

		if tsvWriter != nil {
			if err := tsvWriter.WriteRow(result, len(results)-1); err != nil {
				log.Printf("write TSV row failed: %v", err)
			}
		}
	})

	results = output.SortResults(results, effectiveMode)

	if *outputPath != "" {
		err = saveConfig(results, effectiveMode)
		if err != nil {
			log.Fatalln("save config file failed: %v", err)
		}
		fmt.Printf("\nsave config file to: %s\n", *outputPath)
	}
}

func saveConfig(results []*speedtester.Result, mode speedtester.SpeedMode) error {
	proxies := make([]map[string]any, 0)
	nameCount := make(map[string]int) // Track name usage to avoid duplicates

	for _, result := range results {
		if *maxLatency > 0 && result.Latency > *maxLatency {
			continue
		}
		if *maxPacketLoss >= 0 && result.PacketLoss > *maxPacketLoss {
			continue
		}
		if *downloadSize > 0 && *minDownloadSpeed > 0 && result.DownloadSpeed < *minDownloadSpeed*1024*1024 {
			continue
		}
		if mode.UploadEnabled() && *minUploadSpeed > 0 && result.UploadSpeed < *minUploadSpeed*1024*1024 {
			continue
		}

		proxyConfig := result.ProxyConfig
		if proxyConfig["name"] == nil || proxyConfig["server"] == nil {
			continue
		}
		if *renameNodes {
			location, err := ip.GetIPLocation(proxyConfig["server"].(string))
			if err != nil || location.CountryCode == "" {
				proxies = append(proxies, proxyConfig)
				continue
			}
			proxyConfig["name"] = ip.GenerateNodeName(location.CountryCode, result.DownloadSpeed, result.UploadSpeed, nameCount)
		}
		proxies = append(proxies, proxyConfig)
	}

	config := &speedtester.RawConfig{
		Proxies: proxies,
	}
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(*outputPath, yamlData, 0o644); err != nil {
		return err
	}
	outputFilename := filepath.Base(filepath.Clean(*outputPath))

	if *gistToken != "" && *gistAddress != "" {
		uploader := gist.NewUploader(nil)
		if err := uploader.UpdateFile(*gistToken, *gistAddress, outputFilename, yamlData); err != nil {
			log.Printf("update gist failed: %v", err)
		}
	}

	if *repoToken != "" && *repoAddress != "" {
		uploader := gist.NewUploader(nil)
		repositoryFilePath := strings.TrimSpace(*repoFilePath)
		if repositoryFilePath == "" {
			repositoryFilePath = outputFilename
		}
		if err := uploader.UpdateRepoFile(*repoToken, *repoAddress, repositoryFilePath, *repoBranch, yamlData); err != nil {
			log.Printf("update repo file failed: %v", err)
		}
	}

	return nil
}
