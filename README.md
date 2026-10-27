# tfplantui

An interactive terminal UI for reading Terraform plans.

![tfplantui demo](docs/demo.gif)

A Terraform configuration is a **graph**, and a plan is a **proposed change over
that graph**. Scrolling through hundrends of lines of `terraform plan` output
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

`terraform plan` gives you its answer as **plain text** — and for any real
change that means hundreds of lines of diff to scroll through, or the raw plan
JSON to parse yourself. Either way, *you* are the one reconstructing the plan's
shape in your head. `tfplantui` does that for you: it turns the plan into a
**graph you can navigate**, so a human can visualize and reason through every
change at once instead of reading a transcript.

- **Dependency layout.** Resources with no dependencies sit on the left; each
  column to the right depends on the one before it. Moving `←`/`→` follows the
  edges, so you can trace *what depends on what*.
- **Expand in place.** Hit `space` on any resource to unfold its attribute diff,
  color-coded the way you'd expect: green additions, red removals, amber
  in-place changes, and `(known after apply)` for computed values.
- **Read the whole resource.** Press `f` on the focused resource to enlarge and
  center its card in the graph, keeping neighboring dependency layers and arrows
  visible on the left and right. Long names and values wrap, and collections are
  indented. Scroll through every attribute with `↑`/`↓` (`j`/`k`), `PgUp`/`PgDn`,
  or `g`/`G`. Use `Tab` / `Shift+Tab` to cycle to the next / previous instance
  in any view; `[` / `]` remain available. Inside the enlarged card, `←` / `→`
  (`h` / `l`) move between dependency levels. Press `u` to include unchanged attributes.
  Sensitive values remain redacted. Press `Esc` or `f` to restore the previous
  layout and selection. Also available from list view.
- **All instance streams.** Press `a` to toggle between grouped resource blocks
  and individual instance cards. Confirmed upstream addresses stay explicit;
  relationships without an instance mapping are marked `? block scope:` rather
  than guessed from matching indexes. The toggle also works in the enlarged card.
- **Built for small *and* large plans.** A six-resource plan and a
  400-instance, five-layer plan use the same view; columns scroll vertically,
  the layout scrolls horizontally, and a flat **list view** (`t`) plus a
  **changes-only** filter (`n`) keep big plans tractable.
- **Readable by design.** A Posting-inspired palette pairs clear text with
  indigo panels and an orchid focus accent. Light and dark terminal backgrounds
  get matching palettes; unchanged resources and keyboard hints stay readable.

## Install

As a CLI:

```sh
go install github.com/omarismail/terraform-plan-tui/cmd/tfplantui@latest
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
import "github.com/omarismail/terraform-plan-tui"

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
| `f` | enlarge / restore the focused card, keeping neighboring resources visible |
| `a` | toggle all instance streams; opens the graph when used from the list |
| `Esc` | close help or restore the enlarged card |
| `↑ ↓` / `k j` (enlarged card) | scroll through attributes |
| `← →` / `h l` (enlarged card) | move between dependency levels, keeping the card open |
| `PgUp` / `PgDn` (enlarged card) | scroll a page (`Ctrl-U` / `Ctrl-D` also work) |
| `[` `]` | cycle instances of a `count`/`for_each` resource |
| `Tab` / `Shift+Tab` | next / previous instance in graph, list, and enlarged card |
| `u` | show / hide unchanged attributes |
| `n` | changes-only — hide `no-op` resources |
| `x` | collapse everything |
| `t` | switch between graph and list views |
| `g` / `G` | jump to top / bottom of the column, list, or enlarged card |
| `/` | search by resource address (`Enter` jumps, `Esc` cancels) |
| `?` | toggle help |
| `q` / `Ctrl-C` | quit |

## Instance streams

With `a` enabled, every visible `count` / `for_each` instance gets its own card,
action, and address. `↑` / `↓` walks the instance cards in a layer; `Tab` /
`Shift+Tab` (or `[` / `]`) cycles within the selected resource block. Search can
jump directly to an indexed address such as `random_integer.port[3]`.

- **Confirmed links** come from explicit indexed references or an upstream block
  with one planned instance. Visible adjacent columns draw these links; each
  dependent card also names its confirmed upstream addresses, including links
  outside the viewport or across non-adjacent layers. A shared rail can carry
  several links, so the address annotations identify the individual targets.
- **Grouped unknowns** are labeled `? block scope: <address>`. An unindexed
  multi-instance reference or dynamic index does not establish a one-to-one
  relationship. The UI never assumes that instance `[2]` depends on `[2]`.
- `space` expands only the selected instance's preview. `n` hides unchanged
  instances, and switching `a` off preserves the selected instance and the
  previous compact previews.

This remains a configuration-derived graph: expressions are shared by instances
of their declaring block, not individually evaluated. Cross-module output
references remain outside the current dependency parser, and expressions omitted
from Terraform's JSON (such as dynamic blocks) cannot produce links.


## Color legend

Action symbols and changed values use semantic colors so plans remain scannable:

| Glyph | Meaning |
|-------|---------|
| `+` green | create |
| `±` purple | replace (destroy then create) |
| `-` red | destroy |
| `~` amber | update in place |
| `*` yellow | mixed — a `count`/`for_each` block whose instances differ |
| `·` slate | no change (quieter, but still readable) |

The interface separates focus from resource actions:

- **The focused card** has an orchid border, a distinct surface, and a bold name.
  Other cards keep neutral borders; their action symbols retain semantic colors.
- **Attributes** use readable labels and neutral values. Before/after changes
  use red and green, and computed values use amber.
- **Unchanged resources** use readable secondary text rather than near-black text.
- **The footer** shows contextual keycaps that fit the terminal width, with help
  and quit kept visible. Press `?` for every shortcut.
- **Light and dark palettes** are selected from the terminal background.
- **Dependencies:** compact mode draws block connections for the selection.
  Instance-stream mode draws confirmed references between visible instances and
  annotates every dependent card with confirmed or unresolved upstream scope.

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
