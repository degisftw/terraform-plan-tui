package tfplan

import (
	"encoding/json"
	"errors"
	"strings"
)

// This file mirrors the parts of `terraform show -json` we read, and the
// JSON decoding helpers. It is the only place that knows the wire format.

type rawPlan struct {
	FormatVersion    string              `json:"format_version"`
	TerraformVersion string              `json:"terraform_version"`
	ResourceChanges  []rawResourceChange `json:"resource_changes"`
	Configuration    rawConfiguration    `json:"configuration"`
}

type rawResourceChange struct {
	Address       string    `json:"address"`
	ModuleAddress string    `json:"module_address"`
	Mode          string    `json:"mode"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	Index         any       `json:"index"`
	ProviderName  string    `json:"provider_name"`
	Change        rawChange `json:"change"`
}

type rawChange struct {
	Actions         []string        `json:"actions"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`
	AfterUnknown    json.RawMessage `json:"after_unknown"`
	BeforeSensitive json.RawMessage `json:"before_sensitive"`
	AfterSensitive  json.RawMessage `json:"after_sensitive"`
	ReplacePaths    json.RawMessage `json:"replace_paths"`
}

type rawConfiguration struct {
	RootModule rawConfigModule `json:"root_module"`
}

type rawConfigModule struct {
	Resources   []rawConfigResource      `json:"resources"`
	ModuleCalls map[string]rawModuleCall `json:"module_calls"`
}

type rawModuleCall struct {
	Module rawConfigModule `json:"module"`
}

type rawConfigResource struct {
	Address     string                     `json:"address"`
	Mode        string                     `json:"mode"`
	Type        string                     `json:"type"`
	Name        string                     `json:"name"`
	Expressions map[string]json.RawMessage `json:"expressions"`
	DependsOn   []string                   `json:"depends_on"`
}

// decode unmarshals JSON preserving numbers as json.Number (no float64
// precision loss on large integers).
func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return errors.New("empty json")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	return dec.Decode(v)
}

// decodeObject unmarshals JSON that is expected to be an object (or null).
func decodeObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var m map[string]any
	if err := decode(raw, &m); err != nil {
		return nil
	}
	return m
}

// decodeAny unmarshals arbitrary JSON (sensitivity flags can be bool or object).
func decodeAny(raw json.RawMessage) any {
	var v any
	if err := decode(raw, &v); err != nil {
		return nil
	}
	return v
}
