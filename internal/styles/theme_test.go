package styles

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Real fixtures, copied verbatim from shipped Omarchy themes so a schema drift
// upstream shows up here rather than as a washed-out UI.

// osaka-jade, Omarchy 4 ("Quattro") semantic schema, dark.
const v4DarkTOML = `
mode = "dark"

accent = "#509475"
selection = "#32473B"
muted = "#53685B"

background = "#111c18"
dark_background = "#0c1512"
darker_background = "#090f0d"
lighter_background = "#23372B"

foreground = "#C1C497"
dark_foreground = "#81B8A8"

red = "#FF5345"
yellow = "#459451"
orange = "#a2734b"
green = "#549e6a"
cyan = "#2DD5B7"
blue = "#509475"
`

// catppuccin-latte, Omarchy 4 semantic schema, light.
const v4LightTOML = `
mode = "light"

accent = "#1e66f5"
selection = "#ccd0da"
muted = "#acb0be"

background = "#eff1f5"
dark_background = "#e3e4e8"
darker_background = "#d7d8dc"
lighter_background = "#dce0e8"

foreground = "#4c4f69"

red = "#d20f39"
yellow = "#df8e1d"
orange = "#d84e2b"
green = "#40a02b"
`

// tokyo-night, pre-4 flat terminal-palette schema.
const legacyTOML = `
accent = "#7aa2f7"
cursor = "#c0caf5"
foreground = "#a9b1d6"
background = "#1a1b26"

color0 = "#32344a"
color1 = "#f7768e"
color2 = "#9ece6a"
color3 = "#e0af68"
color8 = "#444b6a"
color15 = "#acb0d0"
`

// writeTheme drops a colors.toml into a temp dir and returns its path.
func writeTheme(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "colors.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// loadOverride points TAILTUI_THEME at a fixture and loads it.
func loadOverride(t *testing.T, content string) Theme {
	t.Helper()
	t.Setenv("TAILTUI_THEME", writeTheme(t, content))
	return LoadTheme()
}

func TestLoadThemeV4Dark(t *testing.T) {
	got := loadOverride(t, v4DarkTOML)

	want := map[string]struct{ field, value string }{
		"Mode":            {got.Mode, ModeDark},
		"PrimaryAccent":   {string(got.PrimaryAccent), "#509475"},   // accent
		"SecondaryAccent": {string(got.SecondaryAccent), "#549e6a"}, // green
		"Background":      {string(got.Background), "#111c18"},      // background
		"Surface":         {string(got.Surface), "#23372B"},         // lighter_background
		"SurfaceBright":   {string(got.SurfaceBright), "#32473B"},   // selection
		"BorderInactive":  {string(got.BorderInactive), "#53685B"},  // muted
		"TextDim":         {string(got.TextDim), "#53685B"},         // muted
		"TextNormal":      {string(got.TextNormal), "#C1C497"},      // foreground
		"Warning":         {string(got.Warning), "#459451"},         // yellow
		"Error":           {string(got.Error), "#FF5345"},           // red
	}
	for name, c := range want {
		if c.field != c.value {
			t.Errorf("%s = %q, want %q", name, c.field, c.value)
		}
	}
}

// A light theme must shade its elevated surface *darker* than the canvas —
// taking lighter_background (the dark-mode slot) would make modals vanish.
func TestLoadThemeV4LightUsesShadedSurface(t *testing.T) {
	got := loadOverride(t, v4LightTOML)

	if got.Mode != ModeLight {
		t.Errorf("Mode = %q, want %q", got.Mode, ModeLight)
	}
	if string(got.Background) != "#eff1f5" {
		t.Errorf("Background = %q, want %q", got.Background, "#eff1f5")
	}
	if string(got.Surface) != "#e3e4e8" {
		t.Errorf("Surface = %q, want dark_background %q", got.Surface, "#e3e4e8")
	}
	if string(got.Surface) == "#dce0e8" {
		t.Error("Surface took lighter_background; light themes must shade downward")
	}
	if string(got.SurfaceBright) != "#ccd0da" {
		t.Errorf("SurfaceBright = %q, want selection %q", got.SurfaceBright, "#ccd0da")
	}
}

// The pre-4 flat palette must keep working — installs that never upgraded, and
// hand-written TAILTUI_THEME files, still use it.
func TestLoadThemeLegacySchema(t *testing.T) {
	got := loadOverride(t, legacyTOML)

	if got.Mode != ModeDark {
		t.Errorf("Mode = %q, want %q (legacy has no mode key)", got.Mode, ModeDark)
	}
	want := map[string]struct{ field, value string }{
		"PrimaryAccent":   {string(got.PrimaryAccent), "#7aa2f7"},   // accent
		"SecondaryAccent": {string(got.SecondaryAccent), "#9ece6a"}, // color2
		"Background":      {string(got.Background), "#1a1b26"},      // background
		"Surface":         {string(got.Surface), "#32344a"},         // color0
		"SurfaceBright":   {string(got.SurfaceBright), "#444b6a"},   // color8
		"BorderInactive":  {string(got.BorderInactive), "#444b6a"},  // color8
		"TextNormal":      {string(got.TextNormal), "#a9b1d6"},      // foreground
		"Warning":         {string(got.Warning), "#e0af68"},         // color3
		"Error":           {string(got.Error), "#f7768e"},           // color1
	}
	for name, c := range want {
		if c.field != c.value {
			t.Errorf("%s = %q, want %q", name, c.field, c.value)
		}
	}
}

// Themes like "white" omit the yellow slot; the caution color falls back to
// orange rather than reverting to the Matrix Core green-on-dark yellow.
func TestLoadThemeWarningFallsBackToOrange(t *testing.T) {
	got := loadOverride(t, `
mode = "dark"
accent = "#8d8d8d"
muted = "#7a7a7a"
background = "#000000"
red = "#a4a4a4"
orange = "#b9b9b9"
green = "#b6b6b6"
`)
	if string(got.Warning) != "#b9b9b9" {
		t.Errorf("Warning = %q, want orange %q", got.Warning, "#b9b9b9")
	}
}

// Per-field mapping: an unusual palette keeps defaults for what it omits
// instead of rendering blanks.
func TestLoadThemePartialKeepsDefaults(t *testing.T) {
	def := DefaultTheme()
	got := loadOverride(t, `
mode = "dark"
accent = "#ff00ff"
`)
	if string(got.PrimaryAccent) != "#ff00ff" {
		t.Errorf("PrimaryAccent = %q, want %q", got.PrimaryAccent, "#ff00ff")
	}
	if got.Background != def.Background {
		t.Errorf("Background = %q, want default %q", got.Background, def.Background)
	}
	if got.Error != def.Error {
		t.Errorf("Error = %q, want default %q", got.Error, def.Error)
	}
}

// A file carrying only the three keys both schemas share is ambiguous but
// usable — honor them, default the rest.
func TestLoadThemeAmbiguousSharedKeys(t *testing.T) {
	def := DefaultTheme()
	got := loadOverride(t, `
accent = "#112233"
foreground = "#445566"
background = "#778899"
`)
	if string(got.PrimaryAccent) != "#112233" {
		t.Errorf("PrimaryAccent = %q, want %q", got.PrimaryAccent, "#112233")
	}
	if string(got.TextNormal) != "#445566" {
		t.Errorf("TextNormal = %q, want %q", got.TextNormal, "#445566")
	}
	if got.Surface != def.Surface {
		t.Errorf("Surface = %q, want default %q", got.Surface, def.Surface)
	}
}

func TestLoadThemeFallsBackToDefault(t *testing.T) {
	def := DefaultTheme()
	cases := map[string]string{
		"malformed TOML": "this is not = valid = toml [[[",
		"no color keys":  "unrelated = \"value\"\n",
		"empty file":     "",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if got := loadOverride(t, content); got != def {
				t.Errorf("LoadTheme() = %+v, want default", got)
			}
		})
	}
}

func TestLoadThemeMissingFileFallsBackToDefault(t *testing.T) {
	t.Setenv("TAILTUI_THEME", filepath.Join(t.TempDir(), "nope.toml"))
	if got := LoadTheme(); got != DefaultTheme() {
		t.Errorf("LoadTheme() = %+v, want default", got)
	}
}

// stubHome plants colors.toml files at the Omarchy 4 and/or legacy locations
// under a temp HOME, with TAILTUI_THEME cleared.
func stubHome(t *testing.T, v4Content, legacyContent string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TAILTUI_THEME", "")

	plant := func(content string, parts ...string) {
		if content == "" {
			return
		}
		dir := filepath.Join(append([]string{home}, parts...)...)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "colors.toml"), []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	plant(v4Content, ".local", "state", "omarchy", "current", "theme")
	plant(legacyContent, ".config", "omarchy", "current", "theme")
}

// Omarchy 4 moved the current-theme symlink; an upgraded box often still has a
// stale ~/.config copy, so the new location must win.
func TestThemePathPrefersOmarchy4Location(t *testing.T) {
	stubHome(t, v4DarkTOML, legacyTOML)

	want := filepath.Join(os.Getenv("HOME"), ".local", "state", "omarchy", "current", "theme", "colors.toml")
	if got := ThemePath(); got != want {
		t.Errorf("ThemePath() = %q, want %q", got, want)
	}
	if got := LoadTheme(); string(got.PrimaryAccent) != "#509475" {
		t.Errorf("PrimaryAccent = %q, want the Omarchy 4 theme %q", got.PrimaryAccent, "#509475")
	}
}

// Pre-4 installs have only the ~/.config location — it must still be found.
func TestThemePathFallsBackToLegacyLocation(t *testing.T) {
	stubHome(t, "", legacyTOML)

	want := filepath.Join(os.Getenv("HOME"), ".config", "omarchy", "current", "theme", "colors.toml")
	if got := ThemePath(); got != want {
		t.Errorf("ThemePath() = %q, want %q", got, want)
	}
	if got := LoadTheme(); string(got.PrimaryAccent) != "#7aa2f7" {
		t.Errorf("PrimaryAccent = %q, want the legacy theme %q", got.PrimaryAccent, "#7aa2f7")
	}
}

func TestThemePathOverrideWins(t *testing.T) {
	stubHome(t, v4DarkTOML, legacyTOML)
	override := writeTheme(t, v4LightTOML)
	t.Setenv("TAILTUI_THEME", override)

	if got := ThemePath(); got != override {
		t.Errorf("ThemePath() = %q, want %q", got, override)
	}
	if got := LoadTheme(); got.Mode != ModeLight {
		t.Errorf("Mode = %q, want the override's %q", got.Mode, ModeLight)
	}
}

// Every theme installed on this machine must map to a complete palette. Skips
// cleanly where Omarchy isn't installed (CI, other distros).
func TestInstalledOmarchyThemesMapCompletely(t *testing.T) {
	matches, _ := filepath.Glob(filepath.Join(os.Getenv("HOME"), ".local", "share", "omarchy", "themes", "*", "colors.toml"))
	if len(matches) == 0 {
		t.Skip("no Omarchy themes installed")
	}
	def := DefaultTheme()
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		t.Run(name, func(t *testing.T) {
			got, ok := loadThemeFile(path)
			if !ok {
				t.Fatalf("%s: unrecognized schema", path)
			}
			// Every slot the theme should drive must actually come from the
			// file; a default leaking through means an unmapped key.
			slots := map[string]struct{ got, def string }{
				"PrimaryAccent":   {string(got.PrimaryAccent), string(def.PrimaryAccent)},
				"SecondaryAccent": {string(got.SecondaryAccent), string(def.SecondaryAccent)},
				"Background":      {string(got.Background), string(def.Background)},
				"Surface":         {string(got.Surface), string(def.Surface)},
				"SurfaceBright":   {string(got.SurfaceBright), string(def.SurfaceBright)},
				"BorderInactive":  {string(got.BorderInactive), string(def.BorderInactive)},
				"TextNormal":      {string(got.TextNormal), string(def.TextNormal)},
				"Warning":         {string(got.Warning), string(def.Warning)},
				"Error":           {string(got.Error), string(def.Error)},
			}
			for slot, v := range slots {
				if v.got == "" {
					t.Errorf("%s is empty", slot)
				}
				if v.got == v.def {
					t.Errorf("%s fell back to the Matrix Core default %q — unmapped key?", slot, v.def)
				}
			}
		})
	}
}

// tailTUI's own schema, as an Omarchy template renders it.
const ownTOML = `
mode = "dark"
primary = "#509475"
secondary = "#549e6a"
background = "#111c18"
surface = "#23372B"
surface_bright = "#32473B"
border = "#53685B"
text = "#C1C497"
text_dim = "#53685B"
warning = "#a2734b"
error = "#FF5345"
`

func TestLoadThemeOwnSchema(t *testing.T) {
	got := loadOverride(t, ownTOML)

	want := map[string]struct{ field, value string }{
		"Mode":            {got.Mode, ModeDark},
		"PrimaryAccent":   {string(got.PrimaryAccent), "#509475"},
		"SecondaryAccent": {string(got.SecondaryAccent), "#549e6a"},
		"Background":      {string(got.Background), "#111c18"},
		"Surface":         {string(got.Surface), "#23372B"},
		"SurfaceBright":   {string(got.SurfaceBright), "#32473B"},
		"BorderInactive":  {string(got.BorderInactive), "#53685B"},
		"TextNormal":      {string(got.TextNormal), "#C1C497"},
		"TextDim":         {string(got.TextDim), "#53685B"},
		"Warning":         {string(got.Warning), "#a2734b"},
		"Error":           {string(got.Error), "#FF5345"},
	}
	for name, c := range want {
		if c.field != c.value {
			t.Errorf("%s = %q, want %q", name, c.field, c.value)
		}
	}
}

// The whole point of the template: the user resolves mapping calls we cannot.
// osaka-jade defines yellow as a green, so a template can route warning to
// orange instead — something no heuristic in the loader could decide.
func TestOwnSchemaOverridesAmbiguousMapping(t *testing.T) {
	viaColors := loadOverride(t, v4DarkTOML)
	if string(viaColors.Warning) != "#459451" {
		t.Fatalf("colors.toml Warning = %q, want the theme's green-ish yellow", viaColors.Warning)
	}
	viaOwn := loadOverride(t, ownTOML)
	if string(viaOwn.Warning) != "#a2734b" {
		t.Errorf("tailtui.toml Warning = %q, want the orange the template chose", viaOwn.Warning)
	}
}

func TestOwnSchemaLightMode(t *testing.T) {
	got := loadOverride(t, "mode = \"light\"\nprimary = \"#0c6b3d\"\nsurface = \"#e3e4e8\"\n")
	if got.Mode != ModeLight {
		t.Errorf("Mode = %q, want %q", got.Mode, ModeLight)
	}
	if string(got.Surface) != "#e3e4e8" {
		t.Errorf("Surface = %q, want %q", got.Surface, "#e3e4e8")
	}
}

// Detection is by key, not filename, so the three schemas must not be confused
// for one another regardless of where the file came from.
func TestSchemaDetectionIsUnambiguous(t *testing.T) {
	cases := []struct {
		name, content, marker, want string
	}{
		{"own schema", ownTOML, "PrimaryAccent", "#509475"},
		{"omarchy v4", v4DarkTOML, "PrimaryAccent", "#509475"},
		{"legacy", legacyTOML, "PrimaryAccent", "#7aa2f7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(loadOverride(t, c.content).PrimaryAccent); got != c.want {
				t.Errorf("%s = %q, want %q", c.marker, got, c.want)
			}
		})
	}
	// The own-schema fixture must not be mistaken for an Omarchy one: it maps
	// surface directly rather than deriving it from lighter_background.
	if got := loadOverride(t, ownTOML); string(got.Surface) != "#23372B" {
		t.Errorf("Surface = %q — own schema was parsed as an Omarchy file?", got.Surface)
	}
}

// tailtui.toml is opt-in, so it outranks colors.toml in the same directory.
func TestThemePathPrefersOwnSchemaInThemeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TAILTUI_THEME", "")

	dir := filepath.Join(home, ".local", "state", "omarchy", "current", "theme")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range map[string]string{
		"colors.toml":  v4DarkTOML,
		"tailtui.toml": ownTOML,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	want := filepath.Join(dir, "tailtui.toml")
	if got := ThemePath(); got != want {
		t.Errorf("ThemePath() = %q, want %q", got, want)
	}
	// The distinguishing value: only the own-schema fixture sets warning to orange.
	if got := LoadTheme(); string(got.Warning) != "#a2734b" {
		t.Errorf("Warning = %q, want tailtui.toml's %q", got.Warning, "#a2734b")
	}
}

// Falling back to colors.toml when no template is installed is what keeps the
// zero-config path working.
func TestThemePathFallsBackToColorsWhenNoTemplate(t *testing.T) {
	stubHome(t, v4DarkTOML, "")
	want := filepath.Join(os.Getenv("HOME"), ".local", "state", "omarchy", "current", "theme", "colors.toml")
	if got := ThemePath(); got != want {
		t.Errorf("ThemePath() = %q, want %q", got, want)
	}
}

// ThemeStamp drives live reload, so it must track both which file is active and
// when it changed.
func TestThemeStampTracksFileChanges(t *testing.T) {
	path := writeTheme(t, ownTOML)
	t.Setenv("TAILTUI_THEME", path)

	gotPath, mod := ThemeStamp()
	if gotPath != path {
		t.Fatalf("ThemeStamp path = %q, want %q", gotPath, path)
	}
	if mod.IsZero() {
		t.Fatal("ThemeStamp mod time is zero for an existing file")
	}

	// A theme switch rewrites the file; the stamp must move with it.
	future := mod.Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	_, mod2 := ThemeStamp()
	if !mod2.After(mod) {
		t.Errorf("mod time did not advance: %v then %v", mod, mod2)
	}
}

func TestThemeStampEmptyWhenNoThemeFile(t *testing.T) {
	t.Setenv("TAILTUI_THEME", filepath.Join(t.TempDir(), "absent.toml"))
	path, mod := ThemeStamp()
	if path != "" || !mod.IsZero() {
		t.Errorf("ThemeStamp() = (%q, %v), want empty", path, mod)
	}
}
