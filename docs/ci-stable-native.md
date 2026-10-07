# Stable native CI environment (migration candidate)

## Scope and status

This is Issue #136's Ubuntu 26.04 candidate, based on main `63e1c4da`.
It is not the rejected Arch prebuilt-image experiment (#115 / PR #123).
The hypothesis is fewer unrequested native ABI changes and cold-cache events,
not a promise that a larger image or APT is faster than pacman. No private image
is published, no registry credentials or expanded token permissions are needed,
and no analyzer results or prebuilt application binary replace current-checkout
checks. Package installation and all transfer costs still occur in each job.

The six existing gates keep their IDs and responsibilities. Four native jobs
use the stable environment; the GUI-free fuzz/core jobs retain an explicit
Ubuntu 24.04 host label and existing Go 1.27.1 setup. A seventh blocking
`arch-compatibility` job keeps rolling Arch native race/Wayland integration,
whole-package vet/Staticcheck and native build/version/ELF inspection on every
PR and main push. The core/native partition remains 14/5, every package still
receives race/shuffle/count=1 coverage, and the separate core CGO=0 run, all
seven 10-second fuzz allocations, reachable vulnerability scan, dependency
licenses including tests, and actual generated-fixture diff remain unchanged.

Consequently rolling Arch can still make total PR CI slow. This phase cannot
claim to eliminate all 30-minute cold runs. Changing Arch cadence or dropping
its blocking status is a separate decision, not hidden in this migration.
The target remains complete required-workflow warm/source-only p50 <= 180 s
and p95 <= 300 s. No such distribution has yet been established for this
candidate. A single cold and warm validation do not establish p50/p95.

## Reproducibility boundary

- AMD64 official base: `ubuntu:26.04@sha256:88a381d5b5eeb2b35d3ad70925a362c37ce569daf43ede89ff818ec20e4d3794`.
- Official archive snapshot: `20261001T000000Z`, all three pockets
  `resolute`, `resolute-updates`, `resolute-security`, components `main universe`.
- APT phased updates are explicitly included, so a runner machine ID cannot
  vary dependency resolution.
- Every native job replaces the default live sources, requires an error-free
  signed index update, checks committed SHA256 values for all three InRelease
  files, rejects non-snapshot index URLs, upgrades the pinned base to that snapshot, then resolves
  the complete dependency closure from those same indexes. Merely freezing a
  distro tag while running live APT would not meet this contract.
- Snapshot package candidates include GTK 4.22.4, GLib 2.88.0,
  gtk4-layer-shell 1.3.0 and Sway 1.11. Actual installed versions and all
  transitive packages are captured before any compiled-cache restore.
- Build roles install the compiler, linker, pkg-config, introspection headers,
  GTK and layer-shell headers/runtime. Integration additionally includes Sway,
  D-Bus, the real AT-SPI accessibility bus, grim, Noto CJK and color emoji fonts. The same existing non-root,
  two-headless-output/pixman Wayland launcher and GTK main-thread tests run.
- Go is exactly 1.27.1, and analyzer versions remain in `.github/ci-tools.env`.
  Vulnerability data remains live. GitHub runner kernel, action tags and external
  vulnerability/license services are not frozen; this is a native userspace
  compatibility boundary, not a claim of hermetic whole-workflow execution.

`scripts/ci-native-fingerprint.sh` fails closed on inventory/tool errors and
includes distro/architecture, every installed package/version/architecture,
compiler/linker/pkg-config output, GTK dependency flags, job identity and the
workflow/install recipe hash. Ubuntu additionally includes the source definition.
Go version and dependency/tool hashes remain in the cache action's key. Build
cache schema v2 prevents accidental reuse of previous fingerprints. There is
no broad restore prefix or PR-to-main promotion. Download-only module caches
remain separate and keep Go's checksum verification.

## Trust and bootstrap

The official minimal Ubuntu base has no CA bundle. Snapshot HTTP requests
redirect to HTTPS, so live APT bootstrap or disabling TLS verification would
be incorrect. Before checkout, a Node-based action downloads a fixed official
snapshot `ca-certificates` package and verifies its committed SHA256. Its
Mozilla certificate files form a temporary trust bundle for signed APT index
retrieval; APT installs the normal CA package from the same snapshot. The
initial download uses Node's normal certificate verification. Package hashes
and the Ubuntu archive keyring verify indexes and package downloads. No
`trusted=yes`, `--allow-unauthenticated`, TLS bypass or expiry override is used.

The authoritative base-image metadata and archive sources are public, so fork
PRs do not need a private registry login. Repository permissions remain
`contents: read`; normal checkout authentication is unchanged.

## Updates, security and rollback

Review the base digest and snapshot together at least monthly, and promptly
when an applicable native security update requires it. This is a maintenance
policy, not an added scheduled workflow. Keep snapshot dates at or after the
base build date; use the latest reviewed security/update pockets. Ubuntu's
snapshot service aims to retain at least two years, so do not leave old pins
unmaintained. A security update must never be silently delayed to preserve a
warm cache. Ubuntu-native vulnerabilities require separate archive/advisory
review; `govulncheck` covers Go, not all native libraries.

For an update PR:
1. Verify official AMD64 image metadata and the candidate snapshot's signed
   indexes. Update the digest, snapshot and CA bootstrap hash together where
   needed, reviewing changes in the complete installed package inventory.
2. Run all seven gates on the current checkout. A changed native fingerprint
   must miss compiled cache while compatible downloaded modules remain reusable.
3. Retain cold setup/pull/cache/compile/test/scan/cleanup timing and the first warm
   observation separately. Include failure count and aggregate runner-seconds.
4. Roll back the pin change if correctness regresses, except where reverting
   would reintroduce an unacceptable security exposure; then resolve the issue
   before adoption. Do not roll back by weakening tests or ABI fingerprints.

## Validation and measurement

Local shell/YAML and Python contract checks are useful but do not validate the
Ubuntu package resolver, gotk4 compilation, Sway behavior or hosted performance.
Those require the candidate's Actions run. Report passed, failed and unrun
stages separately. Initially use one candidate and necessary corrective runs,
then one same-head warm validation; no bulk benchmark campaign is implied.

Retain image initialization/pull and snapshot install durations from Actions
job steps, complete native fingerprints, exact native/core test diagnostics,
all seven job results, cache hit/miss and transfer bytes. Measure from the
first required job start through the last required job completion. The default
`scripts/ci_measure.py` gate set includes `arch-compatibility`, including its
setup and cleanup, and keeps queue-inclusive latency separate. A six-job
Ubuntu/core diagnostic subset is not the full workflow target. Historical
six-job cohorts remain explicitly separate via `--required`.

Adoption remains contingent on correctness, dependable bootstrap and practical
setup/transfer/maintenance overhead. If this phase increases costs without
reducing unexpected native churn, report that result; do not label it a
successful speedup merely because the environment is pinned.

## Primary references

- [Official Docker Ubuntu AMD64 image metadata](https://github.com/docker-library/repo-info/blob/584ae88613547cccf78d1a3b09f196b2888baa43/repos/ubuntu/remote/26.04.md)
- [Ubuntu Snapshot Service](https://snapshot.ubuntu.com/)
- [Ubuntu GTK package](https://packages.ubuntu.com/resolute/libgtk-4-dev)
- [Ubuntu layer-shell package](https://packages.ubuntu.com/resolute/libgtk4-layer-shell-dev)

## Initial hosted failure and correction

[First run 37572885744](https://github.com/sh4869221b/azerlay/actions/runs/37572885744)
at `8c141c29` completed six of seven gates successfully. Signed snapshot setup,
production GTK/CGo binary build/version/ELF inspection, scanners, core/fuzz and
rolling Arch integration passed. Ubuntu native tests failed because the minimal
no-recommends dependency set lacked the `org.a11y.Bus` service. GTK warnings
correctly failed the existing strict stderr/output assertions.

The correction explicitly installs `at-spi2-core` in the Ubuntu integration
role and checks its real D-Bus service file. Arch already receives it through
GTK dependencies. No assertions, warnings, accessibility behavior or native
checks are disabled. Nix's test role must expose the same real service.
This changes the native fingerprint, so corrective validation is a refresh run.

The failed run measured 1,705 seconds for all seven jobs and 5,295 runner-seconds;
Arch's cold native compilation was the last gate. The Ubuntu race wrapper took
695.15 seconds and returned 1. Package installation took 138–166 seconds in the
four Ubuntu jobs; the license role alone fetched 238 MB of packages in 96 seconds.
These are retained failure/setup observations, not a speedup or warm-budget claim.
