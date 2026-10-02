# Go build cache measurements for issue #114

Measured on GitHub-hosted Actions, 2026-10-02. This is cache verification,
not a claim of a measured incremental 20–26 minute optimization.

## Method and scope

All ordinary quality gates are retained: formatting, vet, Staticcheck,
non-root Wayland tests with `-race -shuffle=on -count=1`, production GTK/CGo,
the CLI TestMain's 30-second ordinary build, and the final binary build.

Initial implementation head: `7f07ea95e1153dc6a0d0c9873e4aaf36f86ceab5`.
Temporary measurement head: `4dd26eb807fa272b61ed3165f0ccf7dbc31fb71b`.
The temporary matrix and source probe are removed from the final PR tree.
No caches are copied or promoted between branch scopes.

The source control adds a comment to a Go test file and diagnostic workflow
changes (`-x`, without changing compilation mode). Native/toolchain and module
inputs remain identical. The two invalidation cases use the same diagnostic
source tree and separately (a) add an explicit pinned Go requirement already
recorded in go.sum, or (b) install an additional Arch package. These are real
manifest/inventory changes in hosted containers, not mocked-key tests. They
are **not** benchmarks of an effective Go dependency-version upgrade or a
native ABI transition. Such future upgrades still need their ordinary QA.

Cache-key contract tests additionally verify toolchain/settings boundaries,
tracked asset changes, repeatability, and fail-closed behavior when `go`,
`pacman`, or `pkg-config` fails. Those checks are synthetic, not real toolchain
upgrade timings.

## Historical baseline

| Run | Classification | Test job | Seven-job runner-seconds |
| --- | --- | ---: | ---: |
| [36297704607](https://github.com/sh4869221b/azerlay/actions/runs/36297704607) | Existing warm main | 180 s | 590 s |
| [36300187973](https://github.com/sh4869221b/azerlay/actions/runs/36300187973) | Existing warm main | 176 s | 584 s |
| [36307801200](https://github.com/sh4869221b/azerlay/actions/runs/36307801200) | Cold PR after native inventory change | 1662 s | 2050 s |
| [36309312391](https://github.com/sh4869221b/azerlay/actions/runs/36309312391) | Cold main, separate cache scope | 1769 s | 2182 s |

The warm runs restored the same 199,985,623-byte combined setup-go archive.
PR #113 added grim and two font packages; Go dependency files did not change.
Its new native key correctly missed in PR/main scopes. The prior cache was
already useful. These runs have different source/native inputs and are not
matched A/B measurements of this patch.

## Controlled post-change observations

| Case | Run / job | Test job | Vet | Race test step | Build restore / save |
| --- | --- | ---: | ---: | ---: | ---: |
| Cold | [36967167062 attempt 1](https://github.com/sh4869221b/azerlay/actions/runs/36967167062/attempts/1), 110713312949 | 1747 s | 750 s | 872 s | 0 / 7 s |
| Same-commit warm | [36967167062 attempt 2](https://github.com/sh4869221b/azerlay/actions/runs/36967167062/attempts/2), 110720340649 | 188 s | 1 s | 96 s | 6 / 0 s |
| Source-generation fallback | [36969867652](https://github.com/sh4869221b/azerlay/actions/runs/36969867652), 110721400408 | 178 s | 0 s | 78 s | 9 / 5 s |
| Go dependency manifest change | [36969867652](https://github.com/sh4869221b/azerlay/actions/runs/36969867652), 110721400389 | 1675 s | 717 s | 833 s | 1 / 6 s |
| Native inventory change | [36969867652](https://github.com/sh4869221b/azerlay/actions/runs/36969867652), 110721400410 | 1712 s | 741 s | 855 s | 0 / 6 s |

Test-step times include compilation and are not test-body-only durations.
Same-commit warm saved 1559 seconds versus that deliberately cold run. This
is a cold-versus-warm difference, **not** incremental improvement over the
old already-warm design. The 178-versus-188 second variation is not evidence
of an additional ten-second optimization.

Cold and same-commit warm use compatibility fingerprint
`94a88813db5bdab0b91826bc28b27880fbd3699e342d0b8a5729bb3b671aa681`,
dependency fingerprint
`6ee190b7e8b48ffe6d705e5ff6f65ecfdd51c3df8cf605704ae1eb6d163fbb56`,
and source generation
`daecf31e3a1b39fd8800a4135aecd944b734b680560aa54665407bd343add7b7`.
The source case keeps the first two hashes, restores that older generation,
and saves
`db58506943cbb9b0ba4869ea025ea7ae9d9dc19bfdaa0568b6c11fd355ba3b36`.

All five cases passed. The pinned additional Go requirement was
`github.com/davecgh/go-spew@v1.1.1`; it changed the dependency fingerprint to
`abcaa6658bc462b00966a99c14254d7c3de0591083ddd5cbbbb94952ea1cc0e4`.
Installing `strace` changed the complete native fingerprint to
`7b58185e7d4e53b61bccc4ed17b8bcb7006b6d0f509e38ec5b96d76daafc9072`.
Each case rejected the previous compatible snapshot and executed cold. Their
`-x` logs each show 40 gotk4 compile and 32 cgo commands across ordinary/race
modes, versus zero of each in the source-fallback control. This demonstrates
actual miss/recompilation behavior. Adding a non-ABI package also invalidates
by design; this preserves the existing conservative safety boundary.
 Ordinary/race Go stale-state checks were false for every
one of the 20 gotk4 packages in each mode. The source-case `-x` logs contain
zero gotk4 compiler or cgo invocations, despite cached warnings appearing.
Restored ownership was repaired before non-root execution; the full tests,
including the bounded ordinary CLI build, succeeded using that cache.

## Cost and interpretation

The first build snapshot expanded to 1,064,383,256 bytes and compressed to
178,774,928 bytes; modules were separately 59,881,232 expanded / 24,709,950
compressed bytes. The source generation compressed to 180,999,980 bytes.
Warm module restore took 3 seconds; source-case module restore took 2 seconds.
Those module figures are not credited as GOCACHE savings.

Each changed tracked tree can add roughly 180 MB in this sample and 5–7
seconds of save overhead. Identical trees do not resave. Documentation-only
changes also rotate the conservative source-tree hash. GitHub's cache quota
and eviction remain relevant: this policy trades storage/transfer cost for
freshness, not free speed. Cold execution and safe invalidations remain slow.

The first whole seven-job run used 4328 runner-seconds. The same-commit rerun
executed only `test` (188 additional runner-seconds); successful jobs shown
again by the run API were reused and must not be counted as fresh work.
The temporary nine-job measurement run used 3958 runner-seconds, of which
3387 seconds were the two deliberate cold invalidation cases. Initial cold,
same-commit warm, and the matrix together used 8474 runner-seconds (141m14s).
These are measured allocations, not a billing estimate. Temporary measurement
jobs are one-time QA cost, not the final workflow's steady job matrix.

Exact final-head ordinary seven-job results and repeated warm CI measurements
are recorded in [PR #120](https://github.com/sh4869221b/azerlay/pull/120) after
cleanup. Updating that report does not change the tested source tree.

No statistically established incremental warm speedup or average saving is
claimed. The observed benefit is a successfully refreshed source generation
while retaining normal/race reuse, explicit compatibility boundaries,
producer ownership, and observable cache behavior. Evaluate the added save
cost against real future source-change workloads before expecting a net
performance improvement.
