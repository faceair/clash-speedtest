package tui

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	perfEventSort   = "sort_results"
	perfEventRows   = "update_table_rows"
	perfEventLayout = "update_table_layout"
)

type perfStats struct {
	Count      int
	Total      time.Duration
	Max        time.Duration
	Last       time.Duration
	ItemsTotal int
	ItemsMax   int
}

// perfTracker provides lightweight timing instrumentation when enabled by env.
type perfTracker struct {
	enabled  bool
	logEvery int
	stats    map[string]*perfStats
}

func newPerfTracker() *perfTracker {
	enabled := strings.TrimSpace(os.Getenv("CLASH_SPEEDTEST_TUI_PERF")) != ""
	logEvery := 50
	if raw := strings.TrimSpace(os.Getenv("CLASH_SPEEDTEST_TUI_PERF_LOG_EVERY")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			logEvery = parsed
		}
	}
	return &perfTracker{
		enabled:  enabled,
		logEvery: logEvery,
		stats:    make(map[string]*perfStats),
	}
}

func (p *perfTracker) record(event string, items int, start time.Time) {
	if p == nil || !p.enabled {
		return
	}
	duration := time.Since(start)
	stats := p.stats[event]
	if stats == nil {
		stats = &perfStats{}
		p.stats[event] = stats
	}
	stats.Count++
	stats.Total += duration
	stats.Last = duration
	if duration > stats.Max {
		stats.Max = duration
	}
	stats.ItemsTotal += items
	if items > stats.ItemsMax {
		stats.ItemsMax = items
	}
	if p.logEvery > 0 && stats.Count%p.logEvery == 0 {
		avg := time.Duration(int64(stats.Total) / int64(stats.Count))
		log.Printf("[tui-perf] event=%s count=%d last=%s avg=%s max=%s items_total=%d items_max=%d", event, stats.Count, stats.Last, avg, stats.Max, stats.ItemsTotal, stats.ItemsMax)
	}
}

func (p *perfTracker) snapshot(event string) perfStats {
	if p == nil {
		return perfStats{}
	}
	stats, ok := p.stats[event]
	if !ok || stats == nil {
		return perfStats{}
	}
	return *stats
}
