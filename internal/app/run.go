package app

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/philippreinke/itop/internal/containercli"
	"github.com/philippreinke/itop/internal/ui"
)

// Run starts the itop command with the provided arguments.
func Run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h", "help":
			fmt.Println(`itop — Apple container TUI

Usage: itop
       itop --help

Inside the TUI, press ? for all keybindings.`)
			return nil
		default:
			return fmt.Errorf("unknown argument %q (try --help)", args[0])
		}
	}
	if _, err := exec.LookPath("container"); err != nil {
		return fmt.Errorf("Apple container CLI was not found in PATH: %w", err)
	}
	program := tea.NewProgram(ui.NewModel(containercli.CLI{}), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func Main() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "itop:", err)
		os.Exit(1)
	}
}
