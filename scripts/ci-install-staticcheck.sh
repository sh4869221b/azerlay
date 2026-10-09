#!/usr/bin/env bash
# Isolated, checksum-verified upstream Go 1.27.2 compatibility build.
set -euo pipefail
repository=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
source "$repository/.github/ci-tools.env"
test "$(go env GOVERSION)" = go1.27.2
test "$(go env GOTOOLCHAIN)" = local
cd "$repository/scripts/ci-staticcheck"
# The upstream PR lives in its author's fork until it is merged/released.
# Pin its exact source and importer; never silently resolve a branch/latest.
test "$(go list -mod=readonly -m -f '{{.Replace.Path}}@{{.Replace.Version}}' honnef.co/go/tools)" \
  = "github.com/stefanb/go-tools@$STATICCHECK_VERSION"
test "$(go list -mod=readonly -m -f '{{.Version}}' golang.org/x/tools)" \
  = "$STATICCHECK_X_TOOLS_VERSION"
before=$(sha256sum go.mod go.sum)
go install -mod=readonly honnef.co/go/tools/cmd/staticcheck
go mod verify
test "$(sha256sum go.mod go.sum)" = "$before"
