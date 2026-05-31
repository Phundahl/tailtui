// Package styles centralizes all lipgloss styling for the TUI.
//
// Colors come from a central Theme (see theme.go). The package keeps a set of
// derived style/color vars that are (re)built by Apply; call Apply once at
// startup with the loaded theme. An init() applies DefaultTheme so the package
// is always usable even before main wires up the loader.
//
// Aesthetic: "Matrix Core" — sharp/brutalist. All containers use single-line
// box drawing (┌─┐│└┘) via Pane/Modal; tonal depth comes from Surface layers
// (panels) and SurfaceBright (selection), not rounded corners or shadows.
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
	Bg             lipgloss.Color // base canvas color
	Surface        lipgloss.Color // elevated panels / modals
	SurfaceBright  lipgloss.Color // selection highlight
	BorderInactive lipgloss.Color // unfocused borders / dividers
)

// Derived text styles, populated by Apply.
var (
	Title, Heading, Label, Value, Dim                     lipgloss.Style
	Selected, SelectBar, Online, Offline, Badge, ExitName lipgloss.Style

	// Icon colors list glyphs by reachability, so node icons follow the theme
	// instead of inheriting the terminal's default foreground.
	IconOnline, IconOffline lipgloss.Style

	Caution lipgloss.Style // relayed/degraded (yellow)
	Button  lipgloss.Style // [ Connect ]-style bracketed buttons

	// Modal surface styles bake in the Surface color so an overlay renders as a
	// fully opaque, tonally-raised rectangle — every span paints the surface,
	// leaving no fg-only gaps for the view behind it to bleed through.
	ModalTitle, ModalHeading, ModalText, ModalKey, ModalAccent, ModalDim lipgloss.Style

	// Routing-table status chips.
	StatusOK, StatusWarn, StatusErr lipgloss.Style

	// Active-account highlight bar (accounts modal).
	AccountActive, AccountActiveSub lipgloss.Style
)

func init() { Apply(DefaultTheme()) }

// Apply rebuilds every package-level color and style from the given theme.
// Funcs like Pane/Divider/LatencyGraph read these vars at call time, so they
// pick up the new theme automatically after Apply.
func Apply(t Theme) {
	Primary = t.PrimaryAccent
	Secondary = t.SecondaryAccent
	Subtle = t.TextDim
	Warn = t.Warning
	Danger = t.Error
	Fg = t.TextNormal
	Bg = t.Background
	Surface = t.Surface
	SurfaceBright = t.SurfaceBright
	BorderInactive = t.BorderInactive

	Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Heading = lipgloss.NewStyle().Foreground(Secondary).Bold(true)
	Label = lipgloss.NewStyle().Foreground(Subtle)
	Value = lipgloss.NewStyle().Foreground(Fg)
	Dim = lipgloss.NewStyle().Foreground(Subtle)
	// Selection: surface-bright highlight bar + accent text (pointer added in delegate).
	Selected = lipgloss.NewStyle().Foreground(Primary).Background(SurfaceBright).Bold(true)
	SelectBar = lipgloss.NewStyle().Foreground(Primary).Background(SurfaceBright).Bold(true)
	Online = lipgloss.NewStyle().Foreground(Secondary)
	Offline = lipgloss.NewStyle().Foreground(Subtle)
	Badge = lipgloss.NewStyle().Foreground(Secondary)
	ExitName = lipgloss.NewStyle().Foreground(Warn).Bold(true)

	IconOnline = lipgloss.NewStyle().Foreground(Secondary)
	IconOffline = lipgloss.NewStyle().Foreground(Subtle)
	Caution = lipgloss.NewStyle().Foreground(Warn) // relayed / degraded (yellow)
	Button = lipgloss.NewStyle().Foreground(Primary).Bold(true)

	// Opaque modal surface: foreground colors over the elevated Surface color.
	ModalTitle = lipgloss.NewStyle().Foreground(Primary).Background(Surface).Bold(true)
	ModalHeading = lipgloss.NewStyle().Foreground(Secondary).Background(Surface).Bold(true)
	ModalText = lipgloss.NewStyle().Foreground(Fg).Background(Surface)
	ModalKey = lipgloss.NewStyle().Foreground(Primary).Background(Surface)
	ModalAccent = lipgloss.NewStyle().Foreground(Secondary).Background(Surface)
	ModalDim = lipgloss.NewStyle().Foreground(Subtle).Background(Surface)

	// Routing-table status chips (on the modal surface).
	StatusOK = lipgloss.NewStyle().Foreground(Secondary).Background(Surface).Bold(true)
	StatusWarn = lipgloss.NewStyle().Foreground(Warn).Background(Surface).Bold(true)
	StatusErr = lipgloss.NewStyle().Foreground(Danger).Background(Surface).Bold(true)

	// Active account row: dark text on a solid primary-green bar.
	AccountActive = lipgloss.NewStyle().Foreground(Bg).Background(Primary).Bold(true)
	AccountActiveSub = lipgloss.NewStyle().Foreground(Bg).Background(Primary)
}

// LogLevelColor maps a log level to its theme color, so the level chip is
// scannable in both the tail pane and the [v] overlay. ERROR → error (red),
// INFO → primary accent (green), WARN → warning (yellow), DEBUG → secondary
// accent; anything else falls back to the dim/subtle color.
func LogLevelColor(level string) lipgloss.Color {
	switch level {
	case "ERROR":
		return Danger
	case "INFO":
		return Primary
	case "WARN":
		return Warn
	case "DEBUG":
		return Secondary
	default:
		return Subtle
	}
}

// ModalFill returns a style that paints content to the given width with the
// Surface color, padding the remainder so the line is fully opaque.
func ModalFill(width int) lipgloss.Style {
	if width < 0 {
		width = 0
	}
	return lipgloss.NewStyle().Width(width).Background(Surface)
}

const boxHPad = 1 // horizontal breathing room inside panes/modals

// ContentWidth returns the usable inner width of a Pane of the given outer width
// (subtracting both the single-line border and the horizontal padding).
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
// padded to the given width. The left segment is CLIPPED if the two segments
// can't both fit, and the whole line is capped at width — so a long hint can
// never overflow and wrap onto a second row (which would push the layout down
// and scroll the top borders off the alt-screen).
func Bar(width int, left, right string) string {
	rw := lipgloss.Width(right)
	if maxLeft := width - rw - 1; lipgloss.Width(left) > maxLeft {
		if maxLeft < 0 {
			maxLeft = 0
		}
		left = lipgloss.NewStyle().MaxWidth(maxLeft).Render(left)
	}
	gap := width - lipgloss.Width(left) - rw
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().MaxWidth(width).Render(line) // final safety cap
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

// LatencyGraphWidth renders LatencyGraph resampled to exactly width bars, so the
// graph fills the available pane width.
func LatencyGraphWidth(values []int, width int) string {
	if width <= 0 || len(values) == 0 {
		return LatencyGraph(values)
	}
	rs := make([]int, width)
	for i := range rs {
		rs[i] = values[i*len(values)/width]
	}
	return LatencyGraph(rs)
}

// latencyColor maps an absolute latency (ms) to its bar style: faint accent for
// low, solid accent for medium, bold warning for high, bold error for critical.
func latencyColor(v int) lipgloss.Style {
	switch {
	case v >= 100:
		return lipgloss.NewStyle().Foreground(Danger).Bold(true)
	case v >= 60:
		return lipgloss.NewStyle().Foreground(Warn).Bold(true)
	case v >= 30:
		return lipgloss.NewStyle().Foreground(Primary)
	default:
		return lipgloss.NewStyle().Foreground(Primary).Faint(true)
	}
}

// LatencyGraphArea renders a multi-row vertical bar chart filling exactly width
// columns × height rows. Bar HEIGHT is scaled to the series min/max (so the
// shape reads at any range) using eighth-block glyphs for sub-row precision,
// while each bar's COLOR encodes absolute latency (latencyColor) — the same
// language as the single-row LatencyGraph, just taller to fill the pane.
func LatencyGraphArea(values []int, width, height int) string {
	if height < 1 {
		height = 1
	}
	if width < 1 || len(values) == 0 {
		return Dim.Render("no samples")
	}
	if height == 1 {
		return LatencyGraphWidth(values, width)
	}

	// Resample to exactly width columns (nearest-neighbor).
	cols := make([]int, width)
	for i := range cols {
		cols[i] = values[i*len(values)/width]
	}
	min, max := cols[0], cols[0]
	for _, v := range cols {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	maxEighths := height * 8

	var b strings.Builder
	for row := 0; row < height; row++ {
		fromBottom := height - 1 - row // 0 = bottom row
		for _, v := range cols {
			level := maxEighths
			if span > 0 {
				level = (v - min) * maxEighths / span
			} else {
				level = maxEighths / 2 // flat series: half-height bars
			}
			if level < 1 {
				level = 1 // always show at least a sliver
			}
			full, rem := level/8, level%8
			switch {
			case fromBottom < full:
				b.WriteString(latencyColor(v).Render("█"))
			case fromBottom == full && rem > 0:
				b.WriteString(latencyColor(v).Render(string(latencyBlocks[rem-1])))
			default:
				b.WriteByte(' ')
			}
		}
		if row < height-1 {
			b.WriteByte('\n')
		}
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
