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
	headerHeight = 1
	footerHeight = 1
	logsHeight   = 5  // TERMINAL_LOGS pane (border + log lines)
	localNodeH   = 11 // LOCAL_NODE pane height (border + fields + Connect)
	gutter       = 1  // column gap between the left and right columns
	minWidth     = 72
	minHeight    = 24
)

// layout holds the computed geometry of every region for a given terminal size.
type layout struct {
	leftW, rightW    int // column widths (sum + gutter == width)
	midH             int // height of the middle band (between header and logs)
	topH             int // height of the top pane in each column (aligned)
	nodesH, latencyH int // bottom pane heights per column
	listW, listH     int // peer list dimensions inside the NODES pane
}

func computeLayout(w, h int) layout {
	midH := h - headerHeight - footerHeight - logsHeight

	leftW := w * 2 / 5
	rightW := w - leftW - gutter

	topH := localNodeH
	if topH > midH-4 {
		topH = midH - 4 // keep at least a few rows for the lower panes
	}
	if topH < 3 {
		topH = 3
	}

	lay := layout{
		leftW:    leftW,
		rightW:   rightW,
		midH:     midH,
		topH:     topH,
		nodesH:   midH - topH,
		latencyH: midH - topH,
		listW:    styles.ContentWidth(leftW),
		listH:    midH - topH - 2, // NODES pane inner height
	}
	if lay.listH < 1 {
		lay.listH = 1
	}
	if lay.listW < 1 {
		lay.listW = 1
	}
	return lay
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

	left := lipgloss.JoinVertical(lipgloss.Left,
		m.renderLocalNode(lay),
		m.renderNodes(lay),
	)
	right := lipgloss.JoinVertical(lipgloss.Left,
		m.renderDetails(lay),
		m.renderLatency(lay),
	)
	mid := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gutter), right)

	base := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		mid,
		m.renderLogs(),
		m.renderFooter(),
	)

	// Overlays float on top of the still-visible base layout.
	if m.state != stateMain {
		return m.renderOverlay(base)
	}
	return base
}

// --- header / footer ---------------------------------------------------------

func (m Model) renderHeader() string {
	left := styles.Title.Render("TAILSCALE_TUI_V1.0")
	right := styles.Dim.Render("(q)uit  (?)help  ⚙")
	return styles.Bar(m.width, left, right)
}

func (m Model) renderFooter() string {
	left := styles.Dim.Render("[j/k] Navigate  [/] Search  [s] SSH  [p] Ping  [t] Connect  [e] Expand  [?] Help")
	right := styles.Online.Render("●") + styles.Dim.Render(" CONNECTED   [l] ACCOUNTS")
	return styles.Bar(m.width, left, right)
}

// --- left column: LOCAL_NODE + NODES -----------------------------------------

func (m Model) renderLocalNode(lay layout) string {
	l := m.local
	cw := styles.ContentWidth(lay.leftW)
	fields := []string{
		field("User:", l.User),
		field("Host:", l.Hostname),
		field("Local IP:", l.LocalIP),
		field("Tailscale IP:", l.TailscaleIP),
		styles.Label.Render("State:") + " " + connSymbol(l.Conn) + " " + styles.Value.Render(connText(l.Conn, l.Relay)),
		styles.Label.Render("Exit:") + " " + m.renderExitValue(),
		styles.Label.Render("Latency:") + " " + styles.Value.Render(fmt.Sprintf("%dms", l.LatencyMs)) + " " + styles.Sparkline(l.LatencyHistory),
		"",
		lipgloss.PlaceHorizontal(cw, lipgloss.Center, styles.Button.Render("[ Connect ]")),
	}
	body := lipgloss.JoinVertical(lipgloss.Left, fields...)
	return styles.Pane("LOCAL_NODE", body, lay.leftW, lay.topH, false)
}

func (m Model) renderNodes(lay layout) string {
	// The bubbles list is the focused element, so the NODES pane gets the bright border.
	return styles.Pane("FILTER NODES...", m.peers.View(), lay.leftW, lay.nodesH, true)
}

func (m Model) renderExitValue() string {
	name := m.activeExitNodeName()
	if name == "None" {
		return styles.Dim.Render("None")
	}
	return styles.ExitName.Render("⏏ " + name)
}

// --- right column: PEER DETAILS + LATENCY HISTORY ----------------------------

func (m Model) renderDetails(lay layout) string {
	p, ok := m.selectedPeer()
	if !ok {
		return styles.Pane("PEER DETAILS", styles.Dim.Render("No node selected."), lay.rightW, lay.topH, false)
	}
	lines := []string{
		styles.Label.Render("IDENTITY"),
		field("OS:", p.OS.Icon()+" "+p.OS.Name()),
		field("IP:", p.TailscaleIP),
		styles.Label.Render("Conn:") + " " + connSymbol(p.Conn) + " " + styles.Value.Render(connText(p.Conn, p.Relay)),
		field("Version:", p.Version),
		field("Tags:", tagList(p.Tags)),
		field("Last Seen:", p.LastSeen),
	}
	if n := len(p.AdvertisedRoutes); n > 0 {
		lines = append(lines, styles.Caution.Render(fmt.Sprintf("[e] %d advertised routes", n)))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return styles.Pane("PEER DETAILS: "+p.Hostname, body, lay.rightW, lay.topH, false)
}

func (m Model) renderLatency(lay layout) string {
	cw := styles.ContentWidth(lay.rightW)
	var graph, stats string
	if p, ok := m.selectedPeer(); ok {
		stats = latencyStats(p.LatencyHistory)
		graph = styles.LatencyGraphWidth(p.LatencyHistory, cw)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, stats, "", graph)
	return styles.Pane("LATENCY HISTORY (60s)", body, lay.rightW, lay.latencyH, false)
}

// --- bottom: TERMINAL_LOGS ---------------------------------------------------

func (m Model) renderLogs() string {
	lines := make([]string, 0, len(m.logs))
	for _, e := range m.logs {
		lines = append(lines, styles.Dim.Render("> ["+e.Level+"]")+" "+styles.Value.Render(e.Message))
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return styles.Pane("TERMINAL_LOGS", body, m.width, logsHeight, false)
}

// --- small helpers -----------------------------------------------------------

func field(label, value string) string {
	return styles.Label.Render(label) + " " + styles.Value.Render(value)
}

// tagList renders tags as dim-bracketed chips: [tag:dev].
func tagList(tags []string) string {
	if len(tags) == 0 {
		return styles.Dim.Render("—")
	}
	chips := make([]string, len(tags))
	for i, t := range tags {
		chips[i] = styles.Dim.Render("[") + styles.Online.Render(t) + styles.Dim.Render("]")
	}
	return strings.Join(chips, " ")
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
		return styles.Caution.Render("●") // yellow: connected but relayed
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
