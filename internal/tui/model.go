// Package tui implements the Bubble Tea program: Model, Update, and View.
//
// The architecture is the standard Elm loop. The peer pane is a
// bubbles/list.Model driving the details pane: whatever item is highlighted in
// the list is rendered on the right.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailtui/internal/mock"
	"github.com/Phundahl/tailtui/internal/types"
)

// appState is the top-level UI state machine. Only one overlay is active at a
// time; stateMain is the default full layout.
type appState int

const (
	stateMain appState = iota
	stateHelp
	stateRoutes
	stateAccounts
	stateLogs
)

// maxLogEntries caps the in-memory log ring (FIFO) so a long-running session
// can't leak memory. Oldest entries are dropped once the cap is exceeded.
const maxLogEntries = 500

// Model is the root Bubble Tea model holding all UI state.
type Model struct {
	// Terminal dimensions, updated on tea.WindowSizeMsg.
	width  int
	height int
	ready  bool

	// state selects which overlay (if any) is showing; overlay is the shared
	// scrollable viewport used to render the help and routes modals.
	state   appState
	overlay viewport.Model

	// Domain state.
	local    types.LocalStatus
	peers    list.Model   // the peer list; its highlighted item drives the details pane
	allPeers []types.Peer // full sorted set; the source of truth for filtering
	logs     []types.LogEntry

	// Search/filter state (custom, not bubbles/list's built-in filter — see
	// search.go). searchFocused == Input Mode; searchQuery == applied filter.
	searchQuery   string
	searchFocused bool

	// Accounts modal state.
	accounts      []types.Account
	accountCursor int

	// fetchErr holds the last `tailscale status` failure (nil when healthy); it
	// surfaces as an error line in the logs pane. The last good data stays on
	// screen across a transient failure.
	fetchErr error

	// latency holds live ping history per node, keyed by Tailscale IP. The ping
	// ticker measures the highlighted node; applyStatus re-injects these into
	// the list items so accumulated samples survive a status refresh.
	latency map[string][]int
}

// New constructs the initial model. Node data is empty until the first
// `tailscale status` poll resolves (kicked off by Init); only the logs and
// accounts panes remain mock-backed for now.
func New() Model {
	return Model{
		state:   stateMain,
		overlay: viewport.New(0, 0), // sized when an overlay is opened
		peers:   newPeerList(nil),
		logs:    mock.Logs(),
		latency: make(map[string][]int),
		// accounts are fetched live (tailscale switch --list) by Init / on open.
	}
}

// Init implements tea.Model: fetch live status + account profiles immediately,
// and start the status-refresh and ping tickers.
func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchStatusCmd(), fetchAccountsCmd(), tickCmd(), pingTickCmd())
}

// selectedPeer returns the peer currently highlighted in the list, and false
// when nothing is selected (e.g. a filter with no matches).
func (m Model) selectedPeer() (types.Peer, bool) {
	p, ok := m.peers.SelectedItem().(types.Peer)
	return p, ok
}

// activeExitNodeName returns the hostname of the active exit node, or "None"
// when no peer is currently selected as the exit node. The peer list is the
// source of truth, so the dashboard stays in sync with the list automatically.
func (m Model) activeExitNodeName() string {
	for _, item := range m.peers.Items() {
		if p, ok := item.(types.Peer); ok && p.IsActiveExitNode {
			return p.Hostname
		}
	}
	return "None"
}

// selectedAccount returns the account under the modal cursor, or false when the
// list is empty / the cursor is out of range.
func (m Model) selectedAccount() (types.Account, bool) {
	if m.accountCursor >= 0 && m.accountCursor < len(m.accounts) {
		return m.accounts[m.accountCursor], true
	}
	return types.Account{}, false
}

// activeExitNodeIP returns the Tailscale IP of the active exit node when one is
// set and online (so it can be pinged for the "Exit Latency" readout), or "".
func (m Model) activeExitNodeIP() string {
	for _, item := range m.peers.Items() {
		if p, ok := item.(types.Peer); ok && p.IsActiveExitNode && p.Online {
			return p.TailscaleIP
		}
	}
	return ""
}

// appendLog records a timestamped entry in the model's log ring, enforcing the
// FIFO cap. It returns the updated model (value receiver), so callers fold the
// result back into the Elm loop.
func (m Model) appendLog(level, message string) Model {
	m.logs = append(m.logs, types.LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: message,
	})
	if len(m.logs) > maxLogEntries {
		m.logs = m.logs[len(m.logs)-maxLogEntries:]
	}
	return m
}
