# Compatibility and qualification

This page separates the v1 support target from the narrower configurations
that have direct evidence. Being inside the target does not mean every version
or acceptance criterion has been qualified.

| Classification | Meaning |
| --- | --- |
| Formal support | Part of the declared v1 target. The evidence and limits in the row still apply. |
| Provisional / prepared | A limited observation or test setup exists; it does not establish general support. |
| Unsupported for v1 | Outside the current target or admission contract. This means unverified unless the row says incompatibility was observed. |

## Compatibility matrix

| Area | Classification | Verified scope and evidence | Limits |
| --- | --- | --- | --- |
| OS and architecture | Formal support | Linux/Wayland x86-64; Arch and CachyOS are the v1 distribution target in the [project status](../README.md#status). | This does not qualify every distribution, kernel, or package combination. The Cyberpunk display record does not identify its OS/kernel tuple. |
| Compositor | Formal support | Hyprland is the v1 compositor. The live game-display observation used Hyprland 0.56.2; see the [display decision](decisions/fullscreen-performance.md). | Evidence is specific to the recorded run, not every Hyprland version or configuration. |
| Niri | Unsupported for v1 | Deferred and unverified; see the [placement decision](decisions/compositor-placement.md). | No incompatibility has been demonstrated. |
| Sway | Provisional / prepared | Sway supplies isolated test infrastructure. | It is not a formally qualified product target; test infrastructure is not a product-support result. |
| KDE | Unsupported for v1 | No KDE qualification is recorded. | This is a target-scope limit, not a demonstrated incompatibility. |
| Ubuntu | Unsupported for v1 | Explicitly outside v1; qualification is tracked by [Issue #98](https://github.com/sh4869221b/azerlay/issues/98). | This is a scope exclusion, not a demonstrated incompatibility. |
| Device and hand | Formal support | Left-hand Azeron Cyborg II is the v1 device target. The physical mapping evidence is limited to the documented left-hand unit; see the [device identity decision](decisions/device-identity.md) and [physical mapping decision](decisions/cyborg-ii-physical-map.md). | Other Cyborg models, hardware revisions, firmware, or hands are not implied. Right-hand Cyborg II is outside v1 under [Issue #89](https://github.com/sh4869221b/azerlay/issues/89). |
| USB identity and interface | Formal support | Admission requires USB `16d0:12f7:0111`, interface04 (`03/00/00`) and the qualified vendor report descriptor. | A matching name, path, or product family alone is insufficient. Other identities are unsupported by the current admission contract; unavailable or conflicting metadata is indeterminate. |
| Azeron software and device mode | Provisional / prepared | The owner-operated lifecycle observation used Software 2.0.2, displayed firmware 111, left-hand Cyborg II, and official Azeron Software running in SOFTWARE mode. Details and limits are in the [physical mapping decision](decisions/cyborg-ii-physical-map.md). | This observation does not qualify the separate packaging access check. These are operating prerequisites for that lifecycle setup, not runtime-detected facts or qualification of all Software releases and firmware. The displayed firmware value is not an adapter selector. |
| GPU and driver | Provisional / prepared | The Cyberpunk display observation used NVIDIA RTX 4070 with driver 615.71.09; see the [display decision](decisions/fullscreen-performance.md). | One host does not establish general NVIDIA, AMD, Intel, or driver-version support. |
| Monitor and scale | Provisional / prepared | The game was displayed on DP-2 at 2560×1440, 200 Hz, scale 1. | This records one physical display. Synthetic fractional-scale probes and renderer scale tests are not physical-monitor qualification; see [overlay appearance](overlay.md) and the [placement decision](decisions/compositor-placement.md). No minimum resolution, refresh rate, or fractional scale is established. |
| Cyberpunk 2077 and Proton | Provisional / prepared | The normal overlay was visible above Cyberpunk 2077 2.31 (Steam build 20383525) with DW-Proton 11.0-14. This is display-only evidence. | No active profile or physical input events were confirmed in that game run, so it does not prove game-visible physical highlights or input delivery. |
| Borderless and fullscreen | Provisional / prepared | Borderless mode is the recorded game-display mode. | Fullscreen comparison is unverified and deferred post-v1 to [Issue #132](https://github.com/sh4869221b/azerlay/issues/132); no fullscreen compatibility result is claimed. |
| VRR and HDR | Provisional / prepared | VRR was inactive and the game's HDR setting was `None` in the display observation. | VRR-on and HDR-on behavior are unverified post-v1 research under [Issue #132](https://github.com/sh4869221b/azerlay/issues/132). |
| Direct Scanout | Provisional / prepared | Preliminary frame-time observations were collected while Direct Scanout was enabled; `directScanoutTo` was `0` even with Azerlay absent. | This does not show that Azerlay caused scanout loss or that hiding it restores scanout. Causality and recovery remain unverified post-v1 in [Issue #132](https://github.com/sh4869221b/azerlay/issues/132). |
| Nested or embedded Gamescope | Unsupported for v1 | No nested or embedded Gamescope qualification is recorded; see [Issue #132](https://github.com/sh4869221b/azerlay/issues/132). | A nested Hyprland headless-output test is not a Gamescope test. No incompatibility has been demonstrated. |

The game-display tuple was Hyprland 0.56.2, GTK 4.22.5, gtk4-layer-shell 1.3.0,
NVIDIA RTX 4070 / driver 615.71.09, Cyberpunk 2077 2.31 (Steam build 20383525),
DW-Proton 11.0-14, and DP-2 at 2560×1440, 200 Hz, scale 1. VRR was inactive;
HDR was `None`. The overlay used compact layout, top-right placement, opacity
0.8 and a 60 Hz maximum redraw rate. It appeared above the game, with unknown
physical-button state. The [tracked decision](decisions/fullscreen-performance.md)
contains the qualification boundary and the separate preliminary measurements.

Detailed fullscreen comparisons, repeated performance runs, physical
input-to-presentation latency, VRR/HDR-on behavior, Direct Scanout causality and
hide recovery, Gamescope, and other game-specific behavior remain nonblocking
post-v1 research under [Issue #132](https://github.com/sh4869221b/azerlay/issues/132).
