package tui

import (
	"strings"
	"testing"
)

// Phase 23: Enter in the routing list opens the "Command Room" confirmation,
// which floats over the still-visible base and shows the exact command,
// the Admin Console reminder, and the apply/copy/back keymap.
func TestRoutingEnterOpensConfirm(t *testing.T) {
	const w, h = 120, 40
	m := openRoutingModal(t, true, "192.168.1.0/24", "10.0.0.0/16")

	m2, _ := m.Update(key("enter"))
	m = m2.(Model)
	if m.state != stateRoutingConfirm {
		t.Fatalf("Enter did not open the confirm modal (state=%v)", m.state)
	}

	view := m.View()
	assertFlush(t, view, w, h)
	for _, want := range []string{
		"CONFIRM ROUTING CHANGES",
		"ABOUT TO EXECUTE:",
		"tailscale set", // the command preview (long commands wrap, so check tokens)
		"--advertise-exit-node=true",
		"--advertise-routes=192.168.1.0/24,10.0.0.0/16",
		"Admin Console",
		"APPLY",
		"COPY TO CLIPBOARD",
		"BACK",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirm modal missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, appName) {
		t.Fatalf("header not visible behind confirm modal — base was blanked")
	}
}

// Phase 23: Esc in the confirm modal returns to the routing list WITHOUT
// applying — it must not close the whole feature.
func TestRoutingConfirmEscGoesBack(t *testing.T) {
	m := openRoutingModal(t, false, "192.168.1.0/24")
	m2, _ := m.Update(key("enter")) // → confirm
	m = m2.(Model)
	m2, _ = m.Update(key("esc")) // → back to list
	m = m2.(Model)
	if m.state != stateRouting {
		t.Fatalf("Esc in confirm went to %v, want stateRouting (back to list)", m.state)
	}
	// A further Esc closes the modal as usual.
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("Esc in list did not close the modal (state=%v)", m.state)
	}
}

// Phase 23: Enter in the confirm modal applies asynchronously and leaves the
// modal entirely (returns a command; never blocks).
func TestRoutingConfirmApplyExecutes(t *testing.T) {
	m := openRoutingModal(t, true, "192.168.1.0/24")
	m2, _ := m.Update(key("enter")) // → confirm
	m = m2.(Model)
	m2, cmd := m.Update(key("enter")) // apply
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("apply did not leave the modal (state=%v)", m.state)
	}
	if cmd == nil {
		t.Fatalf("apply did not return an async command")
	}

	// The follow-up routingActionMsg logs the outcome and refreshes (batched cmd).
	m2, refresh := m.Update(routingActionMsg{desc: "tailscale set --advertise-exit-node=true --advertise-routes=192.168.1.0/24", err: nil})
	m = m2.(Model)
	if refresh == nil {
		t.Fatalf("routingActionMsg did not trigger a status/prefs refresh")
	}
	last := m.logs[len(m.logs)-1]
	if last.Level != "INFO" || !strings.Contains(last.Message, "applied:") {
		t.Fatalf("routingActionMsg did not log the applied command: %+v", last)
	}
}

// Phase 23: [C] in the confirm modal copies the command (returns a cmd, stays in
// the modal); a successful clipboardMsg flashes the Copied! indicator.
func TestRoutingConfirmCopy(t *testing.T) {
	m := openRoutingModal(t, false, "10.0.0.0/16")
	m2, _ := m.Update(key("enter")) // → confirm
	m = m2.(Model)

	m2, cmd := m.Update(key("c"))
	m = m2.(Model)
	if m.state != stateRoutingConfirm {
		t.Fatalf("[C] left the confirm modal (state=%v)", m.state)
	}
	if cmd == nil {
		t.Fatalf("[C] did not return a clipboard command")
	}

	// Simulate the copy completing successfully.
	m2, _ = m.Update(clipboardMsg{err: nil})
	m = m2.(Model)
	if !m.routingCopied {
		t.Fatalf("clipboardMsg success did not set routingCopied")
	}
	if !strings.Contains(m.View(), "Copied to clipboard!") {
		t.Fatalf("confirm modal did not flash the Copied! indicator:\n%s", m.View())
	}
}
