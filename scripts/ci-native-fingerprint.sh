#!/usr/bin/env bash
# Rolling Arch: retain the native build dependency closure, not all packages.
# Go is installed later; the cache action adds its effective build settings.
set -euo pipefail
export LC_ALL=C
printf 'fingerprint_schema=arch-3\n'
cat /etc/os-release
uname -m
python3 scripts/ci_native_packages.py
cc --version
ld --version
pkg-config --version
pkg-config --modversion gtk4 gtk4-layer-shell-0 glib-2.0 pango gobject-introspection-1.0 cairo
pkg-config --cflags --libs gtk4 gtk4-layer-shell-0 glib-2.0 pango gobject-introspection-1.0 cairo
