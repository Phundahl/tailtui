package tui

import (
	"os"
	"os/exec"
	osuser "os/user"
	"strings"
	"testing"

	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

// realIdentity returns the strings that identify whoever is running this, and
// that must therefore never reach a frame of the demo recording.
func realIdentity(t *testing.T) []string {
	t.Helper()
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		// Too short to match meaningfully, and anything that collides with the
		// fixture vocabulary would be a false positive rather than a leak.
		if len(s) < 4 || strings.Contains("demo tailtui-demo example-tailnet", s) {
			return
		}
		out = append(out, s)
	}
	if u, err := osuser.Current(); err == nil {
		add(u.Username)
	}
	add(os.Getenv("USER"))
	if h, err := os.Hostname(); err == nil {
		add(h)
	}
	return out
}

// The demo GIF published in the README is recorded with TAILTUI_MOCK=1, so
// anything the UI renders under mock becomes public. The fixture's rule is
// that the whole rendered surface is fictional — identity included, since the
// SSH launcher pre-fills an operator name. This asserts that rule holds.
//
// mockEnabled is resolved at package init, so the assertions run in a
// subprocess with the variable set.
func TestMockModeRendersNoRealIdentity(t *testing.T) {
	if os.Getenv("TAILTUI_MOCK") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=TestMockModeRendersNoRealIdentity", "-test.v")
		cmd.Env = append(os.Environ(), "TAILTUI_MOCK=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mock-mode subprocess failed:\n%s", out)
		}
		return
	}

	if !tailscale.MockEnabled() {
		t.Fatal("subprocess did not enter mock mode")
	}
	needles := realIdentity(t)
	if len(needles) == 0 {
		t.Skip("no usable identity needles on this machine")
	}

	peer := types.Peer{
		Hostname: "srv-web-01", DNSName: "srv-web-01.example-tailnet.ts.net",
		OS: types.OSLinux, TailscaleIP: "100.64.0.10",
		Conn: types.ConnDirect, Online: true, OffersSSH: true,
	}
	// Every key the demo tape presses, plus the dashboard it rests on.
	for _, k := range []string{"", "s", "S", "R", "F", "l", "v", "?"} {
		m := newModelWithPeers(t, 120, 40, peer)
		if k != "" {
			m = mustModel(m.Update(key(k)))
		}
		view := m.View()
		for _, n := range needles {
			if strings.Contains(view, n) {
				t.Errorf("key %q renders this machine's identity under mock "+
					"(needle length %d); the fixture must be fictional throughout", k, len(n))
			}
		}
	}

	if got := tailscale.CurrentUser(); got != "demo" {
		t.Errorf("CurrentUser() = %q under mock, want the fictional user", got)
	}
}
