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
not persist into later reports. Axis metadata is copied once into immutable
session storage; it does not establish an initial raw-axis observation.

`Session.Latest` reads an atomic pointer and returns an independently owned
scalar header over immutable storage; collection accessors return copies.
The connected initial snapshot has sequence zero and no observed state. Every
`SYN_REPORT`, including an empty report, increments sequence once. Publication
timestamps never decrease; frame events retain their own monotonic timestamps.
Caller-supplied device/profile generations stay fixed throughout the session.
Consumers must stop and join an old session before starting its replacement.

A fatal read error, invalid key value, or `SYN_DROPPED` stops the whole selected
group. Cancellation also stops the group, without a device error. Shutdown
closes owned descriptors, joins readers, discards pending state, and publishes
one disconnected snapshot with all state and frame data cleared, sequence
incremented once, and generations preserved. `Done` closes only after this
work finishes; `Close` is idempotent and waits for it. There is no resync or
reconnect. This package is not yet wired into `run`, controller, or GTK; its
pipeline tests use synthetic OS pipes and do not establish live-device acceptance.

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
