package tailscale

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Phundahl/tailtui/internal/types"
)

// The fixtures below are the real shapes captured from a live daemon, with
// fictional hostnames substituted. Guessing this format wrong is easy and the
// failures are silent, so each shape gets its own test.

func parse(t *testing.T, raw string) []types.ServePort {
	t.Helper()
	var w serveWire
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return mapServe(w)
}

// The ordinary background config.
func TestServeFlatShape(t *testing.T) {
	ports := parse(t, `{
      "TCP": {"443": {"HTTPS": true}},
      "Web": {"tailtui-demo.example-tailnet.ts.net:443": {
        "Handlers": {"/": {"Proxy": "http://127.0.0.1:3000"}}}},
      "AllowFunnel": {"tailtui-demo.example-tailnet.ts.net:443": true}}`)

	if len(ports) != 1 {
		t.Fatalf("got %d ports, want 1", len(ports))
	}
	p := ports[0]
	if p.Port != 443 || !p.HTTPS || !p.Funnel {
		t.Fatalf("port = %+v, want 443 HTTPS funnelled", p)
	}
	if len(p.Paths) != 1 || p.Paths[0].Kind != types.ServeProxy {
		t.Fatalf("paths = %+v, want one proxy", p.Paths)
	}
}

// A FOREGROUND session nests under a session id. Handling only the flat shape
// would report "nothing is shared" while a share is live — the failure this
// whole test exists to prevent.
func TestServeForegroundShape(t *testing.T) {
	ports := parse(t, `{"Foreground": {"9eaca38eddee23d0": {
      "TCP": {"443": {"HTTPS": true}},
      "Web": {"tailtui-demo.example-tailnet.ts.net:443": {
        "Handlers": {"/": {"Proxy": "http://127.0.0.1:3000"}}}}}}}`)

	if len(ports) != 1 {
		t.Fatalf("foreground session not parsed: got %d ports, want 1", len(ports))
	}
	if ports[0].Port != 443 || len(ports[0].Paths) != 1 {
		t.Fatalf("port = %+v", ports[0])
	}
	if ports[0].Funnel {
		t.Fatalf("foreground session wrongly marked funnelled")
	}
}

// All three target kinds must map distinctly. A parser assuming Proxy renders
// a directory share blank — silently hiding the most dangerous kind.
func TestServeAllThreeKinds(t *testing.T) {
	ports := parse(t, `{
      "TCP": {"443": {"HTTPS": true}},
      "Web": {"tailtui-demo.example-tailnet.ts.net:443": {"Handlers": {
        "/":      {"Proxy": "http://127.0.0.1:3000"},
        "/docs/": {"Path":  "/srv/docs"},
        "/motd":  {"Text":  "back at 14:00"}}}}}`)

	if len(ports) != 1 || len(ports[0].Paths) != 3 {
		t.Fatalf("want 1 port with 3 paths, got %+v", ports)
	}
	want := map[string]struct {
		kind   types.ServeKind
		target string
	}{
		"/":      {types.ServeProxy, "http://127.0.0.1:3000"},
		"/docs/": {types.ServeFile, "/srv/docs"},
		"/motd":  {types.ServeText, "back at 14:00"},
	}
	for _, got := range ports[0].Paths {
		w, ok := want[got.Path]
		if !ok {
			t.Fatalf("unexpected path %q", got.Path)
		}
		if got.Kind != w.kind || got.Target != w.target {
			t.Fatalf("%q: kind=%v target=%q, want kind=%v target=%q",
				got.Path, got.Kind, got.Target, w.kind, w.target)
		}
	}
}

// Two ports at once, only one funnelled — funnel is per-port, so the flag must
// not bleed onto the other.
func TestServeFunnelIsPerPort(t *testing.T) {
	ports := parse(t, `{
      "TCP": {"443": {"HTTPS": true}, "8443": {"HTTPS": true}},
      "Web": {
        "h.example-tailnet.ts.net:443":  {"Handlers": {"/": {"Proxy": "http://127.0.0.1:3000"}}},
        "h.example-tailnet.ts.net:8443": {"Handlers": {"/": {"Text": "hi"}}}},
      "AllowFunnel": {"h.example-tailnet.ts.net:443": true}}`)

	if len(ports) != 2 {
		t.Fatalf("got %d ports, want 2", len(ports))
	}
	if ports[0].Port != 443 || ports[1].Port != 8443 {
		t.Fatalf("ports not sorted ascending: %+v", ports)
	}
	if !ports[0].Funnel {
		t.Fatalf(":443 should be funnelled")
	}
	if ports[1].Funnel {
		t.Fatalf(":8443 must NOT inherit the funnel flag from :443")
	}
}

// An unconfigured node returns {} — empty, not an error.
func TestServeEmptyConfig(t *testing.T) {
	if got := parse(t, `{}`); len(got) != 0 {
		t.Fatalf("empty config produced %+v, want none", got)
	}
}

func TestNormalizeServePath(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/docs/", "/docs"}, // directory form
		{"/docs", "/docs"},  // file form — already normal
		{"/", "/"},          // root must survive intact
		{"/a/b/", "/a/b"},
	} {
		if got := NormalizeServePath(tc.in); got != tc.want {
			t.Fatalf("NormalizeServePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- command assembly ---------------------------------------------------------
//
// Each expectation below was confirmed by running the command against a live
// daemon and reading back `serve status --json`.

func TestServeArgs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action ServeAction
		port   int
		path   string
		target string
		want   string
	}{
		{"add proxy at root", ServeAdd, 443, "/", "3000",
			"tailscale serve --bg --https=443 3000"},
		{"add proxy at a path", ServeAdd, 443, "/api", "http://127.0.0.1:8080",
			"tailscale serve --bg --https=443 --set-path=/api http://127.0.0.1:8080"},
		{"add a directory", ServeAdd, 8443, "/docs", "/srv/docs",
			"tailscale serve --bg --https=8443 --set-path=/docs /srv/docs"},
		{"trailing slash is stripped from the flag", ServeAdd, 443, "/docs/", "/srv/docs",
			"tailscale serve --bg --https=443 --set-path=/docs /srv/docs"},
		{"remove one path", ServeRemovePath, 443, "/api", "",
			"tailscale serve --https=443 --set-path=/api off"},
		{"remove a whole port", ServeRemovePort, 443, "/", "",
			"tailscale serve --https=443 off"},
		{"publish needs --yes", ServePublish, 443, "/api", "http://127.0.0.1:8080",
			"tailscale funnel --bg --yes --https=443 --set-path=/api http://127.0.0.1:8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ServeCommandString(tc.action, tc.port, tc.path, tc.target); got != tc.want {
				t.Fatalf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// THE regression that matters most. `tailscale funnel … off` removes the whole
// serve config, so implementing "back to tailnet-only" that way would silently
// destroy the share the user was trying to keep. Re-issuing `serve` is correct.
func TestServeUnpublishReissuesServe(t *testing.T) {
	got := ServeCommandString(ServeUnpublish, 443, "/api", "http://127.0.0.1:8080")
	want := "tailscale serve --bg --https=443 --set-path=/api http://127.0.0.1:8080"
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
	if strings.Contains(got, "funnel") {
		t.Fatalf("unpublish must NOT use funnel — `funnel ... off` deletes the share: %s", got)
	}
	if strings.HasSuffix(got, " off") {
		t.Fatalf("unpublish must re-serve the target, not turn the port off: %s", got)
	}
}

// The preview shown in the Command Room must be exactly what runs.
func TestServeCommandStringMatchesArgs(t *testing.T) {
	for _, a := range []ServeAction{ServeAdd, ServeRemovePath, ServeRemovePort, ServePublish, ServeUnpublish} {
		argv := "tailscale " + strings.Join(ServeArgs(a, 443, "/x", "3000"), " ")
		if got := ServeCommandString(a, 443, "/x", "3000"); got != argv {
			t.Fatalf("action %v: preview %q != argv %q", a, got, argv)
		}
	}
}

// Filesystem targets are privileged; ports, URLs and text are not.
func TestServeNeedsRoot(t *testing.T) {
	for _, tc := range []struct {
		target string
		want   bool
	}{
		{"/srv/docs", true},
		{"./relative", true},
		{"unix:/run/app.sock", true},
		{"3000", false},
		{"localhost:3000", false},
		{"http://127.0.0.1:3000", false},
		{"https+insecure://localhost:8443", false},
		{"text:hello", false},
	} {
		if got := ServeNeedsRoot(ServeAdd, tc.target); got != tc.want {
			t.Fatalf("ServeNeedsRoot(add, %q) = %v, want %v", tc.target, got, tc.want)
		}
	}
	// Removal never needs root — nothing is being read from disk.
	if ServeNeedsRoot(ServeRemovePath, "/srv/docs") {
		t.Fatalf("removing a path must not require root")
	}
}
