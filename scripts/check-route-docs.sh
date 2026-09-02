#!/usr/bin/env bash
# Every mux.Handle path in router_domain.go must appear in README.md or docs/api.md.
set -euo pipefail
cd "$(dirname "$0")/.."
missing=0
while read -r path; do
  key="${path%/}"            # strip trailing slash: /api/agents/ -> /api/agents
  if ! grep -q -- "$key" README.md docs/api.md; then
    echo "undocumented route: $path"; missing=1
  fi
done < <(grep -o 'mux.Handle\(Func\)\?("[^"]*"' internal/http/router_domain.go | sed 's/.*("//;s/"$//')
exit $missing
