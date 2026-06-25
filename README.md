# tfplantui

An interactive terminal UI for reading HashiCorp Terraform plans.

A Terraform configuration is a **graph**, and a plan is a **proposed change over
that graph**. Scrolling through thousands of lines of `terraform plan` output
makes that structure almost impossible to see. `tfplantui` renders the plan as
its dependency graph — laid out in columns by dependency depth — and lets you
walk it with the arrow keys, expanding any resource to see a color-coded
before → after diff of its attributes.

```
 Layer 0                          Layer 1                          Layer 2
╭──────────────────────────────╮  ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓  ╭──────────────────────────────╮
│ · random_id.seed             │  ┃ ± random_string.api_key ×4   ┃  │ ± random_password.master     │
╰──────────────────────────────╯  ┃ keepers:                     ┃  ╰──────────────────────────────╯
╭──────────────────────────────╮  ┃   ~ rotation = "v1" → "v2"   ┃  ╭──────────────────────────────╮
│ · random_pet.org             │  ┃ ~ result = "abc…" → (known…) ┃  │ ± random_pet.release         │
╰──────────────────────────────╯  ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛  ╰──────────────────────────────╯
```

## Why

`terraform plan` answers *"what will change?"* but not *"how do these changes
relate?"*. When a single edited value forces a cascade of replacements, the flat
output gives you no way to follow the blast radius. `tfplantui` is built around
one idea: **let a human reason about a plan by navigating its graph**, not by
reading a transcript.

- **Dependency layout.** Resources with no dependencies sit on the left; each
  column to the right depends on the one before it. Moving `←`/`→` follows the
  edges, so you can trace *what depends on what*.
- **Expand in place.** Hit `space` on any resource to unfold its attribute diff,
  color-coded the way you'd expect: green additions, red removals, amber
  in-place changes, and `(known after apply)` for computed values.
- **Built for small *and* large plans.** A six-resource plan and a
  400-instance, five-layer plan use the same view; columns scroll vertically,
  the layout scrolls horizontally, and a flat **list view** (`t`) plus a
  **changes-only** filter (`n`) keep big plans tractable.

## Install

As a CLI:

```sh
go install github.com/omarismail/tfplantui/cmd/tfplantui@latest
# or build from a checkout
go build -o tfplantui ./cmd/tfplantui
```

Requires Go 1.24+. Terraform is only needed if you hand it a *binary* plan file
(see below); reading plan JSON needs nothing else.

## Use as a library

`tfplantui` is also a package, so any Go CLI can embed the same plan explorer
behind its own flag. Hand it a plan file and the rest — loading binary plans via
`terraform show -json`, parsing, layout, and the interactive UI — is handled for
you:

```go
import "github.com/omarismail/tfplantui"

// In your own command / flag handler:
func runPlanExplorer(planPath string) error {
	// planPath may be a binary plan file, plan JSON, or "-" for stdin.
	return tfplantui.Run(planPath)
}
```

The public surface is intentionally small:

```go
// Launch the interactive TUI, blocking until the user quits.
func Run(path string, opts ...Option) error
// Same, but on plan JSON you already hold in memory.
func RunJSON(planJSON []byte, opts ...Option) error
// Write the non-interactive text summary instead (handy for CI).
func Summary(w io.Writer, path string, opts ...Option) error

// Options: WithAltScreen, WithTerraformPath, WithPlanName, WithProgramOptions
```

## Usage

`tfplantui` reads the JSON form of a plan (`terraform show -json`). You can
point it at any of three things:

```sh
# 1. a binary plan file — tfplantui runs `terraform show -json` for you
terraform plan -out=plan.tfplan
tfplantui plan.tfplan

# 2. plan JSON you produced yourself
terraform show -json plan.tfplan > plan.json
tfplantui plan.json

# 3. piped on stdin
terraform show -json plan.tfplan | tfplantui -

# non-interactive text summary (great for CI / quick triage)
tfplantui -summary plan.json
```

Try it immediately from a checkout — no install needed:

```sh
go run ./cmd/tfplantui examples/medium/plan.json
```

## Keys

| Key | Action |
|-----|--------|
| `↑ ↓` / `k j` | move within a dependency layer (column) |
| `← →` / `h l` | move across layers, following dependency edges |
| `space` / `enter` | expand / collapse the selected resource's diff |
| `[` `]` | cycle instances of a `count`/`for_each` resource |
| `u` | show / hide unchanged attributes |
| `n` | changes-only — hide `no-op` resources |
| `x` | collapse everything |
| `t` | switch between graph and list views |
| `g` / `G` | jump to top / bottom of the column or list |
| `/` | search by resource address (`Enter` jumps, `Esc` cancels) |
| `?` | toggle help |
| `q` / `Ctrl-C` | quit |

## Color legend

Color carries one thing — the **action** — so it stays legible at a glance:

| Glyph | Meaning |
|-------|---------|
| `+` green | create |
| `±` purple | replace (destroy then create) |
| `-` red | destroy |
| `~` amber | update in place |
| `*` yellow | mixed — a `count`/`for_each` block whose instances differ |
| `·` dim | no change (recedes into the background) |

Everything else is deliberately neutral so it doesn't compete with the action colors:

- **The cursor** is a dark card with bright white text — it reads by contrast, not hue.
- **Unchanged (no-op) resources** are dimmed almost to the background; they only
  brighten when you move the cursor onto them.
- **Dependencies are drawn on demand:** selecting a resource draws arrows from its
  dependencies into it and out to its dependents (`─▶`), instead of recoloring
  other nodes. The footer also lists them by name.

## Examples

Three plans are checked in under `examples/`, all using only the
`hashicorp/random` provider so they are fully local and reproducible:

| Example | Shape | Changes |
|---------|-------|---------|
| `simple` | 6 resources, 3 layers, light dependencies | a fresh plan — all creates |
| `medium` | ~10 blocks / 28 instances, wide + 3 layers | create / replace / delete / no-op |
| `large`  | 64 blocks / ~400 instances, very wide + 5 layers | ~200 to add, ~180 to destroy, 138 replace |

Each `plan.json` is committed, so the examples run without Terraform. To
regenerate them (this *does* require Terraform):

```sh
examples/generate.sh all          # or: simple | medium | large
```

Each example directory holds:

- `providers.tf` — the `required_providers` block (always loaded),
- `main.tf` — the **after** world (what the plan ends at; this is the readable config),
- `before.tf.txt` — the optional **before** world, applied first to seed prior
  state so the plan contains updates/replaces/deletes (it is not a `.tf` file, so
  Terraform ignores it until `generate.sh` swaps it in),
- `plan.json` — the generated, committed artifact.

The `large` example's configs are emitted by `examples/large/gen_config.py`.

## How it works

- **Parse** (`internal/tfplan`): reads `resource_changes` and the `configuration`
  block from the plan JSON. Instances (`count`/`for_each`) are grouped into their
  resource *block*, which becomes one graph node.
- **Graph**: dependency edges come from the references Terraform records in each
  resource's `configuration.expressions`. Nodes are assigned to columns by
  longest dependency path (a Sugiyama-style layering), so depth reads
  left → right.
- **Diff** (`internal/tfplan/diff.go`): walks `before` / `after` together,
  threading `after_unknown` (for `(known after apply)`) and the
  `before_sensitive`/`after_sensitive` trees (sensitive values are redacted, never
  printed), recursing into nested maps such as `keepers`.
- **UI** (`internal/ui`): a [Bubble Tea](https://github.com/charmbracelet/bubbletea)
  model. Rendering is windowed — only the visible columns and rows are drawn — so
  large plans stay responsive.

## Limitations

- Cross-module dependency edges that pass through a module **input variable**
  (e.g. `module.x.foo` wired from a root resource) are not drawn; intra-module
  edges and direct resource references are. Resources are always shown; only that
  one class of edge may be missing.
- The bundled examples use the `random` provider, whose attributes are almost all
  force-new, so they demonstrate create/replace/delete rather than many in-place
  `~` updates. In-place updates render correctly (see the tests) — point the tool
  at a real plan to see them.

## Development

```sh
go test ./...                                   # unit + headless-TUI tests
TFPLAN_DUMP=1 go test ./internal/ui -run TestDumpVisual -v   # eyeball the layout
```

## License

MIT — see [LICENSE](LICENSE).
