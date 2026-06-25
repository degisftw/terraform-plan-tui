#!/usr/bin/env bash
#
# Regenerate the plan.json artifact for an example.
#
#   ./generate.sh <example-dir>      # e.g. ./generate.sh simple
#   ./generate.sh all                # regenerate every example
#
# Each example directory contains:
#   providers.tf      terraform{} + required_providers (always loaded)
#   main.tf           the "after" config — what the plan ends at (human-readable)
#   before.tf.txt     optional "before" config used to seed prior state, so the
#                     plan shows updates/replaces/deletes (ignored by terraform
#                     because it is not a .tf file)
#   plan.json         generated artifact, committed so the TUI runs offline
#
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

gen_one() {
  local dir="$1"
  local path="$here/$dir"
  [ -d "$path" ] || { echo "no such example: $dir" >&2; return 1; }
  echo "==> $dir"
  (
    cd "$path"
    rm -rf .terraform terraform.tfstate terraform.tfstate.backup plan.tfplan .after.tf.bak
    terraform init -input=false -no-color >/dev/null

    if [ -s before.tf.txt ]; then
      # Seed prior state from the "before" world, then restore "after".
      # Crash-safe: keep a copy of the canonical main.tf and restore it on ANY
      # exit (including a failed apply), so main.tf is never stranded.
      cp main.tf .after.tf.bak
      trap 'cp -f .after.tf.bak main.tf 2>/dev/null; rm -f .after.tf.bak' EXIT
      cp before.tf.txt main.tf
      terraform apply -auto-approve -input=false -no-color >/dev/null
      cp -f .after.tf.bak main.tf
      rm -f .after.tf.bak
      trap - EXIT
    fi

    terraform plan -out=plan.tfplan -input=false -no-color >/dev/null
    terraform show -json plan.tfplan >plan.json
    rm -f plan.tfplan
    echo "    wrote $dir/plan.json ($(wc -c <plan.json | tr -d ' ') bytes)"
  )
}

if [ "${1:-}" = "all" ] || [ -z "${1:-}" ]; then
  for d in simple medium large; do
    [ -d "$here/$d" ] && gen_one "$d"
  done
else
  gen_one "$1"
fi
