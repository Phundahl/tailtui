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
		lay := computeLayout(m.width, m.height)
		m.peers.SetSize(lay.listW, lay.listH)
		if m.state != stateMain {
			m = m.resizeOverlay()
		}
		return m, nil

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
	}

	// Everything else (navigation keys, filter input, etc.) goes to the list.
	var cmd tea.Cmd
	m.peers, cmd = m.peers.Update(msg)
	return m, cmd
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
// The list items are the source of truth, so we rewrite each one via SetItem;
// the dashboard derives its "Exit:" value from this same state.
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
	return m, tea.Batch(cmds...)
}
