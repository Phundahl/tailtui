---
name: Matrix Core
colors:
  surface: '#0e150f'
  surface-dim: '#0e150f'
  surface-bright: '#333b34'
  surface-container-lowest: '#09100a'
  surface-container-low: '#161d17'
  surface-container: '#1a211b'
  surface-container-high: '#242c25'
  surface-container-highest: '#2f372f'
  on-surface: '#dde5da'
  on-surface-variant: '#bccabb'
  inverse-surface: '#dde5da'
  inverse-on-surface: '#2b322b'
  outline: '#869486'
  outline-variant: '#3d4a3e'
  surface-tint: '#4de082'
  primary: '#6bfb9a'
  on-primary: '#003919'
  primary-container: '#4ade80'
  on-primary-container: '#005e2d'
  inverse-primary: '#006d36'
  secondary: '#c0c7d3'
  on-secondary: '#2a313b'
  secondary-container: '#404752'
  on-secondary-container: '#afb5c2'
  tertiary: '#ffdd75'
  on-tertiary: '#3c2f00'
  tertiary-container: '#ebbf00'
  on-tertiary-container: '#624f00'
  error: '#ffb4ab'
  on-error: '#690005'
  error-container: '#93000a'
  on-error-container: '#ffdad6'
  primary-fixed: '#6dfe9c'
  primary-fixed-dim: '#4de082'
  on-primary-fixed: '#00210c'
  on-primary-fixed-variant: '#005227'
  secondary-fixed: '#dce3f0'
  secondary-fixed-dim: '#c0c7d3'
  on-secondary-fixed: '#151c25'
  on-secondary-fixed-variant: '#404752'
  tertiary-fixed: '#ffe083'
  tertiary-fixed-dim: '#eec200'
  on-tertiary-fixed: '#231b00'
  on-tertiary-fixed-variant: '#574500'
  background: '#0e150f'
  on-background: '#dde5da'
  surface-variant: '#2f372f'
typography:
  display-header:
    fontFamily: JetBrains Mono
    fontSize: 18px
    fontWeight: '700'
    lineHeight: 24px
  body-main:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: '400'
    lineHeight: 20px
  label-bold:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: '700'
    lineHeight: 16px
  label-dim:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: '400'
    lineHeight: 16px
  mono-data:
    fontFamily: JetBrains Mono
    fontSize: 13px
    fontWeight: '500'
    lineHeight: 18px
spacing:
  cell-padding-x: 1ch
  cell-padding-y: '0'
  container-margin: '1'
  gutter: '2'
---

## Brand & Style
The design system is a high-density, professional Terminal User Interface (TUI) optimized for a Tailscale wrapper. The brand personality is technical, precise, and utilitarian, catering to system administrators and power users who value speed and density over decorative flair. 

The aesthetic draws from **Minimalism** and **Brutalism**, utilizing a structured grid of monospaced characters. It avoids modern abstractions like blurs or rounded corners in favor of raw ASCII-style geometry. The goal is to evoke the reliability of a low-level system utility like `htop` or `k9s`, where information hierarchy is conveyed through color-coded status indicators and rigid containment boxes.

## Colors
The color palette is functional and semantic, designed for high legibility against a deep matte background.

- **Primary (Online):** `#4ADE80` (Green) signifies active connections and healthy nodes.
- **Secondary (Neutral/Offline):** `#9CA3AF` (Gray) is used for inactive states, secondary metadata, and structural borders.
- **Warning:** `#FACC15` (Yellow) alerts the user to expiring keys or high-latency nodes.
- **Error:** `#F87171` (Red) identifies connection failures or authentication errors.

Backgrounds remain strictly dark (`#0F172A`) to reduce eye strain during prolonged CLI sessions, with surfaces (`#1E293B`) used sparingly for highlighted rows or focused panes.

## Typography
This design system exclusively employs **JetBrains Mono** to ensure perfect character alignment and support for Nerd Font symbols. 

- **Headers:** Use uppercase with bold weighting for primary sections.
- **Data Tables:** Maintain a consistent 13px size to maximize information density.
- **Nerd Font Integration:** Use specific glyphs for OS identification: `` (Ubuntu), `󰖟` (Web/Exit Node), `󰒄` (Network), `` (Linux), `` (Windows), `` (macOS).
- **Styling:** Use "Dim" or 50% opacity for non-essential characters like bracket delimiters `[ ]` and pipe separators `|`.

## Layout & Spacing
The layout follows a **Fixed Grid** based on character cells (ch). There are no fluid margins; instead, the UI is divided into "Panes" using ASCII-style box-drawing characters.

- **Density:** High. Elements should be separated by a single character width or height.
- **Alignment:** All numerical data (IP addresses, Latency) must be right-aligned or decimal-aligned to ensure vertical scanability.
- **Breakpoints:** 
  - **Compact (< 80 chars):** Collapse secondary columns (OS version, Last Seen).
  - **Standard (80-120 chars):** Display full node list and basic metadata.
  - **Expanded (> 120 chars):** Enable side-pane for detailed node inspection.

## Elevation & Depth
In this TUI environment, depth is achieved through **Tonal Layers** and **Border Logic** rather than shadows.

- **Base Layer:** The terminal background.
- **Surface Layer:** Use a slightly lighter background color (`#1E293B`) to indicate a focused pane or a selected list item.
- **Borders:** Use single-line box-drawing characters (┌ ─ ┐ │ └ ┘) for standard containers. Use double-line characters (╔ ═ ╗ ║ ╚ ╝) to indicate an active modal or high-priority focus.
- **Contrast Outlines:** Active input fields should use the Primary color for their borders to draw the eye.

## Shapes
The design system strictly adheres to a **Sharp (0)** roundedness level. All containers, buttons, and highlights are rectangular. This ensures the UI feels native to the terminal buffer and maintains the integrity of the character-based grid.

## Components
- **Status Indicators:** Use a solid circle `●` (Primary for Online, Gray for Offline). For critical errors, use a blinking or high-contrast `!`.
- **Bordered Boxes:** Every logical section (Node List, Global Stats, Logs) must be wrapped in a box-drawing frame with a label centered in the top border: `─┤ Node List ├─`.
- **Lists/Tables:** Use row highlighting for navigation. The selected row should have a background color of `#1E293B` and a leading pointer `❯`.
- **Buttons:** Represented as bracketed text `[ Connect ]`. Active buttons use Primary color text; hovered/focused buttons invert the background/foreground.
- **Input Fields:** Displayed as a label followed by an underscored or bordered area: `Search: ________________`.
- **Chips/Tags:** Minimalist tags using brackets, e.g., `[exit-node]` or `[expired]`, color-coded by state.
- **Progress Bars:** Constructed using block characters: `[██████░░░░░░] 50%`.