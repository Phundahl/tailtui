# Changelog

Every released version of `tailTUI`, newest first.

The project began on 2026-05-29; anything before v1.1.0 lives in the commit log
rather than here. Dates are the release dates.

## v1.5.0 — 2026-09-25

*Serve & Funnel*

- **Share a local service from the TUI.** `F` opens Serve & Funnel: `a` adds a
  port, directory or text, `d` removes it, and `Space` switches a port between
  tailnet-only and public. Every change is previewed as the exact command
  before it runs.
- **The confirmation names what you are exposing, and to whom.** A directory is
  inspected first, so sharing one that holds `.ssh` or `.env` says so in as many
  words; going public lists every path on that port, because Funnel is a
  property of the port rather than the path. It warns — it never refuses.
- **Getting back to private is spelled out.** A public entry is flagged under
  its URL and the key is labelled `MAKE PRIVATE`, not a generic toggle, because
  undoing exposure should be the most obvious thing in the view.
- **`c` copies the entry's real URL** — built from the daemon's own hostname, so
  it is the link you can actually send to someone.
- **`r` re-reads the daemon and logs what it found**, with counts, so you can
  confirm a change landed instead of trusting the screen.
- **A live Funnel no longer floods the node list.** Tailscale adds roughly two
  dozen of its own ingress nodes while a funnel is up; they are kept out of the
  list, which says how many it is hiding, and `/` still finds them.
- **Anything public is flagged on the dashboard**, because the real hazard is
  forgetting a Funnel is still running.

## v1.4.0 — 2026-09-17

*SSH launcher*

- **SSH straight from the node list.** Highlight a peer, press `s`, and a
  launcher shows the target, an editable username, and the *exact* command
  about to run. `Enter` hands your terminal to `tailscale ssh` — MagicDNS
  resolution, host-key verification and auth all handled by the wrapper — and
  `tailTUI` restores itself the moment the session ends. `c` copies the command
  instead of running it.
- **Nodes running Tailscale SSH are marked.** The peer list flags which nodes
  advertise an SSH server, read live from the daemon at no extra cost. The
  launcher still opens on *any* online peer, though — a node without Tailscale
  SSH may well be reachable through its own `sshd`, and the modal tells you
  which case you are in before anything runs.
- **The username is remembered for the session.** It starts as your local user
  and, once you change it, stays changed until you quit — so a non-default
  remote account is typed once, not once per connection.
- **`[O] Operator` retires once it is done.** Operator setup is a one-time
  step, so the footer hint now disappears after it has been granted, freeing
  the space for `[s] SSH`.

## v1.3.0 — 2026-08-21

*Live theme switching*

- **Live theme switching.** Switch your desktop theme and a running `tailTUI`
  re-colors itself within a few seconds — no restart, and it works with a modal
  open. Implemented on the existing refresh tick, so there is no file-watcher
  dependency and no extra background work.
- **A theme format of tailTUI's own.** A `tailtui.toml` with one key per UI role
  (`primary`, `surface`, `warning`, …) now takes precedence over the raw Omarchy
  palette. Write it by hand on any distro, or generate it from the bundled
  Omarchy template.
- **Ambiguous mappings are settled explicitly.** Some palettes cannot be mapped
  automatically: a theme may define its `yellow` as a green, which would leave
  exit-node markers nearly indistinguishable from the online color. A template
  resolves cases like that in one line, per theme, instead of `tailTUI` guessing.

None of this is required: with no template installed, `tailTUI` reads the
theme's `colors.toml` and maps it itself, exactly as before.

## v1.2.0 — 2026-08-20

*Omarchy 4, light themes, prebuilt binaries*

- **Omarchy 4 ("Quattro") theme support.** Omarchy 4 moved the current-theme
  store to `~/.local/state/omarchy/` *and* replaced the flat `color0`–`color15`
  palette with semantic slots. tailTUI now probes both locations and reads both
  schemas — detected by the keys present, not the filename — so upgraded and
  older installs alike keep tracking the desktop theme instead of silently
  falling back to the built-in palette.
- **Light theme support.** The theme's `mode` key is honored, shading panels and
  modals in the right direction, so light palettes like `catppuccin-latte`,
  `flexoki-light`, and `white` render as a light UI rather than an inverted one.
- **Demo / screenshot mode.** `TAILTUI_MOCK=1` runs the whole UI against an
  in-memory fictional tailnet — every pane, modal, and animation, with no
  daemon, no network, and no risk of touching a real configuration.
- **Account management is unprivileged-aware.** The four profile actions
  (`a`/`Enter`/`d`/`l`) elevate properly for the root-owned Linux profile store,
  and a session without access shows a clear "profile store locked" hint inside
  the modal instead of quietly repeating errors into the log.
- **Prebuilt binaries.** Releases are built and published with goreleaser as
  `.tar.gz`, `.deb`, and `.rpm` for amd64 and arm64, and the version shown in
  the footer is stamped from the release tag.

## v1.1.0 — 2026-06-05

*Routing Management, Command Room, Advanced Settings*

tailTUI grew from a read-only dashboard into a full configuration tool:

- **Advanced Settings modal (`S`).** Toggle live local-node preferences —
  accept-routes, exit-node LAN access, Tailscale SSH, MagicDNS, and shields-up —
  each driving the real `tailscale set --<flag>`, with optimistic updates that
  reconcile against the daemon. Operator setup (`O`) is built in. The settings
  hotkey moved to uppercase `S`, keeping the lowercase keys free (search stays on
  `/`).
- **Routing Management (`R`).** Stage advertised exit-node and subnet-route
  changes locally: add routes through a CIDR field validated with
  `net.ParseCIDR`, remove them, or pop a just-deleted route back via a smart
  pre-fill **undo** (`d` then `a`).
- **The "Command Room."** Before anything is applied, a transparent confirmation
  overlay shows the exact `tailscale set …` command, so there's never a hidden
  mutation — plus a reminder that routes/exit nodes still need Admin Console
  approval.
- **Clipboard integration.** Copy the generated command straight to the system
  clipboard (`c`), asynchronously so the UI never blocks.
