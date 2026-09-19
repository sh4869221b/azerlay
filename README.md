# Azerlay

Wayland-native Azeron profile and live input overlay for Linux.

## Status

Bootstrap phase. The CLI can validate and save Azeron Software 2.0.2 profile
exports. A successful `import` saves the exact export and a one-based selected
profile ordinal; `profiles show` reads that saved selection in a later process.
Device and overlay functionality is not implemented yet.

An internal configuration package loads, validates, and watches TOML settings
with last-good reload preservation. A same-user control socket and CLI can
change requested visibility, request configuration reloads, select an active
profile for the session, report status, and stop the control server.
The server currently starts through an internal API used by integration tests;
public `run`/startup, duplicate-instance handling, and stale-socket recovery
remain [issue #25](https://github.com/sh4869221b/azerlay/issues/25).

The initial supported target is Linux/Wayland on x86-64 with Azeron Cyborg II,
Niri or Hyprland, and a Bodycam game profile.

## Development

Go 1.27.x is required.

```sh
go test ./...
go run ./cmd/azerlay version
```

## Profile export CLI

Both commands require exactly one source: a file, `-` for stdin, or literal
`--text` input. Attribute the export with the exact supported Software release.

```sh
azerlay validate --software-release 2.0.2 --text '{"id":"fixture","inputs":[]}' --json
azerlay import --software-release 2.0.2 --profile-index 2 internal/profileadapter/testdata/bundle.input.json --json
cat internal/profileadapter/testdata/single.input.json | azerlay validate --software-release 2.0.2 - --json
```

`validate` lists every normalized profile and doesn't save state. `import`
selects its sole profile by default, but a multi-profile export requires
`--profile-index N`. Indices are one-based source ordinals, not Azeron IDs. A
successful import saves the export and selection before it reports success.
Read the saved selection without supplying the input, release, or selector:

```sh
azerlay profiles show --json
```

`profiles show` doesn't save or repair state. It reconstructs a missing,
corrupt, or incompatible normalized cache in memory from the saved original.
If the saved index or original is corrupt, it returns a storage error and never
selects another source.

Saved imports live below `$XDG_DATA_HOME/azerlay`. If `XDG_DATA_HOME` is unset,
empty, or relative, Azerlay uses `$HOME/.local/share/azerlay`. Application
directories use mode `0700`; originals, caches, index, and lock use `0600`.
The store retains the exact input bytes under their SHA-256 hash, an authoritative
catalog and selected ordinal, plus complete bundle and `pN.json` profile caches.
Identical bytes imported through any admitted route share one saved source;
byte-different inputs remain distinct.

Use `--json` for one schema-versioned JSON object. Successful operations exit
0. Invalid input, normalization failures, and missing or invalid selections
exit 1. Invalid CLI arguments exit 2. Unknown bindings produce
`WARN_IMPORT_UNKNOWN_BINDINGS` warnings but don't make a successful operation
fail.

Storage failures use `ERR_PROFILE_STORAGE` at the `storage` stage. An empty
saved store uses `ERR_PROFILE_NOT_FOUND` at the `selection` stage. `profiles
show` syntax failures use `ERR_CLI_USAGE` at the `usage` stage. If reporting an
already committed import fails, the command exits with that output failure but
the saved import remains available to `profiles show`.

Reports include deliberately permitted metadata: root kind, raw export version,
profile name, input count, selection, and Unknown warning counts. They don't
include IDs, labels, bindings, macros, unknown fields, or other private export
content. The raw export version is reported metadata. It isn't the
`--software-release` attribution and doesn't establish Software support.

## Control CLI

These commands require an already running control server:

```sh
azerlay show
azerlay hide
azerlay toggle
azerlay reload --json
azerlay status --json
azerlay profiles select -- "Bodycam"
azerlay quit
```

Every control command supports `--json` and `--help`; help does not connect.
`show`, `hide`, and `toggle` change requested visibility only: there is no
renderer yet. Status reports device, input, and rendering capabilities as
unavailable. A successful status response can still describe degraded state.

`reload` acknowledges acceptance; inspect `status` for the eventual configuration
result. `profiles select` requires an exact, unique imported profile ID or name.
Its session-only selection survives configuration reloads and leaves persisted
selection unchanged; `profiles show` continues to show the saved selection.
The next controller initialization uses configuration and saved selection again.

See the [control protocol](docs/control-protocol.md) for socket permissions,
framing, status fields, errors, and lifecycle boundaries.

## Project documents

- [System design and research](docs/design-research.md)
- [Architecture principles](docs/architecture.md)
- [Security principles](docs/security.md)
- [Configuration schema and reload behavior](docs/config.md)
- [Control protocol and CLI](docs/control-protocol.md)
- [GitHub roadmap](https://github.com/sh4869221b/azerlay/milestone/1)

## License

The project license is not finalized. MPL-2.0 is recommended by the design,
pending dependency-license, NOTICE, and SBOM review before a public release.
No final license grant is made by this repository at this stage.

## Trademark notice

Azerlay is an independent, unofficial project and is not affiliated with,
endorsed by, or sponsored by Azeron SIA. Azeron and Cyborg are trademarks
of their respective owner.
