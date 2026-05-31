// Command tailTUI is a brutalist, keyboard-centric terminal UI for managing a
// Tailscale tailnet, built on the Charmbracelet stack (bubbletea, lipgloss,
// bubbles). See internal/tui for the Elm-style Model/Update/View implementation.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/tui"
)

func main() {
	// Load the theme (native Omarchy palette if present, else the built-in
	// "Matrix Core" default) and apply it before building any styles.
	styles.Apply(styles.LoadTheme())

	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tailTUI: %v\n", err)
		os.Exit(1)
	}
}
