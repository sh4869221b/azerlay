# Game overlay display and deferred performance research

## v1 decision

On 2026-10-04, the normal foreground Azerlay overlay was visibly displayed
above Cyberpunk 2077 in borderless mode on Hyprland. This satisfies Issue #16's
revised v1 completion condition: display the overlay above a real game.
Detailed performance conditions are nonblocking post-v1 work in
[Issue #132](https://github.com/sh4869221b/azerlay/issues/132).

The tested tuple was Hyprland 0.56.2, GTK 4.22.5, gtk4-layer-shell 1.3.0,
NVIDIA RTX 4070 / driver 615.71.09, Cyberpunk 2077 2.31 (Steam build 20383525),
and DW-Proton 11.0-14. DP-2 was 2560×1440 at 200 Hz, scale 1, color preset
`wide`, format `XBGR2101010`, VRR inactive. The game's HDR setting was `None`.
The normal overlay used compact layout, top-right placement, opacity 0.8,
60 Hz maximum redraw rate and `show_unbound=true`. No active profile or physical
input events were confirmed; the displayed controls had unknown state.
This confirms display, without claiming live physical highlights in the game.

The local evidence is `.omo/evidence/issue-16/game-overlay-visible.png`,
`runtime/visible-status.json` and `runtime/visible-layers.json` under the same
directory. The image shows the overlay above the game's benchmark result;
status reports `visible=true`, `overlay.mapped=true` and
`overlay.input_region_applied=true`, and the compositor records one `azerlay`
layer on DP-2. The input-region flag records the API call, not delivery proof.
After `hide`, `runtime/hidden-status.json` reports `visible=false` and
`overlay.mapped=false`; `runtime/hidden-layers.json` records no `azerlay` layer
on either DP-1 or DP-2. This confirms unmapping in the captured session, without
establishing Direct Scanout recovery.
The [public status excerpt](fullscreen-performance/display-status.json) omits
device paths and process identifiers. No private settings or screenshot is
included in the public excerpt.

## Preliminary measurements, without performance qualification

The first absent / visible / hidden trio used Cyberpunk's built-in benchmark,
direct Hyprland session, borderless mode, VRR off and HDR off. MangoHud logging
started before benchmark confirmation; analysis uses only elapsed time
`25 <= seconds < 60`, a fixed 35-second inner window that excludes startup.
There was **one run per state**, with a static overlay and no confirmed input
events. The [numeric excerpt](fullscreen-performance/preliminary-metrics.csv)
records mean and nearest-rank p50/p95 frame times, using index `ceil(n*p)-1`.

| State | Frames in window | Mean ms | p50 ms | p95 ms |
| --- | ---: | ---: | ---: | ---: |
| Absent | 2334 | 14.996589 | 14.8557 | 18.2336 |
| Visible | 2311 | 15.144349 | 15.1124 | 18.5269 |
| Hidden | 2310 | 15.152762 | 15.0588 | 18.4998 |

These are preliminary observations, not repeated comparisons or a performance
guarantee. Source CSVs and compositor state captures are retained locally at
`.omo/evidence/issue-16/direct/borderless-vrr-off-hdr-off/`, named
`{absent,visible,hidden}-1.csv`, `*-1-state.txt` and `*-1-result.png`.
Direct Scanout was enabled for these observations, but `directScanoutTo` was
`0` even with Azerlay absent. Existing cursor and desktop-shell layers remained
present. This baseline cannot establish overlay-caused scanout loss or recovery
after hide.

A separate native test measured 225 synthetic pipe-read-return to actual GTK
draw-end samples with the 60 Hz redraw setting: p50 1.262557 ms,
p95 1.737491 ms and maximum 7.784565 ms. These use the existing native test's
sorted-sample index `(n-1)*percentile/100` with integer division. Its raw samples are
`.omo/evidence/issue-16/app-latency/latency-60hz.csv`; the passing native-test
transcript is `app-latency/test-output.txt`. This interval excludes physical
device latency, compositor presentation and panel scanout. It does not prove
physical input-to-presentation latency or a game performance limit.

Fullscreen/borderless comparisons, repeated runs, CPU/GPU overhead, physical
presentation latency, Direct Scanout causality and hide recovery, VRR on, HDR
on, Gamescope and other game-specific compatibility remain unverified and are
deferred to Issue #132. Niri remains outside v1.
