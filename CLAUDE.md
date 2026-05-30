# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A lightweight, keyboard-driven terminal UI ("Tailscale TUI", codename "Matrix Core") that wraps Tailscale, built on the Charmbracelet stack. `design-spec.md` is the authoritative source for layout, interactions, and styling. The project is built **iteratively in numbered phases**, each ending with user approval before the next begins.

Module path: `github.com/Phundahl/tailscaleTUI`.

## Commands

```bash
go run .              # run the TUI (needs a real terminal / TTY)
go build ./...        # compile everything
go vet ./...          # static checks
go test ./...         # tests (none committed yet)
go test ./internal/tui/ -run TestName -v   # run a single test
```

The View layer is testable without a TTY: construct `tui.New()`, send a `tea.WindowSizeMsg`, then call `Update`/`View` directly (see how the phase smoke tests were written). Prefer this over launching the program when verifying render/logic changes.

## Standing constraints (do not violate without being asked)

- **Mock data only.** No real `tailscale` CLI calls or JSON parsing yet — all data comes from `internal/mock`. A real `tailscale status --json` adapter is a later phase; when it lands it must map into the existing `internal/types` structs, not replace them.
- **Theme Engine (no more strict-ANSI rule).** Colors are TrueColor hex, centralized in a `styles.Theme` struct (`internal/styles/theme.go`). `DefaultTheme()` is the "Stitch" neon-on-near-black aesthetic. `LoadTheme()` reads the **native Omarchy** (Aether-managed) theme from `~/.config/omarchy/current/theme/colors.toml` — a flat TOML table of hex strings (`accent`, `foreground`, `background`, `color0`–`color15`) parsed with `github.com/pelletier/go-toml/v2`. The path is overridable via `TAILSCALE_TUI_THEME`. Mapping is per-field, so any missing key keeps its Stitch default and a missing/malformed file falls back entirely — it never crashes or leaves blanks. Omarchy→Theme: `accent`→PrimaryAccent, `color2`→SecondaryAccent, `background`→Background, `foreground`→TextNormal, `color8`→BorderInactive & TextDim, `color3`→Warning, `color1`→Error. `main` calls `styles.Apply(styles.LoadTheme())` at startup. `Apply` rebuilds the package-level color vars (`Primary`, `Secondary`, `Subtle`, `Warn`, `Danger`, `Fg`, `BorderInactive`) and derived `lipgloss.Style` vars from the theme; helper funcs (`Box`, `Divider`, `LatencyGraph`, …) read those vars at call time, so a different theme needs no other changes. Add new colors as `Theme` fields, not raw codes. (Hex degrades to the nearest ANSI on non-TrueColor terminals, so theme adaptivity is preserved.) The theme also has a `Background` field, but the app renders on the terminal's native background — see the note in `styles.go` for why a global fill is intentionally avoided.
- **Elm architecture.** Standard Bubble Tea Model/Update/View; keep them split across `internal/tui/{model,update,view}.go`.

## Architecture

```
main.go                     entry point: tea.NewProgram(tui.New(), WithAltScreen)
internal/
  types/types.go            domain models (Peer, LocalStatus, enums) — CLI-agnostic
  mock/mock.go              hardcoded sample data (swapped for a tailscale adapter later)
  styles/theme.go           Theme struct, DefaultTheme (Stitch), Omarchy TOML loader (LoadTheme)
  styles/styles.go          theme-derived styles (Apply), Box/Divider/Bar/LatencyGraph helpers
  tui/
    model.go                Model struct, New(), Init(), selectedPeer()
    update.go               Update(): resize + quit; everything else delegated to the list
    peerlist.go             bubbles/list construction, custom 1-line delegate, sort logic
    overlay.go              help/routes modal state machine + floating-modal compositor (x/ansi)
    view.go                 View(): computeLayout() + pane rendering + routesSummary
```

Key design points that span multiple files:

- **`types` is decoupled from any Tailscale wire format on purpose.** `Peer`/`LocalStatus` use semantic enums (`ConnType`, `NodeType`, `OS`) so the view picks glyphs/colors by switching on a type, never by string-matching CLI output. The future adapter maps wire data → these structs.
- **The peer list drives the details pane.** The middle pane is a `bubbles/list.Model`; `Model.selectedPeer()` reads `list.SelectedItem()`, and `View` renders that peer on the right every frame. There is no separate "selected index" state to keep in sync — the list is the single source of truth for selection.
- **`peerDelegate` renders one dense line per node** (`> 󰖟 [EXIT] amsterdam-exit ●`) instead of the default two-line delegate, to match the spec's high-density list. Exit nodes and subnet routers are stable-sorted to the top via `nodeRank`.
- **Filtering vs. quit/commands.** `/` enters the list's fuzzy filter (over hostname + tags via `Peer.FilterValue`). `Update` must NOT treat command keys (`q`, `x`, `j`/`k`, …) as commands while `list.FilterState() == Filtering`, or it would swallow the keystroke — they're literal filter text then. `ctrl+c` always quits.
- **Wrap-around navigation.** `wrapNav` intercepts the single-step nav keys (`j`/`k`, ↓/↑) and jumps to the opposite end *only* when already at a boundary (top→bottom, bottom→top); otherwise it returns `handled=false` and the key falls through to the list so normal movement and pagination are untouched. Boundaries are computed over `list.VisibleItems()` (the filtered subset) using `Index()`/`Select()`, so wrap respects an active `/` filter. Note: bubbles computes filtering asynchronously via a `tea.Cmd`, so this path can only be exercised end-to-end under the real tea runtime, not by calling `Update` in isolation.
- **Exit node state lives in the list items, not in `LocalStatus`.** `Peer.IsActiveExitNode` is the single source of truth; at most one peer has it set. `Peer.OffersExitNode` is a *capability* flag — `toggleExitNode` (`x`) is a no-op unless the highlighted peer offers exit-node service. When it does toggle, it rewrites every peer via `list.SetItem` — enabling the highlighted one and clearing the rest, or clearing all if it was already active. The dashboard's `Exit:` value is *derived* via `Model.activeExitNodeName()` (scans the list), so it stays in sync automatically; `LocalStatus.ExitNode` is left empty on purpose. The active node is marked in the list with a soft yellow `󰖟 exit` text label (`styles.ExitName`) — a minimal marker, not a filled chip.
- **Advertised routes.** `Peer.AdvertisedRoutes` holds subnet CIDRs (subnet routers only). The details pane (`routesSummary`) shows at most 5, then a yellow `[+N more routes... Press 'e' to expand]` hint. Pressing `e` on a peer that has routes opens the routes overlay.
- **Overlay state machine.** `Model.state` (`stateMain`/`stateHelp`/`stateRoutes`) gates input and rendering. `internal/tui/overlay.go` owns it: `openHelp`/`openRoutes` size a shared `bubbles/viewport` and set its content; `updateOverlay` handles keys while an overlay is open — `esc`/`q` (and `?` for help) close it, everything else (`j/k`, arrows, page keys) scrolls *only* the viewport. The crucial isolation rule lives in `Update`: when `state != stateMain`, keys are routed to `updateOverlay` and never reach the peer list. Overlays re-size on `WindowSizeMsg` via `resizeOverlay`.
- **Floating modals (compositing).** `View` always renders the full base layout first, then `renderOverlay(base)` blits the modal on top so the dashboard/list/details stay visible behind it. lipgloss v1 has no overlay primitive, so `overlayCenter` does it by hand: for each background row covered by the modal it keeps the left columns (`ansi.Truncate`), splices in the modal line, then resumes the background past the modal (`ansi.TruncateLeft`) — both are ANSI-aware and carry SGR state across the cut, with explicit `\x1b[0m` resets isolating the three segments. This is why `internal/tui` depends directly on `github.com/charmbracelet/x/ansi`.
- (Modal opacity is covered under "Opaque modals" below — the modal now uses a solid theme-`Background` fill; `overlayHeight` sizes the viewport to its content so there are no ragged padding rows.)
- **Layout is centralized in `computeLayout`.** Both `Update` (which sizes the list via `SetSize`) and `View` (which draws) read the same geometry, so they never disagree. `WindowSizeMsg` is intercepted (not forwarded to the list) so the list gets the *pane* size, not the *terminal* size. Box math: `styles.Box`/`styles.BoxFocused` render to exact OUTER w×h (rounded border + `Padding(0,1)` included); use `styles.ContentWidth(outer)` to get the usable inner width (outer − border − padding) when sizing pane content and the list.
- **Border = focus, color = state.** Pane borders signal focus: the peer-list pane uses `styles.BoxFocused` (bright `Primary` border), the details/logs panes use `styles.Box` (`Subtle` gray border). The selected list row is marked by a colored left-gutter `│` (`styles.SelectBar`) + bold accent text (`styles.Selected`) — no full-width background block. Content colors still follow state: accent = online/direct/routes, warning = exit node/relayed, error = conflict, dim = offline/secondary. `●`/`○` mark online/offline.
- **Latency graph.** The details pane uses `styles.LatencyGraph`: bar *height* is scaled to the series min/max (so the shape reads well at any range) while bar *color/weight* encodes absolute latency — faint accent <30ms, solid accent 30–59, bold warning 60–99, bold error ≥100ms. The compact dashboard graph keeps the simpler single-color `styles.Sparkline`. Both pull their colors from the active theme (`Primary`/`Warn`/`Danger`) — never hardcoded.
- **Color consistency audit (theme-purity invariant).** There must be **no hardcoded color anywhere outside `theme.go`** — no hex, no `lipgloss.Color("<n>")`. Every styled element references the theme via the derived `styles.*` vars/styles. Common leak spots and the rule: list glyphs must be colored (`styles.IconOnline`/`IconOffline`), not left raw (a raw glyph inherits the terminal's default fg and silently ignores the theme); both ping graphs read theme colors; modal content uses the `styles.Modal*` surface styles. To verify after a change: `grep -rnE 'lipgloss\.Color\("(#|[0-9])' internal/` must return nothing outside `theme.go`.
- **Opaque modals (no bleed).** The overlay is a fully opaque rectangle painted with the theme `Background`: every content span uses a `styles.Modal*` style that bakes in the background, every line is padded to full width via `modalLine`/`styles.ModalFill`, and the container sets explicit `Width`/`Height` **including** padding plus `Background` + `BorderBackground` so the whole bounding box (padding and border included) overwrites the view behind it. `overlayCenter` then splices it over the base. (Earlier the modal used the terminal default with no fill; with the theme engine the background is a known dark value, so a solid fill is both opaque and not "blinding".)

Before changing any interaction, consult the keybinding matrix and overlay specs in `design-spec.md` — keys are scoped global vs. context (list/modal) and must not collide.

## Phase log

- **Phase 1** — skeleton: Model/Update/View, three-pane responsive lipgloss layout, mock data, ANSI theme.
- **Phase 2** — `bubbles/list` peer pane: navigation (`j/k`/arrows), `/` fuzzy filter, dynamic details pane driven by selection, special-node-first sorting.
- **Phase 2 refinement** — exit node toggle (`x`): `Peer.IsActiveExitNode` flag, single-active invariant, yellow `EXIT` list chip, and dashboard `Exit:` derived from list state.
- **Phase 2 completion** — strict exit-node capability (`OffersExitNode`), advertised routes + details-pane summary (max 5 + "more" hint), and the overlay state machine: help (`?`) and scrollable routes (`e`) modals with keybinding isolation.
- **UI refresh** — minimalist restyle: rounded borders, focus-colored pane borders (bright = focused, subtle = inactive), no-fill modals with bright rounded border + colored title, `Padding(0,1)` breathing room, icon/text spacing, and a gutter-bar list selection (replacing the solid block).
- **Phase 4 — Theme Engine** — dropped the strict-ANSI rule for TrueColor hex; central `styles.Theme` + default "Stitch" palette; refactored all styles through `Apply`; premium per-bar `LatencyGraph` with faint/normal/bold-warning/bold-error color nuance.
- **Phase 4.1 — Native Omarchy theme** — loader now parses the system Omarchy TOML (`~/.config/omarchy/current/theme/colors.toml`, go-toml/v2), mapping `accent`/`foreground`/`background`/`color0-15` onto the `Theme` struct, with graceful per-field fallback to Stitch.
- **Phase 4.2 — Theme-purity audit + opaque modals** — confirmed zero hardcoded colors; themed the previously-raw list glyphs (`IconOnline`/`IconOffline`); made help/routes modals a fully opaque theme-`Background` fill so nothing bleeds through.
- **Upcoming** — remaining node actions (`s` SSH, `p` ping, `t` toggle, `l` accounts), then the real Tailscale data adapter.

## Keybindings (implemented)

| Key | Action |
| :-- | :-- |
| `j`/`k`, ↑/↓ | Move selection in the peer list (wraps around at top/bottom) |
| `/` | Fuzzy filter (hostname + tags); `esc` cancels |
| `x` | Toggle highlighted peer as the active exit node (only if it offers exit; off if already active) |
| `e` | Open the routes overlay (only on a peer with advertised routes) |
| `?` | Open/close the help overlay |
| `esc` / `q` | Close the active overlay (in an overlay); `q` quits in the main view (ignored while filtering) |
| `ctrl+c` | Quit (always) |

Keys from the design spec not yet wired: `s` `p` `t` `l`.

## Documentation workflow (required)

At the end of **every** successful phase, automatically update both `CLAUDE.md` (technical decisions, architecture state, constraints) and `README.md` (tech stack, working features, run instructions). This is a standing user requirement, not a per-request ask.
