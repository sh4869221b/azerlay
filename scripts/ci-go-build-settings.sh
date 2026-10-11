#!/usr/bin/env bash
# Capture effective settings after selecting Go and the native environment.
# Source/module contents and per-command flags (including -race) are checked
# by Go itself. External C libraries are covered by the native fingerprint.
set -euo pipefail
go env -json GOVERSION GOOS GOARCH GOAMD64 GOARM GOARM64 GO386 GOMIPS GOMIPS64 \
  GOPPC64 GORISCV64 GOWASM GOEXPERIMENT CGO_ENABLED CC CXX \
  CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_FFLAGS CGO_LDFLAGS GOFLAGS
# Nix's compiler wrappers read these independently of go env.
for setting in NIX_CC NIX_CFLAGS_COMPILE NIX_LDFLAGS NIX_ENFORCE_PURITY \
  CPATH C_INCLUDE_PATH CPLUS_INCLUDE_PATH LIBRARY_PATH \
  PKG_CONFIG PKG_CONFIG_SYSROOT_DIR; do
  printf '%s=%s\n' "$setting" "${!setting-}"
done
