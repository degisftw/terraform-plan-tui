# Medium example — a wider, slightly deeper graph with moderate changes.
#
# "after" world (this file). The "before" world lives in before.tf.txt and is
# applied first by generate.sh, so the plan contains a mix of:
#   no-op    unchanged foundation + most service instances
#   create   one extra service + a new trace_id resource
#   replace  api_key rotation bump, widened port range, and their dependents
#   delete   a dropped db_password instance + the removed legacy shuffle

# ── Layer 0 — platform foundation (unchanged → no-op) ──────────────────────
resource "random_id" "seed" {
  byte_length = 8
}

resource "random_pet" "org" {
  length = 1
}

# ── Layer 1 — services (wide) ──────────────────────────────────────────────

# 6 services; the before world had 5, so instance [5] is a fresh create.
resource "random_pet" "service" {
  count  = 6
  length = 2
  keepers = {
    seed = random_id.seed.hex
  }
}

# API keys; rotation bumped v1 -> v2 forces a replace of all four.
resource "random_string" "api_key" {
  count   = 4
  length  = 24
  special = false
  keepers = {
    rotation = "v2"
    org      = random_pet.org.id
  }
}

# Ports; the max range widened (8000-9000 -> 8000-9500) forces a replace.
resource "random_integer" "port" {
  count = 5
  min   = 8000
  max   = 9500
  keepers = {
    seed = random_id.seed.hex
  }
}

# DB passwords; before world had 4, so instance [3] is destroyed.
resource "random_password" "db_password" {
  count   = 3
  length  = 20
  special = true
  keepers = {
    seed = random_id.seed.hex
  }
}

# Brand new in the after world → pure creates.
resource "random_uuid" "trace_id" {
  count = 3
  keepers = {
    org = random_pet.org.id
  }
}

# ── Layer 2 — depend on layer-1 resources (depth) ──────────────────────────

# api_key[0] is replaced, so its id changes and master is replaced in turn.
resource "random_password" "master" {
  length = 32
  keepers = {
    api_key = random_string.api_key[0].id
    org     = random_pet.org.id
  }
}

# port[0] is replaced, so release is replaced as well.
resource "random_pet" "release" {
  length = 3
  keepers = {
    service = random_pet.service[0].id
    port    = random_integer.port[0].result
  }
}
