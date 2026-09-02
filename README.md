# Azerlay

Wayland-native Azeron profile and live input overlay for Linux.

## Status

Bootstrap phase. The current binary only exposes version and help output; profile,
device, and overlay functionality is not implemented yet.

The initial supported target is Linux/Wayland on x86-64 with Azeron Cyborg II,
Niri or Hyprland, and a Bodycam game profile.

## Development

Go 1.27.x is required.

```sh
go test ./...
go run ./cmd/azerlay version
```

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
