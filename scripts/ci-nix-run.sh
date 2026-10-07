#!/usr/bin/env bash
# Launch inside `nix develop --no-update-lock-file .#ci-test (or ci-build) --command bash ...`.
# This is a controlled devShell, not a sandboxed/fully hermetic derivation.
set -euo pipefail
: "${AZERLAY_NIX_CI:?Run this command inside the pinned Nix CI shell}"
: "${AZERLAY_NIX_PATH:?}"
: "${AZERLAY_NIX_FONT_DIRS?}"
: "${CI_NIX_ROLE:?Select test or build}"
if [[ "$CI_NIX_ROLE" != test && "$CI_NIX_ROLE" != build ]]; then exit 2; fi
test "$AZERLAY_NIX_CI_ROLE" = "$CI_NIX_ROLE"
: "${AZERLAY_NIX_DATA_DIRS:?}"
: "${CI_NIX_STATE:?}"
: "${GOCACHE:?}"
: "${GOMODCACHE:?}"
for directory in "$CI_NIX_STATE" "$GOCACHE" "$GOMODCACHE"; do
  if [[ "$directory" != /* ]]; then printf 'CI paths must be absolute\n' >&2; exit 2; fi
done
if (($# == 0)); then printf 'A command is required\n' >&2; exit 2; fi
if ((EUID == 0)); then printf 'Nix comparison tests must be non-root\n' >&2; exit 1; fi
unset GOROOT GOTOOLDIR
export GOTOOLCHAIN=local CGO_ENABLED=1 GOENV=off GOFLAGS=
export GOOS=linux GOARCH=amd64 GOAMD64=v1
# Match the Nix Go builder's explicit dynamic loader selection for CGo.
: "${NIX_CC:?Pinned Nix C compiler is required}"
if [[ "$NIX_CC" != /nix/store/* || ! -f "$NIX_CC/nix-support/dynamic-linker" ]]; then
  printf 'Missing pinned C compiler dynamic linker\n' >&2
  exit 1
fi
export GO_LDSO
GO_LDSO=$(cat "$NIX_CC/nix-support/dynamic-linker")
export GOBIN="$CI_NIX_STATE/bin" GOPATH="$CI_NIX_STATE/go"
export PATH="$GOBIN:$AZERLAY_NIX_PATH"
export HOME="$CI_NIX_STATE/home"
export XDG_CONFIG_HOME="$CI_NIX_STATE/config" XDG_CACHE_HOME="$CI_NIX_STATE/cache"
export XDG_CONFIG_DIRS="$XDG_CONFIG_HOME"
export XDG_DATA_HOME="$CI_NIX_STATE/data" XDG_DATA_DIRS="$AZERLAY_NIX_DATA_DIRS"
export FONTCONFIG_FILE="$CI_NIX_STATE/fonts.conf" FONTCONFIG_PATH="$CI_NIX_STATE"
unset LD_LIBRARY_PATH LD_PRELOAD GTK_PATH GTK_MODULES GTK_IM_MODULE
unset GSETTINGS_SCHEMA_DIR GIO_EXTRA_MODULES GI_TYPELIB_PATH
unset DISPLAY WAYLAND_DISPLAY DBUS_SESSION_BUS_ADDRESS XDG_RUNTIME_DIR
unset DBUS_STARTER_ADDRESS DBUS_STARTER_BUS_TYPE AT_SPI_BUS_ADDRESS NO_AT_BRIDGE
unset GTK_A11Y GTK_THEME GTK_DEBUG GDK_DEBUG GSK_DEBUG
export GSETTINGS_BACKEND=memory
mkdir -p "$GOBIN" "$HOME" "$XDG_CONFIG_HOME" "$XDG_CACHE_HOME" "$XDG_DATA_HOME" "$GOCACHE" "$GOMODCACHE"
python3 - <<'PY'
import os
from pathlib import Path
import xml.etree.ElementTree as ET
root = ET.Element('fontconfig')
for path in filter(None, os.environ['AZERLAY_NIX_FONT_DIRS'].split(':')):
    ET.SubElement(root, 'dir').text = path
ET.SubElement(root, 'cachedir').text = os.environ['XDG_CACHE_HOME'] + '/fontconfig'
ET.ElementTree(root).write(os.environ['FONTCONFIG_FILE'], encoding='utf-8', xml_declaration=True)
PY
if [[ "$CI_NIX_ROLE" = test ]]; then
  : "${AZERLAY_NIX_ATSPI:?Pinned accessibility services are required}"
  : "${AZERLAY_NIX_DBUS:?Pinned D-Bus tools are required}"
  : "${AZERLAY_NIX_BASH:?}"
  export GTK_A11Y=atspi ATSPI_DBUS_IMPLEMENTATION=dbus-daemon
  python3 "$(dirname "${BASH_SOURCE[0]}")/ci-nix-session.py"
  # Preserve scripts/test-wayland.sh; this shim supplies an isolated session
  # config and fails before its test command if real AT-SPI activation fails.
  export PATH="$CI_NIX_STATE/session-bin:$PATH"
fi
exec "$@"
