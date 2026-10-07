# CI measurement contract: ordinary p50 3 minutes, p95 5 minutes

This is measurement infrastructure, not a CI speedup or a completed acceptance
claim. The six existing workflow gates and cache keys remain; a Python tooling
self-test is added to `generated-files` without removing any existing check. The
measurement scripts never authenticate, start CI, mutate caches, or modify
branch protection. Python 3.10+ is needed only for offline analysis.

## Stable-native migration

The initial six-gate mapping and historical observations below are preserved.
The current Ubuntu 26.04 candidate adds a seventh blocking `arch-compatibility`
job on every PR and main push; the default measurement manifest includes it.
See [the migration contract](../../ci-stable-native.md). Full-workflow timing
must include Arch setup, validation and cleanup. Historical six-job replay uses
an explicit original job list and cannot establish the current seven-job budget.

## Clocks and acceptance

- **Required execution wall time:** earliest required job `started_at` through
  latest required job `completed_at`, including job preparation and post steps.
  Parallel package and step durations must not be added to this number.
- **Runner-seconds:** sum of required job intervals, not elapsed wall time or
  billed minutes. Cost requires runner prices and billing granularity separately.
- **Run-created to required-result:** `created_at` through the last required
  completion. A dependent job's delay includes dependencies, not just queue.
- **Run-created to updated:** an API lifecycle observation, not execution time.
- **Push/PR-event to result:** only calculated with a verified event timestamp.
  Neither commit author time nor API `created_at` substitutes for the event.
- Queue, per-job creation/start, and results-notification delivery are separate
  measurements when those timestamps are available. Unknown stays unknown.

## Current goal (owner update, 2026-10-07)

For ordinary warm/source-only runs, required workflow execution aims for **p50
<=180 seconds and p95 <=300 seconds**. Queue/event-to-result remains separate.
Cold, Go dependency changes and native/image/tool refresh have separate cohorts
and regression/failure reporting; these bounds are not guarantees for them.
The original 60-second goal and this directory name are historical. No broad
native replacement, speculative cache pipeline or extra fuzz sharding is mandatory
solely to reach that old number. All existing quality gates remain required.

Use the predeclared nearest-rank p50/p95 method: sorted sample at `ceil(p*n)`,
one-indexed. First use comparable existing and ordinary-development observations.
Fix representative conditions, the selection window and anticipated cost before
requesting extra runs. At least 20 comparable observations is the minimum for
an empirical p95 budget flag, not a statistical confidence guarantee. Small
samples support preliminary range/p50 discussion only. Do not automatically
launch 20 runs per condition, especially cold/refresh. A bulk campaign requires
an explicit necessity/cost decision. Keep priming and instrumented runs distinct.
Retain every failed, cancelled, timed-out and outlier attempt.

Reruns do not replace failed observations. Failed-jobs-only retries need separate
recovery records: GitHub copies prior successful jobs with new IDs and attempt
numbers but old execution timestamps. The analyzer rejects that mixed timing
history using `run_started_at`, so prior runner time is not counted twice or
presented as an independent successful sample. Preserve the failed attempt and
the retry's raw data/quality outcome together.

Each compared variant needs a fixed revision and documented environment cohort.
Alternate baseline/candidate runs where practical; do not pool different
revisions, gate manifests or environments. For source-only runs, prime the
parent and repeatedly measure the same source-change commit. Same-commit warm
runs need evidence of identical inputs and actual cache reuse, not an inferred
label from short duration. A cohort ID identifies the reviewed fingerprint and
protocol; the analyzer does not prove that its human-supplied label is true.

The analyzer reports descriptive percentiles even with one sample and marks
`small_sample_warning`. `ordinary_budget_observed` requires at least 20 complete,
successful comparable samples, the complete current seven-job execution set, an ordinary
warm/source-only condition, p50 <=180 seconds and p95 <=300 seconds. Missing,
skipped, failed or cancelled checks block the flag. `budget_applicable` is false
for cold/refresh/historical cohorts; report those outcomes separately. The flag
is an empirical observation, not statistical confidence or automatic issue closure.
Any future gate split requires a reviewed complete execution-manifest update.
Saved summary files can be regenerated under this current policy without changing
raw run evidence or pretending historical one-sample outcomes established p95.

## Reproduction

Export the run response and **all pages for the specific attempt** through an
existing authorized read-only GitHub client. For example, the API resources are:

```text
GET /repos/sh4869221b/azerlay/actions/runs/RUN_ID/attempts/ATTEMPT
GET /repos/sh4869221b/azerlay/actions/runs/RUN_ID/attempts/ATTEMPT/jobs?per_page=100&page=PAGE
```

Do not save credentials in the measurement directory. Preserve API timestamp,
run ID, attempt, head SHA, job names and URLs. Missing pagination, duplicate job
IDs, mismatched attempts/commits and duplicate required names are rejected.
The script only reads local exports; GitHub response access is not required to
run the tests or replay the included observation.

```sh
python3 -m unittest discover -s scripts/tests -v
python3 scripts/ci_measure.py run \
  --run docs/measurements/ci-60s/37160320446-run.json \
  --jobs docs/measurements/ci-60s/37160320446-jobs.json \
  --condition historical --cohort main-09db7b3d-observation \
  --required test fuzz native-build vulnerability licenses generated-files
python3 scripts/ci_measure.py summary \
  docs/measurements/ci-60s/37160320446-measurement.json
```

For a candidate matrix, provide **all actual required job names** through
`--required`; do not list just an aggregator or the fast shard. Review this list
against workflow and actual check names. Measurement output is not a CI gate:
a successful analyzer process means parsing succeeded, not that CI passed.
Review `quality_success`, `errors` and the complete cohort report. Do not make
this offline script an always-green required-check proxy.

For diagnostic native runs, before/after the unchanged validation commands:

```sh
bash scripts/ci-measure-environment.sh "$RUNNER_TEMP/ci-environment-before"
set -o pipefail
scripts/test-wayland.sh go test -json -race -shuffle=on -count=1 ./... \
  | tee "$RUNNER_TEMP/go-test.jsonl"
python3 scripts/ci_measure.py tests "$RUNNER_TEMP/go-test.jsonl"
bash scripts/ci-measure-environment.sh "$RUNNER_TEMP/ci-environment-after"
```

The Wayland command must run under the existing dedicated **non-root** user
and ownership setup. This example does not replace it. JSON and profiles are
optional diagnostic runs, timed separately from ordinary CI. Record paired
instrumented/uninstrumented durations; do not assume their overhead is zero.
The JSON parser preserves Go's reported active `seconds` and separately pairs
package `start` / test `run` with terminal events to obtain `wall_seconds`.
Parallel parents can have short active elapsed while waiting for child tests;
the report ranks observed wall time when available and otherwise falls back to
reported elapsed. Wall time includes scheduling pauses and child waits; it is
not CPU time or isolated test execution cost. Repeated tests are paired
independently. Missing starts leave wall time unknown. Neither clock is summed across overlapping parent, child,
and package events. A truncated stream or crash requires the original command's
exit status and log; a terminal-event list alone cannot establish complete test
execution.

The environment capture fails if required native tools are absent; partial files
are not a successful capture. Retain separately from Actions logs: resolved Go,
container image digest and pull bytes/time, full native inventory, full cache
key/hit/miss, compressed restore/save bytes and duration, module extraction,
scanner load/check, cleanup, CPU/memory peak and runner type. Do not derive image
digest from a mutable tag or shrink the native cache fingerprint. The script
captures only allowlisted metadata and local raw cache size; it does not dump
credentials, all environment variables, HOME, private profiles or fuzz inputs.

## Quality mapping, unchanged initial stage

| Invariant | Required gate / implementation | Migration rule |
| --- | --- | --- |
| Format, all-package vet and Staticcheck | `test`, `.github/workflows/ci.yml` | Include new core/native packages; no path-filter gap |
| Race, shuffle, uncached all-package tests | `test` native manifest + `generated-files` core manifest, both `-race -shuffle=on -count=1` | Exact disjoint manifest union must equal root `go list ./...`; CGO=0 is additional, not a race/native replacement |
| GTK main OS thread, non-root Sway, two outputs | `test`, `scripts/test-wayland.sh`, native subprocess tests | Keep production GTK/CGo and runtime ownership |
| Current-checkout CLI/process contract and native build | `test` CLI TestMain and final build; `native-build` explicit/default CGo, version and ELF checks | No prebuilt executable or fake UI substitutes |
| 64 MiB and 64 MiB+1, bounded complete decode, trailing-data rejection, zero-on-error | `test`, `internal/profiledecode`; `generated-files` actual zero-fixture regeneration and diff | Preserve real boundary sizes and safety seeds |
| Cancellation/reload, last-known-good, lifecycle/cleanup | `test`, source/runtime/native/CLI packages | Preserve real timing integration, not solely fake clock tests |
| Seven parser/adapter fuzz targets | `fuzz`, each 10s, `-parallel=1`, `-timeout=2m`, anchored target | No target/seed/budget loss; all shards must finish |
| Reachable vulnerabilities | `vulnerability`, `govulncheck ./...` | Current checkout and current vulnerability DB; failure remains blocking |
| Go dependency license report including tests | `licenses`, `go-licenses report ./... --include_tests` and report upload | Preserve scope and nonzero failure behavior |
| Actual regenerated fixtures | `generated-files`, generator and exact diff | No cached-success substitution |

Before adopting splits, record `go list ./...`, the full dependency and test
manifest, and a before/after correspondence. A synthetic analyzer unit test is
not a real CI failure-propagation test. In an explicitly authorized experiment,
inject one failure at a time in each gate/shard; then cancellation and timeout.
Confirm actual required check failure, missing/skipped rejection and matrix
completion behavior. Do not change repository required-check configuration as
part of this initial tooling stage.

## Retained historical observation

[Run 37160320446](https://github.com/sh4869221b/azerlay/actions/runs/37160320446),
main `09db7b3d0525de5cbe1f00ba111de10d271ba4c8`, attempt 1:

- Required execution: **186 s** (23:00:20Z to 23:03:26Z).
- Created to required completion: **188 s**; created to updated: **189 s**.
- Required runner-seconds: **539 s**.
- `test` 185 s; `fuzz` 120 s; `vulnerability` 80 s; `native-build` 68 s;
  `licenses` 66 s; `generated-files` 20 s.
- Actual push timestamp, hardware CPU, full image/cache provenance and local
  resource peaks were not recovered in this initial export. Do not invent them.

The issue's 189-second observation used a different clock from its explicit
first-job-to-last-required-job definition. Both are preserved here; this
3-second difference is **not an optimization**. One observation is not evidence
of p95 stability. The older cold run uses another commit and is not an A/B
comparison. The JSON exports retain the relevant API data; unused account and
repository fields are omitted from the run export.

## Cloud execution limits for this initial implementation

The dot cloud computer provides Python, shell and source editing. It has no Go
SDK (the `/usr/bin/go` command is a different program), GTK4/layer-shell, Sway,
or Docker/Podman. Direct clone of this private repository cannot authenticate;
authorized GitHub reads supplied a **partial, revision-pinned source snapshot**.
The public Go download probe timed out. No credentials or network permissions
were changed. Python tests and replay were executed here; Go build/race/shuffle,
actual native lifecycle and all-six-gate failure-injection runs were **not run**.

This finishes a reviewable initial measurement-tooling patch, not all acceptance
criteria of #135. Stages #137/#138 cannot be safely measured or declared complete
without an appropriate toolchain/native environment. Real GitHub Actions runs,
publication and repeated cold/warm experiments require their own authorized
execution route. Hosted validation is tracked separately in the pull request; no merge or
protection change is part of this patch.

## Hosted validation after initial staging

The first authorized [PR #140 validation](37553800743-notes.md) passed all six
gates at exact head `4faedfba7b67f650907b48efefeb0fe54df190fe`. It required
1,782 seconds with changed native fingerprints and cold native build caches;
application/tool module caches hit. This is retained as a native-refresh
observation and cache-priming run, not a warm A/B result or a speedup claim.
The 22 Python tests also passed on Actions. Local runtime limitations above
remain distinct from this successful hosted validation.

## Bounded candidate validation: run 37565507636

Candidate `549e1c51` completed all six jobs but failed the core race gate:
`TestControlTimeoutClient/wrong-id` attempted its fake-server response after the
fixture's shared 80 ms client deadline, receiving a broken pipe at 0.18 s. The
other five gates passed, including the full native race suite and three shuffled
repetitions of the changed native capture/CLI tests. The pure PNG helper's 20-repeat
check and fixture regeneration were skipped after the core race failure. Retain
this failure, rather than treating a retry as an independent successful sample.

The workflow measured 249 s execution and 730 runner-seconds, including the extra
native stress (94.77 s); it is diagnostic and ineligible for the ordinary budget.
The ordinary native race command was 56.69 s (baseline 73.74 s), child user/system
CPU 42.49/13.42 s (67.14/14.86 s), and maximum child RSS 629512 KiB (616860 KiB).
These are single observations, not a performance acceptance or process-tree peak.

The correction keeps the actual timeout case at 80 ms, uses the existing 5 s
production allowance for response/cancellation cases, and keeps the fixture's
3 s guards, exact error codes and closed-connection/no-retry assertions. A focused
20-repeat race/shuffle check validates these cases in the corrective workflow.
No production timeouts or 64 MiB boundary tests change.

The native diagnostic upload reported only one file because its `runner.temp`
expression used a host path inside a container job. The corrective workflow emits
the actual shell `RUNNER_TEMP` as a step output for both native artifact paths;
verify that both the JSONL and timing report upload. Timing values above remain
available in the job log even for the original upload.

## Corrected candidate and removal of one-off stress

[Run 37566091667](https://github.com/sh4869221b/azerlay/actions/runs/37566091667)
at `1d97a8d4327aef9d81a32cf6b393ed1e6bfb9c9f` passed all six gates, the three
native/CLI repetitions, 20 pure PNG-helper repetitions, 20 timeout-client
repetitions, and real boundary-fixture regeneration/diff. Native JSONL and timing
artifacts each now upload both files. The observed execution was 244 s and
744 runner-seconds, including 91.90 s of one-off native stress. Native ordinary
race was 54.28 s (user/system CPU 43.56/8.86 s, maximum child RSS 627040 KiB);
core race was 108.86 s. Selected events and metrics are retained beside the raw
run/job data. This is a diagnostic sample, not ordinary p50/p95 evidence.

After this successful validation, the one-off count3 native stress and count20
helper/control invocations are removed. All named tests remain in the ordinary
native/core race manifests and run with `-race -shuffle=on -count=1`; CGO-disabled
core validation, seven 10 s fuzz targets, and all other gates remain. The next
single final-code validation retains JSON/timing instrumentation and must report
that instrumentation when comparing results. No bulk workflow campaign is started.

## Ordinary-count instrumented final validation

[Run 37566672762](https://github.com/sh4869221b/azerlay/actions/runs/37566672762)
at `709b05fdbb3e42051ac121a036bfefb126966b60` passed all six gates with ordinary
counts after removing the temporary stress commands. Execution was 162 s,
run-created to required result 164 s, and runner-seconds 635. Native race took
55.58 s and core race 101.57 s. The exact native cache key hit and both files
were uploaded for each race diagnostic artifact. This is one source-only,
normal-count observation with JSON/timing instrumentation, stored as diagnostic
so it cannot silently enter the ordinary budget cohort. It does not establish
p50 or p95. No extra sampling campaign was run.

A subsequent offline tooling audit tightened summary input validation: saved
successful samples must agree with their embedded required job rows and derived
execution/runner intervals. A top-level success flag cannot override missing,
failed, duplicated, or incomplete job evidence. Parallel-test hotspot reports
also now expose wall time rather than mistaking a parent's short active elapsed
for the duration of its children.
