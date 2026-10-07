# Completed Ubuntu / Nix comparison

Date: 2026-10-07. Comparison head
`abef695f45aa57886672839b1f8d8fe7d877675b`, actual checkout
`d2bb6c37e9578ff85c67e1eabadf52ee0aa9a2f0`, identical source tree
`2dfb86d7ebc6d51445bc385d1fb1d833280db4fd` to preceding green head `64ef167`.

[Ubuntu/shared run](https://github.com/sh4869221b/azerlay/actions/runs/37588708053)
and [Nix run](https://github.com/sh4869221b/azerlay/actions/runs/37588708066)
completed all eleven jobs successfully. All eight native build-cache keys hit.
Nix runtime installation and fresh closure download are included.

| Scope | Ubuntu | Nix |
| --- | ---: | ---: |
| Four native gates, elapsed | 276s | 106s |
| Four native gates, runner-seconds | 867 | 227 |
| Seven-gate profile including shared core/fuzz/Arch, elapsed | 1518s | 1518s |
| Seven-gate profile runner-seconds | 2583 | 1943 |

The actual eleven-job experiment consumed 2810 runner-seconds, counting shared
work once. The native Nix profile saved 170s elapsed and 640 runner-seconds;
complete profile runner usage was about 25% lower, with no elapsed reduction
while shared Arch dominated. Native race commands took 47.61s Nix / 47.47s Ubuntu.
Ubuntu package setup took 157–169s per job; Nix installer 3–4s and closure setup
16–22s. Individual CPU models and native versions differed; this is not a pure
package-manager causal result or sufficient p50/p95/variance evidence.

The Arch cache correctly missed after GTK 4.22.5→4.24.1, AT-SPI
2.60.7→2.62.0.1 and other rolling updates. The preceding change had included
GLib 2.88.3→2.90.1. Full profiles are native-refresh observations even though
all eight candidate-native caches were warm.

Corrective runs exposed Nix's version-4 derivation JSON envelope and omitted
fontconfig bitmap scaling; both were fixed without weakening application
assertions. Real accessibility bus/registry and Unicode/Wayland tests passed.
All resumed comparison work, including failed/cancelled/corrective runs, totalled
15746 runner-seconds across eight workflow runs. These are resource times, not
billing amounts. [Issue #134](https://github.com/sh4869221b/azerlay/issues/134),
[Issue #136](https://github.com/sh4869221b/azerlay/issues/136) and
[PR #141](https://github.com/sh4869221b/azerlay/pull/141) retain the result record.

The owner then selected [normal Nix CI with daily/pre-release full Arch](../../ci-adoption.md).
Archived `workflows/ci-ubuntu.yml` and `workflows/ci-nix.yml` preserve the exact
comparison recipes, outside the executable Actions directory. Historical
`ci_compare_profiles.py` intentionally keeps its seven-plus-four gate contract;
current normal `ci_measure.py` defaults to six gates. Do not mix these cohorts.
