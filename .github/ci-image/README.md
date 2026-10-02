# Native CI image prototype (Issue #115)

This is an unbuilt candidate, not the active CI environment. The CI workflow
continues to use `archlinux:base` and install packages normally. No registry,
publishing workflow, package permission, or schedule is enabled by these files.

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

## Required decision before publication

Recommended candidate destination: a private GHCR package owned by the repository
owner, linked to this private repository only. First confirm the selected name,
existing visibility/access, and approval to create/use it. Package inventory was
not accessible with the current repository credential. Do not assume a name is
unused or that an existing package is private.

Only a separately approved publisher needs `contents: read, packages: write`;
consuming jobs need `contents: read, packages: read` and GHCR container credentials
using their ephemeral repository GITHUB_TOKEN. Do not create or save a PAT or
widen other jobs. No public access, external repository access, automatic
publication from PR code, or `pull_request_target` is proposed. Use manual
rebuilds initially; any scheduled rebuild and retention deletion needs its own
approved operating policy. Never expose tokens in image layers or build args.

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
