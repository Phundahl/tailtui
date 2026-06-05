package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// newReadyModel returns a Model sized to w×h (ready to render).
func newReadyModel(t *testing.T, w, h int) Model {
	t.Helper()
	m, _ := New().Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(Model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// assertFlush checks the render is exactly h rows and no row exceeds w cells —
// the invariant that keeps the alt-screen layout from wrapping/scrolling.
func assertFlush(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Fatalf("view has %d rows, want %d", len(lines), h)
	}
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got > w {
			t.Fatalf("row %d width %d exceeds terminal width %d", i, got, w)
		}
	}
}

// Phase 20: PEER DETAILS stays locked to LOCAL_NODE height so the top panes'
// bottom borders remain flush after stacking the new [s] action label.
func TestLayoutSymmetryFlush(t *testing.T) {
	lay := computeLayout(120, 40)
	if lay.localH != lay.detailsH {
		t.Fatalf("detailsH (%d) != localH (%d): top borders won't align", lay.detailsH, lay.localH)
	}
	if lay.localH != localNodeH {
		t.Fatalf("localH = %d, want %d (unclamped)", lay.localH, localNodeH)
	}
	// LOCAL_NODE must fit its 9 content rows (4 identity + 3 exit rows + Connect
	// button + the grouped Settings/Routing action row) in localH-2 inner rows
	// without clipping.
	if inner := lay.localH - 2; inner < 9 {
		t.Fatalf("LOCAL_NODE inner height %d can't fit 9 content rows", inner)
	}
}

// Phase 20.1: Advanced Settings is bound to uppercase [S] (shift+s); lowercase
// `s` is reserved for the future SSH-as-action and must be a no-op.
func TestSettingsHotkeyIsUppercase(t *testing.T) {
	m := newReadyModel(t, 120, 40)

	m2, _ := m.Update(key("s")) // lowercase: reserved, ignored
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("lowercase s opened a modal (state=%v); it must be reserved for SSH", m.state)
	}

	m2, _ = m.Update(key("S")) // uppercase: opens settings
	m = m2.(Model)
	if m.state != stateSettings {
		t.Fatalf("uppercase S did not open settings (state=%v)", m.state)
	}
}

// Phase 20: the [S] entry point opens the settings modal, which renders as a
// flush floating overlay over the still-visible base view.
func TestSettingsOverlayRendersFlush(t *testing.T) {
	const w, h = 120, 40
	m := newReadyModel(t, w, h)

	m2, _ := m.Update(key("S"))
	m = m2.(Model)
	if m.state != stateSettings {
		t.Fatalf("[S] did not open settings (state=%v)", m.state)
	}

	view := m.View()
	assertFlush(t, view, w, h)
	if !strings.Contains(view, "ADVANCED_SETTINGS") || !strings.Contains(view, "DESCRIPTION") {
		t.Fatalf("settings modal missing its two boxes:\n%s", view)
	}
	// The background must remain visible behind the modal (true overlay, not a
	// blank screen): the header brand and a base pane title should survive.
	if !strings.Contains(view, appName) {
		t.Fatalf("header not visible behind modal — overlay blanked the base view")
	}
}

// Phase 20: j/k navigate the toggle list and Space optimistically flips the
// highlighted setting (and emits a command to drive `tailscale set`).
func TestSettingsNavigateAndToggle(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m2, _ := m.Update(key("S"))
	m = m2.(Model)

	// Navigate down twice → cursor on settingDefs[2] (Run Tailscale SSH).
	for i := 0; i < 2; i++ {
		m2, _ = m.Update(key("j"))
		m = m2.(Model)
	}
	if m.settingCursor != 2 {
		t.Fatalf("settingCursor = %d after 2×j, want 2", m.settingCursor)
	}

	// Prefs start at zero value (all false); Space toggles the cursor's setting on
	// and must return a command (the async `tailscale set`).
	before := settingDefs[2].get(m.prefs)
	m2, cmd := m.Update(key("space"))
	m = m2.(Model)
	if got := settingDefs[2].get(m.prefs); got == before {
		t.Fatalf("Space did not flip setting (still %v)", got)
	}
	if cmd == nil {
		t.Fatalf("Space did not emit a tailscale set command")
	}

	// k clamps at the top; j clamps at the bottom — never out of range.
	for i := 0; i < 10; i++ {
		m2, _ = m.Update(key("k"))
		m = m2.(Model)
	}
	if m.settingCursor != 0 {
		t.Fatalf("cursor = %d after spamming k, want 0 (clamped)", m.settingCursor)
	}

	// Esc closes back to the main view.
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("Esc did not close settings (state=%v)", m.state)
	}
}

// Phase 20: prefsMsg populates the checkboxes; a flipped pref shows [x].
func TestSettingsReflectsPrefs(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m.prefs.AcceptRoutes = true // simulate a live read

	m2, _ := m.Update(key("S"))
	m = m2.(Model)
	view := m.View()
	if !strings.Contains(view, "[x]") {
		t.Fatalf("enabled pref not shown as [x]:\n%s", view)
	}
	if !strings.Contains(view, "[ ]") {
		t.Fatalf("disabled prefs not shown as [ ]:\n%s", view)
	}
}
