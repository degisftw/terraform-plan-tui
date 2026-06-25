package ui

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

// PrintSummary writes a concise, non-interactive overview of the plan. Color is
// emitted only when the writer is a terminal (lipgloss strips it when piped).
func PrintSummary(w io.Writer, g *tfplan.Graph) {
	sm := g.Summary
	bold := lipgloss.NewStyle().Bold(true)
	fmt.Fprintln(w, bold.Render(fmt.Sprintf("Plan: %d to add, %d to change, %d to destroy",
		sm.Add, sm.Change, sm.Destroy)))

	var extra []string
	if sm.Replace > 0 {
		extra = append(extra, fmt.Sprintf("%d replace", sm.Replace))
	}
	if sm.Read > 0 {
		extra = append(extra, fmt.Sprintf("%d read", sm.Read))
	}
	if sm.NoOp > 0 {
		extra = append(extra, fmt.Sprintf("%d no-op", sm.NoOp))
	}
	if len(extra) > 0 {
		fmt.Fprintf(w, "      %s\n", strings.Join(extra, ", "))
	}
	fmt.Fprintf(w, "      %d instances · %d resource blocks · %d dependency layers\n\n",
		g.TotalInstances(), g.TotalNodes(), g.MaxLayer()+1)

	order := []tfplan.Action{
		tfplan.ActionCreate, tfplan.ActionUpdate, tfplan.ActionReplace,
		tfplan.ActionDelete, tfplan.ActionMixed, tfplan.ActionRead, tfplan.ActionNoOp,
	}
	for _, act := range order {
		var nodes []*tfplan.Node
		for _, n := range g.Nodes {
			if n.ActionAgg() == act {
				nodes = append(nodes, n)
			}
		}
		if len(nodes) == 0 {
			continue
		}
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Layer != nodes[j].Layer {
				return nodes[i].Layer < nodes[j].Layer
			}
			return nodes[i].Block < nodes[j].Block
		})

		style := lipgloss.NewStyle().Foreground(summaryColor(act)).Bold(true)
		fmt.Fprintln(w, style.Render(fmt.Sprintf("%s %s (%d)", act.Symbol(), titleCase(act.Label()), len(nodes))))
		for _, n := range nodes {
			line := "  " + style.Render(act.Symbol()) + " " + n.Label()
			if n.Count() > 1 {
				line += lipgloss.NewStyle().Faint(true).Render(" ×" + strconv.Itoa(n.Count()))
			}
			line += lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("  · layer %d", n.Layer))
			if len(n.DepsOut) > 0 {
				line += lipgloss.NewStyle().Faint(true).Render("  depends on " + strconv.Itoa(len(n.DepsOut)))
			}
			fmt.Fprintln(w, line)
		}
		fmt.Fprintln(w)
	}
}

// summaryColor is actionColor with one deliberate override: the summary lists
// no-op resources to be read, so they use the readable dim grey rather than the
// near-invisible colNoOp the graph uses to make unchanged cards recede.
func summaryColor(a tfplan.Action) lipgloss.Color {
	if a == tfplan.ActionNoOp {
		return colDim
	}
	return actionColor(a)
}

// titleCase upper-cases the first letter of s.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
