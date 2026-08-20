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
	Mode            string         // "dark" or "light" — drives the surface ladder direction
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

// Theme modes. Omarchy 4 declares this per theme; the legacy schema has no
// such key, so anything we can't read is treated as dark (the historical
// assumption, and what the Matrix Core default is built for).
const (
	ModeDark  = "dark"
	ModeLight = "light"
)

// DefaultTheme is the "Matrix Core" master design: the EXACT hex codes from the
// style guide's YAML frontmatter (_designs/00_STYLE_GUIDE.md). It is the
// fallback whenever the system theme cannot be loaded.
func DefaultTheme() Theme {
	return Theme{
		Mode:            ModeDark,
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

// omarchyV4 mirrors the Omarchy 4 ("Quattro") colors.toml schema: semantically
// named slots rather than a raw terminal palette. Only the fields we map are
// declared — go-toml ignores the rest (cyan/blue/magenta/brown/bright_*).
type omarchyV4 struct {
	Mode              string `toml:"mode"`
	Accent            string `toml:"accent"`
	Selection         string `toml:"selection"`
	Muted             string `toml:"muted"`
	Background        string `toml:"background"`
	DarkBackground    string `toml:"dark_background"`
	LighterBackground string `toml:"lighter_background"`
	Foreground        string `toml:"foreground"`
	Red               string `toml:"red"`
	Yellow            string `toml:"yellow"`
	Orange            string `toml:"orange"`
	Green             string `toml:"green"`
}

// hasMarkers reports whether the parsed file actually looks like the v4 schema.
// The three keys v4 shares with the legacy schema (accent/foreground/
// background) are deliberately excluded — only v4-exclusive names count.
func (o omarchyV4) hasMarkers() bool {
	return o.Mode != "" || o.Selection != "" || o.Muted != "" ||
		o.DarkBackground != "" || o.LighterBackground != "" ||
		o.Red != "" || o.Yellow != "" || o.Green != ""
}

// omarchyLegacy mirrors the pre-4 Omarchy colors.toml schema: a flat table of
// hex strings — accent / foreground / background plus the standard 16-color
// terminal palette color0..color15.
type omarchyLegacy struct {
	Accent     string `toml:"accent"`
	Foreground string `toml:"foreground"`
	Background string `toml:"background"`
	Color0     string `toml:"color0"`
	Color1     string `toml:"color1"`
	Color2     string `toml:"color2"`
	Color3     string `toml:"color3"`
	Color8     string `toml:"color8"`
}

// hasMarkers reports whether the parsed file looks like the legacy schema —
// i.e. it carries at least one colorN slot.
func (o omarchyLegacy) hasMarkers() bool {
	return o.Color0 != "" || o.Color1 != "" || o.Color2 != "" ||
		o.Color3 != "" || o.Color8 != ""
}

// themeCandidates lists the colors.toml paths to try, in priority order. A
// TAILTUI_THEME override short-circuits the search entirely (one candidate, so
// a bad override fails loudly-ish rather than silently loading a system theme
// the user meant to replace).
//
// Omarchy 4 ("Quattro") moved the current-theme symlink out of ~/.config and
// into ~/.local/state; we probe the new location first and keep the old one as
// a fallback so pre-4 installs still work.
func themeCandidates() []string {
	if p := os.Getenv("TAILTUI_THEME"); p != "" {
		return []string{p}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "state", "omarchy", "current", "theme", "colors.toml"), // Omarchy 4+
		filepath.Join(home, ".config", "omarchy", "current", "theme", "colors.toml"),         // Omarchy <= 3
	}
}

// ThemePath returns the colors.toml that LoadTheme will actually read: the
// first candidate that exists on disk. It returns the highest-priority
// candidate when none exist (useful for diagnostics), or "" if the home
// directory can't be resolved.
func ThemePath() string {
	candidates := themeCandidates()
	if len(candidates) == 0 {
		return ""
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

// LoadTheme returns the system (Omarchy) theme if one can be found and parsed,
// otherwise it silently falls back to the default Matrix Core theme. It
// understands both the Omarchy 4 semantic schema and the legacy color0..15
// schema, detected by which keys are present rather than by which file it came
// from — so a TAILTUI_THEME override of either vintage works.
//
// Mapping is per-field: any key missing from the TOML keeps its default, so a
// partial or unusual palette never crashes and never leaves blanks.
func LoadTheme() Theme {
	for _, path := range themeCandidates() {
		if t, ok := loadThemeFile(path); ok {
			return t
		}
	}
	return DefaultTheme()
}

// loadThemeFile reads and maps a single colors.toml. ok is false when the file
// is missing, unreadable, malformed, or carries no recognizable color keys, so
// the caller can fall through to the next candidate.
func loadThemeFile(path string) (Theme, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, false // no theme file (or unreadable) — try the next
	}

	var v4 omarchyV4
	if err := toml.Unmarshal(data, &v4); err != nil {
		return Theme{}, false // malformed TOML — don't crash
	}
	if v4.hasMarkers() {
		return mapV4(v4), true
	}

	var legacy omarchyLegacy
	if err := toml.Unmarshal(data, &legacy); err != nil {
		return Theme{}, false
	}
	if legacy.hasMarkers() {
		return mapLegacy(legacy), true
	}

	// Neither schema's markers are present. If the file still carries the three
	// keys both schemas share, honor those and keep the defaults for the rest;
	// otherwise it isn't a palette we recognize at all.
	if v4.Accent == "" && v4.Foreground == "" && v4.Background == "" {
		return Theme{}, false
	}
	t := DefaultTheme()
	set(&t.PrimaryAccent, v4.Accent)
	set(&t.TextNormal, v4.Foreground)
	set(&t.Background, v4.Background)
	return t, true
}

// set assigns a color only when the source key was actually present, so every
// unmapped slot keeps its Matrix Core default.
func set(dst *lipgloss.Color, hex string) {
	if hex != "" {
		*dst = lipgloss.Color(hex)
	}
}

// mapV4 maps the Omarchy 4 semantic schema onto the Theme.
//
//	accent                              → PrimaryAccent
//	green                               → SecondaryAccent  (online / routes)
//	background                          → Background
//	lighter_background / dark_background → Surface          (mode-dependent)
//	selection                           → SurfaceBright    (selection bar)
//	muted                               → BorderInactive and TextDim
//	foreground                          → TextNormal
//	yellow (or orange)                  → Warning
//	red                                 → Error
func mapV4(o omarchyV4) Theme {
	t := DefaultTheme()

	if o.Mode == ModeLight {
		t.Mode = ModeLight
	}

	// The elevated surface must move *away* from the canvas: lighter than a
	// dark background, shaded against a light one. Getting this backwards is
	// what made light themes render as mud.
	elevated := o.LighterBackground
	if t.Mode == ModeLight {
		elevated = o.DarkBackground
	}

	set(&t.PrimaryAccent, o.Accent)
	set(&t.SecondaryAccent, o.Green)
	set(&t.Background, o.Background)
	set(&t.Surface, elevated)
	set(&t.SurfaceBright, o.Selection)
	set(&t.BorderInactive, o.Muted)
	set(&t.TextDim, o.Muted)
	set(&t.TextNormal, o.Foreground)
	// yellow is the semantic caution slot; orange covers the few themes that
	// omit it (e.g. "white").
	if o.Yellow != "" {
		set(&t.Warning, o.Yellow)
	} else {
		set(&t.Warning, o.Orange)
	}
	set(&t.Error, o.Red)
	return t
}

// mapLegacy maps the pre-4 Omarchy schema onto the Theme.
//
//	accent      → PrimaryAccent
//	color2      → SecondaryAccent   (green palette slot — online/routes)
//	background  → Background
//	color0      → Surface           (darkest palette slot)
//	color8      → SurfaceBright, BorderInactive and TextDim
//	foreground  → TextNormal
//	color3      → Warning           (yellow palette slot)
//	color1      → Error             (red palette slot)
func mapLegacy(o omarchyLegacy) Theme {
	t := DefaultTheme()
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
