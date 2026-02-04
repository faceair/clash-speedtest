package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

func TestHelpViewShowsQuitAndDetailKeys(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 1, resultChannel)
	model.windowWidth = 80
	model.windowHeight = 20

	result := &speedtester.Result{
		ProxyName: "Proxy",
		ProxyType: "SS",
		Latency:   120 * time.Millisecond,
	}
	model.results = []*speedtester.Result{result}
	model.updateTableRows()
	model.updateTableLayout()

	helpView := model.help.view()
	if !strings.Contains(helpView, "q/ctrl+c") {
		t.Fatalf("expected help to include quit shortcut, got %q", helpView)
	}
	if strings.Contains(helpView, "esc") {
		t.Fatalf("expected help to hide detail shortcut when detail is closed, got %q", helpView)
	}

	model.toggleDetail(result)
	helpView = model.help.view()
	if !strings.Contains(helpView, "esc") {
		t.Fatalf("expected help to include detail shortcut when detail is visible, got %q", helpView)
	}
}
