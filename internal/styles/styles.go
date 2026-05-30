// Package styles centralizes all lipgloss styling for the TUI.
//
// THEME CONSTRAINT: colors are expressed exclusively as ANSI 16-color codes
// (lipgloss.Color("0") .. lipgloss.Color("15")). No hex values are used, so the
// UI inherits and follows the user's terminal color scheme automatically.
//
//	ANSI reference: 0 black 1 red 2 green 3 yellow 4 blue 5 magenta 6 cyan
//	7 white  8-15 = bright variants of 0-7.
package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Semantic palette. Map meaning -> ANSI code in one place so a theme tweak is
// a single-line change and the rest of the code stays intention-revealing.
var (
	Primary   = lipgloss.Color("10") // bright green  — "Matrix Core" accent
	Secondary = lipgloss.Color("2")  // green         — borders / online
	Subtle    = lipgloss.Color("8")  // bright black  — dimmed text
	Warn      = lipgloss.Color("3")  // yellow        — disabled / degraded
	Danger    = lipgloss.Color("1")  // red           — conflict / error
	Fg        = lipgloss.Color("15") // bright white  — primary text
	Bg        = lipgloss.Color("0")  // black         — selection foreground
	ModalBg   = lipgloss.Color("8")  // dark gray     — solid overlay backdrop (distinct from terminal bg)
)

// Modal content styles. Every span bakes in ModalBg so a modal renders as a
// 100% opaque rectangle — there are no fg-only spans whose reset would let the
// background bleed through. Do NOT use Subtle ("8") for text here; it equals
// ModalBg. Dimming is done with Faint instead.
var (
	ModalTitle   = lipgloss.NewStyle().Foreground(Primary).Background(ModalBg).Bold(true)
	ModalHeading = lipgloss.NewStyle().Foreground(Secondary).Background(ModalBg).Bold(true)
	ModalText    = lipgloss.NewStyle().Foreground(Fg).Background(ModalBg)
	ModalKey     = lipgloss.NewStyle().Foreground(Primary).Background(ModalBg)
	ModalAccent  = lipgloss.NewStyle().Foreground(Secondary).Background(ModalBg) // arrows, dividers
	ModalDim     = lipgloss.NewStyle().Foreground(Fg).Background(ModalBg).Faint(true)
)

// ModalFill returns a style that paints content of the given width with the
// modal background, padding the remainder of the line so it is fully opaque.
func ModalFill(width int) lipgloss.Style {
	return lipgloss.NewStyle().Width(width).Background(ModalBg)
}

// Reusable text styles.
var (
	Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	Heading = lipgloss.NewStyle().Foreground(Secondary).Bold(true)

	Label = lipgloss.NewStyle().Foreground(Subtle)

	Value = lipgloss.NewStyle().Foreground(Fg)

	Dim = lipgloss.NewStyle().Foreground(Subtle)

	// Selected highlights the active list row with a high-contrast green block.
	Selected = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Primary).
			Bold(true)

	Online  = lipgloss.NewStyle().Foreground(Secondary)
	Offline = lipgloss.NewStyle().Foreground(Subtle)
	Badge   = lipgloss.NewStyle().Foreground(Warn)

	// ExitChip is the high-visibility marker for the active exit node in the
	// list: black text on a yellow block (ANSI 3) so it pops against both
	// normal rows and the green selection bar.
	ExitChip = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Warn).
			Bold(true)

	// ExitName highlights the active exit node's hostname in unselected rows
	// and in the dashboard.
	ExitName = lipgloss.NewStyle().Foreground(Warn).Bold(true)
)

// Box returns a bordered container style sized to the given OUTER width/height
// (border included). Content is given width-2 x height-2 to fit inside.
func Box(width, height int) lipgloss.Style {
	w, h := width-2, height-2
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Width(w).
		Height(h)
}

// Divider returns a full-width horizontal rule for use inside a pane.
func Divider(width int) string {
	if width < 0 {
		width = 0
	}
	return lipgloss.NewStyle().Foreground(Secondary).Render(strings.Repeat("─", width))
}

// Bar renders a single-line status bar with left- and right-justified text
// padded to the given width.
func Bar(width int, left, right string) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// sparkChars maps a 0..7 bucket to an ascending block glyph.
var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a slice of values as a unicode block sparkline.
func Sparkline(values []int) string {
	if len(values) == 0 {
		return ""
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	var b strings.Builder
	for _, v := range values {
		idx := 0
		if span > 0 {
			idx = (v - min) * (len(sparkChars) - 1) / span
		}
		b.WriteRune(sparkChars[idx])
	}
	return lipgloss.NewStyle().Foreground(Primary).Render(b.String())
}
