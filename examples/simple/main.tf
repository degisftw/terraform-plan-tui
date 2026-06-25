# Simple example: a handful of resources with light dependencies.
# This is a fresh plan (no prior state), so every resource is a "create".
#
#   random_id.seed ────────► random_pet.app ───┐
#                                               ├─► random_password.db
#   random_integer.port ──► random_string.token┘
#
#   random_uuid.request_id   (independent leaf)

# Layer 0 — roots (no dependencies)
resource "random_id" "seed" {
  byte_length = 8
}

resource "random_integer" "port" {
  min = 8000
  max = 9000
}

resource "random_uuid" "request_id" {}

# Layer 1 — depend on a single root
resource "random_pet" "app" {
  length    = 2
  separator = "-"
  keepers = {
    seed = random_id.seed.hex
  }
}

resource "random_string" "token" {
  length  = 16
  special = false
  keepers = {
    port = random_integer.port.result
  }
}

# Layer 2 — depends on two layer-1 resources
resource "random_password" "db" {
  length = 20
  keepers = {
    app   = random_pet.app.id
    token = random_string.token.id
  }
}
