# tailscaleTUI

A lightweight, keyboard-driven **terminal UI for Tailscale**, with a high-density
"Matrix Core" aesthetic. Built on the [Charmbracelet](https://charm.sh) stack.

> **Status:** early development. The node list, details pane, local-node
> dashboard, **live latency graph** (`tailscale ping`), and subnet-routes modal
> all reflect **live Tailscale data**, refreshed every few seconds. The
> exit-node toggle drives the real daemon (`tailscale set`), and a real
> in-app event log (capped, scrollable via `[v]`) records actions and errors.
> Account switching is still mocked. See the roadmap below.

```
 TAILSCALE_TUI_V1.0                                  (q)uit (?)help
┌─┤ LOCAL_NODE ├────────────┐ ┌─┤ PEER DETAILS: dc-subnet-01 ├───┐
│ User / Host / IPs / State │ │ IDENTITY · OS · IP · Conn · Tags │
│ Exit / Exit Latency       │ │ [e] 12 advertised routes         │
└───────────────────────────┘ └──────────────────────────────────┘
┌─┤ FILTER NODES ├──────────┐ ┌─┤ LATENCY HISTORY ├──────────────┐
│ ❯ 󰖟 [EXIT] amsterdam-exit●│ │     ██      ▄▄                   │
│   󰒄 [ROUT] dc-subnet-01  ●│ │ ▂▂▂▂██▅▅████████▇▇               │
│   󰌽 peer-dev-box         ●│ │ ████████████████████            │
│   󰌽 omarchy             ●│ ├─┤ TERMINAL_LOGS ├────────────────┤
│   ...                     │ │ > 14:55 [INFO] exit node set     │
└───────────────────────────┘ └──────────────────────────────────┘
 [j/k] Nav  [/] Search  [x] Exit  [O] Operator  [v] Logs ● CONNECTED
```

## Tech stack

- **Go**
- [bubbletea](https://github.com/charmbracelet/bubbletea) — Elm-style TUI runtime (Model/Update/View)
- [bubbles](https://github.com/charmbracelet/bubbles) — `list` for the peer pane (navigation + fuzzy filter) and `viewport` for scrollable overlays
- [lipgloss](https://github.com/charmbracelet/lipgloss) — layout & styling
- [x/ansi](https://github.com/charmbracelet/x) — ANSI-aware compositing for the floating modals
- [go-toml/v2](https://github.com/pelletier/go-toml) — parsing the Omarchy system theme
- `os/exec` + `encoding/json` — live data from the `tailscale status --json` CLI (`internal/tailscale`)

The UI ships a **theme engine**: a default **"Matrix Core"** TrueColor palette
(sharp/brutalist, neon-green on near-black) that automatically adopts your
**Omarchy** system theme when present. Hex colors degrade gracefully to the
nearest ANSI color on terminals without 24-bit support.

## Features

Working today:

- **Live Tailscale data** — the local node, peer list, and details pane are populated from `tailscale status --json` and auto-refresh every 4s; the fetch runs off the UI thread (an async `tea.Cmd` under a timeout) so the interface never blocks. Your selection is preserved across refreshes and a daemon hiccup leaves the last good data on screen (with a red error line in the logs pane)
- **Live latency graph** — the highlighted node is pinged every 2s (`tailscale ping`) off-thread; real round-trip times feed the Unicode bar graph and the MIN/AVG/MAX labels. Each node keeps its own rolling history, so moving between nodes shows live, per-node latency
- **Live subnet routes** — the routes overlay (`e`) lists a node's real advertised/approved subnets (gateway + live latency + routing status) straight from the daemon
- **Real exit-node control** — `x`/`t` sets or clears the active exit node via `tailscale set --exit-node=…`; the list updates instantly and the next poll reconciles with the daemon's true state (no flicker), surfacing any error in the logs pane
- **Priority-sorted peer list** — exit nodes, then subnet routers, then online peers, then offline peers (each alphabetical), so the nodes you act on stay at the top
- **Event log** — a capped (500-entry, FIFO) in-app log records exit-node actions, the real `tailscale set` error output, and connectivity changes. The bottom pane tails the latest line; `[v]` opens the full scrollable history in an opaque `─┤ TERMINAL_LOGS ├─` modal
- **Exit Latency** — the dashboard shows live ping latency to the active exit node (the node your traffic routes through), or `N/A` when none is set
- **Optimized layout** — left column: your local node over the (flex) node list; right column: peer details over a tall multi-row latency chart that grows to fill the space, over the log tail. Subnet routers show an `[e] N advertised routes` hint. Borders stay sharp and flush at any terminal size
- **Operator setup (`O`)** — if Tailscale reports `checkprefs access denied`, press `O` to drop to the terminal and run `sudo tailscale set --operator=$USER` (password prompt and all), then the TUI restores itself and refreshes
- Three-pane responsive layout (local dashboard, peer list, details) that resizes with the terminal
- Peer list with `j/k`/arrow navigation (wrap-around at the top/bottom) and `/` fuzzy filtering (by hostname and tags)
- Details pane that updates instantly as you highlight different nodes
- **Exit node toggle (`x`)** — set/clear the active exit node (only on exit-capable nodes), with a yellow `󰖟 EXIT` chip on the list row and a live `Exit:` field in the dashboard
- **Subnet routes** — the details pane shows a peer's advertised routes (first 5, with a "+N more" hint); press `e` for a scrollable overlay of the full list
- **Floating overlays** — true modals for help (`?`) and routes (`e`) that composite over the still-visible background; while open, `j/k` scroll only the overlay and `esc`/`q` close it
- Exit nodes and subnet routers sorted to the top of the list
- Node-type glyphs (exit / subnet / OS), online/offline indicators, and a color-graded latency graph (faint → accent → warning → error by latency)
- Native theme integration — adopts the system **Omarchy** palette (falls back to the built-in Matrix Core theme)
- Terminal log pane and status/help bars
- Sharp/brutalist styling — single-line panes with titles in the top border, a bright border on the focused pane, and a `❯` surface-bright list selection

Still mocked / not yet implemented:

- Account switching (`l`) still uses sample data (the log seed is a couple of sample lines, then real events take over)
- Node actions: SSH (`s`) and ping-as-action (`p`)
- Local-node self-latency (no meaningful self-RTT; the dashboard shows *exit-node* latency instead, and the live graph is always the selected *peer's*)

## Getting started

Requirements: Go (1.26+), a working **Tailscale** install (the `tailscale` CLI on
your `PATH`, with the daemon running and logged in), and a terminal with a
[Nerd Font](https://www.nerdfonts.com/) for the node glyphs.

```bash
git clone https://github.com/Phundahl/tailscaleTUI
cd tailscaleTUI
go run .
```

### Keybindings

| Key | Action |
| :-- | :-- |
| `j`/`k`, ↑/↓ | Navigate the peer list (wraps around at top/bottom) |
| `/` | Search / filter nodes (`esc` to cancel) |
| `x` / `t` | Toggle the highlighted peer as the active exit node — runs `tailscale set --exit-node` (exit-capable only) |
| `e` | Expand a subnet router's advertised routes (overlay) |
| `v` | Open/close the full event-log overlay (`j/k` scroll) |
| `O` | Operator setup — runs `sudo tailscale set --operator=$USER` interactively |
| `?` | Open/close the help overlay |
| `esc` | Close the active overlay |
| `q` / `ctrl+c` | Quit (or close an overlay) |

Press `q` or `ctrl+c` to quit.

### Theming

The default **Matrix Core** theme is a sharp, neon-green-on-near-black palette. The app also
reads the native **Omarchy** (Aether) system theme automatically from
`~/.config/omarchy/current/theme/colors.toml` (override the path with the
`TAILSCALE_TUI_THEME` env var). That file is a flat TOML palette:

```toml
accent     = "#509475"
foreground = "#C1C497"
background = "#111c18"
color0  = "#23372B"   # ... through ...
color15 = "#9eebb3"
```

Mapped onto the theme as: `accent`→primary accent, `color2`→secondary,
`background`→background, `foreground`→text, `color8`→inactive borders/dim text,
`color3`→warning, `color1`→error. Any missing key, or a missing/malformed file,
falls back to the corresponding Matrix Core default — it never crashes.

### Build

```bash
go build -o tailscaleTUI .
./tailscaleTUI
```

## Project layout

```
main.go                entry point
internal/types         domain models (Peer, LocalStatus, enums)
internal/tailscale     live adapter: `tailscale status --json` → domain models
internal/mock          sample data (logs/accounts) + synthetic latency
internal/styles        theme engine (theme.go) + lipgloss styling (styles.go)
internal/tui           Bubble Tea Model/Update/View + peer list + async polling
design-spec.md         authoritative UI/UX specification
```

## Roadmap

- [x] Phase 1 — layout skeleton + mock data
- [x] Phase 2 — interactive peer list, filtering, dynamic details
- [x] Phase 2 refinement — exit node toggle & indicators
- [x] Phase 2 completion — strict exit logic, subnet routes, help & routes overlays
- [x] Phase 4 — theme engine (TrueColor + Omarchy), color-graded latency graph
- [x] Phase 5 — Matrix Core master design (sharp panes, surface tonal depth, tabular modals)
- [x] Phase 6 — real data integration: live `tailscale status --json` adapter + async 4s polling
- [x] Phase 7 — live latency (`tailscale ping`), live routes modal, exit-node action wiring (`tailscale set`), priority sort
- [x] Phase 8 — event log system (FIFO ring + `[v]` overlay), footer fix, Exit Latency readout
- [x] Phase 9 — layout reorg (multi-row latency chart), flush-border fix, `[O]` sudo operator setup
- [x] Phase 10 — final two-column grid + restored subnet-router routes hint
- [ ] Remaining node actions (SSH, ping-as-action)
- [ ] Live account switching (`tailscale switch`)
