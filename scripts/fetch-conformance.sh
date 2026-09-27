#!/usr/bin/env bash
# Fetches the public fixtures of duva-mail/duva-conformance (generated and tested in the duva
# repository: see docs/bibliotheques-clientes.md section 6). Never committed here (see
# .gitignore): always the freshest version, never a copy that could silently drift.
#
#   bash scripts/fetch-conformance.sh
set -euo pipefail
cd "$(dirname "$0")/.."

base="https://raw.githubusercontent.com/duva-mail/duva-conformance/main"
mkdir -p conformance
for name in webhooks.json requests.json retries.json; do
  curl -fsSL "${base}/${name}" -o "conformance/${name}"
  echo "conformance/${name} fetched"
done
