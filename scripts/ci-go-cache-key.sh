#!/usr/bin/env bash
# This is a compatibility boundary, not an ABI-minimization heuristic.
set -euo pipefail
export LC_ALL=C
manifest=${1:?usage: ci-go-cache-key.sh MANIFEST}
{
  printf 'schema=azerlay-test-build-v1\n'
  cat /etc/os-release
  uname -m
  pacman -Q | sort
  go version
  go tool compile -V=full
  # setup-go selects the executable; hash its bytes as well as its version.
  sha256sum "$(command -v go)"
  go env -json GOVERSION GOOS GOARCH GOAMD64 GOARM GOARM64 GO386 \
    GOTOOLCHAIN GOEXPERIMENT CGO_ENABLED CC CXX CGO_CFLAGS CGO_CPPFLAGS \
    CGO_CXXFLAGS CGO_FFLAGS CGO_LDFLAGS GOFLAGS GOCACHEPROG
  pkg-config --modversion gtk4 gtk4-layer-shell-0
  pkg-config --cflags --libs gtk4 gtk4-layer-shell-0
} > "$manifest"
compatibility=$(sha256sum "$manifest" | cut -d' ' -f1)
dependencies=$(sha256sum go.mod go.sum | sha256sum | cut -d' ' -f1)
# Tracked blobs include embedded assets and test fixtures, not private runtime data.
# One snapshot per source tree, rather than one per run/attempt/commit message.
source=$(git ls-files -s -z | sha256sum | cut -d' ' -f1)
prefix="azerlay-test-build-v1-${compatibility}-${dependencies}-"
printf 'compatibility=%s\ndependencies=%s\nsource=%s\nprefix=%s\nkey=%s%s\n' \
  "$compatibility" "$dependencies" "$source" "$prefix" "$prefix" "$source"
