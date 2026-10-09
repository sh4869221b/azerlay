# Device and permission troubleshooting

## Start with doctor

Run the read-only diagnostic report first:

```sh
azerlay doctor --json
```

Exit `0` means no findings, `1` means warnings, `2` means a diagnostic or
argument error, and `3` means an internal diagnostic or output failure. A
healthy current installation can still return `1`: library, Layer Shell and
monitor checks are not implemented and appear as warnings. Doctor checks only
that `WAYLAND_DISPLAY` is set; it does not test compositor connectivity. Device
checks inspect identity and read-only access without consuming reports or
testing notification readiness. Use `azerlay status --json` for live runtime
state.

Doctor reads the configuration and profile that would be used at the next
start. A missing configuration returns `2` with `ERR_CONFIG_NOT_FOUND`; create
the documented `schema_version = 1` file before retrying. `--help` performs no
checks. `--include-bindings` displays labels and normalized bindings for that
invocation only. Device inspection and some reports can include serials or
paths. Review output before sharing it; do not send private exports, macro
contents, or indiscriminate logs.

## Local profile selection and recovery

Use `azerlay status --json` to inspect `active_profile` and `degraded_reasons`.
The selected definition is configured through the userData root, device and
final filename; see [configuration](config.md#profile). Profile settings apply
at the next start. Reloading configuration preserves the running selection.

`ERR_PROFILE_LOCAL_READ` means the selected file is absent, unreadable,
or changed during stable reading. After three bounded attempts, the watcher
waits for another relevant event. Repair the selected file or replace it at the
same filename, then inspect status again. An active last-good definition remains
selected; restart may restore matching private state as degraded.
`ERR_PROFILE_LOCAL_WATCH` means watching stopped or could not start. Correct the
directory/watch condition and restart; a load alone does not clear watch failure.
`ERR_PROFILE_LOCAL_STATE` means the private state destination is unsafe,
overlaps userData, or publication failed. Azerlay preserves prior state and does
not repair permissions or switch an explicit local selection to imports.

`ERR_PROFILE_LOCAL_UNSUPPORTED` means strict JSON validation or the closed
Software 2.0.2 saved-JSON schema/history predicates were not met. Incomplete JSON
also has this code; repairing the same selected file may recover on a new event.
For unsupported format/history, use the official Software export UI,
then the existing export route:

```sh
azerlay import --software-release 2.0.2 --profile-index N <export-file> --json
azerlay profiles show --json
```

Replace `N` with the intended one-based export ordinal. Set `source = "imported"`,
remove both `local_device` and `local_profile_file`, and restart to use the saved import. `auto`
can fall back initially to a saved import, or a newest loadable single-profile
import when saved selection is unavailable. An active local last-good remains sticky
after update failure. Azerlay never follows official UI active/favorite state,
opens its browser databases, or writes its store.

## Imported profile selection

Use `azerlay profiles show --json` to inspect the persisted imported selection.
If no usable selection exists, import an official Software 2.0.2 export with an
explicit one-based profile ordinal, then inspect the result:

```sh
azerlay import --software-release 2.0.2 --profile-index N <export-file> --json
azerlay profiles show --json
```

`ERR_PROFILE_NOT_FOUND` means the requested profile or saved source is absent;
choose an ordinal from a nonempty export or restore the configured source.
`ERR_PROFILE_AMBIGUOUS` means the requested imported ID or name is not unique;
select an exact unique imported ID or name. `ERR_PROFILE_SOURCE_UNAVAILABLE` means the
selected source cannot be resolved; correct `source` and its selectors in the
[profile configuration](config.md#profile), then restart. `ERR_PROFILE_STORAGE`
means stored profile data could not be read; preserve the original and inspect
the storage condition before replacing it. A `profiles select` acknowledgement
changes the running session selection; check `azerlay status --json` for the
resulting active profile. Persisted selection continues to come from
configuration and saved import state.

## Overlay click-through and monitor recovery

`show`, `hide`, and `toggle` report accepted requested visibility; GTK applies
the request asynchronously. Use `azerlay status --json` to compare `visible`
with `overlay.mapped` and `overlay.input_region_applied`. The last field means
the empty GDK input-region call was issued for the mapped surface. GDK does not
report compositor acknowledgement, so the field alone cannot prove that input
reaches another application. The raw-only runtime feeds last-observed physical
button state to the overlay and combines it with the selected imported or local
profile's static metadata. `devices list`, `devices inspect` and doctor only
check identity or access; they do not consume input reports. See
[input recovery](#internal-input-recovery) for the live-state limits.

If status contains `OVERLAY_MONITOR_UNAVAILABLE` at stage `overlay`, an
explicitly configured monitor is absent or ambiguous. The request is retained
while the surface remains unmapped. Restore the same selected output or hide
the overlay; status should then clear the diagnostic. Azerlay does not silently
move an explicit selection to another monitor. `ERR_OVERLAY_SURFACE`,
`ERR_OVERLAY_INPUT_REGION`, and `ERR_OVERLAY_PLACEMENT` describe other
recoverable overlay failures. After correcting the condition, issue `show` or
apply a successful monitor configuration and check status again.

### Verified on 2026-09-26

On Hyprland 0.56.2 with GTK 4.22.5 and gtk4-layer-shell 1.3.0, a test-only GTK
receiver overlapped the actual Azerlay-owned layer surface. With the production
empty region active, click, scroll, and motion events reached the lower
receiver. Repeating at the same overlap with a full input region stopped
pointer delivery; keyboard focus and a key event remained with the lower
window. Pointer and keyboard delivery also continued after hide/show remapping
and surface recreation. The production content-free surface separately mapped
at 200 by 200 pixels after `show`, appeared in `hyprctl layers -j`, and was
absent from the layer list after `hide`; status reported those mapping changes.

In a nested Hyprland headless-output test, removing the explicitly selected
output kept the `visible` request true, unmapped the surface, and reported
`OVERLAY_MONITOR_UNAVAILABLE`; returning an output with the same name restored
the mapping and applied the input region. A second run hid the overlay before
output return and stayed hidden afterward. This headless test required a
task-local GBM allocation `LD_PRELOAD` workaround in the nested compositor.
It qualifies that isolated run only, not generic unmodified-headless operation.
Physical output hotplug and remapping initiated independently by the
compositor remain unverified. Niri is outside the v1 target and unverified;
Sway provides an isolated test fixture only.

## Device discovery and permissions

Start with `azerlay devices list`. Use `azerlay devices inspect` to include
excluded candidates, or `azerlay devices inspect /dev/hidrawN --json` to
examine one current hidraw path. Inspect can disclose the device serial, name,
physical path, and sysfs path; review the output before sharing it. List omits
serial and detailed metadata. Neither command reads HID reports, saves host
metadata, changes permissions, or needs a running Azerlay instance.

## Matching and partial results

The supported signature is USB `16d0:12f7:0111`, with interface04 (`03/00/00`) and the qualified vendor report descriptor in the
[identity and grouping contract](decisions/device-identity.md#identity-and-grouping-contract).
A matching name or manually supplied path cannot substitute for those checks.
Other USB releases, products, interfaces, or unqualified report descriptors are
unsupported; unavailable or contradictory metadata is indeterminate. This does
not establish support for other Cyborg variants, hardware revisions, or firmware.

Nodes are grouped by their resolved USB device parent for this scan, not by
serial or hub. Hidraw numbers and parent paths are not persistent device IDs.
Re-enumerate after a reconnect before inspecting a path. Relative paths and
symlinks must resolve to an existing hidraw character device in sysfs;
regular files and other device types are not opened.

A complete group has exactly one admitted interface04, regardless of its access
status. Multiple matching interfaces are ambiguous. Inspecting one path examines
only that node. Automatic unsupported candidates are warnings; an explicitly
unsupported node is an error. Devices schema version2 uses `hidraw_paths` and
`report_descriptor`; it no longer contains `event_paths`, `input_id` or evdev
capabilities. No event path is opened, including a symlink to one.

## Diagnostics

| Code | Meaning and next step |
| --- | --- |
| `ERR_DEVICE_NOT_FOUND` | No qualifying node, or the requested path does not exist. Check the supported signature, connection, and current hidraw path. |
| `ERR_DEVICE_UNSUPPORTED` | Identity, interface, or report descriptor fall outside the researched contract. Check inspect metadata against the linked decision; a different name or path does not enable support. |
| `ERR_DEVICE_METADATA` | Required evidence could not be established, including malformed or contradictory metadata or failed identification queries. Check the connection and inspect the reported target. |
| `ERR_DEVICE_PERMISSION` | Metadata access or a qualifying node's read-only probe was denied. Follow the session and packaging checks below. |
| `ERR_DEVICE_DISCONNECTED` | A node disappeared during discovery. Reconnect and enumerate again. |
| `WARN_DEVICE_ROOT` | Access results describe root, not the ordinary user. Check access as the ordinary active-session user. This warning alone does not change the exit status. |
| `ERR_CLI_USAGE` | Invalid arguments. Use `azerlay devices --help` or the subcommand's `--help`. |

Text errors go to stderr; results and warnings go to stdout. JSON retains the
partial result and all diagnostics, with the first error also in `error`; use
`severity`, not the code prefix alone, to distinguish errors from warnings.
The [CLI contract](design-research.md#1711-devices-implemented) defines the schema.

## Internal input recovery

Runtime input is raw-only and `run` connects its observations to the overlay and
CLI status. Devices and doctor only check metadata and access;
they do not consume reports or initialize notifications.

Use Software 2.0.2, displayed firmware 111 and left-hand Cyborg II, and start the
official Azeron Software in SOFTWARE mode. These are documented operating
prerequisites; no caller confirmation or runtime mode detection is required.
Initial state and every
reopen are unknown. Each valid type57 report establishes the last observed state
of its own physical button. Malformed physical reports invalidate all knowledge;
counter discontinuity, including wrap/reset, conservatively invalidates older
knowledge. Counter semantics are not a continuity guarantee. An undetected final
lost release can leave stale state indefinitely. Silence is unconfirmed, never
proof of release or of a required initialization action.

Disconnect clears observations. `StartManaged` retries discovery/open every
250ms while degraded and retains the selected serial, or the same USB parent
when no serial exists. A duplicate known serial is ambiguous even if one node
is unreadable. `device.auto_reconnect=false` makes one admission/session attempt;
after failure or disconnect it stays unknown until a different device setting is
adopted. New sessions increment Device generation and retain Profile
generation. Only new reports recover individual observations. The library does
not issue state requests, monitor evdev or infer analog/output/trigger state.

| Code | Meaning and next step |
| --- | --- |
| `ERR_DEVICE_AMBIGUOUS` | Multiple matching devices or interfaces; resolve the selection rather than choosing the first. |
| `ERR_INPUT_READ` | Passive reading failed. Check the selected device and access; the manager retries. |
| `ERR_INPUT_EVENT` | Malformed physical report; state is unknown until later valid observations. |
| `ERR_INPUT_DROPPED` | Suspected counter discontinuity; older observations were invalidated. |

After USB reconnect, check whether notifications resume with the official
software still running. If needed, use the owner's normal software restart or
SOFTWARE-mode operation. No automatic recovery or two-second timing guarantee
is established by a readable fd. `Close` cancels retries, closes the fd and joins
all reader/session work before `Done`; cleanup failures are retained.

Synthetic tests cover mapping, unknown state, loss suspicion, same-device
reconnect, recovery after denied access or absence, generation retention and
joined session/fd cleanup. Owner-operated production verification
uses `AZERLAY_TEST_PHYSICAL_HID=1 go test -v ./internal/input -run
'^TestPhysicalHardwareLifecycle$' -count=1 -timeout=5m`: source15/16 press/release,
unplug/replug, observed endpoint-add-to-reopen timing, restored notifications
and close. Without the opt-in it skips;
a skip is not a hardware pass.

## Permission checks and packaging status

Run discovery as the ordinary user in the active local seat/session. If access
is denied, check that the session is active for the device's seat, then check
whether the specific supported-device uaccess packaging is installed and
applicable. Reconnect the device after the applicable session or packaging
conditions have been corrected, then enumerate again.

The earlier event-node rule is historical. Native packaging supplies
`71-azerlay.rules`, installed under `/usr/lib/udev/rules.d/` by the Arch package
or explicitly under `/etc/udev/rules.d/` for the manual archive. See
[installation](../README.md#native-installation) and the
[permission decision](decisions/device-identity.md#permission-contract).
Do not broaden device grants or alter node modes or group membership to work
around a session or rule problem.

After installation, reload rules and reconnect/re-enumerate the device. To
reapply only to an existing qualified interface04 node, replace the example
path with the current supported node reported by discovery:

```sh
node=/dev/hidrawN
azerlay devices inspect "$node"
syspath=$(udevadm info --query=path --name="$node")
sudo udevadm control --reload-rules
sudo udevadm trigger --action=change "/sys$syspath"
sudo udevadm settle
```

Inspect includes private device metadata; review it before sharing. Do not
trigger all devices. Check the active local session with
`loginctl show-session "$XDG_SESSION_ID" -p Active -p Seat` and inspect this
node's ACL with `getfacl "$node"`. An inactive or remote session is not an
active local-seat access qualification. A broad VID-only or `MODE="0666"` rule
can mask failure of this narrower rule; review existing administrator rules
without automatically deleting unrelated rules. Revocation denies fresh opens,
not descriptors already open before the ACL changed.

Expect `Active=yes`, a local `Seat=...`, and an ACL that allows the current
session user to open this qualified node. If the active user's ACL is absent,
check the targeted packaging rule and re-enumerate after the session or rule
condition is corrected. Do not grant access to unrelated devices or change
group membership.

A successful read-only open proves access for the invoking process only. It
does not prove that the specific uaccess rule is installed, that access is
least-privilege, or that seat ACL grant/revocation works. Discovery does not
install rules or repair access. Runtime input supplies the overlay; doctor implements passive
device/access checks. Successful discovery does not establish overlay readiness.

The packaged rule grants only hidraw USB `16d0:12f7:0111`, interface04, before
the platform's `73-seat-late.rules`. It imports `usb_id` when `ID_BUS` is absent
and matches USB-device attributes together with `ID_USB_INTERFACE_NUM`.
Event-node permissions are not required. Discovery
and doctor do not install or change rules, and uaccess does not enforce
read-only operations.
The historical host's broad `0666` grants do not validate this packaging scope.

## Configuration and session startup failures

`ERR_CONFIG_NOT_FOUND` means the explicit or default configuration is missing.
Follow the [default configuration setup](../README.md#configuration-and-optional-autostart)
to create `schema_version = 1` without replacing an existing configuration.
For `ERR_CONFIG_INVALID`, correct the existing file using the
[configuration contract](config.md); reinstalling the package does not repair it.
`azerlay reload --json` only acknowledges the request. Check `azerlay status
--json` afterward for `degraded_reasons` and the eventual configuration result.
An invalid reload retains the last-good settings; after fixing the same file,
request reload again and confirm the configuration failure clears. Profile
selection settings take effect at the next start, not through configuration
reload.

`ERR_RUNTIME_WAYLAND` means the display environment is missing or the display
cannot be reached. Start from the current user's active Wayland session, check
`XDG_RUNTIME_DIR`, and import that session's `WAYLAND_DISPLAY` into the user
manager. Import `XDG_CURRENT_DESKTOP` only when it is set. Do not invent a display
name or use X11 as a fallback. `ERR_LAYER_SHELL_UNAVAILABLE` requires a supported
Layer Shell compositor; Hyprland is the v1 target.

For a read-only session check, inspect the values and active seat from the same
terminal that will run Azerlay:

```sh
printf 'WAYLAND_DISPLAY=%s\nXDG_RUNTIME_DIR=%s\n' "$WAYLAND_DISPLAY" "$XDG_RUNTIME_DIR"
loginctl show-session "$XDG_SESSION_ID" -p Active -p Seat
```

Expect a nonempty display name, a runtime directory owned by the current user,
and an active local session. Doctor only checks that `WAYLAND_DISPLAY` is
nonempty; a successful doctor session check does not prove `run` can connect.
For Arch/CachyOS native-library startup failures, compare the loader message or
unit log with the GTK4 and gtk4-layer-shell packages in the
[installation instructions](../README.md#native-installation). Use
`systemctl --user status azerlay.service` to inspect the service state and any
recorded exit reason;
`pacman -Q gtk4 gtk4-layer-shell` shows whether those packages are installed.

The optional user service retries failures every two seconds. Stop it with
`systemctl --user stop azerlay.service` while correcting configuration or
environment, then start it explicitly again and inspect `azerlay status --json`.
Check `systemctl --user status azerlay.service` for a missing library or incorrect
`ExecStart` path; manual units require the actual absolute binary path.
If the compositor does not manage `graphical-session.target`, use the documented
Hyprland `exec-once` alternative instead of forcing the target active.

### Production lifecycle observation, 2026-09-27

The opt-in `TestPhysicalHardwareLifecycle` passed using the production reader:
initial unknown, source15/16 press and release, physical USB disconnect with
unknown state, reopened session with increased Device generation and
unknown state, source15/16 press and release again, then closed/joined reader.
The owner performed each operation. The official software stayed running;
notifications resumed after reconnect without restarting it. The run took
120.881 seconds including owner interaction; this is not a reconnect-latency
measurement or a continuous-current-state guarantee. Analog, other firmware,
right-hand hardware remain unqualified. The later permission observation below
qualifies the packaging rule separately from this input lifecycle test.

### Production reconnect measurement, 2026-10-03

The opt-in command above passed without the race detector on the qualified
left-hand Cyborg II: USB `16d0:12f7:0111`, interface04, Software 2.0.2,
displayed firmware 111, hardware revision unknown. The owner kept the official
software running in SOFTWARE mode and operated source15/16 press/release,
USB unplug/replug, and the same button sequence after reopening.

With udevadm 262, the KERNEL add receive-handler observation to
`ManagedConnected` publication interval was **361.030245ms**, within the
two-second criterion. The monitor was ready before unplug; one post-disconnect
add matched the freshly opened hidraw endpoint. The test used the printed
KERNEL timestamp and callback-entry `CLOCK_MONOTONIC` on the same host.
Kernel-to-monitor delivery and scheduling before that receive observation,
physical cable insertion, time unplugged and owner button response are outside
this interval. The full test took 85.677 seconds including owner interaction.

Initial and reopened snapshots were unknown. Device generation increased and
Profile generation stayed at 9; valid source15/16 notifications resumed after
reopening. Close joined the production reader, closed its descriptor, and
terminated/joined the monitor and scanner. These observations qualify this
setup and run; they do not establish a universal timing or notification
initialization guarantee, authoritative current state, or input integration
with `run`, the controller or GTK. Packaging ACL qualification remains the
separate observation below.

### Packaging access observation, 2026-10-03

In a disposable Arch guest with the supported USB device passed through,
`16d0:12f7:0111` interface04 had no ordinary-user access before the rule.
After applying `71-azerlay.rules`, the active local seat user gained a uaccess
ACL and a fresh read-only open succeeded. Switching to a second local session
removed the first user's ACL and denied a fresh open on the same still-present
device node; reactivating the first session restored both ACL and access.
The checks used zero-byte reads and did not consume input reports.

Actual udev traces showed no candidate grant for interfaces01/02/03, unrelated
hidraw, event nodes, or the whole USB device. A joystick event node already had
an active-user grant from Arch's standard `70-uaccess.rules`; that existing
platform grant is separate from Azerlay's hidraw-only rule. Other physical
products/releases/revisions remain untested. This isolated result does not rely
on the historical host's broad VID-only `0666` permissions and does not qualify
notifications, input runtime wiring, or authoritative current button state.

## Control socket and client failures

Control commands require the running instance's same-user socket. For
`ERR_CONTROL_RUNTIME`, check that `XDG_RUNTIME_DIR` is set to the current user's
private runtime directory. For `ERR_CONTROL_PERMISSION`, run the client as the
same user that owns the instance and runtime directory; do not broaden socket
permissions. `ERR_CONTROL_TIMEOUT` has an uncertain operation result: inspect
`azerlay status --json` before repeating a command.

| Diagnostic | Next check | Expected result |
| --- | --- | --- |
| `WARN_CONTROL_NOT_RUNNING`, `WARN_CONTROL_STALE`, or `ERR_CONTROL_UNAVAILABLE` | Check `azerlay status --json` and whether the intended foreground process or user service is running. Start the intended instance with `azerlay run --foreground` when none is running. | A responsive instance returns status. Azerlay performs its own stale-socket recovery at startup; do not remove a socket by hand. |
| `ERR_CONTROL_RUNTIME` | Print `XDG_RUNTIME_DIR` and inspect its owner and mode with `stat -c '%U %a %n' "$XDG_RUNTIME_DIR"`. | The directory exists, belongs to the current user and is private. |
| `ERR_CONTROL_PERMISSION` | Compare the account running the client with the instance owner; use the same user and runtime directory. | The control connection is permitted without changing socket modes. |
| `ERR_CONTROL_TIMEOUT` | Read `azerlay status --json` before retrying the timed-out action. | Status shows whether the requested state has already changed. |
| `ERR_CONTROL_VERSION` | Use a client and server that speak protocol version 1; use matching Azerlay builds for the extended status fields. | The control request is accepted and `status --json` includes the expected fields. |

Doctor reports an absent or stale socket as a warning, while unsafe runtime
paths, access failures and malformed communication are errors. A successful
`show`, `hide`, `toggle` or `reload` response acknowledges acceptance only;
inspect status for mapped state, configuration results and degraded reasons.
See the [control protocol](control-protocol.md) for its same-user boundary and
stable error contract.
