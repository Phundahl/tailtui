package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

// SSH launcher (Phase 34) — the `[s]` modal that previews and runs
// `tailscale ssh [user@]host` for the highlighted peer.
//
// TWO SUB-MODES, and this is a forced call rather than a preference: an
// always-focused username field and a `[c]` copy key are mutually exclusive,
// because `c` cannot both type a "c" and copy to the clipboard. So the modal
// opens in NAV mode with the field blurred and pre-filled, and `[e]` enters
// INPUT mode. The username is already correct in the overwhelming majority of
// launches (currentUser, then session-sticky), so blurred-by-default optimizes
// the common path — and it reuses the Routing modal's list/input split verbatim,
// leaving one pattern in the codebase rather than two. (ctrl+c is unavailable as
// a copy key: Update intercepts it as quit before overlay routing happens.)
//
// Gating is deliberately looser than the exit-node action: `[s]` is offered on
// ANY online peer, not only those advertising Tailscale SSH, because a peer
// without the Tailscale SSH server may still be reachable via its own sshd. The
// modal states which case you are in before the terminal suspends.

// newSSHUserInput builds the username editor. 32 is the practical POSIX
// username ceiling; the styling contract lives on newModalInput.
func newSSHUserInput() textinput.Model { return newModalInput(32) }

// sshTargetHost picks the host token to connect to. MagicDNS name first (what
// `tailscale ssh` resolves best), then the hostname, then the raw Tailscale IP
// as a last resort so the action still works on a peer with no DNS name.
func sshTargetHost(p types.Peer) string {
	if h := strings.TrimSpace(p.DNSName); h != "" {
		return h
	}
	if h := strings.TrimSpace(p.Hostname); h != "" {
		return h
	}
	return p.TailscaleIP
}

// sshDefaultUser resolves the username the editor opens with: the one confirmed
// earlier this session, else the local user, else empty (which yields a bare
// `tailscale ssh host` and lets ssh apply its own default).
func (m Model) sshDefaultUser() string {
	if m.sshLastUser != "" {
		return m.sshLastUser
	}
	return currentUser()
}

// validSSHUser rejects input that would not survive being joined into a single
// `user@host` argv token. This is a CORRECTNESS guard, not a security boundary:
// no shell is involved anywhere in this path (exec.Command takes an argv), so
// nothing here is protecting against injection.
func validSSHUser(s string) bool {
	if s == "" {
		return true // empty is legal — the command drops the "user@" prefix
	}
	return !strings.ContainsAny(s, " \t@/")
}

// openSSH transitions to the SSH launcher for the given peer, snapshotting it so
// a background status poll cannot swap the target mid-modal.
func (m Model) openSSH(p types.Peer) Model {
	m.state = stateSSH
	m.sshPeer = p
	m.sshInputMode = false
	m.sshUserErr = false
	m.sshCopied = false
	m.sshUser = newSSHUserInput()
	m.sshUser.Width = clampInputWidth(overlayWidth(m.width))
	m.sshUser.SetValue(m.sshDefaultUser())
	m.sshUser.CursorEnd()
	return m
}

// sshCommandString renders the exact command the launcher will execute.
func (m Model) sshCommandString() string {
	return tailscale.SSHCommandString(m.sshUser.Value(), sshTargetHost(m.sshPeer))
}

// updateSSH handles every key while the launcher is open, dispatching to the
// username editor when in input mode. See the file header for why the field is
// blurred by default.
func (m Model) updateSSH(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.sshInputMode {
		return m.updateSSHInput(msg)
	}
	switch msg.String() {
	case "e":
		m.sshInputMode = true
		m.sshUserErr = false
		m.sshUser.CursorEnd()
		m.sshUser.Focus()
		return m, nil
	case "enter":
		user := strings.TrimSpace(m.sshUser.Value())
		host := sshTargetHost(m.sshPeer)
		// Close BEFORE dispatch (the Phase 24.1 lesson): the terminal must not
		// be handed to a child process with a modal as the last drawn frame, so
		// the post-ExecProcess repaint lands on the dashboard.
		m.state = stateMain
		m.sshInputMode = false
		return m, sshLaunchCmd(user, host)
	case "c", "C":
		return m, copyCmd(clipboardSSH, m.sshCommandString())
	case "esc", "q":
		m.state = stateMain
		m.sshCopied = false
		return m, nil
	}
	return m, nil
}

// updateSSHInput owns every key while the username is being edited — including
// esc/q, which is why updateOverlay routes stateSSH before the global close.
func (m Model) updateSSHInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.sshUser.Value())
		if !validSSHUser(val) {
			m.sshUserErr = true
			return m, nil
		}
		m.sshUser.SetValue(val)
		m.sshLastUser = val // session-sticky, written only on a confirmed edit
		m.exitSSHInput()
		return m, nil
	case "esc":
		// Cancels the EDIT, not the modal.
		m.exitSSHInput()
		return m, nil
	}
	var cmd tea.Cmd
	m.sshUser, cmd = m.sshUser.Update(msg)
	m.sshUserErr = false
	return m, cmd
}

// exitSSHInput returns to nav mode, blurring the field so `[c]` means COPY again.
func (m *Model) exitSSHInput() {
	m.sshInputMode = false
	m.sshUserErr = false
	m.sshUser.Blur()
}

// renderSSHOverlay draws the launcher directly from model state (like the
// Command Room, not the shared viewport) so the command preview and the editor
// re-render on every keystroke.
//
// HEIGHT IS A LIVE CONSTRAINT: overlayCenter silently drops foreground lines
// past the background height, so an over-tall modal loses its BOTTOM — the
// keymap — without ever breaking the flush invariant. The body is budgeted to
// stay well inside the minimum supported terminal; TestSSHOverlayFitsMinimumTerminal
// guards it. Keep the keymap in two columns if this grows.
func (m Model) renderSSHOverlay(base string) string {
	w := overlayWidth(m.width)
	p := m.sshPeer

	lines := []string{
		modalLine(w, lipgloss.PlaceHorizontal(w, lipgloss.Center,
			styles.ModalTitle.Render("[ SSH_LAUNCH ]"),
			lipgloss.WithWhitespaceBackground(styles.Surface))),
		modalDivider(w),
		modalRow(w,
			styles.ModalDim.Render("TARGET:  ")+styles.ModalText.Render(p.Hostname),
			styles.ModalText.Render(connSymbol(p.Conn)+" "+connText(p.Conn, p.Relay))),
		modalLine(w, styles.ModalDim.Render("HOST:    ")+styles.ModalText.Render(sshTargetHost(p))),
	}

	sshState := styles.ModalDim.Render("[NOT ADVERTISED]")
	if p.OffersSSH {
		sshState = styles.StatusOK.Render("[ADVERTISED]")
	}
	lines = append(lines,
		modalLine(w, styles.ModalDim.Render("SSH:     ")+sshState),
		modalLine(w, ""),
	)

	// Username: heading doubles as the mode indicator and the error flash.
	switch {
	case m.sshUserErr:
		lines = append(lines, modalLine(w, styles.StatusErr.Render("USER — invalid, no spaces / @ / /")))
	case m.sshInputMode:
		lines = append(lines, modalLine(w, styles.ModalHeading.Render("USER")+styles.ModalDim.Render("  (enter to confirm)")))
	default:
		lines = append(lines, styles.ModalHeading.Render("USER")+styles.ModalDim.Render("  (press [e] to edit)"))
		lines[len(lines)-1] = modalLine(w, lines[len(lines)-1])
	}
	lines = append(lines,
		modalLine(w, m.sshUser.View()),
		modalLine(w, ""),
		modalLine(w, styles.ModalDim.Render("ABOUT TO EXECUTE:")),
	)
	for _, ln := range wrapCommand(m.sshCommandString(), w) {
		lines = append(lines, modalLine(w, styles.ModalAccent.Render(ln)))
	}

	lines = append(lines, modalLine(w, ""))
	// The structural twin of the Command Room's Admin Console reminder: per-peer
	// ACL permission is not exposed by the CLI, so a warning up front is the only
	// possible mitigation for a connection the tailnet will refuse.
	warn := lipgloss.NewStyle().Foreground(styles.Subtle).Background(styles.Surface).Italic(true)
	for _, ln := range wrapText("* Requires Tailscale SSH on the target and an ACL rule granting you access.", w) {
		lines = append(lines, modalLine(w, warn.Render(ln)))
	}

	lines = append(lines, modalDivider(w))
	if m.sshInputMode {
		lines = append(lines, gridLine(w, accountKey("ENTER", "CONFIRM", false), accountKey("ESC", "CANCEL EDIT", false)))
		lines = append(lines, modalLine(w, styles.ModalDim.Render("-- KEYBOARD INPUT MODE ACTIVE --")))
	} else {
		lines = append(lines,
			gridLine(w, accountKey("E", "EDIT USER", false), accountKey("ENTER", "CONNECT", false)),
			gridLine(w, accountKey("C", "COPY TO CLIPBOARD", false), accountKey("ESC", "CANCEL", false)),
		)
	}
	if m.sshCopied {
		lines = append(lines, modalLine(w, styles.StatusOK.Render("✓ Copied to clipboard!")))
	}

	inner := strings.Join(lines, "\n")
	modal := lipgloss.NewStyle().
		Width(w+2*modalHPad).
		Height(countLines(inner)+2*modalVPad).
		Background(styles.Surface).
		Foreground(styles.Fg).
		Padding(modalVPad, modalHPad).
		Border(lipgloss.NormalBorder()).
		BorderForeground(styles.Primary).
		BorderBackground(styles.Surface).
		Render(inner)

	return overlayCenter(base, modal)
}
