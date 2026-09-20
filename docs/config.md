# Configuration

`internal/config` loads and validates schema-versioned TOML settings and watches
for changes. `azerlay run [--config PATH] [--foreground]` starts the configuration
manager and control server; CLI `reload` and `status` communicate with that
server. Accepted device, input, and overlay values describe configuration, not
implemented runtime backends. See the
[control protocol](control-protocol.md) for the runtime contract.

## File location

`run --config PATH` and the internal `ResolvePath`, `Load`, and `Start` APIs
accept an explicit path. A nonempty path takes precedence and is cleaned and
resolved to an absolute path; relative paths use the current working directory.
`Start` resolves the location once.

Without an explicit path, the location is `$XDG_CONFIG_HOME/azerlay/config.toml`
when `XDG_CONFIG_HOME` is absolute. If it is unset, empty, or relative, the
location is `$HOME/.config/azerlay/config.toml`, provided `HOME` is absolute.
If neither base is usable, resolution fails with `ERR_CONFIG_INVALID` at the
`path` stage. No directories or configuration files are created automatically.

A missing file or ancestor returns `ERR_CONFIG_NOT_FOUND`, for both explicit
and default paths. This includes a missing parent encountered while preparing
the initial watch. Other read or watch setup failures return
`ERR_CONFIG_INVALID`. An initial failure returns no usable manager and closes
any resources opened for it. Starting a new `run` instance requires a valid file;
a missing saved profile is allowed and reported in status. A duplicate `run`
that receives an existing instance's status exits without loading configuration.

## Schema and defaults

Every file must explicitly contain integer `schema_version = 1`. Missing or
unsupported versions fail. The smallest valid file is:

```toml
schema_version = 1
```

Omitted sections and fields inherit the defaults below. Explicit `false`, zero,
and empty strings are preserved and validated as supplied; they do not select
defaults. TOML syntax errors, incorrect field types, unsupported enum values,
out-of-range numbers, and nonfinite floating-point values fail with
`ERR_CONFIG_INVALID`. Numeric ranges below include both endpoints. Enum strings
must match the listed spelling exactly.

### Device

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `device.model` | `"cyborg-ii"` | Device model; only `"cyborg-ii"`. |
| `device.hand` | `"left"` | Hand; `"left"` or `"right"`. |
| `device.serial` | `""` | Device serial identifier; an opaque string. |
| `device.auto_reconnect` | `true` | Automatic reconnect setting; boolean. |

Both `device.hand` values are accepted configuration, not a hardware-support
claim. Formal v1 support is left-hand Cyborg II only; right-hand support is
deferred beyond v1.

### Input

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `input.source` | `"auto"` | Input source setting; `"auto"` or `"evdev"`. |
| `input.refresh_hz` | `60` | Refresh frequency; integer `30`, `60`, or `120`. |
| `input.show_ambiguous` | `true` | Ambiguous-input visibility setting; boolean. |

### Profile

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `profile.source` | `"auto"` | Profile source setting; `"auto"`, `"imported"`, or `"local"`. |
| `profile.selected_id` | `""` | Selected profile identifier; an opaque string. |
| `profile.game` | `"bodycam"` | Game identifier; an opaque string. |
| `profile.watch` | `true` | Future profile-source watch setting; boolean. |

The loader does not resolve `selected_id` through the saved import catalog or
change the saved selection. On controller initialization, `source = "auto"` or
`"imported"` uses saved imports. A nonempty `selected_id` must match exactly one
normalized profile ID across those sources. When `selected_id` is empty, the
controller uses the saved source and one-based profile ordinal. Missing,
ambiguous, or corrupt data
leaves no active profile and is reported as degraded status. `source = "local"`
is unavailable and never falls back to imported profiles.

`profiles select` changes only the active session selection. Neither explicit
nor watcher-driven configuration reload changes that selection or its initial
source policy. Changed profile settings take effect on the next controller
initialization; a controller initialized with `source = "local"` requires a
restart before imported selection becomes available. `profiles show` continues
to read the saved selection. `profile.watch = false` does not disable
configuration watching.

### Overlay

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `overlay.monitor` | `""` | Monitor identifier; an opaque string. Empty means no explicit monitor selection; loading does not detect a monitor. |
| `overlay.anchor` | `"bottom-right"` | Placement anchor; `"top-left"`, `"top"`, `"top-right"`, `"left"`, `"center"`, `"right"`, `"bottom-left"`, `"bottom"`, or `"bottom-right"`. |
| `overlay.margin_x` | `24` | Horizontal margin; integer from `-16384` to `16384`. |
| `overlay.margin_y` | `24` | Vertical margin; integer from `-16384` to `16384`. |
| `overlay.scale` | `1.0` | Overlay scale; finite number from `0.25` to `4`. |
| `overlay.opacity` | `0.88` | Opacity; finite number from `0` to `1`. |
| `overlay.mode` | `"normal"` | Display density; `"compact"`, `"normal"`, or `"detailed"`. |
| `overlay.show_profile_name` | `true` | Profile-name visibility setting; boolean. |
| `overlay.show_status` | `true` | Status visibility setting; boolean. |
| `overlay.show_unbound` | `false` | Unbound-control visibility setting; boolean. |

### Appearance

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `appearance.theme` | `"dark"` | Theme; `"dark"` or `"light"`. |
| `appearance.font_scale` | `1.0` | Font scale; finite number from `0.25` to `4`. |
| `appearance.high_contrast` | `false` | High-contrast setting; boolean. |

### Diagnostics

| Key | Default | Meaning and accepted values |
| --- | --- | --- |
| `diagnostics.log_level` | `"info"` | Log level; `"error"`, `"warn"`, `"info"`, `"debug"`, or `"trace"`. |
| `diagnostics.include_bindings` | `false` | Binding-diagnostics setting; boolean. Does not enable disclosure in `doctor`. |

`doctor` requires `--include-bindings` on each invocation before displaying the
selected profile's labels and normalized bindings. The stored setting alone
never opts the command into disclosure. Raw exports and macros remain excluded
even with that flag.

Unknown keys produce `WARN_CONFIG_UNKNOWN_KEY` warnings with line, column, and
key segments when the rest of the file is valid. They do not bypass schema or
known-field validation. The loader never rewrites the file, so unknown keys and
the original contents remain intact.

Error and warning display strings contain only their stable codes. Errors also
carry a safe stage and reason; underlying causes are available through
`Unwrap`. Manager status exposes safe diagnostics without underlying causes or
TOML excerpts. Structured warning key segments come from the supplied file.

## Watching and reloads

`Load` returns one complete validated candidate and its warnings. `Start(ctx,
explicitPath)` registers the parent directory before the initial read and
returns a manager after initial loading succeeds. The watch resolves symlinks
in the parent directory, filters events to the target configuration path, and
observes create, write, remove, and rename events. This detects an editor's
atomic file replacement as well as edits to the existing file.

Relevant events debounce for 200 ms after the latest event. Events received
during loading request another current load. There is at most one active load
and one latest pending request. Configuration watching operates independently
of `profile.watch`.

`Manager.RequestReload(ctx)` enters the same reload event loop and returns the
request generation once accepted, before parsing completes. CLI `reload`
reports this acceptance as `{accepted:true,request_generation:N}`. Use
`status.last_reload` (inside the JSON report's `result`) to inspect request and
committed configuration generations and failures. A retained failure can belong
to an earlier attempt while another request is pending; acceptance is not proof
of successful application.

Each reload validates a complete candidate before publishing it atomically with
its warnings. A missing or invalid file retains the last good configuration and
warnings in memory and records `Status.ConfigFailure`. Another valid edit
replaces them and clears that failure. No last-good file is persisted, no
settings are rewritten, and no profile selection is saved. A syntax error after
the quiet period waits for another file event; there is no polling or timed
retry.

`Snapshot()` returns owned configuration, warnings, and status values.
`Status.RequestGeneration` identifies the latest requested load;
`Status.ConfigGeneration` identifies the committed configuration. A newer
observed change makes both an older successful result and an older failure
ineligible to publish. `Changes()` is a capacity-one notification channel for
one runtime owner: notifications may coalesce, so the consumer reads
`Snapshot()` for the latest state.

Watcher errors or removal/rename of the parent directory retain configuration
and set the separate `Status.WatchFailure`. A successful file parse does not
clear a watch failure. Relocating or re-registering the watched directory tree
is not supported.

Cancelling the supplied context stops the manager, watcher, timer, and load work
without publishing a cancellation failure over retained state. `Changes()`
closes during shutdown, and `Done()` closes when shutdown has completed.
