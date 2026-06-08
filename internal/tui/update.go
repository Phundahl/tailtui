package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailtui/internal/types"
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
		// restored; whatever the outcome, drop back to the main view and refresh
		// every live data source so any newly-granted perms (accounts visible,
		// prefs readable, status reachable) reflect immediately.
		m.state = stateMain
		refresh := tea.Batch(fetchStatusCmd(), fetchAccountsCmd(), fetchPrefsCmd())
		if msg.err != nil {
			return m.appendLog("ERROR", "operator setup: "+msg.err.Error()), refresh
		}
		return m.appendLog("INFO", "operator set to "+currentUser()+"; refreshing"), refresh

	case connectDoneMsg:
		// `tailscale up`/`down` finished and the TUI is restored; log + refresh so
		// the new state (and any auth completion) reflects immediately.
		action := "tailscale down"
		if msg.up {
			action = "tailscale up"
		}
		if msg.err != nil {
			return m.appendLog("ERROR", action+": "+msg.err.Error()), nil
		}
		return m.appendLog("INFO", action+" succeeded; refreshing"), fetchStatusCmd()

	case prefsMsg:
		// Live local-node preferences arrived; store them so the Advanced Settings
		// checkboxes (rendered from m.prefs each frame) reflect reality. Keep the
		// last good prefs on error.
		if msg.err != nil {
			return m.appendLog("ERROR", "read prefs: "+msg.err.Error()), nil
		}
		m.prefs = msg.prefs
		// If the routing modal is open but the user hasn't edited the working copy
		// yet (and isn't mid-CIDR-entry), refresh that copy from the latest daemon
		// read so it shows current data; once dirtied, local edits are preserved.
		if m.state == stateRouting && !m.routingDirty && !m.routingInputMode {
			m.routingExitNode = m.prefs.AdvertiseExitNode
			m.routingRoutes = append([]string(nil), m.prefs.AdvertiseRoutes...)
			if m.routingCursor >= m.routingItemCount() {
				m.routingCursor = 0
			}
			m.refreshRoutingOverlay()
		}
		return m, nil

	case prefActionMsg:
		// A `tailscale set --<flag>` toggle finished; log it and re-fetch prefs so
		// the checkbox reconciles with the daemon (reverting a failed optimistic
		// flip on the way back).
		if msg.err != nil {
			return m.appendLog("ERROR", msg.desc+": "+msg.err.Error()), fetchPrefsCmd()
		}
		return m.appendLog("INFO", msg.desc), fetchPrefsCmd()

	case routingActionMsg:
		// The `tailscale set --advertise-…` command finished; log the executed
		// command and refresh status + prefs so the new advertised state shows.
		if msg.err != nil {
			return m.appendLog("ERROR", "apply routing: "+msg.err.Error()), tea.Batch(fetchStatusCmd(), fetchPrefsCmd())
		}
		return m.appendLog("INFO", "applied: "+msg.desc), tea.Batch(fetchStatusCmd(), fetchPrefsCmd())

	case clipboardMsg:
		// Copy-to-clipboard finished; flash "Copied!" in the Command Room (or log
		// the failure when no clipboard tool is available).
		if msg.err != nil {
			return m.appendLog("ERROR", "clipboard: "+msg.err.Error()), nil
		}
		m.routingCopied = true
		return m.appendLog("INFO", "routing command copied to clipboard"), nil

	case accountsMsg:
		// Live profile list arrived; refresh the model and re-render the modal if
		// it's open. Keep the last good list on error. A successful read also
		// clears profilesLocked — perms may have just been granted (e.g. operator
		// setup just completed) and the lock hint must drop on the next refresh.
		if msg.err != nil {
			return m.appendLog("ERROR", msg.err.Error()), nil
		}
		m.profilesLocked = false
		m.accounts = msg.accounts
		if m.accountCursor >= len(m.accounts) {
			m.accountCursor = 0
		}
		if m.state == stateAccounts {
			m.overlay.SetContent(m.accountsBody(m.overlay.Width))
		}
		return m, nil

	case accountsLockedMsg:
		// "profiles access denied" — the profile store is root-owned and the
		// session isn't elevated. Mark the lock so the modal can render its
		// "run with sudo" hint, and clear the stored accounts so a stale list
		// from a previous run can't linger behind the lock. Intentionally NO
		// log entry: this command is re-fired on every refresh / action and
		// would otherwise paper the ring with the same line every few seconds.
		m.profilesLocked = true
		m.accounts = nil
		m.accountCursor = 0
		if m.state == stateAccounts {
			m.overlay.SetContent(m.accountsBody(m.overlay.Width))
		}
		return m, nil

	case accountActionMsg:
		// An account command (switch / remove / logout / login) finished; log it
		// and refresh every live data source. Login in particular can change which
		// prefs the daemon will return for this user, so prefs are refreshed too.
		// On error keep the accounts refresh (so the row reverts if needed); on
		// success do the full sync.
		if msg.err != nil {
			return m.appendLog("ERROR", msg.desc+": "+msg.err.Error()), fetchAccountsCmd()
		}
		return m.appendLog("INFO", msg.desc), tea.Batch(fetchAccountsCmd(), fetchStatusCmd(), fetchPrefsCmd())

	case tea.KeyMsg:
		// ctrl+c always quits, even mid-search or with an overlay open.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// While an overlay is open, all keys are routed to it (scroll/close) and
		// never reach the background peer list.
		if m.state != stateMain {
			return m.updateOverlay(msg)
		}
		// Input Mode: the search box is focused — keys edit the query or navigate
		// the filtered list, and never trigger commands or reach the list keymap.
		if m.searchFocused {
			return m.updateSearchInput(msg)
		}
		// Normal Mode commands.
		switch msg.String() {
		case "/":
			m.searchFocused = true // enter Input Mode, keep any existing query
			return m, nil
		case "esc":
			// In Normal Mode, Esc clears an applied filter (full list); otherwise
			// it's a no-op.
			if m.searchQuery != "" {
				m.clearSearch()
			}
			return m, nil
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
			// Open the accounts modal and refresh the live profile list.
			return m.openAccounts(), fetchAccountsCmd()
		case "v":
			return m.openLogs(), nil
		case "S":
			// Open the Advanced Settings modal (uppercase S / shift+s) and refresh
			// the live prefs.
			return m.openSettings(), fetchPrefsCmd()
		case "s":
			// Lowercase `s` is intentionally reserved for the future SSH-as-action
			// feature; ignore it for now so it never reaches the list keymap.
			return m, nil
		case "R":
			// Open the Routing Management modal (uppercase R / shift+r) and refresh
			// the live prefs so the advertised routes are current.
			return m.openRouting(), fetchPrefsCmd()
		case "r":
			// Lowercase `r` is reserved for a future routing-related action; ignore
			// it for now so it never reaches the list keymap.
			return m, nil
		case "O":
			// Suspend the TUI and run the interactive sudo operator setup.
			return m, operatorSetupCmd()
		case "c":
			// Toggle the tailnet connection: down if up, up if down. Runs
			// interactively so an auth URL from `up` is visible.
			return m, connectCmd(!m.localConnected())
		case "x":
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

	// Everything else (navigation keys, filter input, etc.) goes to the list.
	var cmd tea.Cmd
	m.peers, cmd = m.peers.Update(msg)
	return m, cmd
}

// updateSearchInput handles keys while the search box is focused (Input Mode).
// It consumes EVERY key so nothing reaches the command switch or the list keymap
// while typing. Enter/Esc blur back to Normal Mode (keeping the filter); arrows
// and Ctrl+j/k navigate the filtered list; a single printable rune types.
func (m Model) updateSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.searchFocused = false // blur → Normal Mode, keep the filter applied
		return m, nil
	case "up", "ctrl+k":
		m.searchNav(-1)
		return m, nil
	case "down", "ctrl+j":
		m.searchNav(+1)
		return m, nil
	case "backspace":
		m.backspaceSearch()
		return m, nil
	default:
		// A single printable rune (incl. space) types into the query; named keys
		// (tab, pgdown, …) are ignored so they can't corrupt the filter.
		if s := msg.String(); len([]rune(s)) == 1 {
			m.typeSearch(s)
		}
		return m, nil
	}
}

// applyStatus folds a completed `tailscale status` fetch into the model. It
// records any error (kept visible in the logs pane), refreshes the local node
// and the full peer set, then rebuilds the (possibly filtered) visible list —
// preserving the highlighted node by hostname, and always clamping the cursor so
// a refresh can never leave it out of range.
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
	m.allPeers = sortPeers(msg.peers)

	prev, _ := m.selectedPeer()
	m.peers.SetItems(m.filteredItems())

	target := 0 // selection lost (filtered out / removed) → top
	if prev.Hostname != "" {
		for i, it := range m.peers.Items() {
			if p, ok := it.(types.Peer); ok && p.Hostname == prev.Hostname {
				target = i
				break
			}
		}
	}
	m.selectClamped(target)
	return m, nil
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
