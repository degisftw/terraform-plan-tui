package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// This file turns a tfplan.DiffLine (a computed attribute change) into a single
// color-coded, width-clipped row. It owns the mapping from DiffKind to glyph and
// color; the diff itself is computed in package tfplan.

func diffKindColor(k tfplan.DiffKind) lipgloss.TerminalColor {
	switch k {
	case tfplan.KindAdd:
		return colCreate
	case tfplan.KindRemove:
		return colDelete
	case tfplan.KindUpdate:
		return colUpdate
	case tfplan.KindUnknown:
		return colUpdate
	default:
		return colDim
	}
}

func diffKindSymbol(k tfplan.DiffKind) string {
	switch k {
	case tfplan.KindAdd:
		return "+"
	case tfplan.KindRemove:
		return "-"
	case tfplan.KindUpdate, tfplan.KindUnknown:
		return "~"
	default:
		return " "
	}
}

// diffLineBody is shared by compact previews and the untruncated detail view.
func diffLineBody(l tfplan.DiffLine) string {
	key := lipgloss.NewStyle().Foreground(colDim).Render(l.Key)
	text := lipgloss.NewStyle().Foreground(colText)
	before := lipgloss.NewStyle().Foreground(colDelete)
	after := lipgloss.NewStyle().Foreground(colCreate)
	sep := lipgloss.NewStyle().Foreground(colFaint)
	if l.IsParent {
		return text.Bold(true).Render(l.Key + ":")
	}
	prefix := key + sep.Render(" = ")
	switch l.Kind {
	case tfplan.KindRemove:
		return prefix + before.Render(l.Before)
	case tfplan.KindUpdate:
		return prefix + before.Render(l.Before) + sep.Render(" → ") + after.Render(l.After)
	case tfplan.KindUnknown:
		if l.Before != "" {
			prefix += text.Render(l.Before) + sep.Render(" → ")
		}
		return prefix + lipgloss.NewStyle().Foreground(colUpdate).Render(l.After)
	case tfplan.KindNoChange:
		return prefix + lipgloss.NewStyle().Foreground(colDim).Render(l.After)
	default:
		return prefix + text.Render(l.After)
	}
}

// renderDiffLine formats one attribute diff row, color-coded, clipped to width.
func renderDiffLine(l tfplan.DiffLine, width int) string {
	symbol := lipgloss.NewStyle().Foreground(diffKindColor(l.Kind)).Bold(true).Render(diffKindSymbol(l.Kind))
	base := strings.Repeat("  ", l.Indent) + symbol + " " + diffLineBody(l)
	const marker = " # forces replacement"
	if l.Forces && width > len(marker)+6 {
		return clipANSI(base, width-len(marker)) +
			lipgloss.NewStyle().Foreground(colReplace).Render(marker)
	}
	return clipANSI(base, width)
}
