#!/usr/bin/env bash
# Optional diagnostic capture. Use a dedicated directory outside the checkout.
# Do not dump env, HOME, raw profiles, tokens, or fuzz inputs.
set -euo pipefail
if [[ $# != 1 || -e $1 ]]; then
  printf 'usage: %s NEW_OUTPUT_DIRECTORY\n' "$0" >&2
  exit 2
fi
mkdir -p -- "$1"
out=$(cd -- "$1" && pwd)
trap 'printf "capture failed; partial output: %s\n" "$out" >&2' ERR
# Standalone assignments propagate failures; printf "$(command)" would mask them.
captured_at=$(date -u +%FT%TZ)
commit=$(git rev-parse HEAD)
cpu_count=$(getconf _NPROCESSORS_ONLN)
{
  printf 'captured_at=%s\n' "$captured_at"
  printf 'commit=%s\n' "$commit"
  printf 'runner_os=%s\nrunner_arch=%s\n' "${RUNNER_OS:-unknown}" "${RUNNER_ARCH:-unknown}"
  printf 'run_id=%s\nrun_attempt=%s\njob=%s\n' "${GITHUB_RUN_ID:-local}" "${GITHUB_RUN_ATTEMPT:-0}" "${GITHUB_JOB:-local}"
  printf 'cpu_count=%s\n' "$cpu_count"
  uname -srmo
  go version
  go env GOOS GOARCH GOVERSION CGO_ENABLED CC GOAMD64
  pkg-config --modversion gtk4 gtk4-layer-shell-0
} > "$out/environment.txt"
if command -v dpkg-query >/dev/null; then
  LC_ALL=C dpkg-query -W -f='${binary:Package}\t${Version}\t${Architecture}\n' | LC_ALL=C sort > "$out/native-packages.txt"
else
  LC_ALL=C pacman -Q | LC_ALL=C sort > "$out/native-packages.txt"
fi
sha256sum go.mod go.sum .github/ci-tools.env > "$out/input-sha256.txt"
if [[ -f .git/ci-native-cache-input ]]; then
  cp .git/ci-native-cache-input "$out/native-cache-input.txt"
fi
# Full cache hit/key, compressed bytes, transfer and image pull/digest must be
# retained separately from Actions logs. These are only local raw sizes.
for variable in GOCACHE GOMODCACHE; do
  directory=${!variable:-}
  if [[ -n $directory && -d $directory ]]; then
    printf '%s\t' "$variable" >> "$out/cache-raw-bytes.tsv"
    du -sb -- "$directory" | cut -f1 >> "$out/cache-raw-bytes.tsv"
  fi
done
if command -v lscpu >/dev/null; then lscpu > "$out/cpu.txt"; fi
