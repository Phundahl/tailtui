package tui

import "github.com/Phundahl/tailtui/internal/types"

// settingDef describes one toggle in the Advanced Settings modal: its display
// label, help text, the `tailscale set --<flag>` it drives, and accessors that
// read/write the matching field on types.Prefs. Keeping the get/set as closures
// lets the modal stay data-driven (navigate, toggle, describe) without a switch
// per setting.
type settingDef struct {
	label string
	desc  string
	flag  string // the `--<flag>` passed to `tailscale set` (without value)
	get   func(types.Prefs) bool
	set   func(types.Prefs, bool) types.Prefs
}

// settingDefs is the ordered list of the five toggles shown in the modal. Order
// here is the on-screen order and the index space for settingCursor.
var settingDefs = []settingDef{
	{
		label: "Accept Subnet Routes",
		desc:  "Use subnet routes advertised by other nodes, letting this machine reach the LAN subnets they expose on the tailnet.",
		flag:  "accept-routes",
		get:   func(p types.Prefs) bool { return p.AcceptRoutes },
		set:   func(p types.Prefs, v bool) types.Prefs { p.AcceptRoutes = v; return p },
	},
	{
		label: "Allow LAN Access",
		desc:  "While using an exit node, still allow direct access to devices on your local physical LAN instead of routing that traffic through the exit node.",
		flag:  "exit-node-allow-lan-access",
		get:   func(p types.Prefs) bool { return p.ExitNodeAllowLANAccess },
		set:   func(p types.Prefs, v bool) types.Prefs { p.ExitNodeAllowLANAccess = v; return p },
	},
	{
		label: "Run Tailscale SSH",
		desc:  "Run the built-in Tailscale SSH server on this node, letting tailnet peers connect using Tailscale identity and ACLs.",
		flag:  "ssh",
		get:   func(p types.Prefs) bool { return p.RunSSH },
		set:   func(p types.Prefs, v bool) types.Prefs { p.RunSSH = v; return p },
	},
	{
		label: "Accept MagicDNS",
		desc:  "Use the tailnet's MagicDNS configuration, resolving peer hostnames and applying the DNS settings pushed from the admin console.",
		flag:  "accept-dns",
		get:   func(p types.Prefs) bool { return p.AcceptDNS },
		set:   func(p types.Prefs, v bool) types.Prefs { p.AcceptDNS = v; return p },
	},
	{
		label: "Shields Up",
		desc:  "Block all incoming connections from other tailnet nodes. Outgoing connections still work, but this node becomes unreachable to peers.",
		flag:  "shields-up",
		get:   func(p types.Prefs) bool { return p.ShieldsUp },
		set:   func(p types.Prefs, v bool) types.Prefs { p.ShieldsUp = v; return p },
	},
}

// openSettings transitions to the Advanced Settings modal (opened with uppercase
// [S]). The checkboxes are rendered directly from m.prefs each frame, so opening
// just resets the cursor; the caller batches fetchPrefsCmd() to refresh the live
// state.
func (m Model) openSettings() Model {
	m.state = stateSettings
	m.settingCursor = 0
	return m
}
