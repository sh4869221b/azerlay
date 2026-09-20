# Device and permission troubleshooting

Start with `azerlay devices list`. Use `azerlay devices inspect` to include
excluded candidates, or `azerlay devices inspect /dev/input/eventN --json` to
examine one current event path. Inspect can disclose the device serial, name,
physical path, and sysfs path; review the output before sharing it. List omits
serial and detailed metadata. Neither command reads input events, saves host
metadata, changes permissions, or needs a running Azerlay instance.

## Matching and partial results

The supported signature is USB `16d0:12f7:0111`, with the specific interfaces,
descriptor tuples, and minimum capabilities in the
[identity and grouping contract](decisions/device-identity.md#identity-and-grouping-contract).
A matching name or manually supplied path cannot substitute for those checks.
Other USB releases, products, interfaces, or missing required roles are
unsupported; unavailable or contradictory metadata is indeterminate. This does
not establish support for other Cyborg variants, hardware revisions, or firmware.

Nodes are grouped by their resolved USB device parent for this scan, not by
serial or hub. Event numbers and parent paths are not persistent device IDs.
Re-enumerate after a reconnect before inspecting a path. Relative paths and
symlinks must resolve to an existing input event character device in sysfs;
regular files and other device types are not opened.

A group can contain fewer than the three researched interfaces. `complete`
describes admitted interfaces, independently of their access status. Inspecting
one explicit path examines only that node, so it does not establish whether
its siblings are present. Unsupported automatic candidates produce warnings;
an explicitly requested unsupported node produces an error. An admitted readable
subset can succeed with warnings, but any candidate metadata or access error
returns exit `1`, preserving available results. No admitted node also returns
`1`. Invalid arguments return `2`; otherwise successful discovery returns `0`.

## Diagnostics

| Code | Meaning and next step |
| --- | --- |
| `ERR_DEVICE_NOT_FOUND` | No qualifying node, or the requested path does not exist. Check the supported signature, connection, and current event path. |
| `ERR_DEVICE_UNSUPPORTED` | Identity, interface, or required capabilities fall outside the researched contract. Check inspect metadata against the linked decision; a different name or path does not enable support. |
| `ERR_DEVICE_METADATA` | Required evidence could not be established, including malformed or contradictory metadata or failed identification queries. Check the connection and inspect the reported target. |
| `ERR_DEVICE_PERMISSION` | Metadata access or a qualifying node's read-only probe was denied. Follow the session and packaging checks below. |
| `ERR_DEVICE_DISCONNECTED` | A node disappeared during discovery. Reconnect and enumerate again. |
| `WARN_DEVICE_INCOMPLETE` | Expected interfaces are missing or excluded. Inspect the candidates and accompanying diagnostics; a qualifying subset is still usable for discovery. |
| `WARN_DEVICE_ROOT` | Access results describe root, not the ordinary user. Check access as the ordinary active-session user. This warning alone does not change the exit status. |
| `ERR_CLI_USAGE` | Invalid arguments. Use `azerlay devices --help` or the subcommand's `--help`. |

Text errors go to stderr; results and warnings go to stdout. JSON retains the
partial result and all diagnostics, with the first error also in `error`; use
`severity`, not the code prefix alone, to distinguish errors from warnings.
The [CLI contract](design-research.md#1711-devices-implemented) defines the schema.

## Permission checks and packaging status

Run discovery as the ordinary user in the active local seat/session. If access
is denied, check that the session is active for the device's seat, then check
whether the specific supported-device uaccess packaging is installed and
applicable. Reconnect the device after the applicable session or packaging
conditions have been corrected, then enumerate again.

The specific uaccess rule is a researched candidate, **not shipped or installed
by this discovery implementation**. Packaging and target-distribution rule
ordering, active-session grants, and revocation validation belong to
[#37](https://github.com/sh4869221b/azerlay/issues/37). See the
[permission decision](decisions/device-identity.md#permission-validation) for
what has and has not been validated. Do not broaden device grants or alter node
modes or group membership to work around this missing packaging.

A successful read-only open proves access for the invoking process only. It
does not prove that the specific uaccess rule is installed, that access is
least-privilege, or that seat ACL grant/revocation works. Discovery does not
install rules or repair access. Runtime input and the device checks within
`doctor` remain unimplemented; successful discovery does not establish overlay
readiness.
