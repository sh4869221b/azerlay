#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)) || [[ -z "$2" ]]; then
  printf 'Usage: %s VERSION OUTPUT_DIR\n' "$0" >&2
  exit 2
fi
version=$1
if [[ ! "$version" =~ ^[0-9]+(\.[0-9]+)+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
  printf 'VERSION must be numeric and dotted, with an optional prerelease suffix.\n' >&2
  exit 2
fi
if [[ $(uname -s) != Linux || $(uname -m) != x86_64 ]]; then
  printf 'Native packaging requires Linux x86_64.\n' >&2
  exit 1
fi
for command in go pkg-config tar; do
  if ! command -v "$command" >/dev/null; then
    printf 'Native packaging requires %s.\n' "$command" >&2
    exit 1
  fi
done
if [[ $(go env GOVERSION) != go1.27.* ]]; then
  printf 'Native packaging requires Go 1.27.x.\n' >&2
  exit 1
fi
pkg-config --print-errors --exists gtk4 gtk4-layer-shell-0 gobject-introspection-1.0

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
mkdir -p -- "$2"
output_dir=$(cd -- "$2" && pwd -P)
name="azerlay-$version"
source_archive="$output_dir/$name-source.tar.gz"
binary_archive="$output_dir/$name-linux-x86_64.tar.gz"
for archive in "$source_archive" "$binary_archive"; do
  if [[ -e "$archive" || -L "$archive" ]]; then
    printf 'Refusing to overwrite %s.\n' "$archive" >&2
    exit 1
  fi
done

stage=$(mktemp -d /tmp/azerlay-package-XXXXXXXX)
trap 'rm -rf -- "$stage"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
source_root="$stage/source/$name"
binary_root="$stage/binary/$name"
mkdir -p -- "$source_root" "$binary_root/bin" "$binary_root/integration"
excludes=()
if [[ "$output_dir" == "$repo_root/"* ]]; then
  excludes+=(--exclude="${output_dir#"$repo_root/"}")
fi
tar "${excludes[@]}" -C "$repo_root" -cf - -- \
  go.mod go.sum cmd internal scripts packaging README.md docs \
  LICENSE NOTICE THIRD_PARTY_NOTICES.md LICENSES |
  tar -C "$source_root" -xf -
(
  cd -- "$source_root"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -trimpath -buildvcs=false \
    -ldflags "-X main.version=$version" -o "$binary_root/bin/azerlay" ./cmd/azerlay
)
cp -a -- "$source_root/README.md" "$source_root/docs" \
  "$source_root/LICENSE" "$source_root/NOTICE" \
  "$source_root/THIRD_PARTY_NOTICES.md" "$source_root/LICENSES" "$binary_root/"
cp -a -- "$source_root/packaging/udev/71-azerlay.rules" \
  "$source_root/packaging/systemd/azerlay.service.in" \
  "$source_root/packaging/desktop/io.github.@OWNER@.azerlay.desktop.in" \
  "$binary_root/integration/"
tar -C "$stage/source" -czf "$stage/$name-source.tar.gz" -- "$name"
tar -C "$stage/binary" -czf "$stage/$name-linux-x86_64.tar.gz" -- "$name"
for archive in "$source_archive" "$binary_archive"; do
  if [[ -e "$archive" || -L "$archive" ]]; then
    printf 'Refusing to overwrite %s.\n' "$archive" >&2
    exit 1
  fi
done
mv -- "$stage/$name-source.tar.gz" "$stage/$name-linux-x86_64.tar.gz" "$output_dir/"
printf 'Created %s\nCreated %s\n' "$source_archive" "$binary_archive"
