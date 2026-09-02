# Azerlay 1.0 Roadmap

Status: planning complete; implementation has not started.

GitHub Projects could not be provisioned during preflight, so this file is the planning index. Native sub-issue and blocked-by relations remain authoritative in GitHub Issues. No due date is assigned.

- [Milestone 1.0](https://github.com/sh4869221b/azerlay/milestone/1)
- [System design and research](docs/design-research.md)

## Phase 1: Foundation and blocking decisions

Create the repository baseline and resolve implementation-blocking compatibility, legal, format, device, overlay, local-source, and game-profile decisions.

- [#1](https://github.com/sh4869221b/azerlay/issues/1) Repository bootstrap and engineering baseline
- [#2](https://github.com/sh4869221b/azerlay/issues/2) Blocking compatibility and release decisions

## Phase 2: Core non-GUI product behavior

Implement strict profile import, normalized models, persistence, configuration, CLI, IPC, diagnostics, and safe input processing foundations.

- [#3](https://github.com/sh4869221b/azerlay/issues/3) Strict Azeron profile import and normalization
- [#4](https://github.com/sh4869221b/azerlay/issues/4) Profile sources, persistence, selection, and reload recovery
- [#5](https://github.com/sh4869221b/azerlay/issues/5) Device discovery, permissions, event reading, and reduction
- [#6](https://github.com/sh4869221b/azerlay/issues/6) Configuration, CLI, control socket, diagnostics, and lifecycle

## Phase 3: Overlay product behavior

Implement Cyborg II mapping, label resolution, GTK4 Layer Shell overlay, rendering, click-through, and live status visualization.

- [#7](https://github.com/sh4869221b/azerlay/issues/7) Cyborg II physical layout, matching, ambiguity, and labels
- [#8](https://github.com/sh4869221b/azerlay/issues/8) Wayland GTK4 Layer Shell overlay and rendering

## Phase 4: Distribution and release readiness

Add packaging, CI quality gates, acceptance validation, compatibility documentation, and public-release readiness without deciding the license prematurely.

- [#9](https://github.com/sh4869221b/azerlay/issues/9) Packaging, CI quality, security, compatibility, and release readiness

## Epics and review units

### [#1](https://github.com/sh4869221b/azerlay/issues/1) Repository bootstrap and engineering baseline

- [#10](https://github.com/sh4869221b/azerlay/issues/10) Bootstrap buildable repository baseline — `infra`

### [#2](https://github.com/sh4869221b/azerlay/issues/2) Blocking compatibility and release decisions

- [#11](https://github.com/sh4869221b/azerlay/issues/11) Decide supported Azeron export format corpus — `research`
- [#12](https://github.com/sh4869221b/azerlay/issues/12) Decide Azeron Software local store safe-read contract — `research`
- [#13](https://github.com/sh4869221b/azerlay/issues/13) Decide Cyborg II device identity, topology, and permission contract — `research`
- [#14](https://github.com/sh4869221b/azerlay/issues/14) Decide Cyborg II physical mapping contract — `research`
- [#15](https://github.com/sh4869221b/azerlay/issues/15) Decide GTK4/gotk4 ABI and click-through bridge contract — `research`
- [#16](https://github.com/sh4869221b/azerlay/issues/16) Decide fullscreen performance and compatibility support contract — `research`
- [#17](https://github.com/sh4869221b/azerlay/issues/17) Decide Bodycam built-in profile binding evidence — `research`
- [#18](https://github.com/sh4869221b/azerlay/issues/18) Decide public name, trademark, dependency license, NOTICE, SBOM, and final license readiness — `research`
- [#40](https://github.com/sh4869221b/azerlay/issues/40) Decide canonical Azeron binding conversion tables — `research`
- [#41](https://github.com/sh4869221b/azerlay/issues/41) Decide Niri and Hyprland placement and monitor contract — `research`

### [#3](https://github.com/sh4869221b/azerlay/issues/3) Strict Azeron profile import and normalization

- [#19](https://github.com/sh4869221b/azerlay/issues/19) Implement strict export decode envelope — `feature`
- [#20](https://github.com/sh4869221b/azerlay/issues/20) Implement raw export roots and lossless raw scalar model — `feature`
- [#42](https://github.com/sh4869221b/azerlay/issues/42) Implement normalized profile model and versioned adapters — `feature`
- [#43](https://github.com/sh4869221b/azerlay/issues/43) Implement import CLI validation and persistence-independent reporting — `feature`

### [#4](https://github.com/sh4869221b/azerlay/issues/4) Profile sources, persistence, selection, and reload recovery

- [#21](https://github.com/sh4869221b/azerlay/issues/21) Implement ImportedSource storage and selection state — `feature`
- [#22](https://github.com/sh4869221b/azerlay/issues/22) Implement AzeronLocalSource safe read and fallback — `feature`

### [#5](https://github.com/sh4869221b/azerlay/issues/5) Device discovery, permissions, event reading, and reduction

- [#27](https://github.com/sh4869221b/azerlay/issues/27) Implement Cyborg II device discovery and permission diagnostics — `feature`
- [#28](https://github.com/sh4869221b/azerlay/issues/28) Implement ordered evdev readers and immutable reducer snapshots — `feature`
- [#44](https://github.com/sh4869221b/azerlay/issues/44) Implement analog-axis normalization and WASD mode state — `feature`
- [#45](https://github.com/sh4869221b/azerlay/issues/45) Implement SYN_DROPPED recovery and device reconnect — `feature`

### [#6](https://github.com/sh4869221b/azerlay/issues/6) Configuration, CLI, control socket, diagnostics, and lifecycle

- [#23](https://github.com/sh4869221b/azerlay/issues/23) Implement XDG TOML config and atomic reload — `feature`
- [#24](https://github.com/sh4869221b/azerlay/issues/24) Implement secure control socket and runtime CLI controls — `feature`
- [#25](https://github.com/sh4869221b/azerlay/issues/25) Implement run lifecycle, single instance, cleanup, and systemd-ready foreground mode — `feature`
- [#26](https://github.com/sh4869221b/azerlay/issues/26) Implement doctor diagnostics — `feature`

### [#7](https://github.com/sh4869221b/azerlay/issues/7) Cyborg II physical layout, matching, ambiguity, and labels

- [#29](https://github.com/sh4869221b/azerlay/issues/29) Implement Cyborg II layout assets and validation — `feature`
- [#30](https://github.com/sh4869221b/azerlay/issues/30) Implement physical matching and duplicate-output candidate sets — `feature`
- [#31](https://github.com/sh4869221b/azerlay/issues/31) Implement game-profile loading and deterministic label resolution — `feature`
- [#46](https://github.com/sh4869221b/azerlay/issues/46) Implement bounded long, double, and macro sequence matching — `feature`
- [#47](https://github.com/sh4869221b/azerlay/issues/47) Ship evidence-backed Bodycam game profiles — `feature`

### [#8](https://github.com/sh4869221b/azerlay/issues/8) Wayland GTK4 Layer Shell overlay and rendering

- [#32](https://github.com/sh4869221b/azerlay/issues/32) Implement internal gtk4-layer-shell CGo bridge and overlay window lifecycle — `feature`
- [#33](https://github.com/sh4869221b/azerlay/issues/33) Implement click-through, focus avoidance, monitor/remap handling, and show/hide map behavior — `feature`
- [#34](https://github.com/sh4869221b/azerlay/issues/34) Implement Cairo renderer for layout, labels, status, analog state, and visual tokens — `feature`
- [#35](https://github.com/sh4869221b/azerlay/issues/35) Integrate live overlay snapshots and performance validation hooks — `feature`

### [#9](https://github.com/sh4869221b/azerlay/issues/9) Packaging, CI quality, security, compatibility, and release readiness

- [#36](https://github.com/sh4869221b/azerlay/issues/36) Implement CI quality gates and fixture policy — `infra`
- [#37](https://github.com/sh4869221b/azerlay/issues/37) Add native packaging assets for tarball, Arch, udev, systemd user service, and desktop integration — `feature`
- [#38](https://github.com/sh4869221b/azerlay/issues/38) Add compatibility matrix, acceptance validation checklist, and troubleshooting guide — `docs`
- [#39](https://github.com/sh4869221b/azerlay/issues/39) Prepare release artifacts, SBOM, license report, checksums, signatures, and final public license files — `infra`

## Dependency overview

The native dependency DAG contains 64 edges and is acyclic.

### Critical path

#10 → #11 → #14 → #29 → #30 → #46 → #34 → #35 → #16 → #38 → #39

### Research gates

- [#11](https://github.com/sh4869221b/azerlay/issues/11) Decide supported Azeron export format corpus → #40, #20
- [#12](https://github.com/sh4869221b/azerlay/issues/12) Decide Azeron Software local store safe-read contract → #22
- [#13](https://github.com/sh4869221b/azerlay/issues/13) Decide Cyborg II device identity, topology, and permission contract → #27, #37
- [#14](https://github.com/sh4869221b/azerlay/issues/14) Decide Cyborg II physical mapping contract → #29, #30
- [#15](https://github.com/sh4869221b/azerlay/issues/15) Decide GTK4/gotk4 ABI and click-through bridge contract → #32, #33
- [#16](https://github.com/sh4869221b/azerlay/issues/16) Decide fullscreen performance and compatibility support contract → #38, #39
- [#17](https://github.com/sh4869221b/azerlay/issues/17) Decide Bodycam built-in profile binding evidence → #47
- [#18](https://github.com/sh4869221b/azerlay/issues/18) Decide public name, trademark, dependency license, NOTICE, SBOM, and final license readiness → #39
- [#40](https://github.com/sh4869221b/azerlay/issues/40) Decide canonical Azeron binding conversion tables → #42
- [#41](https://github.com/sh4869221b/azerlay/issues/41) Decide Niri and Hyprland placement and monitor contract → #32, #33

## Parallel work

Each row is a topological wave; work within a row can proceed in parallel once prior dependencies are complete.

| Wave | Issues |
|---:|---|
| 1 | #10 |
| 2 | #11, #12, #13, #15, #17, #18, #19, #23, #36, #41 |
| 3 | #14, #20, #24, #27, #40 |
| 4 | #25, #26, #28, #29, #42 |
| 5 | #21, #30, #31, #32, #37, #43, #44 |
| 6 | #22, #33, #45, #46, #47 |
| 7 | #34 |
| 8 | #35 |
| 9 | #16 |
| 10 | #38 |
| 11 | #39 |

## Deferred non-blocking research

- R-020 MSC_SCAN physical-origin identification remains deferred because required fallback is all-candidate highlighting.
- R-021 read-only hidraw physical button/on-board profile read remains deferred because 1.0 normal path does not require it.
- R-022 Sway/KDE additional compositor verification remains deferred and must only be documented as provisional if later evidenced.
- R-023 AppImage/Flatpak packaging research remains deferred because initial distribution is native package/tarball.
- R-024 game process detection remains deferred because manual profile selection is the 1.0 behavior.

## Recommended first implementation issue

[#10](https://github.com/sh4869221b/azerlay/issues/10) Bootstrap buildable repository baseline
