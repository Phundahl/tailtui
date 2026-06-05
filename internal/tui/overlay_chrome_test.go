package tui

import (
	"strings"
	"testing"

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
	ti := newRoutingInput()

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

	ti := newRoutingInput()
	ti.Width = 40
	ti.Focus()
	line := styles.ModalFill(64).Render(ti.View()) // exactly how routingBody wraps the field

	// A reset immediately followed by several spaces means those cells carry no
	// background — the glitch. A correctly styled pad re-opens a bg SGR first.
	if strings.Contains(line, "\x1b[0m     ") {
		t.Fatalf("CIDR field emits unstyled (black) padding spaces:\n%q", line)
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

	cur := newRoutingInput().Cursor
	cur.SetChar(" ")
	cur.Blink = false // force the visible (block) phase
	view := cur.View()

	if !strings.Contains(view, "\x1b[7") { // reverse video → Foreground becomes the block background
		t.Fatalf("cursor visible state is not a reverse-video block (would be invisible): %q", view)
	}
	if !strings.Contains(view, primSGR) {
		t.Fatalf("cursor block is not Primary-colored: %q", view)
	}
}
