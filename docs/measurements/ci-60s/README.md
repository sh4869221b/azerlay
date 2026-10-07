# CI 60-second measurement contract (Issue #135, initial stage)

This is measurement infrastructure, not a CI speedup or a completed acceptance
claim. The six existing workflow gates and cache keys remain; a Python tooling
self-test is added to `generated-files` without removing any existing check. The
measurement scripts never authenticate, start CI, mutate caches, or modify
branch protection. Python 3.10+ is needed only for offline analysis.

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

Use nearest-rank percentiles: sorted sample at `ceil(p*n)`, one-indexed. Fix the
cohort, variant, repetition plan and selection window **before** running. Plan
20 measured runs each for identical-commit warm and source-only conditions.
Plan the same for cold, Go dependency refresh, native/image refresh and tool
refresh only after estimating their runner cost and receiving authorization for
that experiment. A plan does not authorize dozens of CI runs. Keep priming runs
separate and record every failed, cancelled, timed-out and outlier attempt.
Reruns are new attempts, not replacements for failed observations.

Each compared variant needs a fixed revision and documented environment cohort.
Alternate baseline/candidate runs where practical; do not pool different
revisions, gate manifests or environments. For source-only runs, prime the
parent and repeatedly measure the same source-change commit. Same-commit warm
runs need evidence of identical inputs and actual cache reuse, not an inferred
label from short duration. A cohort ID identifies the reviewed fingerprint and
protocol; the analyzer does not prove that its human-supplied label is true.

The analyzer reports descriptive percentiles even with one sample and marks
`small_sample_warning`. Its `warm_60s_observed` flag requires at least 20 complete
successful samples in one warm/source-only cohort, the exact initial six-gate
set and p95 <=60 seconds. Future gate splits need a reviewed manifest update
before this initial implementation can issue that flag. This is
an observation, not a statistical confidence guarantee or an overall Issue #134
completion. Missing/skipped/failed required jobs block it. All other conditions
must report their gap to 60 seconds, even if warm passes.

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
  --condition historical --cohort main-09db7b3d-observation
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
The JSON parser ranks package/test terminal events without summing them. A
truncated stream or crash requires the original command's exit status and log;
a terminal-event list alone cannot establish complete test execution.

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
| Race, shuffle, uncached all-package tests | `test`, `go test -race -shuffle=on -count=1 ./...` | Every package maps to a required gate; CGO=0 is not a replacement for native tests |
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
