# Overlay ABI decision evidence

## ABI evidence (Issue #15, Todo 1; 2026-09-23)

The standalone probe is a nested module at `internal/layershell/probe/`. It
uses the gotk4 `v0.4.1` tag, which resolves to branch `4` commit
`f90c213bbae79f6dcb74c5101a88b630f158de25`. Both the root gotk4 module
and its generated `pkg` module are pinned to `v0.4.1` in the nested module.
The alternate `4.16` candidate was not needed. The tested host tuple was Go
`go1.27.1-X:nodwarf5 linux/amd64`, GTK4 `4.22.5`, gtk4-layer-shell `1.3.0`,
Linux `7.3.0-rc3-1-cachyos-rc`, and Hyprland `0.56.2`. This does not establish
compatibility with earlier GTK4 releases. Niri runtime behavior is unverified
and excluded from v1.

| Scenario | Invocation | Observable result | Verdict |
| --- | --- | --- | --- |
| Native tuple | `pkg-config --modversion gtk4 gtk4-layer-shell-0` | `4.22.5`, `1.3.0` | Pass |
| Pinned module | `cd internal/layershell/probe && go list -m -json github.com/diamondburned/gotk4` | `Version: v0.4.1`; this tag resolves to the stated commit | Pass |
| Compile | `cd internal/layershell/probe && CGO_ENABLED=1 go build -o /tmp/azerlay-overlay-abi-probe .` | exit 0; generated gotk4 C wrappers emitted nonfatal `free` declaration warnings | Pass |
| Direct link order | `readelf -d /tmp/azerlay-overlay-abi-probe` | `NEEDED libgtk4-layer-shell.so.0` precedes `libgtk-4.so.1`; no direct `libwayland-client.so.0` entry | Pass |
| Actual load and first map | `env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=wayland-1 GDK_BACKEND=wayland GSK_RENDERER=cairo G_DEBUG=fatal-warnings LD_DEBUG=libs timeout 10s /tmp/azerlay-overlay-abi-probe --smoke` | `find library=libgtk4-layer-shell.so.0` precedes `find library=libwayland-client.so.0`; `layer-shell surface mapped`; exit 0 | Pass |
| Unavailable display | `env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=azerlay-probe-nonexistent GDK_BACKEND=wayland GSK_RENDERER=cairo timeout 5s /tmp/azerlay-overlay-abi-probe --smoke` | `GTK initialization failed: Wayland display unavailable`; exit 1 before timeout | Pass |
| Root dependency isolation | `git diff -- go.mod go.sum` from repository root | empty | Pass |
| Temporary binary cleanup | `rm /tmp/azerlay-overlay-abi-probe` after the checks | exit 0 | Pass |

The smoke assertion requires both a mapped GDK surface and a Layer Shell
window. A test-only mutation removed `C.init_layer(...)` without changing the
assertion: the rebuilt probe reported `mapped surface is not a Layer Shell
window` and exited 1. After restoring initialization and rebuilding, the probe
reported `layer-shell surface mapped` and exited 0. The mutation was not kept.
The CGo entry receives `glib.BaseObject(window).Native()` synchronously on the
locked GTK OS thread; it does not store the native pointer. This step does not
test pointer lifetime across remap or input-region click-through.

The [gotk4 branch guidance](https://github.com/diamondburned/gotk4/blob/f90c213bbae79f6dcb74c5101a88b630f158de25/README.md)
identifies branch `4` for GTK 4.14, while the
[gtk4-layer-shell linking guidance](https://github.com/wmww/gtk4-layer-shell/blob/e08cf8776969327849e1f28ad44d72bc038122c5/linking.md)
requires Layer Shell to load before Wayland client. The observations above
apply only to the tested host tuple.


## Pointer/lifecycle evidence (Issue #15, Todo 2; 2026-09-23)

The probe defaults to `--lifecycle`, a bounded automatic sequence: first map,
hide/show, destroy the old window, and create a new Layer Shell window. Its
`--reactive` mode runs the same sequence with a full input region as a negative
control; `--absent-surface` exits 1 before Layer Shell initialization. The
existing `--smoke` mode still stops after its first verified Layer Shell map.

Native access is synchronous on the locked GTK main OS thread.
`glib.BaseObject(window).Native()` exists only as the argument to an immediate
C call as `C.uintptr_t`, followed by `runtime.KeepAlive(window)`. The C helper
casts that argument to `GtkWindow *` synchronously. The probe retains Go wrappers
and signal handles, never native pointer addresses. The current surface is
obtained through `GtkWidget.Native()` and `GtkNative.Surface()` with explicit
absent-native, absent-surface, and unexpected-concrete-type guards. The pinned
binding returns `*gdk.Surface` on the tested host. Generated binding methods
keep their receivers alive across their C calls. Empty regions come from
`cairo.RegionCreate()` and are passed to `Surface.SetInputRegion`; only the
explicit reactive negative control passes nil, which means the full surface.

The surface observer connects `notify::mapped` during widget realization,
checks `GdkSurface.Mapped()`, and applies the selected region whenever the
actual surface becomes mapped. Hide/show leaves that observer connected.
Before destroying/replacing the window, and on unrealize, the observer is
disconnected; a newly realized window acquires its own surface and observer.
The lifecycle sequence requires the observed unmap before showing again and
requires successful region application at each of its three mapped stages.

All runtime commands below use this exact live Hyprland environment prefix:

```sh
env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=wayland-1 GDK_BACKEND=wayland GSK_RENDERER=cairo G_DEBUG=fatal-warnings timeout 10s /tmp/azerlay-overlay-t2-probe
```

Append the argument in the table to that prefix. Build and vet commands run
in `internal/layershell/probe`; root checks run at the repository root. This
section is the captured evidence artifact; no separate ignored log is needed.

| Scenario | Exact invocation or argument | Binary observable | Result |
| --- | --- | --- | --- |
| RED: original smoke-only probe | `CGO_ENABLED=1 go build -o /tmp/azerlay-overlay-t2-probe .`, then prefix + `--lifecycle` | Build exit 0; invocation prints `usage: overlay-abi-probe --smoke`, exit 2; lifecycle and region evidence absent | Expected failure before change |
| GREEN: updated probe compile | `CGO_ENABLED=1 go build -o /tmp/azerlay-overlay-t2-probe .` | Exit 0; existing generated C wrapper `free` declaration warnings remain nonfatal | Pass |
| GREEN: empty-region lifecycle | prefix + `--lifecycle` | Exact ordered transcript below; exit 0, no fatal GTK warning | Pass |
| GREEN: full-reactive negative control | prefix + `--reactive` | Same three mapped stages, each reports `input-region=full-reactive (negative control)`; `lifecycle complete`; exit 0 | Pass for selecting reactive region; click blocking is not yet tested |
| Missing native/surface failure | prefix + `--absent-surface` | `before-parent: native absent: non-success`, then `before-realize: surface absent: non-success`; exit 1, no C dereference of either absent value | Pass |
| Existing smoke mode | prefix + `--smoke` | Absence checks, `initial surface-acquired mapped=false`, `initial notify::mapped=true`, `initial mapped input-region=empty`, `layer-shell surface mapped`, `initial handler-disconnected`; exit 0 | Pass |

Captured stdout for the empty-region lifecycle:

```text
before-parent: native absent: non-success
before-realize: surface absent: non-success
initial surface-acquired mapped=false
initial notify::mapped=true
initial mapped input-region=empty
initial notify::mapped=false
hide-show-remap notify::mapped=true
hide-show-remap mapped input-region=empty
hide-show-remap handler-disconnected
recreated surface-acquired mapped=false
recreated notify::mapped=true
recreated mapped input-region=empty
lifecycle complete
recreated handler-disconnected
```

These observations establish the native lifetime and region-application path
on the stated host tuple. They do not establish actual compositor click-through:
no lower click target or real click was exercised in Todo 2. Monitor changes,
compositor-originated remaps, lower GTK versions, and Niri remain unverified.


Additional validation and cleanup for Todo 2:

| Scenario | Invocation | Observable result |
| --- | --- | --- |
| Nested vet regression and correction | `CGO_ENABLED=1 go vet ./...` inside the probe module | Initial Go-side `unsafe.Pointer` conversion produced `possible misuse of unsafe.Pointer` at both C calls, exit 1. Converting the immediate `uintptr_t` argument inside the C helper removed those diagnostics; final vet exit 0. |
| Final native regression after that correction | Rebuild command above, then the four runtime commands above | Build exit 0; lifecycle/reactive/smoke exit 0; absent-surface exit 1 as designed. Lifecycle transcript unchanged. |
| Root gates | `go vet ./... && go test -race -shuffle=on -count=1 ./... && go build -o /dev/null ./cmd/azerlay` | Exit 0; tests passed for cmd/azerlay, config, control, device, diagnostics, input, layout, profileadapter, profiledecode, profileraw, and profilesource; profile has no test files. No root source was changed by Todo 2. |
| Formatting and whitespace | `gofmt -l internal/layershell/probe`; `git diff --check` | Each exit 0, no output. |
| Root dependency isolation | `git diff -- go.mod go.sum` | Exit 0, empty. Nested module files also required no change in Todo 2. |
| Cleanup | `rm /tmp/azerlay-overlay-t2-probe` | Exit 0. Each bounded runtime invocation had already exited; cleanup disconnected its surface handler and destroyed its window. |

The probe remains one lifecycle test surface in one file (210 nonblank,
non-comment lines under the simple source-count check). The reviewed native
boundary uses the pinned library wrappers for surface/region ownership; no
new dependency, production abstraction, or test-only root package was added.
