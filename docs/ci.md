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
| `native-build` | Verify default CGo, compile the native binary in Arch, run version, and inspect shared libraries | 15 minutes |
| `vulnerability` | Reachable Go vulnerability scan | 20 minutes |
| `licenses` | Third-party Go dependency license CSV | 10 minutes |
| `generated-files` | Regenerate both LZMA fixtures and reject differences | 15 minutes |

The `test` job rejects any output from `gofmt -l .`, then runs:

```sh
go vet ./...
go install honnef.co/go/tools/cmd/staticcheck@2026.2.1
staticcheck ./...
scripts/test-wayland.sh go test -race -shuffle=on -count=1 ./...
go build ./cmd/azerlay
```

Ordinary tests run once, retaining the race detector, shuffled ordering and
uncached execution, including the production GTK/CGo packages. Native child
tests initialize GTK on their process main thread.
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

`test`, `native-build`, `vulnerability`, and `licenses` use
`archlinux:base`. Before checkout and Go setup, they run `pacman -Syu` and install
`ca-certificates git curl tar gzip zstd base-devel pkgconf gtk4 gtk4-layer-shell
gobject-introspection`. Ubuntu distribution support is excluded from v1 and
tracked in [Issue #98](https://github.com/sh4869221b/azerlay/issues/98).
The Ubuntu-hosted runner and the pure `fuzz`/`generated-files` jobs do not qualify
Ubuntu as a product target. The distribution build runs:

```sh
git config --global --add safe.directory "$GITHUB_WORKSPACE"
go version
go env GOOS GOARCH CGO_ENABLED CC
test "$(go env CGO_ENABLED)" = 1
pkg-config --modversion gtk4 gtk4-layer-shell-0
CGO_ENABLED=1 go build -o /tmp/azerlay ./cmd/azerlay
/tmp/azerlay version
readelf -d /tmp/azerlay
```

The checkout action's safe-directory setting lives in a temporary Git config
that is removed after checkout. Container users can differ from the mounted
workspace's owner, so the build step trusts only that workspace in its own
global Git config. This preserves normal Go VCS metadata during compilation.

### Module and native build caches

The four Arch jobs use `.github/actions/native-go-cache`. Automatic setup-go
caching is disabled for those jobs to avoid saving the same directories twice.
The two pure Ubuntu jobs retain their existing setup-go caches unchanged.

- **Application modules:** all four jobs restore `/tmp/azerlay-go-mod`, keyed
  by OS, architecture, resolved Go toolchain, `go.mod`, `go.sum`, and the module
  preparation script. The key excludes the job, native packages and analyzer
  versions. All four install `zstd` before restore, so cache paths and compression
  agree. Only the existing `native-build` job saves this snapshot, after a
  successful `go mod download` and `go mod verify`. No job waits for it;
  concurrent cold consumers may miss and download normally.
- **Analyzer modules:** Staticcheck, govulncheck and go-licenses install with
  `GOMODCACHE=/tmp/azerlay-tool-go-mod`, separately cached by owning job, OS,
  architecture, resolved Go version and `.github/ci-tools.env`. That manifest
  drives the unchanged pinned tool versions. This prevents an incomplete
  first-writer snapshot and avoids transferring all three tools' dependency
  graphs to every consumer. Native package updates do not invalidate tool
  downloads. Each tool installs normally on a cache miss; no priming build or
  cross-job dependency is introduced.
- **Build results:** each job restores and saves only `/tmp/azerlay-go-build`.
  Its key includes the job ID and complete sorted installed Arch package list,
  plus OS, architecture, resolved Go toolchain, application dependency files,
  and CI-tool versions. No restore prefix crosses the native compatibility
  boundary. Native updates still produce a cold build cache.

App download, checksum verification and an unchanged `go.mod`/`go.sum` check run
on every cache hit or miss. Application and analyzer cache paths do not overlap.
Each analyzer retains its own native-keyed build results while sharing the
job's GOCACHE with the application's other checks.

Cache hits never skip formatting, vet, analyzers, tests, compilation, or binary
execution. No credentials, HOME directory, binaries, or user profiles are included
in the module snapshots. GitHub's normal branch/PR cache scope remains in force;
there is no privileged workflow, ref override, or PR-to-main cache promotion.
The test job's existing ownership transition covers both normalized cache roots.

Changing to separate caches initially invalidates the old combined snapshots.
This migration cost is distinct from steady-state results. It does not implement
Issue #114's rejected source-generation experiment: build snapshots remain keyed
by dependency/native inputs, without a source or run-ID generation.

#### Measurement and adoption

This is a measured candidate for Issue #116, not a demonstrated wall-time win.
`scripts/ci-go-modules.sh` reports application-download/verification milliseconds
and raw module-cache bytes. Analyzer installation steps report their own module
cache bytes. The Actions cache
logs provide compressed bytes, hit/key details, and restore/save step durations.
Compare these overheads and job durations against the preceding combined-cache
workflow; do not add parallel-job seconds to claim a workflow elapsed reduction.

The target cases are a cold first run, identical warm run, source-only change,
Go-dependency change, and native-only change. A native-only change must preserve
the app/tool module keys while invalidating the build key. A Go-dependency change
invalidates the app snapshot; a tool-version change invalidates tool snapshots;
a source-only change does neither. Key
simulation is distinct from actual hosted restore and timing evidence. Keep
adoption contingent on warm/native-update measurements: extra cache round trips
may outweigh avoided downloads. If so, narrow sharing or leave this candidate
unadopted. A preliminary local combined app/tool snapshot was about 244 MB raw
and 86 MB compressed, so the candidate separates analyzer modules instead of
making every consumer transfer that union. The issue's 0–10-second estimate
is a hypothesis, not a measured result or a compile-time saving.

The test job also installs Sway and D-Bus, removes Sway's file capability inside
the container, and runs tests as a dedicated non-root user with writable Go
caches. `scripts/test-wayland.sh` starts a private two-output headless Sway with
the pixman renderer, waits for its socket, and provides an absolute test display
path. CLI fixtures retain separate runtime/config/data directories. The launcher
stops its compositor and removes its private runtime directory on exit. This
is protocol coverage, not Sway support or physical monitor/device qualification.

`native-build` replaces `container-build` and `cgo-smoke`: both previously
compiled and ran the same checkout with the same Arch packages and Go version.
It asserts that the default CGo setting is `1` before the explicit CGo build,
so a changed default fails rather than silently losing a distinct build condition.
It retains the Go version, pkg-config versions, version execution and ELF dynamic
section inspection. All commands fail the job on a nonzero exit; no cache-hit
condition skips a build or runs a previously saved executable. The final `test`
job build and the CLI tests' current-checkout build remain independent and unchanged.

The job ID remains part of the native cache input, so `native-build` has one
isolated build cache rather than the two equivalent caches it replaced. Old
combined cache entries expire under GitHub's normal retention policy.

### Required-check migration

Before merging this job rename, repository administrators should inspect branch
protection and rulesets. If either `container-build` or `cgo-smoke` is required,
replace both with the actual successful `native-build` check from this PR, while
keeping all other required checks. Coordinate the rename with the merge so that
neither stale check names nor an unprotected gap is left behind. This change does
not edit repository protection, create always-successful compatibility jobs, or
skip verification. A successful PR run does not establish post-merge `main`
success; verify the first `main` run separately after an authorized merge.

The compile check has no skip guard. Hyprland placement, fractional scaling, and
output lifecycle require separate isolated compositor QA; CI does not establish
physical hotplug, Niri behavior, or game input/rendering acceptance.

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
