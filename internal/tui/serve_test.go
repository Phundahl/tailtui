package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

func servePorts() []types.ServePort {
	return []types.ServePort{
		{Port: 443, HTTPS: true, Funnel: true, Paths: []types.ServePath{
			{Path: "/", Kind: types.ServeProxy, Target: "http://127.0.0.1:3000"},
			{Path: "/api", Kind: types.ServeProxy, Target: "http://127.0.0.1:8080"},
		}},
		{Port: 8443, HTTPS: true, Paths: []types.ServePath{
			{Path: "/docs/", Kind: types.ServeFile, Target: "/srv/docs"},
			{Path: "/motd", Kind: types.ServeText, Target: "back at 14:00"},
		}},
	}
}

func openServeModal(t *testing.T, w, h int, ports []types.ServePort) Model {
	t.Helper()
	m := newReadyModel(t, w, h)
	m2, _ := m.Update(serveMsg{ports: ports})
	m = m2.(Model)
	m3, _ := m.Update(key("F"))
	m = m3.(Model)
	if m.state != stateServe {
		t.Fatalf("[F] did not open the serve modal (state=%v)", m.state)
	}
	return m
}

// --- entry point --------------------------------------------------------------

func TestServeHotkeyIsUppercase(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m2, _ := m.Update(key("f"))
	if got := m2.(Model).state; got != stateMain {
		t.Fatalf("lowercase f opened %v; it is reserved and must be inert", got)
	}
	m3, cmd := m.Update(key("F"))
	if m3.(Model).state != stateServe {
		t.Fatalf("uppercase F did not open the serve modal")
	}
	if cmd == nil {
		t.Fatalf("[F] should also refresh the live serve config")
	}
}

// --- rendering ----------------------------------------------------------------

func TestServeOverlayRendersFlush(t *testing.T) {
	const w, h = 120, 40
	m := openServeModal(t, w, h, servePorts())
	view := m.View()
	assertFlush(t, view, w, h)
	for _, want := range []string{
		"SERVE_AND_FUNNEL", ":443", ":8443", "LISTENING",
		"/api", "/docs/", "/motd", "NAVIGATE",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("serve modal missing %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, appName) {
		t.Fatalf("header not visible behind modal — overlay blanked the base view")
	}
}

func TestServeOverlayFitsMinimumTerminal(t *testing.T) {
	const w, h = 72, 24
	m := openServeModal(t, w, h, servePorts())
	view := m.View()
	assertFlush(t, view, w, h)
	if !strings.Contains(view, "SERVE_AND_FUNNEL") {
		t.Fatalf("modal title dropped at %dx%d", w, h)
	}
}

// A funnelled port must read PUBLIC and a plain one TAILNET. Getting this
// backwards would tell the user something private is public, or worse.
func TestServeScopeLabels(t *testing.T) {
	view := openServeModal(t, 120, 40, servePorts()).View()
	if !strings.Contains(view, "PUBLIC") {
		t.Fatalf("funnelled port not labelled PUBLIC:\n%s", view)
	}
	if !strings.Contains(view, "TAILNET") {
		t.Fatalf("non-funnelled port not labelled TAILNET:\n%s", view)
	}
}

// Each handler kind gets its own glyph. A parser or view that only understood
// proxies would render a directory share blank — hiding the riskiest kind.
func TestServeAllKindsRender(t *testing.T) {
	view := openServeModal(t, 120, 40, servePorts()).View()
	for kind, glyph := range map[string]string{
		"proxy": types.ServeProxy.Icon(),
		"file":  types.ServeFile.Icon(),
		"text":  types.ServeText.Icon(),
	} {
		if !strings.Contains(view, glyph) {
			t.Fatalf("%s glyph %q missing from the view:\n%s", kind, glyph, view)
		}
	}
}

// The empty state is the state MOST nodes are in, so it has to advertise how
// to leave it. An earlier version returned before rendering the keymap, which
// made the modal look read-only on any node sharing nothing.
func TestServeEmptyState(t *testing.T) {
	m := openServeModal(t, 120, 40, nil)
	view := m.View()
	assertFlush(t, view, 120, 40)
	if !strings.Contains(view, "No services are being shared") {
		t.Fatalf("empty state message missing:\n%s", view)
	}
	if !strings.Contains(view, "ADD") {
		t.Fatalf("empty state must advertise the add key, or the modal looks read-only:\n%s", view)
	}
}

// Pressing add on an empty config must actually render the editor. The early
// return meant input mode was entered but the field was never drawn.
func TestServeAddFromEmptyStateRendersInput(t *testing.T) {
	m := openServeModal(t, 120, 40, nil)
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	if !m.serveInputMode {
		t.Fatalf("[a] did not enter input mode on an empty config")
	}
	view := m.View()
	assertFlush(t, view, 120, 40)
	if !strings.Contains(view, "Share what?") {
		t.Fatalf("target editor not rendered on an empty config:\n%s", view)
	}
	if !strings.Contains(view, "CONFIRM") {
		t.Fatalf("input keymap missing on an empty config:\n%s", view)
	}
}

// And the typed target must reach the Command Room from the empty state too.
func TestServeAddFromEmptyStateReachesConfirm(t *testing.T) {
	m := openServeModal(t, 120, 40, nil)
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	m.serveInput.SetValue("3000")
	m3, _ := m.Update(key("enter"))
	m = m3.(Model)
	if m.state != stateServeConfirm {
		t.Fatalf("Enter did not reach the Command Room (state=%v)", m.state)
	}
	if m.servePending.port != 443 || m.servePending.target != "3000" {
		t.Fatalf("staged action wrong: %+v", m.servePending)
	}
}

// --- navigation ---------------------------------------------------------------

func TestServeCursorClamps(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	want := m.serveItemCount() // 2 ports + 4 paths
	if want != 6 {
		t.Fatalf("serveItemCount = %d, want 6", want)
	}
	for i := 0; i < 20; i++ {
		m2, _ := m.Update(key("j"))
		m = m2.(Model)
	}
	if m.serveCursor != want-1 {
		t.Fatalf("cursor ran past the end: %d, want %d", m.serveCursor, want-1)
	}
	for i := 0; i < 20; i++ {
		m2, _ := m.Update(key("k"))
		m = m2.(Model)
	}
	if m.serveCursor != 0 {
		t.Fatalf("cursor ran past the start: %d", m.serveCursor)
	}
}

// Editing keys stage an action and open the Command Room. Nothing is applied
// until Enter is pressed there — and no key leaks to the peer list behind.
func TestServeEditKeysOpenConfirm(t *testing.T) {
	for _, tc := range []struct {
		key    string
		cursor int
		want   tailscale.ServeAction
	}{
		{"d", 0, tailscale.ServeRemovePort},    // cursor on a port row
		{"d", 1, tailscale.ServeRemovePath},    // cursor on a path row
		{"space", 0, tailscale.ServeUnpublish}, // :443 is funnelled -> make private
	} {
		t.Run(tc.key, func(t *testing.T) {
			m := openServeModal(t, 120, 40, servePorts())
			m.serveCursor = tc.cursor
			m2, cmd := m.Update(key(tc.key))
			got := m2.(Model)
			if got.state != stateServeConfirm {
				t.Fatalf("%q did not open the Command Room (state=%v)", tc.key, got.state)
			}
			if got.servePending.action != tc.want {
				t.Fatalf("%q staged action %v, want %v", tc.key, got.servePending.action, tc.want)
			}
			if cmd != nil {
				t.Fatalf("%q applied something before confirmation", tc.key)
			}
		})
	}
}

// Scope belongs to the port. A path row has no funnel flag of its own, so
// Space there must do nothing rather than silently act on the parent port.
func TestServeSpaceOnlyActsOnPortRows(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	m.serveCursor = 1 // a path row
	m2, _ := m.Update(key("space"))
	if got := m2.(Model).state; got != stateServe {
		t.Fatalf("Space on a path row opened %v; scope is a port-level property", got)
	}
}

// THE regression that matters. `tailscale funnel ... off` deletes the whole
// share, so unpublishing must re-issue `serve` with the original target.
func TestServeUnpublishKeepsTheShare(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	m.serveCursor = 0 // :443, funnelled
	m2, _ := m.Update(key("space"))
	p := m2.(Model).servePending
	cmd := tailscale.ServeCommandString(p.action, p.port, p.path, p.target)
	if strings.Contains(cmd, "funnel") || strings.HasSuffix(cmd, " off") {
		t.Fatalf("unpublish would delete the share: %s", cmd)
	}
	if p.target == "" {
		t.Fatalf("unpublish needs the original target to re-serve, got empty")
	}
}

// Enter applies; Esc goes BACK to the list rather than closing the feature.
func TestServeConfirmApplyAndBack(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	m2, _ := m.Update(key("d"))
	m = m2.(Model)

	back, cmd := m.Update(key("esc"))
	if back.(Model).state != stateServe {
		t.Fatalf("Esc in the Command Room should return to the list")
	}
	if cmd != nil {
		t.Fatalf("Esc applied something")
	}

	applied, cmd := m.Update(key("enter"))
	if applied.(Model).state != stateMain {
		t.Fatalf("Enter should close to the dashboard before dispatch")
	}
	if cmd == nil {
		t.Fatalf("Enter did not dispatch the edit")
	}
}

// The add field owns every key, so a target containing "q" survives.
func TestServeAddInputOwnsKeys(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	m2, _ := m.Update(key("a"))
	m = m2.(Model)
	if !m.serveInputMode {
		t.Fatalf("[a] did not enter the target editor")
	}
	m3, _ := m.Update(key("q"))
	m = m3.(Model)
	if m.state != stateServe {
		t.Fatalf("q while typing closed the modal")
	}
	if m.serveInput.Value() != "q" {
		t.Fatalf("q did not reach the editor: %q", m.serveInput.Value())
	}
	m4, _ := m.Update(key("esc"))
	m = m4.(Model)
	if m.state != stateServe || m.serveInputMode {
		t.Fatalf("Esc should cancel the entry, not close the modal")
	}
}

// --- target resolution --------------------------------------------------------

// The safety column: a bare path is easy to skim past, "directory, N entries"
// is not. Missing targets must say so rather than look healthy.
func TestResolveTarget(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok := resolveTarget(dir); !ok || !strings.Contains(got, "3 entries") {
		t.Fatalf("directory resolved to %q (ok=%v), want an entry count", got, ok)
	}

	file := filepath.Join(dir, "a")
	if got, ok := resolveTarget(file); !ok || !strings.HasPrefix(got, "file,") {
		t.Fatalf("file resolved to %q (ok=%v), want a file size", got, ok)
	}

	got, ok := resolveTarget(filepath.Join(dir, "nope"))
	if ok || got != "missing" {
		t.Fatalf("absent path resolved to %q (ok=%v), want missing", got, ok)
	}
}

// URLs are built from the STORED key. "/docs" and "/docs/" are not
// interchangeable for a directory listing, so the key is used verbatim.
func TestServeURLUsesStoredPath(t *testing.T) {
	if got := serveURL("host", 443, "/docs/"); got != "https://host/docs/" {
		t.Fatalf("serveURL = %q", got)
	}
	if got := serveURL("host", 8443, "/motd"); got != "https://host:8443/motd" {
		t.Fatalf("non-443 port must appear in the URL, got %q", got)
	}
}

// --- layout -------------------------------------------------------------------

func TestServeRowKeepsPanesFlush(t *testing.T) {
	lay := computeLayout(120, 40)
	if lay.localH != lay.detailsH {
		t.Fatalf("detailsH (%d) != localH (%d): top borders won't align", lay.detailsH, lay.localH)
	}
	// 10 content rows now: 7 identity + Serve + Connect + the action row.
	if inner := lay.localH - 2; inner < 10 {
		t.Fatalf("LOCAL_NODE inner height %d can't fit 10 content rows", inner)
	}
}

func TestServeStateRowReflectsFunnel(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m2, _ := m.Update(serveMsg{ports: servePorts()})
	if !strings.Contains(m2.(Model).View(), "PUBLIC :443") {
		t.Fatalf("LOCAL_NODE must name the publicly-exposed port")
	}

	m3, _ := m.Update(serveMsg{ports: []types.ServePort{{Port: 443,
		Paths: []types.ServePath{{Path: "/"}}}}})
	v := m3.(Model).View()
	if !strings.Contains(v, "tailnet only") {
		t.Fatalf("non-funnelled config should read 'tailnet only'")
	}
	if strings.Contains(v, "PUBLIC") {
		t.Fatalf("nothing is funnelled, but the view claims PUBLIC")
	}
}

// --- help overlay -------------------------------------------------------------

func TestHelpListsServeAndDropsStalePaneRow(t *testing.T) {
	m := newReadyModel(t, 120, 40)
	m2, _ := m.Update(key("?"))
	view := m2.(Model).View()
	if !strings.Contains(view, "Serve & Funnel") {
		t.Fatalf("help overlay missing the [F] Serve entry")
	}
	// There is no pane switching: `h` pages the list and `l` opens Accounts.
	if strings.Contains(view, "Switch Pane") {
		t.Fatalf("help overlay still advertises pane switching, which does not exist")
	}
}

// --- target risk --------------------------------------------------------------

func TestAnalyzeTargetCleanDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"index.html", "style.css"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := analyzeTarget(dir)
	if r.Risky() {
		t.Fatalf("an ordinary web directory must not warn: %+v", r)
	}
	if r.Entries != 2 {
		t.Fatalf("Entries = %d, want 2", r.Entries)
	}
}

// The case the whole feature exists for: the danger is in the contents, and a
// name-based denylist would never catch a project directory holding a .env.
func TestAnalyzeTargetFindsSensitiveContents(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := analyzeTarget(dir)
	if !r.Risky() {
		t.Fatalf("directory containing .ssh and .env must warn: %+v", r)
	}
	if len(r.Sensitive) != 2 || r.Sensitive[0] != ".env" || r.Sensitive[1] != ".ssh" {
		t.Fatalf("Sensitive = %v, want [.env .ssh] sorted", r.Sensitive)
	}
	if r.SystemDir {
		t.Fatalf("a temp dir is not a system directory")
	}
}

// System directories match EXACTLY. Prefix matching would flag /var/www, which
// is the normal thing to serve — and a warning that fires on ordinary use is
// one people learn to ignore.
func TestAnalyzeTargetSystemDirsAreExactMatches(t *testing.T) {
	if !analyzeTarget("/etc").SystemDir {
		t.Fatalf("/etc must be flagged as a system directory")
	}
	if !analyzeTarget("/var").SystemDir {
		t.Fatalf("/var must be flagged")
	}
	if analyzeTarget("/var/www").SystemDir {
		t.Fatalf("/var/www must NOT be flagged — prefix matching regression")
	}
	if analyzeTarget("/usr/share/doc").SystemDir {
		t.Fatalf("/usr/share/doc must NOT be flagged — prefix matching regression")
	}
}

func TestAnalyzeTargetTrailingSlashAndMissing(t *testing.T) {
	if !analyzeTarget("/etc/").SystemDir {
		t.Fatalf("a trailing slash must not defeat the system-directory check")
	}
	r := analyzeTarget(filepath.Join(t.TempDir(), "does-not-exist"))
	if r.Risky() || r.Entries != 0 {
		t.Fatalf("a missing path must be inert, got %+v", r)
	}
}

// The user's own home directory is the canonical mistake this guards against.
func TestAnalyzeTargetFlagsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	if !analyzeTarget(home).SystemDir {
		t.Fatalf("the user's home directory must be flagged")
	}
}

// --- the confirmation's warnings ----------------------------------------------

// confirmFor stages an action and returns the rendered Command Room.
func confirmFor(t *testing.T, p servePendingAction) string {
	t.Helper()
	m := openServeModal(t, 120, 40, servePorts())
	m.servePending = p
	m.state = stateServeConfirm
	return m.View()
}

// A sensitive directory going public must show BOTH warnings: what is being
// exposed, and to whom. They are independent risks.
func TestConfirmShowsBothWarnings(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServePublish, port: 443, path: "/files",
		target: dir, kind: types.ServeFile,
		paths: []types.ServePath{
			{Path: "/", Kind: types.ServeProxy, Target: "http://127.0.0.1:3000"},
			{Path: "/api", Kind: types.ServeProxy, Target: "http://127.0.0.1:8080"},
		},
	})
	assertFlush(t, view, 120, 40)
	for _, want := range []string{
		"SENSITIVE FILES", ".ssh",
		"PUBLIC INTERNET", "all 2 path(s)",
		"/api", // the blast radius names every path on the port
		"ABOUT TO EXECUTE", "EXPOSE PUBLICLY",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
}

// An ordinary proxy going to the tailnet warrants no warning at all. A dialog
// that shouts every time is one people stop reading.
func TestConfirmQuietForOrdinaryAdd(t *testing.T) {
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServeAdd, port: 443, path: "/app",
		target: "3000", kind: types.ServeProxy,
	})
	for _, unwanted := range []string{"SENSITIVE FILES", "PUBLIC INTERNET"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("ordinary add should not warn about %q:\n%s", unwanted, view)
		}
	}
	if !strings.Contains(view, "ABOUT TO EXECUTE") {
		t.Fatalf("confirmation must still show the command")
	}
}

// Friction is asymmetric: going back to tailnet-only reduces exposure, so it
// gets a quiet confirmation.
func TestConfirmQuietForUnpublish(t *testing.T) {
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServeUnpublish, port: 443, path: "/",
		target: "3000", kind: types.ServeProxy,
		paths: []types.ServePath{{Path: "/", Kind: types.ServeProxy, Target: "3000"}},
	})
	if strings.Contains(view, "PUBLIC INTERNET") {
		t.Fatalf("unpublishing must not warn about public exposure:\n%s", view)
	}
	if strings.Contains(view, "EXPOSE PUBLICLY") {
		t.Fatalf("unpublish should not offer to expose:\n%s", view)
	}
}

// A clean directory is not flagged just for being a directory.
func TestConfirmNoWarningForCleanDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<p>hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServeAdd, port: 8443, path: "/docs",
		target: dir, kind: types.ServeFile,
	})
	if strings.Contains(view, "SENSITIVE FILES") {
		t.Fatalf("a clean web directory should not warn:\n%s", view)
	}
}

// Replacing an existing mount point is an upsert — say so, rather than letting
// a share disappear silently.
func TestConfirmWarnsOnUpsert(t *testing.T) {
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServeAdd, port: 443, path: "/api",
		target: "9000", kind: types.ServeProxy,
	})
	if !strings.Contains(view, "REPLACES") {
		t.Fatalf("adding over an existing path must warn:\n%s", view)
	}
}

// A path target needs root; the confirmation should say so before the prompt.
func TestConfirmMentionsSudoForPathTargets(t *testing.T) {
	view := confirmFor(t, servePendingAction{
		action: tailscale.ServeAdd, port: 443, path: "/docs",
		target: "/srv/docs", kind: types.ServeFile,
	})
	if !strings.Contains(view, "requires root") {
		t.Fatalf("path targets should warn about the sudo prompt:\n%s", view)
	}
}

func TestConfirmFitsMinimumTerminal(t *testing.T) {
	m := openServeModal(t, 72, 24, servePorts())
	m.servePending = servePendingAction{
		action: tailscale.ServePublish, port: 443, path: "/",
		target: "3000", kind: types.ServeProxy,
		paths: []types.ServePath{{Path: "/", Kind: types.ServeProxy, Target: "3000"}},
	}
	m.state = stateServeConfirm
	view := m.View()
	assertFlush(t, view, 72, 24)
	if !strings.Contains(view, "EXPOSE PUBLICLY") {
		t.Fatalf("keymap dropped at 72x24:\n%s", view)
	}
}

// --- URL and copy -------------------------------------------------------------

func hostedPorts() []types.ServePort {
	p := servePorts()
	for i := range p {
		p[i].Host = "tailtui-demo.example-tailnet.ts.net"
	}
	return p
}

// The URL must come from the daemon's own config key. Rebuilding it from the
// local node's Hostname yields the SHORT name and a link that does not resolve.
func TestServeURLUsesFullMagicDNSHost(t *testing.T) {
	m := openServeModal(t, 120, 40, hostedPorts())
	view := m.View()
	if !strings.Contains(view, "https://tailtui-demo.example-tailnet.ts.net/") {
		t.Fatalf("URL must use the full MagicDNS host from the config:\n%s", view)
	}
	// A short hostname alone would be a broken link.
	if strings.Contains(view, "https://tailtui-demo/") {
		t.Fatalf("URL was rebuilt from the short hostname and will not resolve")
	}
}

func TestServeURLFollowsTheCursor(t *testing.T) {
	m := openServeModal(t, 120, 40, hostedPorts())
	for i, want := range []string{
		"https://tailtui-demo.example-tailnet.ts.net/",      // :443 port row
		"https://tailtui-demo.example-tailnet.ts.net/",      // /
		"https://tailtui-demo.example-tailnet.ts.net/api",   // /api
		"https://tailtui-demo.example-tailnet.ts.net:8443/", // :8443 port row
	} {
		if i > 0 {
			m2, _ := m.Update(key("j"))
			m = m2.(Model)
		}
		if !strings.Contains(m.View(), want) {
			t.Fatalf("row %d: expected URL %q:\n%s", i, want, m.View())
		}
	}
}

func TestServeCopyURLDispatches(t *testing.T) {
	m := openServeModal(t, 120, 40, hostedPorts())
	m2, cmd := m.Update(key("c"))
	if cmd == nil {
		t.Fatalf("[c] did not dispatch a clipboard command")
	}
	if got := m2.(Model).state; got != stateServe {
		t.Fatalf("[c] should stay in the list, got %v", got)
	}
	m3, _ := m2.(Model).Update(clipboardMsg{kind: clipboardServe})
	if !m3.(Model).serveCopied {
		t.Fatalf("clipboardServe did not set the copied flag")
	}
	if !strings.Contains(m3.(Model).View(), "copied") {
		t.Fatalf("copy confirmation not shown:\n%s", m3.(Model).View())
	}
}

// The add field teaches the format while empty, and switches to the live
// resolution once typing starts — the same space doing both jobs.
func TestServeAddFieldShowsExamplesThenResolution(t *testing.T) {
	m := openServeModal(t, 120, 40, nil)
	m2, _ := m.Update(key("a"))
	m = m2.(Model)

	view := m.View()
	for _, want := range []string{"3000", "/srv/docs", "text:back at 14:00"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty add field should show the example %q:\n%s", want, view)
		}
	}

	// Type through Update: the modal body is re-rendered into the viewport on
	// each keystroke, so setting the value directly would not refresh it.
	for _, r := range "3000" {
		m2, _ := m.Update(key(string(r)))
		m = m2.(Model)
	}
	view = m.View()
	if !strings.Contains(view, "proxy to 3000") {
		t.Fatalf("typing should show what the target resolves to:\n%s", view)
	}
	if strings.Contains(view, "a literal message") {
		t.Fatalf("examples should give way to the resolution once typing starts")
	}
}
