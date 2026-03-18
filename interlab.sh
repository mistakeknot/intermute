#!/usr/bin/env bash
set -euo pipefail
# core/intermute/interlab.sh — wraps Intermute Go benchmarks for interlab consumption.
# Primary metric: glob_overlap_ns (BenchmarkPatternsOverlapWildcard)
# Secondary: name_gen_ns

MONOREPO="$(cd "$(dirname "$0")/../.." && pwd)"
HARNESS="${INTERLAB_HARNESS:-$MONOREPO/interverse/interlab/scripts/go-bench-harness.sh}"
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "--- glob overlap ---" >&2
bash "$HARNESS" --pkg ./internal/glob/ --bench 'BenchmarkPatternsOverlapWildcard$' --metric glob_overlap_ns --dir "$DIR"

echo "--- name generation ---" >&2
bash "$HARNESS" --pkg ./internal/names/ --bench 'BenchmarkGenerate$' --metric name_gen_ns --dir "$DIR"
