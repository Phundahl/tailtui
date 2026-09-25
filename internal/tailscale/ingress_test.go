package tailscale

import "testing"

// The classification is structural, not a hostname match: it must survive
// Tailscale renaming the fleet, and must not swallow a real node someone
// tagged themselves.
func TestIsIngressNode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tags    []string
		dnsName string
		want    bool
	}{
		{"the real fleet shape", []string{"tag:ingress"}, "", true},
		{"trailing dot is still empty", []string{"tag:ingress"}, ".", true},
		{"alongside other tags", []string{"tag:prod", "tag:ingress"}, "", true},

		{"a user's own tagged node", []string{"tag:ingress"}, "k8s-ingress.example-tailnet.ts.net.", false},
		{"tagged but not ingress", []string{"tag:prod"}, "", false},
		{"no tags at all", nil, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isIngressNode(tc.tags, tc.dnsName); got != tc.want {
				t.Fatalf("isIngressNode(%v, %q) = %v, want %v", tc.tags, tc.dnsName, got, tc.want)
			}
		})
	}
}
