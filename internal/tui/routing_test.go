package tui

import (
	"strings"
	"testing"
)

// Phase 21: Routing Management is bound to uppercase [R] (shift+r); lowercase
// `r` is reserved and must be a no-op.
func TestRoutingHotkeyIsUppercase(t *testing.T) {
	m := newReadyModel(t, 120, 40)

	m2, _ := m.Update(key("r")) // lowercase: reserved, ignored
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("lowercase r opened a modal (state=%v); it must be reserved", m.state)
	}

	m2, _ = m.Update(key("R")) // uppercase: opens routing
	m = m2.(Model)
	if m.state != stateRouting {
		t.Fatalf("uppercase R did not open routing (state=%v)", m.state)
	}
}

// Phase 21: the routing modal floats over the still-visible base view and shows
// the ACCOUNT_MANAGEMENT-style chrome, the exit-node toggle, and the keymap.
func TestRoutingOverlayRendersFlush(t *testing.T) {
	const w, h = 120, 40
	m := newReadyModel(t, w, h)
	m.prefs.AdvertiseExitNode = true
	m.prefs.AdvertiseRoutes = []string{"192.168.1.0/24", "10.0.0.0/16"}

	m2, _ := m.Update(key("R"))
	m = m2.(Model)

	view := m.View()
	assertFlush(t, view, w, h)
	for _, want := range []string{
		"ROUTING_MANAGEMENT",     // title chrome
		"Exit Node (Advertise):", // fixed toggle item
		"[ON]",                   // advertising exit node
		"192.168.1.0/24",         // advertised route
		"10.0.0.0/16",            // advertised route
		"NAVIGATE",               // keymap
		"CLOSE",                  // keymap
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("routing modal missing %q:\n%s", want, view)
		}
	}
	// True overlay: the base header must remain visible behind the modal.
	if !strings.Contains(view, appName) {
		t.Fatalf("header not visible behind modal — overlay blanked the base view")
	}
}

// Phase 21: with no advertised routes, the modal shows the placeholder and the
// only navigable item is the exit-node toggle.
func TestRoutingPlaceholderAndItemCount(t *testing.T) {
	m := newReadyModel(t, 120, 40) // zero prefs → no routes
	if got := m.routingItemCount(); got != 1 {
		t.Fatalf("routingItemCount = %d with no routes, want 1 (just the toggle)", got)
	}

	m2, _ := m.Update(key("R"))
	m = m2.(Model)
	if !strings.Contains(m.View(), "No custom routes advertised.") {
		t.Fatalf("empty routing modal missing placeholder:\n%s", m.View())
	}
}

// Phase 21: j/k navigate the toggle + route list and clamp at both ends.
func TestRoutingNavigationClamps(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m.prefs.AdvertiseRoutes = []string{"192.168.1.0/24", "10.0.0.0/16"} // itemCount = 3

	m2, _ := m.Update(key("R"))
	m = m2.(Model)
	if m.routingCursor != 0 {
		t.Fatalf("routingCursor = %d on open, want 0", m.routingCursor)
	}

	// Down past the end clamps at itemCount-1 (=2).
	for i := 0; i < 5; i++ {
		m2, _ = m.Update(key("j"))
		m = m2.(Model)
	}
	if m.routingCursor != 2 {
		t.Fatalf("routingCursor = %d after spamming j, want 2 (clamped)", m.routingCursor)
	}

	// Up past the top clamps at 0.
	for i := 0; i < 5; i++ {
		m2, _ = m.Update(key("k"))
		m = m2.(Model)
	}
	if m.routingCursor != 0 {
		t.Fatalf("routingCursor = %d after spamming k, want 0 (clamped)", m.routingCursor)
	}

	// Esc closes back to the main view.
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("Esc did not close routing (state=%v)", m.state)
	}
}
