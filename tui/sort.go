package tui

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

func (m *tuiModel) recordSequence(result *speedtester.Result) {
	if _, ok := m.sequence[result]; ok {
		return
	}
	m.nextSequence++
	m.sequence[result] = m.nextSequence
}

func defaultSortState(fastMode bool) (int, bool) {
	if fastMode {
		return 3, true
	}
	return 6, false
}

func defaultSortAscending(column int) bool {
	switch column {
	case 6, 7:
		return false
	default:
		return true
	}
}

func (m *tuiModel) sortResults() {
	sort.SliceStable(m.results, func(i, j int) bool {
		comparison := m.compareResults(m.results[i], m.results[j])
		if m.sortAscending {
			return comparison < 0
		}
		return comparison > 0
	})
}

func (m *tuiModel) compareResults(a, b *speedtester.Result) int {
	switch m.sortColumn {
	case 0:
		return compareInt(m.sequence[a], m.sequence[b])
	case 1:
		return strings.Compare(a.ProxyName, b.ProxyName)
	case 2:
		return strings.Compare(a.ProxyType, b.ProxyType)
	case 3:
		return compareDuration(a.Latency, b.Latency)
	case 4:
		return compareDuration(a.Jitter, b.Jitter)
	case 5:
		return compareFloat(a.PacketLoss, b.PacketLoss)
	case 6:
		return compareFloat(a.DownloadSpeed, b.DownloadSpeed)
	case 7:
		return compareFloat(a.UploadSpeed, b.UploadSpeed)
	default:
		return 0
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareDuration(a, b time.Duration) int {
	aValue := durationSortValue(a)
	bValue := durationSortValue(b)
	switch {
	case aValue < bValue:
		return -1
	case aValue > bValue:
		return 1
	default:
		return 0
	}
}

func durationSortValue(value time.Duration) time.Duration {
	if value == 0 {
		return time.Duration(math.MaxInt64)
	}
	return value
}
