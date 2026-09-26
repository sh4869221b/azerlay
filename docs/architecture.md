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
Azeron evdev nodes
  -> read-only readers
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

`internal/device.OpenGroup` revalidates only the selected nodes and transfers
read-only descriptors to `internal/input.Start`. Matching uses fresh metadata
and the retained descriptor's identity; a caller-provided admission value does
not bypass verification. A qualifying partial group is allowed. Startup sets
each descriptor's event clock to `CLOCK_MONOTONIC` and reads `EVIOCGABS` for
its advertised ABS codes before starting readers. Minimum, maximum, flat, and
fuzz come from the retained verified descriptor; an open, clock, or axis-query
failure closes the acquired descriptors.

Each node has one reader, feeding one reducer through a cancellation-aware
unbuffered channel. The reader preserves that node's event order. Cross-node
order follows reducer receipt and depends on scheduling; timestamps do not
sort or synchronize nodes. A node's `SYN_REPORT` commits only its pending frame
and publishes combined committed state. Equal codes on different nodes remain
independent, and other nodes' pending values remain unpublished.

Keys preserve ordered press, release, and repeat transitions. Key and raw signed
ABS values persist per node; presence lookups distinguish unobserved values
from known releases or zeroes. No initial-state capture is performed. REL deltas
accumulate within the reporting frame, with legacy and high-resolution wheel
codes kept separate. `MSC_SCAN` stays in that frame's compact event list and is
not a physical-control identity. Transitions, REL deltas, and frame events do
not persist into later reports. Axis metadata is copied into immutable storage
at startup and refreshed on successful resync; startup metadata does not
establish an initial raw-axis observation.

`Session.Latest` reads an atomic pointer and returns an independently owned
scalar header over immutable storage; collection accessors return copies.
The connected initial snapshot has sequence zero and no observed state. Every
ordinary `SYN_REPORT`, including an empty report, increments sequence once. Publication
timestamps never decrease; frame events retain their own monotonic timestamps.
Caller-supplied device/profile generations stay fixed throughout the session.
Consumers must stop and join an old session before starting its replacement.

On `SYN_DROPPED`, the reader first sends an ordered loss notification. The
reducer clears only that node's pending frame, keys, and raw ABS values to
unknown, retaining its axis metadata and the other nodes' state and pending
frames. The session remains connected. The reader discards events through the
next `SYN_REPORT`, the rest of the already-read buffer, and queued records on
the retained nonblocking descriptor through EAGAIN before querying current
state. Repeated drop markers in this interval do not publish repeated loss.

Bounded `EVIOCGKEY` and `EVIOCGABS` queries recover advertised keys as known
pressed or released values, plus raw axes and current normalization metadata.
Only a complete successful query replaces the affected node's state in one
reducer publication. This is atomic publication, not a simultaneous kernel
sample across ioctls or nodes. Loss and replacement each increment sequence
once, preserve nondecreasing timestamps, and have no reporting node, frame
events, key transitions, or REL deltas. Old snapshots remain unchanged; normal
events resume after the replacement without replaying the discarded queue.

A fatal read or recovery-query error, or an invalid ordinary key value, stops
the whole selected group. Cancellation also stops the group, without a device
error. Shutdown closes owned descriptors, joins readers, discards pending
state, and publishes one disconnected snapshot with observed state and frame
data cleared, sequence incremented once, and generations and axis metadata
preserved. `Done` closes only after this work finishes; `Close` is idempotent
and waits for it. Read `Session.Err` only after `Done`.

### Managed selected-device lifecycle

`device.NewReconnectTarget(result, selected)` captures the selected hardware
identity and node slots from discovery metadata, including when read access is
denied. Its serial/port and ambiguity rules are defined in the
[reconnect selection contract](decisions/device-identity.md#reconnect-selection).
`input.StartManaged(ctx, target, generations)` keeps that target across
discovery, access, and read failures. Each attempt uses `target.Select` and
`target.Open`, then the ordinary session clock and axis setup. No initial-state
capture is added on reconnect. An invalid target is a synchronous startup error;
a valid target starts asynchronously and may initially be degraded with unknown
input and no diagnostic until the first attempt completes.

One supervisor owns session replacement and a cancellable 250ms retry timer,
used only while degraded. It waits for the old session's `Done` and resource
cleanup before starting a replacement. `Managed.Latest` returns a coherent
`ManagedSnapshot` containing a snapshot, connected/degraded/stopped state, and
a copied diagnostic when available. It reads the active session directly;
there is no snapshot-forwarding worker. A disconnected session cannot appear
connected while the supervisor is still handling its completion.

The first successful session uses the supplied Device generation. Each later
successful replacement increments it once; failed attempts do not. Profile
generation stays fixed, and input recovery does not own or mutate profiles or
their persisted selection. Snapshot sequence remains session-local, so
consumers must also use Device generation. Invalid ordinary input stops the
manager with `ERR_INPUT_EVENT`; availability and read failures remain degraded
and retry. `Managed.Close` cancels retries, closes and joins active work, and is
idempotent. `Done` closes after cleanup; `Err`, read afterward, retains terminal
and cleanup failures, including rollback close errors from earlier attempts.

These APIs are not yet wired into `run`, the controller, or GTK. Tests use
synthetic device metadata and ioctl responses with real OS pipes and fd cleanup
checks. They do not establish live-device acceptance or the two-second reconnect
target, which remains unmeasured.

### Axis normalization and stick projection

`Snapshot.NormalizedAbsolute` retains the raw signed value and exposes known
and valid flags separately. `NormalizeAxis` widens values to `float64` before
subtraction. With center `(minimum+maximum)/2`, each side uses its own span.
Values within the inclusive center +/- flat region map to zero; values outside
it use `(raw-center-flat)/(maximum-center-flat)` on the positive side and
`(raw-center+flat)/(center-minimum-flat)` on the negative side, clamped to
`[-1,1]`. Raw values are never clamped. Equal/reversed bounds, negative flat or
fuzz, or flat reaching either span are invalid. Fuzz remains metadata because
the kernel already filters it. Unobserved raw values or absent capabilities
do not become a guessed neutral value.

`ProjectStick` consumes a normalized `StickBinding` and an immutable snapshot
without parsing or I/O. The confirmed export determines Keyboard or Xbox mode;
the caller selects the control and explicit input sources. Xbox selects two
distinct axes on one node, never combines nodes, and requires both axes known
and valid. Eligible nodes can advertise identical ABS codes, so there is no
first-match selection. Keyboard uses the binding's canonical W/D/S/A directions
and explicit nodes per direction. Each segment keeps its own known/down flags;
the vector is known only after all four keys have been observed.

Coordinates are +X right and +Y down, a display convention rather than inferred
physical orientation. Direction is the unit vector, and intensity is the vector
length capped at one. Keyboard opposites cancel while pressed segments remain
visible; diagonals have unit direction and full intensity. Results preserve
mode, connection, sequence, and generations. Pending frames cannot affect them,
retained snapshots remain unchanged, and disconnected snapshots expose no live
known state. Unknown bindings are not sticks. Rendering and runtime device
wiring remain separate from this implemented library projection.

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
