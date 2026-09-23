# Overlay ABI decision evidence

## Decision for the Issue #32 bridge

Use the tested gotk4 `v0.4.1` modules (the `4` branch commit
`f90c213bbae79f6dcb74c5101a88b630f158de25`) with GTK4 `4.22.5` and
gtk4-layer-shell `1.3.0` as the proven v1 Hyprland ABI tuple. The probe was
built with Go `go1.27.1-X:nodwarf5 linux/amd64` on Linux
`7.3.0-rc3-1-cachyos-rc` and exercised on Hyprland `0.56.2`. Keep its
reproducible source and nested pin at `internal/layershell/probe/`; the root
module and CI acquire no GTK dependency from this decision. The ABI evidence
below records the build, direct link order, actual library load order, and
first mapped Layer Shell surface. Earlier GTK4 versions have not been tested.

The observed ownership contract is: lock the GTK main OS thread before GTK
work; keep GTK objects and their callbacks there; pass the `GtkWindow` native
address only into an immediate CGo call and keep the Go window wrapper alive
through that call. Obtain the current `GdkSurface` from
`GtkWidget.Native().Surface()` after realization, check absent or unexpected
values before use, and retain the Go surface wrapper rather than a native
pointer. Keep C types inside `internal/layershell`. The pointer/lifecycle
evidence below covers the guards and disconnection of stale surface handlers.

Set an empty Cairo region on the *actual mapped* `GdkSurface`, as indicated by
`GdkSurface.Mapped()` and `notify::mapped`; a widget `map` signal alone is not
the observed timing. On hide/show of the same surface, the handler remains
connected and reapplies the empty region when that surface maps again. On
unrealize or window replacement, disconnect the old handler; acquire the new
surface and connect its own handler before applying the region at its mapped
state. The six Hyprland counter rows below establish compositor-routed
pass-through for empty regions and blocking for full-region controls at
initial map, hide/show remap, and application-driven surface recreation.

Niri runtime verification was excluded from v1 by the owner and remains
unverified. Monitor hotplug and compositor-originated remap were not exercised;
the observed hide/show and application-driven recreation do not prove them.
This decision makes no claim about those cases or lower GTK4 versions.

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
| GREEN: full-reactive negative control | prefix + `--reactive` | Same three mapped stages, each reports `input-region=full-reactive (negative control)`; `lifecycle complete`; exit 0 | Pass for selecting reactive region; click blocking is tested separately in Todo 3 below |
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

The reviewed native boundary uses the pinned library wrappers for
surface/region ownership; no new dependency, production abstraction, or
test-only root package was added by Todo 2.

## Hyprland click-through evidence (Issue #15, Todo 3; 2026-09-23)

`--clicks` adds a lower, undecorated GTK window containing one expanding button
(`Lower clicks: N`) and accepts `count`, `full`, `empty`, `hide`, `show`,
`recreate`, and `quit` on stdin. The scanner sends strings through a channel;
only the locked GTK main thread handles widgets, regions, counters and lifecycle
commands. EOF/quit closes both windows; a five-minute deadline bounds the mode.
The overlay now contains a synthetic label, and region application requests a
redraw with `QueueDraw()` so the changed region participates in a subsequent
render/commit. Commands are observed asynchronously: wait for their printed
state and the next frame before clicking. `count` prints the current counter;
only the lower button's clicked signal increments it.

The fresh tuple checks were `go version` → `go1.27.1-X:nodwarf5 linux/amd64`,
`pkg-config --modversion gtk4 gtk4-layer-shell-0` → `4.22.5`, `1.3.0`,
`uname -r` → `7.3.0-rc3-1-cachyos-rc`, `hyprctl version` → Hyprland `0.56.2`,
and `pacman -Q ydotool` → `ydotool 1.0.4-2.1`. `ydotool --version` is not
supported by this executable and was not used as version evidence.

Build and start in a terminal with stdin kept open:

```sh
cd internal/layershell/probe
CGO_ENABLED=1 go build -o /tmp/azerlay-overlay-t3-probe .
env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=wayland-1 \
  GDK_BACKEND=wayland GSK_RENDERER=cairo G_DEBUG=fatal-warnings \
  /tmp/azerlay-overlay-t3-probe --clicks
```

Build exited 0, with the same nonfatal generated-wrapper `free` declaration
warnings as Todo 1. The live session reported actual `notify::mapped=true` and
`mapped input-region=empty` before controls were applied. The successful run
exited 0 after `quit`, without fatal GTK warnings.

In another terminal, use the running session's
`HYPRLAND_INSTANCE_SIGNATURE` and query geometry at **each** mapped phase:

```sh
hyprctl clients -j | jq '[.[] | select(.title == "Azerlay ABI lower target") | {at,size}]'
hyprctl layers -j | jq '[.. | objects | select(.namespace? == "azerlay-abi-probe") | {x,y,w,h}]'
```

All three queries returned lower `at=[10,50]`, `size=[1904,1024]` and overlay
`x=0,y=44,w=160,h=80`. Their overlap was `x=[10,160)`, `y=[50,124)`; its
midpoint `(85,87)` was used for every click. No placement policy or monitor
identifier/serial is implied. `hyprctl cursorpos` confirmed `85, 87`.

The planned legacy command `hyprctl dispatch movecursor 85 87` failed with
exit 7 and a Lua syntax error (`')' expected near '85'`), so no click was sent
by that `&&` chain. This installed compositor uses the documented
[Lua cursor dispatcher](https://wiki.hypr.land/configuring/core/dispatchers/).
The real compositor-routed command used for **each** table row was:

```sh
hyprctl dispatch 'hl.dsp.cursor.move({x=85,y=87})' && \
  YDOTOOL_SOCKET=/run/user/1000/.ydotool_socket ydotool click 0xC0
```

Both commands exited 0 on every successful row (`ok`, then `c0 110`). The socket
is the existing user's `$XDG_RUNTIME_DIR/.ydotool_socket`; no daemon was started
or stopped, and no permissions were changed. Window-directed synthetic clicks
were not used.

The first draft with an unpainted overlay and no redraw request failed its
negative control: `full` printed the requested mode, but the real lower counter
changed `0→1`; `empty` then changed `1→2`. That was **not** accepted as full
input-region evidence. Adding the visible label and redraw request, rebuilding,
and restarting from counter zero produced the complete results below. These
observations prove the corrected probe path; they do not isolate which of the
two render changes caused the earlier failure.

For each phase: enter `full`, then `count`; wait one second; run the click;
enter `count`, then `empty`; wait one second; run the same click; enter `count`.
Between phases, enter `hide`, wait for `notify::mapped=false`, then `show` and
wait for `hide-show-remap notify::mapped=true`. Next enter `recreate`; observe
old-handler disconnection, new `surface-acquired mapped=false`, and
`recreated notify::mapped=true`. Re-query geometry before each phase's clicks.

| Phase | Region | Lower before | Lower after | Binary observable / verdict |
| --- | --- | ---: | ---: | --- |
| Initial map | Full reactive | 0 | 0 | RED pass-through control: blocked; no lower clicked signal |
| Initial map | Empty | 0 | 1 | GREEN: exactly one lower clicked signal; pass-through |
| Hide/show remap | Full reactive | 1 | 1 | RED pass-through control: blocked; no lower clicked signal |
| Hide/show remap | Empty | 1 | 2 | GREEN: exactly one lower clicked signal; pass-through |
| Recreated surface | Full reactive | 2 | 2 | RED pass-through control: blocked; no lower clicked signal |
| Recreated surface | Empty | 2 | 3 | GREEN: exactly one lower clicked signal; pass-through |

The explicit `count` outputs establish the before/after values even when a click
is blocked. Every empty click additionally printed `lower count=1`, `2`, or `3`
from the actual clicked callback. `full`/`empty` commands also call the existing
observer to read and print current mapped state; those command-triggered lines
are not additional compositor map events.

| Additional scenario | Exact invocation | Observable result |
| --- | --- | --- |
| Invalid display | `env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=azerlay-probe-nonexistent GDK_BACKEND=wayland GSK_RENDERER=cairo G_DEBUG=fatal-warnings timeout 5s /tmp/azerlay-overlay-t3-probe --clicks` | `GTK initialization failed: Wayland display unavailable`; exit 1 before timeout, no crash or hang |
| Existing lifecycle regression | `env XDG_RUNTIME_DIR=/run/user/1000 WAYLAND_DISPLAY=wayland-1 GDK_BACKEND=wayland GSK_RENDERER=cairo G_DEBUG=fatal-warnings timeout 10s /tmp/azerlay-overlay-t3-probe --lifecycle` | All three empty mapped stages and `lifecycle complete`; exit 0 |
| Nested vet | `CGO_ENABLED=1 go vet ./...` in probe module | Exit 0; only existing generated C warnings |
| Root gates | `go vet ./... && go test -race -shuffle=on -count=1 ./... && go build -o /dev/null ./cmd/azerlay` | Exit 0; all test-bearing packages passed, profile has no test files |
| Formatting/scope | `gofmt -l internal/layershell/probe`; `git diff --check`; `git diff -- go.mod go.sum .github/workflows/ci.yml` | Exit 0, no output for each |
| Cleanup | `quit` on probe stdin; `rm /tmp/azerlay-overlay-t3-probe`; `test ! -e /tmp/azerlay-overlay-t3-probe` | Probe exit 0 after `recreated handler-disconnected`; binary removal and absence check exit 0 |
| No remaining test windows | Geometry queries above with `length` appended, selecting both probe titles for clients | Zero matching clients and zero matching layers |
| No remaining test process | `pgrep -af '^/tmp/azerlay-overlay-t3-probe'` | Exit 1, no matches; no child process was spawned by the probe |
| User daemon preserved | `pgrep -a -u 1000 ydotoold`; `stat -c '%U %F' /run/user/1000/.ydotool_socket` | Existing `/usr/bin/ydotoold` still running, socket still owned by `sh4869` |

This section is the captured Todo 3 evidence artifact. Only initial mapping,
application hide/show remapping and application-driven surface recreation were
exercised. Monitor hotplug and compositor-originated remapping remain unverified.
Niri was not run and remains excluded from v1 runtime verification.

## Evidence boundary and Issue #32 handoff

**Observed on the tested host:** The nested probe built and mapped with the
pinned tuple; `readelf` and `LD_DEBUG=libs` established the recorded link/load
order; an absent pre-realization surface and unavailable Wayland display failed
without a native dereference, crash, or timeout. The probe applied an empty
region at each actual mapped stage and disconnected the stale handler before
surface replacement. The six real Hyprland click results above establish only
the three tested lifecycle phases. A test-only omission of Layer Shell
initialization failed the smoke assertion; the corrected path passed.

**Upstream contracts:** The linked gotk4 README identifies branch `4` for
GTK 4.14. The linked gtk4-layer-shell guidance requires the Layer Shell
library to load before Wayland client. The
[GDK input-region API](https://docs.gtk.org/gdk4/method.Surface.set_input_region.html)
defines an empty region as nonreactive and `nil` as the full surface. These
sources explain the chosen calls; they do not establish behavior on untested
compositors or library versions.

**Implementation advice for #32:** Start from the probe's pinned tuple and
keep the same synchronous GTK-thread CGo boundary, surface guards, and
`notify::mapped` observer. Initialize Layer Shell before presenting the window;
reject unavailable display or Layer Shell support explicitly. Reapply the
empty region after a mapped hide/show and attach a fresh handler to each new
surface, disconnecting the old handler on unrealize or replacement. Preserve
the proven library load order when linking the production bridge. Verify
compositor-routed clicks for any newly supported lifecycle or environment;
the probe evidence alone does not cover monitor hotplug or compositor-originated
remap. This document selects an ABI contract; it does not implement #32.

## Issue #15 acceptance traceability

| Issue #15 checkbox | Direct evidence or decision | Verification |
| --- | --- | --- |
| `docs/decisions/overlay-abi.md is committed` | This decision document and the probe source are the deliverables. | Confirm that the delivered commit contains this version of the document in PR/repository history; a working-tree diff alone does not satisfy the checkbox. |
| Compatible library versions or commits are pinned | [Decision for the Issue #32 bridge](#decision-for-the-issue-32-bridge) and [ABI evidence](#abi-evidence-issue-15-todo-1-2026-09-23), including the nested `go.mod` pin and native tuple. | Evidence recorded for the tested tuple. |
| Native pointer lifetime and error behavior are documented | [Decision for the Issue #32 bridge](#decision-for-the-issue-32-bridge), [Pointer/lifecycle evidence](#pointerlifecycle-evidence-issue-15-todo-2-2026-09-23), and [Evidence boundary and Issue #32 handoff](#evidence-boundary-and-issue-32-handoff). | Documented for observed cases. |
| Empty input-region lifecycle timing is evidenced | [Pointer/lifecycle evidence](#pointerlifecycle-evidence-issue-15-todo-2-2026-09-23) and the six [Hyprland click-through rows](#hyprland-click-through-evidence-issue-15-todo-3-2026-09-23). | Evidenced for initial map, hide/show remap, and application-driven surface recreation on Hyprland. |
