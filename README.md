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
profile for the session, report status, and stop the application. `run` starts
the application in foreground, coordinates duplicate starts, and safely recovers
stale control sockets.
`doctor` reads configuration, the next-start profile selection, and live socket
status without changing state; unavailable backend checks remain warnings.

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

## Run and control CLI

Create a configuration file, for example `config.toml`, containing:

```toml
schema_version = 1
```

Start in a Wayland session with a valid `XDG_RUNTIME_DIR`:

```sh
azerlay run --config config.toml --foreground
```

Foreground execution is the default; omitting `--foreground` has the same effect.
Without `--config`, the [default configuration path](docs/config.md#file-location)
is used. Missing or invalid configuration is fatal; Azerlay does not create it.
Startup requires nonempty `WAYLAND_DISPLAY`; this checks the environment only,
not compositor connectivity or Layer Shell support.

`run` prints initial text status and stays running. With no saved profile, status
has `active_profile:null` and degraded reasons, and startup prints guidance to
import a supported profile and use `status` and `profiles select`. No device or
renderer backend is available yet. A second `run` against a responsive instance
prints its status and exits 0 without loading the second invocation's config.

From another terminal, inspect status and stop the application:

```sh
azerlay status --json
azerlay quit
```

`quit`, SIGINT, and SIGTERM stop the foreground process with exit 0 after cleanup.
Startup, runtime cleanup, or output failures exit 1; invalid arguments exit 2.
`run` supports `--config PATH` (also `--config=PATH`), `--foreground`, and
`--help`, but not `--json`.

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

## Doctor CLI

```sh
azerlay doctor
azerlay doctor --config config.toml --json
azerlay doctor --include-bindings --json
azerlay doctor --help
```

`--config=PATH` is also accepted. No positional arguments are allowed, and help
performs no checks. The configuration and imported profile are those that would
be used at the next start; a running process is optional. Live socket status is
reported separately, and a session-only profile selection does not replace the
configured profile in the report.

Text reports go to stdout; text syntax errors go to stderr. `--json` returns one
newline-terminated schema-versioned object, including for syntax errors, with
no stderr output when writing succeeds. See the
[doctor output contract](docs/design-research.md#174-doctor) for fields and codes.

| Exit | Meaning |
| --- | --- |
| `0` | No findings. |
| `1` | Warnings, including checks that are not implemented. |
| `2` | Diagnostic errors or invalid command arguments. |
| `3` | Internal diagnostic or output failure. |

A healthy current installation still returns `1`: library, Layer Shell, monitor,
device discovery, device/udev permissions, and evdev capability checks are not
implemented. The session check examines `WAYLAND_DISPLAY` only, not compositor
connectivity. If the default configuration is absent, `azerlay doctor --json`
returns `2` with `ERR_CONFIG_NOT_FOUND`; create a valid configuration containing
`schema_version = 1` before retrying.

Reports may include target paths. Labels and normalized bindings require this
invocation's `--include-bindings`; the persistent `diagnostics.include_bindings`
setting does not enable disclosure. Even with that flag, reports omit raw
exports, macros, profile IDs/names, storage hashes, and source-origin paths.
Doctor never repairs configuration or storage, creates a runtime lock, removes
a stale socket, or changes running visibility or selection.

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
