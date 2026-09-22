# Cyborg II left-hand screen layout and physical mapping

## Scope and status

For [#14](https://github.com/sh4869221b/azerlay/issues/14), the owner selected
the supplied Azeron Software screenshot's arrangement as the overlay reference.
This decision describes screen positions, owner-confirmed physical responses,
and observed export correlations without assigning finger names or anatomical
press directions. Further anatomical descriptions are not needed to recreate
this arrangement.

Formal v1 support targets left-hand Cyborg II only. The screenshot has 31
visible control regions: 30 buttons and one joystick, matching the
[manufacturer's device description](https://www.azeron.eu/support/faq/).
The installed Software `2.0.2` renderer directly identifies all 31 regions by
input ID. Every listed ID/pin pair agrees with the owner's same-profile export,
and the owner confirmed that each region responds to its own physical operation
and returns after release or joystick recentering. This establishes the
left-hand position map for the observed Software/profile/device combination.
It does not prove other firmware, hardware revisions, hands, or an
evdev-to-control matching rule. No layout assets, rendering, or matching code
are implemented by this document.

## Evidence and limitations

On 2026-09-20 the owner supplied a private export and UI screenshot and confirmed
the same profile, left-hand Cyborg II, Software `2.0.2`, and displayed firmware
`111`. Hardware revision is unknown. The export contains one profile with
`isSoftware: true` and 43 inputs; the screenshot shows a keyboard joystick.

Read-only Python inspection in memory checked complete LZMA-Alone decompression,
end-of-stream without trailing compressed data, exact MessagePack str32 length,
and a complete JSON bundle. This was not a production decoder test. Structure
alone does not prove physical positions. The [export corpus](export-format-corpus.md#issue-14-physical-mapping)
and [device identity observation](device-identity.md) supply the earlier
evidence boundary. Displayed firmware is not an independent query or
equivalent to USB `bcdDevice` or hardware revision.

The owner export and screenshot are **S1**, a document-local evidence name.
The locally installed `azeron-software` package identifies itself as version
`2.0.2`; its bundled renderer data is **S2**. S2 selects the Cyborg II
31-button-code set and supplies the screen-position-to-ID layout variant that
matches every visible position in S1. All 31 IDs in that set appear in S1 with
the listed pins. The other 12 export inputs are outside this displayed button
set; their physical role is not inferred. Private exports, decoded profiles,
screenshots, assignments, labels, macros, serials, local paths, and bundled
application code are not reproduced.

For repeatable source review, S2 is the installed package's `resources/app.asar`:
its `package.json` states the version, and the renderer asset under
`out/renderer/assets/` contains the Cyborg II visible-button list and layout
variant. The numeric result below was compared against both S1 sources in
memory; no application asset is copied into this repository.

In the subsequent owner-operated test with the same displayed profile, the
owner pressed controls one at a time while watching Azeron Software. The owner
reported that each named button region lit separately and returned after
release. The joystick region responded to movement and returned at neutral.
The agent did not operate or independently observe the device; these results
are owner attestations, not a captured input-event trace.

## Source-to-screen matching

S2's Cyborg II button list contains exactly the 31 IDs in the table below.
Its screen layout places each ID at the corresponding S1 screenshot region,
including `cluster.top` (`28`) and `cluster.bottom` (`30`). The table's pin
values come from those exact IDs in S1. A row-by-row comparison confirmed all
31 position/ID/pin triples; no assignment value, source array index, or
macro count is needed to choose a position.

S1's distinct keyboard symbols, modifier metadata, joystick type, and trigger
glyphs provide independent visual corroboration for most regions. The macro
count and raw type value used as tentative clues in the first research pass
are not generic position or binding rules. S2 resolves the two formerly
tentative positions directly. Neither source establishes an evdev event-to-ID
rule, a new canonical binding conversion, or applicability beyond the stated
combination.

## Screen position contract

Names are screen-relative, independent of labels, assignments, and source
array order. `grid.c1` through `grid.c4` run left to right; `r1` through
`r5` run top to bottom. `cluster` is the upper-right cross; `stick` is
the lower-right group. Follow S1 as displayed, without mirroring or interpreting
screen directions as anatomical directions.

These are stable screen-position IDs with owner-confirmed physical responses
for the tested profile, not anatomical finger or action names. They provide
the position identity required by the
[Physical Control design](../design-research.md#93-physical-control) for this
observed scope. Model, hand, and firmware applicability belong at a future
version-scoped adapter boundary. Rendering and matching must not inspect
opaque raw fields for new semantics. This decision does not extend the
normalized model or admit parser rules for the recorded raw fields.

## Left-hand UI-to-export correspondence

All rows refer to S1's owner-attributed left-hand unit / Software `2.0.2` /
displayed firmware `111` / unknown hardware revision, with positions verified
against S2's matching screen layout. `id`, `pinOne`, and `pinTwo` are JSON
numbers, not strings or array indices. Literal `255` remains a number, with
no absent-pin or sentinel semantics inferred. Every region also has an
owner-confirmed physical-operation-to-screen response.

| Screen region | `id` | `pinOne` | `pinTwo` |
| --- | --- | --- | --- |
| `grid.c1.r1` | 4 | 5 | 255 |
| `grid.c1.r2` | 3 | 4 | 255 |
| `grid.c1.r3` | 2 | 3 | 255 |
| `grid.c1.r4` | 1 | 2 | 255 |
| `grid.c1.r5` | 37 | 1 | 255 |
| `grid.c2.r1` | 8 | 11 | 255 |
| `grid.c2.r2` | 7 | 10 | 255 |
| `grid.c2.r3` | 6 | 9 | 255 |
| `grid.c2.r4` | 5 | 8 | 255 |
| `grid.c2.r5` | 38 | 7 | 255 |
| `grid.c3.r1` | 12 | 17 | 255 |
| `grid.c3.r2` | 11 | 16 | 255 |
| `grid.c3.r3` | 10 | 15 | 255 |
| `grid.c3.r4` | 9 | 14 | 255 |
| `grid.c3.r5` | 13 | 13 | 255 |
| `grid.c4.r1` | 17 | 26 | 255 |
| `grid.c4.r2` | 16 | 25 | 255 |
| `grid.c4.r3` | 15 | 24 | 255 |
| `grid.c4.r4` | 14 | 23 | 255 |
| `grid.c4.r5` | 18 | 22 | 255 |
| `grid.side-left` | 36 | 6 | 255 |
| `grid.side-right` | 19 | 27 | 255 |
| `cluster.top` | 28 | 34 | 255 |
| `cluster.left` | 29 | 35 | 255 |
| `cluster.center` | 22 | 37 | 255 |
| `cluster.right` | 31 | 33 | 255 |
| `cluster.bottom` | 30 | 36 | 255 |
| `stick.main` | 24 | 31 | 30 |
| `stick.right-upper` | 41 | 20 | 255 |
| `stick.right-lower` | 20 | 19 | 255 |
| `stick.below` | 23 | 32 | 255 |

The remaining 12 numeric source IDs are `21`, `25`, `26`, `27`, `32`,
`33`, `34`, `35`, `39`, `40`, `42`, and `43`. They are not in S2's
Cyborg II visible-button list or the selected screenshot. Do not invent
regions or merge them into visible controls. The 31 listed and 12 excluded
source records account for all 43 export inputs exactly once. The role of the
excluded records is not established; their absence from this layout does not
prove they are unused across modes or revisions.

## Schematic geometry

Recreate the relative grouping as independent shapes, omitting Software
navigation and profile headers. Coordinates are layout units, not screen
pixels. Use `viewBox = (0, 0, 694, 640)`, origin top-left, positive x rightward
and y downward. Ordinary rectangles are 70 by 100 with an 8-unit adjacent gap.

| Region | x | y | Width | Height |
| --- | --- | --- | --- | --- |
| `grid.c1..c4.r1..r5` | `78 * column` | `108 * row` | 70 | 100 |
| `grid.side-left` | 0 | 324 | 70 | 100 |
| `grid.side-right` | 390 | 324 | 70 | 100 |
| `cluster.top` | 546 | 0 | 70 | 100 |
| `cluster.left` | 468 | 108 | 70 | 100 |
| `cluster.center` | 546 | 108 | 70 | 100 |
| `cluster.right` | 624 | 108 | 70 | 100 |
| `cluster.bottom` | 546 | 216 | 70 | 100 |
| `stick.main` | 468 | 324 | 148 | 208 |
| `stick.right-upper` | 624 | 324 | 70 | 100 |
| `stick.right-lower` | 624 | 432 | 70 | 100 |
| `stick.below` | 468 | 540 | 70 | 100 |

Columns and rows are one-based. Each label anchor is the shape center,
`(x + width/2, y + height/2)`. The analog visualization uses the center of
`stick.main`; no extra directional source controls are invented. Preserve
region IDs and grouping under scaling. Use normal runtime label resolution,
without private labels, official artwork, screenshots, or copied Software UI
assets embedded in the layout.

## Owner-operated screen response and remaining limit

The owner confirmed the following groups, one physical operation at a time,
against the selected screenshot:

| Group | Screen regions confirmed | Observed result reported by owner |
| --- | --- | --- |
| Grid top | `grid.c1.r1`, `grid.c2.r1`, `grid.c3.r1`, `grid.c4.r1` | Four distinct buttons lit their respective regions and returned after release. |
| Grid remainder | `grid.c1..c4.r2..r5` | Sixteen distinct buttons lit their respective regions and returned after release. |
| Grid sides and cluster | `grid.side-left`, `grid.side-right`, `cluster.top`, `cluster.left`, `cluster.center`, `cluster.right`, `cluster.bottom` | Seven distinct buttons lit their respective regions and returned after release. |
| Stick group | `stick.main`, `stick.right-upper`, `stick.right-lower`, `stick.below` | Joystick movement and three distinct buttons activated their respective regions; the display returned after recentering or release. |

These reports establish which displayed region reacts to each tested physical
operation. The owner was not asked to identify fingers or anatomical press
directions. The test did not expose live raw input IDs/pins for each press; S2's
matching screen layout supplies those ID positions independently. It also did
not identify physical positions for the 12 inputs outside the displayed button
set. If a future direct input trace conflicts with a recorded row, keep that
relation unresolved instead of overriding the observation.

## Unmapped and ambiguous behavior

Unknown physical mapping, unknown binding semantics, and duplicate-output
candidates are separate states. A source ID outside the evidenced 31-control
set must not select an arbitrary physical highlight. Preserve all known
duplicate-output candidates under #30's existing contract. Missing or
conflicting ID/pin facts stay unresolved; no field precedence, substituted pin,
or array-index fallback is admitted. Missing pin evidence does not invalidate
an independently proven input-ID relation. A keyboard output alone does not
prove which of multiple known controls generated it.

## Revision and firmware coverage

S1 and S2 jointly support the observed left-hand Software `2.0.2` keyboard-stick
arrangement with displayed firmware `111` and unknown hardware revision. They
do not prove equivalence to on-board or Xbox modes, a universal left-hand pin
map, or automatic detection of a compatible device. Rebinding changes labels
and output semantics, not these position IDs in S2, but other Software
releases, firmware versions, revisions, and hands need applicability evidence.
Unknown revision does not admit all revisions. Runtime selection and
highlighting still require scoped adapter and device evidence.

## Deferred right-hand support

[#89](https://github.com/sh4869221b/azerlay/issues/89) tracks right-hand support
after v1: independent annotated exports, actual presses, validated layout,
and matching/integration coverage. Mirroring S1 is not evidence. Accepted
configuration `right` is not formal v1 support or a blocker for left-hand work.

## Layout fixture requirements

These are downstream synthetic-fixture requirements, not created fixtures or
completed implementation checks. Synthetic data is not hardware evidence and
must not copy S1's private assignments or labels.

| Owner | Scenario | Required observable |
| --- | --- | --- |
| #29 | Selected screen arrangement | Exactly 31 uniquely named regions; all positions and sizes match the geometry and fit the viewBox. |
| #29 | Every label and analog center | Shape-center label anchors; analog confined to `stick.main`; no labels overlap unrelated shapes. |
| #29 | Reordered inputs and changed labels | Region names and geometry remain unchanged; no array index or private label becomes a region key. |
| #29 | Missing or conflicting mapping | No fabricated region, pin precedence, mirrored mapping, or promotion of an unsupported combination to physical proof. |
| #29 | Twelve excluded export inputs and literal pins | No visible position is invented for excluded IDs; numeric `255` is not silently treated as missing. |
| #29 | Unsupported hand or other applicability | Explicit unsupported scope; no unverified automatic layout selection. |
| #30 | Duplicate outputs among known candidates | Preserve the complete candidate set independent of source order. |
| #30 | Equal outputs with distinct triggers | Preserve single/long/double distinctions without adding canonical conversion rules. |
| #30 | Macro-count and residual-type clues | Sample-specific clues never authorize generic matching or new binding semantics. |

## Downstream readiness

#29 can implement the owner-selected left-hand schematic with all 31 region
names, shapes, label anchors, and source ID/pin relations for the evidenced
combination. #30 can consume this scoped position mapping as candidate data,
but must still resolve actual device events and duplicate outputs under its
own contract. No new binding conversion is admitted here.

The #14 research requirements for this observed left-hand combination are
met by S1's attributed export and owner-operated screen response, S2's direct
position/ID mapping, and the explicit revision gaps above. This does not
complete v1 or waive #29's asset, #30's matching, integration, compatibility,
or release gates.
