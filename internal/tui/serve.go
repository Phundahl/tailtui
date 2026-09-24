package tui

import (
	"fmt"
	"os"
	"strings"

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
	// Close is advertised by the modal chrome; repeating it here just doubles
	// the hint on screen.
	lines = append(lines, modalDivider(w), modalLine(w, accountKey("J/K", "NAVIGATE", false)))

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
	}
	return m, false
}
