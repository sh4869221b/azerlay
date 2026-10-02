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
| `container-build` | Build and run the binary in Arch | 15 minutes |
| `cgo-smoke` | Compile the native binary, run version, and inspect shared libraries | GitHub default (360 minutes) |
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

`test`, `container-build`, `cgo-smoke`, `vulnerability`, and `licenses` use
`archlinux:base`. Before checkout and Go setup, they run `pacman -Syu` and install
`ca-certificates git curl tar gzip base-devel pkgconf gtk4 gtk4-layer-shell
gobject-introspection`. Ubuntu distribution support is excluded from v1 and
tracked in [Issue #98](https://github.com/sh4869221b/azerlay/issues/98).
The Ubuntu-hosted runner and the pure `fuzz`/`generated-files` jobs do not qualify
Ubuntu as a product target. The distribution build runs:

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

`container-build`, `cgo-smoke`, `vulnerability`, and `licenses` retain their
isolated setup-go caches keyed by dependencies, job ID, and the sorted full
Arch package inventory. The `test` job uses explicit restore/save instead:

- `setup-go` caching is disabled so it does not also own the same paths.
- The build-cache compatibility fingerprint includes the **entire** sorted
  `pacman -Q` inventory, distribution/architecture, actual Go version/compiler
  identity, Go target/toolchain/build/CGO settings, and GTK pkg-config inputs.
  Unknown native changes fail cold; this is deliberately not a five-package
  ABI approximation. Dependency hashes also belong to the restore boundary.
- The exact key adds a hash of the tracked source tree (including embedded
  assets and fixtures). Repeated runs of the same tree do not create new
  generations. A changed source tree can restore the newest compatible
  snapshot and save its updated compilation results. The sole fallback prefix
  contains the full compatibility and dependency hashes; there is no broader
  fallback across native or toolchain environments.
- `/tmp/azerlay-go-build` contains both ordinary and race entries. Only the
  `test` job writes this namespace, after vet, Staticcheck, the complete
  non-root race/Wayland tests, and ordinary build succeed. A short job cannot
  save an incomplete snapshot first. Hits never skip a quality gate.
- Saves are limited to pushes to `main` and same-repository pull requests.
  Fork PRs restore only. GitHub scopes PR snapshots to their merge ref; they
  can speed up the same PR's reruns but are never promoted to `main`. New PRs
  can restore compatible seeds generated on `main` after this change merges.
  A first main run can therefore still be cold even after a warm PR run.
- `/tmp/azerlay-go-mod` keeps a separate, exact, test-job-local dependency
  cache. Cross-job module sharing is not implemented here and module download
  savings must not be attributed to build-cache freshness.
- Before non-root execution the restored cache and workspace are recursively
  assigned to the test user; readable/writable access is checked. Root still
  runs vet and the final build. The CLI TestMain keeps its 30-second ordinary
  build and inherits the same GOCACHE warmed by vet, including on a cold run.
- Only the two Go cache directories are archived, never HOME, runtime/config
  directories, credentials, or user profiles. Fixture privacy rules still
  apply. Cache absence does not suppress compilation or test failures.

The source fingerprint favors correctness and simplicity over minimizing
snapshots: even a documentation-only tracked change can make a generation.
GitHub's cache quota/eviction still bounds storage; a run ID or attempt number
is never included. Transfer cost and eviction pressure must be measured when
judging whether saving fresh generations is worthwhile.

### Cache measurements and limitations

`Prepare Go cache keys` prints compatibility, dependency, source and exact
keys. Restore actions report the matched key, compressed bytes and transfer
time; `Record cache restore` / `Record normal and race GTK cache state` print uncompressed
bytes. Save steps and the Actions job/step timestamps supply save duration,
job wall time, and summed runner-seconds across all seven jobs. The GTK state
step lists Go's ordinary/race stale status and reason without compiling. The temporary
compatibility manifest contains only the explicit public tool/native inputs,
not arbitrary environment variables, and is not uploaded.

`bash scripts/test-ci-go-cache-key.sh` checks repeatability, source-only
fallback, and cold boundaries for changed dependencies, toolchain/settings,
and a transitive native package. These synthetic contract tests are not
measurements of real Go or native upgrades.

Measure cold, same-commit warm, source-only, Go dependency/toolchain change,
and native-package change separately, with repeated successful runs. Keep
main-versus-PR cache scope and full native fingerprints in each record. Check
ordinary and race gotk4 compiler invocations with `-x` when needed; repeated
cached compiler warnings alone are not proof of recompilation. Compare
end-to-end wall time and runner-seconds, including transfer/save overhead,
not just the test command's time (which includes compilation).

The pre-change cold example is **not** the usual baseline: main runs
[36300187973](https://github.com/sh4869221b/azerlay/actions/runs/36300187973)
and [36297704607](https://github.com/sh4869221b/azerlay/actions/runs/36297704607)
restored the same approximately 191 MB cache and completed in about three
minutes. PR #113 then added grim and two font packages, legitimately changing
the full native boundary. Its PR run and subsequent main run
[36309312391](https://github.com/sh4869221b/azerlay/actions/runs/36309312391)
missed the new key in their separate scopes. Source-generation freshness is
the intended incremental improvement; this change does not avoid required
native invalidations or establish a 20–26 minute average saving.

The test job also installs Sway and D-Bus, removes Sway's file capability inside
the container, and runs tests as a dedicated non-root user with writable Go
caches. `scripts/test-wayland.sh` starts a private two-output headless Sway with
the pixman renderer, waits for its socket, and provides an absolute test display
path. CLI fixtures retain separate runtime/config/data directories. The launcher
stops its compositor and removes its private runtime directory on exit. This
is protocol coverage, not Sway support or physical monitor/device qualification.

`cgo-smoke` performs actual native compilation and inspection:

```sh
pkg-config --modversion gtk4 gtk4-layer-shell-0
CGO_ENABLED=1 go build -o /tmp/azerlay ./cmd/azerlay
/tmp/azerlay version
readelf -d /tmp/azerlay
```

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
