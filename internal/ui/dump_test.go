package ui

import (
	"fmt"
	"os"
	"testing"
)

// TestDumpVisual is a manual harness: `TFPLAN_DUMP=1 go test -run TestDumpVisual -v`
// prints the ANSI-stripped layout so we can eyeball box alignment and columns.
func TestDumpVisual(t *testing.T) {
	if os.Getenv("TFPLAN_DUMP") == "" {
		t.Skip("set TFPLAN_DUMP=1 to dump rendered layouts")
	}
	cases := []struct {
		name    string
		example string
		w, h    int
		keys    []string
	}{
		{"simple-graph", "simple", 120, 30, nil},
		{"simple-expanded", "simple", 120, 30, []string{"right", "space"}},
		{"medium-graph", "medium", 140, 32, []string{"right", "space"}},
		{"medium-list", "medium", 140, 32, []string{"t", "down", "down", "space"}},
		{"large-graph", "large", 160, 30, nil},
		{"large-scrolled-right", "large", 160, 30, []string{"right", "right", "right", "right", "down", "down"}},
		{"large-changes-only", "large", 160, 30, []string{"n"}},
	}
	for _, c := range cases {
		m := loadUI(t, c.example, c.w, c.h)
		m = send(m, c.keys...)
		fmt.Printf("\n========== %s (%dx%d) ==========\n", c.name, c.w, c.h)
		fmt.Println(plain(m.View()))
	}
}
