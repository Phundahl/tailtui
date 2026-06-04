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
	User           string
	Hostname       string
	LocalIP        string
	TailscaleIP    string
	Conn           ConnType
	Relay          string
	ExitNode       string
	LatencyMs      int
	LatencyHistory []int
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
