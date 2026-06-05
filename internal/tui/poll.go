package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	osuser "os/user"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

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

// clipboardMsg carries the result of a copy-to-clipboard action.
type clipboardMsg struct{ err error }

// copyRoutingCmd copies text to the system clipboard off the UI thread (atotto/
// clipboard shells out to the platform tool — pbcopy / wl-copy / xclip / clip),
// so a missing tool surfaces as an error rather than blocking or crashing.
func copyRoutingCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardMsg{err: clipboard.WriteAll(text)}
	}
}

// accountsMsg carries the result of a `tailscale switch --list` fetch.
type accountsMsg struct {
	accounts []types.Account
	err      error
}

// accountActionMsg carries the result of an account-mutating command (switch,
// remove, logout, login). desc is logged; on completion the model refreshes the
// account list and status so the UI reflects the new reality.
type accountActionMsg struct {
	desc string
	err  error
}

// fetchAccountsCmd lists the local profiles off the UI thread.
func fetchAccountsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		accounts, err := tailscale.Accounts(ctx)
		return accountsMsg{accounts: accounts, err: err}
	}
}

// switchAccountCmd switches the active profile (`tailscale switch <id>`).
func switchAccountCmd(id, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return accountActionMsg{desc: "switched account → " + name, err: tailscale.SwitchAccount(ctx, id)}
	}
}

// removeAccountCmd forgets a stored profile (`tailscale switch remove <id>`).
func removeAccountCmd(id, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return accountActionMsg{desc: "removed account " + name, err: tailscale.RemoveAccount(ctx, id)}
	}
}

// logoutCmd logs the current session out (`tailscale logout`).
func logoutCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return accountActionMsg{desc: "logged out", err: tailscale.Logout(ctx)}
	}
}

// addAccountCmd runs `tailscale login` interactively via tea.ExecProcess so the
// user can complete the auth URL in the terminal, then refreshes on return.
func addAccountCmd() tea.Cmd {
	c := exec.Command("tailscale", "login")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return accountActionMsg{desc: "added account (tailscale login)", err: err}
	})
}

// operatorDoneMsg is delivered after the interactive operator-setup command
// finishes and the TUI has been restored.
type operatorDoneMsg struct{ err error }

// currentUser resolves the local username for `--operator=`, preferring $USER.
func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u, err := osuser.Current(); err == nil {
		return u.Username
	}
	return ""
}

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
func connectCmd(up bool) tea.Cmd {
	name := "down"
	if up {
		name = "up"
	}
	c := exec.Command("tailscale", name)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return connectDoneMsg{up: up, err: err}
	})
}

// operatorSetupCmd suspends the TUI and hands the terminal to an interactive
// `sudo tailscale set --operator=$USER`, so the user can type their password.
// tea.ExecProcess releases the terminal before running and restores it after;
// stdin/stdout/stderr are left unset so they inherit the program's terminal.
// The result comes back as operatorDoneMsg once the TUI is restored.
func operatorSetupCmd() tea.Cmd {
	c := exec.Command("sudo", "tailscale", "set", "--operator="+currentUser())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return operatorDoneMsg{err: err}
	})
}
