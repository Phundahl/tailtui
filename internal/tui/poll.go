package tui

import (
	"context"
	"os"
	"os/exec"
	osuser "os/user"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Phundahl/tailscaleTUI/internal/tailscale"
	"github.com/Phundahl/tailscaleTUI/internal/types"
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
