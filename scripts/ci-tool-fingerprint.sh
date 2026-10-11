#!/usr/bin/env bash
# Select the analyzer graph by purpose, independently of workflow/job names.
set -euo pipefail
source .github/ci-tools.env
case "${1:?Select staticcheck, govulncheck or go-licenses}" in
  staticcheck)
    settings=(STATICCHECK_VERSION STATICCHECK_X_TOOLS_VERSION)
    sha256sum scripts/ci-staticcheck/go.mod scripts/ci-staticcheck/go.sum
    ;;
  govulncheck) settings=(GOVULNCHECK_VERSION);;
  go-licenses) settings=(GO_LICENSES_VERSION);;
  *) printf 'Unsupported Go analyzer graph\n' >&2; exit 2;;
esac
printf 'analyzer=%s\n' "$1"
for setting in "${settings[@]}"; do
  printf '%s=%s\n' "$setting" "${!setting:?Missing Go analyzer pin}"
done
