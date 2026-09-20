# Cyborg II device identity

## Scope and status

This decision records the selected Cyborg II observation for
[#13](https://github.com/sh4869221b/azerlay/issues/13). The acquisition below is
complete for one connected device. The observed signature is ready for device
discovery implementation, with a syntax-validated permission candidate for
packaging. This does not establish support for all Cyborg II revisions,
handedness variants, firmware versions, or operating modes, and does not claim
that discovery or permission installation is implemented.

The observed USB device is called **D1**. Its three input event nodes are **N1**,
**N2**, and **N3**, named by interface below. These are document-local labels, not
stable paths or runtime identifiers. Serial values, machine-specific paths,
private profile labels, and input event streams are not reproduced.

## Evidence and limitations

Acquired on 2026-09-20 with Linux `7.3.0-rc3-1-cachyos-rc`, usbutils `019`,
udevadm `261`, and evtest `1.36`. The evtest input driver interface reported
`1.0.1` for all three nodes. Only one USB device with `16d0:12f7` was present.
The target was resolved anew through sysfs before opening any event node; all
three nodes had D1 as the same resolved USB device ancestor.

| Source / invocation | Disposition and admitted evidence | Limit or consequence |
| --- | --- | --- |
| `lsusb -d 16d0:12f7 -v` | Collected, exit 0; device/configuration/interface descriptors below; serial omitted before display | HID report descriptors were reported `UNAVAILABLE`; no report-descriptor or physical-control mapping claim is made. |
| `udevadm info --query=path --name="$node"` and `--attribute-walk` | Collected for N1–N3; resolved device ancestry, USB-device VID/PID/release and interface number/class/subclass/protocol agree with sysfs | USB hubs are ancestors too; their attributes are not D1's identity. |
| `udevadm info --query=property --name="$node"` | Collected; retained only device node, VID/PID, interface number, `ID_INPUT*`, `TAGS`, and `CURRENT_TAGS` | Input classifications and existing tags are observations, not the complete capability set or proof of a proposed permission rule. |
| Resolved `/sys/class/input/$event/device` and its `capabilities/*` files | Collected for N1–N3; exact capability bitmaps below | These describe this kernel's exposed input devices, not every firmware or mode. |
| Selected D1 blocks from `/proc/bus/input/devices` | Collected; input IDs, names, handlers, properties, and all reported capability bitmaps agree with sysfs | Host-specific physical paths and unique identifiers omitted. Handler numbers are transient. |
| `stdbuf -oL -eL evtest "$node"` | Collected for N1–N3; identification and named capability headers reached the `Testing ...` banner; each child was terminated and reaped | Bounded to 3 seconds per target, without `--grab`; only header fields retained, event lines discarded. This does not guarantee the subprocess read zero input events before termination. N1's repeat bit has the reporting difference noted below. |
| Owner statement and supplied settings screenshot, 2026-09-20 | Owner identifies the device as left-hand; screenshot displays Device `Cyborg II`, Current version `111`, and Software `2.0.2` | Firmware version evidence is the displayed current-version value, not an independent device query. This source is separate from the earlier binding-conversion evidence; no screenshot or private labels are copied here. |
| Hardware revision and right-hand coverage | Unknown revision; no right-hand specimen observed | No revision-wide or right-hand topology claim is admitted. |
| Operating mode | Screenshot shows `SOFTWARE` selected and a `Keyboard Joystick` label | These UI observations do not establish the Linux USB operating mode. No mode switch or alternative-mode acquisition was performed. |

The acquired USB `bcdDevice` value is `0111` (rendered `1.11` by lsusb).
Its numerical resemblance to the screenshot's Current version `111` is not
evidence that the USB release field encodes firmware or hardware revision.
No host permissions, udev rules, firmware, or modes were changed for acquisition.
Successful evtest opens establish current readability only.

## Observed topology

D1 reports VID `16d0`, PID `12f7`, manufacturer `Azeron LTD`, product
`Azeron Cyborg II`, and a present, nonempty serial. Its device
class/subclass/protocol are `00/00/00`, USB version is `2.00`, and configuration
`1` has five interfaces. lsusb and sysfs agree on the following descriptors;
interface values are hexadecimal, and every observed alternate setting is `0`.

| Interface | Class / subclass / protocol | USB interface string | Observed input event nodes |
| --- | --- | --- | --- |
| `00` | `ff/5d/01` | `Azeron Keypad - XInput` | None under this interface in the current sysfs topology |
| `01` | `03/00/00` | `Azeron Keypad - Keyboard` | N1 |
| `02` | `03/01/02` | `Azeron Keypad - Mouse` | N2 |
| `03` | `03/00/00` | `Azeron Keypad - DirectInput` | N3 |
| `04` | `03/00/00` | `Azeron Keypad - C0nf1gur4t0r` | None under this interface in the current sysfs topology |

All three input devices resolve through their respective USB interface to D1,
not merely to a shared hub. Five USB interfaces therefore do not imply five
event nodes. The absence of an event node for `00` or `04` does not establish
that the interface is inactive, safe to open through another subsystem, or
incapable of exposing input in another configuration.

| Node | Interface | evtest / proc input name | udev input flags (all `1`) | Existing `TAGS` and `CURRENT_TAGS` | proc handler kinds |
| --- | --- | --- | --- | --- | --- |
| N1 | `01` | `Azeron LTD Azeron Cyborg II Keyboard` | `ID_INPUT`, `ID_INPUT_KEY`, `ID_INPUT_KEYBOARD` | Both `:power-switch:` | `sysrq`, `kbd`, event, joystick |
| N2 | `02` | `Azeron LTD Azeron Cyborg II` | `ID_INPUT`, `ID_INPUT_MOUSE` | Neither property present | event, mouse |
| N3 | `03` | `Azeron LTD Azeron Cyborg II` | `ID_INPUT`, `ID_INPUT_JOYSTICK` | Both `:seat:uaccess:` | event, joystick |

All three nodes report input bus `0003`, vendor `16d0`, product `12f7`, and
version `0111` in proc and evtest. Each udev property set reports the same
VID/PID and its own `ID_USB_INTERFACE_NUM` from the table. All proc `PROP`
bitmaps are `0` and evtest lists no input properties.

The following hexadecimal bitmaps are the observed sysfs values; proc reports
the same nonzero families. Multiword values retain their printed order from
this 64-bit host. A zero is an observed empty capability family.

| Node | `ev` | `rel` | `abs` | `msc` | `sw`, `led`, `ff`, `snd` |
| --- | --- | --- | --- | --- | --- |
| N1 | `10001f` | `1040` | `101000301ff` | `10` | All `0` |
| N2 | `17` | `1943` | `0` | `10` | All `0` |
| N3 | `1b` | `0` | `30067` | `10` | All `0` |

Exact `key` bitmaps:

```text
N1: 73feff 0 0 4c3ffff17aff32d bfd4445600000000 c00000000000001 1130ffb8b17c007 ffe77bfad951dfff e0beffdf01cfffff fffffffffffffffe
N2: ff0000 0 0 0 0
N3: ffff 0 0 0 0 0 0 ffff00000000 0 0 0 0
```

evtest directly enumerated these capability roles and code sets:

| Node | Named event types in evtest header | Relevant enumerated codes |
| --- | --- | --- |
| N1 | `EV_SYN`, `EV_KEY`, `EV_REL`, `EV_ABS`, `EV_MSC` | Broad keyboard/media keys plus buttons including `BTN_0`, `BTN_SELECT`, `BTN_START`; `REL_HWHEEL`, `REL_HWHEEL_HI_RES`; absolute X/Y/Z/RX/RY/RZ, throttle, rudder, wheel, HAT0X/Y, volume, misc; `MSC_SCAN` |
| N2 | `EV_SYN`, `EV_KEY`, `EV_REL`, `EV_MSC` | Button codes 272–279; relative X/Y, horizontal/vertical wheel and their high-resolution forms; `MSC_SCAN` |
| N3 | `EV_SYN`, `EV_KEY`, `EV_ABS`, `EV_MSC` | Button codes 288–303 and 704–719; absolute X/Y/Z/RZ, throttle, HAT0X/Y; `MSC_SCAN` |

N1's sysfs/proc `ev=10001f` additionally contains bit 20 (`EV_REP`), which
evtest did not list as a named event-type section. This is a reporting
difference, not grounds to remove the observed repeat capability. evtest also
printed `?` for some key codes; the numeric bitmaps remain the precise evidence.
N1 is a single mixed keyboard/relative/absolute node despite its name and udev
keyboard classification. Capability enumeration does not prove which physical
controls produce those codes or which capabilities the current profile uses.

## Identity and grouping contract

For [#27](https://github.com/sh4869221b/azerlay/issues/27), admit an input event
node only when all of the following observable conditions hold:

1. It is an `input` subsystem `event*` node with a resolved USB interface and
   USB device ancestor. The interface must belong to that device, not a hub or
   another USB device further up the ancestry.
2. That USB device reports `idVendor=16d0`, `idProduct=12f7`, and
   `bcdDevice=0111`. The release restriction bounds this contract to the observed
   USB signature; it does not identify a hardware revision or assert firmware
   compatibility.
3. The interface number and class/subclass/protocol match one row below, and
   the node's capability bitsets contain every required type and code in that
   row. These are minimum role checks, not equality with the full recorded
   bitmaps. Additional key codes or capabilities do not alone disqualify a node.

| Interface | Required class / subclass / protocol | Required event types | Required codes | Admitted role |
| --- | --- | --- | --- | --- |
| `01` | `03/00/00` | `EV_SYN`, `EV_KEY`, `EV_REL`, `EV_ABS` | `KEY_A`, `REL_HWHEEL`, `ABS_X`, `ABS_Y` | One mixed keyboard/relative/absolute node |
| `02` | `03/01/02` | `EV_SYN`, `EV_KEY`, `EV_REL` | `BTN_LEFT`, `REL_X`, `REL_Y` | Mouse buttons and relative motion |
| `03` | `03/00/00` | `EV_SYN`, `EV_KEY`, `EV_ABS` | `BTN_TRIGGER`, `ABS_X`, `ABS_Y` | Joystick buttons and absolute axes |

The listed codes are observed representatives of each role. Matching every
keyboard/media key, axis, repeat bit, or udev classification flag would add an
unsupported compatibility requirement. Conversely, capabilities alone cannot
admit a general keyboard or mouse: the device and interface predicates are
mandatory. `ID_VENDOR_ID`, `ID_MODEL_ID`, and `ID_USB_INTERFACE_NUM` corroborate
the resolved ancestry; they must not override conflicting ancestor metadata.
If an input ID is obtained, a conflicting bus/VID/PID/version also prevents
admission. Device names, descriptor strings, screenshot versions, and handedness
are not runtime admission conditions.

An unknown VID/PID, USB release, interface, descriptor tuple, or missing required
role is outside this supported signature. Missing, unreadable, or contradictory
identity/ancestry/capability metadata leaves admission indeterminate; do not
select that node. Report the unsupported or unavailable evidence so that manual
inspection can explain it. A manually supplied event path never bypasses these
conditions. This decision does not define the diagnostic API or error codes.

Group admitted nodes by equality of their resolved USB **device** ancestor
within the current enumeration. N1, N2, and N3 therefore form one DeviceGroup
for D1. A shared hub, identical serial, matching name, or identical VID/PID does
not merge two distinct USB devices. Serial and physical path may help the owner
select among groups; they do not replace the parent grouping key, and an absent
serial does not by itself invalidate an otherwise qualifying node. Event numbers
are transient locators, not identity or group keys.

Admission is per node: an unsupported sibling cannot gain admission through an
admitted sibling. Missing or excluded siblings must remain visible as incomplete
or unsupported topology, without manufacturing a replacement node. Do not
require all three observed nodes merely to list a qualifying subset. N1 stays
one node with all its roles; it must not be duplicated into separate keyboard
and joystick nodes. Group discovery does not authorize reading every group:
runtime input remains restricted to the owner's selected qualifying group.

## Matching checks

The predicate above was evaluated against the acquired target metadata and two
unrelated USB input nodes on 2026-09-20. The negative-device observations used
`udevadm info --query=property --path=/sys/class/input/$event` and sysfs ancestor
and capability files only; their event nodes were not opened. The table is a
direct review of metadata against the contract, not an executed discovery
implementation or an applied udev rule test.

| Case | Evidence | Predicate result |
| --- | --- | --- |
| N1, N2, N3 | Observed `16d0:12f7`, release `0111`; matching interface descriptor tuples and required capability codes above; same D1 parent | All three admitted; one group; N1 retains its mixed role |
| Unrelated keyboard | Observed `056e:0134`, release `0120`, interface `00`; `ID_INPUT_KEYBOARD=1`; `ev=120013`, `rel=0`, `abs=0` | Excluded by device identity; keyboard classification cannot admit it |
| Unrelated mouse | Observed `046d:c33f`, release `3102`, interface `01`; `ID_INPUT_MOUSE=1`; `ev=17`, `rel=1943`, `abs=0` | Excluded by device identity, regardless of mouse capabilities |
| Same vendor, different product | Hypothetical `16d0:ffff` with otherwise matching node fields | Excluded by PID |
| Same name, different identity | Hypothetical name `Azeron LTD Azeron Cyborg II Keyboard`, VID/PID `0001:0001` | Excluded by VID/PID; name has no admitting role |
| Different USB release | Hypothetical `16d0:12f7`, `bcdDevice=0112` | Unsupported until separately evidenced; no firmware interpretation |
| Unknown event interface | Hypothetical event node under D1 interface `00`, `04`, or an unlisted number | Excluded; no event-node role is admitted for those interfaces |
| Descriptor or capability mismatch | Hypothetical interface `01` with class `ff`, or its observed tuple but no `EV_ABS`/`ABS_X` | Excluded by the descriptor or role requirement |
| Unavailable parent evidence | Hypothetical correct name and capabilities with unresolved USB device ancestor | Indeterminate; not selected |
| Two physical USB devices | Hypothetical two qualifying devices with equal serial and VID/PID under the same hub, but distinct resolved USB device ancestors | Qualifying nodes remain in two groups; neither serial nor hub merges them |

These results establish the documented predicate for the observed signature.
They do not independently qualify unseen hardware revisions, right-hand units,
or operating modes that happen to expose the same fields.

## Permission contract

The candidate for [#37](https://github.com/sh4869221b/azerlay/issues/37) is
`71-azerlay.rules`, placed after input/USB property imports and before seat ACL
handling. It adds `uaccess` only to input event nodes for the observed USB
VID/PID/release and interfaces `01`, `02`, or `03`:

```udev
ACTION!="remove", SUBSYSTEM=="input", KERNEL=="event*", ATTRS{idVendor}=="16d0", ATTRS{idProduct}=="12f7", ATTRS{bcdDevice}=="0111", ENV{ID_USB_INTERFACE_NUM}=="01|02|03", TAG+="uaccess"
```

All three `ATTRS` predicates refer to the same USB device ancestor.
`ID_USB_INTERFACE_NUM` is an event-device property imported from the USB
interface; using it avoids combining a device's VID/PID with interface
attributes in one parent match. The [systemd udev manual](https://github.com/systemd/systemd/blob/main/man/udev.xml)
requires parent matches in a rule to resolve on the same ancestor, defines
`ENV` as property matching and `|` as alternatives, and orders rule files
lexicographically.

The installed systemd 261 rules inspected on 2026-09-20 establish this order:

| Rule file | Observed relevant operation | Candidate dependency |
| --- | --- | --- |
| `60-input-id.rules` | Imports `input_id` for input devices when `ID_INPUT` is empty | Supplies observed input classifications; candidate does not rely on these as complete capabilities |
| `60-persistent-input.rules` | For USB ancestry and empty `ID_BUS`, imports builtin `usb_id` | N1–N3 already have interface properties `01`, `02`, `03`; these must be available before the candidate |
| `71-azerlay.rules` (candidate) | Matches the specific device release and event interfaces above | Adds the tag before seat processing |
| `73-seat-late.rules` | For a matching access tag and nonempty `MAJOR`, schedules builtin `uaccess` | Performs the platform's seat ACL handling |

The upstream [persistent-input rules](https://github.com/systemd/systemd/blob/main/rules.d/60-persistent-input.rules)
show the USB import, and [usb_id implementation](https://github.com/systemd/systemd/blob/main/src/udev/udev-builtin-usb_id.c)
reads `bInterfaceNumber` from the USB interface and exports
`ID_USB_INTERFACE_NUM`. The upstream [seat-late rules](https://github.com/systemd/systemd/blob/main/rules.d/73-seat-late.rules.in)
show the access-tag handling. These primary sources explain the mechanism;
the inspected installed files establish this host's ordering. Packaging must
check its target distribution's equivalent imports and seat rule order.
An absent interface property causes no match; do not weaken the rule to a
VID-only grant to compensate.

This permission candidate does not evaluate the full application admission
predicate. It does not test capability codes, interface class/subclass/protocol,
or which qualifying DeviceGroup the owner selects. An event node with the same
USB tuple and interface number but changed capabilities could receive the tag
while the application rejects it. Application admission must therefore retain
its independent ancestry, descriptor, and capability checks. The rule grants
no whole-USB, hidraw, `mouse*`, or `js*` access and changes no mode or group.
It neither requires root/input-group membership for normal use nor removes
access independently granted by existing system rules.

## Permission validation

The exact candidate line above was placed in an owned temporary
`.issue-13-71-azerlay.rules` outside the installed rule directories and checked
with `udevadm verify .issue-13-71-azerlay.rules` (systemd 261, 2026-09-20).
The command returned exit 0: `Success: 1`, `Fail: 0`, with one rules file checked.
The temporary file was then removed. This proves syntax/style acceptance only,
not event matching or effective permissions.

The following checks evaluate only this candidate's predicates against observed
metadata or explicitly hypothetical cases. They do not execute udev matching
or demonstrate an ACL change.

| Case | Conditions checked | Candidate outcome |
| --- | --- | --- |
| Observed N1 | For a non-remove event: `input`, `event*`, D1 `16d0:12f7:0111`, interface property `01` | Every condition matches; adds `uaccess` |
| Observed N2 | Same device and event conditions, interface property `02` | Every condition matches; adds `uaccess` |
| Observed N3 | Same device and event conditions, interface property `03` | Every condition matches; adds `uaccess` (already present independently) |
| Observed unrelated keyboard / mouse | `056e:0134` / `046d:c33f` from Matching checks | VID/PID conditions fail; no grant through this rule |
| Hypothetical same VID, other PID | `16d0:ffff`, other conditions matching | PID fails; no grant |
| Hypothetical other USB release | `16d0:12f7:0112`, other conditions matching | Release fails; no grant |
| Hypothetical unknown/missing interface | Interface `00`, `04`, unlisted, or property absent | Interface condition fails; no grant |
| Hypothetical non-input node | Same USB identity on `usb` or `hidraw` subsystem | Subsystem condition fails; no grant |
| Hypothetical non-event input node | Same USB identity and interface on a `mouse*`, `js*`, or `input*` node | Kernel-name condition fails; no grant |
| Hypothetical remove event | All identity conditions matching, `ACTION=remove` | Action condition fails; no grant |

Current permissions were inspected with `getfacl --absolute-names --numeric
"$node"` and node metadata; all three ACL queries returned exit 0. N1–N3 are
owned by root, group `input`, mode `0660`. N1 and N2 have only the owner/group/
other ACL entries. N3 additionally grants the current user `rw-`, with mask
`rw-`. The acquisition user already belongs to `input`, so successful evtest
opens do not demonstrate operation without that group. No membership was changed.
N3's pre-existing `uaccess` tag is consistent with the installed
`70-uaccess.rules` joystick rule; no fresh rule application was observed.

The host also has an existing `99-azeron-devices.rules` with VID-only USB/hidraw
grants and `MODE="0666"`. Those broader subsystem rules are not this candidate,
were not changed, and are not evidence that event-node access came from them.
The candidate was not installed or triggered; no live `udevadm test` or
`test-builtin` was run. Seat ACL application, access without `input` membership,
and access revocation remain **not tested**, for packaging validation.

## Supported scope and gaps

| Scope | Decision |
| --- | --- |
| Observed D1 signature | Researched: `16d0:12f7`, USB release `0111`, interfaces `01/02/03` with the descriptor and minimum capability roles specified above. This is the implementable initial admission contract. |
| Left-hand and firmware evidence | One owner-attested left-hand Cyborg II; the supplied screenshot displays Current version `111` and Software `2.0.2`. These qualify the observation, not runtime identity. |
| Right-hand units and hardware revisions | Right-hand evidence absent; hardware revision unknown. No general variant compatibility claim. |
| Other releases, products, interfaces, or operating modes | Unqualified until separately observed. The screenshot's software-mode labels do not establish Linux mode, and USB release is not a hardware-revision identifier. |
| Physical controls and active profile behavior | Not established by capabilities; physical mapping belongs to the separate mapping research. |
| Permission deployment | Specific candidate and ordering established; actual installation and seat ACL behavior remain for packaging validation. |

The documented variant gaps satisfy #13's requirement to cover left/right or
note gaps. They do not prevent implementation for the observed signature, but
they keep broader R-005 coverage in [design research](../design-research.md)
unresolved. Unavailable HID report descriptors do not prevent this evdev-level
identity decision; no raw HID interpretation is required by the contract.

## Downstream handoff

- **[#27 device discovery](https://github.com/sh4869221b/azerlay/issues/27):**
  implement the admission and USB-device-parent grouping contract above, retaining
  N1's mixed role. Use the positive and negative cases as requirements for its
  synthetic fixtures. Report unsupported metadata and denied access without
  falling back to name-only or arbitrary-path selection. This research supplies
  the initial identity contract; discovery code, CLI, and implementation tests
  remain that issue's work.
- **[#37 packaging](https://github.com/sh4869221b/azerlay/issues/37):** package
  the specific candidate rule with target-distribution property-import and seat
  ordering verified. Validate installed access without root or permanent `input`
  membership, and the target session's grant/revocation behavior. Document
  permission troubleshooting and the unsupported signatures. This research
  supplies rule specificity, not an installed-access pass or approval of the
  host's pre-existing broad USB/hidraw grants.

## Acceptance status

The research criteria for #13 are met for the stated initial signature:
`lsusb`, udev, sysfs, evtest, and proc observations are recorded; firmware,
revision, and handedness have explicit evidence or gaps. The matching and
permission tables show why unrelated input devices do not match the proposed
conditions. The concrete rule passed `udevadm verify`, and both downstream
issues have an actionable contract. Static predicate review and syntax
validation are not claimed as runtime discovery or applied ACL testing.

This document is #13's research deliverable for the observed signature. Broader
variant qualification and packaging's installed ACL validation remain
unverified and outside this observation's scope.
