package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/types"
)

// Branding shown in the UI chrome.
const (
	appName    = "tailTUI"
	appVersion = "v1.1.0"
)

// Fixed layout constants.
const (
	headerHeight = 1
	footerHeight = 1
	logsHeight   = 5  // TERMINAL_LOGS pane (border + tail lines)
	localNodeH   = 11 // LOCAL_NODE pane height (border + fields + Connect + grouped Settings/Routing)
	minLatencyH  = 4  // LATENCY HISTORY pane never shrinks below this
	gutter       = 1  // column gap between the left and right columns
	minWidth     = 72
	minHeight    = 24

	// Floors used when the terminal is too short to honour the fixed heights.
	localFloor = 6
	nodesFloor = 3
	logsFloor  = 3
)

// layout holds the computed geometry of every region for a given terminal size.
//
// Phase 11 grid (two flush columns, each summing to bodyH):
//   - Left:  LOCAL_NODE (fixed) over FILTER NODES list (flex).
//   - Right: PEER DETAILS (fixed) over LATENCY HISTORY (flex) over TERMINAL_LOGS (fixed).
//
// PEER DETAILS is LOCKED to the LOCAL_NODE height (detailsH == localH) so the
// horizontal border under the two top panes aligns perfectly across the screen,
// regardless of the selected peer's content. The routes hint (when present) and
// any short fall are absorbed by the Pane's own bottom padding.
type layout struct {
	leftW, rightW int // column widths (leftW + gutter + rightW == width)
	bodyH         int // height of the body band (between header and footer)

	localH, nodesH            int // left-column pane heights (sum == bodyH)
	detailsH, latencyH, logsH int // right-column pane heights (sum == bodyH)

	listW, listH int // peer list dimensions inside the NODES pane
}

// computeLayout derives the geometry for a terminal of w×h.
func computeLayout(w, h int) layout {
	bodyH := h - headerHeight - footerHeight

	leftW := w * 2 / 5
	rightW := w - leftW - gutter

	logsH := logsHeight

	// LOCAL_NODE height: fixed, but clamped on short terminals so the (flex) node
	// list keeps nodesFloor and the (flex) latency pane keeps minLatencyH.
	localH := localNodeH
	if maxLocal := bodyH - logsH - minLatencyH; localH > maxLocal {
		localH = maxLocal
	}
	if localH > bodyH-nodesFloor {
		localH = bodyH - nodesFloor
	}
	if localH < localFloor {
		localH = localFloor
	}

	// SYMMETRY: PEER DETAILS shares the exact LOCAL_NODE height.
	detailsH := localH

	nodesH := bodyH - localH // left column sums to bodyH
	if nodesH < 1 {
		nodesH = 1
	}

	latencyH := bodyH - detailsH - logsH // right column sums to bodyH (flex)
	if latencyH < 1 {
		// Extreme: reclaim from the logs tail so the column still sums to bodyH.
		logsH = bodyH - detailsH - 1
		if logsH < logsFloor {
			logsH = logsFloor
		}
		latencyH = bodyH - detailsH - logsH
		if latencyH < 1 {
			latencyH = 1
		}
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

// layout computes the current geometry. Used by both View (drawing) and Update
// (sizing the list).
func (m Model) layout() layout {
	return computeLayout(m.width, m.height)
}

// View implements tea.Model and assembles the full-screen layout.
func (m Model) View() string {
	if !m.ready {
		return "Initializing " + appName + "..."
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
	left := styles.Title.Render(appName)
	right := styles.Dim.Render("(q)uit  (?)help  ⚙")
	return styles.Bar(m.width, left, right)
}

func (m Model) renderFooter() string {
	// Dynamic connection hint + state-aware status, kept short so Bar() doesn't
	// have to clip on common widths. "[x] Exit Node" (not "[x] Exit") avoids the
	// "quit the app" confusion. Help and Routes are discoverable elsewhere (the
	// header's "(?)help" and the PEER DETAILS "[e] N advertised routes" hint).
	connect := "[c] Connect"
	icon, status := styles.Offline.Render("○"), styles.Dim.Render(" DISCONNECTED")
	if m.localConnected() {
		connect = "[c] Disconnect"
		icon, status = styles.Online.Render("●"), styles.Dim.Render(" CONNECTED")
	}
	// "[x] Exit Node" is shown only when the highlighted peer actually offers
	// exit-node service (the toggle is a no-op otherwise), matching the
	// contextual-hint pattern used elsewhere.
	exitHint := ""
	if p, ok := m.selectedPeer(); ok && p.OffersExitNode {
		exitHint = "  [x] Exit Node"
	}
	left := styles.Dim.Render("[j/k] Nav  [/] Search  " + connect + exitHint + "  [O] Operator  [v] Logs")
	if m.searchFocused {
		// In Input Mode, surface the otherwise-hidden search navigation shortcuts.
		left = styles.Dim.Render("[↑↓ Ctrl+j/k] Nav   [Enter/Esc] Apply   type to filter")
	}
	// Version pinned to the far-right edge (Bar right-justifies this segment),
	// kept on the single footer line.
	right := icon + status + styles.Dim.Render("   [l] ACCOUNTS   "+appVersion)
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
		styles.Label.Render("Exit State:") + " " + m.renderExitState(),
		styles.Label.Render("Exit:") + " " + m.renderExitValue(),
		styles.Label.Render("Exit Latency:") + " " + m.renderExitLatency(),
		m.renderConnectButton(cw),
		m.renderActionButtons(cw),
	}
	body := lipgloss.JoinVertical(lipgloss.Left, fields...)
	return styles.Pane("LOCAL_NODE", body, lay.leftW, lay.localH, false)
}

// localConnected reports whether the local node is currently up on the tailnet,
// derived from the polled status (ConnOffline == down/stopped).
func (m Model) localConnected() bool {
	return m.local.Conn != types.ConnOffline
}

// renderConnectButton renders the centered, state-aware connection toggle:
// a green "[c] Connect" when the node is down, or a warning-yellow
// "[c] Disconnect" when it's up (pressing it will drop the tailnet connection).
func (m Model) renderConnectButton(cw int) string {
	label, st := "[c] Connect", styles.Button // Primary, bold
	if m.localConnected() {
		label = "[c] Disconnect"
		st = lipgloss.NewStyle().Foreground(styles.Warn).Bold(true)
	}
	return lipgloss.PlaceHorizontal(cw, lipgloss.Center, st.Render(label))
}

// renderActionButtons renders the modal-trigger actions — "[S] Advanced
// Settings" and "[R] Routing" — on a single centered line, grouped horizontally
// with spacing between them. [S] (shift+s) opens the settings modal and [R]
// (shift+r) the routing modal; the lowercase keys remain reserved. Keeping both
// on one row keeps the LOCAL_NODE pane from looking bottom-heavy.
func (m Model) renderActionButtons(cw int) string {
	row := styles.Button.Render("[S] Advanced Settings") +
		styles.Value.Render("    ") +
		styles.Button.Render("[R] Routing")
	return lipgloss.PlaceHorizontal(cw, lipgloss.Center, row)
}

func (m Model) renderNodes(lay layout) string {
	// The bubbles list is the focused element, so the NODES pane gets the bright
	// border. It flexes to fill the left column below LOCAL_NODE. The pane title
	// doubles as the search box: a trailing block cursor in Input Mode (focused),
	// no cursor when blurred — exactly the Normal-Mode indicator.
	title := "FILTER NODES..."
	if m.searchFocused {
		title = "SEARCH: " + searchDisplay(m.searchQuery, lay.leftW) + "▌"
	} else if m.searchQuery != "" {
		title = "FILTER: " + searchDisplay(m.searchQuery, lay.leftW)
	}
	return styles.Pane(title, m.peers.View(), lay.leftW, lay.nodesH, true)
}

// searchDisplay clamps the query so the NODES title still fits its top border,
// showing the tail (most recently typed) with a leading ellipsis when too long.
func searchDisplay(q string, leftW int) string {
	max := leftW - 14
	if max < 4 {
		max = 4
	}
	if r := []rune(q); len(r) > max {
		return "…" + string(r[len(r)-max+1:])
	}
	return q
}

// renderExitState shows the route state (DIRECT / RELAY) of the *active exit
// node* connection — the peer all traffic is currently routed through. It looks
// that peer up in the polled status (the list is the source of truth) and
// reports its real reachability. When no exit node is active it shows a dim
// "N/A", keeping the LOCAL_NODE layout static and consistent with the other
// inactive readouts (Exit / Exit Latency).
func (m Model) renderExitState() string {
	for _, item := range m.peers.Items() {
		if p, ok := item.(types.Peer); ok && p.IsActiveExitNode {
			return connSymbol(p.Conn) + " " + styles.Value.Render(connText(p.Conn, p.Relay))
		}
	}
	return styles.Dim.Render("N/A")
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
// background): muted timestamp, level chip colored by severity (theme), and the
// message in the default text color.
func logTailLine(e types.LogEntry) string {
	lvl := lipgloss.NewStyle().Foreground(styles.LogLevelColor(e.Level))
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
