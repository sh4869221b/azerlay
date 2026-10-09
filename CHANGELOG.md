# Changelog

This file records reviewed changes for candidate preparation. Entries describe
the checked-in project behavior; they do not imply a published release.

## [Unreleased]

- Import and validate Azeron Software 2.0.2 profile exports, preserve a selected
  profile, and support configured local profile files.
- Run a Wayland overlay for left-hand Azeron Cyborg II using passive hidraw
  button observations, with configuration reload, local controls and diagnostics.
- The v1 target is Linux/Wayland x86-64 on Arch/CachyOS with Hyprland. Other
  Software releases, Niri, Ubuntu and right-hand Cyborg II remain outside the
  qualified scope. Live analog/output-key state and macro completion are not
  reported; silently lost final releases can leave last-observed button state
  stale.
