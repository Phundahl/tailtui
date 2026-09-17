package tui

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

// newModelWithPeers returns a ready model whose peer list is populated, by
// folding a statusMsg through Update — the same path applyStatus takes in
// production. The rest of the suite uses newReadyModel, which builds an EMPTY
// list, so anything gated on a selected peer needs this instead.
func newModelWithPeers(t *testing.T, w, h int, peers ...types.Peer) Model {
	t.Helper()
	m := newReadyModel(t, w, h)
	m2, _ := m.Update(statusMsg{
		local: types.LocalStatus{Hostname: "tailtui-test", Conn: types.ConnDirect},
		peers: peers,
	})
	return m2.(Model)
}

func sshTestPeer(host string, online, offersSSH, offersExit bool) types.Peer {
	conn := types.ConnOffline
	if online {
		conn = types.ConnDirect
	}
	return types.Peer{
		ID: "id-" + host, Hostname: host, DNSName: host + ".tailtui.dev",
		OS: types.OSLinux, TailscaleIP: "100.64.0.9",
		Conn: conn, Online: online, LastSeen: "Connected",
		NodeType: types.NodeRegular, OffersSSH: offersSSH, OffersExitNode: offersExit,
	}
}

// openSSHModal seeds one online, SSH-capable peer and opens the launcher on it.
func openSSHModal(t *testing.T, w, h int) Model {
	t.Helper()
	m := newModelWithPeers(t, w, h, sshTestPeer("srv-web-01", true, true, false))
	m2, _ := m.Update(key("s"))
	m = m2.(Model)
	if m.state != stateSSH {
		t.Fatalf("[s] did not open the SSH launcher (state=%v)", m.state)
	}
	return m
}

// --- gating ------------------------------------------------------------------

func TestSSHOpensOnOnlinePeer(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	if m.sshPeer.Hostname != "srv-web-01" {
		t.Fatalf("sshPeer = %q, want srv-web-01", m.sshPeer.Hostname)
	}
}

// `[s]` is offered on any ONLINE peer, including one with no Tailscale SSH
// server — it may still run a plain sshd, and the modal says which case it is.
func TestSSHOpensOnPeerWithoutSSHServer(t *testing.T) {
	m := newModelWithPeers(t, 120, 40, sshTestPeer("home-nas", true, false, false))
	m2, _ := m.Update(key("s"))
	m = m2.(Model)
	if m.state != stateSSH {
		t.Fatalf("[s] should open on any online peer (state=%v)", m.state)
	}
	if !strings.Contains(m.View(), "[NOT ADVERTISED]") {
		t.Fatalf("modal should flag a peer with no Tailscale SSH server:\n%s", m.View())
	}
}

func TestSSHNoopOnOfflinePeer(t *testing.T) {
	m := newModelWithPeers(t, 120, 40, sshTestPeer("field-laptop", false, true, false))
	m2, cmd := m.Update(key("s"))
	m = m2.(Model)
	if m.state != stateMain {
		t.Fatalf("[s] on an offline peer opened %v, want stateMain", m.state)
	}
	if cmd != nil {
		t.Fatalf("[s] on an offline peer dispatched a command")
	}
}

func TestSSHNoopWithNoSelection(t *testing.T) {
	m := newReadyModel(t, 120, 40) // empty peer list
	m2, cmd := m.Update(key("s"))
	if m2.(Model).state != stateMain || cmd != nil {
		t.Fatalf("[s] with no selection should be inert")
	}
}

// --- rendering ---------------------------------------------------------------

func TestSSHOverlayRendersFlush(t *testing.T) {
	const w, h = 120, 40
	m := openSSHModal(t, w, h)
	view := m.View()
	assertFlush(t, view, w, h)
	for _, want := range []string{
		"SSH_LAUNCH", "srv-web-01", "ABOUT TO EXECUTE:", "tailscale ssh",
		"[ADVERTISED]", "EDIT USER", "CONNECT", "COPY TO CLIPBOARD", "CANCEL",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("launcher missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, appName) {
		t.Fatalf("header not visible behind modal — overlay blanked the base view")
	}
}

// overlayCenter silently DROPS foreground lines past the background height, so
// an over-tall modal loses its bottom (the keymap) without breaking flush.
func TestSSHOverlayFitsMinimumTerminal(t *testing.T) {
	const w, h = 72, 24
	m := openSSHModal(t, w, h)
	view := m.View()
	assertFlush(t, view, w, h)
	if !strings.Contains(view, "CANCEL") {
		t.Fatalf("keymap dropped at %dx%d — modal body is too tall:\n%s", w, h, view)
	}
}

// The Command Room's core guarantee: what you are shown is what runs.
func TestSSHPreviewMatchesExecutedCommand(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	want := tailscale.SSHCommandString(m.sshUser.Value(), sshTargetHost(m.sshPeer))
	if got := m.sshCommandString(); got != want {
		t.Fatalf("sshCommandString = %q, want %q", got, want)
	}
	if !strings.Contains(m.View(), want) {
		t.Fatalf("preview %q absent from the rendered modal:\n%s", want, m.View())
	}
}

func TestSSHCapabilityMarkerInList(t *testing.T) {
	// Hostnames deliberately free of the substring "ssh", so the assertions
	// below can only match the capability marker itself.
	m := newModelWithPeers(t, 120, 40,
		sshTestPeer("alpha-node", true, true, false),
		sshTestPeer("beta-node", true, false, false),
	)
	// Scan for the marker anywhere on a row naming each peer. Other panes (the
	// PEER DETAILS title) also name the selection, so this is an ANY check
	// rather than a per-line assertion.
	var marked, wronglyMarked bool
	for _, ln := range strings.Split(m.View(), "\n") {
		if strings.Contains(ln, "alpha-node") && strings.Contains(ln, "ssh") {
			marked = true
		}
		if strings.Contains(ln, "beta-node") && strings.Contains(ln, "ssh") {
			wronglyMarked = true
		}
	}
	if !marked {
		t.Fatalf("SSH-capable peer has no list marker:\n%s", m.View())
	}
	if wronglyMarked {
		t.Fatalf("peer without Tailscale SSH was marked:\n%s", m.View())
	}
}

// Both contextual hints can coexist in PEER DETAILS without busting the pane.
func TestDetailsShowsBothHintsFlush(t *testing.T) {
	const w, h = 120, 40
	p := sshTestPeer("dc-subnet-router", true, true, false)
	p.NodeType = types.NodeSubnetRouter
	p.AdvertisedRoutes = []string{"192.168.10.0/24", "192.168.20.0/24"}
	m := newModelWithPeers(t, w, h, p)
	view := m.View()
	assertFlush(t, view, w, h)
	for _, want := range []string{"advertised routes", "[s] Tailscale SSH"} {
		if !strings.Contains(view, want) {
			t.Fatalf("PEER DETAILS missing %q:\n%s", want, view)
		}
	}
}

// styles.Bar clips the LEFT segment, so the last hint on the left is the canary
// — not the right-hand version string.
func TestFooterKeepsAllHintsAt120(t *testing.T) {
	const w, h = 120, 40
	worst := sshTestPeer("exit-and-ssh", true, true, true) // both hints active
	for _, tc := range []struct {
		name     string
		operator string
		wantO    bool
	}{
		{"operator configured", currentUser(), false},
		{"operator not yet set", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newModelWithPeers(t, w, h, worst)
			m.prefs.OperatorUser = tc.operator
			view := m.View()
			assertFlush(t, view, w, h)
			footer := strings.Split(view, "\n")[h-1]
			if !strings.Contains(footer, "[v] Logs") {
				t.Fatalf("footer clipped, [v] Logs truncated:\n%q", footer)
			}
			if !strings.Contains(footer, "[s] SSH") {
				t.Fatalf("footer missing the SSH hint:\n%q", footer)
			}
			if got := strings.Contains(footer, "[O] Operator"); got != tc.wantO {
				t.Fatalf("[O] Operator shown=%v, want %v:\n%q", got, tc.wantO, footer)
			}
		})
	}
}

// --- key handling ------------------------------------------------------------

func TestSSHUsernameDefaultsToCurrentUser(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	if got := m.sshUser.Value(); got != currentUser() {
		t.Fatalf("username defaulted to %q, want %q", got, currentUser())
	}
}

func TestSSHEditUsername(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, _ := m.Update(key("e"))
	m = m2.(Model)
	if !m.sshInputMode {
		t.Fatalf("[e] did not enter input mode")
	}
	m.sshUser.SetValue("")
	for _, r := range "root" {
		m2, _ = m.Update(key(string(r)))
		m = m2.(Model)
	}
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if m.sshInputMode {
		t.Fatalf("Enter did not leave input mode")
	}
	if m.sshUser.Value() != "root" {
		t.Fatalf("username = %q, want root", m.sshUser.Value())
	}
	if !strings.Contains(m.View(), "root@srv-web-01.tailtui.dev") {
		t.Fatalf("preview did not follow the edited username:\n%s", m.View())
	}
}

// THE guard-ordering regression: input mode must own every key, including the
// ones that close overlays globally. A username may legitimately contain a `q`.
func TestSSHInputModeSwallowsQuitKeys(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, _ := m.Update(key("e"))
	m = m2.(Model)
	m.sshUser.SetValue("")
	m2, _ = m.Update(key("q"))
	m = m2.(Model)
	if m.state != stateSSH {
		t.Fatalf("`q` while editing closed the modal (state=%v)", m.state)
	}
	if m.sshUser.Value() != "q" {
		t.Fatalf("`q` did not reach the editor: value=%q", m.sshUser.Value())
	}
	// Esc cancels the EDIT, not the modal.
	m2, _ = m.Update(key("esc"))
	m = m2.(Model)
	if m.state != stateSSH {
		t.Fatalf("Esc in input mode closed the modal instead of cancelling the edit")
	}
	if m.sshInputMode {
		t.Fatalf("Esc did not leave input mode")
	}
	// A second Esc, now in nav mode, closes.
	m2, _ = m.Update(key("esc"))
	if m2.(Model).state != stateMain {
		t.Fatalf("Esc in nav mode did not close the launcher")
	}
}

func TestSSHInvalidUsernameRejected(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, _ := m.Update(key("e"))
	m = m2.(Model)
	m.sshUser.SetValue("bad user")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	if !m.sshUserErr {
		t.Fatalf("invalid username was accepted")
	}
	if !m.sshInputMode {
		t.Fatalf("rejected entry should stay in input mode")
	}
	if m.sshLastUser != "" {
		t.Fatalf("rejected username leaked into session memory: %q", m.sshLastUser)
	}
}

func TestSSHUsernameStickyForSession(t *testing.T) {
	m := newModelWithPeers(t, 120, 40,
		sshTestPeer("srv-web-01", true, true, false),
		sshTestPeer("srv-web-02", true, true, false),
	)
	m2, _ := m.Update(key("s"))
	m = m2.(Model)
	m2, _ = m.Update(key("e"))
	m = m2.(Model)
	m.sshUser.SetValue("deploy")
	m2, _ = m.Update(key("enter"))
	m = m2.(Model)
	m2, _ = m.Update(key("esc")) // close the modal
	m = m2.(Model)

	// Reopen (on the same selection or another peer): the username persists.
	m2, _ = m.Update(key("s"))
	m = m2.(Model)
	if got := m.sshUser.Value(); got != "deploy" {
		t.Fatalf("username not remembered across opens: %q", got)
	}
}

func TestSSHEscCancels(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, cmd := m.Update(key("esc"))
	if m2.(Model).state != stateMain {
		t.Fatalf("Esc did not close the launcher")
	}
	if cmd != nil {
		t.Fatalf("Esc dispatched a command")
	}
}

func TestSSHEnterLaunches(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, cmd := m.Update(key("enter"))
	m = m2.(Model)
	// Closed BEFORE dispatch, so the post-ExecProcess repaint lands on the
	// dashboard rather than a stale modal frame.
	if m.state != stateMain {
		t.Fatalf("Enter left state=%v, want stateMain", m.state)
	}
	if cmd == nil {
		t.Fatalf("Enter did not dispatch the launch command")
	}
}

func TestSSHCopyStaysInModal(t *testing.T) {
	m := openSSHModal(t, 120, 40)
	m2, cmd := m.Update(key("c"))
	m = m2.(Model)
	if m.state != stateSSH {
		t.Fatalf("[c] closed the modal (state=%v)", m.state)
	}
	if cmd == nil {
		t.Fatalf("[c] did not dispatch a clipboard command")
	}
	m2, _ = m.Update(clipboardMsg{kind: clipboardSSH})
	m = m2.(Model)
	if !m.sshCopied {
		t.Fatalf("clipboardSSH did not set sshCopied")
	}
	if m.routingCopied {
		t.Fatalf("clipboardSSH wrongly flashed the routing indicator")
	}
	if !strings.Contains(m.View(), "Copied") {
		t.Fatalf("copy indicator not rendered:\n%s", m.View())
	}
}

// --- result logging ----------------------------------------------------------

func TestSSHDoneMsgLogging(t *testing.T) {
	// A non-zero exit is the remote command's status, not a tailTUI failure, so
	// it must be WARN — never a red ERROR in the ring.
	exitErr := exec.Command("sh", "-c", "exit 3").Run()
	if exitErr == nil {
		t.Fatalf("expected a non-zero exit to produce an error")
	}
	for _, tc := range []struct {
		name      string
		msg       sshDoneMsg
		wantLevel string
		wantCmd   bool
	}{
		{"mock", sshDoneMsg{target: "a@b", mock: true}, "INFO", false},
		{"clean exit", sshDoneMsg{target: "a@b"}, "INFO", true},
		{"non-zero exit", sshDoneMsg{target: "a@b", err: exitErr}, "WARN", true},
		{"launch failure", sshDoneMsg{target: "a@b", err: errFake{}}, "ERROR", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newReadyModel(t, 120, 40)
			m2, cmd := m.Update(tc.msg)
			m = m2.(Model)
			if len(m.logs) == 0 {
				t.Fatalf("no log entry appended")
			}
			last := m.logs[len(m.logs)-1]
			if last.Level != tc.wantLevel {
				t.Fatalf("log level = %q, want %q (%q)", last.Level, tc.wantLevel, last.Message)
			}
			if (cmd != nil) != tc.wantCmd {
				t.Fatalf("refresh cmd present = %v, want %v", cmd != nil, tc.wantCmd)
			}
		})
	}
}

type errFake struct{}

func (errFake) Error() string { return "dial failed" }
