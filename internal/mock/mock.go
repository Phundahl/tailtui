// Package mock provides hardcoded sample data for Phase 1 development.
//
// Everything here will be replaced by a real `tailscale status` adapter in a
// later phase; keep the shapes identical to the domain models in types.
package mock

import "github.com/Phundahl/tailscaleTUI/internal/types"

// Local returns the mocked local node ("Phundahl").
func Local() types.LocalStatus {
	return types.LocalStatus{
		User:        "phundahl@tailnet.com",
		Hostname:    "workstation-7",
		LocalIP:     "192.168.1.15",
		TailscaleIP: "100.64.0.1",
		Conn:        types.ConnRelay,
		Relay:       "fra",
		// ExitNode is derived from whichever peer has IsActiveExitNode set
		// (see Model.activeExitNodeName), so it is intentionally left empty here.
		ExitNode:       "",
		LatencyMs:      24,
		LatencyHistory: []int{18, 20, 24, 30, 22, 19, 24, 28, 26, 22, 21, 24},
	}
}

// Peers returns the mocked tailnet peers: an exit node, a subnet router,
// a regular Linux peer (selected by default), and an offline Windows VM.
func Peers() []types.Peer {
	return []types.Peer{
		{
			ID:               "n-exit",
			Hostname:         "amsterdam-exit",
			DNSName:          "amsterdam-exit.tailnet.ts.net",
			OS:               types.OSLinux,
			TailscaleIP:      "100.64.0.10",
			Conn:             types.ConnDirect,
			Version:          "1.54.0",
			Tags:             []string{"tag:exit"},
			LastSeen:         "Just now",
			Online:           true,
			NodeType:         types.NodeExitNode,
			LatencyMs:        31,
			LatencyHistory:   []int{29, 31, 33, 30, 28, 31, 35, 31},
			OffersExitNode:   true, // the only exit-capable node in the mock
			IsActiveExitNode: true, // active by default so the styling is visible
		},
		{
			ID:             "n-subnet",
			Hostname:       "dc-subnet-01",
			DNSName:        "dc-subnet-01.tailnet.ts.net",
			OS:             types.OSLinux,
			TailscaleIP:    "100.64.0.20",
			Conn:           types.ConnDirect,
			Version:        "1.54.0",
			Tags:           []string{"tag:infra"},
			LastSeen:       "Just now",
			Online:         true,
			NodeType:       types.NodeSubnetRouter,
			LatencyMs:      12,
			LatencyHistory: []int{10, 12, 11, 13, 12, 12, 14, 12},
			AdvertisedRoutes: []string{
				"192.168.0.0/24",
				"192.168.1.0/24",
				"192.168.10.0/24",
				"10.0.0.0/8",
				"10.10.0.0/16",
				"10.20.30.0/24",
				"172.16.0.0/12",
				"172.16.5.0/24",
				"172.20.0.0/16",
				"100.96.0.0/16",
				"203.0.113.0/24",
				"198.51.100.0/24",
			},
		},
		{
			ID:             "n-dev",
			Hostname:       "peer-dev-box",
			DNSName:        "peer-dev-box.tailnet.ts.net",
			OS:             types.OSLinux,
			TailscaleIP:    "100.64.0.42",
			Conn:           types.ConnRelay,
			Relay:          "ams",
			Version:        "1.54.0",
			Tags:           []string{"tag:dev"},
			LastSeen:       "Just now",
			Online:         true,
			NodeType:       types.NodeRegular,
			LatencyMs:      22,
			LatencyHistory: []int{18, 22, 45, 21, 19, 23, 24, 22, 20, 25, 22, 18},
		},
		{
			ID:             "n-winvm",
			Hostname:       "peer-windows-vm",
			DNSName:        "peer-windows-vm.tailnet.ts.net",
			OS:             types.OSWindows,
			TailscaleIP:    "100.64.0.51",
			Conn:           types.ConnOffline,
			Version:        "1.52.1",
			Tags:           []string{"tag:dev"},
			LastSeen:       "3h ago",
			Online:         false,
			NodeType:       types.NodeRegular,
			LatencyMs:      0,
			LatencyHistory: nil,
		},
	}
}

// Logs returns the mocked terminal log lines.
func Logs() []types.LogEntry {
	return []types.LogEntry{
		{Time: "14:55:02", Level: "INFO", Message: "connection established to peer-dev-box via relay (ams)"},
		{Time: "14:55:02", Level: "DEBUG", Message: "mtu discovery: 1280 bytes"},
	}
}
