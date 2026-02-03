package output

import (
	"fmt"
	"sort"

	"github.com/faceair/clash-speedtest/speedtester"
)

// GetHeaders returns table headers based on mode
// fast mode: ID, Name, Type, Latency
// normal mode: ID, Name, Type, Latency, Jitter, Packet Loss, Download Speed, Upload Speed
func GetHeaders(fastMode bool) []string {
	if fastMode {
		return []string{
			"序号",
			"节点名称",
			"类型",
			"延迟",
		}
	}
	return []string{
		"序号",
		"节点名称",
		"类型",
		"延迟",
		"抖动",
		"丢包率",
		"下载速度",
		"上传速度",
	}
}

// FormatRow formats a single result row without ANSI colors
// Returns plain text strings using speedtester.Result's Format* methods
func FormatRow(result *speedtester.Result, fastMode bool, index int) []string {
	idStr := fmt.Sprintf("%d.", index+1)

	if fastMode {
		return []string{
			idStr,
			result.ProxyName,
			result.ProxyType,
			result.FormatLatency(),
		}
	}
	return []string{
		idStr,
		result.ProxyName,
		result.ProxyType,
		result.FormatLatency(),
		result.FormatJitter(),
		result.FormatPacketLoss(),
		result.FormatDownloadSpeed(),
		result.FormatUploadSpeed(),
	}
}

// SortResults sorts results based on mode
// fast mode: latency ascending (lower is better)
// normal mode: download speed descending (higher is better)
func SortResults(results []*speedtester.Result, fastMode bool) {
	if fastMode {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Latency < results[j].Latency
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			return results[i].DownloadSpeed > results[j].DownloadSpeed
		})
	}
}
