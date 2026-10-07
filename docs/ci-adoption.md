# Adopted CI and pre-release verification policy

Owner decision: 2026-10-07. Keep the existing quality checks and use pinned Nix
for normal PR/main validation. Run the latest Arch environment daily and before
version publication. The owner explicitly accepted later detection of an
Arch-specific regression compared with checking rolling Arch on every PR.

## Normal PR/main CI

`.github/workflows/ci.yml` runs six familiar jobs on every pull request and main
push: `test`, `fuzz`, `native-build`, `vulnerability`, `licenses`, and
`generated-files`. Four native roles use the [locked Nix environment](ci-nix.md).
Core CGO=0 plus fresh race validation and all seven 10-second fuzz targets remain.
Every check still builds/scans/tests the current checkout. No cache hit skips a
quality command. All runners use the standard `ubuntu-24.04` hosted class.

The old Ubuntu/Nix comparison workflows are archived under
`docs/measurements/ci-136-comparison/workflows/`. They are evidence, not scheduled
or PR-triggered Actions files. There is no duplicate four-job comparison run.

## Daily latest Arch

`.github/workflows/arch-ci.yml` retains the complete original six-gate suite,
including scanners, core race/CGO=0, fuzz, native race/Wayland and generated-file
checks. The four native roles run in Arch containers; the two GUI-free core/fuzz
gates keep the original non-root Ubuntu hosted environment. Native roles use rolling `archlinux:base` and `pacman -Syu`; the complete
installed native inventory and actual Arch workflow recipe isolate compiled
caches. A native update legitimately forces recompilation.

The cron is `17 20 * * *`: 20:17 UTC, or 05:17 JST the following day. This is a
GitHub Actions schedule, not an assistant reminder. GitHub may delay scheduled
jobs; it is not an exact-time service guarantee. Scheduled workflows use the
latest default-branch commit and become active only after this configuration is
merged there. GitHub may disable scheduled workflows in a public repository
after 60 days without repository activity; check Actions status if daily runs
stop. The full Arch workflow is also manually dispatchable and reusable
by the pre-release entrypoint. Its aggregate fails for a failed, skipped or
cancelled required gate. Its failures remain visible independently of normal CI.

## Before version publication

`.github/workflows/release-validation.yml` is a manual verification entrypoint.
It runs the complete reusable Arch suite against the dispatch's immutable
`github.sha`, then requires its success before the final readiness result.
No moving branch or later checkout may substitute for that tested commit.
Release-readiness blockers in #18/#39 and the [release-safety decision](decisions/public-release-safety.md)
remain separate from a green CI result. The final pre-release readiness job
performs a read-only API check and fails unless both issues are closed with
reason `completed`; a failed/read-denied API call also fails closed. Review the exact source/binary artifact,
SBOM, dependency notices, rights/provenance and other release prerequisites.
A closed issue alone is not a legal clearance or a completed artifact review.

There is currently no automated release publisher. These workflows do not
create tags, packages, GitHub releases, signing credentials or repository
security settings. Passing verification does not itself publish or authorize a
release. Conversely, this verification workflow cannot technically prevent an
owner from using GitHub's manual release UI or an unrelated external publisher.
Use the verified exact commit before publication; any future publisher must
explicitly depend on this same-commit validation before its publication step.
A `release: published` hook would be too late and is not used here.

## Required checks and operating boundary

The normal six check names are retained. Repository branch/tag rules and required
checks are not changed by this PR. If an administrator configured the comparison's
extra `arch-compatibility` check as required, they must deliberately reconcile it
with the approved separate daily/pre-release policy; do not pretend the setting
was changed or create an always-green substitute.

Daily Arch findings may arrive after a normal Nix PR is green or merged. They
must be investigated before a version release; moving the schedule is a latency
versus detection tradeoff, not expanded platform qualification or reduced tests.
A post-merge first normal run and first daily/manual invocation are distinct
observations. Do not claim those events ran while this configuration is only in
a draft PR.

## Evidence and remaining performance uncertainty

The [completed comparison](measurements/ci-136-comparison/README.md) found native
warm elapsed 106s Nix versus 276s Ubuntu; native runner-seconds were 227 versus
867. Both complete historical seven-gate profiles took 1,518s because rolling
Arch became cold. The adopted normal workflow has six gates, so its exact-head
run must be measured as a new cohort. Old native-only numbers are not a measured
six-gate p50/p95 result. Workflow/cache-input restructuring also causes a valid
initial cold boundary; do not relabel it warm.

The target remains normal warm/source-only p50 <=180s and p95 <=300s, unproven by
one pair. No repeated benchmark campaign, larger runner, paid cache or new
credential is part of this adoption. Source/checkout provenance, failed runs,
setup/cache transfer and summed runner-seconds remain part of reports.

## GitHub execution references

- [Schedule events and default-branch/inactivity behavior](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#schedule)
- [Same-repository reusable workflows use the caller commit](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows#calling-a-reusable-workflow)
- [Reusable workflow GitHub context belongs to the caller](https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations#github-context)
