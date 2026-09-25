package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

// Serve & Funnel modal (Phase 40) — READ-ONLY this phase.
//
// The tree is grouped by LISTENING PORT, because that is the unit Tailscale
// actually funnels: AllowFunnel is keyed by host:port, so making a port public
// exposes every path mounted on it. A per-path scope toggle would be a UI
// fiction the daemon cannot honour.
//
// The port number is where TAILSCALE listens, not where the local service
// does — `tailscale serve 3000` yields port 443 proxying to localhost:3000.
// The rows say LISTENING for exactly that reason.

// serveItemCount is the number of navigable rows: every port plus every path.
func (m Model) serveItemCount() int {
	n := 0
	for _, p := range m.serve {
		n += 1 + len(p.Paths)
	}
	return n
}

// openServe shows the modal. The caller refreshes via fetchServeCmd.
func (m Model) openServe() Model {
	m.state = stateServe
	m.serveCursor = 0
	w := overlayWidth(m.width)
	content := m.serveBody(w)
	m.overlay = newOverlayVP(w, overlayHeight(m.height, countLines(content)), content)
	return m
}

// resolveTarget describes what a filesystem target actually is, by stating it
// locally — tailtui runs on the machine doing the serving, so it can look
// rather than infer from the path string.
//
// This is the safety column: "directory, 2481 entries" is impossible to
// misread, where a bare path is easy to skim past. A missing target is worth
// surfacing too — it means the served path has been deleted since.
func resolveTarget(path string) (string, bool) {
	// Mock mode never touches the real filesystem, so a fixture path would
	// always resolve to "missing" and make the demo look broken. Return a
	// deterministic stand-in instead, matching how the rest of the mock works.
	if tailscale.MockEnabled() {
		return "directory, 12 entries", true
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "missing", false
	}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "directory", true
		}
		return fmt.Sprintf("directory, %d entries", len(entries)), true
	}
	return fmt.Sprintf("file, %s", humanBytes(fi.Size())), true
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// --- target risk -------------------------------------------------------------

// sensitiveNames are entries whose presence in a directory means serving it
// would expose credentials. Matched on IMMEDIATE children only: a recursive
// walk would be slow on a large tree and would flag far too much.
var sensitiveNames = map[string]bool{
	".ssh": true, ".aws": true, ".kube": true, ".gnupg": true,
	".env": true, ".git-credentials": true, ".netrc": true, ".npmrc": true,
	".docker": true, "id_rsa": true, "id_ed25519": true, "credentials.json": true,
}

// systemDirs are directories that should never be served wholesale.
//
// Matched EXACTLY, never by prefix. "/var" warrants a warning; "/var/www" is
// the normal thing to serve, and a warning that fires on ordinary use is one
// people learn to click through — worse than no warning at all.
var systemDirs = map[string]bool{
	"/": true, "/home": true, "/root": true, "/etc": true, "/boot": true,
	"/usr": true, "/var": true, "/opt": true, "/srv": true,
	"/bin": true, "/sbin": true, "/lib": true,
}

// TargetRisk is what an about-to-be-served directory actually contains.
//
// The danger of a share is in its CONTENTS, not its name: a home directory is
// obvious, but ~/Projects/app holding a .env is not, and no list of system
// directories would catch it. Naming the files found is also more persuasive
// than naming the directory — it says *why* to stop.
type TargetRisk struct {
	Entries   int
	Sensitive []string
	SystemDir bool
}

// Risky reports whether anything found warrants a warning block.
func (r TargetRisk) Risky() bool { return r.SystemDir || len(r.Sensitive) > 0 }

// analyzeTarget inspects a filesystem target. Local filesystem only, and only
// ever called for ServeFile kinds.
func analyzeTarget(path string) TargetRisk {
	var risk TargetRisk
	if path == "" {
		return risk
	}
	clean := filepath.Clean(path)
	if systemDirs[clean] {
		risk.SystemDir = true
	}
	if home, err := os.UserHomeDir(); err == nil && clean == filepath.Clean(home) {
		risk.SystemDir = true
	}
	entries, err := os.ReadDir(clean)
	if err != nil {
		return risk // a file, or missing: resolveTarget reports that separately
	}
	risk.Entries = len(entries)
	for _, e := range entries {
		if sensitiveNames[e.Name()] {
			risk.Sensitive = append(risk.Sensitive, e.Name())
		}
	}
	sort.Strings(risk.Sensitive)
	return risk
}

// serveURL builds the browsable URL for a path. The stored key is used
// VERBATIM: the daemon writes a directory as "/docs/" and a file as "/motd",
// and those are not interchangeable for a directory listing.
func serveURL(host string, port int, path string) string {
	if host == "" {
		host = "this-node"
	}
	if port != 443 {
		return fmt.Sprintf("https://%s:%d%s", host, port, path)
	}
	return fmt.Sprintf("https://%s%s", host, path)
}

// serveBody renders the port/path tree.
func (m Model) serveBody(w int) string {
	var lines []string

	if len(m.serve) == 0 {
		lines = append(lines,
			modalLine(w, styles.ModalDim.Render("  No services are being shared.")),
			modalLine(w, ""),
			modalLine(w, styles.ModalText.Render("  Serve shares a local service with your tailnet over HTTPS.")),
			modalLine(w, styles.ModalText.Render("  Funnel additionally exposes it to the public internet.")),
		)
		return strings.Join(lines, "\n")
	}

	host := m.local.Hostname
	row := 0
	var highlighted string

	for _, p := range m.serve {
		scope := styles.ModalDim.Render("[ TAILNET ]")
		if p.Funnel {
			scope = styles.StatusErr.Render("[ PUBLIC ]")
		}
		label := fmt.Sprintf(" :%d  LISTENING", p.Port)
		if m.serveCursor == row {
			plain := "[ TAILNET ]"
			if p.Funnel {
				plain = "[ PUBLIC ]"
			}
			lines = append(lines, styles.AccountActive.Render(joinRow(label, plain+" ", w)))
		} else {
			lines = append(lines, modalRow(w, styles.ModalHeading.Render(label), scope))
		}
		row++

		for _, sp := range p.Paths {
			detail := ""
			if sp.Kind == types.ServeFile {
				d, ok := resolveTarget(sp.Target)
				if ok {
					detail = styles.ModalDim.Render("  " + d)
				} else {
					detail = styles.StatusErr.Render("  " + d)
				}
			}
			left := fmt.Sprintf("    %-10s %s %s", sp.Path, sp.Kind.Icon(), sp.Target)
			if m.serveCursor == row {
				lines = append(lines, styles.AccountActive.Render(joinRow(left, "", w)))
				highlighted = serveURL(host, p.Port, sp.Path)
			} else {
				lines = append(lines, modalRow(w, styles.ModalText.Render(left), detail))
			}
			row++
		}
		lines = append(lines, modalLine(w, ""))
	}

	if highlighted != "" {
		lines = append(lines, modalDivider(w), modalLine(w, styles.ModalAccent.Render("  "+highlighted)))
	}
	lines = append(lines, modalDivider(w))
	if m.serveInputMode {
		prompt := "Share what?  (port, path, URL, or text:…)"
		if m.serveInputErr {
			prompt = "Enter something to share — port, path, URL or text:…"
		}
		style := styles.ModalHeading
		if m.serveInputErr {
			style = styles.StatusErr
		}
		lines = append(lines, modalLine(w, style.Render("  "+prompt)),
			modalLine(w, m.serveInput.View()))
		// Say what the typed target will actually become, before committing —
		// naming the contents is the whole point of this feature's safety.
		if _, detail := classifyTarget(m.serveInput.Value()); detail != "" {
			lines = append(lines, modalLine(w, styles.ModalDim.Render("  → "+detail)))
		}
		lines = append(lines, modalDivider(w),
			gridLine(w, accountKey("ENTER", "CONFIRM", false), accountKey("ESC", "CANCEL", false)))
	} else {
		lines = append(lines,
			gridLine(w, accountKey("J/K", "NAVIGATE", false), accountKey("A", "ADD", false)),
			gridLine(w, accountKey("SPACE", "TAILNET/PUBLIC", false), accountKey("D", "REMOVE", false)))
	}

	return strings.Join(lines, "\n")
}

// updateServeList handles navigation. Read-only: no add, remove or scope
// toggle this phase, so every other key is swallowed rather than acted on.
func (m Model) updateServeList(key string) (Model, bool) {
	switch key {
	case "j", "down":
		if m.serveCursor < m.serveItemCount()-1 {
			m.serveCursor++
		}
		return m, true
	case "k", "up":
		if m.serveCursor > 0 {
			m.serveCursor--
		}
		return m, true

	case "a":
		m.serveInputMode = true
		m.serveInputErr = false
		m.serveInput = newServeInput()
		m.serveInput.Width = clampInputWidth(overlayWidth(m.width))
		m.serveInput.Focus()
		return m, true

	case "d":
		port, pathIdx, ok := m.serveRowAt(m.serveCursor)
		if !ok {
			return m, true
		}
		p := servePendingAction{port: port.Port, paths: port.Paths}
		if pathIdx < 0 {
			// A port row: removing it unmounts every path on it.
			p.action, p.path = tailscale.ServeRemovePort, "/"
		} else {
			p.action = tailscale.ServeRemovePath
			p.path = port.Paths[pathIdx].Path
			p.kind = port.Paths[pathIdx].Kind
		}
		m.servePending = p
		m.state = stateServeConfirm
		m.serveCopied = false
		return m, true

	case " ", "space":
		// Scope belongs to the PORT: AllowFunnel is keyed by host:port, so a
		// path row has no scope of its own to toggle.
		port, pathIdx, ok := m.serveRowAt(m.serveCursor)
		if !ok || pathIdx >= 0 || len(port.Paths) == 0 {
			return m, true
		}
		// Re-issuing the command needs the original target — and for
		// unpublish it MUST be `serve`, never `funnel ... off`, which would
		// delete the share rather than making it private.
		first := port.Paths[0]
		action := tailscale.ServePublish
		if port.Funnel {
			action = tailscale.ServeUnpublish
		}
		m.servePending = servePendingAction{
			action: action, port: port.Port, path: first.Path,
			target: serveTargetArg(first), kind: first.Kind, paths: port.Paths,
		}
		m.state = stateServeConfirm
		m.serveCopied = false
		return m, true
	}
	return m, false
}

// serveTargetArg turns a parsed path back into the argument form the CLI
// expects, which is not always how it is stored: a text handler stores the
// literal but must be passed back as "text:<literal>".
func serveTargetArg(sp types.ServePath) string {
	if sp.Kind == types.ServeText {
		return "text:" + sp.Target
	}
	return sp.Target
}

// --- editing -----------------------------------------------------------------

// serveRowAt maps a cursor index onto the port/path tree. pathIdx is -1 when
// the cursor is on a port row.
func (m Model) serveRowAt(idx int) (port types.ServePort, pathIdx int, ok bool) {
	row := 0
	for _, p := range m.serve {
		if row == idx {
			return p, -1, true
		}
		row++
		for i := range p.Paths {
			if row == idx {
				return p, i, true
			}
			row++
		}
	}
	return types.ServePort{}, -1, false
}

// newServeInput builds the target editor, inheriting the shared styling
// contract (no placeholder, visible cursor) from newModalInput.
func newServeInput() textinput.Model { return newModalInput(128) }

// classifyTarget reports what a typed target will become, so the add field can
// say so before the user commits. This is the same "name the contents, not the
// port" principle the confirmation uses.
func classifyTarget(raw string) (types.ServeKind, string) {
	t := strings.TrimSpace(raw)
	switch {
	case t == "":
		return types.ServeProxy, ""
	case strings.HasPrefix(t, "text:"):
		return types.ServeText, "literal text"
	case isFilesystemTarget(t):
		detail, _ := resolveTarget(t)
		return types.ServeFile, detail
	default:
		return types.ServeProxy, "proxy to " + t
	}
}

// isFilesystemTarget mirrors the adapter's rule: bare paths are filesystem
// targets, anything with a supported scheme (except unix:) is not.
func isFilesystemTarget(t string) bool {
	for _, scheme := range []string{"http://", "https://", "https+insecure://"} {
		if strings.HasPrefix(t, scheme) {
			return false
		}
	}
	return strings.HasPrefix(t, "/") || strings.HasPrefix(t, "./") ||
		strings.HasPrefix(t, "~") || strings.HasPrefix(t, "unix:")
}

// pathExists reports whether a mount point is already taken on a port. Adding
// an existing path is an UPSERT — the daemon replaces it silently — so the UI
// warns rather than letting a share vanish.
func (m Model) pathExists(port int, path string) bool {
	want := tailscale.NormalizeServePath(path)
	for _, p := range m.serve {
		if p.Port != port {
			continue
		}
		for _, sp := range p.Paths {
			if tailscale.NormalizeServePath(sp.Path) == want {
				return true
			}
		}
	}
	return false
}

// updateServeInput owns every key while a target is being typed, including
// esc/q — so a target containing a "q" survives and the global close handler
// cannot fire mid-entry.
func (m Model) updateServeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		raw := strings.TrimSpace(m.serveInput.Value())
		if raw == "" {
			m.serveInputErr = true
			return m, nil
		}
		kind, _ := classifyTarget(raw)
		port, path := 443, "/"
		if p, _, ok := m.serveRowAt(m.serveCursor); ok {
			port = p.Port
		}
		m.servePending = servePendingAction{
			action: tailscale.ServeAdd,
			port:   port, path: path, target: raw, kind: kind,
		}
		m.serveInputMode = false
		m.serveInput.Blur()
		m.state = stateServeConfirm
		m.serveCopied = false
		return m, nil
	case "esc":
		m.serveInputMode = false
		m.serveInputErr = false
		m.serveInput.Blur()
		m.serveInput.SetValue("")
		return m.resizeOverlay(), nil
	}
	var cmd tea.Cmd
	m.serveInput, cmd = m.serveInput.Update(msg)
	m.serveInputErr = false
	return m.resizeOverlay(), cmd
}

// updateServeConfirm handles the Command Room. Enter applies, c copies, Esc
// returns to the list without acting.
func (m Model) updateServeConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.servePending
	switch msg.String() {
	case "enter":
		// Close before dispatch (the Phase 24.1 lesson): a path target hands
		// the terminal to sudo, and the last drawn frame must not be a modal.
		m.state = stateMain
		return m, serveApplyCmd(p)
	case "c", "C":
		return m, copyCmd(clipboardServe,
			tailscale.ServeCommandString(p.action, p.port, p.path, p.target))
	case "esc", "q":
		m.state = stateServe
		m.serveCopied = false
		return m.resizeOverlay(), nil
	}
	return m, nil
}

// renderServeConfirmOverlay draws the Command Room for a pending edit.
//
// Two INDEPENDENT risks get two independent warning blocks, because they are
// genuinely different questions: *what* is being exposed, and *to whom*. The
// worst case — a directory full of credentials going public — shows both.
//
// Friction is asymmetric on purpose. Publishing warns loudly; unpublishing,
// removing, or serving an ordinary port is a quiet confirmation. A warning
// that fires on every action is one people learn to click through.
func (m Model) renderServeConfirmOverlay(base string) string {
	w := overlayWidth(m.width)
	p := m.servePending
	cmd := tailscale.ServeCommandString(p.action, p.port, p.path, p.target)

	title := "[ CONFIRM SERVE CHANGE ]"
	if p.action == tailscale.ServePublish {
		title = "[ CONFIRM PUBLIC EXPOSURE ]"
	}

	lines := []string{
		modalLine(w, lipgloss.PlaceHorizontal(w, lipgloss.Center,
			styles.ModalTitle.Render(title),
			lipgloss.WithWhitespaceBackground(styles.Surface))),
		modalDivider(w),
	}

	warn := lipgloss.NewStyle().Foreground(styles.Subtle).Background(styles.Surface).Italic(true)

	// --- risk 1: what is being exposed -------------------------------------
	if p.kind == types.ServeFile {
		if risk := analyzeTarget(p.target); risk.Risky() {
			lines = append(lines,
				modalLine(w, styles.StatusErr.Render("⚠  THIS DIRECTORY CONTAINS SENSITIVE FILES")),
				modalLine(w, ""))
			detail, _ := resolveTarget(p.target)
			// Wrap explicitly: a long path plus its resolution easily exceeds
			// the modal width, and modalLine clips rather than wraps.
			for _, ln := range wrapText(p.target+"  —  "+detail, w-4) {
				lines = append(lines, modalLine(w, styles.ModalText.Render("   "+ln)))
			}
			if len(risk.Sensitive) > 0 {
				lines = append(lines, modalLine(w,
					styles.StatusErr.Render("   Found:  "+strings.Join(risk.Sensitive, "   "))))
			}
			if risk.SystemDir {
				lines = append(lines, modalLine(w,
					styles.ModalText.Render("   This is a system or home directory.")))
			}
			for _, ln := range wrapText("Serving it publishes every file beneath it.", w-3) {
				lines = append(lines, modalLine(w, warn.Render("   "+ln)))
			}
			lines = append(lines, modalLine(w, ""))
		}
	}

	// --- risk 2: who can reach it ------------------------------------------
	if p.action == tailscale.ServePublish {
		lines = append(lines,
			modalLine(w, styles.StatusErr.Render("⚠  REACHABLE FROM THE PUBLIC INTERNET")),
			modalLine(w, ""))
		blast := fmt.Sprintf("   Port %d becomes public, including all %d path(s) on it:",
			p.port, len(p.paths))
		lines = append(lines, modalLine(w, styles.ModalText.Render(blast)))
		for _, sp := range p.paths {
			lines = append(lines, modalLine(w, styles.ModalText.Render(
				fmt.Sprintf("      %-10s %s %s", sp.Path, sp.Kind.Icon(), sp.Target))))
		}
		for _, ln := range wrapText("Anyone with the URL can reach them. No tailnet membership, no authentication.", w-3) {
			lines = append(lines, modalLine(w, warn.Render("   "+ln)))
		}
		lines = append(lines, modalLine(w, ""))
	}

	// --- upsert warning ----------------------------------------------------
	if p.action == tailscale.ServeAdd && m.pathExists(p.port, p.path) {
		lines = append(lines, modalLine(w, styles.StatusWarn.Render(
			fmt.Sprintf("⚠  This REPLACES the existing %s on :%d.", p.path, p.port))),
			modalLine(w, ""))
	}

	lines = append(lines, modalLine(w, styles.ModalDim.Render("ABOUT TO EXECUTE:")))
	for _, ln := range wrapCommand(cmd, w) {
		lines = append(lines, modalLine(w, styles.ModalAccent.Render(ln)))
	}

	if tailscale.ServeNeedsRoot(p.action, p.target) {
		lines = append(lines, modalLine(w, ""),
			modalLine(w, warn.Render("   Serving a filesystem path requires root; you will be")),
			modalLine(w, warn.Render("   prompted for your password.")))
	}

	if p.action == tailscale.ServePublish {
		lines = append(lines, modalLine(w, ""),
			modalLine(w, styles.ModalDim.Render("Public URL:")),
			modalLine(w, styles.ModalAccent.Render(serveURL(m.local.Hostname, p.port, p.path))))
	}

	apply := "APPLY"
	if p.action == tailscale.ServePublish {
		apply = "EXPOSE PUBLICLY"
	}
	lines = append(lines, modalDivider(w),
		gridLine(w, accountKey("ENTER", apply, p.action == tailscale.ServePublish),
			accountKey("C", "COPY", false)),
		modalLine(w, accountKey("ESC", "BACK", false)))

	if m.serveCopied {
		lines = append(lines, modalLine(w, styles.StatusOK.Render("✓ Copied to clipboard!")))
	}

	inner := strings.Join(lines, "\n")
	modal := lipgloss.NewStyle().
		Width(w+2*modalHPad).
		Height(countLines(inner)+2*modalVPad).
		Background(styles.Surface).
		Foreground(styles.Fg).
		Padding(modalVPad, modalHPad).
		Border(lipgloss.NormalBorder()).
		BorderForeground(styles.Primary).
		BorderBackground(styles.Surface).
		Render(inner)

	return overlayCenter(base, modal)
}
