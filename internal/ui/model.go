package ui

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/philippreinke/itop/internal/containercli"
)

var (
	errNoSelection = errors.New("select a resource first")
	kinds          = []string{"containers", "images", "networks", "volumes"}
)

type listMsg struct {
	kind    string
	rows    []containercli.Resource
	service containercli.ServiceState
	err     error
}

type actionMsg struct {
	label  string
	output string
	err    error
}

type panelMsg struct {
	title string
	body  string
	err   error
}

type tickMsg struct{}

type model struct {
	cli containercli.Runner

	view              int
	history           []int
	rows              map[string][]containercli.Resource
	cursor            int
	offset            int
	width             int
	height            int
	filter            string
	filterBeforeInput string
	mode              string
	input             string
	panelTitle        string
	panelBody         string
	panelScroll       int

	pending      []string
	pendingLabel string
	busy         bool
	status       string
	statusError  bool
	service      containercli.ServiceState
	lastRefresh  time.Time
}

func NewModel(cli containercli.Runner) model {
	return model{
		cli:    cli,
		rows:   make(map[string][]containercli.Resource),
		width:  80,
		height: 24,
		status: "Loading containers…",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.load(), tick())
}

func tick() tea.Cmd {
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) kind() string {
	return kinds[m.view]
}

func (m model) load() tea.Cmd {
	kind := m.kind()
	cli := m.cli
	return func() tea.Msg {
		service, err := containercli.CheckService(cli)
		if service != containercli.ServiceRunning {
			return listMsg{kind: kind, service: service, err: err}
		}

		rows, err := containercli.List(cli, kind)
		return listMsg{kind: kind, rows: rows, service: service, err: err}
	}
}

func (m model) selected() (containercli.Resource, error) {
	rows := m.visible()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return containercli.Resource{}, errNoSelection
	}
	return rows[m.cursor], nil
}

func (m model) visible() []containercli.Resource {
	rows := m.rows[m.kind()]
	if m.filter == "" {
		return rows
	}

	needle := strings.ToLower(m.filter)
	filtered := make([]containercli.Resource, 0, len(rows))
	for _, row := range rows {
		text := row.ID + " " + row.Name + " " + row.State + " " + row.Detail
		if strings.Contains(strings.ToLower(text), needle) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (m *model) resetSelection() {
	m.cursor = 0
	m.offset = 0
}

func (m *model) move(delta int) {
	count := len(m.visible())
	if count == 0 {
		return
	}

	m.cursor = max(0, min(count-1, m.cursor+delta))
	page := max(1, m.pageSize())
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+page {
		m.offset = m.cursor - page + 1
	}
}

func (m model) pageSize() int {
	return max(0, m.height-10)
}

func (m model) operation(args []string, label string) tea.Cmd {
	cli := m.cli
	return func() tea.Msg {
		output, err := containercli.Run(cli, args...)
		return actionMsg{label: label, output: output, err: err}
	}
}

func (m model) panel(args []string, title string) tea.Cmd {
	cli := m.cli
	return func() tea.Msg {
		output, err := containercli.Run(cli, args...)
		return panelMsg{title: title, body: containercli.PrettyJSON(output), err: err}
	}
}

func (m *model) beginAction(args []string, label string, confirm bool) tea.Cmd {
	if confirm {
		m.pending = args
		m.pendingLabel = label
		m.mode = "confirm"
		return nil
	}

	m.busy = true
	m.status = label + "…"
	return m.operation(args, label)
}

func (m *model) startService() tea.Cmd {
	m.busy = true
	m.status = "Starting Apple container service…"
	cmd := exec.Command("container", "system", "start")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionMsg{label: "Start container service", err: err}
	})
}

func (m *model) prompt(kind string) {
	m.mode = kind
	m.input = ""
}
