# Non-GUI CLI verification boundary

Issue [#137](https://github.com/sh4869221b/azerlay/issues/137) introduces one
behavior-bearing package, `internal/cli`. This is a Linux non-GUI verification
boundary, not a new supported operating system or a replacement production UI.

## Ownership

`internal/cli` owns the existing strict CLI grammar, help, version, profile
validation/import/storage commands, control client, device commands, doctor and
privacy-safe reports. It receives the version value and one synchronous
foreground-start callback. Only a valid non-help `run` request invokes that
callback; its config path, output writers and exit status are forwarded intact.
The core tests fail if a help, usage or other non-startup path invokes it.

`cmd/azerlay` retains `runtime.LockOSThread`, `os.Exit`, doctor/devices SIGPIPE
registration, Wayland admission, foreground signal handling, the live bridge,
GTK/GLib ownership, startup error classification and shutdown ordering. It calls
`cli.WriteRunResult` or `cli.WriteRunFailure` with a classified `cli.ReportError`.
Other report DTOs remain private. Native integration tests carry independent
wire-contract structs with the original JSON field types and null behavior.

There is no new fake runtime. The callback contract test checks forwarding only.
Production startup continues to use the real `runForeground` implementation.
`internal/live` remains native because its renderer dependency imports
Cairo/Pango; this change does not restructure renderer or native bindings.

## Test and package accounting

The baseline source is commit
[`4faedfba7b67f650907b48efefeb0fe54df190fe`](https://github.com/sh4869221b/azerlay/tree/4faedfba7b67f650907b48efefeb0fe54df190fe).
The [complete CLI test map](measurements/ci-60s/137-cli-test-map.csv) records all
86 existing top-level CLI tests: 43 move to `internal/cli`, and 43 stay in
`cmd/azerlay`. No existing test name is removed. The original
`TestDoctorOutputFailure/broken_pipe` subprocess case becomes the native
`TestCLIDoctorBrokenPipe`; its pipe, deadline, exit-code and stderr assertions
are unchanged. Its remaining in-process cases use isolated HOME/XDG/config
state without a compositor. Three additional core tests cover callback
forwarding, the supplied version and run-report output/failure handling.

There are 46 core CLI and 44 native CLI top-level tests after extraction,
excluding the unchanged native `TestMain`. The six fully moved test files
retain their bodies, and the split control/run tests retain every assertion.
The native `TestMain` still builds the executable from the current checkout;
all existing native process/IPC/GTK integration files remain in place.
Test helpers are local to each test package, not a new production testing API.

The explicit manifests assign all 19 ordinary root-module Go packages:

- [Core manifest](../scripts/ci-core-packages.txt): 14 packages
- [Native manifest](../scripts/ci-native-packages.txt): 5 packages

`scripts/ci_check_packages.py` rejects omissions, stale entries, duplicates,
overlap and wildcard entries. Full mode compares the exact disjoint union
against `go list ./...`. Both modes examine `go list -deps -test`, including
external test dependencies and test variants, and reject native application
packages or gotk4 anywhere in the core dependency closure. `--core-only` never
lists native packages, so it can run on a runner without GTK or Sway.

## Required validation commands

Use the repository's pinned Go/tool versions. All six existing job IDs remain.
The `test` job checks the exact root-package partition and dependency boundary,
then runs repository-wide formatting, vet and Staticcheck, the five native
packages under non-root two-output Sway with race/shuffle/count=1, and the binary
build. Formatting failures print `gofmt -d .` for review from hosted logs.

The Ubuntu `generated-files` job owns the 14 core packages. It validates/tests
CGO=0 separately from CGO=1 race/shuffle/count=1, without GTK/Sway setup. Its race
step uses a fresh `RUNNER_TEMP` GOCACHE and deletes it, so the existing Ubuntu
cache does not begin reusing native outputs without a complete ABI fingerprint.
Fixture regeneration and clean-diff checks remain afterward.

The initial duplicate native/core proof passed at head e54d3b32 (vulnerability
required a package-download retry). The partition now removes only the redundant
core race execution from the native job. Every root-module package still has a
required race run; core also has the additional CGO=0 contract run. The exact
manifest union rejects missing/duplicate packages, and failure in either job
fails the workflow. Package-install commands, cache inputs and native process
coverage are unchanged. The fresh core race cache remains measurable overhead;
this does not establish the revised p50 180-second / p95 300-second performance result.

These Bash commands also describe the individual verification units:

On a Linux runner without GTK/Sway, first prove the non-CGo path:

```bash
mapfile -t core < scripts/ci-core-packages.txt
CGO_ENABLED=0 python3 scripts/ci_check_packages.py --core-only
CGO_ENABLED=0 go test -shuffle=on -count=1 "${core[@]}"
```

Separately, with a C compiler for the race detector but no GTK/Sway:

```bash
(
  set -euo pipefail
  race_cache=$(mktemp -d "${RUNNER_TEMP:-/tmp}/azerlay-core-race.XXXXXX")
  trap 'rm -rf "$race_cache"' EXIT
  export GOCACHE="$race_cache" CGO_ENABLED=1
  mapfile -t core < scripts/ci-core-packages.txt
  python3 scripts/ci_check_packages.py --core-only
  go vet "${core[@]}"
  staticcheck "${core[@]}"
  go test -race -shuffle=on -count=1 "${core[@]}"
)
```

`-race` requires CGo; do not combine it with `CGO_ENABLED=0`.

On the existing native runner with production GTK/CGo and the non-root,
two-output Sway fixture:

```bash
mapfile -t native < scripts/ci-native-packages.txt
CGO_ENABLED=1 python3 scripts/ci_check_packages.py
CGO_ENABLED=1 go vet "${native[@]}"
CGO_ENABLED=1 staticcheck "${native[@]}"
CGO_ENABLED=1 scripts/test-wayland.sh go test -race -shuffle=on -count=1 "${native[@]}"
CGO_ENABLED=1 go build ./cmd/azerlay
```

Continue to check formatting repository-wide. Preserve the existing fuzz,
fixture-generation, native-build, vulnerability and license gates. In particular,
Go's `./...` excludes `testdata` tools and nested modules. `generated-files`
explicitly runs the boundary-fixture generator. Existing external-package
profileadapter integration tests remain; the standalone testdata consumer is
not an existing automated root-CI check. `internal/layershell/probe` has its own
`go.mod` and was already outside root-module CI. The consumer fixture and
research probe remain separately invoked tools, not omitted root packages.

For this partition, compare setup-inclusive core/native
wall time, compile work, aggregate runner-seconds and the complete workflow
critical path using [the measurement contract](ci.md#ci-wide-measurement-contract).
Removing the unconditional CLI `TestMain` from in-process tests establishes a
usable targeted verification unit; it does not demonstrate a whole-workflow
speedup. Native compilation and the checkout-built executable still have to run.

## Staging verification limits

The extraction was prepared in a cloud environment without a Go SDK, GTK,
Sway or a container engine. Source/test-name comparison, source import-closure
inspection, Python checker tests and whitespace checks can run there. They are
not compilation, `go list`, gofmt, vet, Staticcheck, race, compositor tests or
measurements. Hosted validation and before/after timing remain required before
claiming issue #137's acceptance criteria are complete.
