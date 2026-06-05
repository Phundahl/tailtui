package tui

import (
	"strings"
	"testing"
)

// openRoutingModal returns a ready model with the routing modal open and the
// given routes/exit-node snapshotted into the working copy.
func openRoutingModal(t *testing.T, exit bool, routes ...string) Model {
	t.Helper()
	m := newReadyModel(t, 120, 40)
	m.prefs.AdvertiseExitNode = exit
	m.prefs.AdvertiseRoutes = routes
	m2, _ := m.Update(key("R"))
	return m2.(Model)
}

// Phase 22: Space toggles the exit-node advertise flag in local state only.
func TestRoutingToggleExitNode(t *testing.T) {
	m := openRoutingModal(t, false)
	if m.routingCursor != 0 {
		t.Fatalf("cursor = %d on open, want 0 (exit toggle)", m.routingCursor)
	}

	m2, _ := m.Update(key("space"))
	m = m2.(Model)
	if !m.routingExitNode {
		t.Fatalf("Space did not toggle exit node ON")
	}
	if !m.routingDirty {
		t.Fatalf("toggle did not mark the working copy dirty")
	}
	if !strings.Contains(m.View(), "[ON]") {
		t.Fatalf("view does not show [ON] after toggle:\n%s", m.View())
	}
	// prefs (daemon truth) must be untouched — no CLI side effect this phase.
	if m.prefs.AdvertiseExitNode {
		t.Fatalf("toggle leaked into m.prefs (should stay local)")
	}

	m2, _ = m.Update(key("space"))
	m = m2.(Model)
	if m.routingExitNode {
		t.Fatalf("second Space did not toggle exit node back OFF")
	}
}

// Phase 22: Space is a no-op on a route row (only the exit-node item toggles).
func TestRoutingToggleOnlyOnExitItem(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24")
	m2, _ := m.Update(key("j")) // cursor → route row
	m = m2.(Model)
	before := m.routingExitNode
	m2, _ = m.Update(key("space"))
	m = m2.(Model)
	if m.routingExitNode != before {
		t.Fatalf("Space on a route row toggled the exit node")
	}
}

// Phase 22: [d] removes the highlighted route and clamps the cursor when the
// last item is deleted.
func TestRoutingDeleteRouteClamps(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24", "10.0.0.0/16")

	// Highlight the first route (cursor 1) and delete it.
	m2, _ := m.Update(key("j"))
	m = m2.(Model)
	m2, _ = m.Update(key("d"))
	m = m2.(Model)
	if len(m.routingRoutes) != 1 || m.routingRoutes[0] != "10.0.0.0/16" {
		t.Fatalf("delete removed the wrong route: %v", m.routingRoutes)
	}
	if !m.routingDirty {
		t.Fatalf("delete did not mark dirty")
	}

	// Cursor is on the last (now only) route; delete it → list empties, cursor
	// must clamp back to the exit-node toggle (0) without panicking.
	m2, _ = m.Update(key("d"))
	m = m2.(Model)
	if len(m.routingRoutes) != 0 {
		t.Fatalf("second delete left routes: %v", m.routingRoutes)
	}
	if m.routingCursor != 0 {
		t.Fatalf("cursor = %d after deleting last route, want 0 (clamped)", m.routingCursor)
	}
	if !strings.Contains(m.View(), "No custom routes advertised.") {
		t.Fatalf("emptied list missing placeholder:\n%s", m.View())
	}
}

// Phase 22: [a] enters input mode, a valid CIDR is appended on Enter.
func TestRoutingAddValidCIDR(t *testing.T) {
	m := openRoutingModal(t, false)

	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	if !m.routingInputMode {
		t.Fatalf("[a] did not enter input mode")
	}
	// Dynamic keymap swaps to CONFIRM/CANCEL + the prompt.
	view := m.View()
	if !strings.Contains(view, "CONFIRM") || !strings.Contains(view, "CANCEL") {
		t.Fatalf("input-mode keymap missing CONFIRM/CANCEL:\n%s", view)
	}
	if !strings.Contains(view, "Enter CIDR") {
		t.Fatalf("input-mode prompt missing:\n%s", view)
	}
	assertFlush(t, view, 120, 40)

	m2, _ = m.Update(key("10.1.0.0/16"))
	m = m2.(Model)
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)

	if m.routingInputMode {
		t.Fatalf("input mode did not exit after a valid Enter")
	}
	if len(m.routingRoutes) != 1 || m.routingRoutes[0] != "10.1.0.0/16" {
		t.Fatalf("valid CIDR not appended: %v", m.routingRoutes)
	}
	if m.routingCursor != 1 {
		t.Fatalf("cursor = %d after add, want 1 (the new route)", m.routingCursor)
	}
	if !m.routingDirty {
		t.Fatalf("add did not mark dirty")
	}
}

// Phase 22: an invalid CIDR is rejected — flagged, cleared, stays in input mode,
// no crash, nothing appended.
func TestRoutingAddInvalidCIDR(t *testing.T) {
	m := openRoutingModal(t, false)
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	m2, _ = m.Update(key("not-a-cidr"))
	m = m2.(Model)
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)

	if !m.routingInputMode {
		t.Fatalf("invalid Enter should keep input mode active")
	}
	if !m.routingInputErr {
		t.Fatalf("invalid CIDR did not set the error flag")
	}
	if len(m.routingRoutes) != 0 {
		t.Fatalf("invalid CIDR was appended: %v", m.routingRoutes)
	}
	if v := m.routingInput.Value(); v != "" {
		t.Fatalf("invalid input not cleared, value = %q", v)
	}
	assertFlush(t, m.View(), 120, 40)
}

// Phase 22: Esc in input mode cancels (no append) and returns to list nav —
// it must NOT close the whole modal.
func TestRoutingInputEscCancels(t *testing.T) {
	m := openRoutingModal(t, false)
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	m2, _ = m.Update(key("10.0.0.0/8"))
	m = m2.(Model)
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)

	if m.state != stateRouting {
		t.Fatalf("Esc in input mode closed the modal (state=%v); it should only cancel", m.state)
	}
	if m.routingInputMode {
		t.Fatalf("Esc did not leave input mode")
	}
	if len(m.routingRoutes) != 0 {
		t.Fatalf("Esc-cancel still appended a route: %v", m.routingRoutes)
	}

	// A second Esc (now in list mode) closes the modal as usual.
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("Esc in list mode did not close the modal (state=%v)", m.state)
	}
}

// Phase 22 Addendum: deleting a route stashes it, and the next [a] pre-fills the
// editor with that CIDR (cursor at end) — a lightweight undo / edit-typo flow.
func TestRoutingDeletedRoutePrefillsAdd(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24", "10.0.0.0/16")

	// On open the undo buffer is empty: [a] starts with a blank field.
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	if v := m.routingInput.Value(); v != "" {
		t.Fatalf("fresh add field should be empty, got %q", v)
	}
	m2, _ = m.Update(key("esc")) // cancel back to list
	m = m2.(Model)

	// Highlight + delete the first route.
	m2, _ = m.Update(key("j"))
	m = m2.(Model)
	m2, _ = m.Update(key("d"))
	m = m2.(Model)
	if m.lastDeletedRoute != "192.168.1.0/24" {
		t.Fatalf("lastDeletedRoute = %q, want the deleted CIDR", m.lastDeletedRoute)
	}

	// [a] now pre-fills with the deleted CIDR, cursor at the end.
	m2, _ = m.Update(key("a"))
	m = m2.(Model)
	if v := m.routingInput.Value(); v != "192.168.1.0/24" {
		t.Fatalf("add field not pre-filled with deleted route, got %q", v)
	}
	if pos, want := m.routingInput.Position(), len("192.168.1.0/24"); pos != want {
		t.Fatalf("cursor at %d, want end (%d)", pos, want)
	}

	// Confirming pops the route straight back in (undo) — and it still passes the
	// Phase 22 net.ParseCIDR validation.
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.routingInputMode {
		t.Fatalf("valid pre-filled CIDR should have been accepted")
	}
	found := false
	for _, r := range m.routingRoutes {
		if r == "192.168.1.0/24" {
			found = true
		}
	}
	if !found {
		t.Fatalf("undo did not restore the route: %v", m.routingRoutes)
	}
}

// Phase 22 Addendum: the pre-fill is editable — a typo'd restore can be fixed
// before resubmitting, and invalid edits are still rejected by validation.
func TestRoutingPrefillEditableAndValidated(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24")
	m2, _ := m.Update(key("j"))
	m = m2.(Model)
	m2, _ = m.Update(key("d")) // delete → stash
	m = m2.(Model)
	m2, _ = m.Update(key("a")) // pre-fill
	m = m2.(Model)

	// Append junk to the pre-filled value (cursor is at the end) → invalid CIDR.
	m2, _ = m.Update(key("zz"))
	m = m2.(Model)
	if v := m.routingInput.Value(); v != "192.168.1.0/24zz" {
		t.Fatalf("typing did not append at the cursor end, got %q", v)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if !m.routingInputErr || !m.routingInputMode {
		t.Fatalf("invalid edited CIDR should be rejected and stay in input mode")
	}
}

// Phase 22: the list-mode keymap advertises all the new actions.
func TestRoutingListKeymap(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24")
	view := m.View()
	for _, want := range []string{"NAVIGATE", "ADD ROUTE", "TOGGLE", "REMOVE", "CLOSE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("list keymap missing %q:\n%s", want, view)
		}
	}
}
