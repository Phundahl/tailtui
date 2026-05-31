package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/types"
)

// Modal sizing constants. The modal is an opaque, tonally-raised box filled
// with the theme Surface color and framed by a sharp single-line border. Every
// content line is painted full-width with the Surface (modalLine), so nothing
// from the base view bleeds through after overlayCenter composites it on top.
const (
	modalHPad   = 2 // horizontal padding cells per side
	modalVPad   = 1 // vertical padding cells per side
	modalBorder = 1 // border cells per side
	modalChrome = 4 // non-viewport content lines: title, 2 dividers, hint
)

// overlayWidth returns the inner (viewport/content) width for a modal, clamped
// to fit within the terminal once border + padding are accounted for.
func overlayWidth(termW int) int {
	w := termW * 6 / 10
	if max := termW - 2*(modalHPad+modalBorder) - 2; w > max {
		w = max
	}
	if w > 64 {
		w = 64
	}
	if w < 20 {
		w = 20
	}
	return w
}

// logOverlayWidth returns the inner content width for the log overlay. Logs
// (timestamp + level + network paths) are wide, so this modal takes ~85% of the
// terminal — capped at 120 cols — so most entries fit on a single line, far
// wider than the standard overlayWidth used by the other modals.
func logOverlayWidth(termW int) int {
	w := termW * 85 / 100
	if max := termW - 2*(modalHPad+modalBorder) - 2; w > max {
		w = max
	}
	if w > 120 {
		w = 120
	}
	if w < 20 {
		w = 20
	}
	return w
}

// overlayHeight returns the viewport height: just enough to show contentLines,
// but never taller than the terminal allows (the viewport scrolls past that).
func overlayHeight(termH, contentLines int) int {
	max := termH - 2*(modalVPad+modalBorder) - modalChrome - 2
	h := contentLines
	if h > max {
		h = max
	}
	if h < 1 {
		h = 1
	}
	return h
}

// countLines reports how many lines a rendered content block occupies.
func countLines(s string) int { return strings.Count(s, "\n") + 1 }

// newOverlayVP builds a viewport for the modal content, with an opaque Surface
// content area so short/blank lines don't reveal the view behind the modal.
func newOverlayVP(w, h int, content string) viewport.Model {
	vp := viewport.New(w, h)
	vp.Style = lipgloss.NewStyle().Background(styles.Surface)
	vp.SetContent(content)
	return vp
}

// openHelp transitions to the help overlay.
func (m Model) openHelp() Model {
	m.state = stateHelp
	w := overlayWidth(m.width)
	content := helpBody(w)
	m.overlay = newOverlayVP(w, overlayHeight(m.height, countLines(content)), content)
	return m
}

// openRoutes transitions to the routes overlay for the given peer.
func (m Model) openRoutes(p types.Peer) Model {
	m.state = stateRoutes
	w := overlayWidth(m.width)
	content := routesBody(p, w)
	m.overlay = newOverlayVP(w, overlayHeight(m.height, countLines(content)), content)
	return m
}

// openAccounts transitions to the accounts modal.
func (m Model) openAccounts() Model {
	m.state = stateAccounts
	// Start the cursor on the active session, matching the mockup.
	for i, a := range m.accounts {
		if a.Active {
			m.accountCursor = i
			break
		}
	}
	w := overlayWidth(m.width)
	content := m.accountsBody(w)
	m.overlay = newOverlayVP(w, overlayHeight(m.height, countLines(content)), content)
	return m
}

// openLogs transitions to the log overlay, showing the full log ring scrolled
// to the most recent entry.
func (m Model) openLogs() Model {
	m.state = stateLogs
	w := logOverlayWidth(m.width)
	content := logBody(m.logs, w)
	m.overlay = newOverlayVP(w, logViewportHeight(m.height, countLines(content)), content)
	m.overlay.GotoBottom() // newest entries are appended last
	return m
}

// resizeOverlay re-sizes and re-renders the active overlay after a window
// resize, so the modal tracks the terminal dimensions.
func (m Model) resizeOverlay() Model {
	w := overlayWidth(m.width)
	var content string
	switch m.state {
	case stateHelp:
		content = helpBody(w)
	case stateRoutes:
		if p, ok := m.selectedPeer(); ok {
			content = routesBody(p, w)
		}
	case stateAccounts:
		content = m.accountsBody(w)
	case stateLogs:
		lw := logOverlayWidth(m.width) // wider than the other modals
		content = logBody(m.logs, lw)
		m.overlay.Width = lw
		m.overlay.Height = logViewportHeight(m.height, countLines(content))
		m.overlay.SetContent(content)
		return m
	}
	m.overlay.Width = w
	m.overlay.Height = overlayHeight(m.height, countLines(content))
	m.overlay.SetContent(content)
	return m
}

// updateOverlay handles keys while an overlay is active. Esc/q close it; "?"
// closes the help overlay specifically; all other keys (j/k, arrows, page
// keys) are forwarded ONLY to the overlay viewport — never the background list.
func (m Model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "q" {
		m.state = stateMain
		return m, nil
	}

	switch m.state {
	case stateHelp:
		if key == "?" {
			m.state = stateMain
		}
		return m, nil
	case stateLogs:
		if key == "v" {
			m.state = stateMain
			return m, nil
		}
		// Other keys (j/k, arrows, page) scroll the log viewport.
	case stateAccounts:
		// The accounts modal owns all keys: navigate the cursor and run the live
		// profile commands; nothing falls through to the background or a viewport.
		switch key {
		case "j", "down":
			if m.accountCursor < len(m.accounts)-1 {
				m.accountCursor++
			}
			m.overlay.SetContent(m.accountsBody(m.overlay.Width))
			return m, nil
		case "k", "up":
			if m.accountCursor > 0 {
				m.accountCursor--
			}
			m.overlay.SetContent(m.accountsBody(m.overlay.Width))
			return m, nil
		case "enter":
			// Switch to the highlighted profile (if not already active), close the
			// modal, and let the action refresh status + accounts.
			if a, ok := m.selectedAccount(); ok && !a.Active {
				m.state = stateMain
				return m, switchAccountCmd(a.ID, a.Email)
			}
			return m, nil
		case "a":
			// Add account: interactive `tailscale login` (suspends the TUI).
			return m, addAccountCmd()
		case "d":
			// Remove the highlighted profile (won't forget the active one).
			if a, ok := m.selectedAccount(); ok && !a.Active {
				return m, removeAccountCmd(a.ID, a.Email)
			}
			return m, nil
		case "l":
			// Log the current session out.
			return m, logoutCmd()
		}
		return m, nil
	}

	// Help / routes: forward scroll keys to the viewport.
	var cmd tea.Cmd
	m.overlay, cmd = m.overlay.Update(msg)
	return m, cmd
}

// renderOverlay draws the active modal as a true floating box composited over
// the (still visible) base view. The modal is 100% opaque: every line is the
// full content width painted with the theme background (modalLine / modal
// styles), and the container sets explicit Width/Height + a solid Background so
// the whole bounding box — padding and border included — overwrites whatever is
// behind it. A bright rounded border + colored title make it float.
func (m Model) renderOverlay(base string) string {
	// The log overlay uses the pane-style titled border (─┤ TERMINAL_LOGS ├─) to
	// mirror the base TERMINAL_LOGS pane, rather than the tab-title chrome.
	if m.state == stateLogs {
		return m.renderLogOverlay(base)
	}

	w := m.overlay.Width

	var title, hint string
	switch m.state {
	case stateHelp:
		title = "HELP & SHORTCUTS"
		hint = "-- KEYBOARD INPUT MODE ACTIVE --"
	case stateRoutes:
		name := ""
		if p, ok := m.selectedPeer(); ok {
			name = p.Hostname
		}
		title = "PEER DETAILS: " + name
		hint = "[Esc] Back to List"
	case stateAccounts:
		title = "ACCOUNT_MANAGEMENT"
		hint = "-- KEYBOARD INPUT MODE ACTIVE --"
	}

	// Title rendered as a tab "⌐ TITLE ¬", matching the mockups.
	titleBar := modalLine(w, styles.ModalTitle.Render("⌐ "+ansi.Truncate(title, w-4, "…")+" ¬"))

	inner := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		modalDivider(w),
		m.overlay.View(),
		modalDivider(w),
		modalLine(w, styles.ModalDim.Render(hint)),
	)

	innerH := modalChrome + m.overlay.Height
	// lipgloss Width/Height include padding, so add it back: the content area
	// stays exactly w x innerH and the full-width lines never wrap. Sharp
	// single-line border + opaque Surface fill = a tonally-raised brutalist modal.
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

// logChrome counts the non-viewport lines of the log overlay: top border, a
// fixed hint line, the bottom border, plus the top/bottom vertical padding.
const logChrome = 3 + 2*modalVPad

// logViewportHeight sizes the scrollable log area to fit the content but never
// overflow the terminal (the viewport scrolls past that).
func logViewportHeight(termH, contentLines int) int {
	max := termH - 2*modalBorder - logChrome
	h := contentLines
	if h > max {
		h = max
	}
	if h < 1 {
		h = 1
	}
	return h
}

// renderLogOverlay draws the log modal as a fully opaque box whose top border
// embeds the title (┌─┤ TERMINAL_LOGS ├──┐), matching the brutalist Pane used
// in the base layout. Every cell carries the Surface background so nothing
// behind it bleeds through; overlayCenter then composites it on top.
func (m Model) renderLogOverlay(base string) string {
	w := m.overlay.Width
	innerW := w + 2*modalHPad // cells between the │ borders (content + padding)

	border := lipgloss.NewStyle().Foreground(styles.Primary).Background(styles.Surface)
	side := border.Render("│")
	padCell := styles.ModalFill(modalHPad).Render("")
	blank := side + styles.ModalFill(innerW).Render("") + side

	lines := []string{modalTitledTop("TERMINAL_LOGS", innerW)}
	for i := 0; i < modalVPad; i++ {
		lines = append(lines, blank)
	}
	for _, ln := range strings.Split(m.overlay.View(), "\n") {
		lines = append(lines, side+padCell+ln+padCell+side)
	}
	// Fixed hint row (not part of the scrollable area).
	hintTxt := styles.ModalDim.Render("[j/k] Scroll   [v]/[Esc] Close")
	lines = append(lines, side+padCell+styles.ModalFill(w).Render(hintTxt)+padCell+side)
	for i := 0; i < modalVPad; i++ {
		lines = append(lines, blank)
	}
	lines = append(lines, border.Render("└"+strings.Repeat("─", innerW)+"┘"))

	return overlayCenter(base, strings.Join(lines, "\n"))
}

// modalTitledTop renders an opaque "┌─┤ TITLE ├────┐" top border exactly innerW
// cells wide (the span between the corner glyphs), Surface-backed throughout.
func modalTitledTop(title string, innerW int) string {
	border := lipgloss.NewStyle().Foreground(styles.Primary).Background(styles.Surface)
	const lead, trail = "─┤ ", " ├"
	used := lipgloss.Width(lead) + lipgloss.Width(title) + lipgloss.Width(trail)
	if dashes := innerW - used; dashes >= 0 {
		return border.Render("┌"+lead) +
			styles.ModalTitle.Render(title) +
			border.Render(trail+strings.Repeat("─", dashes)+"┐")
	}
	return border.Render("┌" + strings.Repeat("─", innerW) + "┐")
}

// logBody renders the full log ring for the overlay, one opaque line per entry.
func logBody(logs []types.LogEntry, w int) string {
	if len(logs) == 0 {
		return modalLine(w, styles.ModalDim.Render("No log entries yet."))
	}
	lines := make([]string, len(logs))
	for i, e := range logs {
		lines[i] = modalLine(w, logLine(e))
	}
	return strings.Join(lines, "\n")
}

// logLine formats one entry "HH:MM:SS [LEVEL] message" on the modal surface:
// muted timestamp, level chip colored by severity (theme) and bold to match the
// other modal status chips, and the message in the default text color.
func logLine(e types.LogEntry) string {
	lvl := lipgloss.NewStyle().
		Foreground(styles.LogLevelColor(e.Level)).
		Background(styles.Surface).
		Bold(true)
	return styles.ModalDim.Render(e.Time+" ") +
		lvl.Render("["+e.Level+"]") +
		styles.ModalText.Render(" "+e.Message)
}

// modalLine paints a single content line opaque across the full modal width.
func modalLine(w int, content string) string {
	return styles.ModalFill(w).Render(content)
}

// modalRow lays out a left and right segment with a surface-filled gap between,
// keeping the whole line opaque (used for the aligned help/routes tables).
func modalRow(w int, left, right string) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return modalLine(w, left+styles.ModalText.Render(strings.Repeat(" ", gap))+right)
}

// modalDivider renders a full-width rule on the modal surface.
func modalDivider(w int) string {
	return styles.ModalFill(w).Render(lipgloss.NewStyle().
		Foreground(styles.BorderInactive).Background(styles.Surface).
		Render(strings.Repeat("─", w)))
}

// overlayCenter composites the fg block centered over the bg block, line by
// line. It is ANSI-aware: each background row is split around the modal's
// columns with ansi.Truncate / ansi.TruncateLeft (which carry SGR state across
// the cut), and the modal's cells (including its padding spaces) overwrite the
// background entirely so no text shows through. Explicit resets isolate the
// three segments' styles.
func overlayCenter(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	bgH := len(bgLines)
	fgH := len(fgLines)
	fgW := lipgloss.Width(fg)

	x := (lipgloss.Width(bg) - fgW) / 2
	if x < 0 {
		x = 0
	}
	y := (bgH - fgH) / 2
	if y < 0 {
		y = 0
	}

	out := make([]string, bgH)
	for i, line := range bgLines {
		if i < y || i >= y+fgH {
			out[i] = line
			continue
		}
		fgLine := fgLines[i-y]

		left := ansi.Truncate(line, x, "")
		if pad := x - ansi.StringWidth(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}
		right := ansi.TruncateLeft(line, x+ansi.StringWidth(fgLine), "")
		out[i] = left + "\x1b[0m" + fgLine + "\x1b[0m" + right
	}
	return strings.Join(out, "\n")
}

// --- overlay content ---------------------------------------------------------
//
// Both builders pad every line to width w with the theme background (modalLine
// + Modal* styles), so the viewport never exposes a transparent cell.

// helpBody renders the keybinding reference shown in the help overlay, grouped
// by category with right-aligned [ key ] columns, ending in an [ ESC TO CLOSE ]
// button — matching the mockup.
func helpBody(w int) string {
	group := func(title string, rows [][2]string) []string {
		out := []string{modalLine(w, styles.ModalHeading.Render(title))}
		for _, r := range rows {
			out = append(out, modalRow(w,
				styles.ModalText.Render(r[0]),
				styles.ModalKey.Render("[ "+r[1]+" ]")))
		}
		return append(out, modalLine(w, ""))
	}

	var lines []string
	lines = append(lines, group("NAVIGATION", [][2]string{
		{"Move Selection Up / Down", "j / k"},
		{"Switch Pane Left / Right", "h / l"},
		{"Jump to Top / Bottom", "g / G"},
	})...)
	lines = append(lines, group("SEARCH / FILTER", [][2]string{
		{"Open / Edit Filter", "/"},
		{"Navigate While Typing", "↑↓  Ctrl+j/k"},
		{"Apply Filter (blur box)", "Enter / Esc"},
		{"Clear Filter", "Esc (in list)"},
	})...)
	lines = append(lines, group("NODE ACTIONS", [][2]string{
		{"Connect / Disconnect", "c"},
		{"Toggle Exit Node", "x"},
		{"Expand Subnet Routes", "e"},
		{"Operator Setup (sudo)", "O"},
	})...)
	lines = append(lines, group("GLOBAL", [][2]string{
		{"Switch Accounts", "l"},
		{"View Logs", "v"},
		{"Toggle Help Overlay", "?"},
		{"Quit Application", "q"},
	})...)

	btn := styles.ModalTitle.Render("[ ESC TO CLOSE ]")
	lines = append(lines, modalLine(w, lipgloss.PlaceHorizontal(
		w, lipgloss.Center, btn, lipgloss.WithWhitespaceBackground(styles.Surface))))
	return strings.Join(lines, "\n")
}

// routesBody renders the peer's live advertised/approved subnet routes as an
// aligned routing table (DESTINATION / GATEWAY / LATENCY / STATUS). The routes
// and gateway come straight from the daemon (Peer.AdvertisedRoutes are the
// node's PrimaryRoutes); LATENCY is the node's live ping reading and every
// approved route is actively ROUTING through this gateway.
func routesBody(p types.Peer, w int) string {
	dest := w * 36 / 100
	gw := w * 26 / 100
	lat := w * 13 / 100

	header := styles.ModalDim.Render(
		padCol("DESTINATION", dest) + padCol("GATEWAY", gw) + padCol("LATENCY", lat) + "STATUS")
	lines := []string{modalLine(w, header), modalLine(w, "")}

	latency := "—"
	if p.LatencyMs > 0 {
		latency = fmt.Sprintf("%dms", p.LatencyMs)
	}

	for _, r := range p.AdvertisedRoutes {
		row := styles.ModalText.Render(padCol(r, dest)) +
			styles.ModalText.Render(padCol(p.TailscaleIP, gw)) +
			styles.ModalText.Render(padCol(latency, lat)) +
			styles.StatusOK.Render("[ ROUTING ]")
		lines = append(lines, modalLine(w, row))
	}
	return strings.Join(lines, "\n")
}

// padCol pads (or truncates) plain ASCII text s to exactly width cells.
func padCol(s string, width int) string {
	if width < 1 {
		width = 1
	}
	if w := lipgloss.Width(s); w > width {
		return s[:width-1] + " "
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

// accountsBody renders the accounts modal: each account as a (highlighted-when-
// active) two-line block, then a divider and a two-column action grid.
func (m Model) accountsBody(w int) string {
	var lines []string
	if len(m.accounts) == 0 {
		lines = append(lines, modalLine(w, styles.ModalDim.Render("No accounts. Press [a] to add one.")), modalLine(w, ""))
	}
	for i, acc := range m.accounts {
		lines = append(lines, m.accountRows(acc, i, w)...)
		lines = append(lines, modalLine(w, ""))
	}
	lines = append(lines, modalDivider(w), modalLine(w, ""))
	lines = append(lines,
		gridLine(w, accountKey("J/K", "NAVIGATE", false), accountKey("ENTER", "SWITCH", false)),
		gridLine(w, accountKey("A", "ADD ACCOUNT", false), accountKey("D", "REMOVE", false)),
		gridLine(w, accountKey("L", "LOGOUT", true), accountKey("Q/ESC", "CLOSE", false)),
	)
	return strings.Join(lines, "\n")
}

// accountRows renders one account as two opaque, full-width lines. The active
// session is a solid primary-green bar; the cursor marks the focused account.
func (m Model) accountRows(acc types.Account, i, w int) []string {
	if acc.Active {
		email := joinRow(" "+acc.Email, "[d] REMOVE  ✓", w)
		sub := joinRow(" * ACTIVE SESSION", "", w)
		return []string{styles.AccountActive.Render(email), styles.AccountActiveSub.Render(sub)}
	}
	ptr := "  "
	if i == m.accountCursor {
		ptr = "❯ "
	}
	email := modalRow(w, styles.ModalText.Render(ptr+acc.Email), styles.ModalDim.Render("[d] REMOVE"))
	sub := modalLine(w, styles.ModalDim.Render("  INACTIVE"))
	return []string{email, sub}
}

// gridLine lays out two cells in a two-column grid, surface-filled and opaque.
func gridLine(w int, left, right string) string {
	half := w / 2
	gap := half - lipgloss.Width(left)
	if gap < 1 {
		gap = 1
	}
	return modalLine(w, left+styles.ModalText.Render(strings.Repeat(" ", gap))+right)
}

// accountKey renders a "[KEY] LABEL" action cell; danger labels (LOGOUT) are red.
func accountKey(key, label string, danger bool) string {
	lbl := styles.ModalText
	if danger {
		lbl = styles.StatusErr
	}
	return styles.ModalKey.Render("["+key+"]") + styles.ModalText.Render(" ") + lbl.Render(label)
}
