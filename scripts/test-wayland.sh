#!/usr/bin/env bash
set -euo pipefail

if (($# == 0)); then
  printf 'Usage: %s COMMAND [ARG...]\n' "$0" >&2
  exit 2
fi
for command in sway dbus-run-session; do
  if ! command -v "$command" >/dev/null; then
    printf 'Wayland tests require %s; install sway and dbus.\n' "$command" >&2
    exit 1
  fi
done
if ((EUID == 0)); then
  printf 'Run Wayland tests as a non-root user.\n' >&2
  exit 1
fi

runtime=$(mktemp -d "${TMPDIR:-/tmp}/azerlay-wayland-XXXXXXXX")
compositor=
trap '
  if [[ -n "$compositor" ]]; then
    kill "$compositor" 2>/dev/null || true
    wait "$compositor" 2>/dev/null || true
  fi
  rm -rf -- "$runtime"
' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
chmod 700 "$runtime"
printf 'output * mode 1280x720\nseat * fallback true\n' >"$runtime/config"
export XDG_RUNTIME_DIR="$runtime"
WLR_BACKENDS=headless WLR_RENDERER=pixman WLR_HEADLESS_OUTPUTS=2 \
  sway --config "$runtime/config" >"$runtime/sway.log" 2>&1 &
compositor=$!
for ((attempt = 0; attempt < 200; attempt++)); do
  for socket in "$runtime"/wayland-*; do
    if [[ -S "$socket" ]]; then
      export WAYLAND_DISPLAY="$socket"
      export AZERLAY_TEST_WAYLAND_DISPLAY="$socket"
      export GDK_BACKEND=wayland GSK_RENDERER=cairo
      dbus-run-session -- "$@"
      exit $?
    fi
  done
  if ! kill -0 "$compositor" 2>/dev/null; then
    printf 'Headless Wayland compositor failed to start.\n' >&2
    cat "$runtime/sway.log" >&2
    exit 1
  fi
  sleep 0.05
done
printf 'Headless Wayland compositor socket timed out.\n' >&2
cat "$runtime/sway.log" >&2
exit 1
