// Mock mode: when the process is started with TAILTUI_MOCK=1, every adapter
// function short-circuits to in-memory anonymized fixtures instead of shelling
// out to the real `tailscale` CLI. The TUI layer also branches on
// MockEnabled() to skip its tea.ExecProcess flows (login, switch, operator,
// connect), so a demo / VHS recording is fully interactive without ever
// touching the host's Tailnet state. Live behavior is unchanged when the env
// var is unset — the var is evaluated once at package init.
package tailscale

import (
	"os"
	"sync"

	"github.com/Phundahl/tailtui/internal/types"
)

// mockEnabled is captured once at package load from $TAILTUI_MOCK=1.
var mockEnabled = os.Getenv("TAILTUI_MOCK") == "1"

// MockEnabled reports whether the in-memory mock state is active.
func MockEnabled() bool { return mockEnabled }

// mockState holds the writable in-memory tailnet used during a demo. The TUI
// mutates it from multiple goroutines (Cmd callbacks), so every read or write
// goes through mockMu.
var (
	mockMu        sync.Mutex
	mockState     = newMockState()
	mockPingTicks int
)

type mockData struct {
	local      types.LocalStatus
	peers      []types.Peer
	accounts   []types.Account
	prefs      types.Prefs
	connected  bool
	activeExit string // hostname of active exit node, "" when none
}

// newMockState seeds a small but realistic fictional tailnet: a mix of OS
// types, node roles (exit / subnet router / regular), connectivity (direct /
// relay / offline), and tags — enough to show off every list-rendering branch
// without leaking any real topology.
func newMockState() *mockData {
	peers := []types.Peer{
		{
			ID: "n-exit", Hostname: "exit-node-se", DNSName: "exit-node-se.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.10",
			Conn: types.ConnDirect, Tags: []string{"tag:exit"},
			LastSeen: "Connected", Online: true,
			NodeType: types.NodeExitNode, OffersExitNode: true,
		},
		{
			ID: "n-subnet1", Hostname: "dc-subnet-router", DNSName: "dc-subnet-router.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.11",
			Conn: types.ConnDirect, Tags: []string{"tag:router", "tag:prod"},
			LastSeen:         "Connected",
			Online:           true,
			NodeType:         types.NodeSubnetRouter,
			AdvertisedRoutes: []string{"192.168.10.0/24", "192.168.20.0/24"},
		},
		{
			ID: "n-subnet2", Hostname: "home-nas", DNSName: "home-nas.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.12",
			Conn: types.ConnRelay, Relay: "fra", Tags: []string{"tag:home"},
			LastSeen:         "Connected",
			Online:           true,
			NodeType:         types.NodeSubnetRouter,
			AdvertisedRoutes: []string{"10.0.0.0/24"},
		},
		{
			ID: "n-web", Hostname: "srv-web-01", DNSName: "srv-web-01.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.20",
			Conn: types.ConnDirect, Tags: []string{"tag:server", "tag:prod"},
			LastSeen: "Connected", Online: true, NodeType: types.NodeRegular,
		},
		{
			ID: "n-db", Hostname: "db-cluster-prod", DNSName: "db-cluster-prod.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.21",
			Conn: types.ConnDirect, Tags: []string{"tag:database", "tag:prod"},
			LastSeen: "Connected", Online: true, NodeType: types.NodeRegular,
		},
		{
			ID: "n-mac", Hostname: "dev-macbook", DNSName: "dev-macbook.tailtui.dev",
			OS: types.OSMacOS, TailscaleIP: "100.64.0.30",
			Conn: types.ConnDirect, Tags: []string{"tag:dev"},
			LastSeen: "Connected", Online: true, NodeType: types.NodeRegular,
		},
		{
			ID: "n-laptop", Hostname: "field-laptop", DNSName: "field-laptop.tailtui.dev",
			OS: types.OSLinux, TailscaleIP: "100.64.0.31",
			Conn: types.ConnOffline, Tags: []string{"tag:laptop"},
			LastSeen: "2h ago", Online: false, NodeType: types.NodeRegular,
		},
	}
	return &mockData{
		local: types.LocalStatus{
			User:        "demo@tailtui.dev",
			Hostname:    "tailtui-demo",
			LocalIP:     "192.168.1.42",
			TailscaleIP: "100.64.0.1",
			Conn:        types.ConnDirect,
			Relay:       "fra",
		},
		peers: peers,
		accounts: []types.Account{
			{ID: "p-demo", Email: "demo@tailtui.dev", Active: true},
			{ID: "p-personal", Email: "personal@tailtui.dev", Active: false},
		},
		prefs: types.Prefs{
			AcceptRoutes:    true,
			RunSSH:          true,
			AcceptDNS:       true,
			AdvertiseRoutes: []string{"192.168.1.0/24", "10.0.0.0/16"},
		},
		connected: true,
	}
}

// mockStatusSnapshot returns a fresh snapshot of the mock local + peers with
// transient flags (active exit node, local connectivity) applied. Returned
// peers are values, so callers can't mutate the underlying state by accident.
func mockStatusSnapshot() (types.LocalStatus, []types.Peer, error) {
	mockMu.Lock()
	defer mockMu.Unlock()
	local := mockState.local
	if !mockState.connected {
		local.Conn = types.ConnOffline
	}
	peers := make([]types.Peer, len(mockState.peers))
	for i, p := range mockState.peers {
		p.IsActiveExitNode = p.OffersExitNode && p.Hostname == mockState.activeExit
		peers[i] = p
	}
	return local, peers, nil
}

func mockAccountsSnapshot() ([]types.Account, error) {
	mockMu.Lock()
	defer mockMu.Unlock()
	out := make([]types.Account, len(mockState.accounts))
	copy(out, mockState.accounts)
	return out, nil
}

func mockPrefsSnapshot() (types.Prefs, error) {
	mockMu.Lock()
	defer mockMu.Unlock()
	p := mockState.prefs
	p.AdvertiseRoutes = append([]string(nil), mockState.prefs.AdvertiseRoutes...)
	return p, nil
}

// mockBaseline returns a per-IP baseline latency derived deterministically
// from the last byte of the address, so different peers have different
// "typical" RTTs but the same peer always lands near the same value.
func mockBaseline(ip string) int {
	if ip == "" {
		return 16
	}
	return 8 + int(ip[len(ip)-1]%16)
}

// mockWaveOffset is a 12-step "cosine-ish" jitter applied on top of the
// per-IP baseline so the live graph looks alive instead of flatlined.
var mockWaveOffset = [12]int{0, 2, 4, 5, 4, 2, 0, -2, -4, -5, -4, -2}

// mockPingValue returns a varying ms reading for ip. Tick-based so the
// observed series traces a smooth wave across the graph.
func mockPingValue(ip string) (int, error) {
	mockMu.Lock()
	mockPingTicks++
	tick := mockPingTicks
	mockMu.Unlock()
	return mockBaseline(ip) + mockWaveOffset[tick%len(mockWaveOffset)], nil
}

// MockLatencySeed returns a pre-populated per-IP history so the LATENCY
// HISTORY pane isn't empty during the first few seconds of a recording. The
// TUI installs this into Model.latency at startup; live ping ticks append on
// top of it and the FIFO cap rolls the seed out naturally over time.
func MockLatencySeed() map[string][]int {
	mockMu.Lock()
	defer mockMu.Unlock()
	out := make(map[string][]int, len(mockState.peers))
	for _, p := range mockState.peers {
		if !p.Online || p.TailscaleIP == "" {
			continue
		}
		hist := make([]int, 0, 20)
		base := mockBaseline(p.TailscaleIP)
		for i := 0; i < 20; i++ {
			hist = append(hist, base+mockWaveOffset[i%len(mockWaveOffset)])
		}
		out[p.TailscaleIP] = hist
	}
	return out
}

// MockSetExitNode toggles the active exit node by Tailscale IP. An empty ip
// clears the selection; an IP that doesn't belong to an exit-capable peer is
// a no-op (mirrors live behavior — `tailscale set --exit-node=` rejects it).
func MockSetExitNode(ip string) error {
	mockMu.Lock()
	defer mockMu.Unlock()
	if ip == "" {
		mockState.activeExit = ""
		return nil
	}
	for _, p := range mockState.peers {
		if p.TailscaleIP == ip && p.OffersExitNode {
			mockState.activeExit = p.Hostname
			return nil
		}
	}
	return nil
}

// MockSetPref flips one boolean preference in the in-memory mock state. The
// flag names mirror the `tailscale set --<flag>` switches.
func MockSetPref(flag string, val bool) error {
	mockMu.Lock()
	defer mockMu.Unlock()
	switch flag {
	case "accept-routes":
		mockState.prefs.AcceptRoutes = val
	case "exit-node-allow-lan-access":
		mockState.prefs.ExitNodeAllowLANAccess = val
	case "ssh":
		mockState.prefs.RunSSH = val
	case "accept-dns":
		mockState.prefs.AcceptDNS = val
	case "shields-up":
		mockState.prefs.ShieldsUp = val
	}
	return nil
}

// MockSetRouting applies the routing modal's working copy to the mock prefs.
func MockSetRouting(exitNode bool, routes []string) error {
	mockMu.Lock()
	defer mockMu.Unlock()
	mockState.prefs.AdvertiseExitNode = exitNode
	mockState.prefs.AdvertiseRoutes = append([]string(nil), routes...)
	return nil
}

// MockSetConnected toggles the local-node "connected" flag (drives [c]).
func MockSetConnected(up bool) {
	mockMu.Lock()
	defer mockMu.Unlock()
	mockState.connected = up
}

// MockAddAccount appends a new inactive profile to the mock account list, so
// the post-action refresh shows the modal grow by one row.
func MockAddAccount() {
	mockMu.Lock()
	defer mockMu.Unlock()
	mockState.accounts = append(mockState.accounts, types.Account{
		ID:     "p-new",
		Email:  "new-user@tailtui.dev",
		Active: false,
	})
}

// MockSwitchAccount sets the named profile as the sole active session.
func MockSwitchAccount(id string) {
	mockMu.Lock()
	defer mockMu.Unlock()
	for i := range mockState.accounts {
		mockState.accounts[i].Active = mockState.accounts[i].ID == id
	}
}

// MockRemoveAccount drops the profile with the given id (skipping the active
// one — same guard the modal applies before dispatch).
func MockRemoveAccount(id string) {
	mockMu.Lock()
	defer mockMu.Unlock()
	out := make([]types.Account, 0, len(mockState.accounts))
	for _, a := range mockState.accounts {
		if a.ID == id && !a.Active {
			continue
		}
		out = append(out, a)
	}
	mockState.accounts = out
}

// MockLogout clears the active flag on every mock account (mirrors how
// `tailscale logout` leaves no active session until a switch / login).
func MockLogout() {
	mockMu.Lock()
	defer mockMu.Unlock()
	for i := range mockState.accounts {
		mockState.accounts[i].Active = false
	}
}
