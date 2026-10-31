package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/philippreinke/itop/internal/containercli"
)

func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24}, {60, 18}, {44, 14}, {30, 10},
	}
	modes := []string{"", "command", "filter", "confirm", "panel"}
	for _, size := range sizes {
		for _, mode := range modes {
			m := NewModel(containercli.CLI{})
			m.width, m.height = size.w, size.h
			m.service = containercli.ServiceRunning
			m.mode = mode
			m.input = "images"
			m.panelBody = strings.Repeat("a long log line here\n", 30)
			m.pendingLabel = "Delete web-server"
			m.rows["containers"] = []containercli.Resource{
				{
					ID: "web", Name: "web-server", State: "running",
					Detail: "registry.example.com/team/nginx:latest",
				},
			}
			lines := strings.Split(m.View(), "\n")
			if len(lines) >= size.h {
				t.Errorf(
					"%dx%d %s: %d lines reach terminal bottom",
					size.w, size.h, mode, len(lines),
				)
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got >= size.w {
					t.Errorf(
						"%dx%d %s line %d width %d reaches right edge",
						size.w, size.h, mode, i, got,
					)
				}
			}
			if size.h >= 14 &&
				!strings.Contains(lines[len(lines)-1], "q quit") {
				t.Errorf("%dx%d %s: footer missing", size.w, size.h, mode)
			}
		}
	}
}

func TestCommandSwitchesView(t *testing.T) {
	m := NewModel(containercli.CLI{})
	command := m.command("images")
	if m.view != 1 || command == nil {
		t.Fatalf("view=%d command=%v", m.view, command)
	}
}

func TestCommandHistoryAndNoTabs(t *testing.T) {
	m := NewModel(containercli.CLI{})
	m.command("images")
	m.command("volumes")
	if len(m.history) != 2 {
		t.Fatalf("history=%v", m.history)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if m.kind() != "images" {
		t.Fatalf("after escape: %s", m.kind())
	}
	view := m.View()
	if strings.Contains(view, "1 CONTAINERS") ||
		strings.Contains(view, "tab switch") {
		t.Fatal("tab navigation remains in view")
	}
}

type fakeRunner struct {
	calls     []string
	status    []byte
	statusErr error
	list      []byte
	listErr   error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	if len(args) > 0 && args[0] == "system" {
		return f.status, f.statusErr
	}
	return f.list, f.listErr
}

func TestStoppedServiceOffersStart(t *testing.T) {
	runner := &fakeRunner{
		statusErr: errors.New("apiserver is not running"),
	}
	m := NewModel(runner)
	message := m.load()().(listMsg)
	if message.service != containercli.ServiceStopped ||
		message.err != nil || len(runner.calls) != 1 {
		t.Fatalf("message=%+v calls=%v", message, runner.calls)
	}
	next, _ := m.Update(message)
	m = next.(model)
	if !strings.Contains(m.View(), "[ Enter / s ] Start service") {
		t.Fatal("start action not shown")
	}
	next, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if command == nil || !m.busy {
		t.Fatal("Enter did not start service")
	}
}

func TestServiceCheckDistinguishesErrors(t *testing.T) {
	running := &fakeRunner{
		status: []byte(`{"status":"running"}`),
		list:   []byte(`[{"id":"web","name":"web"}]`),
	}
	message := NewModel(running).load()().(listMsg)
	if message.service != containercli.ServiceRunning ||
		message.err != nil || len(message.rows) != 1 ||
		len(running.calls) != 2 {
		t.Fatalf("running: %+v calls=%v", message, running.calls)
	}

	denied := &fakeRunner{statusErr: errors.New("Operation not permitted")}
	message = NewModel(denied).load()().(listMsg)
	if message.service != containercli.ServiceUnknown || message.err == nil {
		t.Fatalf("permission error classified incorrectly: %+v", message)
	}
}
