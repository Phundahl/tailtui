// Package tui implements the Bubble Tea program: Model, Update, and View.
//
// The architecture is the standard Elm loop. The peer pane is a
// bubbles/list.Model driving the details pane: whatever item is highlighted in
// the list is rendered on the right.
package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailscaleTUI/internal/mock"
	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// appState is the top-level UI state machine. Only one overlay is active at a
// time; stateMain is the default full layout.
type appState int

const (
	stateMain appState = iota
	stateHelp
	stateRoutes
)

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
	local types.LocalStatus
	peers list.Model // the peer list; its highlighted item drives the details pane
	logs  []types.LogEntry
}

// New constructs the initial model populated with mock data.
func New() Model {
	return Model{
		state:   stateMain,
		overlay: viewport.New(0, 0), // sized when an overlay is opened
		local:   mock.Local(),
		peers:   newPeerList(mock.Peers()),
		logs:    mock.Logs(),
	}
}

// Init implements tea.Model. No initial command for the mock-data build.
func (m Model) Init() tea.Cmd {
	return nil
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
