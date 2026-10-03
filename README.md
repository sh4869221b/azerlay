# Azerlay

Wayland-native Azeron profile and live input overlay for Linux.

## Status

Bootstrap phase. The CLI can validate and save Azeron Software 2.0.2 profile
exports. A successful `import` saves the exact export and a one-based selected
profile ordinal; `profiles show` reads that saved selection in a later process.
`devices list` and `devices inspect` report qualified interface04 hidraw identity
and read-only access without consuming reports. Their JSON schema is version2.
The internal input library reads only USB `16d0:12f7:0111` and the qualified
vendor descriptor, identifies 30 physical buttons independently of output
assignments, and publishes immutable **last-observed** state. It never opens
evdev nodes or sends device protocol requests. Official Azeron Software running
in SOFTWARE mode is an explicit prerequisite; successful access does not prove
notification initialization.

Initial, disconnected, reopened and invalidated state is unknown. A valid report
establishes only the reported button's state. Detected counter discontinuity is
conservative loss suspicion, not continuity proof; an undetected final lost
release can leave stale state indefinitely. No timeout fabricates release.
Managed reconnect retains the selected serial or, without serial, the same USB
parent, while allowing hidraw renumbering. Input is not wired into `run`, the
controller or GTK. Live analog/output-key state and long/double/macro completion
remain unsupported. Static profile labels, assignments, stick configuration and
output candidates remain available. See [input recovery](docs/troubleshooting.md#internal-input-recovery)
for lifecycle limits and the opt-in owner-operated hardware test.

Normalization requires an explicitly
attributed Software 2.0.2 export and the closed predicates in
[binding conversion](docs/decisions/binding-conversion.md). Legacy numeric
conversion is outside v1 and tracked in
[Issue #102](https://github.com/sh4869221b/azerlay/issues/102).

An internal configuration package loads, validates, and watches TOML settings
with last-good reload preservation. A same-user control socket and CLI can
change requested visibility, request configuration reloads, select an active
profile for the session, report status, and stop the application. `run` starts
the application in foreground, coordinates duplicate starts, and safely recovers
stale control sockets.
It starts with a hidden GTK4 Layer Shell window and applies successful configuration
reloads on the locked main OS thread. `show`, `hide`, and `toggle` control the
live window; when mapped, its empty pointer input region lets pointer events
reach the application below while keyboard focus remains below. Status reports
requested visibility separately from whether the surface is mapped and whether
the empty input-region call was applied. The call is not compositor
acknowledgement. Profile rendering and device input remain unavailable.
`doctor` reads configuration, the next-start profile selection, and live socket
status without changing state; unavailable backend checks remain warnings.

The initial supported target is Linux/Wayland on x86-64 with left-hand Azeron
Cyborg II, Arch/CachyOS, Hyprland, and a Bodycam game profile. Niri and right-hand
Cyborg II are deferred beyond v1 and remain unverified. Ubuntu is excluded from
v1; implementation and qualification are tracked in
[Issue #98](https://github.com/sh4869221b/azerlay/issues/98).
The [placement decision](docs/decisions/compositor-placement.md) records
the measured Hyprland scope and a successful same-name headless return on the
tested host. The cause of an earlier compositor crash remains unresolved.

## Native installation

Local packaging targets Arch/CachyOS x86-64 and Hyprland. It packages the current
behavior described above; it does not connect input or profiles to the renderer.
GTK4, gtk4-layer-shell and their native shared-library dependencies must be
installed on the destination. The binary archive does not bundle them.
There is no published release assumed by these commands. The local smoke
version `0.0.0` and package recipe make no redistribution license grant;
the license and desktop application owner remain unresolved.

From a source checkout with the [development dependencies](#development):

```sh
archive_dir=$(mktemp -d)
scripts/package.sh 0.0.0 "$archive_dir"
```

This produces `azerlay-0.0.0-source.tar.gz` and
`azerlay-0.0.0-linux-x86_64.tar.gz`, each rooted at `azerlay-0.0.0/`.
The source archive includes current working files and embedded assets, without
Git metadata, `.omo`, or the output directory. Existing final archives are
refused. Other versions must be explicit numeric dotted versions, optionally
with a prerelease suffix.

### Arch package

Install the local build tools, then build as an ordinary user:

```sh
sudo pacman -S --needed base-devel go pkgconf gtk4 gtk4-layer-shell gobject-introspection pacman-contrib
build_dir=$(mktemp -d)
cp packaging/arch/PKGBUILD "$archive_dir/azerlay-0.0.0-source.tar.gz" "$build_dir/"
(cd "$build_dir" && updpkgsums && makepkg --verifysource && makepkg --cleanbuild)
sudo pacman -U "$build_dir/azerlay-0.0.0-1-x86_64.pkg.tar.zst"
azerlay version
```

`updpkgsums` replaces the clearly named checksum placeholder in the temporary
recipe with the normal makepkg source checksum. Keep the source archive beside
that recipe. Maintainers changing the version must update `pkgver` and the local
source before generating checksums. No remote release URL or `SKIP` is used.

The package installs `/usr/bin/azerlay`,
`/usr/lib/systemd/user/azerlay.service`, `/usr/lib/udev/rules.d/71-azerlay.rules`,
and documentation under `/usr/share/doc/azerlay/`. The unresolved desktop
template stays under `/usr/share/azerlay/integration/`, outside applications.
Installation neither enables nor starts the service.

### Manual binary archive

On a destination with the native runtime libraries installed:

```sh
sudo pacman -S --needed gtk4 gtk4-layer-shell libgirepository
tar -xzf "$archive_dir/azerlay-0.0.0-linux-x86_64.tar.gz" -C "$archive_dir"
install -Dm755 "$archive_dir/azerlay-0.0.0/bin/azerlay" "$HOME/.local/bin/azerlay"
"$HOME/.local/bin/azerlay" version
```

Add `$HOME/.local/bin` to the shell's PATH if needed. The archive's `integration/`
directory contains the permission rule, user-unit template and desktop template.
An administrator can install the permission rule explicitly:

```sh
sudo install -Dm644 "$archive_dir/azerlay-0.0.0/integration/71-azerlay.rules" /etc/udev/rules.d/71-azerlay.rules
sudo udevadm control --reload-rules
```

Reloading does not by itself reapply permissions to an existing node. Reconnect
the device and enumerate again, or follow the
[targeted permission checks](docs/troubleshooting.md#permission-checks-and-packaging-status).
Do not install `io.github.@OWNER@.azerlay.desktop.in` as a launcher; application
identity has not been finalized.

### Configuration and optional autostart

Create the default configuration explicitly, preserving an existing file:

```sh
case "${XDG_CONFIG_HOME:-}" in
  /*) config_home=$XDG_CONFIG_HOME ;;
  *) config_home=$HOME/.config ;;
esac
mkdir -p "$config_home/azerlay"
if [ ! -e "$config_home/azerlay/config.toml" ]; then
  (set -C; printf 'schema_version = 1\n' > "$config_home/azerlay/config.toml")
fi
```

A missing saved profile is allowed. Device input and profile rendering remain
unavailable in `run`; readable hardware does not initialize notifications.
The raw input library requires the official Azeron Software in SOFTWARE mode
and publishes last-observed state, with the loss limits described above.

For the manual archive, substitute the absolute binary directory into the unit:

```sh
mkdir -p "$config_home/systemd/user"
sed "s|@BINDIR@|$HOME/.local/bin|g" "$archive_dir/azerlay-0.0.0/integration/azerlay.service.in" > "$config_home/systemd/user/azerlay.service"
```

The installed unit's `ExecStart` must contain the actual absolute binary path,
not a literal `~` or `$HOME`. The Arch package already uses `/usr/bin`.
In the active Wayland terminal, confirm the session and target first:

```sh
printf 'WAYLAND_DISPLAY=%s\nXDG_RUNTIME_DIR=%s\n' "$WAYLAND_DISPLAY" "$XDG_RUNTIME_DIR"
loginctl show-session "$XDG_SESSION_ID" -p Active -p Seat
systemctl --user is-active graphical-session.target
```

A one-off service start requires a reachable display and a runtime directory
owned by the current user. An inactive `graphical-session.target` does not
prevent this manual start: `After` and `PartOf` do not require that target.
Import the variables actually set in the session, then start and inspect the
application:

```sh
systemctl --user import-environment WAYLAND_DISPLAY
if [ -n "${XDG_CURRENT_DESKTOP:-}" ]; then
  systemctl --user import-environment XDG_CURRENT_DESKTOP
fi
systemctl --user daemon-reload
systemctl --user start azerlay.service
azerlay status --json
azerlay quit
```

After normal quit the service becomes inactive and the control socket is
removed. To opt into future session startup, explicitly run
`systemctl --user enable azerlay.service` only when the compositor manages an
active `graphical-session.target`. The unit follows that graphical session
lifecycle; enabling it does not start the target.

If plain Hyprland does not manage this target, use its foreground autostart
alternative in your own Hyprland configuration, with an actual absolute path:

```ini
exec-once = /usr/bin/azerlay run --foreground
```

For a manual installation, replace `/usr/bin/azerlay` with the expanded absolute
`$HOME/.local/bin/azerlay` path. Do not combine both autostart routes or force
`graphical-session.target` active just for Azerlay.

### Removal

Remove any Hyprland `exec-once` entry you added. For the service route:

```sh
systemctl --user disable --now azerlay.service
```

Remove an Arch installation with `sudo pacman -R azerlay`. For a manual
installation, remove only the files you installed:

```sh
rm -- "$HOME/.local/bin/azerlay" "$config_home/systemd/user/azerlay.service"
sudo rm -- /etc/udev/rules.d/71-azerlay.rules
```

If you did not install a manual unit or rule, omit its removal command.
Then run `systemctl --user daemon-reload` and
`sudo udevadm control --reload-rules`; reconnect/re-enumerate or reapply to the
identified node as above. User configuration and saved imports are retained.
Removing an ACL does not invalidate descriptors that were already open.

## Development

Go 1.27.x, CGo, a C compiler, `pkgconf`, GTK4, gtk4-layer-shell, and
`gobject-introspection` development files are required. On Arch/CachyOS:

```sh
sudo pacman -S --needed go gcc pkgconf gtk4 gtk4-layer-shell gobject-introspection sway dbus
```

Sway and D-Bus provide the isolated headless test session. Run the launcher as a
non-root user; Sway is a test fixture, not a qualified product target. Hyprland
is the v1 compositor target; Niri is deferred beyond v1 and remains unverified.

```sh
scripts/test-wayland.sh go test -race -shuffle=on -count=1 ./...
CGO_ENABLED=1 go build -o /tmp/azerlay ./cmd/azerlay
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

## Game profile labels (internal library)

Game profiles can supply display labels without changing the actual Azeron
assignment. Put `.toml` files in `$XDG_CONFIG_HOME/azerlay/games/`. When
`XDG_CONFIG_HOME` is unset, empty, or relative, the directory is
`$HOME/.config/azerlay/games/`. The loader reads that directory without creating
it. For example, this is the tested `internal/gameprofile/testdata/sample.toml`:

```toml
schema_version = 1
id = "sample"
name = "Synthetic Example"

[bindings]
KEY_U = "Use"
KEY_P = "Pause"

[controls]
"input:15:single" = "Primary use"
```

`schema_version = 1`, `id`, and `name` are required. `locale` is optional and
does not select a translation automatically. `[bindings]` uses case-sensitive
`KEY_` or `BTN_` symbolic codes; a code in this file does not make an unsupported
export binding convertible. `[controls]` uses a positive decimal input ID and
the exact `single`, `long`, or `double` trigger. The filename does not set the
ID. Unknown fields, invalid types, and invalid keys reject the complete file.

A user file with the same ID as a built-in profile replaces that entire profile,
including its name, locale, and mappings. Duplicate IDs among user files are
invalid. Azerlay currently embeds only an empty `generic` profile; it includes
no Bodycam bindings. A missing game ID provides no game mappings.

For each normalized binding, a non-blank physical-control label wins first,
then a non-empty Azeron label, then a non-blank game binding label, then the
binding's human-readable assignment. Blank or whitespace-only game labels fall
through; an Azeron label containing only whitespace remains a literal label.
Game binding labels apply to single unmodified keys and Turbo codes, not to a
key inside a chord, multi-action binding, or macro. `BindingDisplay` remains a
separate description of the actual assignment, including for `Unknown` values.

The library supports explicit synchronous reload while retaining the last good
catalog after an invalid edit. `run` and GTK do not yet load or render these game
labels; `profile.game` in application configuration does not activate this
library at runtime.

## Device CLI

```sh
azerlay devices list
azerlay devices list --json
azerlay devices inspect
azerlay devices inspect /dev/hidrawN --json
azerlay devices inspect --help
```

Replace `/dev/hidrawN` with a current path from `devices list`. Hidraw paths
are temporary locators and can change after reconnecting. Without a path,
`inspect` includes relevant candidates and excluded siblings; with a path, it
examines only that node. An explicit path never bypasses device matching.

Support requires USB `16d0:12f7:0111`, interface04 and the qualified HID
report descriptor in the
[device identity decision](docs/decisions/device-identity.md#identity-and-grouping-contract).
Matching nodes are grouped by their current USB device parent. A qualifying
group remains visible when access is denied; a complete group means one
interface04 was admitted, independently of read access. Other releases and Cyborg variants are not qualified.

Exit `0` requires at least one admitted readable node and no error diagnostic;
exit `1` covers discovery, access, or output failure; invalid arguments exit
`2`. Warnings alone do not fail an otherwise successful scan. Text results and
warnings go to stdout, while errors go to stderr. `--json` emits one
newline-terminated schema-version-2 object on stdout, including for usage
errors, and retains partial results. See the
[device CLI contract](docs/design-research.md#1711-devices-implemented) for fields.

List output omits serial and detailed metadata. **Inspect output includes serial
values when available**, even without a path; review it before sharing. These
commands do not require a running instance or configuration and do not change
permissions. Doctor implements passive device checks; the input library is not yet wired
into the runtime overlay. See [device troubleshooting](docs/troubleshooting.md) for stable
diagnostic codes and the limits of access and uaccess packaging validation.

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
Startup requires a reachable Wayland display and Layer Shell support before it
prints readiness. Display failure reports `ERR_RUNTIME_WAYLAND`; missing Layer
Shell reports `ERR_LAYER_SHELL_UNAVAILABLE`. GTK is restricted to Wayland.

`run` prints initial text status and stays running. With no saved profile, status
has `active_profile:null` and degraded reasons, and startup prints guidance to
import a supported profile and use `status` and `profiles select`. No device
backend or live profile-to-renderer connection is available yet. The renderer
can display a supplied snapshot, as described in [overlay appearance](docs/overlay.md).
A second `run` against a responsive instance
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
`show`, `hide`, and `toggle` return the accepted requested visibility. GTK
applies that request asynchronously on its main thread, so the response does
not confirm that a surface has mapped. `status` keeps `visible` as the requested
value and includes an `overlay` object with the observed `mapped` and
`input_region_applied` booleans when the runtime overlay owner is available.
The latter means the empty input-region call was applied to the currently
mapped surface; it is not compositor acknowledgement or proof of pointer
routing. If the configured monitor is unavailable, the request remains true,
the surface stays hidden, and status reports an overlay diagnostic. Restore the
same monitor or hide the overlay, then check status again. Device input and
profile rendering remain unavailable. A successful status response can still
describe degraded state.

Use matching client and server versions for the extended status response.
Older strict clients may reject the new `overlay` field. Sway is used only as
an isolated test fixture; it is not a qualified product target. Click-through
and monitor recovery were exercised on the Hyprland v1 target; see
[overlay troubleshooting](docs/troubleshooting.md#overlay-click-through-and-monitor-recovery)
for the scope and remaining limits.

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

A healthy current installation still returns `1`: library, Layer Shell and
monitor checks are not implemented. Doctor schema version2 implements
`device-discovery`, `permissions` and `hidraw-capabilities` through passive
identity/access checks, without consuming reports. Missing devices and
unsupported descriptors are warnings; ambiguous selection, permission and
metadata failures are errors. Success does not confirm notification initialization.
Use the official software in SOFTWARE mode. The session check examines `WAYLAND_DISPLAY` only, not
compositor connectivity. If the default configuration is absent, `azerlay doctor --json`
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
- [Cyborg II device identity and permission decision](docs/decisions/device-identity.md)
- [Device and permission troubleshooting](docs/troubleshooting.md)
- [CI and synthetic fixture policy](docs/ci.md)
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
