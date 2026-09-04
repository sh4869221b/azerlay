# PROJECT KNOWLEDGE BASE

**Generated:** 2026-09-04T10:53:33Z
**Commit:** edf444b
**Branch:** main

## OVERVIEW

Azerlay is a Go 1.27 Linux/Wayland application for an Azeron profile and live input overlay. The repository is in bootstrap phase: the CLI exposes only version/help, while strict profile export decoding is the sole implemented domain.

## STRUCTURE

```text
azerlay/
├── cmd/azerlay/             # Current executable and CLI behavior tests
├── internal/profiledecode/  # Bounded untrusted-profile decoder
├── docs/                    # Normative architecture, security, and format contracts
│   └── decisions/           # Evidence and compatibility decisions
├── .github/workflows/       # CI quality gates
├── go.mod                   # Go 1.27 module; one production dependency
└── README.md                # Current status and basic development commands
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Understand current product status | `README.md` | Distinguishes implemented bootstrap behavior from plans |
| Change executable behavior | `cmd/azerlay/main.go` | Sole binary entry point |
| Change profile import decoding | `internal/profiledecode/` | Has its own local guidance |
| Check architecture boundaries | `docs/architecture.md` | Concise non-negotiable system constraints |
| Check security/privacy rules | `docs/security.md` | Device access, local boundaries, and sensitive data |
| Check decoder behavior | `docs/profile-format.md` | Strict accepted forms, limits, and ownership |
| Check future design | `docs/design-research.md` | Normative plan; most described packages do not exist yet |
| Check compatibility evidence | `docs/decisions/export-format-corpus.md` | Observed versus synthetic versus unsupported forms |
| Change CI gates | `.github/workflows/ci.yml` | Formatting, vet, race tests, build |

## CODE MAP

| Symbol | Type | Location | Refs | Role |
|--------|------|----------|------|------|
| `main` | function | `cmd/azerlay/main.go:11` | entry | Process exit boundary |
| `run` | function | `cmd/azerlay/main.go:15` | CLI tests | Version/help dispatch |
| `DecodeText` | function | `internal/profiledecode/decode.go:32` | 25 | Central text/JSON/Base64 decode entry |
| `DecodeReader` | function | `internal/profiledecode/decode.go:61` | 19 | Bounded reader and raw-LZMA entry |
| `DecodeFile` | function | `internal/profiledecode/decode.go:92` | 11 | Read-only file lifecycle entry |
| `Document` | struct | `internal/profiledecode/decode.go:26` | package API | Owned JSON plus provisional root kind |
| `DecodeError` | struct | `internal/profiledecode/errors.go:19` | package API/tests | Stable privacy-safe failure contract |
| `decodeLZMA` | function | `internal/profiledecode/lzma.go:42` | internal/tests | Bounded complete decompression |
| `parseRoot` | function | `internal/profiledecode/root.go:16` | internal/fuzz | Bundle/single root classification |

## CONVENTIONS

- Require Go 1.27.x; CI currently installs 1.27.1.
- Add packages only when an issue introduces behavior; do not pre-create the planned package tree.
- Keep the application one Go binary and one process, controlled by CLI/configuration and a same-user Unix socket.
- Keep tests colocated, deterministic, race-enabled, shuffled, and parallel where fixtures are isolated.
- Treat `docs/design-research.md` as normative future design and the shorter architecture/security documents as implementation constraints.
- Preserve stable machine-readable errors and avoid source-content disclosure at untrusted boundaries.
- The license is not finalized; do not represent the repository as granting MPL-2.0 yet.

## ANTI-PATTERNS (THIS PROJECT)

- Do not add a Web UI, database, separate daemon/service, or speculative abstraction/package tree.
- Do not monitor general keyboards or grab, remap, inject, or mutate input/device configuration.
- Do not require root, permanent `input`-group membership, `uinput`, or external network access.
- Do not guess a physical control when bindings are ambiguous.
- Do not persist partial imports or replace last-known-good state after a reload failure.
- Do not put raw exports, complete input streams, macros, full labels, or private Azeron database contents in logs or fixtures.
- Do not claim Software 1.x/2.x support from synthetic or version-unknown format evidence.

## UNIQUE STYLES

- Planned input flow is evdev readers -> ordered reducer -> immutable snapshot -> overlay projection.
- GTK objects stay on the locked main OS thread; draw callbacks perform no I/O, parsing, or waiting.
- The gtk4-layer-shell CGo bridge remains isolated at the Layer Shell boundary.
- Raw Azeron schema remains behind version-specific adapters; provisional decoder classification is not the authoritative model decision.

## COMMANDS

```bash
go run ./cmd/azerlay version
gofmt -l .
go vet ./...
go test -race -shuffle=on -count=1 ./...
go build ./cmd/azerlay
```

## NOTES

- Current binary functionality is intentionally limited to `version` and `--help`.
- `docs/design-research.md` describes future packages, not current code.
- `azerlay-system-design.md` is an ignored local duplicate; the tracked canonical file is `docs/design-research.md`.
- Normal operation is designed to make no external network connections and send no telemetry.
