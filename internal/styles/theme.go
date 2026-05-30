package styles

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

// Theme is the central palette for the whole UI. Colors are lipgloss.Color
// values, which accept TrueColor hex ("#RRGGBB") and degrade gracefully to the
// nearest ANSI color on terminals without 24-bit support.
type Theme struct {
	PrimaryAccent   lipgloss.Color // active elements, focus, accents
	SecondaryAccent lipgloss.Color // online state, routes, secondary highlights
	Background      lipgloss.Color // base canvas color
	Surface         lipgloss.Color // elevated panels / modals (tonal depth)
	SurfaceBright   lipgloss.Color // selected row / highlight bar
	BorderInactive  lipgloss.Color // unfocused pane borders, dividers
	TextNormal      lipgloss.Color // primary text
	TextDim         lipgloss.Color // secondary / faded text
	Warning         lipgloss.Color // exit node, degraded, high latency
	Error           lipgloss.Color // conflicts, critical latency
}

// DefaultTheme is the "Matrix Core" master design: the EXACT hex codes from the
// style guide's YAML frontmatter (_designs/00_STYLE_GUIDE.md). It is the
// fallback whenever the system theme cannot be loaded.
func DefaultTheme() Theme {
	return Theme{
		PrimaryAccent:   "#6bfb9a", // primary
		SecondaryAccent: "#4ade80", // primary-container — online/icons green
		Background:      "#0e150f", // background / surface
		Surface:         "#1a211b", // surface-container — elevated panels/modals
		SurfaceBright:   "#333b34", // surface-bright — selection highlight
		BorderInactive:  "#3d4a3e", // outline-variant — dim borders
		TextNormal:      "#dde5da", // on-surface
		TextDim:         "#869486", // outline — labels / dim text
		Warning:         "#ffdd75", // tertiary
		Error:           "#ffb4ab", // error
	}
}

// omarchyTheme mirrors the Omarchy (Aether-managed) colors.toml schema: a flat
// table of hex strings — accent / foreground / background plus the standard
// 16-color terminal palette color0..color15.
type omarchyTheme struct {
	Accent     string `toml:"accent"`
	Foreground string `toml:"foreground"`
	Background string `toml:"background"`
	Color0     string `toml:"color0"`
	Color1     string `toml:"color1"`
	Color2     string `toml:"color2"`
	Color3     string `toml:"color3"`
	Color4     string `toml:"color4"`
	Color5     string `toml:"color5"`
	Color6     string `toml:"color6"`
	Color7     string `toml:"color7"`
	Color8     string `toml:"color8"`
	Color9     string `toml:"color9"`
	Color10    string `toml:"color10"`
	Color11    string `toml:"color11"`
	Color12    string `toml:"color12"`
	Color13    string `toml:"color13"`
	Color14    string `toml:"color14"`
	Color15    string `toml:"color15"`
}

// ThemePath returns the Omarchy master theme file. It honors the
// TAILSCALE_TUI_THEME env override, otherwise defaults to the standard
// Omarchy "current theme" symlink target.
func ThemePath() string {
	if p := os.Getenv("TAILSCALE_TUI_THEME"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "omarchy", "current", "theme", "colors.toml")
}

// LoadTheme returns the system (Omarchy) theme if it can be found and parsed,
// otherwise it silently falls back to the default Matrix Core theme. Mapping is
// per-field: any key missing from the TOML keeps its default, so a partial or
// unusual palette never crashes and never leaves blanks.
//
// Omarchy → Theme mapping:
//
//	accent      → PrimaryAccent
//	color2      → SecondaryAccent   (green palette slot — online/routes)
//	background  → Background
//	foreground  → TextNormal
//	color8      → BorderInactive    (dark gray) and TextDim
//	color3      → Warning           (yellow palette slot)
//	color1      → Error             (red palette slot)
func LoadTheme() Theme {
	t := DefaultTheme()

	path := ThemePath()
	if path == "" {
		return t
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return t // no theme file (or unreadable) — use the default silently
	}
	var o omarchyTheme
	if err := toml.Unmarshal(data, &o); err != nil {
		return t // malformed TOML — don't crash, just use the default
	}

	set := func(dst *lipgloss.Color, hex string) {
		if hex != "" {
			*dst = lipgloss.Color(hex)
		}
	}
	set(&t.PrimaryAccent, o.Accent)
	set(&t.SecondaryAccent, o.Color2)
	set(&t.Background, o.Background)
	set(&t.Surface, o.Color0)       // darkest palette slot → elevated surface
	set(&t.SurfaceBright, o.Color8) // bright black → selection highlight
	set(&t.TextNormal, o.Foreground)
	set(&t.BorderInactive, o.Color8)
	set(&t.TextDim, o.Color8)
	set(&t.Warning, o.Color3)
	set(&t.Error, o.Color1)
	return t
}
