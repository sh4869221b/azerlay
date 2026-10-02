# Issue #115: image experiment measurements

Status: candidate only; do not infer a workflow wall-time improvement. Final-head
warm repetition and the resulting recommendation are recorded in
[PR #123](https://github.com/sh4869221b/azerlay/pull/123).

## Environment and controls

The baseline is merged #116, `82e15f7bf1af814d91609972d1c39982d59dbd52`.
Candidate `b584292b69863fa1161751700a70a0386434425b` changes native setup,
registry permissions and the native cache boundary; all six jobs, check commands,
Go/analyzer versions, test flags, ownership transitions and timeouts are preserved.
An independent static comparison verified every non-setup step unchanged.

All image acquisitions below use fresh GitHub-hosted runners. “Warm” refers to
Go caches, not warm image pulls. The candidate's first run had cold native build
keys; it is not a fully cache-disabled run. Its warm run restored every expected
application/tool/native cache. Application/tool module keys stayed unchanged from
#116; native build keys differ by image digest as intended.

| Job | Baseline warm: init + pacman | Candidate cold-native run: init + verification | Candidate warm 1: init + verification | Baseline warm total | Candidate warm 1 total |
| --- | ---: | ---: | ---: | ---: | ---: |
| test | 42 s | 31 s | 50 s | 181 s | 184 s |
| native-build | 34 s | 30 s | 30 s | 73 s | 64 s |
| vulnerability | 32 s | 27 s | 28 s | 69 s | 59 s |
| licenses | 31 s | 28 s | 28 s | 66 s | 65 s |
| fuzz | — | — | — | 118 s | 112 s |
| generated-files | — | — | — | 23 s | 18 s |
| **Aggregate runner job seconds** | | | | **530 s** | **502 s** |

Baseline: [main run attempt 2](https://github.com/sh4869221b/azerlay/actions/runs/36985688702/attempts/2).
Candidate: [cold-native attempt 1](https://github.com/sh4869221b/azerlay/actions/runs/36987897494/attempts/1),
[warm attempt 2](https://github.com/sh4869221b/azerlay/actions/runs/36987897494/attempts/2).
All three passed all six jobs. Full per-step timings are in
[measurements.json](measurements.json).

The first warm comparison saves 28 aggregate runner seconds, but `test` is
3 seconds slower. Baseline job-start-to-finish span was 209 seconds because its
test job started later; candidate span was 185 seconds. That 24-second span
difference includes scheduler staggering and is not evidence of a 24-second
optimization. The untouched pure-Go jobs also vary. The candidate Wayland image
initialization varied from 31 to 50 seconds, so subtracting pacman time alone
would overstate the gain.

The cold-native candidate took 1,669 seconds in `test` and 3,357 aggregate runner
seconds; cold baseline took 1,771 and 3,443 respectively. Compilation variability
dominates those differences. They are not attributable image-setup savings.

## Transfer and amortization

`native`: 520,195,982 compressed image-layer bytes, 1,578,797,702 uncompressed.
`wayland`: 799,297,914 compressed bytes, 1,937,849,242 uncompressed. A separate
read-only recovery pulled native in 31 seconds and then Wayland in 14 seconds;
the latter reused native layers and is **not** a cold Wayland measurement.

The first publisher occupied 131 runner seconds, including build and push.
Its final REST linkage assertion failed after both successful pushes; the package
was verified private/source-linked through settings, then recovered read-only.
The assertion was corrected without changing package access. The diagnostic and
recovery runs are experiment overhead, not recurring rebuild estimates.
At the first warm sample's 28-second aggregate saving, 131 seconds takes about
five CI runs to amortize. This is an illustrative ratio, not a reliable recurring
saving: image pull, checks, cache operations and runner speeds varied. Cold native
cache migration and future refreshes must be considered separately.

## Adoption and remaining gates

No merge or automatic adoption is authorized by this experiment. A second warm
sample is needed before a recommendation. Private-image acquisition and all
quality gates are proven for this same-repository PR; image-based `main` execution
and fully cache-disabled image-based execution have **not** been run. Do not mark
those Issue #115 acceptance boxes complete. Avoid another expensive cold loop
unless a reproducible benefit justifies adoption and those final gates.

Keep the two image variants and rollback digests while evaluating. No scheduled
refresh, public sharing, PAT, automatic pin update or deletion is enabled. If the
critical-path benefit is not reproducible, leave the default workflow unchanged
rather than accepting maintenance and transfer costs for an assumed gain.
