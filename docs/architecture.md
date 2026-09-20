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
each descriptor's event clock to `CLOCK_MONOTONIC` before starting readers; any
open or clock failure closes the acquired descriptors.

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
not persist into later reports. Axis normalization belongs to later processing.

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
