// Package tfplantui renders a HashiCorp Terraform plan as an interactive
// terminal UI: a dependency graph laid out in columns by dependency depth, with
// expandable, color-coded before→after attribute diffs.
//
// It is designed to drop into any CLI. Give it a plan and it takes over the
// terminal until the user quits — the loading (binary plan file vs. plan JSON
// vs. stdin) is handled for you. The whole integration is usually one line from
// your own flag handler:
//
//	func runPlanFlag(path string) error { return tfplantui.Run(path) }
//
// For a non-interactive overview (CI, quick triage) use [Summary]; if you
// already hold plan JSON in memory use [RunJSON].
package tfplantui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarismail/tfplantui/internal/tfplan"
	"github.com/omarismail/tfplantui/internal/ui"
)

// defaultPlanName is the source label shown in the UI when none can be derived
// from a path — notably for [RunJSON]. Override it with [WithPlanName].
const defaultPlanName = "plan"

// Option configures [Run], [RunJSON], and [Summary].
type Option func(*config)

type config struct {
	altScreen     bool
	terraformPath string // "" => look up "terraform" on PATH
	planName      string // overrides the displayed source label
	planNameSet   bool
	programOpts   []tea.ProgramOption
}

func resolve(opts []Option) config {
	c := config{altScreen: true}
	for _, o := range opts {
		o(&c)
	}
	return c
}

func (c config) planNameOr(def string) string {
	if c.planNameSet {
		return c.planName
	}
	return def
}

// WithAltScreen controls whether the TUI takes over the full screen (the
// terminal's alternate screen buffer). The default is true; pass false to keep
// the plan in the normal scrollback.
func WithAltScreen(v bool) Option {
	return func(c *config) { c.altScreen = v }
}

// WithTerraformPath overrides the terraform binary used to render a *binary*
// plan file via "terraform show -json". By default "terraform" is looked up on
// PATH. It is ignored when the input is already plan JSON or stdin.
func WithTerraformPath(path string) Option {
	return func(c *config) { c.terraformPath = path }
}

// WithPlanName overrides the source label shown in the UI. By default this is
// the base name of the plan file ("stdin" for "-", or "plan" for [RunJSON],
// which has no path to derive a name from).
func WithPlanName(name string) Option {
	return func(c *config) { c.planName = name; c.planNameSet = true }
}

// WithProgramOptions appends extra Bubble Tea program options (for example
// tea.WithMouseCellMotion). The alternate-screen option is managed by
// [WithAltScreen] and should not be passed here.
func WithProgramOptions(opts ...tea.ProgramOption) Option {
	return func(c *config) { c.programOpts = append(c.programOpts, opts...) }
}

// Run loads a Terraform plan from path and launches the interactive TUI,
// blocking until the user quits. path may be:
//
//   - a binary plan file from "terraform plan -out=plan.tfplan" — "terraform
//     show -json" is invoked to render it (see [WithTerraformPath]),
//   - the JSON from "terraform show -json plan.tfplan", or
//   - "-" to read plan JSON from stdin.
func Run(path string, opts ...Option) error {
	return RunContext(context.Background(), path, opts...)
}

// RunContext is [Run] with a context: cancelling ctx tears the UI down and
// returns. Normal quit (q / Ctrl-C) is unaffected. Use it when embedding the
// explorer in a longer-lived program that owns signal handling or a deadline.
func RunContext(ctx context.Context, path string, opts ...Option) error {
	cfg := resolve(opts)
	g, err := load(path, cfg)
	if err != nil {
		return err
	}
	return runGraph(ctx, g, cfg)
}

// RunJSON launches the interactive TUI on plan JSON (the output of "terraform
// show -json") that you already hold in memory, blocking until the user quits.
// The source label defaults to "plan"; override it with [WithPlanName].
func RunJSON(planJSON []byte, opts ...Option) error {
	return RunJSONContext(context.Background(), planJSON, opts...)
}

// RunJSONContext is [RunJSON] with a context; see [RunContext] for the semantics.
func RunJSONContext(ctx context.Context, planJSON []byte, opts ...Option) error {
	cfg := resolve(opts)
	g, err := tfplan.Parse(planJSON)
	if err != nil {
		return err
	}
	g.PlanPath = cfg.planNameOr(defaultPlanName)
	return runGraph(ctx, g, cfg)
}

// Summary writes a concise, non-interactive text overview of the plan at path
// to w instead of launching the TUI. path accepts the same forms as [Run].
// Color is emitted only when w is a terminal.
func Summary(w io.Writer, path string, opts ...Option) error {
	cfg := resolve(opts)
	g, err := load(path, cfg)
	if err != nil {
		return err
	}
	ui.PrintSummary(w, g)
	return nil
}

func runGraph(ctx context.Context, g *tfplan.Graph, cfg config) error {
	progOpts := make([]tea.ProgramOption, 0, len(cfg.programOpts)+2)
	progOpts = append(progOpts, tea.WithContext(ctx))
	if cfg.altScreen {
		progOpts = append(progOpts, tea.WithAltScreen())
	}
	// Caller options come last so they can override the managed defaults.
	progOpts = append(progOpts, cfg.programOpts...)
	_, err := tea.NewProgram(ui.New(g), progOpts...).Run()
	return err
}

// load reads, parses, and labels a plan from a path that may be plan JSON, a
// binary plan file, or "-" for stdin.
func load(path string, cfg config) (*tfplan.Graph, error) {
	data, src, err := readPlan(path, cfg)
	if err != nil {
		return nil, err
	}
	g, err := tfplan.Parse(data)
	if err != nil {
		return nil, err
	}
	g.PlanPath = cfg.planNameOr(src)
	return g, nil
}

// readPlan returns plan JSON for a path that may be JSON, a binary plan file, or "-".
func readPlan(path string, cfg config) (data []byte, src string, err error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		return b, "stdin", nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	src = filepath.Base(path)

	// JSON already? (terraform show -json output)
	if looksLikeJSON(raw) {
		return raw, src, nil
	}

	// Otherwise treat it as a binary plan file and ask terraform to render it.
	jsonOut, err := terraformShowJSON(path, cfg.terraformPath)
	if err != nil {
		return nil, "", err
	}
	return jsonOut, src, nil
}

func looksLikeJSON(b []byte) bool {
	for _, c := range b {
		switch c {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

func terraformShowJSON(planPath, tfBinary string) ([]byte, error) {
	tf := tfBinary
	if tf == "" {
		var err error
		tf, err = exec.LookPath("terraform")
		if err != nil {
			return nil, fmt.Errorf("%q is not plan JSON and terraform was not found in PATH.\n"+
				"  Convert it first:  terraform show -json %s > plan.json\n"+
				"  then point tfplantui at plan.json", planPath, planPath)
		}
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(tf, "show", "-json", planPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("terraform show -json %s failed: %s", planPath, msg)
	}
	return stdout.Bytes(), nil
}
