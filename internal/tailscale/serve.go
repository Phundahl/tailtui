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

// ApplyServe runs one serve/funnel command. The adapter owns this because it
// is the only package that knows the CLI's shape and error format.
func ApplyServe(ctx context.Context, args []string) error {
	if mockEnabled {
		return MockApplyServe(args)
	}
	out, err := exec.CommandContext(ctx, "tailscale", args...).CombinedOutput()
	return cliError("tailscale "+strings.Join(args, " "), out, err)
}

// --- command assembly --------------------------------------------------------
//
// Every operation below was verified against a live daemon. Two of them are
// counter-intuitive enough to be worth stating plainly:
//
//   - Turning funnel OFF is NOT `tailscale funnel … off`. That command removes
//     the entire serve config, so a user switching a port back to tailnet-only
//     would silently lose the share. The correct move is to RE-ISSUE the serve
//     command with the original target, which the daemon answers with
//     "Removing Funnel …" while leaving the share intact.
//   - Funnel needs --yes because the CLI has its own interactive prompt, and
//     tailtui has already asked via the Command Room.

// ServeAction is one pending edit, awaiting confirmation.
type ServeAction int

const (
	// ServeAdd mounts a target at a path (an upsert — an existing path is
	// replaced, not duplicated).
	ServeAdd ServeAction = iota
	// ServeRemovePath unmounts a single path.
	ServeRemovePath
	// ServeRemovePort unmounts every path on a listening port.
	ServeRemovePort
	// ServePublish makes a port reachable from the public internet.
	ServePublish
	// ServeUnpublish returns a port to tailnet-only WITHOUT dropping the share.
	ServeUnpublish
)

// ServeArgs builds the argv for an action. port is the LISTENING port, path the
// mount point, target the proxy URL / filesystem path / "text:…" literal.
func ServeArgs(action ServeAction, port int, path, target string) []string {
	var args []string
	switch action {
	case ServePublish:
		args = []string{"funnel", "--bg", "--yes"}
	case ServeAdd, ServeUnpublish:
		args = []string{"serve", "--bg"}
	default:
		args = []string{"serve"}
	}
	if port != 0 {
		args = append(args, fmt.Sprintf("--https=%d", port))
	}
	if path != "" && path != "/" {
		args = append(args, "--set-path="+strings.TrimSuffix(path, "/"))
	}
	switch action {
	case ServeRemovePath, ServeRemovePort:
		return append(args, "off")
	default:
		return append(args, target)
	}
}

// ServeCommandString renders the copy-pasteable command shown in the Command
// Room. Built FROM ServeArgs so the preview cannot drift from what executes.
func ServeCommandString(action ServeAction, port int, path, target string) string {
	return "tailscale " + strings.Join(ServeArgs(action, port, path, target), " ")
}

// ServeNeedsRoot reports whether an action requires elevation. The daemon
// refuses to serve a filesystem path or Unix socket unless the caller is root:
// "401 Unauthorized: must be root, or be an operator and able to run
// 'sudo tailscale' to serve a path or Unix socket". Ports and text do not.
func ServeNeedsRoot(action ServeAction, target string) bool {
	switch action {
	case ServeAdd, ServePublish, ServeUnpublish:
		return isPathTarget(target)
	default:
		return false
	}
}

// isPathTarget reports whether a target string names a filesystem path rather
// than a port, URL or text literal.
func isPathTarget(t string) bool {
	if t == "" || strings.HasPrefix(t, "text:") {
		return false
	}
	for _, scheme := range []string{"http://", "https://", "https+insecure://", "unix:"} {
		if strings.HasPrefix(t, scheme) {
			return strings.HasPrefix(t, "unix:")
		}
	}
	return strings.HasPrefix(t, "/") || strings.HasPrefix(t, "./") || strings.HasPrefix(t, "~")
}
