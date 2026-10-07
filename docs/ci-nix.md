# Pinned Nix native CI

The owner selected this environment for normal PR/main CI on 2026-10-07 after
[the bounded comparison](measurements/ci-136-comparison/README.md). Four native
roles retain the existing check names in `.github/workflows/ci.yml`; core and
fuzz remain required there. Full latest-Arch checks run daily and before version
publication through a manual verification entrypoint. See [policy](ci-adoption.md).
This does not claim reproducible binary bytes or new product platform support.

## Fixed inputs and source verification

- nixpkgs revision: `494ce7fd23ff6a5dff39e1fb11e9b6f2ac74bf25`
- upstream Git tree: `b1fecbc4c1d64b90e44ae463a20dab5c72413183`
- commit timestamp: `1791179034`
- source NAR hash: `sha256-Ni0LBydzaCi8oek12r4mVreuFHqYlM1eTD6SfiENKfw=`
- Nix installer/runtime: `2.35.2`, official x86_64 binary archive SHA-256
  `0c3960a9792331a22081c3c7a5d8465db9b17c50b3acdf18587fa4c6f2cb1158`

The lock's NAR hash was computed from the exact official GitHub source archive,
not copied from an unrelated flake or guessed. All archive files, executable
bits, symlink targets, and directory entries were independently rehashed as Git
objects; the reconstructed root tree matched the upstream commit's tree above.
The NAR serializer follows Nix's `src/libutil/archive.cc`; its regular-file and
directory encodings matched official fixture Git blobs
`1621b2982c808adc667b93071a08814e66f8b8c3` and
`3a9452f67fd7dc8b8c9328c767337c5c51b006c4` byte-for-byte. Nix itself still verifies
the committed NAR hash when fetching/evaluating the flake in CI. A mismatch must
fail; never auto-update the lock to bypass it.

Verified package definitions at this revision: Go 1.27.1, GTK 4.22.4,
gtk4-layer-shell 1.3.0, GLib 2.88.3, Cairo 1.18.4, Pango 1.57.1, Sway 1.12,
grim 1.5.0, AT-SPI 2.60.6 and D-Bus 1.16.2. Preparation validates these exact
versions before downloading the dependency closure. The Ubuntu candidate uses
GLib 2.88.0, Pango 1.57.0 and Sway 1.11;
results compare pinned environments, not an identical library/compiler closure.
Actual runtime versions and the complete evaluated closure are retained in CI.

`buildGoModule` at this pin defaults to Go 1.26. This experiment deliberately
uses a `go_1_27` devShell and persistent Go compilation caches. It does not use
`buildGoModule`, whose ordinary temporary GOCACHE would change the experiment.

## Roles and invocation

- `CI_NIX_ROLE=test`: `.#ci-test`, includes Sway, D-Bus, AT-SPI, grim, CJK and emoji fonts.
- `CI_NIX_ROLE=build`: `.#ci-build`, for native-build, vulnerability and licenses;
  excludes the additional compositor and font closure.

Set absolute job-local `CI_NIX_STATE` and `CI_NIX_OUTPUT` paths under RUNNER_TEMP,
plus the selected Go build/module cache paths. Preparation runs outside the shell:

```sh
python3 scripts/ci_time_command.py --output "$RUNNER_TEMP/nix-install-time.json" -- \
  bash scripts/ci-nix-install.sh
python3 scripts/ci_time_command.py --output "$RUNNER_TEMP/nix-prepare-time.json" -- \
  python3 scripts/ci_nix_prepare.py --role "$CI_NIX_ROLE" --output "$CI_NIX_OUTPUT"
nix develop --no-update-lock-file ".#ci-$CI_NIX_ROLE" --command \
  bash scripts/ci-nix-run.sh bash scripts/ci-nix-fingerprint.sh
```

Run every Go command through the same `ci-nix-run.sh` wrapper. Do not use
`actions/setup-go` in these native jobs: Nix owns the compiler. The wrapper
removes inherited GOROOT, uses GOTOOLCHAIN=local and GOENV=off, controls executable
PATH and font discovery, and selects the CGo loader from Nix's compiler wrapper.
It preserves stdenv CC/CXX and NIX compiler/linker flags. The runner and all
headless tests remain non-root. Existing `scripts/test-wayland.sh` retains the
same two outputs, pixman compositor and Cairo GTK renderer.

Font discovery stays limited to the pinned CJK and color-emoji directories.
The test role also includes the pinned fontconfig `10-scale-bitmap-fonts.conf`
rule. Color emoji uses bitmap strikes even though fontconfig marks it scalable;
without this standard rule, a requested 14px mixed Unicode line can inherit the
emoji strike's native height and overflow the renderer. No host font config is
included. A host-library Pango regression reproduces the original 129px height
and verifies the corrected 22px height; the actual Nix renderer gate remains the
authoritative check for the pinned environment.

### Accessibility and session isolation

GTK's Nix package does not itself supply the AT-SPI bus service. The test shell
explicitly includes the unmodified, pinned `at-spi2-core` package. The runtime
helper writes a job-local session-bus configuration and activation service. It
does not read host `/etc/dbus-1` configuration or activate host systemd services.
The AT-SPI service invokes nixpkgs' already-built hidden launcher executable:
the public wrapper at this pin prepends `/usr/bin` and would select host D-Bus.
The hidden path is checked and fails closed if this package layout changes.
No nixpkgs package is overridden or rebuilt to make this configuration.

The existing `dbus-run-session` test entry point receives the pinned daemon and
private config through a job-local shim. Before executing the test command it
calls `org.a11y.Bus.GetAddress`, then requires the registry's `Peer.Ping` on the
returned local accessibility bus. Both calls have bounded timeouts. A missing
service or failed activation fails the tests; accessibility is never suppressed.
The test environment explicitly uses `GTK_A11Y=atspi`, selects `dbus-daemon`,
removes inherited accessibility-bus addresses and bridge-disable settings, and
uses an in-memory GSettings backend in its private home directory.

This is a controlled development shell, not a sandboxed application derivation.
The host kernel and some inherited process environment remain outside the pin.
The installer uses the official archive with a checked hash, keeps root-only Nix
trust, and does not enable KVM, add substitute keys, or persist a GitHub token.
It runs only on fresh hosted CI; it is not a workstation installer.

## Binary substitution and caches

Preparation resolves the exact devShell input derivations, then requests them
with `--max-jobs 0 --builders ''`. Missing binary substitutes stop the experiment
rather than compiling GTK/Go/nixpkgs on the runner. Once dependencies exist, the
small project mkShell output is generated locally. Import-from-derivation is
disabled during evaluation so evaluation cannot silently compile dependencies
ahead of the substitution check. All dependency paths, closure
metadata and versions are retained. Capture preparation failure logs/artifacts
with `always()` and do not label a blocked profile as a speed result.

Use only the default signed `cache.nixos.org` in the first comparison. No Cachix
account or credentials are required. Each fresh hosted runner downloads its
needed closure; the first experiment's warm condition means restored Go caches,
not a fictitious already-populated Nix store. Record that distinction explicitly.
Do not cache all of `/nix/store` with a generic archive action. A targeted closure
cache can be a separately measured later change if transfer is the bottleneck.

Go module downloads, tool module downloads, normal compiled results and race
compiled results need distinct caches. Compile-cache keys must include the
complete shell closure fingerprint, role, job, Go/tool versions, go.mod/go.sum,
and the relevant scripts/workflow. Do not restore across Nix/Ubuntu/Arch or ABI
boundaries. No broad restore prefix should bypass those boundaries. Module
archives may share only when the existing path/compression/key contract matches.

## Historical comparison and acceptance

Keep the original Ubuntu quality gates and cache contract, adding only the
missing `at-spi2-core` runtime package and runner comparison metadata. Both candidates require
working accessibility services rather than suppressing GTK warnings.
The experimental workflow provides the four Nix-native roles; core/generated,
fuzz and Arch compatibility gates are shared only with verified results for the
same source head. Report both seven-gate profiles and the actual combined
11-job comparison cost. A subset passing is not a complete profile passing.

For the initial bounded comparison, run one cold and one warm paired attempt.
Create a fresh pull-request commit with the identical source tree for the warm
pair, so GitHub creates a new run eligible to save caches. Do not use a rerun of
the original run as the warm-cache validation. Record both commit IDs, tree IDs,
run IDs and actual restore hits, rather than assuming the second pair is warm.
Retain installer, closure preparation, module transfer, analyzer installation,
normal compilation, race tests, cache restore/save and whole-job elapsed times.
Use the Actions run/job/step timestamps to include setup and post-job work.
The Python timing wrapper reports wall time and waited-for child CPU/RSS; Nix
build daemon CPU is not necessarily charged to that wrapper, so do not call its
child CPU figure the total installer/build-daemon CPU cost.

Observe the existing warm median <=180s / p95 <=300s goals, but two observations
cannot establish those distribution statistics. Do not extrapolate an adoption
verdict or omit cold/download/post-cache costs. The owner chose Nix after reviewing those results. Retain the version and host
qualifications; adoption does not prove a p50/p95 target.

## Validation limits

The local Python regression suite checks exact lock identity, package-version
gates, derivation JSON formats, substitution-only command ordering and failure,
session configuration, accessibility preflight failures and command forwarding.
An optional host D-Bus smoke test checks the generated config when private
sockets are allowed; it is explicitly skipped in socket-restricted executors.
This is not a Nix runtime pass. Nix evaluation, official substitute availability,
the installer/daemon, the accessibility activation and native Go/Wayland gates
still require the hosted CI comparison. A blocked profile is not a timing result.

## References

- [Pinned nixpkgs](https://github.com/NixOS/nixpkgs/commit/494ce7fd23ff6a5dff39e1fb11e9b6f2ac74bf25)
- [Official Nix 2.35.2 installer](https://releases.nixos.org/nix/nix-2.35.2/install)
- [NAR implementation](https://github.com/NixOS/nix/blob/master/src/libutil/archive.cc)
- [Development shells](https://nix.dev/manual/nix/2.34/command-ref/new-cli/nix3-develop.html)
- [Nix trust and sandbox settings](https://nix.dev/manual/nix/2.34/command-ref/conf-file.html)
- [Pinned AT-SPI package and launcher wrapper](https://github.com/NixOS/nixpkgs/blob/494ce7fd23ff6a5dff39e1fb11e9b6f2ac74bf25/pkgs/by-name/at/at-spi2-core/package.nix)
- [AT-SPI 2.60.6 bus launcher](https://github.com/GNOME/at-spi2-core/blob/2.60.6/bus/at-spi-bus-launcher.c)
- [D-Bus session launcher options](https://dbus.freedesktop.org/doc/dbus-run-session.1.html)
- [Nix 2.35 derivation JSON format](https://nix.dev/manual/nix/2.35/protocols/json/derivation/)
- [Pinned fontconfig bitmap-scaling rule](https://github.com/fontconfig/fontconfig/blob/2.18.3/conf.d/10-scale-bitmap-fonts.conf)
