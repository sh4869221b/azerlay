#!/usr/bin/env bash
# Store output identities include their build inputs, without the GUI shell.
set -euo pipefail
export LC_ALL=C
: "${AZERLAY_NIX_CI:?}"
: "${AZERLAY_NIX_BUILD_INPUTS:?Pinned native build outputs are required}"
printf 'fingerprint_schema=nix-3\n'
printf '%s\n' "$AZERLAY_NIX_BUILD_INPUTS" | sort -u
bash scripts/ci-go-build-settings.sh
cc --version
ld --version
pkg-config --version
pkg-config --modversion gtk4 gtk4-layer-shell-0 glib-2.0 pango gobject-introspection-1.0 cairo
pkg-config --cflags --libs gtk4 gtk4-layer-shell-0 glib-2.0 pango gobject-introspection-1.0 cairo
