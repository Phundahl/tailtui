# tailscaleTUI

A lightweight, keyboard-driven **terminal UI for Tailscale**, with a high-density
"Matrix Core" aesthetic. Built on the [Charmbracelet](https://charm.sh) stack.

> **Status:** early development. The UI runs against **mock data** — it does not
> yet talk to the real `tailscale` CLI. See the roadmap below.

```
┌ TAILSCALE_TUI_V1.0 ──────────────────────────── (q)uit (?)help ┐
│ LOCAL_NODE                  │ PEER DETAILS: amsterdam-exit      │
│ User / Host / IPs / State   │ IDENTITY · OS · IP · Conn · Tags  │
│ ─────────────────────────── │ LATENCY HISTORY ▂▄▆▃▁▄█▄          │
│ NODES                       │                                   │
│ > 󰖟 [EXIT] amsterdam-exit ● │                                   │
│   󰒄 [ROUT] dc-subnet-01   ● │                                   │
│   󰌽 peer-dev-box          ● │                                   │
├─────────────────────────────┴───────────────────────────────────┤
│ TERMINAL_LOGS                                                    │
└ [j/k] Nav  [/] Search  [s] SSH  [p] Ping  [t] Connect  [q] Quit ─┘
```

## Tech stack

- **Go**
- [bubbletea](https://github.com/charmbracelet/bubbletea) — Elm-style TUI runtime (Model/Update/View)
- [bubbles](https://github.com/charmbracelet/bubbles) — `list` for the peer pane (navigation + fuzzy filter) and `viewport` for scrollable overlays
- [lipgloss](https://github.com/charmbracelet/lipgloss) — layout & styling
- [x/ansi](https://github.com/charmbracelet/x) — ANSI-aware compositing for the floating modals
- [go-toml/v2](https://github.com/pelletier/go-toml) — parsing the Omarchy system theme

The UI ships a **theme engine**: a default **"Matrix Core"** TrueColor palette
(sharp/brutalist, neon-green on near-black) that automatically adopts your
**Omarchy** system theme when present. Hex colors degrade gracefully to the
nearest ANSI color on terminals without 24-bit support.

## Features

Working today (mock data):

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

Not yet implemented:

- Real Tailscale integration (live `tailscale status`, actions)
- Node actions: SSH (`s`), ping (`p`), connect toggle (`t`), expand routes (`e`), account management (`l`), help overlay (`?`)

## Getting started

Requirements: Go (1.26+) and a terminal with a [Nerd Font](https://www.nerdfonts.com/)
for the node glyphs.

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
| `x` | Toggle the highlighted peer as the active exit node (exit-capable only) |
| `e` | Expand a subnet router's advertised routes (overlay) |
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
internal/mock          hardcoded sample data (temporary)
internal/styles        theme engine (theme.go) + lipgloss styling (styles.go)
internal/tui           Bubble Tea Model/Update/View + peer list
design-spec.md         authoritative UI/UX specification
```

## Roadmap

- [x] Phase 1 — layout skeleton + mock data
- [x] Phase 2 — interactive peer list, filtering, dynamic details
- [x] Phase 2 refinement — exit node toggle & indicators
- [x] Phase 2 completion — strict exit logic, subnet routes, help & routes overlays
- [x] Phase 4 — theme engine (TrueColor + Omarchy), color-graded latency graph
- [x] Phase 5 — Matrix Core master design (sharp panes, surface tonal depth, tabular modals)
- [ ] Remaining node actions (SSH, ping, connect toggle, accounts)
- [ ] Real `tailscale status --json` data adapter
