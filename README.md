<img src="assets/logo.png" alt="tailTUI Logo" width="150" />

# tailTUI
*A brutalist, keyboard-centric terminal user interface for Tailscale.*

`tailTUI` is a fast, dense, single-screen control panel for your tailnet. It
wraps the `tailscale` CLI in a sharp, no-nonsense TUI built on the
[Charmbracelet](https://charm.sh) stack — so you can see your whole network,
ping peers, switch accounts, and flip your connection without ever leaving the
terminal.

```
 tailTUI                                              (q)uit (?)help
┌─┤ LOCAL_NODE ├────────────┐ ┌─┤ PEER DETAILS: dc-subnet-01 ├───┐
│ User / Host / Exit State  │ │ IDENTITY · OS · IP · Conn · Tags │
│ Exit / Exit Latency       │ │ [e] 12 advertised routes         │
│      [c] Disconnect       │ └──────────────────────────────────┘
└───────────────────────────┘ ┌─┤ LATENCY HISTORY ├──────────────┐
┌─┤ SEARCH: dc▌ ├───────────┐ │     ██      ▄▄                   │
│ ❯ 󰒄 [ROUT] dc-subnet-01  ●│ │ ▂▂▂▂██▅▅████████▇▇               │
│   󰒄 [ROUT] dc-subnet-02  ●│ │ ████████████████████            │
│   󰌽 dc-bastion           ●│ ├─┤ TERMINAL_LOGS ├────────────────┤
│   ...                     │ │ 14:55 [INFO] exit node set       │
└───────────────────────────┘ └──────────────────────────────────┘
 [j/k] Nav [/] Search [c] Disconnect [x] Exit Node …  ● CONNECTED  v1.0.0
```

## Why tailTUI?

The official `tailscale` CLI is excellent, but managing a large tailnet means
re-running `status`, squinting at JSON, and copy-pasting IPs. `tailTUI` is built
for the opposite workflow:

- **Built for speed and flow-state.** Everything is one keystroke away. No menus,
  no mouse, no context switching. The whole network is on one screen, refreshed
  live.
- **At home in a tiling window manager.** A sharp, flush, single-line-bordered
  layout that snaps cleanly into any pane and stays flush at any size — no
  wasted space, no wrapping, no rounded-corner fluff.
- **Never leaves the terminal.** Auth flows, operator setup, and login prompts
  drop you to the shell only when *they* need to (to paste an auth URL), then
  restore the UI automatically.

## Features

- **Live, multi-row latency graphing.** Select any peer and watch a real-time
  vertical bar chart of round-trip latency (`tailscale ping`), color-graded by
  severity, with live MIN / AVG / MAX. The chart grows to fill the pane.
- **fzf-style fuzzy search.** Press `/` and type to instantly filter massive
  tailnets by hostname or tag. Navigate the results *while typing* with the
  arrows or `Ctrl+j`/`Ctrl+k`; `Enter`/`Esc` to apply, `Esc` again to clear.
- **Live, color-coded log tailing.** A capped in-app event log records every
  action and the real `tailscaled` error output, syntax-highlighted by level
  (`ERROR` red, `INFO` green, `WARN` yellow). Tail it at the bottom or pop the
  full scrollable history with `v`.
- **Fast user switching.** Manage your Tailscale profiles right in the UI
  (`l`) — switch, add a login, remove, or log out, all live via
  `tailscale switch`.
- **Interactive operator & connection control.** Toggle your tailnet connection
  (`c` → `tailscale up`/`down`) or fix operator permissions (`O` →
  `sudo tailscale set --operator`) with the auth/password prompt handled
  inline.
- **Exit nodes & subnet routes at a glance.** One-key exit-node toggling (`x`),
  advertised-route inspection (`e`), and a priority-sorted node list (exit
  nodes → subnet routers → online → offline). The LOCAL_NODE panel's **Exit
  State** reads the real-time route status — `DIRECT` or `RELAY` — of the
  *active* exit node connection (the peer all traffic is routed through), or
  `N/A` when no exit node is active.

## Installation

> Public release path (placeholder until the repository is published):

```bash
go install github.com/Phundahl/tailtui@latest
```

Or build from source:

```bash
git clone https://github.com/Phundahl/tailtui
cd tailtui
go build -o tailtui .
./tailtui
```

**Requirements:** Go 1.26+, a working [Tailscale](https://tailscale.com)
install (the `tailscale` CLI on your `PATH`, daemon running), and a terminal
with a [Nerd Font](https://www.nerdfonts.com/) for the node glyphs.

## Keybindings

| Key | Action |
| :-- | :-- |
| `j` / `k`, ↑ / ↓ | Navigate the peer list (wraps at top/bottom) |
| `/` | Search / fuzzy-filter. While typing: ↑↓ or `Ctrl+j`/`Ctrl+k` navigate |
| `Enter` / `Esc` | Apply the filter (blur the box); `Esc` again clears it |
| `c` | Connect / disconnect the local node (`tailscale up`/`down`) |
| `x` | Toggle the highlighted peer as the active exit node (exit-capable peers only) |
| `e` | Expand a subnet router's advertised routes |
| `v` | Open / close the full event-log overlay |
| `l` | Account management — switch · add · remove · logout |
| `O` | Operator setup (`sudo tailscale set --operator=$USER`) |
| `?` | Toggle the help overlay |
| `q` / `Ctrl+c` | Quit |

## Theming

`tailTUI` ships with a sharp, neon-green-on-near-black **"Matrix Core"** palette
and automatically adopts your system **[Omarchy](https://omarchy.org)** theme
when present (read from `~/.config/omarchy/current/theme/colors.toml`). Point
the `TAILTUI_THEME` environment variable at any compatible `colors.toml` to
override the path. All colors are TrueColor and degrade gracefully to the
nearest ANSI color on terminals without 24-bit support.

## Status

`tailTUI` is in active development. The node list, details, latency graphs,
routes, logs, exit-node control, connection toggle, and account management are
all wired to live Tailscale data. Planned: in-UI SSH and ping-as-action.

## Acknowledgments

This project was designed and directed by Patrick Hundahl, with AI-assisted code
generation (Claude / Gemini) used to rapidly prototype and build the Bubble Tea
interface.

## License

Released under the [MIT License](LICENSE). © 2026 Patrick Hundahl.
