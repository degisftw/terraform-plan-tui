package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// This file holds low-level text mechanics — rune-width clipping/padding, ANSI-aware
// width handling, block layout, and scrolling math. Nothing here makes rendering
// decisions; it just measures and shapes strings.

// clip truncates s to a max display width (rune-based), appending … when cut.
func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

// pad right-pads s with spaces to exactly width (rune-based, no clipping if longer).
func pad(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// clipANSI truncates a possibly-ANSI-styled string to a display width.
func clipANSI(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// padANSI right-pads a possibly-ANSI string to a display width (clipping if longer).
func padANSI(s string, width int) string {
	dw := lipgloss.Width(s)
	if dw > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	if dw < width {
		return s + strings.Repeat(" ", width-dw)
	}
	return s
}

// centerPad centers s within width, clipping if it does not fit.
func centerPad(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return clip(s, w)
	}
	left := (w - n) / 2
	right := w - n - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// padLines pads/truncates to exactly n rows and right-pads each row to width.
func padLines(lines []string, n, width int) string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if i < len(lines) {
			out = append(out, padANSI(lines[i], width))
		} else {
			out = append(out, strings.Repeat(" ", width))
		}
	}
	return strings.Join(out, "\n")
}

// centerMessage centers a single dimmed message in a width×height block.
func centerMessage(msg string, height, width int) string {
	plain := clip(msg, width)
	left := (width - len([]rune(plain))) / 2
	if left < 0 {
		left = 0
	}
	line := strings.Repeat(" ", left) + lipgloss.NewStyle().Foreground(colDim).Render(plain)
	mid := height / 2
	lines := make([]string, height)
	for i := range lines {
		if i == mid {
			lines[i] = padANSI(line, width)
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
	}
	return strings.Join(lines, "\n")
}

// scrollFor returns a top line offset that keeps [selStart, selStart+selH)
// visible within a window of the given height over a total number of lines.
func scrollFor(selStart, selH, total, height int) int {
	if total <= height {
		return 0
	}
	scroll := selStart - 1
	if scroll < 0 {
		scroll = 0
	}
	if selStart+selH > scroll+height {
		scroll = selStart + selH - height
	}
	if scroll > total-height {
		scroll = total - height
	}
	if scroll < 0 {
		scroll = 0
	}
	return scroll
}

// shortList joins up to max block names ("type.name"), with a "+N" overflow tail.
func shortList(blocks []string, max int) string {
	names := make([]string, 0, len(blocks))
	for _, b := range blocks {
		names = append(names, lastSegment(b))
	}
	if len(names) <= max {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:max], ", ") + fmt.Sprintf(" +%d", len(names)-max)
}

// lastSegment shortens a block address to "type.name".
func lastSegment(block string) string {
	parts := strings.Split(block, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return block
}
