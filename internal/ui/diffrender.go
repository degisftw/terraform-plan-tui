package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

// This file turns a tfplan.DiffLine (a computed attribute change) into a single
// color-coded, width-clipped row. It owns the mapping from DiffKind to glyph and
// color; the diff itself is computed in package tfplan.

func diffKindColor(k tfplan.DiffKind) lipgloss.Color {
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

// renderDiffLine formats one attribute diff row, color-coded, clipped to width.
func renderDiffLine(l tfplan.DiffLine, width int) string {
	indent := strings.Repeat("  ", l.Indent)
	sym := diffKindSymbol(l.Kind)
	color := diffKindColor(l.Kind)

	var body string
	switch {
	case l.IsParent:
		body = l.Key + ":"
	case l.Kind == tfplan.KindAdd:
		body = l.Key + " = " + l.After
	case l.Kind == tfplan.KindRemove:
		body = l.Key + " = " + l.Before
	case l.Kind == tfplan.KindUnknown:
		if l.Before != "" {
			body = l.Key + " = " + l.Before + " → " + l.After
		} else {
			body = l.Key + " = " + l.After
		}
	case l.Kind == tfplan.KindUpdate:
		body = l.Key + " = " + l.Before + " → " + l.After
	default: // no change
		body = l.Key + " = " + l.After
	}

	base := indent + sym + " " + body
	const marker = " # forces replacement"
	if l.Forces && width > len(marker)+6 {
		base = clip(base, width-len(marker))
		return lipgloss.NewStyle().Foreground(color).Render(base) +
			lipgloss.NewStyle().Foreground(colReplace).Render(marker)
	}
	return lipgloss.NewStyle().Foreground(color).Render(clip(base, width))
}
