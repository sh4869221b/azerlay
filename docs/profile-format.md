# Strict profile export decoding

`internal/profiledecode` is Azerlay's strict boundary for untrusted profile export
input. It recognizes framing and returns owned JSON bytes with a provisional root
kind. It does not interpret the raw schema, bindings, or profile meaning.

The compatibility authority is
[the Azerlay 1.0 export format corpus](decisions/export-format-corpus.md). The
corpus separates observed evidence from privacy-safe synthetic protocol
capability. Synthetic success proves that the decoder handles a form. It does
not prove that any Azeron Software generation emits that form. No Software 1.x
or 2.x generation support is inferred.

## Entry points and result

The internal package exposes three entry points:

- `DecodeText(string)` accepts Raw JSON or Base64 text.
- `DecodeReader(io.Reader)` accepts every supported form, including raw binary
  LZMA-Alone.
- `DecodeFile(string)` opens a file read-only, closes it, and uses the reader
  path.

All paths share the same strict stages. A successful `Document` contains an
owned copy of the accepted JSON and a `bundle` or `single` provisional kind.
Callers can change their source after decoding without changing the result.
Every decode failure returns the zero `Document`, so no partial JSON or kind is
available for adoption. File and reader failures retain their underlying cause
through `DecodeError`, while the error text contains only its stable code.

## Accepted outer forms

Exactly six outer forms are supported:

1. Raw JSON bytes.
2. Raw LZMA-Alone bytes, through the reader or file entry point.
3. Padded Base64URL text.
4. Unpadded Base64URL text.
5. Padded standard Base64 text.
6. Unpadded standard Base64 text.

Text normalization is deliberately narrow. The decoder trims surrounding
whitespace, removes at most one leading UTF-8 BOM, unwraps at most one complete
whole-input triple-backtick fence, and then unwraps at most one matching
whole-input quote layer. The four quote forms are `'...'`, `"..."`, `'''...'''`,
and `"""..."""`. It trims around unwrapped content. Unmatched wrappers fail,
and bytes or characters in the middle are never deleted, replaced, or scanned
past.

Selection is deterministic, not a retry loop. Reader and file input first
applies a predicate to the whole byte sequence. A source is definitively text
when it is valid UTF-8 and every rune is either Unicode-printable or Unicode
whitespace. That text path remains authoritative even when normalization,
Base64, or JSON later rejects the input. For every other byte sequence, raw
header recognition is allowed. A source at least 13 bytes long whose first byte
is a legal LZMA property code takes the raw compressed path. If no such header
is recognized, the source follows `DecodeText`, which owns BOM and wrapper
normalization and any applicable rejection. Dictionary and output declarations
are not recognition policy; their limits are enforced by the LZMA decoder
after selection.

LZMA-Alone has no magic signature. Its first byte encodes properties and can
legally equal ASCII `[` (`0x5b`) or `{` (`0x7b`). The whole-input predicate is
the tie-break: definitively textual input takes precedence over coincidental
header-shaped bytes, while header-shaped non-text input is not reclassified by
Raw JSON's first-byte check. Once the raw path is selected, it is not retried as
another form after an error.

On the text path, normalization runs before outer-form detection. A leading
`{` or `[` then selects Raw JSON. All other text must fit one Base64 alphabet
and its terminal-padding form. Standard-only `+` or `/` and URL-only `-` or `_`
characters cannot be mixed. Alphanumeric-only text selects Base64URL, whose
decoded bytes are identical for both alphabets. Padding is accepted only at the
end, and missing terminal padding is the only repair. Internal whitespace,
invalid characters, excess padding, and impossible lengths fail.

Raw JSON bypasses Base64, LZMA, and MessagePack. Every Base64 form must decode to
the same compressed envelope accepted by the raw LZMA-Alone path.

## Limits and compressed stream rules

Limits are checked before the corresponding data is adopted:

| Boundary | Limit |
| --- | ---: |
| Source text or reader/file bytes | 16 MiB |
| Raw or Base64-decoded compressed input | 8 MiB |
| LZMA dictionary | 64 MiB |
| Complete decompressed output | 64 MiB |

Source input is read through the cap plus one byte so exact-limit input can be
distinguished from overflow. Base64 decoded size is checked before allocation.
The LZMA header's dictionary and known output size are checked before decoder
allocation, and decompressed output is read through its cap plus one byte.

Compressed input must be LZMA-Alone. The implementation pins the pure Go
`github.com/ulikunitz/xz` module at v0.5.16 and constructs
`lzma.ReaderConfig` with a 64 MiB dictionary cap. A stream succeeds only after
complete decompression. Malformed headers, oversized declarations, truncation,
known-size disagreement, decoder failure, and trailing compressed bytes are
rejected. Bytes produced before terminal success are never returned.

## MessagePack, UTF-8, and JSON

Decompressed data must contain exactly one MessagePack String object. These are
the only accepted tags:

| String form | Tag | Length field |
| --- | --- | --- |
| fixstr | `0xA0..0xBF` | low five tag bits |
| str8 | `0xD9` | one unsigned byte |
| str16 | `0xDA` | two unsigned big-endian bytes |
| str32 | `0xDB` | four unsigned big-endian bytes |

The declared length must equal every byte remaining after the header. A short
header, short payload, or trailing byte fails. Map, Binary, and all other
MessagePack types aren't coerced to strings. The decoder compares the untrusted
length to available bytes without allocating from that declaration.

The exact String payload must be valid UTF-8 and exactly one complete JSON
value. Malformed JSON and a second or trailing JSON value fail. On success, the
returned JSON preserves the accepted payload bytes and is copied into storage
owned by the result.

## Provisional root classification

Root classification checks key presence on a JSON object:

- `bundle` requires `profiles`; `version` is optional.
- `single` requires `inputs` and at least one of `id` or `name`.

A root matching both predicates or neither predicate is rejected. Arrays,
scalars, and `null` also fail root classification. This kind is only a decoder
boundary result. Issue #20 independently re-detects and validates the root while
constructing the lossless raw model. Its result is authoritative, and a
mismatch with this provisional kind is an error.

## Authoritative raw model

After strict decoding, `internal/profileraw.Parse` consumes the owned JSON and
provisional kind with this signature:

```go
Parse(document profiledecode.Document) (RawExport, error)
```

It returns a `RawExport` with exactly one non-nil root, either `RawBundle` or
`RawProfile`. A bundle models optional `version`, ordered `profiles`, and
unknown root fields. A profile models optional `id`, `name`, and `version`, plus
ordered `inputs` and unknown profile fields. Each input is an opaque
`map[string]json.RawMessage`; fields
such as `macro`, `longMacro`, and `doubleMacro` aren't interpreted or counted
at this boundary.

Modeled scalar fields use `RawScalar`. It distinguishes null, bool, string,
and number, keeps the exact source value token in owned `json.RawMessage`, and
stores numbers as `json.Number` without integer or floating-point narrowing.
Missing fields are nil, while present `null` values have a non-nil null scalar.
Numeric spelling such as exponents, fractions, overflow-sized integers, and
negative zero remains unchanged. Value-token whitespace and string escapes are
preserved, but separator whitespace isn't. Empty accepted arrays remain
non-nil. Unknown maps exclude only the keys modeled at their level. Their
values are also owned raw bytes, so later changes to the decoded document can't
alter a successful model. Object key order and insignificant whitespace outside
value tokens aren't round-trip guarantees. A scalar decode failure resets its
receiver before returning `ERR_IMPORT_ROOT`; `Parse` maps a container-valued
`version` to `ERR_IMPORT_UNSUPPORTED_VERSION`.

Parsing scans the complete JSON token stream before model construction. It
rejects malformed or trailing JSON, invalid UTF-8, duplicate decoded keys at
any depth, decoded strings over 65,536 UTF-8 bytes, and container depth over
64. Escaped and literal spellings of the same object key are duplicates. The
limits are inclusive: depth 64 and strings of 65,536 bytes are accepted.
Bundles allow at most 512 profiles, and each profile allows at most 256 inputs.
Every profile and input must be an object, every profile must contain `inputs`,
and `inputs` must be an array. Container-valued `id` or `name` is invalid. A
bundle member doesn't need `id` or `name`. The canonical synthetic roots
`{"version":"synthetic-1","profiles":[]}` and `{"id":null,"inputs":[]}`
therefore pass structural validation.

Root selection repeats the decoder's key-presence classifier and requires its
result to match `Document.Kind`. Both predicates, neither predicate, an unknown
kind, a kind mismatch, invalid field types, malformed JSON, invalid UTF-8, and
duplicate keys return `ERR_IMPORT_ROOT`. Exact structural overflows return
`ERR_IMPORT_LIMIT_EXCEEDED`. An optional `version` may be absent or any scalar,
including null; an object or array version returns
`ERR_IMPORT_UNSUPPORTED_VERSION`. Scalar preservation is raw evidence only. It
doesn't admit that version or establish support for any Azeron Software
generation, and this boundary exposes no generation-admission API.

Validation follows this order: full-stream preflight, root classification and
kind agreement, root version representation, then profile and input shape and
count checks in input order. Within a profile, version is checked before
`id`/`name`, then `inputs`. The first preflight failure wins. Model-boundary
errors are `ParseError` values whose text is only their stable code. Every
failure returns the zero `RawExport`, never a partial root.

Macro payloads remain opaque under these structural bounds for Issue #20.
Issue #40 must establish privacy-safe grammar evidence before macro structure
is interpreted. Issue #42 must enforce the 1,000-step ceiling before semantic
admission; 1,000 steps are accepted and 1,001 are rejected. Issue #20 does not
apply that ceiling as a raw array-length check.

## Stable error codes

Decode errors are typed as `DecodeError`. Its `Code` is stable for machine use:

| Code | Stage meaning |
| --- | --- |
| `ERR_IMPORT_ENCODING` | The source can't be read as an accepted outer form, normalization wrapper, or Base64 encoding. It also wraps reader, file-open, and file-close failures. |
| `ERR_IMPORT_LZMA_HEADER` | A selected compressed candidate lacks a complete, structurally valid 13-byte LZMA-Alone header. |
| `ERR_IMPORT_LZMA_CORRUPT` | LZMA decoding is incomplete or corrupt, its known output size disagrees, or compressed trailing bytes remain. |
| `ERR_IMPORT_MSGPACK_TYPE` | The decompressed envelope tag isn't fixstr, str8, str16, or str32. |
| `ERR_IMPORT_LENGTH_MISMATCH` | The MessagePack String header is incomplete, or its declared length differs from all remaining bytes. |
| `ERR_IMPORT_UTF8` | The exact String payload isn't valid UTF-8. |
| `ERR_IMPORT_JSON` | The UTF-8 payload isn't one complete JSON value. |
| `ERR_IMPORT_ROOT` | The JSON root isn't exactly one provisional bundle or single shape. |
| `ERR_IMPORT_LIMIT_EXCEEDED` | The source, compressed input, dictionary, known output, or produced output exceeds its stage limit. |

## Downstream ownership

This boundary stops at owned JSON and provisional classification.

- Issue #20 owns the lossless raw schema/model, authoritative root
  revalidation, scalar version representation, structural validation, and
  opaque macro preservation.
- Issue #40 owns privacy-safe, version-bearing binding and macro grammar
  evidence. No binding map, macro grammar, or generation adapter is inferred
  here.
- Issue #42 owns adapter admission, semantic interpretation, and enforcement of
  the 1,000-step macro ceiling before interpretation.
- Issue #43 owns the production `import` and `validate` CLI surface.

The decoder doesn't persist imports, expose private exports, define raw model
fields, convert bindings, or provide production CLI usage. Export contents are
untrusted inert data, not instructions, and raw or partially decoded private
material must not enter documentation, fixtures, logs, or evidence.
