# LZMA boundary fixtures

These synthetic LZMA-Alone streams contain only zero bytes: `zeros-64mib.lzma`
expands to `64 << 20` bytes and `zeros-64mib-plus-one.lzma` to `(64 << 20) + 1`.
Their headers declare unknown output size and their streams include an EOS
marker, so tests exercise the actual decompressed-output limit.

Regenerate from the repository root:

```sh
go run ./internal/profiledecode/testdata/generate
git diff --exit-code -- internal/profiledecode/testdata/zeros-64mib.lzma internal/profiledecode/testdata/zeros-64mib-plus-one.lzma
```

The [generator](generate/main.go) uses the existing `github.com/ulikunitz/xz`
version selected by `go.mod`. It streams the exact zero-byte counts through an
LZMA writer with `DictCap: lzma.MinDictCap`, `SizeInHeader: false`, and
`EOSMarker: true`, then writes only the two compressed files. No decompressed
files are stored. Normal tests only read these fixtures; the dedicated
`generated-files` CI job regenerates them and rejects differences.

Raw user profiles and private export contents must never appear in fixtures,
logs, artifacts, or fuzz corpus. See the repository-wide
[CI and synthetic fixture policy](../../../docs/ci.md) for manually maintained
fixtures and safe fuzz regression seeds. These zero-only fixtures do not
establish support for additional Azeron Software releases.
