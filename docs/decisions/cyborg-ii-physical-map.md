# Cyborg II screen layout and physical-mapping limits

## Scope and status

For [#14](https://github.com/sh4869221b/azerlay/issues/14), the owner selected
the supplied Azeron Software screenshot's arrangement as the overlay reference.
This decision describes screen positions, owner-confirmed physical responses,
and observed export correlations without assigning finger names or anatomical
press directions. Further anatomical descriptions are not needed to recreate
this arrangement.

Formal v1 support targets left-hand Cyborg II only. The screenshot has 31
visible control regions: 29 have direct UI/export correlations and two have
weaker, explicitly provisional correlations. The owner subsequently confirmed
that each of the 31 screen regions responds to a distinct physical operation
and returns to its prior display after release or joystick recentering. This
verifies the screen arrangement for the tested profile, but does not by itself
prove each export ID/pin relation or an evdev-to-control mapping. Issue #14
remains open for those gaps. No layout assets, rendering, or matching code are
implemented by this document.

## Evidence and limitations

On 2026-09-20 the owner supplied a private export and UI screenshot and confirmed
the same profile, left-hand Cyborg II, Software `2.0.2`, and displayed firmware
`111`. Hardware revision is unknown. The export contains one profile with
`isSoftware: true` and 43 inputs; the screenshot shows a keyboard joystick.

Read-only Python inspection in memory checked complete LZMA-Alone decompression,
end-of-stream without trailing compressed data, exact MessagePack str32 length,
and a complete JSON bundle. This was not a production decoder test. Structure
does not prove physical positions. The [export corpus](export-format-corpus.md#issue-14-physical-mapping)
and [device identity observation](device-identity.md) also supply no physical map.
Displayed firmware is not an independent query or equivalent to USB
`bcdDevice` or hardware revision.

The source pair is **S1**, a document-local evidence name. Private exports,
decoded profiles, screenshots, assignments, labels, macros, serials, and local
paths are not reproduced. Published numeric fields identify source inputs and
pins, not profiles; UI correlations do not establish anatomical positions.

In the subsequent owner-operated test with the same displayed profile, the
owner pressed controls one at a time while watching Azeron Software. The owner
reported that each named button region lit separately and returned after
release. The joystick region responded to movement and returned at neutral.
The agent did not operate or independently observe the device; these results
are owner attestations, not a captured input-event trace.

## UI correlation method

The central 20 and two side regions have unique single-trigger keyboard-symbol
or modifier metadata matches within S1. The cluster's left and center regions
also have unique single-trigger matches. Its right region has a unique private
label match and matching macro step count, with neither private content copied.

The main stick matches the sole keyboard-stick input. Its three adjacent
regions match single/long/double trigger combinations. The below-stick region
is distinguished from another input sharing its output by the long-trigger
glyph. Output equality without trigger context is insufficient. These source
comparisons add no canonical binding or generic modifier conversion.

Two correlations are weaker. `cluster.top` matches the sample's unique
single-trigger macro with three steps, but count alone is not stable identity.
`cluster.bottom` is a residual correspondence to the sole input with first
`types` value string `"38"`, not an admitted meaning for that value. Both
are possible pairings, not confirmed source mappings or generic lookup rules.
Their geometry may be drawn with unknown mapping; neither authorizes runtime
physical highlighting.

## Screen position contract

Names are screen-relative, independent of labels, assignments, and source
array order. `grid.c1` through `grid.c4` run left to right; `r1` through
`r5` run top to bottom. `cluster` is the upper-right cross; `stick` is
the lower-right group. Follow S1 as displayed, without mirroring or interpreting
screen directions as anatomical directions.

These are stable screen-position IDs with owner-confirmed physical responses
for the tested profile, not anatomical finger or action names. The
[Physical Control design](../design-research.md#93-physical-control) still
requires a supported source-to-position mapping before runtime highlighting.
Model, hand, and firmware applicability belong at a future version-scoped
adapter boundary. Rendering and matching must not inspect opaque raw fields
for new semantics. This decision does not extend the normalized model or
admit parser rules for the recorded raw fields.

## Left-hand UI-to-export correspondence

All rows reference S1: owner-attributed left-hand / Software `2.0.2` / displayed
firmware `111` / unknown revision. `id`, `pinOne`, and `pinTwo` are JSON
numbers, not strings or array indices. Literal `255` remains a number, with
no absent-pin or sentinel semantics inferred. Every row has an owner-confirmed
physical-operation-to-screen-region response. The numeric source relation
remains UI/export correlation, including rows marked `UI-correlated`.

| Screen region | `id` | `pinOne` | `pinTwo` | UI evidence / disposition |
| --- | --- | --- | --- | --- |
| `grid.c1.r1` | 4 | 5 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c1.r2` | 3 | 4 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c1.r3` | 2 | 3 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c1.r4` | 1 | 2 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c1.r5` | 37 | 1 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c2.r1` | 8 | 11 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c2.r2` | 7 | 10 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c2.r3` | 6 | 9 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c2.r4` | 5 | 8 | 255 | UI-correlated: unique single-trigger modifier |
| `grid.c2.r5` | 38 | 7 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c3.r1` | 12 | 17 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c3.r2` | 11 | 16 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c3.r3` | 10 | 15 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c3.r4` | 9 | 14 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c3.r5` | 13 | 13 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c4.r1` | 17 | 26 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c4.r2` | 16 | 25 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c4.r3` | 15 | 24 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.c4.r4` | 14 | 23 | 255 | UI-correlated: unique single-trigger modifier |
| `grid.c4.r5` | 18 | 22 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.side-left` | 36 | 6 | 255 | UI-correlated: unique single-trigger symbol |
| `grid.side-right` | 19 | 27 | 255 | UI-correlated: unique single-trigger symbol |
| `cluster.top` | 28 | 34 | 255 | Provisional: sample-specific macro count only |
| `cluster.left` | 29 | 35 | 255 | UI-correlated: unique single-trigger symbol |
| `cluster.center` | 22 | 37 | 255 | UI-correlated: unique single-trigger symbol |
| `cluster.right` | 31 | 33 | 255 | UI-correlated: private label and macro count |
| `cluster.bottom` | 30 | 36 | 255 | Provisional: residual type correspondence |
| `stick.main` | 24 | 31 | 30 | UI-correlated: sole keyboard-stick input |
| `stick.right-upper` | 41 | 20 | 255 | UI-correlated: symbol and three trigger glyphs |
| `stick.right-lower` | 20 | 19 | 255 | UI-correlated: double-trigger symbol |
| `stick.below` | 23 | 32 | 255 | UI-correlated: long-trigger symbol |

The remaining 12 numeric source IDs are `21`, `25`, `26`, `27`, `32`,
`33`, `34`, `35`, `39`, `40`, `42`, and `43`. Their displayed
controls are unknown; do not invent regions or merge them into visible controls.
The 31 listed, including two provisional, and 12 unresolved records account for
all 43 inputs exactly once. This is not a physical-button count or proof that
unresolved inputs are unused.

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
directions. The test did not expose live raw input IDs/pins for each press, and
it did not resolve the two provisional UI/export correspondences. It also did
not identify physical positions for the 12 inputs absent from the screenshot.
If a future source exposes raw input IDs during a press, compare them to the
recorded rows one at a time without rebinding; keep conflicts unresolved.

## Unmapped and ambiguous behavior

Unknown physical mapping, unknown binding semantics, and duplicate-output
candidates are separate states. Unresolved or provisional evidence must not
select an arbitrary physical highlight. Preserve all known duplicate-output
candidates under #30's existing contract. Missing or conflicting ID/pin facts
stay unresolved; no field precedence, substituted pin, or array-index fallback
is admitted. Missing pin evidence does not invalidate an independently proven
input-ID relation; this test did not expose raw IDs during a press.

## Revision and firmware coverage

S1 supplies one left-hand software-profile keyboard-stick arrangement. It does
not prove unchanged source positions after rebinding, equivalence to on-board
or Xbox modes, or a universal left-hand pin map. Automatic runtime selection
and highlighting still require supported adapter/device evidence. Unknown
revision does not admit all revisions. Other profiles, modes, releases,
firmware versions, and hands need their own applicability evidence.

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
| #29 | Missing, conflicting, or provisional mapping | No fabricated region, pin precedence, mirrored mapping, or promotion to physical proof. |
| #29 | Twelve unresolved inputs and literal pins | Inputs remain unresolved; numeric `255` is not silently treated as missing. |
| #29 | Unsupported hand or other applicability | Explicit unsupported scope; no unverified automatic layout selection. |
| #30 | Duplicate outputs among known candidates | Preserve the complete candidate set independent of source order. |
| #30 | Equal outputs with distinct triggers | Preserve single/long/double distinctions without adding canonical conversion rules. |
| #30 | Macro-count and residual-type matches | Provisional observations never authorize generic matching or physical highlighting. |

## Downstream readiness

#29 can use the names, geometry, owner-confirmed screen responses, and fixture
requirements to prepare the owner-selected schematic. The UI table provides
scoped research correlations; its two provisional rows require stronger source
evidence before admission as mapping rules. #30 receives no verified
evdev-to-physical candidate rule or new binding conversions.

Source review, the screen layout decision, and owner-operated screen-response
check are complete at this scope. The complete input-ID/pin-to-physical map
remains unverified. Committing this document alone does not complete #14 or v1,
and does not waive rendering, integration, compatibility, or release gates.
