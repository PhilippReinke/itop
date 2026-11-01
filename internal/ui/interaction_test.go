package ui

import (
	"errors"
	"os"
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

func TestEmptyCommandOpensResourcePicker(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{':'},
	})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != "picker" || m.pickerCursor != m.view {
		t.Fatalf("picker mode=%q cursor=%d", m.mode, m.pickerCursor)
	}
	if !strings.Contains(m.View(), "RESOURCES") ||
		!strings.Contains(m.View(), "› CONTAINERS") {
		t.Fatal("resource picker overlay is missing")
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyDown})
	m, command := press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != "" || m.kind() != "images" || command == nil {
		t.Fatalf("picker did not open images: mode=%q kind=%s",
			m.mode, m.kind())
	}
}

func TestResourcePickerCancelAndTypedCommand(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{':'},
	})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != "" || m.kind() != "containers" {
		t.Fatal("Escape did not close the picker")
	}
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{':'},
	})
	for _, r := range "volumes" {
		m, _ = press(m, tea.KeyMsg{
			Type: tea.KeyRunes, Runes: []rune{r},
		})
	}
	m, command := press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != "" || m.kind() != "volumes" || command == nil {
		t.Fatal("typed resource command did not switch views")
	}
}

func TestResourcePickerTabNavigation(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	for _, step := range []struct {
		key  tea.KeyType
		want int
	}{
		{tea.KeyShiftTab, 0},
		{tea.KeyTab, 1},
		{tea.KeyTab, 2},
		{tea.KeyTab, 3},
		{tea.KeyTab, 3},
		{tea.KeyShiftTab, 2},
	} {
		var command tea.Cmd
		m, command = press(m, tea.KeyMsg{Type: step.key})
		if m.mode != "picker" || m.pickerCursor != step.want || command != nil {
			t.Fatalf("key %s: mode=%q cursor=%d, want %d",
				tea.KeyMsg{Type: step.key}, m.mode, m.pickerCursor, step.want)
		}
	}
	m, command := press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != "" || m.kind() != "networks" || command == nil {
		t.Fatal("Enter did not open the resource selected with Tab")
	}
}

func TestResourcePickerFitsTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{80, 24}, {44, 14}, {30, 10},
	} {
		m := NewModel(&fakeRunner{})
		m.width, m.height = size.width, size.height
		m.mode = "picker"
		lines := strings.Split(m.View(), "\n")
		if len(lines) >= size.height {
			t.Fatalf("picker has %d rows in %d", len(lines), size.height)
		}
		for _, line := range lines {
			if lipgloss.Width(line) >= size.width {
				t.Fatalf("picker exceeds %d columns", size.width)
			}
		}
		if !strings.Contains(lines[len(lines)-4], "╰") {
			t.Fatal("picker hid the main bottom border")
		}
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

func TestImagePageUsesReferenceForDelete(t *testing.T) {
	fixture, err := os.ReadFile("../containercli/testdata/images.json")
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{
		status: []byte(`{"status":"running"}`),
		list:   fixture,
	}
	m := NewModel(runner)
	m.view = 1
	next, _ := m.Update(m.load()())
	m = next.(model)
	selected, err := m.selected()
	if err != nil || selected.Name != "alpine" ||
		selected.State != "latest" {
		t.Fatalf("image row = %+v, error = %v", selected, err)
	}
	if !strings.Contains(m.View(), "TAG") ||
		!strings.Contains(m.View(), "DIGEST") {
		t.Fatal("image columns are missing")
	}
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'d'},
	})
	m, command := press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'y'},
	})
	if command == nil {
		t.Fatal("confirmed image delete did not run")
	}
	command()
	want := "image delete docker.io/library/alpine:latest"
	if got := runner.calls[len(runner.calls)-1]; got != want {
		t.Fatalf("delete command = %q, want %q", got, want)
	}
}

func TestActionErrorSurvivesListRefresh(t *testing.T) {
	runner := &fakeRunner{
		status: []byte(`{"status":"running"}`),
		list:   []byte(`[]`),
	}
	m := NewModel(runner)
	next, refresh := m.Update(actionMsg{
		label: "Delete alpine", err: errors.New("image is in use"),
	})
	m = next.(model)
	if refresh == nil {
		t.Fatal("action did not refresh the list")
	}
	next, _ = m.Update(refresh())
	m = next.(model)
	if m.status != "image is in use" || !m.statusError {
		t.Fatalf("action error was hidden: %q", m.status)
	}
	m, _ = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.statusError || m.actionError {
		t.Fatal("Escape did not dismiss the error")
	}
}

func TestBusyActionCannotStartAnotherDelete(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.service = containercli.ServiceRunning
	m.rows["containers"] = []containercli.Resource{{ID: "web"}}
	m.busy = true
	m, command := press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'d'},
	})
	if command != nil || m.mode == "confirm" {
		t.Fatal("delete was accepted while another action was running")
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

func TestOlderRefreshOfSameViewIsIgnored(t *testing.T) {
	runner := &fakeRunner{
		status: []byte(`{"status":"running"}`),
		list:   []byte(`[{"id":"old"}]`),
	}
	m := NewModel(runner)
	older := m.load()().(listMsg)
	runner.list = []byte(`[{"id":"new"}]`)
	newer := m.load()().(listMsg)
	next, _ := m.Update(newer)
	m = next.(model)
	next, _ = m.Update(older)
	m = next.(model)
	if len(m.rows["containers"]) != 1 ||
		m.rows["containers"][0].ID != "new" {
		t.Fatalf("stale refresh replaced new rows: %+v", m.rows)
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

func TestLogBackspaceErasesWithinItsLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc\bX", "abX"},
		{"abc\b \bX", "abX"},
		{"a\n\bX", "a\nX"},
		{"ä\bX", "X"},
		{"abc\x7fX", "abX"},
	}
	for _, test := range tests {
		if got := cleanPanelText(test.input); got != test.want {
			t.Errorf("cleanPanelText(%q) = %q, want %q",
				test.input, got, test.want)
		}
	}
}

func TestLogScrollClampsAtBottom(t *testing.T) {
	m := NewModel(&fakeRunner{})
	m.mode = "panel"
	m.panelBody = strings.Repeat("line\n", 30)
	m.panelScroll = m.maxPanelScroll()
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'j'},
	})
	if m.panelScroll != m.maxPanelScroll() {
		t.Fatal("scroll moved beyond the last log line")
	}
	m, _ = press(m, tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune{'k'},
	})
	if m.panelScroll != m.maxPanelScroll()-1 {
		t.Fatal("scroll up did not move immediately")
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
