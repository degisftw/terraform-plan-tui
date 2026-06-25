package tfplan

import "testing"

func lineFor(lines []DiffLine, key string, indent int) (DiffLine, bool) {
	for _, l := range lines {
		if l.Key == key && l.Indent == indent {
			return l, true
		}
	}
	return DiffLine{}, false
}

func TestDiffSensitiveRedaction(t *testing.T) {
	// create with a known, sensitive value -> redacted, not leaked.
	create := &Resource{
		Action:         ActionCreate,
		After:          map[string]any{"password": "hunter2", "name": "db"},
		AfterUnknown:   map[string]any{},
		AfterSensitive: map[string]any{"password": true},
	}
	lines := create.Diff(false)
	if l, ok := lineFor(lines, "password", 0); !ok || l.After != sensitiveText {
		t.Errorf("create password = %+v (ok=%v), want redacted %q", l, ok, sensitiveText)
	}
	if l, ok := lineFor(lines, "name", 0); !ok || l.After != `"db"` {
		t.Errorf("create name = %+v, want \"db\"", l)
	}

	// delete with a known, sensitive value -> redacted on the before side.
	del := &Resource{
		Action:          ActionDelete,
		Before:          map[string]any{"password": "hunter2"},
		BeforeSensitive: map[string]any{"password": true},
	}
	if l, ok := lineFor(del.Diff(false), "password", 0); !ok || l.Before != sensitiveText {
		t.Errorf("delete password before = %+v, want redacted", l)
	}
}

func TestDiffSensitiveInList(t *testing.T) {
	// Terraform marks per-element sensitivity in a list/set with an ARRAY of
	// bools (e.g. {"input":[false,true]}). The whole collection must be redacted
	// — never render the secret element in plaintext.
	r := &Resource{
		Action:         ActionCreate,
		After:          map[string]any{"input": []any{"plainvalue", "topsecret"}},
		AfterUnknown:   map[string]any{},
		AfterSensitive: map[string]any{"input": []any{false, true}},
	}
	l, ok := lineFor(r.Diff(true), "input", 0)
	if !ok {
		t.Fatalf("no input line")
	}
	if l.After != sensitiveText || !l.Sensitive {
		t.Errorf("list with a sensitive element leaked: After=%q Sensitive=%v, want redacted", l.After, l.Sensitive)
	}
}

func TestForcesReplacement(t *testing.T) {
	g := loadGraph(t, "medium")
	lines := g.ByBlock["random_string.api_key"].Instances[0].Diff(false)
	found := false
	for _, l := range lines {
		if l.Key == "keepers" && l.IsParent {
			found = l.Forces
		}
	}
	if !found {
		t.Errorf("keepers should be flagged as forcing replacement (replace_paths [[keepers]])")
	}
}

func TestDiffUnknownAndNested(t *testing.T) {
	// id known-after-apply on create; nested keepers map with a known + unknown sub-key.
	r := &Resource{
		Action:       ActionCreate,
		After:        map[string]any{"keepers": map[string]any{"k1": "v1"}, "len": float64(8)},
		AfterUnknown: map[string]any{"id": true, "keepers": map[string]any{"k2": true}},
	}
	lines := r.Diff(false)
	if l, ok := lineFor(lines, "id", 0); !ok || l.Kind != KindUnknown || l.After != unknownText {
		t.Errorf("id = %+v, want unknown", l)
	}
	// keepers is a parent header, with k1 (add) and k2 (unknown) beneath it.
	if l, ok := lineFor(lines, "keepers", 0); !ok || !l.IsParent {
		t.Errorf("keepers should be a parent header, got %+v", l)
	}
	if l, ok := lineFor(lines, "k1", 1); !ok || l.Kind != KindAdd || l.After != `"v1"` {
		t.Errorf("keepers.k1 = %+v, want add v1", l)
	}
	if l, ok := lineFor(lines, "k2", 1); !ok || l.Kind != KindUnknown {
		t.Errorf("keepers.k2 = %+v, want unknown", l)
	}
}

func TestLargeIntPrecision(t *testing.T) {
	// 123456789012345678 > 2^53, so float64 decoding would corrupt it.
	js := `{"format_version":"1.2","terraform_version":"1.14.1","resource_changes":[
		{"address":"x.y","type":"x","name":"y","change":{"actions":["create"],
		 "after":{"big":123456789012345678,"small":42}}}]}`
	g, err := Parse([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	lines := g.ByBlock["x.y"].Instances[0].Diff(false)
	if l, ok := lineFor(lines, "big", 0); !ok || l.After != "123456789012345678" {
		t.Errorf("big int = %q, want 123456789012345678 (no float64 precision loss)", l.After)
	}
	if l, ok := lineFor(lines, "small", 0); !ok || l.After != "42" {
		t.Errorf("small int = %q, want 42", l.After)
	}
}

func TestDiffUpdateInPlace(t *testing.T) {
	r := &Resource{
		Action: ActionUpdate,
		Before: map[string]any{"size": float64(10), "tag": "old", "stable": true},
		After:  map[string]any{"size": float64(20), "tag": "old", "stable": true},
	}
	changed := r.Diff(false)
	if l, ok := lineFor(changed, "size", 0); !ok || l.Kind != KindUpdate || l.Before != "10" || l.After != "20" {
		t.Errorf("size = %+v, want 10 -> 20", l)
	}
	// unchanged attrs hidden unless asked for.
	if _, ok := lineFor(changed, "stable", 0); ok {
		t.Errorf("unchanged 'stable' should be hidden by default")
	}
	if _, ok := lineFor(r.Diff(true), "stable", 0); !ok {
		t.Errorf("unchanged 'stable' should appear with showUnchanged=true")
	}
}

func TestDiffReplace(t *testing.T) {
	g := loadGraph(t, "medium")
	apiKey := g.ByBlock["random_string.api_key"]
	lines := apiKey.Instances[0].Diff(false)

	var rotation *DiffLine
	for i := range lines {
		if lines[i].Key == "rotation" && lines[i].Indent == 1 {
			rotation = &lines[i]
		}
	}
	if rotation == nil {
		t.Fatalf("no keepers.rotation line found in %+v", lines)
	}
	if rotation.Kind != KindUpdate || rotation.Before != `"v1"` || rotation.After != `"v2"` {
		t.Errorf("rotation = %+v, want update v1->v2", *rotation)
	}
}

func TestDiffCreate(t *testing.T) {
	g := loadGraph(t, "simple")
	app := g.ByBlock["random_pet.app"]
	if app.Count() != 1 {
		t.Fatalf("expected single instance, got %d", app.Count())
	}
	lines := app.Instances[0].Diff(false)

	byKey := map[string]DiffLine{}
	for _, l := range lines {
		if !l.IsParent {
			byKey[l.Key] = l
		}
	}
	// length is a concrete add.
	if l, ok := byKey["length"]; !ok || l.Kind != KindAdd || l.After != "2" {
		t.Errorf("length line = %+v (ok=%v), want add 2", l, ok)
	}
	// id is known after apply.
	if l, ok := byKey["id"]; !ok || l.Kind != KindUnknown {
		t.Errorf("id line = %+v (ok=%v), want unknown", l, ok)
	}
	// separator is a concrete string add.
	if l, ok := byKey["separator"]; !ok || l.After != `"-"` {
		t.Errorf("separator line = %+v (ok=%v), want \"-\"", l, ok)
	}
}
