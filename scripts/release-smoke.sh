#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)); then
  printf 'Usage: %s BINARY_ARCHIVE VERSION\n' "$0" >&2
  exit 2
fi
archive=$1
version=$2
if [[ ! "$version" =~ ^[0-9]+(\.[0-9]+)+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
  printf 'VERSION must be numeric and dotted, with an optional prerelease suffix.\n' >&2
  exit 2
fi

stage=$(mktemp -d /tmp/azerlay-release-smoke-XXXXXXXX)
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
tar -xzf "$archive" -C "$stage"
shopt -s nullglob
binaries=("$stage"/azerlay-*/bin/azerlay)
if ((${#binaries[@]} != 1)) || [[ ! -x "${binaries[0]}" ]]; then
  printf 'Archive must contain one executable at azerlay-*/bin/azerlay.\n' >&2
  exit 1
fi
binary=${binaries[0]}

readelf -d "$binary"
ldd "$binary" > "$stage/ldd.txt"
cat -- "$stage/ldd.txt"
if grep -q 'not found' "$stage/ldd.txt"; then
  printf 'Binary has unresolved runtime dependencies.\n' >&2
  exit 1
fi

mkdir -p -- "$stage/home" "$stage/config" "$stage/cache" "$stage/data" "$stage/state" "$stage/runtime"
chmod 700 "$stage/runtime"
runtime_env=(env -i PATH=/usr/bin:/bin HOME="$stage/home"
  XDG_CONFIG_HOME="$stage/config" XDG_CACHE_HOME="$stage/cache"
  XDG_DATA_HOME="$stage/data" XDG_STATE_HOME="$stage/state"
  XDG_RUNTIME_DIR="$stage/runtime")
"${runtime_env[@]}" "$binary" version > "$stage/version.txt"
# Keep the trailing newline when comparing command output.
actual_version=$(cat -- "$stage/version.txt"; printf '.')
expected_version=$(printf 'azerlay %s\n.' "$version")
if [[ "$actual_version" != "$expected_version" ]]; then
  printf 'Binary version does not match expected azerlay %s.\n' "$version" >&2
  cat -- "$stage/version.txt" >&2
  exit 1
fi
cat -- "$stage/version.txt"
"${runtime_env[@]}" "$binary" --help
printf 'Release CLI startup and loader smoke passed.\n'
