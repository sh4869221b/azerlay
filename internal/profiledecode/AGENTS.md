# PROFILE DECODE KNOWLEDGE BASE

## OVERVIEW

Strict, bounded trust boundary for untrusted Azeron profile exports. This package earns local guidance as the repository's sole implemented domain and its densest API and validation surface.

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Trace the complete decode path | `decode.go` | `DecodeText`, `DecodeReader`, and `DecodeFile` converge here |
| Change outer-form detection | `base64.go` | Raw JSON and four Base64 forms; detection is deterministic |
| Change compressed-input handling | `lzma.go` | LZMA-Alone header, dictionary, output, and trailing-byte checks |
| Change envelope parsing | `msgpack_string.go` | Accepts only fixstr, str8, str16, and str32 |
| Change text wrappers | `normalize.go` | Narrow BOM, fence, quote, and whitespace normalization |
| Change JSON root rules | `root.go` | Provisional bundle/single classifier |
| Change machine errors | `errors.go` | Stable `ErrorCode` values and privacy-safe `DecodeError` |
| Check format authority | `../../docs/profile-format.md` | Current decoder contract and downstream ownership |
| Check admitted evidence | `../../docs/decisions/export-format-corpus.md` | Separates observed facts from synthetic capability |
| Exercise all entry points | `entrypoints_test.go` | Text, reader, and file equivalence and cleanup |
| Exercise boundary routing | `classification_test.go` | Text versus raw LZMA selection |
| Extend fuzz coverage | `fuzz_test.go` | Normalization, Base64, LZMA, MessagePack, and roots |

## CONVENTIONS

- Every failure returns the zero `Document`; adopt JSON only after every selected stage succeeds.
- Preserve caller independence by returning owned JSON bytes.
- Keep `DecodeError.Error()` equal to its stable code; retain underlying I/O causes through `Unwrap`.
- Enforce limits before allocating from or adopting untrusted declarations: 16 MiB source, 8 MiB compressed, 64 MiB dictionary/output.
- Select one input path deterministically. A selected text or compressed path is not retried as another form.
- Treat `../../docs/profile-format.md` as the decoder contract; future raw-model interpretation belongs downstream.
- Cross-check equivalent inputs through `DecodeText`, `DecodeReader`, and `DecodeFile`; fuzz each parsing stage independently.

## ANTI-PATTERNS

- Never recover data by scanning for the first `{` or by accepting partial decompression or trailing JSON.
- Never expose private raw exports or partially decoded source content in errors or logs; public fixtures must be privacy-safe and synthetic.
- Do not reinterpret MessagePack Map/Binary values as strings or repair input beyond allowed missing Base64 padding.
- Do not let raw schema, binding interpretation, persistence, or production CLI behavior enter this package.
- Do not weaken exact-length, UTF-8, root-disjointness, or compressed trailing-byte checks.
