package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// renderHelp draws the full-screen key reference, action legend, and a short
// "reading the graph" note.
func (m Model) renderHelp() string {
	s := m.styles
	title := s.header.Render("tfplantui — keys")
	rows := [][2]string{
		{"↑ ↓ / k j", "move within a layer; scroll the enlarged card"},
		{"← → / h l", "move across dependency layers, including enlarged card"},
		{"space / enter", "expand or collapse the selected resource's diff"},
		{"f / Esc", "enlarge / restore focused card; neighbors stay visible"},
		{"a", "toggle all instance streams (confirmed / block scope)"},
		{"PgUp / PgDn", "scroll a page inside the enlarged card"},
		{"[  ]", "cycle instances of a count/for_each resource"},
		{"Tab / Shift-Tab", "next / previous instance (also inside enlarged card)"},
		{"u", "show or hide unchanged attributes"},
		{"n", "toggle changes-only (hide no-op resources)"},
		{"x", "collapse all expanded resources"},
		{"t", "switch between graph and list views"},
		{"g / G", "top / bottom of column, list, or enlarged card"},
		{"/", "search by resource address (Enter jumps, Esc cancels)"},
		{"? ", "toggle this help"},
		{"q / Ctrl-C", "quit"},
	}
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for _, r := range rows {
		b.WriteString("  " + s.key.Render(pad(strings.TrimSpace(r[0]), 14)) + "  " + s.text.Render(r[1]) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(s.header.Render("legend") + "\n")
	legend := []struct {
		a tfplan.Action
		d string
	}{
		{tfplan.ActionCreate, "create"},
		{tfplan.ActionUpdate, "update in place"},
		{tfplan.ActionReplace, "replace (destroy then create)"},
		{tfplan.ActionDelete, "destroy"},
		{tfplan.ActionMixed, "mixed (a count block with differing actions)"},
		{tfplan.ActionNoOp, "no change"},
	}
	for _, l := range legend {
		swatch := lipgloss.NewStyle().Foreground(actionColor(l.a)).Bold(true).Render(l.a.Symbol() + " " + pad(legendWord(l.a), 9))
		b.WriteString("  " + swatch + s.dim.Render(l.d) + "\n")
	}
	b.WriteString("\n" + s.header.Render("reading the graph") + "\n")
	b.WriteString("  " + s.accent.Render(pad("selection", 11)) + s.dim.Render("an orchid border and distinct surface mark the focused card\n"))
	b.WriteString("  " + lipgloss.NewStyle().Foreground(colArrow).Render(pad("─▶", 11)) + s.dim.Render("compact: selected block; streams: confirmed instance references\n"))
	b.WriteString("  " + s.faint.Render(pad("unchanged", 11)+"no-op resources stay readable without competing for focus\n"))
	b.WriteString("\n" + s.faint.Render("press ? or Esc to return"))

	content := b.String()
	lines := strings.Split(content, "\n")
	return padLines(lines, m.height, m.width)
}
