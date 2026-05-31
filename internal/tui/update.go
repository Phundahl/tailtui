package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// Update implements tea.Model.
//
// Navigation (j/k, arrows), fuzzy filtering ("/"), and selection are delegated
// to the embedded bubbles/list. We only intercept resize and quit — and we are
// careful not to swallow "q" while the user is typing into the filter.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		lay := m.layout()
		m.peers.SetSize(lay.listW, lay.listH)
		if m.state != stateMain {
			m = m.resizeOverlay()
		}
		return m, nil

	case statusMsg:
		return m.applyStatus(msg)

	case tickMsg:
		// Each tick fires the next background fetch and reschedules itself.
		return m, tea.Batch(fetchStatusCmd(), tickCmd())

	case pingTickMsg:
		// Ping the highlighted online node, and also the active exit node (so the
		// LOCAL_NODE "Exit Latency" readout stays live even when it isn't
		// selected). Always reschedule the ticker.
		cmds := []tea.Cmd{pingTickCmd()}
		pinged := map[string]bool{}
		if p, ok := m.selectedPeer(); ok && p.Online && p.TailscaleIP != "" {
			cmds = append(cmds, pingCmd(p.TailscaleIP))
			pinged[p.TailscaleIP] = true
		}
		if ip := m.activeExitNodeIP(); ip != "" && !pinged[ip] {
			cmds = append(cmds, pingCmd(ip))
		}
		return m, tea.Batch(cmds...)

	case pingMsg:
		return m.applyPing(msg)

	case actionMsg:
		// Record the outcome of a CLI action (e.g. exit-node set) in the log ring
		// so it persists for the [v] log overlay instead of flashing past.
		if msg.err != nil {
			return m.appendLog("ERROR", msg.desc+": "+msg.err.Error()), nil
		}
		return m.appendLog("INFO", msg.desc), nil

	case operatorDoneMsg:
		// The interactive `sudo tailscale set --operator` finished and the TUI is
		// restored; log the outcome and refresh status so new perms take effect.
		if msg.err != nil {
			return m.appendLog("ERROR", "operator setup: "+msg.err.Error()), nil
		}
		return m.appendLog("INFO", "operator set to "+currentUser()+"; refreshing"), fetchStatusCmd()

	case tea.KeyMsg:
		// ctrl+c always quits, even mid-filter or with an overlay open.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// While an overlay is open, all keys are routed to it (scroll/close) and
		// never reach the background peer list.
		if m.state != stateMain {
			return m.updateOverlay(msg)
		}
		// While typing into the filter, all other keys are literal text — don't
		// treat command keys as commands.
		if m.peers.FilterState() != list.Filtering {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "?":
				return m.openHelp(), nil
			case "e":
				if p, ok := m.selectedPeer(); ok && len(p.AdvertisedRoutes) > 0 {
					return m.openRoutes(p), nil
				}
				return m, nil
			case "l":
				return m.openAccounts(), nil
			case "v":
				return m.openLogs(), nil
			case "O":
				// Suspend the TUI and run the interactive sudo operator setup.
				return m, operatorSetupCmd()
			case "x", "t":
				return m.toggleExitNode()
			case "up", "k":
				// Wrap to the bottom only when already at the top; otherwise
				// fall through and let the list move/paginate normally.
				if nm, ok := m.wrapNav(-1); ok {
					return nm, nil
				}
			case "down", "j":
				if nm, ok := m.wrapNav(+1); ok {
					return nm, nil
				}
			}
		}
	}

	// Everything else (navigation keys, filter input, etc.) goes to the list.
	var cmd tea.Cmd
	m.peers, cmd = m.peers.Update(msg)
	return m, cmd
}

// applyStatus folds a completed `tailscale status` fetch into the model. It
// records any error (kept visible in the logs pane), refreshes the local node,
// and rebuilds the peer list — preserving the highlighted node by hostname so a
// background refresh doesn't yank the user's selection. The peer list is left
// untouched while the user is actively typing a "/" filter, so a poll can't
// disrupt filtering; the next tick reconciles it.
func (m Model) applyStatus(msg statusMsg) (tea.Model, tea.Cmd) {
	prevErr := m.fetchErr
	m.fetchErr = msg.err
	if msg.err != nil {
		// Log the failure once (on the transition) so it persists in the log
		// overlay without flooding the ring every 4s while the daemon is down.
		if prevErr == nil {
			m = m.appendLog("ERROR", msg.err.Error())
		}
		return m, nil // keep the last good data on screen
	}
	if prevErr != nil {
		m = m.appendLog("INFO", "reconnected to tailscaled")
	}
	m.local = msg.local

	if m.peers.FilterState() == list.Filtering {
		return m, nil
	}

	prev, _ := m.selectedPeer()
	cmd := m.peers.SetItems(peerItems(m.withLatency(msg.peers)))
	if prev.Hostname != "" {
		for i, it := range m.peers.Items() {
			if p, ok := it.(types.Peer); ok && p.Hostname == prev.Hostname {
				m.peers.Select(i)
				break
			}
		}
	}
	return m, cmd
}

// withLatency overlays each peer's accumulated live ping history (keyed by
// Tailscale IP) onto the freshly fetched peers, so the latency graph survives a
// status refresh instead of resetting every 4s.
func (m Model) withLatency(peers []types.Peer) []types.Peer {
	for i := range peers {
		if h := m.latency[peers[i].TailscaleIP]; len(h) > 0 {
			peers[i].LatencyHistory = h
			peers[i].LatencyMs = h[len(h)-1]
		}
	}
	return peers
}

// applyPing records a live latency sample and updates the matching list item in
// place, so the LATENCY HISTORY pane reflects the new reading immediately. A
// failed ping (ok == false) is ignored so the graph isn't poisoned by a zero.
func (m Model) applyPing(msg pingMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		return m, nil
	}
	h := append(m.latency[msg.ip], msg.ms)
	if len(h) > maxLatencySamples {
		h = h[len(h)-maxLatencySamples:]
	}
	m.latency[msg.ip] = h

	for i, it := range m.peers.Items() {
		if p, ok := it.(types.Peer); ok && p.TailscaleIP == msg.ip {
			p.LatencyHistory = h
			p.LatencyMs = h[len(h)-1]
			return m, m.peers.SetItem(i, p)
		}
	}
	return m, nil
}

// wrapNav implements wrap-around (infinite) scrolling for single-step
// navigation. dir is -1 for up/k and +1 for down/j. It only acts at a boundary
// — at the top going up, or the bottom going down — jumping to the opposite
// end and reporting handled=true. Otherwise it returns handled=false so the
// caller defers to the list's normal movement (and pagination).
//
// Indices are over the VISIBLE (filtered) items, so wrap-around respects an
// active "/" filter and stays within the matching subset.
func (m Model) wrapNav(dir int) (Model, bool) {
	n := len(m.peers.VisibleItems())
	if n <= 1 {
		return m, false // nothing to wrap with 0 or 1 items
	}
	switch idx := m.peers.Index(); {
	case dir < 0 && idx == 0:
		m.peers.Select(n - 1)
		return m, true
	case dir > 0 && idx == n-1:
		m.peers.Select(0)
		return m, true
	default:
		return m, false
	}
}

// toggleExitNode toggles the highlighted peer as the active exit node:
//   - if it is not active, it becomes the sole active exit node (all others off);
//   - if it is already active, it is turned off (no active exit node).
//
// It updates the list optimistically (instant UI feedback) AND issues the real
// `tailscale set --exit-node=…` command. The optimistic state holds until the
// next status poll reconciles the model with the daemon's truth — so a
// successful change shows no flicker, and a failed one reverts on the next poll.
func (m Model) toggleExitNode() (tea.Model, tea.Cmd) {
	sel, ok := m.selectedPeer()
	if !ok || !sel.OffersExitNode {
		// Only exit-capable nodes can be toggled; ignore the key otherwise.
		return m, nil
	}
	enable := !sel.IsActiveExitNode // toggle the target's state

	var cmds []tea.Cmd
	for i, item := range m.peers.Items() {
		p, ok := item.(types.Peer)
		if !ok {
			continue
		}
		p.IsActiveExitNode = enable && p.ID == sel.ID
		cmds = append(cmds, m.peers.SetItem(i, p))
	}

	// Drive the daemon: set this node's IP as the exit node, or clear it.
	ip, desc := "", "cleared exit node"
	if enable {
		ip = sel.TailscaleIP
		desc = "set exit node → " + sel.Hostname
	}
	cmds = append(cmds, setExitNodeCmd(ip, desc))
	return m, tea.Batch(cmds...)
}
