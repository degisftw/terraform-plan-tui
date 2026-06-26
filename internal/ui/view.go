package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

const (
	headerHeight = 3 // title · summary counts · icon legend
	footerHeight = 3
)

// View assembles the full screen: header, the active body (graph or list), and
// footer — unless help is showing, which takes over the whole screen.
func (m Model) View() string {
	if !m.ready {
		return "loading plan…"
	}
	if m.showHelp {
		return m.renderHelp()
	}

	var body string
	if m.mode == modeList {
		body = m.renderList()
	} else {
		body = m.renderGraph()
	}

	return strings.Join([]string{
		m.renderHeader(),
		body,
		m.renderFooter(),
	}, "\n")
}

// --- header -----------------------------------------------------------------

func (m Model) renderHeader() string {
	s := m.styles
	title := s.header.Render("tfplantui")
	path := m.g.PlanPath
	if path == "" {
		path = "plan"
	}
	meta := s.dim.Render("  " + path)
	if m.g.TerraformVersion != "" {
		meta += s.faint.Render("  ·  terraform " + m.g.TerraformVersion)
	}

	modeName := "graph"
	if m.mode == modeList {
		modeName = "list"
	}
	flags := "  " + s.faint.Render("["+modeName+"]")
	if m.hideNoOp {
		flags += " " + s.dim.Render("changes-only")
	}
	if m.showUnchanged {
		flags += " " + s.dim.Render("+unchanged")
	}
	line1 := clipANSI(title+meta+flags, m.width)
	return pad(line1, m.width) + "\n" +
		pad(m.renderSummary(), m.width) + "\n" +
		pad(m.renderLegend(), m.width)
}

// renderLegend is a dense, single-line key for the icons actually present.
func (m Model) renderLegend() string {
	parts := make([]string, 0, len(m.present)+1)
	for _, a := range m.present {
		sym := lipgloss.NewStyle().Foreground(actionColor(a)).Bold(true).Render(a.Symbol())
		parts = append(parts, sym+" "+m.styles.dim.Render(legendWord(a)))
	}
	arrow := lipgloss.NewStyle().Foreground(colArrow).Render("─▶")
	parts = append(parts, arrow+" "+m.styles.dim.Render("depends on"))
	return clipANSI(strings.Join(parts, "    "), m.width)
}

// legendWord is the terse word shown for an action in the legend and help
// screen ("add" for create, "destroy" for delete, etc.).
func legendWord(a tfplan.Action) string {
	switch a {
	case tfplan.ActionCreate:
		return "add"
	case tfplan.ActionDelete:
		return "destroy"
	case tfplan.ActionReplace:
		return "replace"
	case tfplan.ActionUpdate:
		return "update"
	case tfplan.ActionMixed:
		return "mixed"
	case tfplan.ActionRead:
		return "read"
	case tfplan.ActionForget:
		return "forget"
	default:
		return "no-op"
	}
}

func (m Model) renderSummary() string {
	sm := m.g.Summary
	s := m.styles
	seg := func(style lipgloss.Style, n int, label string) string {
		if n == 0 {
			return ""
		}
		return style.Render(fmt.Sprintf("%d %s", n, label)) + "  "
	}
	out := seg(s.add, sm.Add, "to add") +
		seg(s.upd, sm.Change, "to change") +
		seg(s.del, sm.Destroy, "to destroy")
	if sm.Replace > 0 {
		out += lipgloss.NewStyle().Foreground(colReplace).Render(fmt.Sprintf("%d to replace", sm.Replace)) + "  "
	}
	if sm.NoOp > 0 {
		out += s.dim.Render(fmt.Sprintf("%d no-op", sm.NoOp)) + "  "
	}
	out += s.faint.Render(fmt.Sprintf("· %d instances / %d blocks", m.g.TotalInstances(), m.g.TotalNodes()))

	// Horizontal scroll indicator.
	if m.mode == modeGraph && m.g.MaxLayer()+1 > m.visibleCols() {
		last := m.colScroll + m.visibleCols()
		if last > m.g.MaxLayer()+1 {
			last = m.g.MaxLayer() + 1
		}
		out += s.faint.Render(fmt.Sprintf("   layers %d–%d of %d", m.colScroll, last-1, m.g.MaxLayer()))
	}
	return clipANSI(out, m.width)
}

// --- footer -----------------------------------------------------------------

func (m Model) renderFooter() string {
	rule := m.styles.faint.Render(strings.Repeat("─", m.width))
	ctx := m.renderContext()
	var bottom string
	switch {
	case m.searching:
		bottom = m.search.View()
	case m.searchMsg != "":
		bottom = m.styles.del.Render(m.searchMsg) + m.styles.faint.Render("   (press / to search again)")
	default:
		bottom = m.renderHints()
	}
	return pad(rule, m.width) + "\n" + pad(clipANSI(ctx, m.width), m.width) + "\n" + pad(clipANSI(bottom, m.width), m.width)
}

func (m Model) renderContext() string {
	n := m.selectedNode()
	if n == nil {
		return m.styles.dim.Render("no selection")
	}
	act := n.ActionAgg()
	sym := lipgloss.NewStyle().Foreground(actionColor(act)).Bold(true).Render(act.Symbol())
	word := lipgloss.NewStyle().Foreground(actionColor(act)).Render(act.Label())

	addr := m.styles.text.Bold(true).Render(n.Label())
	if n.Count() > 1 {
		addr += m.styles.dim.Render(" ×" + strconv.Itoa(n.Count()))
	}
	out := sym + " " + addr + "  " + word
	if len(n.DepsOut) > 0 {
		out += m.styles.faint.Render("   depends on ") + m.styles.dim.Render(shortList(n.DepsOut, 3))
	}
	if len(n.DepsIn) > 0 {
		out += m.styles.faint.Render("   used by ") + m.styles.dim.Render(shortList(n.DepsIn, 3))
	}
	if !m.expanded[n.Block] {
		out += m.styles.faint.Render("   · space to expand")
	}
	return out
}

func (m Model) renderHints() string {
	var keys string
	if m.mode == modeGraph {
		keys = "↑↓←→ move · space expand · t list · / search · u unchanged · n changes-only · ? help · q quit"
	} else {
		keys = "↑↓ move · space expand · t graph · / search · u unchanged · n changes-only · ? help · q quit"
	}
	return m.styles.faint.Render(keys)
}

// --- helpers ----------------------------------------------------------------

func (m Model) emptyMessage() string {
	if m.g.TotalNodes() == 0 {
		return "This plan proposes no resource changes — infrastructure is up to date."
	}
	if m.hideNoOp {
		return "No changes to show. Press  n  to include no-op resources."
	}
	return "Nothing to display."
}
