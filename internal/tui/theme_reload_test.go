package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Phundahl/tailtui/internal/styles"
)

// restoreTheme puts the package-level palette back after a test, since
// styles.Apply mutates globals shared by every other suite in this package.
func restoreTheme(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { styles.Apply(styles.DefaultTheme()) })
}

// A changed theme file must repaint a running tailTUI: the palette is applied,
// the stamp advances, and the reload is logged.
func TestThemeMsgAppliesPalette(t *testing.T) {
	restoreTheme(t)
	m := newReadyModel(t, 120, 40)

	next := styles.DefaultTheme()
	next.PrimaryAccent = "#abcdef"
	stamp := time.Now()

	updated, _ := m.Update(themeMsg{theme: next, path: "/themes/x/tailtui.toml", mod: stamp, changed: true})
	got := updated.(Model)

	if string(styles.Primary) != "#abcdef" {
		t.Errorf("styles.Primary = %q, want the reloaded %q", styles.Primary, "#abcdef")
	}
	if got.themePath != "/themes/x/tailtui.toml" {
		t.Errorf("themePath = %q, not recorded", got.themePath)
	}
	if !got.themeMod.Equal(stamp) {
		t.Errorf("themeMod = %v, want %v", got.themeMod, stamp)
	}
	if len(got.logs) == 0 || !strings.Contains(got.logs[len(got.logs)-1].Message, "Theme reloaded") {
		t.Error("theme reload was not logged")
	}
}

// An unchanged file must still record the stamp — so a theme file appearing (or
// vanishing) later is noticed — but must not repaint or log.
func TestThemeMsgUnchangedIsQuiet(t *testing.T) {
	restoreTheme(t)
	styles.Apply(styles.DefaultTheme())
	before := styles.Primary

	m := newReadyModel(t, 120, 40)
	stamp := time.Now()

	updated, cmd := m.Update(themeMsg{path: "/themes/x/colors.toml", mod: stamp, changed: false})
	got := updated.(Model)

	if styles.Primary != before {
		t.Errorf("palette changed on an unchanged theme: %q -> %q", before, styles.Primary)
	}
	if got.themePath != "/themes/x/colors.toml" || !got.themeMod.Equal(stamp) {
		t.Error("stamp not recorded for an unchanged file")
	}
	if len(got.logs) != 0 {
		t.Errorf("unchanged theme logged %d entries, want 0", len(got.logs))
	}
	if cmd != nil {
		t.Error("unchanged theme should issue no command")
	}
}

// Viewport-backed overlays hold pre-rendered strings with colors baked in as
// ANSI codes, so a theme change has to rebuild them rather than wait for the
// next resize.
func TestThemeMsgRebuildsOpenOverlay(t *testing.T) {
	restoreTheme(t)
	// Tests have no TTY, so lipgloss would strip every SGR and both renders
	// would compare equal no matter what the palette did.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	m := newReadyModel(t, 120, 40)

	opened, _ := m.Update(key("?"))
	m = opened.(Model)
	if m.state != stateHelp {
		t.Fatalf("state = %v, want stateHelp", m.state)
	}
	before := m.overlay.View()

	next := styles.DefaultTheme()
	next.PrimaryAccent = "#ff00ff"
	next.TextNormal = "#00ffff"
	updated, _ := m.Update(themeMsg{theme: next, path: "/t/tailtui.toml", mod: time.Now(), changed: true})
	got := updated.(Model)

	if got.state != stateHelp {
		t.Errorf("state = %v, overlay should stay open across a theme change", got.state)
	}
	if got.overlay.View() == before {
		t.Error("overlay content unchanged after a theme reload — stale ANSI colors")
	}
	assertFlush(t, got.View(), 120, 40)
}

// The stamp seeded in New() must match what main already applied, so the first
// tick doesn't report a spurious change and log a reload at startup.
func TestNewSeedsThemeStamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tailtui.toml")
	if err := os.WriteFile(path, []byte("primary = \"#123456\"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("TAILTUI_THEME", path)

	m := New()
	if m.themePath != path {
		t.Errorf("themePath = %q, want %q", m.themePath, path)
	}
	if m.themeMod.IsZero() {
		t.Error("themeMod not seeded — the first tick would report a false change")
	}
}
