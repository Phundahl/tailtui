# tailTUI theme template for Omarchy
#
# Install:
#   mkdir -p ~/.config/omarchy/themed
#   cp contrib/tailtui.toml.tpl ~/.config/omarchy/themed/
#
# Omarchy renders this into the active theme directory as `tailtui.toml` every
# time you switch themes, and tailTUI picks it up within a few seconds — no
# restart. Without it, tailTUI reads the theme's own `colors.toml` directly and
# maps the palette itself, which needs no setup at all.
#
# The point of installing it is control: you decide which palette slot drives
# which part of the UI, per theme, instead of tailTUI guessing. See below for
# where that matters.
#
# Every {{ placeholder }} is substituted by Omarchy from the current theme.
# Available names include the semantic slots (accent, background, foreground,
# selection, muted, red, green, yellow, orange, blue, cyan, magenta, and the
# bright_/dark_/light_ variants), the legacy terminal palette (color0-color15),
# and `mode`. Modifiers: {{ x_strip }} drops the leading #, {{ x_rgb }} gives
# decimal "r,g,b", and {{ mix a b 30% }} blends two colors.
#
# Run `omarchy-theme-color --file ~/.local/state/omarchy/current/theme/colors.toml --all`
# to print every name your current theme defines.

# "dark" or "light". Controls which direction panels are shaded relative to the
# background: a light theme shades its surfaces down, a dark theme lifts them up.
mode = "{{ mode }}"

# Focus, borders on the active pane, buttons, key hints.
primary = "{{ accent }}"

# Online nodes, approved routes, success chips.
secondary = "{{ green }}"

# The base canvas.
background = "{{ background }}"

# Elevated panels and modals — one step away from the canvas. On a light theme
# swap this for {{ dark_background }} so panels shade downward instead of up.
surface = "{{ lighter_background }}"

# The selected row's highlight bar.
surface_bright = "{{ selection }}"

# Unfocused pane borders and dividers.
border = "{{ muted }}"

# Body text.
text = "{{ foreground }}"

# Labels, timestamps, secondary text.
text_dim = "{{ muted }}"

# Exit nodes, relayed connections, elevated latency.
#
# `yellow` is the obvious choice, but some themes define it as something that
# collides with `green` — osaka-jade sets yellow to #459451, which would make
# exit-node markers nearly indistinguishable from the online color. If that
# happens in your theme, use {{ orange }} here instead. This is exactly the
# judgement call a template exists to hand back to you.
warning = "{{ yellow }}"

# Conflicts, failures, critical latency.
error = "{{ red }}"
