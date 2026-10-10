# CI and synthetic fixture policy

[CI](../.github/workflows/ci.yml) runs for pull requests and pushes to `main`.
It has read-only repository permissions and cancels superseded runs for the
same pull request. All Go jobs use Go 1.27.2. CI-only tools are installed with
explicit versions without changing the application's `go.mod` or `go.sum`.
The owner selected pinned Nix for normal CI on 2026-10-07. Latest Arch runs
independently each day and through manual pre-release verification; see the
[adopted trigger and release-verification policy](ci-adoption.md).

## Dependency updates

[Renovate](../renovate.json) proposes updates for the application and Layer Shell
probe Go modules, GitHub Actions (including composite actions), and the
govulncheck, go-licenses, and Syft versions in `.github/ci-tools.env`. Updates
require review and are not automatically merged. Enable the Renovate GitHub App
for this repository to run these updates after the configuration reaches `main`.

The Go SDK, Nix inputs, and isolated Staticcheck module remain manual updates.
The Nix preparation script verifies exact source pins and native library
versions, and the Staticcheck installer verifies the Go SDK, compatibility
source, and importer versions. Update and validate those coupled pins together;
Renovate excludes them to avoid proposing partial updates that fail these checks.

### Go dependency notices

Renovate updates the application `go.mod` and `go.sum`. After that final Go
artifact update, [the notice workflow](../.github/workflows/renovate-notices.yml)
runs the existing [`scripts/go_notices.py`](../scripts/go_notices.py) generator
and commits only the generated Go table in `THIRD_PARTY_NOTICES.md` to the same
Renovate PR. Every requirement comes from the final `go.mod`, including indirect
versions raised as a side effect of updating a direct dependency. There is no
second regex-managed copy of module versions. CI tools, probe modules, native
notices and historical `go.sum` provenance remain outside this table.

The workflow uses `pull_request_target` to execute trusted base-branch code and
policy, never scripts or dependency code from the PR. Generation has a read-only
token; the separate publisher has an explicitly approved ephemeral
`contents: write` / `actions: write` token. It accepts only open, same-repository
Renovate PRs targeting `main`, with application module/notice changes only.
The PR must include the current trusted base; older branches need rebasing first
so the resulting head also contains the current policy and complete CI workflow.
SHA-checked inputs and an atomic exact-head git lease prevent a stale result
from overwriting a changed branch. The commit is always a one-file child of the
expected head, so the lease cannot authorize a history rewrite. Curated prose
outside the generated block must match the trusted base. No PAT, App secret,
repository setting or permanent permission change is needed.

An unchanged table produces no commit or dispatch. After a changed table is
committed, the publisher dispatches the existing six-gate normal CI on that exact
head. A moved branch fails the dispatch guard; all existing checks remain enabled.
Explicit dispatch is needed because [GITHUB_TOKEN-induced PR runs require approval](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).
The exact `azerlay-notices@users.noreply.github.com` author/committer email is
[ignored by Renovate](https://docs.renovatebot.com/configuration-options/#gitignoredauthors)
so this generated commit does not stop its normal rebasing. Other authors remain
visible to Renovate.

This runs with free Community Cloud and Community OSS Cloud: it does not request
[arbitrary Renovate post-upgrade commands](https://docs.renovatebot.com/mend-hosted/faq/#how-can-i-run-arbitrary-commands-through-postupgradetasks).
The reference [go-nico-list](https://github.com/sh4869221b/go-nico-list/tree/090af4a9570bd6c89fcb4c81786cd6441455f2a9)
uses a GoReleaser generation hook. Azerlay generates during dependency PRs while
preserving its reviewed gotk4 exceptions and native notices.

Both normal CI and full Arch validation retain the read-only check:

```sh
python3 scripts/go_notices.py --check
```

The generator reads requirements with `go mod edit -json` and downloads exact
versions using Go's public checksum database. It never compiles or executes
dependency code. Go and access to `proxy.golang.org` / `sum.golang.org` are needed;
input copies keep `go.mod` and `go.sum` byte-identical.

The existing [reviewed policy](../scripts/go-notice-policy.json) retains terms,
source templates, local notice hashes and recursive upstream
LICENSE/COPYING/NOTICE/PATENTS/COPYRIGHT hashes. Names include case variants and
suffixes separated by `.`, `-` or `_`, such as `LICENSE-MIT` and
`NOTICE-THIRD-PARTY`. The newly detected xz `cmd/xb/copyright.go` contains its
existing BSD copyright template; its full source hash is retained conservatively
without executing that tool or changing the reviewed terms.

New/removed requirements, replacements/exclusions, pseudo-versions,
`+incompatible` tags, changed legal files or local notices require manual review
and fail before publication. gotk4 remains pinned to its reviewed version and
file-level exception hashes. Named-file matching is not an audit of every source
header or bundled asset; existing release/license checks remain required. Do not
approve a new hash just to pass CI: inspect and preserve changed terms first.

Manual regeneration and local regression tests:

```sh
python3 scripts/go_notices.py
python3 scripts/go_notices.py --check
python3 -m unittest discover -s scripts/tests -v
```

Generation changes only the marked table and preserves the historical review
date and manual notices. Tests include added/changed/removed suffixed legal files,
a real offline Go artifact update that raises an indirect version, idempotency,
unchanged module inputs, publisher restrictions and a rejected competing git push.
The workflow becomes active after merging to `main`; existing #158/#159 branches
need rebasing onto that configuration. No merge is performed by this workflow.

An optional offline integration uses real Renovate 44.134.1 with Node 24:

```sh
npm install --prefix /tmp/azerlay-renovate-check --ignore-scripts --no-audit --no-fund renovate@44.134.1
npm rebuild --prefix /tmp/azerlay-renovate-check re2
node /tmp/azerlay-renovate-check/node_modules/renovate/dist/config-validator.js --strict renovate.json
RENOVATE_TEST_DIR=/tmp/azerlay-renovate-check/node_modules/renovate node scripts/tests/renovate_go_notices.mjs
```

It verifies root gomod extraction, indirect updates, removal of notice regex
management, and Renovate's treatment of the exact generated-commit author, using
only a temporary local Git repository.

## Quality gates

| Job | Checks | Job timeout |
| --- | --- | --- |
| `test` | Formatting, all-package vet/Staticcheck, native race tests, binary build | 60 minutes |
| `fuzz` | Seven existing fuzz targets, each with a 10-second fuzz allocation | 10 minutes |
| `native-build` | Verify default CGo, compile the native binary in pinned Nix, run version, and inspect shared libraries | 30 minutes |
| `vulnerability` | Reachable Go vulnerability scan | 30 minutes |
| `licenses` | Third-party Go dependency license CSV | 20 minutes |
| `generated-files` | CI-tool tests, exact Go notices/license texts, core CGO=0 and race tests, regenerate both LZMA fixtures and reject differences | 15 minutes |

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
  `/tmp/azerlay-go-mod/cache/download`, keyed by OS, architecture, resolved Go
  version, `go.mod` and `go.sum`. The module preparation script still downloads
  and verifies the current graph on every run. Job names, script edits, native
  packages and analyzer versions do not invalidate download archives. Only
  `native-build` saves the verified snapshot; cold consumers download normally.
- **Analyzer modules:** each analyzer's `/tmp/azerlay-tool-go-mod/cache/download`
  uses its semantic graph name (`staticcheck`, `govulncheck` or `go-licenses`), OS,
  architecture, resolved Go version and selected Go tool pins. Staticcheck also
  includes its isolated module lock files. Renaming a job, changing another
  analyzer, editing manifest comments or updating Syft preserves that graph's
  key. Purpose-specific graphs retain separate first writers so one analyzer
  cannot publish an incomplete snapshot for a different analyzer.
- **Build results:** `/tmp/azerlay-go-build` is keyed by OS, architecture,
  resolved Go version, application dependency files and the native compatibility
  fingerprint, plus a semantic `normal`/`race` snapshot profile. The Nix
  fingerprint selects compiler, binutils, pkg-config and
  native development/library output store paths, which identify their build
  inputs; the GUI shell closure, fonts, compositor, role name and Nix installer
  version are excluded. Arch uses the installed versions and architectures of
  gcc, binutils, glibc, Linux API headers, pkgconf and actual installed owners of
  the GTK, Layer Shell, GLib, Pango, Cairo and GObject introspection pkg-config files,
  plus their complete dependency closure (including virtual providers), rather
  than all installed packages. This follows header/library package splits and
  avoids selecting an unrelated introspection generator merely by name.
  Missing or ambiguous build
  dependencies fail before restore. Package-level transitive identities are
  deliberately conservative; dependencies of a selected native package still
  invalidate, even if a particular compilation does not use every file.
  Both retain compiler/linker/pkg-config metadata and effective Go target,
  CGO, compiler and link settings. Nix compiler-wrapper settings are retained.
  Arch captures Go settings after setup-go, before restore; its uploaded
  fingerprint includes those settings. Workflow text, job names, scripts and
  unrelated installed packages do not themselves change the build key.
  There are no restore prefixes or cross-environment fallbacks.

Go validates source contents, compiler options and per-command flags such as
`-race` within GOCACHE. A source-only change can reuse the outer snapshot and Go
rebuilds affected entries; application module changes rotate the outer snapshot
so its immutable archive can be refreshed. External C libraries are the crucial
additional boundary: `go help cache` states that Go does not detect their changes.
The native fingerprint retains that boundary instead of forcing every CI edit
through a new key. Updating a recipe affects the key when it changes effective
build inputs/settings, rather than because the recipe text changed. All tests
still use `-count=1`; this shares compiled results, not successful test results.

The `test` gate uses the `race` snapshot profile and saves both normal and race
build entries in its GOCACHE. Other native gates use `normal`. This is a snapshot
ownership decision, not a claim that Go conflates race and normal entries. Go
checks `-race` internally. GitHub cache archives are immutable: allowing a
normal-only first writer to occupy the race job's exact key could leave every
subsequent race run restoring a snapshot without race builds and unable to save
its new entries. Profile names remain stable when jobs/workflows are renamed;
no restore fallback crosses profiles. Equivalent normal builds may still share.
After a compatible warm race snapshot exists, unrelated workflow or analyzer
changes retain both normal and race entries for vet/build/test. The retained
cold/warm behavior has offline regression coverage; the reported long hosted
vet and race durations have not been remeasured with this patch.

The regression suite exercises unrelated workflow/job/session/documentation edits,
selected and unrelated package updates, transitive/virtual dependencies, missing
identities, effective Go/CGO/target/compiler/link settings, analyzer pins and module
lock changes. These are offline key/behavior checks, not measured Actions hits.
The new key schemas incur one initial migration miss. Hosted hit-rate and
wall-time improvements have not been measured for this change.

The module snapshots contain Go's download archives and metadata (`.info`,
`.mod`, `.zip`, `.ziphash`, and cached sumdb data), not duplicate extracted source
trees. Normal `go mod download` and pinned `go install` commands reconstruct
sources and retain their existing checksum checks. This does not promise offline
CI: Go may still query release/deprecation metadata, and vulnerability scanning
still contacts its database. No checksum service is disabled.

App download, checksum verification and an unchanged `go.mod`/`go.sum` check run
on every cache hit or miss. Application and analyzer cache paths do not overlap.
Go keeps individual analyzer build entries within the same native-compatible
GOCACHE used by the application's other checks.

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

Jobs with equivalent native build inputs can reuse the same build snapshot.
Different compiler settings or native dependency identities remain isolated.
Old cache schemas expire under GitHub's normal retention policy.

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
availability are recorded in `THIRD_PARTY_NOTICES.md`; remaining distribution
conditions are recorded in
[`decisions/public-release-safety.md`](decisions/public-release-safety.md). A
successful license job does not establish that these checks are complete.

## Manual release candidate dry-run

The manual `release-dry-run.yml` workflow prepares a non-publishing candidate.
Dispatch it from a branch or tag that contains the workflow and enter the
candidate version. The selected workflow ref itself supplies the source
revision; there is no separate revision input. For a tag, remove exactly one
leading `v` from the tag name for the version input, and the workflow rejects a
mismatch before packaging. Branch refs use the explicit version input.

The workflow runs full Arch validation before packaging and produces source and
binary archives, matching SPDX 2.3 JSON and license-report sidecars,
`release-notes.md`, and `SHA256SUMS`. It validates both SPDX documents against
the pinned SPDX 2.3 schema and checks the listed checksums. Review and retain
the workflow artifact as a candidate only; a successful dry-run does not
resolve the distribution conditions in the release-safety decision or complete
AC-016. The workflow does not sign or publish artifacts. Signing and
publication require a separate later action after the remaining conditions are
met. Dispatch is available only for refs that contain the workflow, which means
older tags without it cannot run this dry-run.

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

The exact adopted head must pass all six normal gates before merge. Changes to
effective build settings or native dependencies invalidate compiled caches; a
cold validation does not establish warm p50/p95. Scheduled workflows begin only after
this configuration reaches the default branch. There is no release publisher;
manual pre-release verification and its enforcement boundary are documented in
[the adoption policy](ci-adoption.md).
