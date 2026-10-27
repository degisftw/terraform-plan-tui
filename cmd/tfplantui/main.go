// Command tfplantui is an interactive terminal UI for exploring a Terraform
// plan as a dependency graph with color-coded, expandable attribute diffs.
//
// It is a thin wrapper over the github.com/omarismail/terraform-plan-tui package; the
// loading, parsing, and rendering all live there so other CLIs can reuse them.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/omarismail/terraform-plan-tui"
)

const usage = `tfplantui — an interactive terminal UI for Terraform plans

USAGE
  tfplantui [flags] <plan>

  <plan> is either:
    • a binary plan file from "terraform plan -out=plan.tfplan"
      (tfplantui runs "terraform show -json" on it for you), or
    • the JSON from "terraform show -json plan.tfplan > plan.json", or
    • "-" to read plan JSON from stdin.

FLAGS
  -summary    print a non-interactive text summary instead of the TUI
  -h, -help   show this help

KEYS (in the TUI)
  arrows/hjkl  navigate the dependency graph (←→ follows edges across layers)
  space        expand/collapse a resource to see its before→after diff
  f            enlarge centered card, keeping neighbors visible; Esc restores
  left/right   move between dependency levels, including enlarged card (h/l too)
  Tab/S-Tab    next/previous instance in any view ([/] still work)
  a            all instance streams on/off; ? block scope marks unknown mappings
  t            toggle graph / list view      n   changes-only (hide no-op)
  u            show/hide unchanged attrs      /   search        ?  help   q  quit

EXAMPLES
  terraform plan -out=plan.tfplan && tfplantui plan.tfplan
  terraform show -json plan.tfplan | tfplantui -
  tfplantui examples/medium/plan.json
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tfplantui: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		summary bool
		path    string
	)
	for _, a := range args {
		switch a {
		case "-h", "-help", "--help":
			fmt.Print(usage)
			return nil
		case "-summary", "--summary":
			summary = true
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return fmt.Errorf("unknown flag %q (try -help)", a)
			}
			if path != "" {
				return fmt.Errorf("unexpected extra argument %q (one plan at a time)", a)
			}
			path = a
		}
	}
	if path == "" {
		fmt.Print(usage)
		return nil
	}

	if summary {
		return tfplantui.Summary(os.Stdout, path)
	}
	return tfplantui.Run(path)
}
