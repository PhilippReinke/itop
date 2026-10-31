package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/philippreinke/itop/internal/containercli"
)

func press(m model, key tea.KeyMsg) (model, tea.Cmd) {
	next, command := m.Update(key)
	return next.(model), command
}

func TestFilterEscapeRestoresPreviousFilter(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.rows["containers"] = []containercli.Resource{
		{ID: "web"}, {ID: "database"},
	}
	m.filter = "web"
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if len(m.visible()) != 0 {
		t.Fatal("filter input did not update visible rows")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != "" || m.filter != "web" || len(m.visible()) != 1 {
		t.Fatalf(
			"cancel left mode=%q filter=%q rows=%v",
			m.mode, m.filter, m.visible(),
		)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	runner := &fakeRunner{}
	m := NewModel(runner)
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{{ID: "web", Name: "web"}}
	m, command := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.mode != "confirm" || command != nil || len(runner.calls) != 0 {
		t.Fatal("delete ran before confirmation")
	}
	m, command = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.mode != "" || command != nil || len(runner.calls) != 0 {
		t.Fatal("cancel executed delete")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m, command = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.busy || command == nil {
		t.Fatal("confirmed delete did not start")
	}
	command()
	if !reflect.DeepEqual(runner.calls, []string{"delete web"}) {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestLateListFromPreviousViewIsIgnored(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.command("images")
	message := listMsg{
		kind:    "containers",
		rows:    []containercli.Resource{{ID: "old"}},
		service: containercli.ServiceRunning,
	}
	next, _ := m.Update(message)
	m = next.(model)
	if m.service != containercli.ServiceChecking ||
		len(m.rows["containers"]) != 0 {
		t.Fatalf("late list changed current view: %+v", m)
	}
}

func TestLogPanelKeepsBottomBorder(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{{ID: "web"}}
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'l'},
	})
	log := strings.Repeat("long log line\n", 50)
	log += "progress\rnext\x1b[2J\x1b[H\x1b]0;title\aend\n"
	next, _ := m.Update(panelMsg{id: m.panelID, body: log})
	m = next.(model)

	if strings.ContainsAny(m.panelBody, "\r\x1b\a") {
		t.Fatalf("terminal controls remain in panel: %q", m.panelBody)
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) >= m.height {
		t.Fatalf("panel uses %d rows in a %d row terminal", len(lines), m.height)
	}
	if !strings.Contains(lines[len(lines)-4], "╰") {
		t.Fatal("bottom border is missing")
	}
	for _, line := range lines {
		if lipgloss.Width(line) >= m.width {
			t.Fatalf("panel line exceeds terminal width: %q", line)
		}
	}
}

func TestLogsRefreshWhileOpen(t *testing.T) {
	runner := &fakeRunner{list: []byte(strings.Repeat("old\n", 30))}
	m := NewModel(runner)
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{{ID: "web"}}
	m, first := press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'l'},
	})
	if m.mode != "panel" || first == nil {
		t.Fatal("logs panel did not open")
	}
	next, _ := m.Update(first())
	m = next.(model)
	if m.panelScroll == 0 || !strings.Contains(m.panelBody, "old") {
		t.Fatal("initial logs did not open at the newest lines")
	}

	runner.list = []byte(strings.Repeat("old\n", 30) + "new\n")
	next, refresh := m.Update(tickMsg{})
	m = next.(model)
	if !m.panelLoading || refresh == nil {
		t.Fatal("open logs did not schedule a refresh")
	}
	commands := refresh().(tea.BatchMsg)
	next, _ = m.Update(commands[0]())
	m = next.(model)
	if m.panelLoading || !strings.Contains(m.panelBody, "new") {
		t.Fatal("refreshed logs were not displayed")
	}
	if m.panelScroll == 0 {
		t.Fatal("new log lines did not stay in view")
	}
	if !reflect.DeepEqual(runner.calls, []string{
		"logs -n 300 web", "logs -n 300 web",
	}) {
		t.Fatalf("unexpected commands: %v", runner.calls)
	}

	m.panelScroll = 0
	runner.list = []byte(strings.Repeat("old\n", 30) + "new\nnewer\n")
	next, refresh = m.Update(tickMsg{})
	m = next.(model)
	commands = refresh().(tea.BatchMsg)
	next, _ = m.Update(commands[0]())
	m = next.(model)
	if m.panelScroll != 0 {
		t.Fatal("refresh moved the user's scroll position")
	}
}

func TestClosedLogPanelIgnoresLateRefresh(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{{ID: "web"}}
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'l'},
	})
	id := m.panelID
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	next, _ := m.Update(panelMsg{id: id, body: "late log"})
	m = next.(model)
	if m.mode != "" || strings.Contains(m.panelBody, "late log") {
		t.Fatal("late refresh reopened the closed logs panel")
	}
}

func TestShellExitRequestsFullRedraw(t *testing.T) {
	runner := &fakeRunner{
		status: []byte(`{"status":"running"}`),
		list:   []byte(`[{"id":"web","name":"web"}]`),
	}
	m := NewModel(runner)
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{
		{ID: "web", Name: "web"},
	}
	m, command := press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'e'},
	})
	if command == nil || !m.busy {
		t.Fatal("shell did not mark the terminal as busy")
	}

	next, redraw := m.Update(actionMsg{
		label: "Shell web", external: true,
	})
	m = next.(model)
	if m.busy || !strings.Contains(m.View(), "╰") {
		t.Fatal("shell exit did not restore the list view")
	}
	commands := reflect.ValueOf(redraw())
	if commands.Kind() != reflect.Slice || commands.Len() != 2 {
		t.Fatal("shell exit did not request clear and refresh")
	}
	clear := commands.Index(0).Interface().(tea.Cmd)
	if reflect.TypeOf(clear()) != reflect.TypeOf(tea.ClearScreen()) {
		t.Fatal("shell exit did not clear the terminal first")
	}
	refresh := commands.Index(1).Interface().(tea.Cmd)
	if message, ok := refresh().(listMsg); !ok ||
		message.service != containercli.ServiceRunning {
		t.Fatal("shell exit did not refresh the list")
	}
}
