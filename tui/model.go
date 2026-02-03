package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faceair/clash-speedtest/output"
	"github.com/faceair/clash-speedtest/speedtester"
)

// Messages for TUI updates
type resultMsg struct {
	result *speedtester.Result
}

type progressMsg struct {
	current int
	total   int
	name    string
}

type doneMsg struct{}

type timerTickMsg struct{}

// tuiModel represents the Bubble Tea model for the TUI
type tuiModel struct {
	fastMode      bool
	totalProxies  int
	currentProxy  int
	results       []*speedtester.Result
	sequence      map[*speedtester.Result]int
	nextSequence  int
	baseHeaders   []string
	testing       bool
	quitting      bool
	progress      progress.Model
	table         table.Model
	resultChannel chan *speedtester.Result
	sortColumn    int
	sortAscending bool
	detailVisible bool
	detailResult  *speedtester.Result
	selectedIndex int
	windowWidth   int
	windowHeight  int
	startTime     time.Time
}

const (
	tableHeaderPadding  = 1
	tableHeaderLines    = 2
	detailPanelMinWidth = 60
	defaultDetailWidth  = 80
)

var selectedRowStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("62")).
	Foreground(lipgloss.Color("230")).
	Bold(true)

// NewTUIModel creates a new TUI model
func NewTUIModel(fastMode bool, totalProxies int, resultChannel chan *speedtester.Result) tuiModel {
	// Initialize progress bar
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	// Initialize table with headers
	headers := output.GetHeaders(fastMode)
	sortColumn, sortAscending := defaultSortState(fastMode)
	columns := buildColumns(addSortIndicators(headers, sortColumn, sortAscending), 0, fastMode)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = selectedRowStyle
	t.SetStyles(s)

	return tuiModel{
		fastMode:      fastMode,
		totalProxies:  totalProxies,
		currentProxy:  0,
		results:       make([]*speedtester.Result, 0),
		sequence:      make(map[*speedtester.Result]int),
		nextSequence:  0,
		baseHeaders:   headers,
		testing:       true,
		quitting:      false,
		progress:      p,
		table:         t,
		resultChannel: resultChannel,
		sortColumn:    sortColumn,
		sortAscending: sortAscending,
		detailVisible: false,
		detailResult:  nil,
		selectedIndex: -1,
		windowWidth:   0,
		windowHeight:  0,
		startTime:     time.Now(),
	}
}

// Init initializes the TUI model
func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		timerTickCmd(),
		m.waitForResult(),
	)
}

// waitForResult waits for results from the channel
func (m tuiModel) waitForResult() tea.Cmd {
	return func() tea.Msg {
		result, ok := <-m.resultChannel
		if !ok {
			return doneMsg{}
		}
		return resultMsg{result: result}
	}
}

// Update handles messages and updates the model
func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.detailVisible {
				m.detailVisible = false
				return m, nil
			}
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionRelease {
			if m.isHeaderClick(msg.Y) {
				if columnIndex := m.columnAtX(msg.X); columnIndex >= 0 {
					if columnIndex == m.sortColumn {
						m.sortAscending = !m.sortAscending
					} else {
						m.sortColumn = columnIndex
						m.sortAscending = defaultSortAscending(columnIndex)
					}
					m.sortResults()
					m.updateTableHeaders()
					m.updateTableRows()
				}
				return m, nil
			}
			if rowIndex, ok := m.rowAtY(msg.Y); ok {
				m.setSelection(rowIndex)
				m.toggleDetail(m.results[rowIndex])
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.updateTableLayout()
		return m, nil

	case resultMsg:
		m.currentProxy++
		m.results = append(m.results, msg.result)
		m.recordSequence(msg.result)

		// Sort results after each update
		m.sortResults()

		// Update table rows
		m.updateTableRows()

		progressCmd := m.progress.SetPercent(float64(m.currentProxy) / float64(m.totalProxies))
		return m, tea.Batch(progressCmd, m.waitForResult())

	case doneMsg:
		m.testing = false
		progressCmd := m.progress.SetPercent(1.0)
		return m, progressCmd

	case progressMsg:
		m.currentProxy = msg.current
		m.totalProxies = msg.total
		m.progress.SetPercent(float64(m.currentProxy) / float64(m.totalProxies))

		// Update progress bar
		progressModel, progressCmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmd = progressCmd

		return m, cmd

	case timerTickMsg:
		return m, timerTickCmd()
	}

	// Default: update progress bar for other messages
	progressModel, progressCmd := m.progress.Update(msg)
	m.progress = progressModel.(progress.Model)
	cmd = progressCmd

	return m, cmd
}

// View renders the TUI
func (m tuiModel) View() string {
	if m.quitting {
		return ""
	}

	// Layout: progress bar at top, table below
	tableView := m.table.View()
	detailView := m.detailPanelView(tableView)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.progressLine(),
		"",
		tableView,
	)
	if detailView != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			"",
			detailView,
		)
	}
	return content
}
