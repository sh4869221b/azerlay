# Architecture Principles

This document summarizes the constraints that implementation work must preserve.
The complete normative design is in [design-research.md](design-research.md).

## Product boundary

Azerlay is a Linux/Wayland application delivered as one Go binary and one
process. Configuration, CLI commands, and a same-user Unix socket provide
control; the project does not introduce a Web UI, database, or separate daemon.

## Data flow

Profile data crosses a strict boundary before reaching device or UI code:

```text
Azeron source
  -> bounded strict decoder
  -> version-specific raw adapter
  -> normalized profile model
  -> profile and layout selection
```

Raw Azeron schema must not escape the adapter boundary. Corrupt compressed data
must not yield partially accepted or persisted profiles.

Input and presentation remain separated:

```text
Qualified Cyborg II interface04 hidraw
  -> read-only physical-report reader
  -> ordered reducer
  -> immutable input snapshot
  -> overlay projection
  -> GTK4 DrawingArea and Cairo
  -> gtk4-layer-shell
```

GTK objects stay on the locked main OS thread. Draw callbacks perform no I/O,
parsing, or waiting. The gtk4-layer-shell CGo bridge remains isolated at the
Layer Shell boundary.

### GTK window lifecycle

`internal/layershell` contains all application-owned CGo and exposes Go GTK/GDK
wrappers. The bridge initializes namespace `azerlay`, the overlay layer, keyboard
mode `none`, and exclusive zone `-1`. It replaces every anchor and margin on
placement updates. Surface acquisition retains a transfer-none GObject reference
through gotk4 without its dynamic pointer marshaler. The native load contract is
gtk4-layer-shell before the Wayland client library.

`internal/overlay` owns the window, monitor selection, and signal handlers on the
locked main OS thread. Explicit selectors match a unique connector first, then
a unique description; an absent or ambiguous selection stays hidden. Monitor
invalidation hides synchronously, and reconciliation recreates the window only
for the same resolved selection. Requested visibility is retained across this
lifecycle. Empty selection delegates the output choice to the compositor. The
owner applies an empty GDK input region after each surface maps and reapplies it
after placement changes. Hiding unmaps the window and clears the observed mapped
and input-region state.

`run` creates this owner hidden before readiness and runs the GLib main loop.
Background work posts main-context notifications; successful configuration
generations apply on that thread, while failed reloads retain the previous
placement. Shutdown joins control handlers and configuration work before
removing notification sources, disconnecting handlers, and destroying the GTK
window. CLI visibility requests are applied asynchronously by the GTK owner.
Status keeps requested visibility separate from observed mapping and reports
whether the empty-region call was applied. GDK provides no compositor
acknowledgement for that call. Profile rendering and device input remain
separate work.

### Reader and reducer contract

`internal/device` enumerates hidraw and resolves USB ancestry through sysfs.
Admission requires USB `16d0:12f7:0111`, interface `04` (`03/00/00`), and the
qualified 28-byte report descriptor: vendor usage page `ff01`, usage `0101`,
unnumbered 64-byte Input/Output reports. The single admitted interface has role
`physical-buttons`. `OpenGroup` uses `O_RDONLY|O_NONBLOCK|O_CLOEXEC`, checks the
retained descriptor's device number, kernel-held HID identity and report
descriptor, and rechecks sysfs after open. Setup failure closes acquired fds.
Discovery, diagnostics, input and reconnect never open evdev nodes.

`internal/input.Start` owns one reader and one ordered reducer. Vendor type57
is byte2, counter byte3, declared payload length byte6, and source/state bytes7/8.
Only 64-byte reports with length2, an evidenced source ID and state0/1 are
accepted. Other vendor types are ignored; padding is uninterpreted. The
validated embedded left-hand layout maps the 30 sources to regions and excludes
`stick.main`. No HID write, Feature, Output or GET_INPUT request is issued.

Snapshots are **last observed**, not authoritative current state. Initial and
reopened state is entirely unknown, with availability `unconfirmed`. A valid
report establishes only its own control's known/down state and availability
`observed`. Invalid physical reports clear all knowledge. Counter steps other
than an ordinary increment, including wrap/reset, conservatively invalidate
older controls before observing the current report. This is suspicion, never
proof of uninterrupted delivery. An undetected final lost release can leave
stale state indefinitely. Silence never produces a release or unavailable state.

`Session.Latest` returns an independent scalar header over immutable storage;
`Controls` returns a copy, and `LastObservation` returns the last report's typed
source/region/state by value. A publication increments the session-local
sequence. Caller-supplied device/profile generations stay fixed in that session.
Read failure/disconnect clears all controls and marks availability `unavailable`.
Cancellation closes the descriptor, joins the reader and publishes disconnected
unknown state before `Done`; `Close` is idempotent. Read `Err` only after `Done`.

### Managed selected-device lifecycle

`device.NewReconnectTarget` captures USB identity and optional serial. A known
serial must resolve uniquely; duplicates, including unreadable duplicates, are
ambiguous. Without a serial, only the original USB parent is eligible. This
same-parent rule cannot establish serial-level identity. Hidraw numbering may
change; there are no evdev interface slots or capability selectors.

`StartManaged` retains the target through absence, permission and read failures,
retrying every 250ms while degraded. Old work is closed and joined before a
replacement starts. The first successful session uses the supplied Device
generation; subsequent successful sessions increment it. Failed attempts do not.
Profile generation and profile/configuration last-good state are unchanged.
Reopen is unknown until individual valid reports arrive; no full-state recovery
is fabricated. Malformed reports invalidate knowledge within the current session.
Cleanup failures remain available through `Err`/`Close` even after a retry.

Official Azeron Software running in SOFTWARE mode is an explicit prerequisite.
A readable fd does not prove notifications have been initialized. These APIs
remain unconnected to `run`, the controller and GTK. Synthetic lifecycle tests
and the opt-in owner-operated hardware test have separate evidence; neither
silence nor metadata alone establishes successful hardware recovery.

### Analog and output state

Live analog, output-key state and long/double/macro firing or completion are
unsupported by this report contract. Static stick, trigger and assignment
metadata remain intact. `NormalizeAxis` remains a standalone mathematical
helper, but there is no axis reader, ioctl recovery or live stick projection.

### Physical matching and snapshot projection

`profileadapter` keeps version-specific source fields behind the normalized
model. `matching.Build` consumes a selected normalized profile and a validated
layout; it does not inspect opaque raw fields or input events. The caller must
provide the model, hand, and applicability explicitly. Mapping is supported
only for the evidenced left-hand Cyborg II Software 2.0.2 export, displayed
firmware 111, unknown hardware revision, and Keyboard-stick mode. A missing
source ID, invalid identity, conflicting present pin, or ID outside the layout
leaves the control unresolved. A proven ID can map when a pin is absent; source
array order and pins alone never establish a region.

The index retains every known candidate for a canonical output, including
unresolved controls and separate triggers. It exposes duplicate-control
ambiguity, unresolved candidate coverage, Unknown bindings, and exact Unbound
bindings separately. A single candidate is only one known possibility, not
proof of the physical source. An empty candidate list does not rule out an
Unknown binding's output.

`input.ProjectMatching` retains the static index and separately projects all 30
physical regions from the snapshot, independent of duplicate, unbound or unknown
output assignments. `PhysicalContext` explicitly supplies the researched
model/hand/applicability and SOFTWARE mode; USB identity cannot infer them.
Unsupported context and disconnection never produce known physical state.
Static output candidates have no live known/down fields. This library projection
is not wired into `run`, the controller or a renderer.

## Repository growth

Packages are added only when an issue introduces behavior that needs them.
The bootstrap does not create an empty copy of the proposed package tree.

## Non-negotiable behavior

- Do not recover exports by searching for the first `{`.
- Do not accept partial JSON from corrupt compressed input.
- Do not monitor general keyboards.
- Do not guess one physical control when several bindings are possible.
- Do not mutate Azeron configuration.
- Preserve the last-known-good state when reloads fail.
- Do not add unnecessary Web technology, databases, service splits, or
  abstraction layers.
