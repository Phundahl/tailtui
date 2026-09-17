package tailscale

import (
	"strings"
	"testing"
)

// Phase 34: the SSH launcher previews the exact command it will run, so the
// preview string and the argv must be assembled from the same place. These are
// pure functions — no CLI, no mock short-circuit — mirroring advertise_test.go.
func TestSSHTarget(t *testing.T) {
	for _, tc := range []struct {
		name, user, host, want string
	}{
		{"user and host", "demo", "srv-web-01.tailtui.dev", "demo@srv-web-01.tailtui.dev"},
		{"empty user yields bare host", "", "srv-web-01.tailtui.dev", "srv-web-01.tailtui.dev"},
		{"whitespace user is empty", "   ", "srv-web-01.tailtui.dev", "srv-web-01.tailtui.dev"},
		{"trailing dot trimmed", "root", "field-laptop.example-tailnet.ts.net.", "root@field-laptop.example-tailnet.ts.net"},
		{"surrounding space trimmed", "  root  ", "  host.ts.net  ", "root@host.ts.net"},
		{"bare ip target", "", "100.64.0.20", "100.64.0.20"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SSHTarget(tc.user, tc.host); got != tc.want {
				t.Fatalf("SSHTarget(%q, %q) = %q, want %q", tc.user, tc.host, got, tc.want)
			}
		})
	}
}

func TestSSHArgs(t *testing.T) {
	got := SSHArgs("demo", "srv-web-01.tailtui.dev")
	want := []string{"ssh", "demo@srv-web-01.tailtui.dev"}
	if len(got) != len(want) {
		t.Fatalf("SSHArgs = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SSHArgs = %q, want %q", got, want)
		}
	}
}

func TestSSHCommandString(t *testing.T) {
	for _, tc := range []struct {
		user, host, want string
	}{
		{"demo", "srv-web-01.tailtui.dev", "tailscale ssh demo@srv-web-01.tailtui.dev"},
		{"", "srv-web-01.tailtui.dev", "tailscale ssh srv-web-01.tailtui.dev"},
		{"root", "field-laptop.example-tailnet.ts.net.", "tailscale ssh root@field-laptop.example-tailnet.ts.net"},
	} {
		if got := SSHCommandString(tc.user, tc.host); got != tc.want {
			t.Fatalf("SSHCommandString(%q, %q) = %q, want %q", tc.user, tc.host, got, tc.want)
		}
	}
}

// The preview must be byte-identical to what gets executed: the launcher shows
// SSHCommandString while sshLaunchCmd runs SSHArgs, so a drift between them
// would silently lie to the user about what is about to run.
func TestSSHPreviewMatchesArgv(t *testing.T) {
	for _, tc := range [][2]string{
		{"demo", "srv-web-01.tailtui.dev"},
		{"", "home-nas.tailtui.dev"},
		{"deploy", "db-cluster-prod.tailtui.dev."},
	} {
		argv := "tailscale " + strings.Join(SSHArgs(tc[0], tc[1]), " ")
		if preview := SSHCommandString(tc[0], tc[1]); preview != argv {
			t.Fatalf("preview %q != argv %q", preview, argv)
		}
	}
}
