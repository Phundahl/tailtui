package tui

import (
	"strings"
	"testing"
)

// restoreVersion snapshots appVersion and puts it back after the test, so the
// package-level override can't leak into the other suites.
func restoreVersion(t *testing.T) {
	t.Helper()
	orig := appVersion
	t.Cleanup(func() { appVersion = orig })
}

// goreleaser's {{.Version}} expands to a bare "1.2.0" (no v prefix), so
// SetVersion normalizes it; an unstamped build passes "" and must keep the
// dev-build literal rather than blanking the footer.
func TestSetVersion(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"bare goreleaser version", "9.9.9", "v9.9.9"},
		{"already prefixed", "v9.9.9", "v9.9.9"},
		{"prerelease", "9.9.9-rc1", "v9.9.9-rc1"},
		{"unstamped build keeps the literal", "", "v0.0.0-devbuild"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			restoreVersion(t)
			appVersion = "v0.0.0-devbuild" // stands in for the dev-build literal
			SetVersion(c.in)
			if appVersion != c.want {
				t.Errorf("SetVersion(%q) → %q, want %q", c.in, appVersion, c.want)
			}
		})
	}
}

// The stamped version has to actually reach the footer — the whole point of
// the ldflag, which previously targeted a main.version var that didn't exist.
func TestVersionRendersInFooter(t *testing.T) {
	restoreVersion(t)
	SetVersion("9.9.9")

	m := newReadyModel(t, 120, 40)
	view := m.View()
	if !strings.Contains(view, "v9.9.9") {
		t.Error("stamped version does not appear in the rendered footer")
	}
	assertFlush(t, view, 120, 40)
}
