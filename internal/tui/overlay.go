package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Phundahl/tailscaleTUI/internal/styles"
	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// Modal sizing: padding/border consume fixed cells around the viewport content.
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

// newOverlayVP builds a viewport with an opaque (modal-background) content area.
func newOverlayVP(w, h int, content string) viewport.Model {
	vp := viewport.New(w, h)
	vp.Style = lipgloss.NewStyle().Background(styles.ModalBg)
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
	switch msg.String() {
	case "esc", "q":
		m.state = stateMain
		return m, nil
	case "?":
		if m.state == stateHelp {
			m.state = stateMain
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.overlay, cmd = m.overlay.Update(msg)
	return m, cmd
}

// renderOverlay draws the active modal as a true floating box composited on top
// of the (still visible) base view. The modal is 100% opaque: every line is the
// full content width with the modal background, and the container has explicit
// Width/Height plus a solid Background so the whole bounding box is painted.
func (m Model) renderOverlay(base string) string {
	w := m.overlay.Width

	var title, hint string
	switch m.state {
	case stateHelp:
		title = "HELP — KEYBINDINGS"
		hint = "[j/k] scroll   [?/Esc] close"
	case stateRoutes:
		name := ""
		if p, ok := m.selectedPeer(); ok {
			name = p.Hostname
		}
		title = "ADVERTISED ROUTES — " + name
		hint = "[j/k] scroll   [Esc/q] close"
	}

	// Each line is padded to the full width w with the modal background.
	inner := lipgloss.JoinVertical(lipgloss.Left,
		modalLine(w, styles.ModalTitle.Render(ansi.Truncate(title, w, "…"))),
		modalDivider(w),
		m.overlay.View(),
		modalDivider(w),
		modalLine(w, styles.ModalDim.Render(hint)),
	)

	innerH := modalChrome + m.overlay.Height
	// lipgloss Width/Height include padding, so add it back: the content area
	// must stay exactly w x innerH or the full-width lines wrap.
	modal := lipgloss.NewStyle().
		Width(w + 2*modalHPad).
		Height(innerH + 2*modalVPad).
		Background(styles.ModalBg).
		Foreground(styles.Fg).
		Padding(modalVPad, modalHPad).
		Border(lipgloss.ThickBorder()).
		BorderForeground(styles.Primary).
		BorderBackground(styles.ModalBg).
		Render(inner)

	return overlayCenter(base, modal)
}

// modalLine paints a single content line opaque across the full modal width.
func modalLine(w int, content string) string {
	return styles.ModalFill(w).Render(content)
}

// modalDivider renders a full-width horizontal rule on the modal background.
func modalDivider(w int) string {
	return styles.ModalAccent.Render(strings.Repeat("─", w))
}

// overlayCenter composites the fg block centered over the bg block, line by
// line. It is ANSI-aware: each background row is split around the modal's
// columns with ansi.Truncate / ansi.TruncateLeft (which carry SGR state across
// the cut), and the modal's cells overwrite the background entirely so no text
// shows through. Explicit resets isolate the three segments' styles.
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
// Both builders return content whose every line is exactly w wide and fully
// painted with the modal background (via modalLine), so the viewport never
// exposes a transparent cell.

// helpBody renders the keybinding reference shown in the help overlay.
func helpBody(w int) string {
	row := func(keys, desc string) string {
		return modalLine(w, styles.ModalText.Render("  ")+
			styles.ModalKey.Render(padRight(keys, 16))+
			styles.ModalText.Render(desc))
	}
	lines := []string{
		modalLine(w, styles.ModalHeading.Render("NAVIGATION")),
		row("j / k, ↑ / ↓", "Move selection (wraps around)"),
		row("/", "Search / filter nodes"),
		modalLine(w, ""),
		modalLine(w, styles.ModalHeading.Render("NODE ACTIONS")),
		row("x", "Toggle exit node (exit-capable nodes only)"),
		row("e", "Expand subnet routes (subnet routers)"),
		modalLine(w, ""),
		modalLine(w, styles.ModalHeading.Render("GLOBAL")),
		row("?", "Toggle this help"),
		row("q / Esc", "Close overlay / quit"),
	}
	return strings.Join(lines, "\n")
}

// routesBody renders the full advertised-route list for the routes overlay.
func routesBody(p types.Peer, w int) string {
	lines := []string{
		modalLine(w, styles.ModalDim.Render(fmt.Sprintf("%d routes advertised by %s", len(p.AdvertisedRoutes), p.Hostname))),
		modalLine(w, ""),
	}
	for _, r := range p.AdvertisedRoutes {
		lines = append(lines, modalLine(w, styles.ModalAccent.Render("  → ")+styles.ModalText.Render(r)))
	}
	return strings.Join(lines, "\n")
}
