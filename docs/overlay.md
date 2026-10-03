# Overlay appearance

The Cairo renderer draws a supplied layout and snapshot on a transparent GTK
layer-shell window. It supports dark and light themes, with an optional
high-contrast palette. `overlay.opacity` applies to the rendered content once;
`overlay.scale` changes its requested size, while the compositor handles output
scaling. `appearance.font_scale` changes text size. The supported settings and
ranges are listed in [configuration](config.md#overlay).

`overlay.mode` selects text density: compact shows the primary assignment,
normal adds its binding display, and detailed adds trigger and binding metadata
plus additional assignments where space permits. Single, long, and double
triggers are shown in that order. `MACRO`, `UNBOUND`, `UNKNOWN`, and `AMBIG`
describe configured assignment metadata, not a macro running or a trigger
firing. `overlay.show_unbound` and the renderer's ambiguity option control the
corresponding labels. Long text is ellipsized to fit the control; secondary
lines may be omitted when the region is too small. Pango uses the system's
available fonts for Unicode fallback.

Physical control color and marker indicate last-observed unknown, released, or
pressed state: unknown has a dashed outline and `?`, observed release a hollow
dot, and press a filled dot with a thicker outline; upper-right `-`, `?`, and
`A` mark static unbound, unknown, and ambiguous assignments. The stick's
configured assignment is static metadata and has no physical press marker. A
silent terminal input-report loss can leave the last observation stale; absence
of a new report does not mean release. The optional
profile title and status band are separate from the controls. Status can show
device disconnection, missing profile, or reload failure.

The running application assembles the selected imported/local profile, game
labels and raw physical observations into immutable latest snapshots. Profile
absence does not suppress known physical-button highlights. Failed source or
label reloads retain last-good content. Changes are coalesced at the configured
`input.refresh_hz` cap of 30, 60 or 120 Hz. Hidden updates retain latest state
without preparing frames; showing or recreating the surface displays that state.
Idle and status reads do not request redraws.

Native integration and opt-in read-to-actual-GTK-draw samples can be produced with
`AZERLAY_TEST_LIVE_EVIDENCE="$PWD/.omo/evidence/issue-35/native" scripts/test-wayland.sh go test ./cmd/azerlay -run '^TestRunLiveOverlay(Native|Latency)$' -count=1 -v`.
On an existing compositor, set `AZERLAY_TEST_WAYLAND_DISPLAY` to its absolute
Wayland socket path and optionally `AZERLAY_TEST_LIVE_MONITOR` to an output
connector. The test uses isolated config/source/control state and synthetic pipe
reports, records app-surface images over an owned solid background, and writes
bounded numeric timing CSV outside GTK callbacks. It reports matching sample and
exclusion counts, p50/p95/max and actual output refresh. This measures the app's
read-to-draw interval, not physical button-to-screen presentation. The 20 ms p95
target remains distinct from hardware and compositor presentation qualification.

Synthetic Cairo samples can be produced with
`AZERLAY_RENDER_QA_DIR="$PWD/.omo/evidence/issue-34/cairo" go test ./internal/renderer -run '^TestRenderSamples$' -count=1 -v`.
With the repository's native test dependencies installed, Sway output-scale
screenshots can be produced with
`AZERLAY_RENDER_QA_DIR="$PWD/.omo/evidence/issue-34/native" scripts/test-wayland.sh go test ./internal/overlay -run '^TestNativeRendererScaled$' -count=1 -v`.
