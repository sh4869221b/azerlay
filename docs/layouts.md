# Layout JSON v1

`internal/layout.Parse` reads one strict JSON object and validates its schema
and geometry. `LoadEmbedded("cyborg-ii", "left")` loads the validated
schematic shipped at `internal/layout/assets/cyborg-ii-left.json`. This package
provides layout data only; it does not render controls, select a device, or
match live input events.

## Definition

The root fields are `schema_version` (integer `1`), `model` (`"cyborg-ii"`),
`hand` (`"left"` or `"right"` as data), `applicability`, `view_box`,
`controls`, and optional `decorations`. `applicability` contains
`software_release`, `displayed_firmware`, nullable `hardware_revision`, and
`mode`. A control has a screen-position `id`, `group`, numeric
`source_input_id`, numeric `pin_one` and `pin_two`, nonnegative integer
`z_index`, one `shape`, and a `label_anchor` point. Source input IDs must be
positive and unique within a definition; pins must be nonnegative. Pin `255`
is an ordinary number, not an absent-value marker.

Position IDs name stable places in the schematic, such as `grid.c1.r1` and
`stick.main`. They do not come from source array order, input IDs, pins, or
profile labels. `source_input_id` and pins record the evidence-backed
correspondence for a particular applicability scope; they are not a rule for
matching runtime input events. Decorations are shapes without control identity
or source correspondence.

## Coordinates and shapes

`view_box` has `x`, `y`, `width`, and `height`. Coordinates are layout units
with an origin at the upper left, positive x to the right, and positive y
downward. Widths, heights, radii where applicable, and ellipse radii must be
finite and positive except rounded-rectangle radius, which may be zero.
Control geometry and label anchors must fit within the view box. An anchor must
also lie within its shape's axis-aligned bounds. The embedded asset places each
anchor at the exact shape center.

`shape` is a tagged object. Its `type` selects exactly the listed payload:

| Type | Required payload |
| --- | --- |
| `rounded_rect` | `x`, `y`, `width`, `height`, `radius` |
| `polygon` | `points`, an array of at least three points |
| `circle` | `cx`, `cy`, `radius` |
| `ellipse` | `cx`, `cy`, `rx`, `ry` |
| `line` | `from` and `to` points |
| `path` | `commands`, an array of path commands |
| `group` | `transform` and a nonempty `children` array of shapes |

Each point has numeric `x` and `y`. Polygon and line geometry must not be
degenerate. A path command has `op` and `points`: `M` and `L` take one point,
`C` takes three points, and `Z` takes none. A path starts with `M`; later
commands must follow a valid path sequence. Group transforms have
`translate_x`, `translate_y`, `scale_x`, and `scale_y`; all values must be
finite, and scales positive. Transformed child geometry must remain in the
view box. Fields that do not belong to the chosen shape type are rejected.

The embedded left layout uses view box `(0, 0, 694, 640)`, 31 independently
drawn schematic regions, and no decorations. Its coordinates reproduce the
screen-relative arrangement and dimensions recorded in the
[physical map decision](decisions/cyborg-ii-physical-map.md#schematic-geometry).
The map was drawn from that documented position table; manufacturer imagery,
Software screenshots, and private profile content are not embedded. The
analog center is the center anchor of `stick.main`.

## Errors and support scope

Layout errors carry a stable code and field path and do not include source JSON
values:

| Code | Meaning |
| --- | --- |
| `ERR_LAYOUT_JSON` | Malformed JSON or more than one JSON value |
| `ERR_LAYOUT_SCHEMA` | Missing, unknown, or incorrectly typed JSON field, or unsupported schema version |
| `ERR_LAYOUT_INVALID` | Invalid semantic value, identifier, shape, geometry, or anchor |
| `ERR_LAYOUT_UNSUPPORTED` | A validly formed model or embedded asset selection is unsupported |

`Parse` accepts right-hand data as a hand value, but v1 ships only the
left-hand Cyborg II asset. `LoadEmbedded("cyborg-ii", "right")` and unknown
model selections return `ERR_LAYOUT_UNSUPPORTED`; right-hand layouts are not
mirrored or inferred.

The embedded asset records the observed left-hand keyboard-stick arrangement
for Azeron Software `2.0.2`, displayed firmware `111`, and
`hardware_revision: null` because the hardware revision was not established.
This evidence does not establish compatibility with other releases, firmware,
hardware revisions, modes, hands, rendered output, or live input matching.
The other twelve source input IDs in the observed export have no positions in
this asset because no regions for them were established by the selected
screen arrangement.
