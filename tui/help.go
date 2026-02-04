package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

type helpState struct {
	model  help.Model
	keyMap helpKeyMap
}

type helpKeyMap struct {
	Quit        key.Binding
	CloseDetail key.Binding
	Table       table.KeyMap
}

func newHelpState(tableKeys table.KeyMap) helpState {
	state := helpState{
		model: help.New(),
		keyMap: helpKeyMap{
			Table: tableKeys,
			Quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q/ctrl+c", "quit"),
			),
			CloseDetail: key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "close details"),
			),
		},
	}
	state.setDetailVisible(false)
	return state
}

func (h *helpState) setWidth(width int) {
	h.model.Width = width
}

func (h *helpState) setDetailVisible(visible bool) {
	h.keyMap.CloseDetail.SetEnabled(visible)
}

func (h helpState) view() string {
	return h.model.View(h.keyMap)
}

func (h helpState) height() int {
	view := h.view()
	if view == "" {
		return 0
	}
	return lipgloss.Height(view)
}

func (km helpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		km.Table.LineUp,
		km.Table.LineDown,
		km.Quit,
		km.CloseDetail,
	}
}

func (km helpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Table.LineUp, km.Table.LineDown, km.Table.GotoTop, km.Table.GotoBottom},
		{km.Table.PageUp, km.Table.PageDown, km.Table.HalfPageUp, km.Table.HalfPageDown},
		{km.CloseDetail, km.Quit},
	}
}
