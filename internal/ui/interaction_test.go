package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
		t.Fatalf("cancel left mode=%q filter=%q rows=%v", m.mode, m.filter, m.visible())
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
	if m.service != containercli.ServiceChecking || len(m.rows["containers"]) != 0 {
		t.Fatalf("late list changed current view: %+v", m)
	}
}
