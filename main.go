// Command tailscaleTUI is a lightweight terminal UI for managing a Tailscale
// tailnet, built on the Charmbracelet stack (bubbletea, lipgloss, bubbles).
//
// Phase 1: a static, mock-data layout. See internal/tui for the Elm-style
// Model/Update/View implementation.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailscaleTUI/internal/styles"
	"github.com/Phundahl/tailscaleTUI/internal/tui"
)

func main() {
	// Load the theme (user override at ~/.config/tailscale-tui/theme.json, or
	// the default Stitch theme) and apply it before building any styles.
	styles.Apply(styles.LoadTheme())

	p := tea.NewProgram(tui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tailscaleTUI: %v\n", err)
		os.Exit(1)
	}
}
