package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestServeEmptyState(t *testing.T) {
	m := openServeModal(t, 120, 40, nil)
	view := m.View()
	assertFlush(t, view, 120, 40)
	if !strings.Contains(view, "No services are being shared") {
		t.Fatalf("empty state missing:\n%s", view)
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

// Read-only phase: no mutation keys are wired, and none may leak to the list.
func TestServeIsReadOnly(t *testing.T) {
	m := openServeModal(t, 120, 40, servePorts())
	for _, k := range []string{"a", "d", "space", "enter"} {
		m2, cmd := m.Update(key(k))
		if got := m2.(Model).state; got != stateServe {
			t.Fatalf("%q changed state to %v; the modal should swallow it", k, got)
		}
		if cmd != nil {
			t.Fatalf("%q dispatched a command in a read-only modal", k)
		}
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
