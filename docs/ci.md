# CI and synthetic fixture policy

[CI](../.github/workflows/ci.yml) runs for pull requests and pushes to `main`.
It has read-only repository permissions and cancels superseded runs for the
same pull request. All Go jobs use Go 1.27.1. CI-only tools are installed with
explicit versions without changing the application's `go.mod` or `go.sum`.

## Quality gates

| Job | Checks | Job timeout |
| --- | --- | --- |
| `test` | Formatting, vet, Staticcheck, race tests, binary build | GitHub default (360 minutes) |
| `fuzz` | Seven existing fuzz targets, each with a 10-second fuzz allocation | 10 minutes |
| `container-build` | Build and run the binary in Ubuntu 24.04 and Arch | 15 minutes per image |
| `cgo-smoke` | Explain the explicit GTK/CGo compile skip | GitHub default (360 minutes) |
| `vulnerability` | Reachable Go vulnerability scan | 10 minutes |
| `licenses` | Third-party Go dependency license CSV | 10 minutes |
| `generated-files` | Regenerate both LZMA fixtures and reject differences | 15 minutes |

The `test` job rejects any output from `gofmt -l .`, then runs:

```sh
go vet ./...
go install honnef.co/go/tools/cmd/staticcheck@2026.2.1
staticcheck ./...
go test -race -shuffle=on -count=1 ./...
go build ./cmd/azerlay
```

Ordinary tests run once, retaining the race detector, shuffled ordering and
uncached execution. The current packages do not depend on a GTK/CGo bridge.
Analyzer installation or execution failures fail CI; they are not suppressed.

## Fuzz budget

The `fuzz` job runs these commands sequentially and stops on the first failure:

```sh
go test ./internal/profiledecode -run='^$' -fuzz='^FuzzNormalizeOuterText$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profiledecode -run='^$' -fuzz='^FuzzDecodeBase64$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profiledecode -run='^$' -fuzz='^FuzzDecodeLZMAEnvelope$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profiledecode -run='^$' -fuzz='^FuzzDecodeMsgpackString$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profiledecode -run='^$' -fuzz='^FuzzParseRoot$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profileraw -run='^$' -fuzz='^FuzzParse$' -fuzztime=10s -parallel=1 -timeout=2m
go test ./internal/profileadapter -run='^$' -fuzz='^FuzzNormalize$' -fuzztime=10s -parallel=1 -timeout=2m
```

The 70-second total is the fuzzing allocation, not the total job duration:
setup, compilation and baseline corpus execution also take time. Each command
has a two-minute timeout and one fuzz worker. Regular unit tests are excluded
from this job by `-run='^$'`. Longer fuzz campaigns are outside PR CI.

## Distribution builds and GTK coverage

The container matrix uses `ubuntu:24.04` and `archlinux:base`. Ubuntu installs
`ca-certificates git curl tar gzip build-essential` with apt; Arch performs
`pacman -Syu` and installs `ca-certificates git curl tar gzip base-devel`.
These dependencies are installed before checkout and Go setup. Each image runs:

```sh
git config --global --add safe.directory "$GITHUB_WORKSPACE"
go version
go build -o /tmp/azerlay ./cmd/azerlay
/tmp/azerlay version
```

The checkout action's safe-directory setting lives in a temporary Git config
that is removed after checkout. Container users can differ from the mounted
workspace's owner, so the build step trusts only that workspace in its own
global Git config. This preserves normal Go VCS metadata during compilation.

Go caching is disabled in these jobs to avoid sharing native build caches
across distributions. Matrix fail-fast is disabled so both results remain
visible; package installation, compilation and binary execution failures block
their respective job. Arch is a rolling image, so its package set can change
between runs. These checks demonstrate builds and CLI startup, not a working
Wayland session or hardware integration.

The isolated `cgo-smoke` job reports **GTK/CGo compilation skipped** in its job
summary and keeps its compile step visibly disabled. There is no production
GTK/gotk4/gtk4-layer-shell bridge yet, which is the documented skip exception
allowed by [Issue #36](https://github.com/sh4869221b/azerlay/issues/36).
The informational job's success is not native compile coverage. Introducing
that bridge must replace the disabled step with native dependency installation
and actual bridge compilation. No placeholder bridge is built for this check.

## Vulnerabilities and dependency licenses

The vulnerability job runs:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
govulncheck ./...
```

Reachable vulnerability findings and nonzero tool or network failures block
CI. An incompatible scanner is an unresolved gate; it is not grounds to skip
the check or downgrade the project's Go version.

The license job runs:

```sh
go install github.com/google/go-licenses/v2@v2.0.1
go-licenses report ./... --include_tests --ignore=github.com/sh4869221b/azerlay > "$RUNNER_TEMP/dependency-licenses.csv"
```

After successful report generation, `actions/upload-artifact@v4` publishes
`dependency-licenses` for seven days and fails if the file is absent. The
report includes test dependencies and excludes only the project's undecided
license, not its third-party dependencies. Stderr remains visible. Upstream
may report unknown licenses or license/URL discovery warnings while exiting
successfully; these warnings are not converted into failures by custom rules.
Every actual nonzero tool result fails the job.

The CSV inventories Go dependencies; it is not an allowlist or a compatibility
decision, and does not cover dynamically loaded or native libraries. The
project license remains undecided, with dependency-license, NOTICE and SBOM
decisions tracked separately in Issue #18.

## Synthetic fixture policy

Raw user profiles, private exports, macro contents, complete input-event
streams, full user labels and private Azeron database contents must never be
committed or included in test logs, CI artifacts or fuzz corpus. Public
fixtures must be minimal synthetic structures. Synthetic coverage does not
establish support for additional Azeron Software releases.

This policy covers every current fixture directory:

| Directory | Maintenance |
| --- | --- |
| `cmd/azerlay/testdata/cli` | Manually maintained synthetic CLI input and expected-output contracts |
| `internal/diagnostics/testdata` | Manually maintained synthetic diagnostic expected outputs |
| `internal/profileadapter/testdata` | Manually maintained synthetic adapter inputs/expected outputs and external-consumer compilation fixture |
| `internal/profileraw/testdata` | Manually maintained synthetic raw-model inputs and expectations |
| `internal/profiledecode/testdata` | Two generated zero-only LZMA boundary streams and their generator |

JSON/text inputs and expected files are reviewed against existing tests and
contracts. They are not regenerated output snapshots to refresh blindly when
a test fails. A fuzz failure may become a public regression seed only after
private contents have been replaced with minimal synthetic structure that
still reproduces the defect. Do not upload private failure inputs or corpus.

The decoder's [fixture notes](../internal/profiledecode/testdata/README.md)
describe its zero-only recipe. From the repository root, `generated-files`
runs actual regeneration followed by:

```sh
go run ./internal/profiledecode/testdata/generate
git diff --exit-code -- internal/profiledecode/testdata/zeros-64mib.lzma internal/profiledecode/testdata/zeros-64mib-plus-one.lzma
```

Only the existing compressed files are written; ordinary tests read them
without regeneration and no decompressed files are stored. Any byte change
fails the diff check and needs an explanation, not an automatic fixture update.
