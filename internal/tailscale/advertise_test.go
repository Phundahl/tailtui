package tailscale

import (
	"strings"
	"testing"
)

// Phase 23: the routing command assembly always expresses the full intended
// state — both --advertise-exit-node and --advertise-routes.
func TestAdvertiseCommandString(t *testing.T) {
	cases := []struct {
		name  string
		exit  bool
		route []string
		want  string
	}{
		{
			name:  "exit on with routes",
			exit:  true,
			route: []string{"192.168.1.0/24", "10.0.0.0/16"},
			want:  "tailscale set --advertise-exit-node=true --advertise-routes=192.168.1.0/24,10.0.0.0/16",
		},
		{
			name:  "exit off, no routes clears with empty flag",
			exit:  false,
			route: nil,
			want:  "tailscale set --advertise-exit-node=false --advertise-routes=",
		},
		{
			name:  "single route",
			exit:  false,
			route: []string{"192.168.1.0/24"},
			want:  "tailscale set --advertise-exit-node=false --advertise-routes=192.168.1.0/24",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AdvertiseCommandString(c.exit, c.route); got != c.want {
				t.Fatalf("AdvertiseCommandString = %q, want %q", got, c.want)
			}
			// The exec arg vector must match the displayed command exactly.
			if got := "tailscale " + strings.Join(AdvertiseArgs(c.exit, c.route), " "); got != c.want {
				t.Fatalf("AdvertiseArgs joined = %q, want %q", got, c.want)
			}
		})
	}
}
