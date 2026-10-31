package ui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/philippreinke/itop/internal/containercli"
)

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if !m.busy && m.mode == "" {
			return m, tea.Batch(m.load(), tick())
		}
		return m, tick()
	case listMsg:
		return m.handleList(msg), nil
	case actionMsg:
		m.handleAction(msg)
		return m, m.load()
	case panelMsg:
		m.handlePanel(msg)
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m model) handleList(msg listMsg) model {
	if msg.kind != m.kind() {
		return m
	}

	m.service = msg.service
	if msg.service == containercli.ServiceStopped {
		m.rows[msg.kind] = nil
		m.status = "Apple container service is stopped"
		m.statusError = false
		return m
	}
	if msg.err != nil {
		m.rows[msg.kind] = nil
		m.status = msg.err.Error()
		m.statusError = true
		return m
	}

	m.rows[msg.kind] = msg.rows
	if m.cursor >= len(m.visible()) {
		m.resetSelection()
	}
	m.status = fmt.Sprintf("%d %s", len(msg.rows), msg.kind)
	m.statusError = false
	m.lastRefresh = time.Now()
	return m
}

func (m *model) handleAction(msg actionMsg) {
	m.busy = false
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusError = true
		return
	}

	m.status = msg.label + " complete"
	if msg.output != "" {
		m.status += " · " + oneLine(msg.output)
	}
	m.statusError = false
}

func (m *model) handlePanel(msg panelMsg) {
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusError = true
		return
	}
	m.panelTitle = msg.title
	m.panelBody = msg.body
	m.panelScroll = 0
	m.mode = "panel"
}

func (m model) handleKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch m.mode {
	case "panel":
		return m.handlePanelKey(msg.String()), nil
	case "confirm":
		return m.handleConfirmKey(msg.String())
	case "filter", "command", "run", "pull", "create":
		return m.handleInputKey(msg)
	default:
		return m.handleNormalKey(msg.String())
	}
}

func (m model) handlePanelKey(key string) model {
	switch key {
	case "esc", "q", "enter":
		m.mode = ""
	case "j", "down":
		m.panelScroll++
	case "k", "up":
		m.panelScroll = max(0, m.panelScroll-1)
	case "pgdown":
		m.panelScroll += max(1, m.pageSize())
	case "pgup":
		m.panelScroll = max(0, m.panelScroll-max(1, m.pageSize()))
	}
	return m
}

func (m model) handleConfirmKey(key string) (model, tea.Cmd) {
	switch key {
	case "y", "Y":
		args, label := m.pending, m.pendingLabel
		m.mode = ""
		m.pending = nil
		return m, m.beginAction(args, label, false)
	case "n", "esc":
		m.mode = ""
		m.pending = nil
	}
	return m, nil
}

func (m model) handleInputKey(msg tea.KeyMsg) (model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		if m.mode == "filter" {
			m.filter = m.filterBeforeInput
			m.resetSelection()
		}
		m.mode = ""
		m.input = ""
	case "backspace", "ctrl+h":
		if len(m.input) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.input)
			m.input = m.input[:len(m.input)-size]
		}
	case "enter":
		return m.submitInput()
	case " ", "space":
		m.input += " "
	default:
		if msg.Type == tea.KeyRunes {
			m.input += string(msg.Runes)
		}
	}
	if m.mode == "filter" {
		m.filter = m.input
		m.resetSelection()
	}
	return m, nil
}

func (m model) submitInput() (model, tea.Cmd) {
	mode := m.mode
	value := strings.TrimSpace(m.input)
	m.mode = ""
	m.input = ""

	switch mode {
	case "filter":
		m.filter = value
		m.resetSelection()
		return m, nil
	case "command":
		return m, m.command(value)
	}
	if value == "" {
		return m, nil
	}

	switch mode {
	case "run":
		return m, m.beginAction([]string{"run", "-d", value}, "Run "+value, false)
	case "pull":
		return m, m.beginAction([]string{"image", "pull", "--progress", "plain", value}, "Pull "+value, false)
	case "create":
		kind := strings.TrimSuffix(m.kind(), "s")
		return m, m.beginAction([]string{kind, "create", value}, "Create "+value, false)
	default:
		return m, nil
	}
}

func (m model) handleNormalKey(key string) (model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.panelTitle = "Help"
		m.panelBody = helpText
		m.panelScroll = 0
		m.mode = "panel"
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g", "home":
		m.resetSelection()
	case "G", "end":
		m.cursor = max(0, len(m.visible())-1)
		m.offset = max(0, m.cursor-max(1, m.pageSize())+1)
	case "pgdown":
		m.move(max(1, m.pageSize()))
	case "pgup":
		m.move(-max(1, m.pageSize()))
	case "r":
		return m, m.load()
	case "S":
		return m, m.startService()
	case "/":
		m.filterBeforeInput = m.filter
		m.mode = "filter"
		m.input = m.filter
	case ":":
		m.prompt("command")
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.resetSelection()
		} else if len(m.history) > 0 {
			last := len(m.history) - 1
			m.view = m.history[last]
			m.history = m.history[:last]
			m.resetSelection()
			return m, m.load()
		}
	case "n":
		switch m.kind() {
		case "containers":
			m.prompt("run")
		case "images":
			m.prompt("pull")
		default:
			m.prompt("create")
		}
	case "enter", "i":
		if m.service == containercli.ServiceStopped {
			return m, m.startService()
		}
		row, err := m.selected()
		if err == nil {
			return m, m.panel(containercli.InspectArgs(m.kind(), row.ID), "Inspect · "+row.Name)
		}
	case "v", "l", "s", "x", "e":
		return m.handleContainerAction(key)
	case "d":
		row, err := m.selected()
		if err == nil {
			return m, m.beginAction(containercli.DeleteArgs(m.kind(), row.ID), "Delete "+row.Name, true)
		}
	}
	return m, nil
}

func (m model) handleContainerAction(key string) (model, tea.Cmd) {
	if key == "s" && m.service == containercli.ServiceStopped {
		return m, m.startService()
	}
	if m.kind() != "containers" {
		return m, nil
	}
	row, err := m.selected()
	if err != nil {
		return m, nil
	}

	switch key {
	case "v":
		return m, m.panel([]string{"stats", "--format", "json", "--no-stream", row.ID}, "Stats · "+row.Name)
	case "l":
		return m, m.panel([]string{"logs", "-n", "300", row.ID}, "Logs · "+row.Name)
	case "s":
		return m, m.beginAction([]string{"start", row.ID}, "Start "+row.Name, false)
	case "x":
		return m, m.beginAction([]string{"stop", row.ID}, "Stop "+row.Name, true)
	case "e":
		cmd := exec.Command("container", "exec", "-it", row.ID, "/bin/sh")
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return actionMsg{label: "Shell " + row.Name, err: err}
		})
	default:
		return m, nil
	}
}

func oneLine(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	runes := []rune(text)
	if len(runes) > 100 {
		return string(runes[:100]) + "…"
	}
	return text
}

func (m *model) switchView(next int) tea.Cmd {
	if next != m.view {
		m.history = append(m.history, m.view)
		m.view = next
	}
	m.filter = ""
	m.resetSelection()
	return m.load()
}

func (m *model) command(input string) tea.Cmd {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "c", "containers", "container", "ls":
		return m.switchView(0)
	case "i", "images", "image":
		return m.switchView(1)
	case "n", "networks", "network":
		return m.switchView(2)
	case "v", "volumes", "volume":
		return m.switchView(3)
	case "help", "?":
		m.panelTitle = "Help"
		m.panelBody = helpText
		m.panelScroll = 0
		m.mode = "panel"
	case "q", "quit", "exit":
		return tea.Quit
	case "r", "refresh":
		return m.load()
	case "system start":
		return m.startService()
	case "":
		return nil
	default:
		m.status = "Unknown command: " + input
		m.statusError = true
	}
	return nil
}
