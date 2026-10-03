# Control protocol

`internal/control` provides a Linux Unix stream socket server and client. The
CLI controls the application started by `azerlay run`, which runs in foreground
with or without `--foreground`. Startup requires nonempty `WAYLAND_DISPLAY`;
an absent value fails with `ERR_RUNTIME_WAYLAND` at the `runtime` stage. This is
an environment check, not compositor connectivity or Layer Shell validation.

## Transport and security

The socket is `$XDG_RUNTIME_DIR/azerlay/control.sock`. `XDG_RUNTIME_DIR` must
be an absolute path to an existing directory owned by the effective user with
mode `0700`; there is no fallback. The server creates the `azerlay` directory
with mode `0700` and the socket with mode `0600`. Existing application directory
permissions and ownership must already match. Directory and socket checks
reject symlinks at the checked paths. Both client and server require Linux
`SO_PEERCRED` to identify the peer as the same effective UID; failure to obtain
credentials also rejects the connection. The client checks socket ownership,
type, and mode before connecting.

`run` holds an exclusive OS lock on the same-user, mode `0600` regular file
`$XDG_RUNTIME_DIR/azerlay/instance.lock` through startup and cleanup. The empty
lock file and private application directory remain after the lock is released;
the lock file is retained so subsequent starts use the same inode. A duplicate
`run` prints a responsive instance's text status and exits 0 without loading its
own configuration. If an owner is still starting and cannot answer, the second
invocation fails safely instead of starting another instance.

Only the exclusive owner may reclaim a stale socket: it must be a same-user,
mode `0600` socket whose connection attempt is refused, with unchanged file
identity and permissions immediately before removal. Live endpoints, timeouts,
malformed responses, unsafe paths, and replacements do not authorize removal.

Messages are UTF-8 JSON followed by LF, with at most 65,536 bytes before LF.
Each connection processes exactly one request and one response, then closes;
later lines are never dispatched. EOF without LF, invalid JSON, duplicate keys,
unknown fields, missing fields, wrong types, and multiple JSON values in a line
are rejected. An escaped newline inside a JSON string is ordinary content.
Responses have the same size limit; an oversized result is replaced with a
complete static error response before writing.

The server has a fixed five-second absolute deadline from accept. The client
has a five-second deadline covering the exchange, including dial, write, and
read. Receiving bytes does not extend either deadline. Unauthorized or timed-out
connections may close without a response. The client validates response version,
request ID, envelope, and method-specific result.

There is no reconnect or automatic retry. A lost response, timeout, or output
write failure does not prove that a mutation failed, and cannot roll it back.
Inspect status before deciding whether to repeat an operation, especially
`toggle`. The protocol exposes no arbitrary file, import, or shell operation.
Errors contain static diagnostics, never selectors, private paths, raw request
contents, or underlying causes.

## Requests and responses

Every request requires integer `version:1`, a nonempty string `id`, a known
`method`, and a `params` object. For example, each of these JSON lines ends in LF:

```json
{"version":1,"id":"42","method":"overlay.toggle","params":{}}
{"version":1,"id":"43","method":"profile.select","params":{"selector":"Bodycam"}}
```

Success and failure use all five response fields:

```json
{"version":1,"id":"42","ok":true,"result":{"visible":true},"error":null}
{"version":1,"id":"43","ok":false,"result":null,"error":{"code":"ERR_PROFILE_NOT_FOUND","stage":"selection","summary":"Profile was not found.","remediation":"Select an existing imported profile."}}
```

An error uses `id:null` when no validated request ID exists. If an oversized
response cannot fit even its error and ID, its replacement error also uses
`id:null`.

| Method | Params | Success result |
| --- | --- | --- |
| `overlay.show` | `{}` | `{"visible":true}` |
| `overlay.hide` | `{}` | `{"visible":false}` |
| `overlay.toggle` | `{}` | `{"visible":boolean}` |
| `config.reload` | `{}` | `{"accepted":true,"request_generation":uint64}` |
| `status` | `{}` | Status object described below |
| `profile.select` | `{"selector":nonempty string}` | `{"active_profile":ProfileStatus,"generation":uint64}` |
| `quit` | `{}` | `{"quitting":true}` |

Visibility initially is false and represents requested state. `overlay.show`,
`overlay.hide`, and `overlay.toggle` return that accepted request; the GTK owner
applies it asynchronously on the locked main OS thread. The response does not
mean that the window has mapped. `config.reload` accepts work into the existing
configuration manager and returns its request generation before parsing
finishes. It does not promise successful application. Status exposes the
eventual result while failed loads retain the last good configuration. Watcher
and explicit reloads share the same manager.

`quit` attempts its response before stopping the control server, even if writing
that response fails. Context cancellation and `Server.Close()` also stop
accepting, unblock connections, and join server goroutines. Close is idempotent.
For `run`, quit, SIGINT, and SIGTERM stop control acceptance and join handlers,
then cancel and join the configuration manager and watcher, remove the owned
socket, and finally release the instance lock. `Server.Done()` closes after
configuration cleanup and socket removal; the caller then releases the lock.
Startup and output failures also clean up resources acquired by that invocation.
Cleanup never removes a replacement at the socket path.

Standalone `control.Start` callers still own their configuration manager and
other application resources. That API rejects occupied socket paths and does
not acquire the instance lock or reclaim stale sockets; its shutdown joins only
the control server before removing its owned socket and closing `Done()`.

## Active selection

The controller initializes once from configuration. A nonempty
`profile.selected_id` selects an exact unique imported ID and its failure is
sticky. Explicit `imported` otherwise uses the saved import and ordinal.
Explicit `local` uses the configured device/file ref or matching durable
last-good, without switching source. `auto` tries that local ref, matching
last-good, saved import, then the newest loadable single-profile import when
saved selection is unavailable. It skips multi-profile imports without a usable
saved ordinal.
See [configuration](config.md#profile) for root resolution and admission.
Failures add safe degraded reasons; status remains usable. Partial or
unsupported local updates retain the active definition. Restart can restore
matching last-good as degraded when the current source cannot be loaded.

`profile.select` matches the exact selector against normalized string IDs or
names in all stored imports. Matching both fields of one profile counts once;
matching distinct profiles, including across sources, is ambiguous. There is no
ID-first priority or ordinal interpretation. An unreadable source causes a
storage error rather than an assumed unique match. A failed selection preserves
the current profile; a successful selection clears its prior initialization
failure.

In `auto`, a successful imported session selection stops and joins the local
watcher before publication, so pending local work cannot replace that selection.
A failed selection leaves local watching active. Explicit `local` rejects
imported session selection.

Selection is session-only and does not write configuration, catalog, caches, or
saved selection. Explicit and watcher-driven configuration reloads preserve the
active selection and initial source policy. Changed profile settings apply only
at the next controller initialization; an instance initialized with `local`
requires restart before it can select imports. `profiles show` still reports
persisted selection, while `status` reports the active session selection.

## Status schema 1

Status success means the server answered, including when degraded. Except for
the optional `overlay` object, all fields below are present:

| Field | Value and meaning |
| --- | --- |
| `schema_version` | `1`; independent of wire protocol and CLI report versions. |
| `uptime_seconds` | Monotonic elapsed seconds since controller initialization. |
| `visible` | Requested visibility, boolean; not proof that a surface is mapped. |
| `overlay` | Optional object with exactly `mapped` and `input_region_applied` boolean fields. Absent or `null` means no overlay backend has published state. |
| `active_profile` | `null` or `ProfileStatus`. |
| `device` | `null`; device connection is unavailable. |
| `event_nodes` | `[]`; no event nodes are monitored. |
| `event_rate`, `dropped_count`, `resync_count`, `render_rate` | `null`; metrics are unavailable. |
| `last_reload` | `{request_generation,config_generation,config_failure,watch_failure}`. |
| `generation` | Runtime counter, initially 1; increments only on a visibility or active-selection change. |
| `degraded_reasons` | Array of `{code,stage,reason}` diagnostics. |

`ProfileStatus` is
`{source:"imported"|"local",source_ref:string,profile_index:int,name:string|null}`.
Imported references retain the existing stored source identifier and one-based
ordinal. A local reference is slash-separated
`Storage/DevicesStorage/<device>/ProfileStorage/profile_<id>.json`, with ordinal
1 and no absolute userData root. This intentional selected-file reference may
contain its profile ID. Reports permit profile names but exclude other raw IDs, labels,
bindings, macros, origin paths, original exports, and configuration warning key
text. Text output quotes and escapes profile names.

`last_reload.request_generation` identifies the latest requested load;
`config_generation` identifies the committed configuration. Both are unsigned
64-bit counters separate from runtime `generation`. Each failure is `null` or a
safe `{code,stage,reason}` diagnostic. A retained configuration failure can
describe a previous attempt while a newer request is pending. A successful
parse clears the configuration failure, but not a watcher failure.

Degraded reasons always include `DEVICE_UNAVAILABLE` at `device`,
`INPUT_METRICS_UNAVAILABLE` at `input`, and `RENDERER_UNAVAILABLE` at `renderer`.
Active-profile initialization, configuration, and watcher failures are included
when present. Overlay lifecycle failures use stage `overlay` and the static
codes `OVERLAY_MONITOR_UNAVAILABLE`, `ERR_OVERLAY_SURFACE`,
`ERR_OVERLAY_INPUT_REGION`, or `ERR_OVERLAY_PLACEMENT`. An unavailable or
ambiguous explicitly selected monitor keeps the request while the surface is
hidden. A later matching monitor/configuration lifecycle or another accepted
show request can recover the surface; a hide clears an obsolete missing-monitor
diagnostic. Unavailable metrics are not reported as zero measurements.

`overlay.input_region_applied` means Azerlay issued the empty GDK input-region
call for the currently mapped surface. GDK exposes no result from that call, so
this field is not compositor acknowledgement and does not itself prove pointer
pass-through. It is false when the surface is unmapped or absent. Status is a
snapshot and may briefly report a mapped surface before the input-region call
has been applied. The status decoder accepts older schema-1 responses without
`overlay`; older strict clients may reject the extension, so use matching
client/server versions.

## Errors and CLI reports

| Code | Stage | Meaning |
| --- | --- | --- |
| `ERR_CONTROL_REQUEST` | `protocol` | Invalid message shape, encoding, or framing. |
| `ERR_CONTROL_VERSION` | `protocol` | Unsupported protocol version. |
| `ERR_CONTROL_METHOD` | `protocol` | Unknown method. |
| `ERR_CONTROL_TOO_LARGE` | `protocol` | Message exceeds the size limit. |
| `ERR_CONTROL_TIMEOUT` | `transport` | Connection deadline exceeded. |
| `ERR_CONTROL_UNAVAILABLE` | `transport` | Instance unavailable, cancelled, or shutting down. |
| `ERR_CONTROL_PERMISSION` | `transport` | Socket access or peer identity rejected. |
| `ERR_CONTROL_RUNTIME` | `path` | Runtime/application directory unavailable or unsafe. |
| `ERR_PROFILE_NOT_FOUND` | `selection` | No matching imported profile. |
| `ERR_PROFILE_AMBIGUOUS` | `selection` | More than one matching profile. |
| `ERR_PROFILE_SOURCE_UNAVAILABLE` | `selection` | Initial profile source is unsupported. |
| `ERR_PROFILE_STORAGE` | `storage` | Imported storage cannot be read. |
| `ERR_PROFILE_LOCAL_UNSUPPORTED` | `profile` | Local schema/release/history is outside admission. |
| `ERR_PROFILE_LOCAL_READ` | `profile` | Selected local file cannot be read as a stable complete definition. |
| `ERR_PROFILE_LOCAL_WATCH` | `watch` | Local watching is unavailable or stopped; retained state remains usable. |
| `ERR_PROFILE_LOCAL_STATE` | `state` | Private durable local state cannot be read or published safely. |

`azerlay show|hide|toggle|reload|status|quit [--json]` takes no positional
arguments. `azerlay profiles select [--json] [--] <id|name>` takes exactly one
nonempty selector. Use `--` before a selector beginning with `-`. Flags may
precede or follow the selector before `--`. Every command supports `--help`
without connecting. Unknown or duplicate flags are syntax errors.

CLI JSON uses `{schema_version:1,command,ok,result,error}`. `command` is the CLI
name, including `"profiles select"`; `result` is the method result above. The
wire version and ID are not CLI report fields. Success has `error:null`;
failure has `result:null` and `{code,stage,summary,remediation}`. JSON reports go
to stdout with empty stderr. Text successes go to stdout and text errors to
stderr. Text status includes all unavailable capabilities and degraded reasons.

Successful operations exit 0, including degraded status and accepted reloads.
Remote, transport, or output failures exit 1. Invalid CLI arguments exit 2 with
`ERR_CLI_USAGE` at `usage`. Reporting failure leaves any completed operation in
effect and never triggers a retry.
