package tfplan

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func parseDependencyPlan(t *testing.T, changes []rawResourceChange, root rawConfigModule) *Graph {
	t.Helper()
	data, err := json.Marshal(rawPlan{
		FormatVersion:   "1.2",
		ResourceChanges: changes,
		Configuration:   rawConfiguration{RootModule: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func dependencyChange(address string, index any) rawResourceChange {
	return rawResourceChange{
		Address: address,
		Index:   index,
		Type:    "test_resource",
		Change:  rawChange{Actions: []string{"no-op"}},
	}
}

func dependencyExpression(t *testing.T, refs ...string) json.RawMessage {
	t.Helper()
	expr, err := json.Marshal(map[string]any{"references": refs})
	if err != nil {
		t.Fatal(err)
	}
	return expr
}

func assertInstanceDeps(t *testing.T, node *Node, want []InstanceDependency) {
	t.Helper()
	if len(node.InstanceDeps) != len(want) {
		t.Fatalf("%s InstanceDeps = %+v, want %+v", node.Block, node.InstanceDeps, want)
	}
	for i, dep := range node.InstanceDeps {
		if dep.Block != want[i].Block || dep.Unresolved != want[i].Unresolved || !slices.Equal(dep.Instances, want[i].Instances) {
			t.Errorf("%s InstanceDeps[%d] = %+v, want %+v", node.Block, i, dep, want[i])
		}
	}
}

func TestParseInstanceDependencies(t *testing.T) {
	const block = "test_resource.upstream"
	multiple := []rawResourceChange{
		dependencyChange(block+"[0]", 0),
		dependencyChange(block+"[1]", 1),
	}
	tests := []struct {
		name        string
		upstream    []rawResourceChange
		expressions [][]string
		dependsOn   []string
		instances   []string
		unresolved  bool
	}{
		{
			name:        "literal zero with implied shorter references",
			expressions: [][]string{{block + "[0].id", block + "[0]", block}},
			instances:   []string{block + "[0]"},
		},
		{
			name:        "attribute index does not replace instance index",
			expressions: [][]string{{block + "[0].items[1].id", block + "[0]", block}},
			instances:   []string{block + "[0]"},
		},
		{
			name: "string key with traversal punctuation",
			upstream: []rawResourceChange{
				dependencyChange(block+`["west.prod[blue]\""]`, `west.prod[blue]"`),
				dependencyChange(block+`["east"]`, "east"),
			},
			expressions: [][]string{{block + `["west.prod[blue]\""].id`, block + `["west.prod[blue]\""]`, block}},
			instances:   []string{block + `["west.prod[blue]\""]`},
		},
		{
			name:        "unindexed multi-instance block",
			expressions: [][]string{{block + ".id", block}},
			unresolved:  true,
		},
		{
			name:        "dynamic count index is not paired",
			expressions: [][]string{{block + "[count.index].id", block, "count.index"}},
			unresolved:  true,
		},
		{
			name: "dynamic key is not paired",
			upstream: []rawResourceChange{
				dependencyChange(block+`["east"]`, "east"),
				dependencyChange(block+`["west"]`, "west"),
			},
			expressions: [][]string{{block + "[each.key].id", block, "each.key"}},
			unresolved:  true,
		},
		{
			name:        "JSON dynamic reference without index",
			expressions: [][]string{{block, "each.key"}},
			unresolved:  true,
		},
		{
			name:        "dynamic singleton index remains unresolved",
			upstream:    multiple[:1],
			expressions: [][]string{{block + "[count.index].id", block, "count.index"}},
			unresolved:  true,
		},
		{
			name:        "missing explicit index does not use a singleton instead",
			upstream:    multiple[1:],
			expressions: [][]string{{block + "[0].id", block + "[0]", block}},
			unresolved:  true,
		},
		{
			name:        "unindexed singleton",
			upstream:    []rawResourceChange{dependencyChange(block, nil)},
			expressions: [][]string{{block + ".id", block}},
			instances:   []string{block},
		},
		{
			name:        "unindexed block with one indexed instance",
			upstream:    multiple[1:],
			expressions: [][]string{{block + ".id", block}},
			instances:   []string{block + "[1]"},
		},
		{
			name:        "independent unindexed attribute in same array",
			expressions: [][]string{{block + "[0].id", block + "[0]", block + ".id", block}},
			instances:   []string{block + "[0]"},
			unresolved:  true,
		},
		{
			name:        "independent expression is not an implied prefix",
			expressions: [][]string{{block + "[0].id", block + "[0]", block}, {block}},
			instances:   []string{block + "[0]"},
			unresolved:  true,
		},
		{
			name:        "depends_on block is not an implied prefix",
			expressions: [][]string{{block + "[0].id", block + "[0]", block}},
			dependsOn:   []string{block},
			instances:   []string{block + "[0]"},
			unresolved:  true,
		},
		{
			name:        "literal and dynamic references coexist",
			expressions: [][]string{{block + "[0].id", block + "[0]", block, "each.key"}},
			instances:   []string{block + "[0]"},
			unresolved:  true,
		},
		{
			name:        "literal and variable lookup references coexist",
			expressions: [][]string{{block + "[0].id", block + "[0]", block, "var.selected_index"}},
			instances:   []string{block + "[0]"},
			unresolved:  true,
		},
		{
			name:        "confirmed instances are sorted and unique",
			expressions: [][]string{{block + "[1].id", block + "[0].id", block + "[1].other", block}},
			dependsOn:   []string{block + "[1]"},
			instances:   []string{block + "[0]", block + "[1]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := tt.upstream
			if upstream == nil {
				upstream = multiple
			}
			expressions := map[string]json.RawMessage{}
			for i, refs := range tt.expressions {
				expressions[fmt.Sprint(i)] = dependencyExpression(t, refs...)
			}
			changes := []rawResourceChange{
				dependencyChange("test_resource.consumer[0]", 0),
				dependencyChange("test_resource.consumer[1]", 1),
			}
			changes = append(changes, upstream...)
			g := parseDependencyPlan(t, changes, rawConfigModule{Resources: []rawConfigResource{{
				Address:     "test_resource.consumer",
				Expressions: expressions,
				DependsOn:   tt.dependsOn,
			}}})
			consumer := g.ByBlock["test_resource.consumer"]
			assertInstanceDeps(t, consumer, []InstanceDependency{{Block: block, Instances: tt.instances, Unresolved: tt.unresolved}})
			if !slices.Equal(consumer.DepsOut, []string{block}) {
				t.Errorf("DepsOut = %v, want [%s]", consumer.DepsOut, block)
			}
			if got := g.ByBlock[block].DepsIn; !slices.Equal(got, []string{consumer.Block}) {
				t.Errorf("DepsIn = %v, want [%s]", got, consumer.Block)
			}
			if g.TotalNodes() != 2 || g.TotalInstances() != len(changes) || g.Summary.NoOp != len(changes) || consumer.Layer != 1 {
				t.Errorf("legacy graph changed: nodes=%d instances=%d summary=%+v layer=%d", g.TotalNodes(), g.TotalInstances(), g.Summary, consumer.Layer)
			}
		})
	}
}

func TestNestedReferencesRetainIndependentAmbiguity(t *testing.T) {
	g := parseDependencyPlan(t, []rawResourceChange{
		dependencyChange("test_resource.consumer", nil),
		dependencyChange("test_resource.upstream[0]", 0),
		dependencyChange("test_resource.upstream[1]", 1),
	}, rawConfigModule{Resources: []rawConfigResource{{
		Address: "test_resource.consumer",
		Expressions: map[string]json.RawMessage{
			"nested": json.RawMessage(`[{"references":["test_resource.upstream[0].id","test_resource.upstream[0]","test_resource.upstream"]},{"value":{"references":["test_resource.upstream"]}}]`),
		},
	}}})
	assertInstanceDeps(t, g.ByBlock["test_resource.consumer"], []InstanceDependency{{
		Block: "test_resource.upstream", Instances: []string{"test_resource.upstream[0]"}, Unresolved: true,
	}})
}

func TestInstanceDependenciesPreserveDataAndModulePrefixes(t *testing.T) {
	consumer := dependencyChange("module.child.test_resource.consumer", nil)
	consumer.ModuleAddress = "module.child"
	data := dependencyChange(`module.child.data.test_resource.upstream["east"]`, "east")
	data.Mode = "data"
	data.ModuleAddress = "module.child"
	otherData := dependencyChange(`module.child.data.test_resource.upstream["west"]`, "west")
	otherData.Mode = "data"
	otherData.ModuleAddress = "module.child"
	g := parseDependencyPlan(t, []rawResourceChange{
		consumer, data, otherData,
		dependencyChange("data.test_resource.upstream", nil),
	}, rawConfigModule{ModuleCalls: map[string]rawModuleCall{"child": {
		Module: rawConfigModule{Resources: []rawConfigResource{{
			Address: "test_resource.consumer",
			Expressions: map[string]json.RawMessage{
				"input": dependencyExpression(t, `data.test_resource.upstream["east"].id`, `data.test_resource.upstream["east"]`, "data.test_resource.upstream"),
				"other": dependencyExpression(t, "module.sibling.output", "var.input"),
			},
		}}},
	}}})
	node := g.ByBlock[consumer.Address]
	assertInstanceDeps(t, node, []InstanceDependency{{
		Block:     "module.child.data.test_resource.upstream",
		Instances: []string{data.Address},
	}})
	if !slices.Equal(node.DepsOut, []string{"module.child.data.test_resource.upstream"}) {
		t.Errorf("module-local DepsOut = %v", node.DepsOut)
	}
	if len(g.ByBlock["data.test_resource.upstream"].DepsIn) != 0 {
		t.Fatal("child module reference incorrectly connected to root data resource")
	}
}

func TestInstanceDependenciesAreSortedAndKeepOnlyPresentEndpoints(t *testing.T) {
	g := parseDependencyPlan(t, []rawResourceChange{
		dependencyChange("test_resource.consumer", nil),
		dependencyChange("test_resource.zulu", nil),
		dependencyChange("test_resource.alpha[0]", 0),
		dependencyChange("test_resource.alpha[1]", 1),
	}, rawConfigModule{Resources: []rawConfigResource{{
		Address: "test_resource.consumer",
		Expressions: map[string]json.RawMessage{
			"input": dependencyExpression(t, "test_resource.zulu.id", "test_resource.alpha", "test_resource.absent.id", "test_resource.consumer.id", "local.input"),
		},
	}}})
	node := g.ByBlock["test_resource.consumer"]
	assertInstanceDeps(t, node, []InstanceDependency{
		{Block: "test_resource.alpha", Unresolved: true},
		{Block: "test_resource.zulu", Instances: []string{"test_resource.zulu"}},
	})
	if !slices.Equal(node.DepsOut, []string{"test_resource.alpha", "test_resource.zulu"}) {
		t.Errorf("DepsOut = %v", node.DepsOut)
	}
}

func TestMediumConfirmedInstanceDependencies(t *testing.T) {
	g := loadGraph(t, "medium")
	assertInstanceDeps(t, g.ByBlock["random_pet.release"], []InstanceDependency{
		{Block: "random_integer.port", Instances: []string{"random_integer.port[0]"}},
		{Block: "random_pet.service", Instances: []string{"random_pet.service[0]"}},
	})
	assertInstanceDeps(t, g.ByBlock["random_password.master"], []InstanceDependency{
		{Block: "random_pet.org", Instances: []string{"random_pet.org"}},
		{Block: "random_string.api_key", Instances: []string{"random_string.api_key[0]"}},
	})
}

func TestResourceAddress(t *testing.T) {
	for _, tt := range []struct {
		resource Resource
		want     string
	}{
		{Resource{Block: "test_resource.single"}, "test_resource.single"},
		{Resource{Block: "test_resource.counted", Index: "0"}, "test_resource.counted[0]"},
		{Resource{Block: "test_resource.keyed", Index: `"east"`}, `test_resource.keyed["east"]`},
		{Resource{Block: `module.child["prod"].data.test_resource.keyed`, Index: `"west[blue]"`}, `module.child["prod"].data.test_resource.keyed["west[blue]"]`},
	} {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.resource.Address(); got != tt.want {
				t.Errorf("Address() = %q, want %q", got, tt.want)
			}
			if got := blockAddress(tt.want); got != tt.resource.Block {
				t.Errorf("blockAddress(%q) = %q, want %q", tt.want, got, tt.resource.Block)
			}
		})
	}
}
