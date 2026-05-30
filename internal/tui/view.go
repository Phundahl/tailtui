package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailscaleTUI/internal/styles"
	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// Fixed layout constants.
const (
	headerHeight    = 1
	footerHeight    = 1
	logsHeight      = 5 // includes the box border
	dashboardHeight = 8 // lines rendered by renderLocalDashboard
	minWidth        = 60
	minHeight       = 18
)

// layout holds the computed geometry of every region for a given terminal size.
// Centralizing it keeps Update (which sizes the list) and View (which draws)
// in agreement.
type layout struct {
	leftW, rightW int
	midH          int
	listW, listH  int
}

func computeLayout(w, h int) layout {
	bodyH := h - headerHeight - footerHeight
	midH := bodyH - logsHeight

	leftW := w * 2 / 5
	rightW := w - leftW

	// The list lives inside the left box, below the local dashboard and a
	// divider: its width is the box's inner content width (border + padding
	// removed); its height subtracts the border, dashboard, and divider line.
	listW := styles.ContentWidth(leftW)
	listH := midH - 2 - dashboardHeight - 1
	if listH < 1 {
		listH = 1
	}
	if listW < 1 {
		listW = 1
	}

	return layout{leftW: leftW, rightW: rightW, midH: midH, listW: listW, listH: listH}
}

// View implements tea.Model and assembles the full-screen layout.
func (m Model) View() string {
	if !m.ready {
		return "Initializing Tailscale TUI..."
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small (need at least %dx%d).", minWidth, minHeight)
	}

	lay := computeLayout(m.width, m.height)

	header := m.renderHeader()
	footer := m.renderFooter()

	left := m.renderLeftPane(lay)
	right := m.renderDetailsPane(lay)
	mid := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	logs := m.renderLogsPane(m.width, logsHeight)

	base := lipgloss.JoinVertical(lipgloss.Left, header, mid, logs, footer)

	// Overlays float on top of the still-visible base layout.
	if m.state != stateMain {
		return m.renderOverlay(base)
	}
	return base
}

// --- header / footer ---------------------------------------------------------

func (m Model) renderHeader() string {
	left := styles.Title.Render("TAILSCALE_TUI_V1.0")
	right := styles.Dim.Render("(q)uit  (?)help")
	return styles.Bar(m.width, left, right)
}

func (m Model) renderFooter() string {
	help := "[j/k] Nav  [/] Search  [x] Exit Node  [e] Routes  [s] SSH  [p] Ping  [t] Connect  [l] Accounts  [?] Help  [q] Quit"
	return styles.Dim.Render(styles.Bar(m.width, help, ""))
}

// --- left pane: local dashboard + peer list ----------------------------------

// renderLeftPane is the focused pane (peer list), so it gets a bright border.
func (m Model) renderLeftPane(lay layout) string {
	cw := styles.ContentWidth(lay.leftW) // inner width (border + padding removed)
	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderLocalDashboard(cw),
		styles.Divider(cw),
		m.peers.View(),
	)
	return styles.BoxFocused(lay.leftW, lay.midH).Render(content)
}

func (m Model) renderLocalDashboard(w int) string {
	l := m.local
	lines := []string{
		styles.Heading.Render("LOCAL_NODE"),
		field("User:", l.User),
		field("Host:", l.Hostname),
		field("Local IP:", l.LocalIP),
		field("Tailscale IP:", l.TailscaleIP),
		styles.Label.Render("State:") + " " + connSymbol(l.Conn) + " " +
			styles.Value.Render(connText(l.Conn, l.Relay)),
		styles.Label.Render("Exit:") + " " + m.renderExitValue(),
		styles.Label.Render("Latency:") + " " +
			styles.Value.Render(fmt.Sprintf("%dms", l.LatencyMs)) + " " +
			styles.Sparkline(l.LatencyHistory),
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderExitValue renders the dashboard's active exit node: highlighted in
// yellow when set, or a dimmed "None" when no exit node is active.
func (m Model) renderExitValue() string {
	name := m.activeExitNodeName()
	if name == "None" {
		return styles.Dim.Render("None")
	}
	return styles.ExitName.Render(name)
}

// --- right pane: peer details + latency history ------------------------------

func (m Model) renderDetailsPane(lay layout) string {
	p, ok := m.selectedPeer()
	if !ok {
		empty := styles.Dim.Render("No node selected.")
		return styles.Box(lay.rightW, lay.midH).Render(empty)
	}

	identity := []string{
		styles.Heading.Render("PEER DETAILS: ") + styles.Title.Render(p.Hostname),
		"",
		styles.Label.Render("IDENTITY"),
		field("OS:", p.OS.Icon()+" "+p.OS.Name()),
		field("IP:", p.TailscaleIP),
		styles.Label.Render("Conn:") + " " + connSymbol(p.Conn) + " " +
			styles.Value.Render(connText(p.Conn, p.Relay)),
		field("Version:", p.Version),
		field("Tags:", strings.Join(p.Tags, " ")),
		field("Last Seen:", p.LastSeen),
		"",
		styles.Heading.Render("LATENCY HISTORY (60s)"),
		latencyStats(p.LatencyHistory),
		styles.LatencyGraph(p.LatencyHistory),
	}
	if section := routesSummary(p); section != "" {
		identity = append(identity, "", section)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, identity...)
	return styles.Box(lay.rightW, lay.midH).Render(content)
}

// routesSummary renders up to maxDetailRoutes advertised routes for the details
// pane, with a "+N more / press e" indicator when the peer advertises more.
// Returns "" for peers with no advertised routes.
func routesSummary(p types.Peer) string {
	const maxDetailRoutes = 5
	n := len(p.AdvertisedRoutes)
	if n == 0 {
		return ""
	}
	lines := []string{styles.Heading.Render(fmt.Sprintf("ADVERTISED ROUTES (%d)", n))}
	for i := 0; i < n && i < maxDetailRoutes; i++ {
		lines = append(lines, "  "+styles.Online.Render("→ ")+styles.Value.Render(p.AdvertisedRoutes[i]))
	}
	if n > maxDetailRoutes {
		lines = append(lines, styles.Badge.Render(
			fmt.Sprintf("  [+%d more routes... Press 'e' to expand]", n-maxDetailRoutes)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// --- bottom pane: terminal logs ----------------------------------------------

func (m Model) renderLogsPane(w, h int) string {
	lines := []string{styles.Heading.Render("TERMINAL_LOGS")}
	for _, e := range m.logs {
		lvl := fmt.Sprintf("[%s]", e.Level)
		lines = append(lines, styles.Dim.Render("> ")+styles.Badge.Render(lvl)+" "+styles.Value.Render(e.Message))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return styles.Box(w, h).Render(content)
}

// --- small helpers -----------------------------------------------------------

func field(label, value string) string {
	return styles.Label.Render(label) + " " + styles.Value.Render(value)
}

func connText(c types.ConnType, relay string) string {
	if c == types.ConnRelay && relay != "" {
		return fmt.Sprintf("%s (via %s)", c, relay)
	}
	return c.String()
}

func connSymbol(c types.ConnType) string {
	switch c {
	case types.ConnDirect:
		return styles.Online.Render("●")
	case types.ConnRelay:
		return styles.Badge.Render("●") // yellow: connected but relayed
	default:
		return styles.Offline.Render("○")
	}
}

func onlineSymbol(online bool) string {
	if online {
		return styles.Online.Render("●")
	}
	return styles.Offline.Render("○")
}

func latencyStats(history []int) string {
	if len(history) == 0 {
		return styles.Dim.Render("no samples")
	}
	min, max, sum := history[0], history[0], 0
	for _, v := range history {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg := sum / len(history)
	return fmt.Sprintf("%s %s   %s %s   %s %s",
		styles.Label.Render("MIN:"), styles.Value.Render(fmt.Sprintf("%dms", min)),
		styles.Label.Render("AVG:"), styles.Value.Render(fmt.Sprintf("%dms", avg)),
		styles.Label.Render("MAX:"), styles.Value.Render(fmt.Sprintf("%dms", max)),
	)
}

// padRight pads s with spaces to the given display width (ANSI-aware).
func padRight(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap < 0 {
		gap = 0
	}
	return s + strings.Repeat(" ", gap)
}
