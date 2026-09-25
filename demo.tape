# tailTUI VHS tape — anonymized demo recording for the README.
#
# Requires:
#   - charmbracelet/vhs on PATH:  go install github.com/charmbracelet/vhs@latest
#   - ttyd, which vhs shells out to (Arch: pacman -S ttyd). ffmpeg too.
#   - the binary built in this directory:  go build -o tailtui .
#
# Render with:
#   TAILTUI_MOCK=1 vhs demo.tape          # writes assets/demo.gif
#
# The env var MUST be set in the invoking shell — VHS's own `Set Env` directive
# does not accept this form and is intentionally not used here. With
# TAILTUI_MOCK=1 every adapter call is routed to the in-memory fixture in
# internal/tailscale/mock.go: no real `tailscale` CLI is invoked, no Tailnet
# topology is read, no sudo prompts can fire. Safe to record in CI / on a
# fresh box.

Output assets/demo.gif

Set Shell      "bash"
Set FontSize   14
Set Width      1280
Set Height     720
Set Padding    16
Set Framerate  24
Set TypingSpeed 60ms

# Launch the app silently so the recording opens on the populated dashboard,
# not on a "$ ./tailtui" prompt.
Hide
Type "./tailtui"
Enter
Sleep 1500ms
Show

# Hold on the dashboard so the viewer sees the live latency graph + node list.
Sleep 2500ms

# Walk the peer list — fictional nodes (exit-node-se, dc-subnet-router,
# home-nas, srv-web-01, db-cluster-prod, dev-macbook, field-laptop).
Down 2
Sleep 800ms
Down 2
Sleep 800ms
Down 1
Sleep 800ms
Up 2
Sleep 1s

# Advanced Settings — local-pref toggles (Shift+S).
Type "S"
Sleep 2500ms
Down 2
Sleep 800ms
Down 1
Sleep 800ms
Escape
Sleep 600ms

# Routing Management — advertised exit-node + subnet routes (Shift+R).
Type "R"
Sleep 2500ms
Down 1
Sleep 800ms
Down 1
Sleep 800ms
Escape
Sleep 600ms

# Serve & Funnel (Shift+F) — what this node shares, and with whom.
# The fixture has a PUBLIC :443 proxying two paths, and a tailnet-only :8443
# serving a directory and a literal string, so the tree shows both scopes and
# all three target kinds. The cursor opens on the public port, so the red
# PUBLIC banner and the green [SPACE] MAKE PRIVATE label are on screen at once.
Type "F"
Sleep 3s

# Move to the tailnet-only :8443 — the keymap flips to MAKE PUBLIC.
Down 3
Sleep 2s

# Space opens the Command Room. Nothing is applied: the confirmation names the
# port, lists every path that would become reachable from the internet, and
# Escape backs out of it. This is the whole safety story in one beat.
Space
Sleep 3500ms
Escape
Sleep 1s
Escape
Sleep 600ms

# SSH launcher on srv-web-01 (the node highlighted after the walk above) — preview
# the exact command, retype the user (Ctrl+U clears; [e] opens with the cursor at the end),
# then cancel.
# Safe under TAILTUI_MOCK=1: the mock branch never spawns a real ssh.
Type "s"
Sleep 2500ms
Type "e"
Sleep 800ms
Ctrl+U
Sleep 400ms
Type "deploy"
Sleep 1s
Enter
Sleep 1500ms
Escape
Sleep 600ms

# Account Management showcase — open, hold so the polished layout reads, close.
Type "l"
Sleep 3s
Escape
Sleep 800ms

# Quit cleanly.
Type "q"
Sleep 400ms
