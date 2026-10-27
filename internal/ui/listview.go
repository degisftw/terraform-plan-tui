package ui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// renderList draws the flat, filtered node list (one row per block, expandable to
// its attribute diff), windowed to the visible band.
func (m Model) renderList() string {
	bodyH := m.bodyHeight()
	items := m.listItems()
	if len(items) == 0 {
		return centerMessage(m.emptyMessage(), bodyH, m.width)
	}

	// Defensive clamp: the selection can lag behind a filter that shrank the list.
	sel := m.listSel
	if sel >= len(items) {
		sel = len(items) - 1
	}
	if sel < 0 {
		sel = 0
	}

	w := m.width

	// Measure item heights cheaply (a Diff is only computed for expanded items)
	// so we can window: render strings only for items inside the visible band.
	starts := make([]int, len(items))
	total := 0
	for i, n := range items {
		starts[i] = total
		total += m.listItemHeight(n)
	}
	scroll := scrollFor(starts[sel], m.listItemHeight(items[sel]), total, bodyH)

	out := make([]string, bodyH)
	for i := range out {
		out[i] = ""
	}
	for i, n := range items {
		s, h := starts[i], m.listItemHeight(n)
		if s+h <= scroll {
			continue // above the window
		}
		if s >= scroll+bodyH {
			break // below the window
		}
		for j, line := range m.renderListItem(n, i == sel, w) {
			if pos := s + j - scroll; pos >= 0 && pos < bodyH {
				out[pos] = line
			}
		}
	}
	return padLines(out, bodyH, w)
}

// listItemHeight returns the rendered line count of a list row without building
// its strings (a Diff is computed only for expanded rows). It MUST stay in sync
// with renderListItem's output length.
func (m Model) listItemHeight(n *tfplan.Node) int {
	h := 1
	if m.expanded[n.Block] {
		if n.Count() > 1 {
			h++
		}
		if d := len(n.Instances[m.shownInstance(n)].Diff(m.showUnchanged)); d == 0 {
			h++
		} else {
			h += d
		}
	}
	return h
}

func (m Model) renderListItem(n *tfplan.Node, selected bool, w int) []string {
	act := n.ActionAgg()
	sym := lipgloss.NewStyle().Foreground(actionColor(act)).Bold(true).Render(act.Symbol())

	bar := "  "
	labelStyle := m.styles.text
	switch {
	case selected:
		bar = m.styles.accent.Render("▌ ")
		labelStyle = m.styles.text.Bold(true)
	case act == tfplan.ActionNoOp:
		labelStyle = m.styles.faint
	}

	badge := ""
	if n.Count() > 1 {
		badge = m.styles.dim.Render(" ×" + strconv.Itoa(n.Count()))
	}
	word := lipgloss.NewStyle().Foreground(actionColor(act)).Render(act.Label())

	// Changed-attribute hint for the representative instance.
	hint := ""
	if act != tfplan.ActionNoOp {
		if ca := n.Instances[m.shownInstance(n)].ChangedAttrs(); ca > 0 {
			hint = m.styles.faint.Render(fmt.Sprintf("  %d attr", ca))
		}
	}

	row := bar + sym + " " + labelStyle.Render(n.Label()) + badge + "   " + word + hint
	row = clipANSI(row, w)
	if selected {
		row = renderSurface(m.styles.screen.Background(colSelBg), padANSI(row, w))
	}
	out := []string{row}

	if m.expanded[n.Block] {
		idx := m.shownInstance(n)
		inst := n.Instances[idx]
		if n.Count() > 1 {
			out = append(out, m.styles.faint.Render(fmt.Sprintf("      instance %d/%d (%s)  [ ] / Tab to cycle", idx+1, n.Count(), inst.Action.Label())))
		}
		diff := inst.Diff(m.showUnchanged)
		if len(diff) == 0 {
			out = append(out, m.styles.dim.Render("      (no attribute changes)"))
		}
		for _, l := range diff {
			out = append(out, "    "+renderDiffLine(l, w-6))
		}
	}
	return out
}
