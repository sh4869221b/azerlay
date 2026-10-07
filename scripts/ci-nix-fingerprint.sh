#!/usr/bin/env bash
# Called inside ci-nix-run; dependency identity is the complete Nix shell closure.
set -euo pipefail
: "${CI_CACHE_JOB:?}"
: "${CI_NIX_OUTPUT:?}"
: "${AZERLAY_NIX_CI:?}"
printf 'fingerprint_schema=nix-2\njob=%s\nrole=%s\n' "$CI_CACHE_JOB" "${CI_NIX_ROLE:?}"
uname -m
cat "$CI_NIX_OUTPUT/versions.json" "$CI_NIX_OUTPUT/nix-version.txt"
LC_ALL=C sort "$CI_NIX_OUTPUT/closure-paths.txt"
go version
cc --version
ld --version
pkg-config --version
pkg-config --modversion gtk4 gtk4-layer-shell-0 glib-2.0 pango
pkg-config --cflags --libs gtk4 gtk4-layer-shell-0
fc-list --format '%{file}\n' | LC_ALL=C sort -u
sha256sum flake.nix flake.lock scripts/ci_nix_prepare.py scripts/ci-nix-install.sh scripts/ci-nix-run.sh scripts/ci-nix-session.py scripts/ci-nix-fingerprint.sh scripts/test-wayland.sh .github/workflows/ci-nix.yml
