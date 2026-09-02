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
