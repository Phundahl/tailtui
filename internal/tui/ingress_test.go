package tui

import (
	"strings"
	"testing"

	"github.com/Phundahl/tailtui/internal/types"
)

// ingressPeers builds a tailnet with one real node and n funnel ingress nodes,
// the shape a live daemon reports whenever a funnel is up.
func ingressPeers(n int) []types.Peer {
	out := []types.Peer{{
		Hostname: "srv-web-01", DNSName: "srv-web-01.example-tailnet.ts.net",
		OS: types.OSLinux, TailscaleIP: "100.64.0.10",
		Conn: types.ConnDirect, Online: true,
	}}
	for i := 0; i < n; i++ {
		out = append(out, types.Peer{
			Hostname: "funnel-ingress-node", Tags: []string{"tag:ingress"},
			TailscaleIP: "100.64.9.1", Conn: types.ConnRelay,
			Online: true, IsIngress: true,
		})
	}
	return out
}

// A live funnel adds ~23 ingress nodes, so a list that showed them would be
// almost entirely infrastructure. They are hidden at rest.
func TestIngressNodesAreHiddenFromTheRestingList(t *testing.T) {
	m := newModelWithPeers(t, 120, 40, ingressPeers(23)...)

	if got := len(m.peers.VisibleItems()); got != 1 {
		t.Fatalf("resting list shows %d peers, want 1 real node", got)
	}
	if v := m.View(); strings.Contains(v, "funnel-ingress-node") {
		t.Fatal("an ingress node reached the resting list")
	}
	// Hidden, not dropped: the source of truth keeps all 24.
	if got := len(m.allPeers); got != 24 {
		t.Fatalf("allPeers holds %d, want all 24 — hiding must not discard", got)
	}
}

// Hiding silently and hiding visibly are different claims. The title makes it
// the second one, and points at the way back.
func TestHiddenIngressIsDeclaredInTheTitle(t *testing.T) {
	m := newModelWithPeers(t, 120, 40, ingressPeers(23)...)
	if v := m.View(); !strings.Contains(v, "23 ingress hidden") {
		t.Fatal("the title does not declare what is being hidden")
	}
	assertFlush(t, m.View(), 120, 40)

	// No funnel, nothing hidden, no label.
	clean := newModelWithPeers(t, 120, 40, ingressPeers(0)...)
	if v := clean.View(); strings.Contains(v, "ingress hidden") {
		t.Fatal("the label shows when nothing is hidden")
	}
}

// The escape hatch: a query still reaches them, so the tool declutters rather
// than deciding what the user may see.
func TestSearchStillFindsIngressNodes(t *testing.T) {
	m := newModelWithPeers(t, 120, 40, ingressPeers(23)...)
	m = mustModel(m.Update(key("/")))
	for _, r := range "ingress" {
		m = mustModel(m.Update(key(string(r))))
	}
	if got := len(m.peers.VisibleItems()); got != 23 {
		t.Fatalf("search found %d ingress nodes, want 23", got)
	}
	// While a query is showing them, nothing is being withheld.
	if n := m.hiddenIngress(); n != 0 {
		t.Fatalf("hiddenIngress = %d during a query, want 0", n)
	}
	if v := m.View(); strings.Contains(v, "ingress hidden") {
		t.Fatal("still claiming to hide nodes that are on screen")
	}
}

// styles.titledTop drops an over-long title ENTIRELY rather than truncating it,
// so an unconditional count suffix costs the whole search indicator on a narrow
// terminal. Regression: this silently removed "FILTER NODES..." at 80 columns.
func TestIngressTitleNeverCostsTheSearchIndicator(t *testing.T) {
	const base = "FILTER NODES..."
	for _, w := range []int{120, 100, 80, 72, 40, 20} {
		for _, n := range []int{0, 5, 23} {
			got := ingressTitle(base, n, w)
			if !strings.HasPrefix(got, base) {
				t.Fatalf("w=%d n=%d: lost the base title: %q", w, n, got)
			}
			if budget := w - 7; len(got) > budget && len(base) <= budget {
				t.Fatalf("w=%d n=%d: %q (%d) overflows the %d-col budget, "+
					"so Pane will drop the title entirely", w, n, got, len(got), budget)
			}
		}
	}
}

// End to end: the count is declared at a width where it fits, and the search
// indicator survives at one where it does not.
func TestNarrowTerminalKeepsTheNodesTitle(t *testing.T) {
	for _, w := range []int{120, 80} {
		m := newModelWithPeers(t, w, 30, ingressPeers(23)...)
		if v := m.View(); !strings.Contains(v, "FILTER NODES...") {
			t.Fatalf("w=%d: the NODES title vanished", w)
		}
	}
}
