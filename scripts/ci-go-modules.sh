#!/usr/bin/env bash
# Shared application dependencies only; analyzer modules have separate caches.
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
start=$(date +%s%N)
go mod download
go mod verify
git -c safe.directory="$root" diff --exit-code -- go.mod go.sum
printf 'app modules: %s ms\n' "$(( ($(date +%s%N) - start) / 1000000 ))"
printf 'module cache bytes: '
du -sb "$(go env GOMODCACHE)" | cut -f1
