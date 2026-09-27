#!/usr/bin/env bash
# Regenerates internal/generated/types.go from docs/openapi.json in the duva repository
# (docs/bibliotheques-clientes.md section 5: models generated, client hand-written).
#
# oapi-codegen itself needs Go 1.25+ to build (independent of this MODULE's own go.mod, which
# stays at 1.23): use a newer toolchain to run this script, e.g. `docker run --rm -v "$PWD":/app
# -v <path to a duva checkout>:/duva -w /app golang:1.25 bash scripts/generate.sh /duva`.
#
# `-generate types,skip-prune`: WebhookEvent is only referenced through the webhook callback
# (what Duva sends to YOUR endpoint, not a request oapi-codegen sees a client making), so its
# pruner drops the schema itself while still emitting an alias to it unless pruning is disabled.
# `-exclude-operation-ids receiveDeliveryEvent` drops the unused param/body aliases for that same
# callback pseudo-operation, which duva-go never uses (it has its own WebhookEvent, see
# webhook_verify.go).
#
#   bash scripts/generate.sh [path to a local duva checkout; defaults to ../duva]
set -euo pipefail
cd "$(dirname "$0")/.."

duva_repo="${1:-../duva}"
spec="${duva_repo}/docs/openapi.json"
if [ ! -f "$spec" ]; then
  echo "OpenAPI spec not found at ${spec} (pass the duva repo path as an argument)" >&2
  exit 1
fi

go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
"$(go env GOPATH)/bin/oapi-codegen" \
  -generate types,skip-prune \
  -package generated \
  -exclude-operation-ids receiveDeliveryEvent \
  -o internal/generated/types.go \
  "$spec"
gofmt -w internal/generated/types.go
echo "internal/generated/types.go regenerated from ${spec}"
