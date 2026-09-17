package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/Phundahl/tailtui/internal/styles"
)

const inputBanner = "KEYBOARD INPUT MODE ACTIVE"

// Phase 23.1: the "-- KEYBOARD INPUT MODE ACTIVE --" banner must NOT appear in
// read-only / navigation overlays (Help, Accounts, the routing list).
func TestNoInputBannerInReadOnlyOverlays(t *testing.T) {
	// Help overlay.
	m := newReadyModel(t, 120, 40)
	m2, _ := m.Update(key("?"))
	m = m2.(Model)
	if v := m.View(); strings.Contains(v, inputBanner) {
		t.Fatalf("Help overlay still shows the input-mode banner:\n%s", v)
	}
	if !strings.Contains(m.View(), "[Esc] Close") {
		t.Fatalf("Help overlay missing the plain close hint")
	}

	// Accounts overlay.
	m = newReadyModel(t, 120, 40)
	m2, _ = m.Update(key("l"))
	m = m2.(Model)
	if v := m.View(); strings.Contains(v, inputBanner) {
		t.Fatalf("Accounts overlay still shows the input-mode banner:\n%s", v)
	}

	// Routing list (navigation) mode.
	m = openRoutingModal(t, false, "192.168.1.0/24")
	if v := m.View(); strings.Contains(v, inputBanner) {
		t.Fatalf("Routing list mode still shows the input-mode banner:\n%s", v)
	}

	// Phase 34: the SSH launcher in nav mode is navigation, not text entry —
	// its username field is blurred until [e].
	m = openSSHModal(t, 120, 40)
	if v := m.View(); strings.Contains(v, inputBanner) {
		t.Fatalf("SSH launcher nav mode still shows the input-mode banner:\n%s", v)
	}
}

// Phase 34: the SSH launcher renders its own chrome (it bypasses renderOverlay's
// hint switch), so its banner in edit mode is a genuinely separate code path.
func TestSSHInputModeShowsBanner(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, _ := m.Update(key("e"))
	m = m2.(Model)
	if !m.sshInputMode {
		t.Fatalf("precondition: [e] should enter the username editor")
	}
	if v := m.View(); !strings.Contains(v, inputBanner) {
		t.Fatalf("SSH username editor missing the input-mode banner:\n%s", v)
	}
}

// Phase 23.1: the banner SHOULD appear only in the genuine inline text-input
// state — the routing CIDR editor.
func TestInputBannerOnlyInRoutingInputMode(t *testing.T) {
	m := openRoutingModal(t, false)
	m2, _ := m.Update(key("a")) // enter CIDR input mode
	m = m2.(Model)
	if !m.routingInputMode {
		t.Fatalf("precondition: [a] should enter input mode")
	}
	if v := m.View(); !strings.Contains(v, inputBanner) {
		t.Fatalf("routing input mode missing the input-mode banner:\n%s", v)
	}
}

// Phase 23.1: the CIDR editor styles paint the modal Surface background (no harsh
// near-black block). The cursor's visible state is rendered via Style.Reverse(),
// so its Style is pre-swapped: Foreground=Surface, Background=Primary → displays
// as a Primary glyph on the Surface.
func TestRoutingInputStylesUseSurface(t *testing.T) {
	for _, tc := range modalInputs() {
		t.Run(tc.name, func(t *testing.T) { assertModalInputStyles(t, tc.build()) })
	}
}

// modalInputs enumerates every editor built through newModalInput, so the
// styling contract is held identically for all of them — the whole point of
// having one constructor.
func modalInputs() []struct {
	name  string
	build func() textinput.Model
} {
	return []struct {
		name  string
		build func() textinput.Model
	}{
		{"routing CIDR", newRoutingInput},
		{"ssh username", newSSHUserInput},
	}
}

func assertModalInputStyles(t *testing.T, ti textinput.Model) {
	t.Helper()

	// Phase 23.2: no in-field placeholder — its empty-state render path emits raw
	// (unstyled, near-black) padding spaces. Dropping it makes the field pad with
	// TextStyle (Surface) instead.
	if ti.Placeholder != "" {
		t.Fatalf("textinput Placeholder = %q, want empty (placeholderView emits the black padding)", ti.Placeholder)
	}

	if got := ti.TextStyle.GetBackground(); got != styles.Surface {
		t.Fatalf("TextStyle background = %v, want Surface", got)
	}
	if got := ti.PromptStyle.GetBackground(); got != styles.Surface {
		t.Fatalf("PromptStyle background = %v, want Surface", got)
	}
	if got := ti.Cursor.TextStyle.GetBackground(); got != styles.Surface {
		t.Fatalf("Cursor.TextStyle background = %v, want Surface", got)
	}
	// Phase 23.3: the cursor must be a VISIBLE bright block. bubbles/cursor renders
	// its visible cell with Style.Reverse(true), so the DISPLAYED background is
	// Style's Foreground — which must be Primary (the bright block), NOT Surface
	// (which camouflaged the cursor in 23.2).
	if got := ti.Cursor.Style.GetForeground(); got != styles.Primary {
		t.Fatalf("Cursor.Style foreground = %v, want Primary (becomes the visible block bg after reverse)", got)
	}
	if got := ti.Cursor.Style.GetForeground(); got == styles.Surface {
		t.Fatalf("cursor is camouflaged — the visible block would blend into the modal Surface")
	}
}

// Phase 23.2: regression — under a real (TrueColor) profile, the rendered CIDR
// field (empty, Width-padded, then Surface-filled like the modal) must NOT emit
// a run of unstyled spaces after a reset. That pattern is the "black box": the
// near-black default background showing through the field's padding.
func TestRoutingInputNoRawBlackPadding(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor) // tests have no TTY; force color so bg SGRs render
	defer lipgloss.SetColorProfile(prev)

	for _, tc := range modalInputs() {
		t.Run(tc.name, func(t *testing.T) {
			ti := tc.build()
			ti.Width = 40
			ti.Focus()
			line := styles.ModalFill(64).Render(ti.View()) // exactly how the modals wrap the field

			// A reset immediately followed by several spaces means those cells carry
			// no background — the glitch. A correctly styled pad re-opens a bg SGR.
			if strings.Contains(line, "\x1b[0m     ") {
				t.Fatalf("field emits unstyled (black) padding spaces:\n%q", line)
			}
		})
	}
}

// Phase 23.3: the cursor's visible (block) state must be a bright Primary block
// — reverse-video carrying the Primary color — so it's clearly seen against the
// Surface (not the invisible Surface-fg camouflage of 23.2).
func TestRoutingCursorIsVisibleBlock(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	// Derive the Primary foreground SGR (e.g. "38;2;107;251;154") from the theme
	// so this doesn't hardcode the palette.
	probe := lipgloss.NewStyle().Foreground(styles.Primary).Render("x")
	primSGR := probe[strings.Index(probe, "38;2;"):strings.IndexByte(probe, 'm')]

	for _, tc := range modalInputs() {
		t.Run(tc.name, func(t *testing.T) {
			cur := tc.build().Cursor
			cur.SetChar(" ")
			cur.Blink = false // force the visible (block) phase
			view := cur.View()

			if !strings.Contains(view, "\x1b[7") { // reverse video → Foreground becomes the block bg
				t.Fatalf("cursor visible state is not a reverse-video block (would be invisible): %q", view)
			}
			if !strings.Contains(view, primSGR) {
				t.Fatalf("cursor block is not Primary-colored: %q", view)
			}
		})
	}
}

// Phase 34: modal editors capture their theme-derived styles BY VALUE at
// construction, so a live theme reload would leave a stale cursor/prompt color
// baked into a stored textinput. resizeOverlay (the post-reload hook) calls
// restyleModalInput to re-apply them; this asserts that actually tracks.
func TestModalInputTracksThemeReload(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)
	defer styles.Apply(styles.DefaultTheme())

	m := openSSHModal(t, 120, 40)
	if got := m.sshUser.Cursor.Style.GetForeground(); got != styles.Primary {
		t.Fatalf("precondition: cursor foreground = %v, want Primary", got)
	}

	th := styles.DefaultTheme()
	th.PrimaryAccent = "#ff00ff"
	styles.Apply(th)
	m = m.resizeOverlay()

	if got := m.sshUser.Cursor.Style.GetForeground(); got != styles.Primary {
		t.Fatalf("cursor kept the old palette after a theme reload: %v, want %v", got, styles.Primary)
	}
}
