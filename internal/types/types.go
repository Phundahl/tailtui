// Package types holds the core domain models for tailTUI.
//
// These structs are deliberately decoupled from the `tailscale` CLI / JSON
// representation. In Phase 1 they are populated from hardcoded mock data; a
// later phase will add a mapping layer from `tailscale status --json`.
package types

import "strings"

// ConnType describes how a node is currently reachable.
type ConnType int

const (
	ConnOffline ConnType = iota
	ConnDirect
	ConnRelay
)

// String returns the short uppercase label used throughout the UI.
func (c ConnType) String() string {
	switch c {
	case ConnDirect:
		return "DIRECT"
	case ConnRelay:
		return "RELAY"
	default:
		return "OFFLINE"
	}
}

// NodeType distinguishes special-purpose nodes for list glyphs and badges.
type NodeType int

const (
	NodeRegular NodeType = iota
	NodeExitNode
	NodeSubnetRouter
)

// OS identifies the operating system of a node, used to pick a Nerd Font glyph.
type OS int

const (
	OSUnknown OS = iota
	OSLinux
	OSWindows
	OSMacOS
)

// Icon returns the Nerd Font glyph for the OS.
func (o OS) Icon() string {
	switch o {
	case OSLinux:
		return "\U000f033d" // 󰌽
	case OSWindows:
		return "\U000f017a" // 󰅺
	case OSMacOS:
		return "\U000f0035" // 󰀵
	default:
		return "" //
	}
}

// Name returns a human-readable OS name.
func (o OS) Name() string {
	switch o {
	case OSLinux:
		return "Linux"
	case OSWindows:
		return "Windows"
	case OSMacOS:
		return "macOS"
	default:
		return "Unknown"
	}
}

// Peer represents a single node in the tailnet other than the local machine.
type Peer struct {
	ID          string
	Hostname    string
	DNSName     string
	OS          OS
	TailscaleIP string
	Conn        ConnType
	Relay       string // DERP region, when Conn == ConnRelay
	Tags        []string
	LastSeen    string
	Online      bool
	NodeType    NodeType

	// Latency in milliseconds and a short rolling history for the sparkline.
	LatencyMs      int
	LatencyHistory []int

	// AdvertisedRoutes are the subnet CIDRs this node advertises (only
	// meaningful for subnet routers).
	AdvertisedRoutes []string

	// OffersExitNode is a capability flag: true if this node advertises itself
	// as an exit node and can therefore be selected with `x`.
	OffersExitNode bool

	// OffersSSH is a capability flag: true when this node runs the Tailscale
	// SSH server (the daemon reports sshHostKeys for it). It is ADVISORY ONLY
	// — it says the server is there, never that the tailnet ACL permits you to
	// use it, because per-peer capabilities are not exposed by the CLI.
	OffersSSH bool

	// IsActiveExitNode is true when the local node is currently routing all
	// traffic through this peer. At most one peer should have this set, and
	// only nodes with OffersExitNode == true may have it.
	IsActiveExitNode bool
}

// Icon returns the leading glyph for the peer in the list: a node-type glyph
// for exit/subnet nodes, otherwise the OS glyph.
func (p Peer) Icon() string {
	switch p.NodeType {
	case NodeExitNode:
		return "\U000f019f" // 󰖟
	case NodeSubnetRouter:
		return "\U000f0484" // 󰒄
	default:
		return p.OS.Icon()
	}
}

// FilterValue implements bubbles/list.Item. Hostname and tags are both
// searchable via the list's "/" fuzzy filter.
func (p Peer) FilterValue() string {
	if len(p.Tags) == 0 {
		return p.Hostname
	}
	return p.Hostname + " " + strings.Join(p.Tags, " ")
}

// Badge returns the bracketed type label shown in the list, or "" for regular.
func (p Peer) Badge() string {
	switch p.NodeType {
	case NodeExitNode:
		return "[EXIT]"
	case NodeSubnetRouter:
		return "[ROUT]"
	default:
		return ""
	}
}

// LocalStatus represents the local machine's Tailscale state.
type LocalStatus struct {
	User     string
	Hostname string
	// DNSName is the node's FULL MagicDNS name ("node.tailnet.ts.net").
	// Hostname is the short form and does not resolve on its own, so anything
	// building a browsable URL must use this.
	DNSName        string
	LocalIP        string
	TailscaleIP    string
	Conn           ConnType
	Relay          string
	ExitNode       string
	LatencyMs      int
	LatencyHistory []int
}

// Prefs holds the local-node Tailscale preferences exposed in the Advanced
// Settings modal. Each field maps to a single `tailscale set --<flag>` toggle
// and is read live from the daemon (see tailscale.GetPrefs). The struct is
// CLI-agnostic — the adapter maps the wire prefs (RouteAll/CorpDNS/…) onto it.
type Prefs struct {
	AcceptRoutes           bool // --accept-routes        (wire: RouteAll)
	ExitNodeAllowLANAccess bool // --exit-node-allow-lan-access
	RunSSH                 bool // --ssh                  (wire: RunSSH)
	AcceptDNS              bool // --accept-dns           (wire: CorpDNS)
	ShieldsUp              bool // --shields-up           (wire: ShieldsUp)

	// Routing management (Phase 21), read from the wire AdvertiseRoutes list:
	// AdvertiseExitNode is true when the node advertises the exit-node default
	// routes (0.0.0.0/0 + ::/0); AdvertiseRoutes holds the remaining subnet CIDRs
	// this node advertises (those defaults stripped out).
	AdvertiseExitNode bool
	AdvertiseRoutes   []string

	// OperatorUser is the OS user configured as the tailscaled operator (wire:
	// OperatorUser). It is not a `tailscale set --<flag>` toggle like the
	// fields above — it is read-only here, and exists so the UI can hide the
	// one-time `[O]` operator-setup hint once setup has actually happened.
	OperatorUser string
}

// Account is a Tailscale login the user can switch between (accounts modal).
type Account struct {
	ID     string // profile ID from `tailscale switch --list` (used to switch/remove)
	Email  string // account / display name
	Active bool   // the currently signed-in session
}

// LogEntry is a single line in the terminal log pane.
type LogEntry struct {
	Time    string
	Level   string // INFO, DEBUG, WARN, ERROR
	Message string
}

// ServeKind classifies what a served path points at. It exists so the view can
// switch on a type rather than string-matching the target — the same reason
// ConnType and NodeType exist. The three kinds are exactly what the daemon's
// serve config can hold, verified against live configs.
type ServeKind int

const (
	// ServeProxy forwards to a local server, e.g. "http://127.0.0.1:3000".
	ServeProxy ServeKind = iota
	// ServeFile serves a filesystem path — a directory or a single file.
	// Configuring one of these requires root (the daemon refuses otherwise),
	// which is Tailscale's own guard against casually publishing a directory.
	ServeFile
	// ServeText serves a literal string.
	ServeText
)

// Icon returns the leading glyph distinguishing the three kinds in the list.
func (k ServeKind) Icon() string {
	switch k {
	case ServeFile:
		return "▤"
	case ServeText:
		return "≡"
	default:
		return "⇢"
	}
}

// ServePath is one path mounted on a listening port, and what it resolves to.
type ServePath struct {
	// Path is the mount point as the daemon stores it. Note the daemon
	// normalises by TARGET SHAPE: a directory is stored with a trailing slash
	// ("/docs/"), a single file or text without ("/motd"). Render URLs from
	// this verbatim — the two forms are not interchangeable for a directory.
	Path   string
	Kind   ServeKind
	Target string // proxy URL, filesystem path, or the literal text
}

// ServePort is a listening port and everything mounted on it.
//
// Port is the port TAILSCALE listens on (443 by default), NOT the port of the
// local service — `tailscale serve 3000` produces Port 443 with a Target of
// "http://127.0.0.1:3000". Conflating the two is the easiest mistake here.
//
// Funnel is per-port, not per-path: it is keyed by host:port in the daemon's
// config, so enabling it exposes EVERY path in Paths to the public internet.
type ServePort struct {
	// Host is the full MagicDNS name the daemon serves on, taken verbatim from
	// its config key (e.g. "node.tailnet.ts.net"). Kept rather than rebuilt
	// from the local hostname, which is the SHORT name and yields a URL that
	// does not resolve.
	Host   string
	Port   int
	HTTPS  bool
	Funnel bool
	Paths  []ServePath
}
