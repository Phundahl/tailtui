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
	logsHeight   = 5  // TERMINAL_LOGS pane (border + tail lines)
	localNodeH   = 11 // LOCAL_NODE pane height (border + fields + Connect)
	minLatencyH  = 4  // LATENCY HISTORY pane never shrinks below this
	gutter       = 1  // column gap between the left and right columns
	minWidth     = 72
	minHeight    = 24

	// PEER DETAILS content lines: IDENTITY + OS + IP + Conn + Version + Tags +
	// Last Seen; the advertised-routes hint adds one more (see detailLines).
	detailsBaseLines = 7

	// Floors used when the terminal is too short to honour the fixed heights.
	localFloor   = 6
	nodesFloor   = 3
	detailsFloor = 6 // IDENTITY + OS + IP + Conn (+ border)
	logsFloor    = 3
)

// layout holds the computed geometry of every region for a given terminal size.
//
// Phase 10 grid (two flush columns, each summing to bodyH):
//   - Left:  LOCAL_NODE (fixed) over FILTER NODES list (flex).
//   - Right: PEER DETAILS (fixed) over LATENCY HISTORY (flex) over TERMINAL_LOGS (fixed).
type layout struct {
	leftW, rightW int // column widths (leftW + gutter + rightW == width)
	bodyH         int // height of the body band (between header and footer)

	localH, nodesH            int // left-column pane heights (sum == bodyH)
	detailsH, latencyH, logsH int // right-column pane heights (sum == bodyH)

	listW, listH int // peer list dimensions inside the NODES pane
}

// computeLayout derives the geometry for a terminal of w×h. detailLines is the
// number of content rows PEER DETAILS needs (so a subnet router's routes hint
// gets a row); the LATENCY pane flexes to absorb the difference.
func computeLayout(w, h, detailLines int) layout {
	bodyH := h - headerHeight - footerHeight

	leftW := w * 2 / 5
	rightW := w - leftW - gutter

	// --- left column: LOCAL_NODE (fixed) over NODES list (flex) ---
	localH := localNodeH
	if localH > bodyH-nodesFloor {
		localH = bodyH - nodesFloor
	}
	if localH < localFloor {
		localH = localFloor
	}
	nodesH := bodyH - localH
	if nodesH < 1 {
		nodesH = 1
	}

	// --- right column: PEER DETAILS (fixed) over LATENCY (flex) over LOGS (fixed) ---
	detailsH := detailLines + 2 // + border
	logsH := logsHeight
	// Reserve at least minLatencyH for the flex latency pane; if it doesn't fit,
	// shrink PEER DETAILS toward its floor, then the logs tail.
	if over := detailsH + logsH + minLatencyH - bodyH; over > 0 {
		if take := min(over, detailsH-detailsFloor); take > 0 {
			detailsH -= take
			over -= take
		}
		if take := min(over, logsH-logsFloor); take > 0 {
			logsH -= take
		}
	}
	latencyH := bodyH - detailsH - logsH // flex: column sums to bodyH
	if latencyH < 1 {
		latencyH = 1
	}

	lay := layout{
		leftW:    leftW,
		rightW:   rightW,
		bodyH:    bodyH,
		localH:   localH,
		nodesH:   nodesH,
		detailsH: detailsH,
		latencyH: latencyH,
		logsH:    logsH,
		listW:    styles.ContentWidth(leftW),
		listH:    nodesH - 2, // NODES pane inner height
	}
	if lay.listH < 1 {
		lay.listH = 1
	}
	if lay.listW < 1 {
		lay.listW = 1
	}
	return lay
}

// detailLines reports how many content rows the PEER DETAILS pane needs for the
// current selection: the base identity fields, plus one for the advertised-
// routes hint when the highlighted peer is a subnet router.
func (m Model) detailLines() int {
	n := detailsBaseLines
	if p, ok := m.selectedPeer(); ok && len(p.AdvertisedRoutes) > 0 {
		n++
	}
	return n
}

// layout computes the current geometry, accounting for the selected peer's
// detail height. Used by both View (drawing) and Update (sizing the list).
func (m Model) layout() layout {
	return computeLayout(m.width, m.height, m.detailLines())
}

// View implements tea.Model and assembles the full-screen layout.
func (m Model) View() string {
	if !m.ready {
		return "Initializing Tailscale TUI..."
	}
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small (need at least %dx%d).", minWidth, minHeight)
	}

	lay := m.layout()

	// Left: LOCAL_NODE (fixed) over the FILTER NODES list (flex).
	left := lipgloss.JoinVertical(lipgloss.Left,
		m.renderLocalNode(lay),
		m.renderNodes(lay),
	)
	// Right: PEER DETAILS (fixed) over LATENCY (flex) over TERMINAL_LOGS tail.
	right := lipgloss.JoinVertical(lipgloss.Left,
		m.renderDetails(lay),
		m.renderLatency(lay),
		m.renderLogs(lay),
	)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gutter), right)

	base := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		body,
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
	// Help is already advertised in the header ("(?)help"), so it's omitted here
	// to leave room for the action hints. Bar() clips this responsively anyway.
	left := styles.Dim.Render("[j/k] Nav  [/] Search  [x] Exit  [O] Operator  [e] Routes  [v] Logs")
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
		styles.Label.Render("Exit Latency:") + " " + m.renderExitLatency(),
		"",
		lipgloss.PlaceHorizontal(cw, lipgloss.Center, styles.Button.Render("[ Connect ]")),
	}
	body := lipgloss.JoinVertical(lipgloss.Left, fields...)
	return styles.Pane("LOCAL_NODE", body, lay.leftW, lay.localH, false)
}

func (m Model) renderNodes(lay layout) string {
	// The bubbles list is the focused element, so the NODES pane gets the bright
	// border. It flexes to fill the left column below LOCAL_NODE.
	return styles.Pane("FILTER NODES...", m.peers.View(), lay.leftW, lay.nodesH, true)
}

func (m Model) renderExitValue() string {
	name := m.activeExitNodeName()
	if name == "None" {
		return styles.Dim.Render("None")
	}
	return styles.ExitName.Render("⏏ " + name)
}

// renderExitLatency shows the live ping latency to the active exit node (the
// node all traffic is routed through), or N/A when none is active. The exit
// node is pinged by the ticker regardless of selection, so this stays live; "—"
// is shown briefly until the first sample lands.
func (m Model) renderExitLatency() string {
	ip := m.activeExitNodeIP()
	if ip == "" {
		return styles.Dim.Render("N/A")
	}
	h := m.latency[ip]
	if len(h) == 0 {
		return styles.Dim.Render("—")
	}
	return styles.Value.Render(fmt.Sprintf("%dms", h[len(h)-1])) + " " + styles.Sparkline(h)
}

// --- right column: PEER DETAILS + LATENCY HISTORY ----------------------------

func (m Model) renderDetails(lay layout) string {
	p, ok := m.selectedPeer()
	if !ok {
		return styles.Pane("PEER DETAILS", styles.Dim.Render("No node selected."), lay.rightW, lay.detailsH, false)
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
	return styles.Pane("PEER DETAILS: "+p.Hostname, body, lay.rightW, lay.detailsH, false)
}

func (m Model) renderLatency(lay layout) string {
	cw := styles.ContentWidth(lay.rightW)
	// The chart fills the pane: inner height minus the stats line and its spacer.
	graphH := lay.latencyH - 2 - 2
	if graphH < 1 {
		graphH = 1
	}
	var graph, stats string
	if p, ok := m.selectedPeer(); ok {
		stats = latencyStats(p.LatencyHistory)
		graph = styles.LatencyGraphArea(p.LatencyHistory, cw, graphH)
	}
	body := lipgloss.JoinVertical(lipgloss.Left, stats, "", graph)
	return styles.Pane("LATENCY HISTORY", body, lay.rightW, lay.latencyH, false)
}

// --- right column bottom: TERMINAL_LOGS tail ---------------------------------

func (m Model) renderLogs(lay layout) string {
	var lines []string
	if m.fetchErr != nil {
		danger := lipgloss.NewStyle().Foreground(styles.Danger)
		lines = append(lines, danger.Render("> [ERROR] "+m.fetchErr.Error()))
	}
	// Tail only: show the most recent entries; the full history lives in the [v]
	// log overlay. Keeps the pane a fixed-height "ticker".
	const tail = 2
	if start := len(m.logs) - tail; start >= 0 {
		for _, e := range m.logs[start:] {
			lines = append(lines, logTailLine(e))
		}
	} else {
		for _, e := range m.logs {
			lines = append(lines, logTailLine(e))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return styles.Pane("TERMINAL_LOGS", body, lay.rightW, lay.logsH, false)
}

// logTailLine formats one entry for the bottom ticker pane (over the base
// background), coloring the level chip by severity.
func logTailLine(e types.LogEntry) string {
	lvl := styles.Dim
	switch e.Level {
	case "ERROR":
		lvl = lipgloss.NewStyle().Foreground(styles.Danger)
	case "WARN":
		lvl = styles.Caution
	}
	return styles.Dim.Render("> "+e.Time+" ") + lvl.Render("["+e.Level+"]") + " " + styles.Value.Render(e.Message)
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
