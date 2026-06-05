package tui

import (
	"net"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/tailscale"
)

// Routing Management modal — a floating overlay, styled after ACCOUNT_MANAGEMENT,
// for managing this node's advertised exit-node + subnet routes.
//
// Phase 21 built the read-only layout; Phase 22 adds LOCAL editing: Space
// toggles the exit-node advertise flag, [d] removes the highlighted route, and
// [a] opens a CIDR text editor (net.ParseCIDR-validated) to append one. All
// edits land in the working copy (Model.routingExitNode / routingRoutes) only —
// NO `tailscale set` runs here; the diff is applied to the daemon in Phase 23.
//
// Navigable items: index 0 = the Exit Node (Advertise) toggle, then one index
// per advertised subnet route. routingItemCount() is the clamp bound.

// routingItemCount returns the number of navigable rows: the exit-node toggle
// plus each advertised route. The "no routes" placeholder is not navigable, so
// the count is at least 1 (the toggle).
func (m Model) routingItemCount() int {
	return 1 + len(m.routingRoutes)
}

// newRoutingInput builds the CIDR text editor, themed to blend into the modal
// Surface.
//
// The "black box" (Phase 23.2): a textinput with a Placeholder set renders, when
// empty, via placeholderView, which fills the remainder of the field's Width
// with RAW unstyled spaces — a default-background (near-black) block that no
// outer Surface wrap can recolor (the spaces sit mid-line, after a reset). With
// NO placeholder the main render path pads instead with TextStyle, which carries
// the Surface background, so every cell of the field is Surface. The prompt label
// above the field already shows the "(e.g., 192.168.1.0/24)" example, so dropping
// the in-field placeholder loses nothing.
//
// The cursor must be a clearly VISIBLE bright block. bubbles/cursor draws its
// visible cell with Style.Reverse(true), which swaps fg/bg at display time — so
// the DISPLAYED background is Style's Foreground. We therefore set Foreground to
// Primary (becomes the bright block background) and Background to Bg (becomes the
// glyph color), giving a solid Primary block with a dark glyph — never invisible
// (the Phase 23.2 fg-Surface camouflage) and never a black block. Cursor.TextStyle
// keeps the blink-"off" phase rendering as normal text on the Surface.
func newRoutingInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 64
	ti.PromptStyle = styles.ModalAccent
	ti.TextStyle = styles.ModalText
	ti.Cursor.TextStyle = styles.ModalText
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(styles.Primary).Background(styles.Bg)
	return ti
}

// openRouting transitions to the Routing Management modal, snapshotting the live
// advertised routes from prefs into the editable working copy (the caller
// refreshes prefs via fetchPrefsCmd). The cursor starts on the Exit Node toggle,
// and the modal opens in list (navigation) mode.
func (m Model) openRouting() Model {
	m.state = stateRouting
	m.routingCursor = 0
	m.routingExitNode = m.prefs.AdvertiseExitNode
	m.routingRoutes = append([]string(nil), m.prefs.AdvertiseRoutes...)
	m.routingDirty = false
	m.routingInputMode = false
	m.routingInputErr = false
	m.lastDeletedRoute = "" // pseudo-undo buffer is scoped to a modal session
	m.routingInput = newRoutingInput()

	w := overlayWidth(m.width)
	m.routingInput.Width = clampInputWidth(w)
	content := m.routingBody(w)
	m.overlay = newOverlayVP(w, overlayHeight(m.height, countLines(content)), content)
	return m
}

// clampInputWidth keeps the text editor's visible window inside the modal.
func clampInputWidth(modalInnerW int) int {
	w := modalInnerW - 4
	if w < 8 {
		w = 8
	}
	if w > 40 {
		w = 40
	}
	return w
}

// refreshRoutingOverlay re-renders the modal content AND resizes the viewport to
// the new line count — necessary because switching to/from input mode changes
// the body height, and a stale viewport would clip the input field.
func (m *Model) refreshRoutingOverlay() {
	content := m.routingBody(m.overlay.Width)
	m.overlay.SetContent(content)
	m.overlay.Height = overlayHeight(m.height, countLines(content))
}

// updateRoutingInput owns every key while the CIDR editor is focused (input
// mode): Enter validates with net.ParseCIDR and appends on success; Esc cancels;
// anything else edits the field. It consumes esc/q so the global close handler
// can't fire mid-edit.
func (m Model) updateRoutingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.routingInput.Value())
		if _, _, err := net.ParseCIDR(val); err == nil {
			// Valid: append to the working copy, focus it, leave input mode.
			m.routingRoutes = append(m.routingRoutes, val)
			m.routingDirty = true
			m.routingCursor = len(m.routingRoutes) // index of the new route (1-based after the toggle)
			m.exitRoutingInput()
			m.refreshRoutingOverlay()
			return m, nil
		}
		// Invalid: flash red and clear, stay in input mode (no crash).
		m.routingInputErr = true
		m.routingInput.SetValue("")
		m.refreshRoutingOverlay()
		return m, nil
	case "esc":
		// Cancel: discard the entry and return to list navigation.
		m.exitRoutingInput()
		m.refreshRoutingOverlay()
		return m, nil
	default:
		var cmd tea.Cmd
		m.routingInput, cmd = m.routingInput.Update(msg)
		m.routingInputErr = false // typing clears the invalid flash
		m.refreshRoutingOverlay()
		return m, cmd
	}
}

// exitRoutingInput leaves input mode and resets the editor.
func (m *Model) exitRoutingInput() {
	m.routingInputMode = false
	m.routingInputErr = false
	m.routingInput.Blur()
	m.routingInput.SetValue("")
}

// updateRoutingList handles keys in list (navigation) mode: j/k navigate, Space
// toggles the exit-node advertise flag (only on item 0), [d] removes the
// highlighted route, [a] enters input mode. All edits are local (working copy).
func (m Model) updateRoutingList(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Open the "Command Room" confirmation overlay showing the exact command
		// that will be run (nothing executes until the user confirms there).
		return m.openRoutingConfirm(), nil
	case "j", "down":
		if m.routingCursor < m.routingItemCount()-1 {
			m.routingCursor++
		}
		m.refreshRoutingOverlay()
	case "k", "up":
		if m.routingCursor > 0 {
			m.routingCursor--
		}
		m.refreshRoutingOverlay()
	case " ", "space":
		// Toggle only applies to the Exit Node (Advertise) item at cursor 0.
		if m.routingCursor == 0 {
			m.routingExitNode = !m.routingExitNode
			m.routingDirty = true
			m.refreshRoutingOverlay()
		}
	case "a":
		// Enter input mode to add a CIDR. Pre-fill with the most recently deleted
		// route (if any) so the user can pop it back in (undo) or fix a typo before
		// resubmitting; cursor goes to the end of the pre-filled text.
		m.routingInputMode = true
		m.routingInputErr = false
		m.routingInput.SetValue(m.lastDeletedRoute)
		m.routingInput.CursorEnd()
		m.routingInput.Focus()
		m.refreshRoutingOverlay()
	case "d":
		// Remove the highlighted route (cursor >= 1), clamping the cursor if the
		// last item was deleted.
		if m.routingCursor >= 1 && m.routingCursor-1 < len(m.routingRoutes) {
			idx := m.routingCursor - 1
			m.lastDeletedRoute = m.routingRoutes[idx] // remember for the next [a] pre-fill (pseudo-undo)
			m.routingRoutes = append(m.routingRoutes[:idx], m.routingRoutes[idx+1:]...)
			m.routingDirty = true
			if m.routingCursor >= m.routingItemCount() {
				m.routingCursor = m.routingItemCount() - 1
			}
			m.refreshRoutingOverlay()
		}
	}
	return m, nil
}

// routingBody renders the ROUTING_MANAGEMENT modal content from the working copy:
// the Exit Node advertise toggle, the advertised subnet routes (or a placeholder),
// a divider, and a dynamic keymap — the CIDR editor + confirm/cancel hints in
// input mode, the full action keymap otherwise. Styling mirrors accountsBody; the
// highlighted row uses the inverted AccountActive bar (green bg, dark text).
func (m Model) routingBody(w int) string {
	var lines []string

	// --- Exit Node (Advertise) toggle — cursor index 0 -----------------------
	lines = append(lines, modalLine(w, styles.ModalHeading.Render("EXIT NODE")), modalLine(w, ""))
	const exitLabel = "Exit Node (Advertise):"
	if m.routingCursor == 0 && !m.routingInputMode {
		state := "[OFF] "
		if m.routingExitNode {
			state = "[ON] "
		}
		lines = append(lines, styles.AccountActive.Render(joinRow(" "+exitLabel, state, w)))
	} else {
		right := styles.ModalDim.Render("[OFF]")
		if m.routingExitNode {
			right = styles.StatusOK.Render("[ON]")
		}
		lines = append(lines, modalRow(w, styles.ModalText.Render(" "+exitLabel), right))
	}
	lines = append(lines, modalLine(w, ""))

	// --- Advertised subnet routes — cursor indices 1.. -----------------------
	lines = append(lines, modalLine(w, styles.ModalHeading.Render("ADVERTISED SUBNET ROUTES")), modalLine(w, ""))
	if len(m.routingRoutes) == 0 {
		lines = append(lines, modalLine(w, styles.ModalDim.Render("  No custom routes advertised.")))
	} else {
		for i, r := range m.routingRoutes {
			if m.routingCursor == i+1 && !m.routingInputMode {
				lines = append(lines, styles.AccountActive.Render(joinRow("  "+r, "ADVERTISED ", w)))
			} else {
				lines = append(lines, modalRow(w,
					styles.ModalText.Render("  "+r),
					styles.ModalDim.Render("ADVERTISED")))
			}
		}
	}
	lines = append(lines, modalLine(w, ""))

	// --- Divider + dynamic keymap (or the CIDR editor) -----------------------
	lines = append(lines, modalDivider(w), modalLine(w, ""))
	if m.routingInputMode {
		prompt := "Enter CIDR (e.g., 192.168.1.0/24):"
		promptStyle := styles.ModalAccent
		if m.routingInputErr {
			prompt = "Invalid CIDR — try again (e.g., 192.168.1.0/24):"
			promptStyle = styles.StatusErr
		}
		lines = append(lines,
			modalLine(w, promptStyle.Render(prompt)),
			modalLine(w, m.routingInput.View()),
			modalLine(w, ""),
			gridLine(w, accountKey("ENTER", "CONFIRM", false), accountKey("ESC", "CANCEL", false)),
		)
	} else {
		lines = append(lines,
			gridLine(w, accountKey("J/K", "NAVIGATE", false), accountKey("A", "ADD ROUTE", false)),
			gridLine(w, accountKey("SPACE", "TOGGLE", false), accountKey("D", "REMOVE", false)),
			gridLine(w, accountKey("ENTER", "APPLY", false), accountKey("ESC", "CLOSE", false)),
		)
	}

	return strings.Join(lines, "\n")
}

// routingCommandString renders the exact `tailscale set …` command the working
// copy would run — shown in the Command Room and copied to the clipboard.
func (m Model) routingCommandString() string {
	return tailscale.AdvertiseCommandString(m.routingExitNode, m.routingRoutes)
}

// openRoutingConfirm transitions from the routing list into the "Command Room"
// confirmation overlay. Nothing executes until the user confirms there; Esc
// returns to the list. The confirm modal is rendered directly from model state
// (renderRoutingConfirmOverlay), so no viewport setup is needed.
func (m Model) openRoutingConfirm() Model {
	m.state = stateRoutingConfirm
	m.routingCopied = false
	return m
}

// updateRoutingConfirm owns the keys of the Command Room overlay: Enter applies
// the staged command (async, closing the modal), [c] copies it to the clipboard
// (staying put), Esc/q go back to the list without applying.
func (m Model) updateRoutingConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Apply asynchronously and leave the modal entirely; the routingActionMsg
		// logs the outcome and refreshes status + prefs.
		cmd := setRoutingCmd(m.routingExitNode, append([]string(nil), m.routingRoutes...))
		m.state = stateMain
		m.routingCopied = false
		return m, cmd
	case "c", "C":
		// Copy the command to the clipboard; stay in the modal.
		return m, copyRoutingCmd(m.routingCommandString())
	case "esc", "q":
		// Back to the list without applying.
		m.state = stateRouting
		m.routingCopied = false
		m.refreshRoutingOverlay()
		return m, nil
	}
	return m, nil
}

// hardWrap breaks s into chunks of at most w runes (used as a safety net for a
// long token, e.g. a comma-joined route list, that word-wrapping can't split).
func hardWrap(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	r := []rune(s)
	var out []string
	for len(r) > w {
		out = append(out, string(r[:w]))
		r = r[w:]
	}
	return append(out, string(r))
}

// wrapCommand word-wraps s to width w, then hard-wraps any line still too long,
// so the command preview never overflows the modal.
func wrapCommand(s string, w int) []string {
	var out []string
	for _, line := range wrapText(s, w) {
		if len([]rune(line)) <= w {
			out = append(out, line)
		} else {
			out = append(out, hardWrap(line, w)...)
		}
	}
	return out
}

// renderRoutingConfirmOverlay draws the "Command Room" — a floating, fully
// opaque confirmation modal composited over the still-visible base view. It
// shows the exact command to be executed (safely wrapped), an Admin Console
// approval reminder, and the apply/copy/back keymap. Rendered directly from
// model state so the "Copied!" indicator updates without viewport plumbing.
func (m Model) renderRoutingConfirmOverlay(base string) string {
	w := overlayWidth(m.width)
	cmd := m.routingCommandString()

	lines := []string{
		modalLine(w, lipgloss.PlaceHorizontal(w, lipgloss.Center,
			styles.ModalTitle.Render("[ CONFIRM ROUTING CHANGES ]"),
			lipgloss.WithWhitespaceBackground(styles.Surface))),
		modalDivider(w),
		modalLine(w, styles.ModalDim.Render("ABOUT TO EXECUTE:")),
	}
	for _, ln := range wrapCommand(cmd, w) {
		lines = append(lines, modalLine(w, styles.ModalAccent.Render(ln)))
	}
	lines = append(lines, modalLine(w, ""))
	warn := lipgloss.NewStyle().Foreground(styles.Subtle).Background(styles.Surface).Italic(true)
	for _, ln := range wrapText("* Note: Advertised routes and exit nodes must be approved in the Tailscale Admin Console.", w) {
		lines = append(lines, modalLine(w, warn.Render(ln)))
	}
	lines = append(lines, modalDivider(w))
	lines = append(lines,
		modalLine(w, accountKey("ENTER", "APPLY", false)),
		modalLine(w, accountKey("C", "COPY TO CLIPBOARD", false)),
		modalLine(w, accountKey("ESC", "BACK", false)),
	)
	if m.routingCopied {
		lines = append(lines, modalLine(w, styles.StatusOK.Render("✓ Copied to clipboard!")))
	}

	inner := strings.Join(lines, "\n")
	innerH := countLines(inner)
	modal := lipgloss.NewStyle().
		Width(w+2*modalHPad).
		Height(innerH+2*modalVPad).
		Background(styles.Surface).
		Foreground(styles.Fg).
		Padding(modalVPad, modalHPad).
		Border(lipgloss.NormalBorder()).
		BorderForeground(styles.Primary).
		BorderBackground(styles.Surface).
		Render(inner)

	return overlayCenter(base, modal)
}
