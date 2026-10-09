#!/usr/bin/env bash
set -euo pipefail

if (($# != 2)) || [[ -z "$2" ]]; then
  printf 'Usage: %s VERSION OUTPUT_FILE\n' "$0" >&2
  exit 2
fi

version=$1
if [[ ! "$version" =~ ^[0-9]+(\.[0-9]+)+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
  printf 'VERSION must be numeric and dotted, with an optional prerelease suffix.\n' >&2
  exit 2
fi

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
changelog="$repo_root/CHANGELOG.md"
if [[ ! -f "$changelog" ]]; then
  printf 'Required reviewed changelog is missing: %s\n' "$changelog" >&2
  exit 1
fi

output_file=$2
mkdir -p -- "$(dirname -- "$output_file")"
changes=$(mktemp)
trap 'rm -f -- "$changes"' EXIT
if ! awk '
  /^## \[Unreleased\]$/ { in_unreleased = 1; found = 1; next }
  in_unreleased && /^## / { exit }
  in_unreleased && !has_content && /^[[:space:]]*$/ { next }
  in_unreleased { has_content = 1; print }
  END { if (!found) exit 1 }
' "$changelog" > "$changes" || ! grep -q '[^[:space:]]' "$changes"; then
  printf 'CHANGELOG.md must contain a non-empty ## [Unreleased] section.\n' >&2
  exit 1
fi

{
  printf '# Azerlay %s release candidate\n\n' "$version"
  printf 'This is a non-publishing candidate. It has not been signed or published.\n\n'
  printf '## Changes\n\n'
  cat -- "$changes"
  printf '\n## Compatibility and troubleshooting\n\n'
  printf 'For version-matched details, see `docs/compatibility.md` and `docs/troubleshooting.md` in the source archive.\n\n'
  printf '## Publication status\n\n'
  printf 'This candidate does not establish public-distribution readiness. Complete the artifact-specific component and license review, applicable source and relinking checks, provenance and privacy review, and final archive and clean-install checks in `docs/decisions/public-release-safety.md`. AC-016 remains blocked until a public release and clean-environment installation are evidenced. Signing and publication require a separate later action.\n'
} > "$output_file"
