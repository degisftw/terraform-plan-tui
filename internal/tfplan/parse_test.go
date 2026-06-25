package tfplan

import (
	"os"
	"path/filepath"
	"testing"
)

func loadGraph(t *testing.T, example string) *Graph {
	t.Helper()
	path := filepath.Join("..", "..", "examples", example, "plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("example %s not generated (%v); run examples/generate.sh", example, err)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(%s): %v", example, err)
	}
	return g
}

func TestParseSimple(t *testing.T) {
	g := loadGraph(t, "simple")

	if got := g.TotalNodes(); got != 6 {
		t.Errorf("TotalNodes = %d, want 6", got)
	}
	if g.Summary.Add != 6 {
		t.Errorf("Summary.Add = %d, want 6", g.Summary.Add)
	}
	if g.Summary.Destroy != 0 || g.Summary.Change != 0 {
		t.Errorf("Summary = %+v, want only adds", g.Summary)
	}

	// Layering: seed/port/request_id are roots; app/token next; db last.
	wantLayer := map[string]int{
		"random_id.seed":         0,
		"random_integer.port":    0,
		"random_uuid.request_id": 0,
		"random_pet.app":         1,
		"random_string.token":    1,
		"random_password.db":     2,
	}
	for block, want := range wantLayer {
		n := g.ByBlock[block]
		if n == nil {
			t.Fatalf("missing node %s", block)
		}
		if n.Layer != want {
			t.Errorf("%s layer = %d, want %d", block, n.Layer, want)
		}
	}

	// Edges.
	app := g.ByBlock["random_pet.app"]
	if len(app.DepsOut) != 1 || app.DepsOut[0] != "random_id.seed" {
		t.Errorf("random_pet.app DepsOut = %v, want [random_id.seed]", app.DepsOut)
	}
	db := g.ByBlock["random_password.db"]
	wantDB := map[string]bool{"random_pet.app": true, "random_string.token": true}
	if len(db.DepsOut) != 2 {
		t.Errorf("random_password.db DepsOut = %v, want 2 deps", db.DepsOut)
	}
	for _, d := range db.DepsOut {
		if !wantDB[d] {
			t.Errorf("unexpected dep %s on db", d)
		}
	}
	// seed should know app depends on it.
	seed := g.ByBlock["random_id.seed"]
	if len(seed.DepsIn) != 1 || seed.DepsIn[0] != "random_pet.app" {
		t.Errorf("random_id.seed DepsIn = %v, want [random_pet.app]", seed.DepsIn)
	}
}

func TestParseMedium(t *testing.T) {
	g := loadGraph(t, "medium")

	if got := g.TotalNodes(); got != 10 {
		t.Errorf("TotalNodes = %d, want 10 blocks", got)
	}
	if got := g.TotalInstances(); got != 28 {
		t.Errorf("TotalInstances = %d, want 28", got)
	}
	// Summary: 11 replaces => +4 create +11 = 15 add; -3 delete +11 = 14 destroy.
	if g.Summary.Add != 15 {
		t.Errorf("Summary.Add = %d, want 15", g.Summary.Add)
	}
	if g.Summary.Destroy != 14 {
		t.Errorf("Summary.Destroy = %d, want 14", g.Summary.Destroy)
	}
	if g.Summary.Replace != 11 {
		t.Errorf("Summary.Replace = %d, want 11", g.Summary.Replace)
	}

	// A multi-instance block aggregates a single action when instances agree.
	apiKey := g.ByBlock["random_string.api_key"]
	if apiKey.Count() != 4 {
		t.Errorf("api_key count = %d, want 4", apiKey.Count())
	}
	if apiKey.ActionAgg() != ActionReplace {
		t.Errorf("api_key aggregate action = %s, want replace", apiKey.ActionAgg())
	}

	// service has mixed actions (5 no-op + 1 create).
	service := g.ByBlock["random_pet.service"]
	if service.ActionAgg() != ActionMixed {
		t.Errorf("service aggregate = %s, want mixed", service.ActionAgg())
	}

	// master depends on api_key and sits in layer 2.
	master := g.ByBlock["random_password.master"]
	if master.Layer != 2 {
		t.Errorf("master layer = %d, want 2", master.Layer)
	}
}
