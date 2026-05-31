package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Pane renders body inside a sharp, single-line box (┌─┐│└┘) of the given OUTER
// width/height, with the pane title embedded in the top border:
//
//	┌─┤ LOCAL_NODE ├───────────────┐
//	│ ...body...                   │
//	└──────────────────────────────┘
//
// focused selects the border color (Primary when focused, BorderInactive
// otherwise). This is the core "brutalist" container for the whole UI.
func Pane(title, body string, width, height int, focused bool) string {
	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}
	bcol := lipgloss.TerminalColor(BorderInactive)
	if focused {
		bcol = Primary
	}
	bs := lipgloss.NewStyle().Foreground(bcol)
	innerW := width - 2

	// MaxHeight/MaxWidth CLIP the content to exactly the pane interior. lipgloss
	// Height/Width are minimums (they pad but never truncate), so without the Max*
	// caps an over-tall body — e.g. a wrapped log line — would make the pane
	// render more rows than requested, overflowing the layout and scrolling the
	// top borders off the alt-screen. Clipping guarantees every pane is exactly
	// width×height, keeping the whole layout flush.
	content := lipgloss.NewStyle().
		Width(innerW).
		Height(height-2).
		MaxHeight(height-2).
		MaxWidth(innerW).
		Padding(0, boxHPad).
		Render(body)

	var b strings.Builder
	b.WriteString(titledTop(title, width, bs))
	b.WriteByte('\n')
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(bs.Render("│"))
		b.WriteString(line)
		b.WriteString(bs.Render("│"))
		b.WriteByte('\n')
	}
	b.WriteString(bs.Render("└" + strings.Repeat("─", innerW) + "┘"))
	return b.String()
}

// titledTop renders "┌─┤ TITLE ├────────┐" sized to exactly width cells. The
// title is left-aligned in the top border (matching the mockups). If the title
// can't fit, the border is drawn plain.
func titledTop(title string, width int, bs lipgloss.Style) string {
	innerW := width - 2
	const lead, trail = "─┤ ", " ├"
	used := lipgloss.Width(lead) + lipgloss.Width(title) + lipgloss.Width(trail)
	dashes := innerW - used
	if dashes < 0 {
		return bs.Render("┌" + strings.Repeat("─", innerW) + "┐")
	}
	return bs.Render("┌"+lead) +
		Title.Render(title) +
		bs.Render(trail+strings.Repeat("─", dashes)+"┐")
}
