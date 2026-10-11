#!/usr/bin/env bash
set -euo pipefail

# Keep test dependencies out of the runner's system Python and package caches.
dependencies=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/azerlay-python-tests.XXXXXX")
trap 'rm -rf "$dependencies"' EXIT
python3 -m pip install --no-cache-dir --no-deps --require-hashes --only-binary=:all: \
  --target "$dependencies" -r scripts/tests/requirements.txt
PYTHONPATH="$dependencies${PYTHONPATH:+:$PYTHONPATH}" \
  python3 -m unittest discover -s scripts/tests -v
