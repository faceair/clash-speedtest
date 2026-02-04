package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/speedtester"
)

func TestTableScrollWithKeyboard(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 5, resultChannel)

	model.results = []*speedtester.Result{
		{ProxyName: "Proxy 1", ProxyType: "SS"},
		{ProxyName: "Proxy 2", ProxyType: "SS"},
		{ProxyName: "Proxy 3", ProxyType: "SS"},
		{ProxyName: "Proxy 4", ProxyType: "SS"},
		{ProxyName: "Proxy 5", ProxyType: "SS"},
	}
	model.updateTableRows()
	model.table.SetHeight(3)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updatedModel := updated.(tuiModel)
	if updatedModel.table.Cursor() != 1 {
		t.Fatalf("expected cursor to move down to 1, got %d", updatedModel.table.Cursor())
	}
	if updatedModel.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to follow cursor, got %d", updatedModel.selectedIndex)
	}
}

func TestTableScrollWithMouseWheel(t *testing.T) {
	resultChannel := make(chan *speedtester.Result, 1)
	model := NewTUIModel(speedtester.SpeedModeDownload, 5, resultChannel)

	model.results = []*speedtester.Result{
		{ProxyName: "Proxy 1", ProxyType: "SS"},
		{ProxyName: "Proxy 2", ProxyType: "SS"},
		{ProxyName: "Proxy 3", ProxyType: "SS"},
		{ProxyName: "Proxy 4", ProxyType: "SS"},
		{ProxyName: "Proxy 5", ProxyType: "SS"},
	}
	model.updateTableRows()
	model.table.SetHeight(3)

	updated, _ := model.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	updatedModel := updated.(tuiModel)
	if updatedModel.table.Cursor() != 1 {
		t.Fatalf("expected cursor to move down to 1, got %d", updatedModel.table.Cursor())
	}
	if updatedModel.selectedIndex != 1 {
		t.Fatalf("expected selectedIndex to follow cursor, got %d", updatedModel.selectedIndex)
	}
}
