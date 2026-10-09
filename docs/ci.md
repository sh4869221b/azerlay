# CI and synthetic fixture policy

[CI](../.github/workflows/ci.yml) runs for pull requests and pushes to `main`.
It has read-only repository permissions and cancels superseded runs for the
same pull request. All Go jobs use Go 1.27.2. CI-only tools are installed with
explicit versions without changing the application's `go.mod` or `go.sum`.
The owner selected pinned Nix for normal CI on 2026-10-07. Latest Arch runs
independently each day and through manual pre-release verification; see the
[adopted trigger and release-verification policy](ci-adoption.md).

## Quality gates

| Job | Checks | Job timeout |
| --- | --- | --- |
| `test` | Formatting, all-package vet/Staticcheck, native race tests, binary build | 60 minutes |
| `fuzz` | Seven existing fuzz targets, each with a 10-second fuzz allocation | 10 minutes |
| `native-build` | Verify default CGo, compile the native binary in pinned Nix, run version, and inspect shared libraries | 30 minutes |
| `vulnerability` | Reachable Go vulnerability scan | 30 minutes |
| `licenses` | Third-party Go dependency license CSV | 20 minutes |
| `generated-files` | CI-tool tests, core CGO=0 and race tests, regenerate both LZMA fixtures and reject differences | 15 minutes |

The `test` job rejects any output from `gofmt -l .`, then runs:

```sh
go vet ./...
bash scripts/ci-install-staticcheck.sh
staticcheck ./...
mapfile -t native < scripts/ci-native-packages.txt
scripts/test-wayland.sh go test -race -json -shuffle=on -count=1 "${native[@]}"
go build ./cmd/azerlay
```

Staticcheck currently uses the exact Go 1.27.2 compatibility source from
[upstream PR #1834](https://github.com/dominikh/go-tools/pull/1834), commit
`a0a7f6a7b6af6eaa66e22ff3a5f78725e27decf2`. This is an unmerged development
commit, not a tagged release. Staticcheck 2026.2.1's old importer cannot decode
Go 1.27.2 export format V5. The upstream fix updates `x/tools` to v0.51.0 and
replaces two removed private-symbol links with their equivalent implementations.
The [Go tools V5 decoder change](https://github.com/golang/tools/commit/89ed5c340cb6d4a9437f801cfc718ac5980c938d)
explains the format compatibility requirement.

`scripts/ci-staticcheck` is a separate CI-only module. Its replacement pin and
checksums select that exact compatibility source without changing application
`go.mod` or `go.sum`. Installation requires Go 1.27.2, checks the exact source and
importer versions, uses `-mod=readonly`, and verifies module contents. Both Arch
and Nix use the same installer; tool cache keys include the tool module's lock
files. Replace this temporary source pin with a compatible tagged upstream
release when one is available. All default Staticcheck findings still fail CI.

Every ordinary root-module package has a required race/shuffle/uncached test run:
core packages in `generated-files`, native packages in `test`. Core packages
also receive a separate CGO=0 contract run. Production GTK/CGo remains native. Native child
tests initialize GTK on their process main thread.
Analyzer installation or execution failures fail CI; they are not suppressed.

## Core/native test partition

The [CLI/core boundary](cli-core-boundary.md) assigns every ordinary package to
an explicit core or native manifest. Before formatting/vet, the native `test`
job compares their exact disjoint union against `go list ./...` and checks the
core dependency closure including tests for native/GTK imports. Its existing
whole-repository formatting, vet and Staticcheck remain. Its race command uses
the five-package native manifest with the same non-root, two-output Sway launcher
and current-checkout CLI build; the binary build also remains.
Formatting failures print `gofmt -d .` for review from hosted logs.

The existing `generated-files` job adds core-only dependency checks and tests
with `CGO_ENABLED=0`, followed by a separate `CGO_ENABLED=1` race run. Both use
shuffle and count=1 without installing or starting GTK/Sway. The race step uses
a new temporary GOCACHE, including for its dependency check, so setup-go's
Ubuntu cache does not start reusing/saving CGo outputs without the native ABI
fingerprint. The original fixture regeneration and diff steps remain afterward.

After successful initial boundary validation, the native race command no longer
repeats the 14 core packages. Their required race run stays in `generated-files`,
which also retains actual fixture regeneration/diff. The manifest checker rejects
omissions and overlap; failure in either job still fails the workflow. Existing job IDs and
the core/native package manifests are unchanged. The fresh
core race cache and extra CGO=0 mode have a cost that remains part of measurement.
This partition is a measured candidate, not a demonstrated p50/p95 budget result.

### Race diagnostics baseline

For [Issue #138](https://github.com/sh4869221b/azerlay/issues/138), both required
race commands now use `-json`. The existing package manifests, `-race`,
`-shuffle=on` and `-count=1` are unchanged. The native command runs as the ordinary non-root hosted user inside the pinned
Nix environment and the unmodified two-output `test-wayland.sh` launcher.
Rolling Arch uses `runuser -u azerlay-test`; neither path runs Go tests as root. The
separate core CGO=0 contract run, fresh core race cache, all six jobs, native
package installs, cache inputs, fuzz budgets and fixture regeneration remain.
This is baseline instrumentation before any test-speed changes.

Each race command is wrapped by the standard-library-only
`scripts/ci_time_command.py`, then piped to `tee` with explicit
`set -euo pipefail`. The wrapper preserves the command's exit status and signal
termination and forwards SIGINT/SIGTERM. Neither `tee` nor artifact upload can
turn a failed command into a successful test step. A timing-report write error
also fails the step, while retaining an already failing command's exit status.
No `continue-on-error` or replacement green check is used.

The corresponding upload step runs after a successful or failed race step.
Artifacts are retained for seven days and have distinct names on full or
failed-jobs-only reruns:

| Artifact | Explicit files |
| --- | --- |
| `nix-native-race-json-attempt-${{ github.run_attempt }}` | `nix-native-race-tests.jsonl`, `nix-native-race-timing.json` |
| `core-race-json-attempt-${{ github.run_attempt }}` | `core-race-tests.jsonl`, `core-race-timing.json` |

Only those files are uploaded, not directories, caches, compositor logs,
profiles or fuzz inputs. Test JSON retains all events, including failures,
skips, shuffle seeds, `run`/`pause`/`cont` timestamps and test output. These are
the repository's synthetic tests only: the wrapper passes output through and
does **not** redact private inputs. The synthetic-fixture policy still applies
to all test output and artifacts. Do not add private fixture or environment
dumps. The wrapper itself records only numeric timing/status fields, never
arguments, environment variables, HOME or paths; its timing summary goes to
stderr separately from test JSON on stdout.

The timing JSON records monotonic command elapsed time, child user/system CPU
seconds, `max_child_rss_kib`, the command `returncode` (negative for a signal),
and an optional received `wrapper_signal`. CPU accounting includes resources
propagated from descendants that the command waits for. On Linux,
`max_child_rss_kib` is in KiB and is the largest child high-water mark, including
propagated waited-for descendant usage. It is **not** the simultaneous peak
memory of the process tree, and does not prove safe memory headroom for adding
parallelism. Killed or orphaned descendants need not yield complete accounting;
SIGKILL of the wrapper, runner loss or job cancellation can leave partial or
missing artifacts. A failed/partial run is not a successful timing sample.

Use `python3 scripts/ci_measure.py tests PATH-TO-TESTS.jsonl` for terminal elapsed
records. Rank rows with a `test` name separately from package rows (`test: null`).
Retain the raw JSON for overlap and pause/continue analysis. Never add parent
and child test times or overlapping package times together. Test elapsed is
not command elapsed: command timing includes compilation/cache work, and the
native command also includes user/Wayland setup, the current-checkout CLI build
inside TestMain, and compositor teardown. It is not a per-child CPU profile.

Diagnostic overhead is unmeasured until hosted validation: JSON formatting and
additional output, the Python process, `tee` file writes, and artifact upload all
cost time. Command elapsed excludes wrapper startup/reporting and artifact
upload; job/workflow elapsed and runner-seconds include those costs. Keep the
uninstrumented partition control, instrumented baseline, any later speed
candidate, and optional profiling runs in distinct measurement cohorts. Compare
like cache/runner conditions and the exact same gate set, disclosing sample and
failure counts. One run establishes neither p95 nor a speedup, and profiling
runs cannot establish ordinary test performance. No test parallelism or
profiling is introduced here.

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

`test`, `native-build`, `vulnerability`, and `licenses` use the pinned
[Nix native environment](ci-nix.md), including the locked Go 1.27.2 and native
closure. The four roles retain current-checkout build/scan/test commands.
The separate [Arch full validation](../.github/workflows/arch-ci.yml) runs all
six quality gates against rolling `archlinux:base` / `pacman -Syu`, once daily
and through the pre-release verification entrypoint. It has no soft-failure path.

Each of the four Arch native jobs retries only its pre-checkout package
installation, at most three times, waiting 10 and then 20 seconds after failed
attempts. Every attempt uses the same full `pacman -Syu --noconfirm --needed`
transaction and package list, with normal package signature verification.
Pacman's nonzero status does not reliably distinguish transient mirror failures
from permanent errors, so any installation failure gets this finite retry budget;
the third failure exits with its status and blocks the job. The job timeout still
bounds total execution, including downloads. The shell stays inline because git,
Python and repository scripts are not available before this bootstrap succeeds.
Tests execute all four actual workflow bootstrap bodies with stubbed commands,
covering immediate success, recovery, exhaustion and the subsequent Sway setup.
Builds, scans and other gates retain their existing failure behavior.

All hosts and GUI-free `fuzz`/`generated-files` jobs use standard `ubuntu-24.04`.
Hosted images are maintained by GitHub, not immutable. The fresh core race cache
remains outside setup-go's cache. Nix as a CI substrate does not qualify a new
product distribution target; Ubuntu support remains tracked in Issue #98.
The distribution build runs:

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

Normal native jobs use `.github/actions/nix-go-cache`; no second Go SDK is
installed by setup-go. Arch native jobs use `.github/actions/native-go-cache`
with setup-go's implicit caching disabled. The two GUI-free jobs retain their
setup-go caches; core CGo/race still uses a fresh step-local GOCACHE.

- **Application modules:** all four jobs restore
  `/tmp/azerlay-go-mod/cache/download`, keyed
  by OS, architecture, resolved Go toolchain, `go.mod`, `go.sum`, and the module
  preparation script. The key excludes the job, native packages and analyzer
  versions. All four install `zstd` before restore, so cache paths and compression
  agree. Only the existing `native-build` job saves this snapshot, after a
  successful `go mod download` and `go mod verify`. No job waits for it;
  concurrent cold consumers may miss and download normally.
- **Analyzer modules:** Staticcheck, govulncheck and go-licenses install with
  `GOMODCACHE=/tmp/azerlay-tool-go-mod`, with its `cache/download` directory
  separately cached by owning job, OS,
  architecture, resolved Go version and `.github/ci-tools.env`. That manifest
  drives the unchanged pinned tool versions. This prevents an incomplete
  first-writer snapshot and avoids transferring all three tools' dependency
  graphs to every consumer. Native package updates do not invalidate tool
  downloads. Each tool installs normally on a cache miss; no priming build or
  cross-job dependency is introduced.
- **Build results:** each job restores and saves only `/tmp/azerlay-go-build`.
  Its key includes the job ID, complete locked Nix closure or full installed Arch
  inventory, actual workflow/install recipe, compiler/linker/pkg-config metadata,
  plus OS, architecture, resolved Go toolchain, application dependency files,
  and CI-tool versions. No restore prefix crosses the native compatibility
  boundary. Native updates still produce a cold build cache.

The module snapshots contain Go's download archives and metadata (`.info`,
`.mod`, `.zip`, `.ziphash`, and cached sumdb data), not duplicate extracted source
trees. Normal `go mod download` and pinned `go install` commands reconstruct
sources and retain their existing checksum checks. This does not promise offline
CI: Go may still query release/deprecation metadata, and vulnerability scanning
still contacts its database. No checksum service is disabled.

App download, checksum verification and an unchanged `go.mod`/`go.sum` check run
on every cache hit or miss. Application and analyzer cache paths do not overlap.
Each analyzer retains its own native-keyed build results while sharing the
job's GOCACHE with the application's other checks.

Cache hits never skip formatting, vet, analyzers, tests, compilation, or binary
execution. No credentials, HOME directory, binaries, or user profiles are included
in the module snapshots. GitHub's normal branch/PR cache scope remains in force;
there is no privileged workflow, ref override, or PR-to-main cache promotion.
Arch test jobs transfer both normalized cache roots to their non-root test user;
Nix jobs already run as the non-root hosted user.

Changing to separate caches initially invalidates the old combined snapshots.
This migration cost is distinct from steady-state results. It does not implement
Issue #114's rejected source-generation experiment: build snapshots remain keyed
by dependency/native inputs, without a source or run-ID generation.

#### Measurement and adoption

This is a measured candidate for Issue #116, not a demonstrated wall-time win.
`scripts/ci-go-modules.sh` reports application-download/verification milliseconds
and raw module/download-cache bytes. Analyzer installation steps report their own
download-cache bytes. The Actions cache
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
making every consumer transfer that union. An initial full-directory shared
snapshot also increased aggregate restore bytes, so the final candidate keeps
only download archives and reconstructs sources locally. The issue's
0–10-second estimate
is a hypothesis, not a measured result or a compile-time saving.

Both native environments include Sway and D-Bus with writable Go caches.
Arch removes Sway capabilities and changes to a dedicated non-root test user;
Nix uses its capability-free pinned compositor as the non-root hosted user. `scripts/test-wayland.sh` starts a private two-output headless Sway with
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

The adopted normal workflow keeps the six familiar job/check names: `test`,
`fuzz`, `native-build`, `vulnerability`, `licenses`, `generated-files`. The
experiment's additional `arch-compatibility` PR job is replaced by independently
visible full Arch daily/pre-release workflows, by explicit owner decision.
Repository rules/settings are not edited here. Administrators must inspect any
rules that explicitly required the experiment-only check before merging; do not
leave an obsolete required name blocking every PR or claim protection changed.

The measurement tool defaults to the six current normal gates. The retained
historical comparison script explicitly validates its original seven-plus-four
contract; historical profiles are not silently reinterpreted as normal CI.
Daily and pre-release validation are separate cohorts, including their aggregate
and readiness jobs where applicable. Their compute cost is reported separately.


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
go-licenses report ./... --include_tests --ignore=github.com/sh4869221b/azerlay > "$RUNNER_TEMP/nix-dependency-licenses.csv"
```

After successful report generation, `actions/upload-artifact@v4` publishes
`nix-dependency-licenses` for seven days in normal CI (the separate Arch
workflow uses an `arch-` artifact name) and fails if the file is absent. The
report includes test dependencies and excludes only the project's own
MIT-licensed module, not its third-party dependencies. Stderr remains visible. Upstream
may report unknown licenses or license/URL discovery warnings while exiting
successfully; these warnings are not converted into failures by custom rules.
Every actual nonzero tool result fails the job.

The CSV inventories Go dependencies; it is not an allowlist or a compatibility
decision, and does not cover dynamically loaded or native libraries. The
project uses MIT for its own material. Third-party notices and source
availability are recorded in `THIRD_PARTY_NOTICES.md`; final native inventory,
SBOM and release-readiness checks remain tracked in Issue #18. A successful
license job does not establish that these checks are complete.

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

## CI-wide measurement contract

The initial [Issue #135 measurement contract](measurements/ci-60s/README.md)
defines distinct execution/queue/event clocks, a six-gate correspondence,
nearest-rank p50/p95 and condition-specific cohorts. The offline
`scripts/ci_measure.py` replays saved API responses and retains failed/cancelled
attempts; it neither starts runs nor changes the required gates. The included
historical run has 186 seconds of required execution and 189 seconds from run
creation to last API update. This clock correction is not a speedup, and one
sample does not establish a p95. All native checks and safety budgets above
remain mandatory.

### Current performance goal

The owner revised #134/#135 on 2026-10-07: ordinary warm/source-only full-required
execution targets p50 <=180 seconds and p95 <=300 seconds. Keep queue/event time
and cold/dependency/native refresh separate. The original 60-second target is
historical. Current 159-second ordinary and 196-second diagnostic observations
are single samples, not established p50/p95. Use representative natural runs
first; no automatic large benchmark campaign or mandatory broad native rewrite.

### Bounded #138 candidate validation

The test-only candidate stores real compositor screenshot crops using lossless
PNG without compression. Pixel/bounds round trips, deterministic equal-image
bytes, per-channel differences and source immutability are checked. Only the
two independently owned CLI input-fixture groups become parallel; all 68 routes
and sequential per-store children remain. No decoder caps, native samples,
sleeps, deadlines, frames or production code change. Larger temporary PNG files
are a tradeoff to measure.

One diagnostic validation run repeats the CLI input groups, live-overlay modes
and live-latency test three times, and the pure capture-encoding helper 20 times.
These are focused stress repetitions inside one workflow, not 20 workflow samples
or a p95 campaign. Ordinary required commands retain count=1. Stress and JSON
artifact overhead must stay separate from ordinary-budget conclusions; failures
remain blocking. Large-decoder parallelism and larger architectural experiments
are deferred under the revised goal.

## Completed Ubuntu/Nix comparison and adoption

The [retained comparison evidence](measurements/ci-136-comparison/README.md)
records all eleven jobs, actual checkout provenance, cold corrections and the
single final native-warm pair. Its two seven-gate profiles include shared Arch;
those historical measurements do not become six-gate normal-CI observations.
The owner subsequently selected normal Nix CI with daily/pre-release full Arch.
The comparison workflows are archived under docs, not active parallel workflows.

The exact adopted head must pass all six normal gates before merge. Workflow
and cache-recipe changes invalidate compiled caches conservatively; a cold
validation does not establish warm p50/p95. Scheduled workflows begin only after
this configuration reaches the default branch. There is no release publisher;
manual pre-release verification and its enforcement boundary are documented in
[the adoption policy](ci-adoption.md).
