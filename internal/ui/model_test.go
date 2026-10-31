package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/philippreinke/itop/internal/containercli"
)

func TestViewFitsTerminal(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {60, 18}, {44, 14}, {30, 10}} {
		for _, mode := range []string{"", "command", "filter", "confirm", "panel"} {
			m := NewModel(containercli.CLI{})
			m.width, m.height = size.w, size.h
			m.service = containercli.ServiceRunning
			m.mode = mode
			m.input = "images"
			m.panelBody = strings.Repeat("a long log line here\n", 30)
			m.pendingLabel = "Delete web-server"
			m.rows["containers"] = []containercli.Resource{{ID: "web", Name: "web-server", State: "running", Detail: "registry.example.com/team/nginx:latest"}}
			lines := strings.Split(m.View(), "\n")
			if len(lines) >= size.h {
				t.Errorf("%dx%d %s: %d lines reach terminal bottom", size.w, size.h, mode, len(lines))
			}
			for i, line := range lines {
				if got := lipgloss.Width(line); got >= size.w {
					t.Errorf("%dx%d %s line %d width %d reaches right edge", size.w, size.h, mode, i, got)
				}
			}
			if size.h >= 14 && !strings.Contains(lines[len(lines)-1], "q quit") {
				t.Errorf("%dx%d %s: footer missing", size.w, size.h, mode)
			}
		}
	}
}

func TestCommandSwitchesView(t *testing.T) {
	m := NewModel(containercli.CLI{})
	cmd := m.command("images")
	if m.view != 1 || cmd == nil {
		t.Fatalf("view=%d cmd=%v", m.view, cmd)
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
	if strings.Contains(view, "1 CONTAINERS") || strings.Contains(view, "tab switch") {
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
	f := &fakeRunner{statusErr: errors.New("apiserver is not running and not registered with launchd")}
	m := NewModel(f)
	msg := m.load()().(listMsg)
	if msg.service != containercli.ServiceStopped || msg.err != nil || len(f.calls) != 1 {
		t.Fatalf("msg=%+v calls=%v", msg, f.calls)
	}
	next, _ := m.Update(msg)
	m = next.(model)
	if !strings.Contains(m.View(), "[ Enter / s ] Start service") {
		t.Fatal("start action not shown")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if cmd == nil || !m.busy {
		t.Fatal("Enter did not start service")
	}
}
func TestServiceCheckDistinguishesErrors(t *testing.T) {
	running := &fakeRunner{status: []byte(`{"status":"running"}`), list: []byte(`[{"id":"web","name":"web"}]`)}
	msg := NewModel(running).load()().(listMsg)
	if msg.service != containercli.ServiceRunning || msg.err != nil || len(msg.rows) != 1 || len(running.calls) != 2 {
		t.Fatalf("running: %+v calls=%v", msg, running.calls)
	}
	denied := &fakeRunner{statusErr: errors.New("Operation not permitted")}
	msg = NewModel(denied).load()().(listMsg)
	if msg.service != containercli.ServiceUnknown || msg.err == nil {
		t.Fatalf("permission error classified incorrectly: %+v", msg)
	}
}

func TestLegacyStatusJSONWithEmptyAPIServer(t *testing.T) {
	stopped := &fakeRunner{
		status:    []byte("{\"apiServerAppName\":\"\",\"apiServerBuild\":\"\",\"apiServerCommit\":\"\",\"apiServerVersion\":\"\",\"status\":\"unregistered\"}\nError: exit status 1"),
		statusErr: errors.New("exit status 1"),
	}
	msg := NewModel(stopped).load()().(listMsg)
	if msg.service != containercli.ServiceStopped || msg.err != nil || len(stopped.calls) != 1 {
		t.Fatalf("stopped: %+v calls=%v", msg, stopped.calls)
	}
	active := &fakeRunner{
		status: []byte(`{"apiServerAppName":"container-apiserver","apiServerVersion":"1.0.0"}`),
		list:   []byte(`[]`),
	}
	msg = NewModel(active).load()().(listMsg)
	if msg.service != containercli.ServiceRunning || msg.err != nil || len(active.calls) != 2 {
		t.Fatalf("active: %+v calls=%v", msg, active.calls)
	}
}

func TestStatusCommandJSONOnFailure(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "container")
	contents := "#!/bin/sh\nprintf '%s\\n' '{\"apiServerAppName\":\"\",\"apiServerVersion\":\"\",\"status\":\"not running\"}'\nexit 1\n"
	if err := os.WriteFile(script, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	state, err := containercli.CheckService(containercli.CLI{})
	if state != containercli.ServiceStopped || err != nil {
		t.Fatalf("state=%v err=%v", state, err)
	}
}
