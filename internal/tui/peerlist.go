package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailscaleTUI/internal/styles"
	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// newPeerList builds the bubbles/list model for the peer pane. Special nodes
// (exit nodes, subnet routers) are sorted to the top per the design spec.
func newPeerList(peers []types.Peer) list.Model {
	sorted := sortPeers(peers)
	items := make([]list.Item, len(sorted))
	for i, p := range sorted {
		items[i] = p
	}

	l := list.New(items, peerDelegate{}, 0, 0)

	// Strip the default chrome we render ourselves: the NODES pane supplies the
	// title in its top border; the filter input still appears while filtering.
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)

	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(styles.Primary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(styles.Primary)
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(styles.Subtle)
	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(styles.Primary)
	l.FilterInput.TextStyle = lipgloss.NewStyle().Foreground(styles.Fg)

	return l
}

// sortPeers returns a copy ordered with exit nodes and subnet routers first,
// preserving the relative order within each group (stable).
func sortPeers(peers []types.Peer) []types.Peer {
	out := make([]types.Peer, len(peers))
	copy(out, peers)
	sort.SliceStable(out, func(i, j int) bool {
		return nodeRank(out[i]) < nodeRank(out[j])
	})
	return out
}

func nodeRank(p types.Peer) int {
	if p.NodeType == types.NodeRegular {
		return 1
	}
	return 0 // exit nodes and subnet routers float to the top
}

// peerDelegate renders one peer per line in the dense "Matrix Core" style:
//
//	> 󰖟 [EXIT] amsterdam-exit ●
//
// It implements list.ItemDelegate.
type peerDelegate struct{}

func (peerDelegate) Height() int                         { return 1 }
func (peerDelegate) Spacing() int                        { return 0 }
func (peerDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (peerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	p, ok := item.(types.Peer)
	if !ok {
		return
	}
	selected := index == m.Index()
	width := m.Width()

	glyph := "○"
	if p.Online {
		glyph = "●"
	}

	// Selected row: a full-width surface-bright bar with a leading ❯ pointer.
	// Built from plain text so the single Selected style paints the whole row
	// uniformly (no fg-only gaps in the highlight).
	if selected {
		exit := ""
		if p.IsActiveExitNode {
			exit = "󰖟 exit  "
		}
		left := "❯ " + joinFields(p.Icon(), p.Badge(), p.Hostname)
		row := joinRow(left, exit+glyph, width)
		fmt.Fprint(w, styles.Selected.Render(row))
		return
	}

	// Unselected row: themed per element.
	icon := styles.IconOnline.Render(p.Icon())
	if !p.Online {
		icon = styles.IconOffline.Render(p.Icon())
	}
	badge := ""
	if b := p.Badge(); b != "" {
		badge = styles.Badge.Render(b)
	}
	name := styles.Value.Render(p.Hostname)
	if p.IsActiveExitNode {
		name = styles.ExitName.Render(p.Hostname)
	}
	right := styles.Offline.Render(glyph)
	if p.Online {
		right = styles.Online.Render(glyph)
	}
	if p.IsActiveExitNode {
		right = styles.ExitName.Render("󰖟 exit") + "  " + right
	}
	left := "  " + joinFields(icon, badge, name)
	fmt.Fprint(w, joinRow(left, right, width))
}

// joinRow left-aligns left and right-aligns right within the given display
// width (ANSI-aware), with at least one space between them.
func joinRow(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// joinFields joins non-empty fields with single spaces.
func joinFields(fields ...string) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			parts = append(parts, f)
		}
	}
	return strings.Join(parts, " ")
}
