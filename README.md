# Azerlay

Wayland-native Azeron profile and live input overlay for Linux.

## Status

Bootstrap phase. The CLI can validate Azeron Software 2.0.2 profile exports and
select one profile for the current `import` invocation. It doesn't save the
export or selection. Device and overlay functionality is not implemented yet.

The initial supported target is Linux/Wayland on x86-64 with Azeron Cyborg II,
Niri or Hyprland, and a Bodycam game profile.

## Development

Go 1.27.x is required.

```sh
go test ./...
go run ./cmd/azerlay version
```

## Profile export CLI

Both commands require exactly one source: a file, `-` for stdin, or literal
`--text` input. Attribute the export with the exact supported Software release.

```sh
azerlay validate --software-release 2.0.2 --text '{"id":"fixture","inputs":[]}' --json
azerlay import --software-release 2.0.2 --profile-index 2 internal/profileadapter/testdata/bundle.input.json --json
cat internal/profileadapter/testdata/single.input.json | azerlay validate --software-release 2.0.2 - --json
```

`validate` lists every normalized profile. `import` selects its sole profile by
default, but a multi-profile export requires `--profile-index N`. Indices are
one-based. Selection is only reported for that command invocation and no state
is saved.

Use `--json` for one schema-versioned JSON object. Successful operations exit
0. Invalid input, normalization failures, and missing or invalid selections
exit 1. Invalid CLI arguments exit 2. Unknown bindings produce
`WARN_IMPORT_UNKNOWN_BINDINGS` warnings but don't make a successful operation
fail.

Reports include deliberately permitted metadata: root kind, raw export version,
profile name, input count, selection, and Unknown warning counts. They don't
include IDs, labels, bindings, macros, unknown fields, or other private export
content. The raw export version is reported metadata. It isn't the
`--software-release` attribution and doesn't establish Software support.

## Project documents

- [System design and research](docs/design-research.md)
- [Architecture principles](docs/architecture.md)
- [Security principles](docs/security.md)
- [GitHub roadmap](https://github.com/sh4869221b/azerlay/milestone/1)

## License

The project license is not finalized. MPL-2.0 is recommended by the design,
pending dependency-license, NOTICE, and SBOM review before a public release.
No final license grant is made by this repository at this stage.

## Trademark notice

Azerlay is an independent, unofficial project and is not affiliated with,
endorsed by, or sponsored by Azeron SIA. Azeron and Cyborg are trademarks
of their respective owner.
