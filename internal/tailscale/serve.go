package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/Phundahl/tailtui/internal/types"
)

// Serve/Funnel adapter. The wire structs below unmarshal a subset of the
// daemon's serve config; only types.ServePort crosses the package boundary.
//
// Everything here was verified against live configs rather than inferred,
// because two properties of the format are easy to guess wrong:
//
//  1. There are TWO top-level shapes. A background config is flat; a
//     foreground `tailscale serve` session nests under "Foreground", keyed by
//     session id. Handling only the flat shape makes an active foreground
//     share render as "nothing is shared" — the worst failure this feature
//     could have, since its entire job is showing what is exposed.
//
//  2. AllowFunnel is keyed by host:port, NOT by path. Funnel is a property of
//     the listening port, so enabling it exposes every path mounted there.

// serveWire is the daemon's serve config. Web/TCP/AllowFunnel are the flat
// (background) form; Foreground holds per-session configs of the same shape.
type serveWire struct {
	TCP         map[string]tcpWire  `json:"TCP"`
	Web         map[string]webWire  `json:"Web"`
	AllowFunnel map[string]bool     `json:"AllowFunnel"`
	Foreground  map[string]*fgpWire `json:"Foreground"`
}

// fgpWire is a foreground session: the same config minus its own Foreground.
type fgpWire struct {
	TCP         map[string]tcpWire `json:"TCP"`
	Web         map[string]webWire `json:"Web"`
	AllowFunnel map[string]bool    `json:"AllowFunnel"`
}

type tcpWire struct {
	HTTPS bool `json:"HTTPS"`
}

type webWire struct {
	Handlers map[string]handlerWire `json:"Handlers"`
}

// handlerWire holds exactly one of the three target kinds.
type handlerWire struct {
	Proxy string `json:"Proxy"`
	Path  string `json:"Path"`
	Text  string `json:"Text"`
}

// kind reports which of the three target kinds this handler carries. Proxy is
// the fallback so an unrecognised future kind degrades to a visible row rather
// than vanishing.
func (h handlerWire) kind() (types.ServeKind, string) {
	switch {
	case h.Path != "":
		return types.ServeFile, h.Path
	case h.Text != "":
		return types.ServeText, h.Text
	default:
		return types.ServeProxy, h.Proxy
	}
}

// ServeStatus reads the live Serve/Funnel configuration via
// `tailscale serve status --json`, mapping it onto the CLI-agnostic
// types.ServePort. An unconfigured node returns an empty slice, not an error.
func ServeStatus(ctx context.Context) ([]types.ServePort, error) {
	if mockEnabled {
		return mockServeSnapshot()
	}
	out, err := exec.CommandContext(ctx, "tailscale", "serve", "status", "--json").Output()
	if err != nil {
		return nil, runError(err)
	}
	var w serveWire
	if err := json.Unmarshal(out, &w); err != nil {
		return nil, fmt.Errorf("parsing serve status: %w", err)
	}
	return mapServe(w), nil
}

// mapServe folds the flat config and every foreground session into one list of
// ports, sorted ascending so the display order is stable across polls.
func mapServe(w serveWire) []types.ServePort {
	byPort := map[int]*types.ServePort{}

	collect := func(tcp map[string]tcpWire, web map[string]webWire, funnel map[string]bool) {
		for hostPort, wv := range web {
			port := portOf(hostPort)
			if port == 0 {
				continue
			}
			p := byPort[port]
			if p == nil {
				p = &types.ServePort{Port: port}
				byPort[port] = p
			}
			if funnel[hostPort] {
				p.Funnel = true
			}
			if t, ok := tcp[strconv.Itoa(port)]; ok && t.HTTPS {
				p.HTTPS = true
			}
			for path, h := range wv.Handlers {
				kind, target := h.kind()
				p.Paths = append(p.Paths, types.ServePath{Path: path, Kind: kind, Target: target})
			}
		}
	}

	collect(w.TCP, w.Web, w.AllowFunnel)
	for _, fg := range w.Foreground {
		if fg != nil {
			collect(fg.TCP, fg.Web, fg.AllowFunnel)
		}
	}

	ports := make([]types.ServePort, 0, len(byPort))
	for _, p := range byPort {
		sort.Slice(p.Paths, func(i, j int) bool { return p.Paths[i].Path < p.Paths[j].Path })
		ports = append(ports, *p)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	return ports
}

// portOf extracts the listening port from a "host:port" config key.
func portOf(hostPort string) int {
	i := strings.LastIndex(hostPort, ":")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(hostPort[i+1:])
	if err != nil {
		return 0
	}
	return n
}

// NormalizeServePath strips a trailing slash for COMPARISON only. The daemon
// stores a directory as "/docs/" and a file as "/motd", so a user typing
// "/docs" is naming the same mount point — and an add is an upsert, silently
// replacing whatever was there. Never use this for building a URL: "/docs"
// and "/docs/" are not interchangeable for a directory listing.
func NormalizeServePath(p string) string {
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		return strings.TrimSuffix(p, "/")
	}
	return p
}
