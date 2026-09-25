package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailtui/internal/styles"
	"github.com/Phundahl/tailtui/internal/tailscale"
	"github.com/Phundahl/tailtui/internal/types"
)

const (
	// refreshInterval is how often the node list is re-fetched from the daemon.
	refreshInterval = 4 * time.Second
	// pingInterval is how often the highlighted node is pinged for live latency.
	pingInterval = 2 * time.Second
	// maxLatencySamples caps a node's rolling latency history (the graph
	// resamples to the pane width regardless).
	maxLatencySamples = 40
)

// statusMsg carries the result of one `tailscale status` fetch back into the
// Elm loop (this is the "TailscaleStatusMsg"). On error, local/peers are zero
// and the model keeps its last good data on screen.
type statusMsg struct {
	local types.LocalStatus
	peers []types.Peer
	err   error
}

// tickMsg fires every refreshInterval to trigger the next background fetch.
type tickMsg time.Time

// fetchStatusCmd runs the (blocking) CLI call off the UI thread and delivers a
// statusMsg. The context bounds it well under refreshInterval so a stalled
// daemon can't pile fetches up. Returning a tea.Cmd keeps os/exec off the main
// Update goroutine entirely — Bubble Tea runs the closure in its own goroutine.
func fetchStatusCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), refreshInterval-time.Second)
		defer cancel()
		local, peers, err := tailscale.Status(ctx)
		return statusMsg{local: local, peers: peers, err: err}
	}
}

// tickCmd schedules the next refresh tick.
func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// themeMsg reports one theme-file check. changed is true only when the active
// file's path or mtime moved, in which case theme carries the freshly parsed
// palette.
type themeMsg struct {
	theme   styles.Theme
	path    string
	mod     time.Time
	changed bool
}

// checkThemeCmd stats the active theme file off the UI thread and re-parses it
// when it has moved since prevPath/prevMod. Omarchy rewrites the theme
// directory on every switch, so this is what makes a running tailTUI follow the
// desktop theme within one refresh tick.
//
// The parsed Theme is *returned*, never applied here: styles.Apply mutates
// package-level vars that View reads, so applying it must happen in Update, on
// the single Elm goroutine. Applying it in this closure would race the renderer.
func checkThemeCmd(prevPath string, prevMod time.Time) tea.Cmd {
	return func() tea.Msg {
		path, mod := styles.ThemeStamp()
		if path == prevPath && mod.Equal(prevMod) {
			return themeMsg{path: path, mod: mod}
		}
		return themeMsg{theme: styles.LoadTheme(), path: path, mod: mod, changed: true}
	}
}

// pingMsg carries one live latency sample (ms) for the node at ip back into the
// loop. ok is false when the node didn't answer, in which case the sample is
// dropped rather than recorded as a fake value.
type pingMsg struct {
	ip string
	ms int
	ok bool
}

// pingTickMsg fires every pingInterval to ping whichever node is highlighted.
type pingTickMsg time.Time

// pingCmd measures live latency to ip off the UI thread, bounded so an
// unreachable node can't stall the ticker.
func pingCmd(ip string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pingInterval+time.Second)
		defer cancel()
		ms, err := tailscale.Ping(ctx, ip)
		return pingMsg{ip: ip, ms: ms, ok: err == nil}
	}
}

// pingTickCmd schedules the next ping tick.
func pingTickCmd() tea.Cmd {
	return tea.Tick(pingInterval, func(t time.Time) tea.Msg {
		return pingTickMsg(t)
	})
}

// actionMsg carries the result of a state-mutating CLI action (e.g. setting the
// exit node). desc is a human description for the log ring; a non-nil err is
// appended as an [ERROR]. The next status poll reconciles the model with the
// daemon's true state.
type actionMsg struct {
	desc string
	err  error
}

// setExitNodeCmd executes `tailscale set --exit-node=<ip>` (empty ip clears it)
// off the UI thread, tagging the result with desc for the log.
func setExitNodeCmd(ip, desc string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return actionMsg{desc: desc, err: tailscale.SetExitNode(ctx, ip)}
	}
}

// prefsMsg carries the result of a `tailscale debug prefs` read back into the
// loop to populate the Advanced Settings checkboxes. On error the last good
// prefs stay on screen.
type prefsMsg struct {
	prefs types.Prefs
	err   error
}

// fetchPrefsCmd reads the live local-node preferences off the UI thread.
func fetchPrefsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		prefs, err := tailscale.GetPrefs(ctx)
		return prefsMsg{prefs: prefs, err: err}
	}
}

// prefActionMsg carries the result of a single `tailscale set --<flag>` toggle
// from the Advanced Settings modal. desc is logged; on completion the model
// re-fetches prefs so the checkbox reconciles with the daemon's true state
// (reverting an optimistic flip if the command failed).
type prefActionMsg struct {
	desc string
	err  error
}

// setPrefCmd toggles one boolean preference off the UI thread, tagging the
// outcome with desc for the log ring.
func setPrefCmd(flag string, val bool, desc string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return prefActionMsg{desc: fmt.Sprintf("%s = %t", desc, val), err: tailscale.SetPref(ctx, flag, val)}
	}
}

// routingActionMsg carries the result of applying the routing modal's staged
// state (`tailscale set --advertise-exit-node/--advertise-routes`). desc is the
// executed command (for the log); on completion the model refreshes status +
// prefs so the UI reflects the new advertised state.
type routingActionMsg struct {
	desc string
	err  error
}

// setRoutingCmd applies the routing working copy off the UI thread.
func setRoutingCmd(exitNode bool, routes []string) tea.Cmd {
	desc := tailscale.AdvertiseCommandString(exitNode, routes)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return routingActionMsg{desc: desc, err: tailscale.SetRouting(ctx, exitNode, routes)}
	}
}

// clipboardKind identifies which modal asked for a copy, so the result flashes
// the right "Copied!" indicator. Routing is the ZERO VALUE deliberately: it
// keeps every existing clipboardMsg construction valid and correct. Carrying the
// kind on the message rather than branching on m.state at delivery time also
// closes a real (if benign) race — press `c`, then Esc before the clipboard
// goroutine returns, and a state-based branch would set the wrong flag.
type clipboardKind int

const (
	clipboardRouting clipboardKind = iota
	clipboardSSH
	clipboardServe
)

// clipboardMsg carries the result of a copy-to-clipboard action.
type clipboardMsg struct {
	kind clipboardKind
	err  error
}

// copyCmd copies text to the system clipboard off the UI thread (atotto/
// clipboard shells out to the platform tool — pbcopy / wl-copy / xclip / clip),
// so a missing tool surfaces as an error rather than blocking or crashing.
func copyCmd(kind clipboardKind, text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardMsg{kind: kind, err: clipboard.WriteAll(text)}
	}
}

// serveMsg carries the result of a `tailscale serve status --json` fetch.
// `manual` marks a user-requested refresh, which is the only kind that logs on
// success: the automatic fetches (startup, modal open, post-action) would
// otherwise narrate themselves, and a log line nobody asked for is noise.
type serveMsg struct {
	ports  []types.ServePort
	manual bool
	err    error
}

// fetchServeCmd reads the live Serve/Funnel configuration off the UI thread.
// An unconfigured node is an empty list, not an error.
func fetchServeCmd() tea.Cmd { return serveFetch(false) }

// refreshServeCmd is the same read, attributed to the user so it leaves a
// receipt in the log ring. That receipt is the point: it is how you tell an
// entry that really went away from a row that merely vanished from the view.
func refreshServeCmd() tea.Cmd { return serveFetch(true) }

func serveFetch(manual bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ports, err := tailscale.ServeStatus(ctx)
		return serveMsg{ports: ports, manual: manual, err: err}
	}
}

// serveActionMsg is delivered after a Serve/Funnel edit finishes.
type serveActionMsg struct {
	desc string
	err  error
}

// serveApplyCmd runs one Serve/Funnel edit.
//
// Filesystem targets are PRIVILEGED — the daemon refuses them unless the
// caller is root — so those go through tea.ExecProcess with sudo, giving the
// password prompt a real terminal. Ports and text need no elevation and stay
// background commands, so the common case never flashes a sudo prompt.
func serveApplyCmd(p servePendingAction) tea.Cmd {
	args := tailscale.ServeArgs(p.action, p.port, p.path, p.target)
	desc := tailscale.ServeCommandString(p.action, p.port, p.path, p.target)

	if tailscale.MockEnabled() {
		return func() tea.Msg { return serveActionMsg{desc: desc + " (mock)"} }
	}

	if tailscale.ServeNeedsRoot(p.action, p.target) {
		c := exec.Command("sudo", append([]string{"tailscale"}, args...)...)
		return tea.ExecProcess(c, func(err error) tea.Msg {
			return serveActionMsg{desc: desc, err: err}
		})
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return serveActionMsg{desc: desc, err: tailscale.ApplyServe(ctx, args)}
	}
}

// accountsMsg carries the result of a `tailscale switch --list` fetch.
type accountsMsg struct {
	accounts []types.Account
	err      error
}

// accountsLockedMsg is returned by fetchAccountsCmd when the daemon reports
// "profiles access denied" — the profile store is root-owned and we're running
// unprivileged. It is intentionally a distinct message (not an accountsMsg with
// an err) so the Update handler can mark the lock without routing through the
// generic error-log path: a non-elevated session would otherwise spam the log
// ring with the same error on every background refresh.
type accountsLockedMsg struct{}

// isProfilesAccessDenied recognizes the daemon's "Access denied: profiles
// access denied" response (case-insensitive substring match), which surfaces
// for every profile-store read or mutation from an unprivileged session.
func isProfilesAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "profiles access denied") || strings.Contains(s, "access denied")
}

// accountActionMsg carries the result of an account-mutating command (switch,
// remove, logout, login). desc is logged; on completion the model refreshes the
// account list and status so the UI reflects the new reality.
type accountActionMsg struct {
	desc string
	err  error
}

// fetchAccountsCmd lists the local profiles off the UI thread. A
// "profiles access denied" failure is folded into a distinct accountsLockedMsg
// so the Update loop can flip the lock flag without logging — this command is
// re-fired on every refresh and every account action, so a non-elevated
// session would otherwise paper the log ring with the same line.
func fetchAccountsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		accounts, err := tailscale.Accounts(ctx)
		if isProfilesAccessDenied(err) {
			return accountsLockedMsg{}
		}
		return accountsMsg{accounts: accounts, err: err}
	}
}

// switchAccountCmd switches the active profile interactively via
// tea.ExecProcess. On Linux the profile store is root-owned, so switching
// requires root (same constraint as login) — the daemon-operator role isn't
// enough. Wrapping in sudo + handing the terminal over lets the password
// prompt be visible/interactive; an inherited-credentials session just runs.
// In mock mode the in-memory state is flipped synchronously so the demo
// stays interactive without ever suspending the TUI.
func switchAccountCmd(id, name string) tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg {
			tailscale.MockSwitchAccount(id)
			return accountActionMsg{desc: "switched account → " + name + " (mock)"}
		}
	}
	c := exec.Command("sudo", "tailscale", "switch", id)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return accountActionMsg{desc: "switched account → " + name, err: err}
	})
}

// removeAccountCmd forgets a stored profile interactively via tea.ExecProcess.
// Removing a profile mutates the root-owned profile store (same constraint as
// add / switch / login on Linux), so we wrap in sudo and hand the terminal
// over for the password prompt; an inherited-credentials session just runs.
// In mock mode the profile is dropped from the in-memory list synchronously.
func removeAccountCmd(id, name string) tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg {
			tailscale.MockRemoveAccount(id)
			return accountActionMsg{desc: "removed account " + name + " (mock)"}
		}
	}
	c := exec.Command("sudo", "tailscale", "switch", "remove", id)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return accountActionMsg{desc: "removed account " + name, err: err}
	})
}

// logoutCmd logs the current session out interactively via tea.ExecProcess.
// Logout clears the active profile in the root-owned store, so it requires
// sudo on Linux for the same reason as the other profile-store mutations.
// In mock mode the active flag is cleared on every in-memory account.
func logoutCmd() tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg {
			tailscale.MockLogout()
			return accountActionMsg{desc: "logged out (mock)"}
		}
	}
	c := exec.Command("sudo", "tailscale", "logout")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return accountActionMsg{desc: "logged out", err: err}
	})
}

// addAccountCmd runs `sudo tailscale login` interactively via tea.ExecProcess.
// On Linux, adding a profile mutates the root-owned profile store and the
// daemon returns "profiles access denied" without elevation regardless of the
// operator role, so we pre-elevate; tea.ExecProcess hands the terminal over so
// both the sudo password prompt and the subsequent auth URL print are visible
// and interactive. In mock mode a new inactive profile is appended to the
// in-memory list synchronously — no shell-out, no auth flow.
func addAccountCmd() tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg {
			tailscale.MockAddAccount()
			return accountActionMsg{desc: "added account (mock)"}
		}
	}
	c := exec.Command("sudo", "tailscale", "login")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return accountActionMsg{desc: "added account (sudo tailscale login)", err: err}
	})
}

// operatorDoneMsg is delivered after the interactive operator-setup command
// finishes and the TUI has been restored.
type operatorDoneMsg struct{ err error }

// currentUser resolves the local username for `--operator=` and as the default
// SSH login. The resolution logic lives in the adapter (tailscale.CurrentUser)
// because the mock fixtures need the same answer; this stays as the in-package
// spelling used by the UI.
func currentUser() string { return tailscale.CurrentUser() }

// connectDoneMsg is delivered after the interactive connect/disconnect command
// finishes and the TUI has been restored.
type connectDoneMsg struct {
	up  bool
	err error
}

// connectCmd suspends the TUI and runs `tailscale up` (up==true) or `tailscale
// down`, handing the terminal over (via tea.ExecProcess) so that an auth URL
// printed by `up` is visible and interactive. No sudo — the user is already a
// configured operator. The result returns as connectDoneMsg once restored.
// In mock mode the connected flag is flipped in memory synchronously.
func connectCmd(up bool) tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg {
			tailscale.MockSetConnected(up)
			return connectDoneMsg{up: up}
		}
	}
	name := "down"
	if up {
		name = "up"
	}
	c := exec.Command("tailscale", name)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return connectDoneMsg{up: up, err: err}
	})
}

// sshDoneMsg is delivered after an interactive `tailscale ssh` session ends and
// the TUI has been restored.
type sshDoneMsg struct {
	target string
	mock   bool
	err    error
}

// sshLaunchCmd suspends the TUI and hands the terminal to `tailscale ssh
// [user@]host`. No sudo — the wrapper needs none — and, uniquely among the
// commands in this file, NO context timeout: every other one is boxed to 8-15s,
// but an interactive session is unbounded by definition and a context would kill
// it mid-work.
//
// `tailscale ssh` execs the SYSTEM ssh client, so a missing `ssh` binary is
// pre-flighted here rather than left to the child: the error then lands in the
// log ring (readable with [v]) instead of flashing past during the restore.
//
// In mock mode nothing is executed — the demo (and `vhs demo.tape`) must never
// spawn a real ssh.
func sshLaunchCmd(user, host string) tea.Cmd {
	target := tailscale.SSHTarget(user, host)
	if tailscale.MockEnabled() {
		return func() tea.Msg { return sshDoneMsg{target: target, mock: true} }
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		return func() tea.Msg {
			return sshDoneMsg{target: target, err: fmt.Errorf("no ssh client on PATH: %w", err)}
		}
	}
	c := exec.Command("tailscale", tailscale.SSHArgs(user, host)...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return sshDoneMsg{target: target, err: err}
	})
}

// operatorSetupCmd suspends the TUI and hands the terminal to an interactive
// `sudo tailscale set --operator=$USER`, so the user can type their password.
// tea.ExecProcess releases the terminal before running and restores it after;
// stdin/stdout/stderr are left unset so they inherit the program's terminal.
// The result comes back as operatorDoneMsg once the TUI is restored. In mock
// mode this is a no-op that just reports success — the demo always has perms.
func operatorSetupCmd() tea.Cmd {
	if tailscale.MockEnabled() {
		return func() tea.Msg { return operatorDoneMsg{} }
	}
	c := exec.Command("sudo", "tailscale", "set", "--operator="+currentUser())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return operatorDoneMsg{err: err}
	})
}
