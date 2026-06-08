# tailTUI VHS tape — anonymized demo recording for the README.
#
# Requires:
#   - charmbracelet/vhs on PATH:  go install github.com/charmbracelet/vhs@latest
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

# Account Management showcase — open, hold so the polished layout reads, close.
Type "l"
Sleep 3s
Escape
Sleep 800ms

# Quit cleanly.
Type "q"
Sleep 400ms
