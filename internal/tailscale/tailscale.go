// Package tailscale is the live data adapter: it shells out to the real
// `tailscale` CLI and maps `tailscale status --json` into the CLI-agnostic
// domain models in internal/types.
//
// This is the live source of node data (Phase 6). The wire structs below are
// private to this package — only types.LocalStatus / types.Peer cross the
// boundary, so the rest of the app never sees a Tailscale-specific shape. Live
// latency is measured per selected peer via Ping (tailscale ping).
package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Phundahl/tailtui/internal/types"
)

// Status runs `tailscale status --json` and returns the mapped local node and
// peers. The context bounds the exec call so a hung daemon can't wedge the UI.
func Status(ctx context.Context) (types.LocalStatus, []types.Peer, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "status", "--json").Output()
	if err != nil {
		return types.LocalStatus{}, nil, runError(err)
	}
	var raw status
	if err := json.Unmarshal(out, &raw); err != nil {
		return types.LocalStatus{}, nil, fmt.Errorf("parsing tailscale status: %w", err)
	}
	return mapLocal(&raw), mapPeers(&raw), nil
}

// profile is the wire shape of one `tailscale switch --list --json` entry.
type profile struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Nickname string `json:"nickname"`
	Tailnet  string `json:"tailnet"`
	Selected bool   `json:"selected"`
}

// Accounts lists the locally-stored Tailscale profiles via `tailscale switch
// --list --json`, mapped to types.Account with the active profile sorted first.
func Accounts(ctx context.Context) ([]types.Account, error) {
	out, err := exec.CommandContext(ctx, "tailscale", "switch", "--list", "--json").Output()
	if err != nil {
		return nil, runError(err)
	}
	var profs []profile
	if err := json.Unmarshal(out, &profs); err != nil {
		return nil, fmt.Errorf("parsing accounts: %w", err)
	}
	accounts := make([]types.Account, 0, len(profs))
	for _, p := range profs {
		name := p.Account
		if name == "" {
			name = p.Nickname
		}
		accounts = append(accounts, types.Account{ID: p.ID, Email: name, Active: p.Selected})
	}
	// Active profile first (matches the mock layout), inactive order preserved.
	sort.SliceStable(accounts, func(i, j int) bool {
		return accounts[i].Active && !accounts[j].Active
	})
	return accounts, nil
}

// SwitchAccount switches the active profile to id (`tailscale switch <id>`).
func SwitchAccount(ctx context.Context, id string) error {
	out, err := exec.CommandContext(ctx, "tailscale", "switch", id).CombinedOutput()
	return cliError("tailscale switch", out, err)
}

// RemoveAccount removes a stored profile (`tailscale switch remove <id>`). It
// only forgets the local profile; it does not delete the account upstream.
func RemoveAccount(ctx context.Context, id string) error {
	out, err := exec.CommandContext(ctx, "tailscale", "switch", "remove", id).CombinedOutput()
	return cliError("tailscale switch remove", out, err)
}

// Logout logs the current session out (`tailscale logout`).
func Logout(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "tailscale", "logout").CombinedOutput()
	return cliError("tailscale logout", out, err)
}

// cliError wraps a failed CombinedOutput call, surfacing the CLI's own message.
func cliError(label string, out []byte, err error) error {
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return fmt.Errorf("%s: %s", label, msg)
	}
	return fmt.Errorf("%s: %w", label, err)
}

// pingLatency matches the "... in 137ms" tail of a `tailscale ping` pong line.
var pingLatency = regexp.MustCompile(`in ([\d.]+)\s*ms`)

// Ping runs a single `tailscale ping` to ip and returns the round-trip latency
// in milliseconds. A node that doesn't answer (offline / unreachable / the local
// node itself) yields an error rather than a zero sample, so callers can choose
// not to pollute the history with a fake value.
func Ping(ctx context.Context, ip string) (int, error) {
	// --c 1: send a single ping (default is up to 10). CombinedOutput because the
	// pong line is on stdout but failures report on stderr.
	out, err := exec.CommandContext(ctx, "tailscale", "ping", "--c", "1", ip).CombinedOutput()
	if m := pingLatency.FindSubmatch(out); m != nil {
		f, perr := strconv.ParseFloat(string(m[1]), 64)
		if perr == nil {
			return int(f + 0.5), nil // round to nearest ms
		}
	}
	if err != nil {
		return 0, fmt.Errorf("tailscale ping: %w", err)
	}
	return 0, fmt.Errorf("tailscale ping: no reply from %s", ip)
}

// SetExitNode runs `tailscale set --exit-node=<ip>`; an empty ip clears the exit
// node. The daemon applies the change, which the next status poll reflects.
func SetExitNode(ctx context.Context, ip string) error {
	out, err := exec.CommandContext(ctx, "tailscale", "set", "--exit-node="+ip).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("tailscale set: %s", msg)
		}
		return fmt.Errorf("tailscale set: %w", err)
	}
	return nil
}

// runError unwraps an *exec.ExitError to surface the CLI's stderr (e.g.
// "failed to connect to local tailscaled") instead of a bare "exit status 1".
func runError(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("tailscale: %s", strings.TrimSpace(string(ee.Stderr)))
	}
	return fmt.Errorf("tailscale status failed: %w", err)
}

// --- wire structs (private): a subset of `tailscale status --json` ----------

type status struct {
	Version      string
	BackendState string // "Running", "Stopped", "NeedsLogin", ...
	TailscaleIPs []string
	Self         *node
	Peer         map[string]*node // keyed by public key
	User         map[string]user  // keyed by stringified UserID
}

type node struct {
	ID             string
	HostName       string
	DNSName        string
	OS             string // "linux", "windows", "macOS", ...
	UserID         int64
	TailscaleIPs   []string
	Addrs          []string // physical endpoints (host:port); used for the LAN IP
	CurAddr        string   // current direct endpoint; empty when relayed
	Relay          string   // home DERP region code (e.g. "fra")
	Tags           []string
	PrimaryRoutes  []string // advertised subnet routes this node is primary for
	Online         bool
	ExitNode       bool // this node is the *active* exit node
	ExitNodeOption bool // this node *offers* exit-node service
	Active         bool
	LastSeen       time.Time
}

type user struct {
	ID          int64
	LoginName   string
	DisplayName string
}

func (s *status) login(id int64) string {
	if u, ok := s.User[strconv.FormatInt(id, 10)]; ok {
		return u.LoginName
	}
	return ""
}

// --- mapping: wire -> domain ------------------------------------------------

func mapLocal(s *status) types.LocalStatus {
	self := s.Self
	if self == nil {
		return types.LocalStatus{}
	}
	return types.LocalStatus{
		User:        s.login(self.UserID),
		Hostname:    self.HostName,
		LocalIP:     firstPrivateAddr(self.Addrs),
		TailscaleIP: first(self.TailscaleIPs),
		Conn:        localConn(s.BackendState, self),
		Relay:       self.Relay,
		// ExitNode is intentionally left empty: the active exit node is derived
		// from the peer list (Model.activeExitNodeName), keeping a single source
		// of truth. Latency is not set here — the local node has no meaningful
		// self-RTT; live latency is measured per selected peer via Ping.
	}
}

func mapPeers(s *status) []types.Peer {
	peers := make([]types.Peer, 0, len(s.Peer))
	for _, n := range s.Peer {
		routes := advertisedRoutes(n.PrimaryRoutes)

		// Latency is no longer synthesized here: live RTT is measured per
		// selected peer by the UI's Ping ticker and injected into the item.
		peers = append(peers, types.Peer{
			ID:               n.ID,
			Hostname:         n.HostName,
			DNSName:          strings.TrimSuffix(n.DNSName, "."),
			OS:               osType(n.OS),
			TailscaleIP:      first(n.TailscaleIPs),
			Conn:             peerConn(n),
			Relay:            n.Relay,
			Tags:             n.Tags,
			LastSeen:         humanizeLastSeen(n.Online, n.LastSeen),
			Online:           n.Online,
			NodeType:         nodeType(n.ExitNodeOption, routes),
			AdvertisedRoutes: routes,
			OffersExitNode:   n.ExitNodeOption,
			IsActiveExitNode: n.ExitNode,
		})
	}
	// Map iteration is random, so sort to a stable, intentional order: exit
	// nodes, then subnet routers, then online peers, then offline peers — each
	// bucket alphabetical by hostname. Offline nodes are pinned to the bottom.
	sort.Slice(peers, func(i, j int) bool {
		if a, b := sortBucket(peers[i]), sortBucket(peers[j]); a != b {
			return a < b
		}
		return strings.ToLower(peers[i].Hostname) < strings.ToLower(peers[j].Hostname)
	})
	return peers
}

// sortBucket assigns a peer to a priority bucket for list ordering: exit nodes
// (0) and subnet routers (1) float to the top regardless of reachability, then
// online (2) and offline (3) regular peers.
func sortBucket(p types.Peer) int {
	switch {
	case p.NodeType == types.NodeExitNode:
		return 0
	case p.NodeType == types.NodeSubnetRouter:
		return 1
	case p.Online:
		return 2
	default:
		return 3
	}
}

// localConn derives the dashboard "State" from the backend state and Self.
func localConn(state string, self *node) types.ConnType {
	if state != "Running" || !self.Online {
		return types.ConnOffline
	}
	if self.CurAddr != "" {
		return types.ConnDirect
	}
	return types.ConnRelay
}

// peerConn maps a peer's reachability: offline, direct (has a current endpoint),
// or relayed (online but routed through DERP).
func peerConn(n *node) types.ConnType {
	switch {
	case !n.Online:
		return types.ConnOffline
	case n.CurAddr != "":
		return types.ConnDirect
	default:
		return types.ConnRelay
	}
}

func nodeType(offersExit bool, routes []string) types.NodeType {
	switch {
	case offersExit:
		return types.NodeExitNode
	case len(routes) > 0:
		return types.NodeSubnetRouter
	default:
		return types.NodeRegular
	}
}

func osType(s string) types.OS {
	switch strings.ToLower(s) {
	case "linux":
		return types.OSLinux
	case "windows":
		return types.OSWindows
	case "macos", "darwin":
		return types.OSMacOS
	default:
		return types.OSUnknown
	}
}

// advertisedRoutes drops the exit-node default routes (0.0.0.0/0, ::/0), leaving
// only the real subnet CIDRs shown in the routes overlay.
func advertisedRoutes(routes []string) []string {
	var out []string
	for _, r := range routes {
		if r == "0.0.0.0/0" || r == "::/0" {
			continue
		}
		out = append(out, r)
	}
	return out
}

// humanizeLastSeen renders a peer's last-seen time as a short relative label.
func humanizeLastSeen(online bool, t time.Time) string {
	if online {
		return "Connected"
	}
	if t.IsZero() {
		return "—"
	}
	switch d := time.Since(t); {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// firstPrivateAddr returns the first RFC1918 host from a list of host:port
// endpoints — used as the local LAN IP (not exposed by status at top level).
func firstPrivateAddr(addrs []string) string {
	for _, a := range addrs {
		host, _, err := net.SplitHostPort(a)
		if err != nil {
			host = a
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsPrivate() {
			return host
		}
	}
	return ""
}

func first(s []string) string {
	if len(s) > 0 {
		return s[0]
	}
	return ""
}
