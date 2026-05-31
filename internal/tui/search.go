package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"

	"github.com/Phundahl/tailscaleTUI/internal/types"
)

// Search/filter is managed by hand rather than via bubbles/list's built-in
// filter (which is disabled). This gives explicit fzf-style modes — Input
// (search box focused, typing filters) vs Normal (box blurred, j/k navigate) —
// deterministic cursor clamping (no out-of-bounds panic), and Ctrl+j/k support.
//
// State lives in three Model fields: allPeers (the full sorted set, the source
// of truth), searchQuery (current filter text), and searchFocused (Input Mode).

// searchActive reports whether a filter is in play (box focused or a query
// applied). Drives the NODES pane title and the dynamic footer hint.
func (m Model) searchActive() bool { return m.searchFocused || m.searchQuery != "" }

// filterPeers narrows allPeers by the current query (case-insensitive fuzzy
// subsequence over hostname+tags), preserving the special-node-first order. An
// empty query returns a copy of the full set. Always returns a fresh slice so
// withLatency can mutate it without touching allPeers.
func (m Model) filterPeers() []types.Peer {
	q := strings.TrimSpace(m.searchQuery)
	if q == "" {
		return append([]types.Peer(nil), m.allPeers...)
	}
	out := make([]types.Peer, 0, len(m.allPeers))
	for _, p := range m.allPeers {
		if fuzzyMatch(q, p.FilterValue()) {
			out = append(out, p)
		}
	}
	return out
}

// filteredItems builds the list items for the current filter, with live latency
// re-injected.
func (m Model) filteredItems() []list.Item {
	return toItems(m.withLatency(m.filterPeers()))
}

// fuzzyMatch reports whether every rune of term appears in target in order
// (case-insensitive) — the fzf-style subsequence test.
func fuzzyMatch(term, target string) bool {
	tr := []rune(strings.ToLower(term))
	ti := 0
	for _, c := range strings.ToLower(target) {
		if ti < len(tr) && c == tr[ti] {
			ti++
		}
	}
	return ti == len(tr)
}

// toItems wraps peers as list.Items WITHOUT re-sorting (allPeers is already
// sorted and filtering preserves that order).
func toItems(peers []types.Peer) []list.Item {
	items := make([]list.Item, len(peers))
	for i, p := range peers {
		items[i] = p
	}
	return items
}

// --- mutation helpers (pointer receiver; m is always addressable in Update) ---

// applyFilter rebuilds the visible list from the current query and resets the
// cursor to the top — so a shrinking match set can never leave a stale cursor
// pointing past the end of the list (the crash this phase fixes).
func (m *Model) applyFilter() {
	m.peers.SetItems(m.filteredItems())
	m.selectClamped(0)
}

// clearSearch drops the query, blurs the box, and restores the full list.
func (m *Model) clearSearch() {
	m.searchQuery = ""
	m.searchFocused = false
	m.applyFilter()
}

// typeSearch appends s to the query and re-filters (cursor → 0).
func (m *Model) typeSearch(s string) {
	m.searchQuery += s
	m.applyFilter()
}

// backspaceSearch removes the last rune from the query and re-filters.
func (m *Model) backspaceSearch() {
	if m.searchQuery == "" {
		return
	}
	r := []rune(m.searchQuery)
	m.searchQuery = string(r[:len(r)-1])
	m.applyFilter()
}

// selectClamped selects index i, clamped to [0, len-1]. Safe on an empty list.
// Every cursor move funnels through here, which is what makes navigation panic-
// proof regardless of how far the filter shrinks the list.
func (m *Model) selectClamped(i int) {
	n := len(m.peers.Items())
	if n == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i > n-1 {
		i = n - 1
	}
	m.peers.Select(i)
}

// searchNav moves the cursor by dir (-1 up, +1 down) within the filtered list,
// clamped — used for arrow / Ctrl+j/k navigation while the box is focused.
func (m *Model) searchNav(dir int) {
	if len(m.peers.Items()) == 0 {
		return
	}
	m.selectClamped(m.peers.Index() + dir)
}
