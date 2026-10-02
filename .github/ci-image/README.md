# Native CI image prototype (Issue #115)

This is a built candidate under evaluation, not yet adopted on the default
branch. `lock.json` records the private package, immutable digests, official base,
image sizes and package manifests. The final publisher is manual on `main` only;
PR CI has read-only package access. The one-off bootstrap trigger was removed.
The first publisher's API linkage assertion failed after both pushes: the REST
response omitted `repository`, while package settings confirmed the correct
source repository, inherited access and exactly one Actions repository. The
read-only recovery run verified source labels and recovered immutable manifests.
The publisher now checks the approved package ID and private visibility; before
manually rebuilding, verify that settings still show only the intended access.

The `native` target preserves the existing native-build/vulnerability/licenses
package set; `wayland` adds Sway, D-Bus, grim, and CJK/emoji fonts for `test`.
Neither target contains Go, analyzer tools, build caches, application sources,
credentials, or user profiles. Build context must be this directory only.
The source label enables repository linkage if publication is later approved.
No license for the entire image is asserted; each distribution package keeps its
own license metadata.

## Build and inspect (requires Docker)

Resolve the official `archlinux:base` Linux/amd64 digest and record it. Pass that
complete immutable reference as ARCH_BASE, for example using a shell variable:

```sh
test -n "$ARCH_BASE" # archlinux@sha256:<verified digest>, never a guessed digest
docker build --pull --platform linux/amd64 --build-arg ARCH_BASE="$ARCH_BASE" \
  --target native -t azerlay-ci:native .github/ci-image
docker build --pull --platform linux/amd64 --build-arg ARCH_BASE="$ARCH_BASE" \
  --target wayland -t azerlay-ci:wayland .github/ci-image
docker run --rm azerlay-ci:native cat /usr/local/share/azerlay-ci/packages.txt
docker run --rm azerlay-ci:wayland cat /usr/local/share/azerlay-ci/packages.txt
docker run --rm azerlay-ci:wayland bash -euc 'test -z "$(getcap /usr/bin/sway)"; pacman -Q sway dbus grim noto-fonts-cjk noto-fonts-emoji'
```

The base digest pins the starting filesystem, not the live Arch repository.
Each build performs a complete `pacman -Syu`; rebuilt output is expected to get
new digests. Record the final registry digest, complete package manifest, base
digest, source commit, and build run together. Never run a partial package
upgrade in consuming CI. The installed package manifest already participates
in the existing strict native build-cache key; also include the chosen image
digest when switching consumers so even a same-package image rebuild invalidates
native build results. Do not add a broad restore prefix. Module caches stay
independent of the image and native-package boundary.

## Approved publication boundary

Approved destination: private `ghcr.io/sh4869221b/azerlay-ci`, package ID
15536413, source-linked to `sh4869221b/azerlay`, inheriting its private access.
The 2026-10-02 settings verification showed only `azerlay` under Actions access.
The source repository receives GitHub's automatic package administration role;
individual jobs are restricted by their explicit GITHUB_TOKEN scopes.

Only the manual publisher has `contents: read, packages: write`; the four
consuming jobs have `contents: read, packages: read` and ephemeral GHCR container
credentials. Other CI jobs retain only repository read access. No PAT, permanent
secret, public access, external repository access, PR publisher or
`pull_request_target` is added. Builds run only from `main` on workflow_dispatch.
A proposed image update requires reviewing manifests, paired measurements and
all quality gates before changing pinned digests in a separate PR. Dispatching a
build never updates consumers automatically.

Rebuild manually after native dependency changes and security updates. There is
no scheduled rebuild or automatic deletion. Review freshness during dependency
maintenance; if manual maintenance is not sustainable, revert to full pacman
setup. Retain the previous tested digests for rollback. Broader sharing,
scheduling, and destructive cleanup need separate authorization. Build/push and
read-only recovery evidence remain in the linked runs even when their short-lived
artifacts expire; selected immutable package manifests are committed here.

## Measurement and adoption gate

Use the current four-Arch-job baseline after #116, commit
`82e15f7bf1af814d91609972d1c39982d59dbd52`. The older five-job estimate is stale.
Run bounded paired baseline/candidate runs on fresh GitHub-hosted runners with
identical sources and comparable cache state. Include at least two warm-build
pairs with cold image pulls, plus a truly cache-free candidate quality-gate run.
Keep PR and main validation separate; a PR run does not prove main cache behavior.
No source-generation, dependency, or package perturbation from #116 probes may
silently enter the control group.

Record per job: container initialization (pull plus extraction), package setup,
cache restore/save bytes and duration, check durations, total runner seconds,
and the workflow elapsed critical path. Record image compressed bytes and
publisher build/push seconds separately. Compare the native/Wayland split with
a single large image only if transfer measurements justify that extra run.
Amortize publisher runner time over a stated number of CI runs; do not add
parallel-job savings to claim workflow elapsed savings. The hypothesized net
0–25 seconds/job may be zero or negative.

Do not drop or skip any current gate: formatting, vet, Staticcheck, race/shuffle
uncached tests, final build, CGo default assertion, GTK pkg-config, CLI version,
ELF inspection, vulnerability scan, license report, fuzz, or fixture generation.
Preserve non-root Wayland execution, writable cache ownership, removed Sway
capability, and Japanese/emoji fonts. An Ubuntu runner is not Ubuntu product
support.

Adopt only after measured benefit and exact-head quality gates. Pin both final
consumer digests in reviewable workflow YAML; do not consume mutable tags.
Rebuild after native-dependency changes and on an agreed maintenance cadence;
review manifests and retest before replacing pins. Retain the previous tested
digests for rollback. If freshness cannot be maintained, restore the existing
full pacman setup rather than leaving a frozen unmaintained image in use. Rollback
is a reviewed change of digest pins (or return to the original install steps),
not tag mutation. No automatic image deletion is part of this prototype.
