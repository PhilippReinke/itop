package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/philippreinke/itop/internal/containercli"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		if msg.kind != m.kind() {
			return m, nil
		}
		m.service = msg.service
		if msg.service == containercli.ServiceStopped {
			m.rows[msg.kind] = nil
			m.status = "Apple container service is stopped"
			m.statusError = false
			return m, nil
		}
		if msg.err != nil {
			m.rows[msg.kind] = nil
			m.status = msg.err.Error()
			m.statusError = true
		} else {
			m.rows[msg.kind] = msg.rows
			if m.cursor >= len(m.visible()) {
				m.resetSelection()
			}
			m.status = fmt.Sprintf("%d %s", len(msg.rows), msg.kind)
			m.statusError = false
			m.lastRefresh = time.Now()
		}
		return m, nil
	case actionMsg:
		m.busy = false
		if msg.err != nil {
			m.status = msg.err.Error()
			m.statusError = true
		} else {
			m.status = msg.label + " complete"
			if msg.output != "" {
				m.status += " · " + oneLine(msg.output)
			}
			m.statusError = false
		}
		return m, m.load()
	case panelMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.statusError = true
			return m, nil
		}
		m.panelTitle = msg.title
		m.panelBody = msg.body
		m.panelScroll = 0
		m.mode = "panel"
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if m.mode == "panel" {
			switch key {
			case "esc", "q", "enter":
				m.mode = ""
			case "j", "down":
				m.panelScroll++
			case "k", "up":
				if m.panelScroll > 0 {
					m.panelScroll--
				}
			case "pgdown":
				m.panelScroll += m.pageSize()
			case "pgup":
				m.panelScroll -= m.pageSize()
				if m.panelScroll < 0 {
					m.panelScroll = 0
				}
			}
			return m, nil
		}
		if m.mode == "confirm" {
			if key == "y" || key == "Y" {
				args, label := m.pending, m.pendingLabel
				m.mode = ""
				m.pending = nil
				return m, m.beginAction(args, label, false)
			}
			if key == "n" || key == "esc" {
				m.mode = ""
				m.pending = nil
			}
			return m, nil
		}
		if m.mode == "filter" || m.mode == "command" || m.mode == "run" || m.mode == "pull" || m.mode == "create" {
			switch key {
			case "esc":
				m.mode = ""
				m.input = ""
				return m, nil
			case "backspace", "ctrl+h":
				if len(m.input) > 0 {
					_, n := utf8.DecodeLastRuneInString(m.input)
					m.input = m.input[:len(m.input)-n]
				}
				if m.mode == "filter" {
					m.filter = m.input
					m.resetSelection()
				}
				return m, nil
			case "enter":
				mode, value := m.mode, strings.TrimSpace(m.input)
				m.mode = ""
				m.input = ""
				if mode == "filter" {
					m.filter = value
					m.resetSelection()
					return m, nil
				}
				if mode == "command" {
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
					return m, m.beginAction([]string{strings.TrimSuffix(m.kind(), "s"), "create", value}, "Create "+value, false)
				}
				return m, nil
			default:
				if key == " " || key == "space" {
					m.input += " "
				} else if msg.Type == tea.KeyRunes {
					m.input += string(msg.Runes)
				}
				if m.mode == "filter" {
					m.filter = m.input
					m.resetSelection()
				}
				return m, nil
			}
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.panelTitle = "Help"
			m.panelBody = helpText
			m.panelScroll = 0
			m.mode = "panel"
			return m, nil
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "g", "home":
			m.resetSelection()
		case "G", "end":
			m.cursor = len(m.visible()) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.offset = m.cursor - m.pageSize() + 1
			if m.offset < 0 {
				m.offset = 0
			}
		case "pgdown":
			m.move(m.pageSize())
		case "pgup":
			m.move(-m.pageSize())
		case "r":
			return m, m.load()
		case "S":
			return m, m.startService()
		case "/":
			m.mode = "filter"
			m.input = m.filter
		case ":":
			m.mode = "command"
			m.input = ""
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.resetSelection()
			} else if len(m.history) > 0 {
				m.view = m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
				m.resetSelection()
				return m, m.load()
			}
		case "n":
			if m.kind() == "containers" {
				m.prompt("run")
			} else if m.kind() == "images" {
				m.prompt("pull")
			} else {
				m.prompt("create")
			}
		case "enter", "i":
			if m.service == containercli.ServiceStopped {
				return m, m.startService()
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.panel(containercli.InspectArgs(m.kind(), r.ID), "Inspect · "+r.Name)
		case "v":
			if m.kind() != "containers" {
				return m, nil
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.panel([]string{"stats", "--format", "json", "--no-stream", r.ID}, "Stats · "+r.Name)
		case "l":
			if m.kind() != "containers" {
				return m, nil
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.panel([]string{"logs", "-n", "300", r.ID}, "Logs · "+r.Name)
		case "s":
			if m.service == containercli.ServiceStopped {
				return m, m.startService()
			}
			if m.kind() != "containers" {
				return m, nil
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.beginAction([]string{"start", r.ID}, "Start "+r.Name, false)
		case "x":
			if m.kind() != "containers" {
				return m, nil
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.beginAction([]string{"stop", r.ID}, "Stop "+r.Name, true)
		case "d":
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			return m, m.beginAction(containercli.DeleteArgs(m.kind(), r.ID), "Delete "+r.Name, true)
		case "e":
			if m.kind() != "containers" {
				return m, nil
			}
			r, err := m.selected()
			if err != nil {
				return m, nil
			}
			cmd := exec.Command("container", "exec", "-it", r.ID, "/bin/sh")
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return actionMsg{"Shell " + r.Name, "", err} })
		}
	}
	return m, nil
}
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 100 {
		return s[:100] + "…"
	}
	return s
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
	cmd := strings.ToLower(strings.TrimSpace(input))
	switch cmd {
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
		return nil
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
		return nil
	}
}
