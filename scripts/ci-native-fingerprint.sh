#!/usr/bin/env bash
# Complete native compatibility boundary; never use a partial package allowlist.
set -euo pipefail
: "${CI_CACHE_JOB:?CI_CACHE_JOB is required}"
printf 'fingerprint_schema=2\njob=%s\n' "$CI_CACHE_JOB"
cat /etc/os-release
uname -m
if command -v dpkg-query >/dev/null; then
  printf 'package_manager=dpkg\n'
  LC_ALL=C dpkg-query -W -f='${binary:Package}\t${Version}\t${Architecture}\n' | LC_ALL=C sort
  # Also pin the archive source and base-image/install recipe, including fonts.
  cat /etc/apt/sources.list.d/azerlay.sources /etc/apt/apt.conf.d/00azerlay-snapshot
elif command -v pacman >/dev/null; then
  printf 'package_manager=pacman\n'
  LC_ALL=C pacman -Q | LC_ALL=C sort
else
  printf 'Unsupported native package inventory\n' >&2
  exit 1
fi
cc --version
ld --version
pkg-config --version
pkg-config --modversion gtk4 gtk4-layer-shell-0 glib-2.0 pango
pkg-config --cflags --libs gtk4 gtk4-layer-shell-0
sha256sum .github/workflows/ci.yml scripts/ci-native-fingerprint.sh scripts/test-wayland.sh
