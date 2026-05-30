// Package styles centralizes all lipgloss styling for the TUI.
//
// Colors come from a central Theme (see theme.go). The package keeps a set of
// derived style/color vars that are (re)built by Apply; call Apply once at
// startup with the loaded theme. An init() applies DefaultTheme so the package
// is always usable even before main wires up the loader.
//
// Note on Background: the theme defines a canvas color, but the app renders on
// the terminal's native background. Painting a full-screen background reliably
// across nested lipgloss blocks is fragile and would complicate the modal
// compositor; the Stitch aesthetic is carried by foreground colors instead.
package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Active palette, populated by Apply from the current Theme.
var (
	Primary        lipgloss.Color // accent / focus
	Secondary      lipgloss.Color // online / routes
	Subtle         lipgloss.Color // dim text
	Warn           lipgloss.Color // exit node / degraded
	Danger         lipgloss.Color // conflict / error
	Fg             lipgloss.Color // primary text
	Bg             lipgloss.Color // theme canvas color (see package note)
	BorderInactive lipgloss.Color // unfocused borders / dividers
)

// Derived text styles, populated by Apply.
var (
	Title, Heading, Label, Value, Dim                     lipgloss.Style
	Selected, SelectBar, Online, Offline, Badge, ExitName lipgloss.Style

	// Icon colors list glyphs by reachability, so node icons follow the theme
	// instead of inheriting the terminal's default foreground.
	IconOnline, IconOffline lipgloss.Style

	// Modal surface styles bake in the theme Background so an overlay renders as
	// a fully opaque rectangle — every span paints the background, leaving no
	// fg-only gaps for the view behind it to bleed through. See overlay.go.
	ModalTitle, ModalHeading, ModalText, ModalKey, ModalAccent, ModalDim lipgloss.Style
)

func init() { Apply(DefaultTheme()) }

// Apply rebuilds every package-level color and style from the given theme.
// Funcs like Box/Divider/LatencyGraph read these vars at call time, so they
// pick up the new theme automatically after Apply.
func Apply(t Theme) {
	Primary = t.PrimaryAccent
	Secondary = t.SecondaryAccent
	Subtle = t.TextDim
	Warn = t.Warning
	Danger = t.Error
	Fg = t.TextNormal
	Bg = t.Background
	BorderInactive = t.BorderInactive

	Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Heading = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
	Label = lipgloss.NewStyle().Foreground(Subtle)
	Value = lipgloss.NewStyle().Foreground(Fg)
	Dim = lipgloss.NewStyle().Foreground(Subtle)
	Selected = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	SelectBar = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Online = lipgloss.NewStyle().Foreground(Secondary)
	Offline = lipgloss.NewStyle().Foreground(Subtle)
	Badge = lipgloss.NewStyle().Foreground(Warn)
	ExitName = lipgloss.NewStyle().Foreground(Warn).Bold(true)

	IconOnline = lipgloss.NewStyle().Foreground(Secondary)
	IconOffline = lipgloss.NewStyle().Foreground(Subtle)

	// Opaque modal surface: foreground colors over the theme Background.
	ModalTitle = lipgloss.NewStyle().Foreground(Primary).Background(Bg).Bold(true)
	ModalHeading = lipgloss.NewStyle().Foreground(Secondary).Background(Bg).Bold(true)
	ModalText = lipgloss.NewStyle().Foreground(Fg).Background(Bg)
	ModalKey = lipgloss.NewStyle().Foreground(Primary).Background(Bg)
	ModalAccent = lipgloss.NewStyle().Foreground(Secondary).Background(Bg)
	ModalDim = lipgloss.NewStyle().Foreground(Subtle).Background(Bg)
}

// ModalFill returns a style that paints content to the given width with the
// theme background, padding the remainder so the line is fully opaque.
func ModalFill(width int) lipgloss.Style {
	if width < 0 {
		width = 0
	}
	return lipgloss.NewStyle().Width(width).Background(Bg)
}

const boxHPad = 1 // horizontal breathing room inside panes/modals

// box is the shared rounded-border container; Box/BoxFocused pick the border
// color. Sized to the given OUTER width/height (border included). Content area
// is width-2(border)-2(padding) wide.
func box(width, height int, border lipgloss.TerminalColor) lipgloss.Style {
	w, h := width-2, height-2
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, boxHPad).
		Width(w).
		Height(h)
}

// Box returns an inactive pane container with a subtle, dim border.
func Box(width, height int) lipgloss.Style { return box(width, height, BorderInactive) }

// BoxFocused returns the active/focused pane container with a bright border.
func BoxFocused(width, height int) lipgloss.Style { return box(width, height, Primary) }

// ContentWidth returns the usable inner width of a Box of the given outer width
// (subtracting both border and padding).
func ContentWidth(outer int) int {
	w := outer - 2 - 2*boxHPad
	if w < 0 {
		w = 0
	}
	return w
}

// Divider returns a full-width horizontal rule in a subtle color.
func Divider(width int) string {
	if width < 0 {
		width = 0
	}
	return lipgloss.NewStyle().Foreground(BorderInactive).Render(strings.Repeat("─", width))
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

// latencyBlocks maps a 0..7 bucket to an ascending vertical block glyph.
var latencyBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders values as a single-accent-color block sparkline (used for
// the compact dashboard graph).
func Sparkline(values []int) string {
	if len(values) == 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(Primary).Render(string(bars(values)))
}

// LatencyGraph renders a premium per-bar latency graph: bar height is scaled to
// the series' min/max so the shape is readable, while each bar's color/weight
// encodes the absolute latency — faint accent for low, solid accent for medium,
// bold warning for high, and bold error for critical spikes.
func LatencyGraph(values []int) string {
	if len(values) == 0 {
		return Dim.Render("no samples")
	}
	low := lipgloss.NewStyle().Foreground(Primary).Faint(true)
	medium := lipgloss.NewStyle().Foreground(Primary)
	high := lipgloss.NewStyle().Foreground(Warn).Bold(true)
	critical := lipgloss.NewStyle().Foreground(Danger).Bold(true)

	runes := bars(values)
	var b strings.Builder
	for i, v := range values {
		var st lipgloss.Style
		switch {
		case v >= 100:
			st = critical
		case v >= 60:
			st = high
		case v >= 30:
			st = medium
		default:
			st = low
		}
		b.WriteString(st.Render(string(runes[i])))
	}
	return b.String()
}

// bars converts values to block runes scaled to the series min/max.
func bars(values []int) []rune {
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
	out := make([]rune, len(values))
	for i, v := range values {
		idx := 0
		if span > 0 {
			idx = (v - min) * (len(latencyBlocks) - 1) / span
		}
		out[i] = latencyBlocks[idx]
	}
	return out
}
