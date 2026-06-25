package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

// hugeGraph replicates the large example R times with unique resource names,
// producing a plan with thousands of blocks (most landing in one giant layer —
// the worst case for column rendering) to verify windowing keeps render O(view).
func hugeGraph(t *testing.T, r int) *tfplan.Graph {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "large", "plan.json"))
	if err != nil {
		t.Skipf("large example not generated: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	base := doc["resource_changes"].([]any)

	var all []any
	for i := 0; i < r; i++ {
		var copyDoc map[string]any
		_ = json.Unmarshal(data, &copyDoc) // fresh deep copy
		for _, rc := range copyDoc["resource_changes"].([]any) {
			m := rc.(map[string]any)
			m["address"] = fmt.Sprintf("%v_r%d", m["address"], i)
			m["name"] = fmt.Sprintf("%v_r%d", m["name"], i)
			all = append(all, m)
		}
	}
	doc["resource_changes"] = all
	merged, _ := json.Marshal(doc)

	g, err := tfplan.Parse(merged)
	if err != nil {
		t.Fatal(err)
	}
	_ = base
	return g
}

func TestScaleRenderIsWindowed(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test skipped in -short")
	}
	g := hugeGraph(t, 16) // ~16 * 397 ≈ 6.3k instances, ~1000 blocks
	g.PlanPath = "huge"
	m := New(g)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = nm.(Model)

	t.Logf("huge graph: %d instances / %d blocks / %d layers",
		g.TotalInstances(), g.TotalNodes(), g.MaxLayer()+1)

	// warm up (first render populates ChangedAttrs caches)
	_ = m.View()

	const frames = 60
	start := time.Now()
	for i := 0; i < frames; i++ {
		if i%2 == 0 {
			m = send(m, "down")
		} else {
			m = send(m, "right", "left")
		}
		_ = m.View()
	}
	per := time.Since(start) / frames
	t.Logf("avg render+update over %d frames: %v", frames, per)
	// Windowing means render cost tracks the viewport, not the ~6k-node column.
	// Rendering every card unwindowed would be an order of magnitude slower; this
	// generous bound just guards against an accidental return to O(plan) per frame.
	if per > 60*time.Millisecond {
		t.Errorf("render too slow at scale: %v/frame (windowing not effective?)", per)
	}
}
