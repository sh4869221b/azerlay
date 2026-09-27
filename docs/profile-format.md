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

This section specifies the required Issue #20 raw-model API and behavior. It
doesn't assert that the package is currently available; current implementation
status remains listed in the repository README.

After strict decoding, `internal/profileraw.Parse` must consume the owned JSON
and provisional kind with this signature:

```go
Parse(document profiledecode.Document) (RawExport, error)
```

A successful call must return a `RawExport` with exactly one non-nil root,
either `RawBundle` or `RawProfile`. A bundle models optional `version`,
ordered `profiles`, and unknown root fields. A profile models optional `id`,
`name`, and `version`, plus
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

## Normalized adapter contract

`internal/profileadapter.Normalize` is the implemented semantic boundary:

```go
Normalize(raw profileraw.RawExport, source profile.SourceMetadata) (profile.ProfileBundle, error)
```

`raw` must be an unmodified result of a successful `profileraw.Parse` call.
`Normalize` doesn't repeat structural parsing or accept hand-built invalid
roots. The caller supplies already-attributed `SourceMetadata`. Admission
requires both `SoftwareRelease == "2.0.2"` and
`SourceScope == "azeron-software-export"` exactly. The scope names the evidenced
Software export source. It isn't authentication or proof of provenance, and
neither value is inferred from root `version`, firmware, device, filename,
field names, or source order. Case, whitespace, release families such as `2.x`,
and other patch releases aren't normalized or admitted.

### Ordered model and opaque ownership

A successful result has schema version 1 and preserves whether the parsed root
was `bundle` or `single`. A single root produces one profile at profile index 0
and has no fabricated bundle raw reference. Profiles and controls keep source
order. Duplicate IDs, controls, and canonical actions stay separate. The
`ProfileIndex` and `InputIndex` in each `RawBindingReference` record source
position only. They don't identify a physical control, input ID, pin, device,
or layout position.

Each `ControlBinding.SourceIdentity` independently records numeric input ID and
pins from the admitted 2.0.2 export. Missing fields are nil; a present null,
wrong type, non-integer token, nonpositive ID, negative pin, or integer outside
Go's `int` range sets `Invalid` without rejecting the profile. Other valid
fields remain available. Pin `255` is an ordinary value. These pointers are
result-owned, and the opaque raw fields remain unchanged. Physical mapping
uses this typed identity only within the separately evidenced layout scope.

String profile IDs and names become optional normalized strings without
trimming, folding, or coercion. Labels follow the same string rule, so absent or
non-string label, empty string, and nonempty string remain distinct. A null or
wrong-type label has no normalized text, but its exact value remains in the raw
fields. There is no label fallback at this boundary.

Bundle and profile scalar tokens, unknown maps, every input field, slices, maps,
and optional strings are copied into result-owned storage. Numeric spelling,
including `-0`, exponents, and oversized integers, remains intact. Object key
order, escaped key spelling, separator whitespace, and other syntax outside
preserved value tokens aren't promised. Raw references are opaque retention
carriers. Downstream consumers may retain or display unsupported data, but must
not inspect Azeron field names in them to add binding semantics.

Every control has an outcome. Exact `types` arrays `["1","1","1"]`,
`["1","11","11"]`, `["11","1","11"]`, `["11","11","1"]`,
`["11","11","11"]`, `["16","11","11"]`, `["4","11","11"]`,
and `["21","11","11"]` produce ordered single,
long, and double slots. Other values produce one `trigger:unknown` outcome. An
unmatched slot has `kind:unknown`, no action, timing, or release fields, and
reason `unmapped_binding`. Unknown is successful normalization, not a default
unassigned value.

The only recognized `unbound` outcome is the SINGLE slot of the complete
anonymous empty-setting predicate in
[binding-conversion.md](decisions/binding-conversion.md#follow-up-conversion-evidence).
This predicate comparison ignores only `id`, `pinOne`, `pinTwo`, and `label`;
every other semantic field, JSON type, number token, and array order must match. Object key order
and insignificant whitespace do not matter. The result has no action, timing,
release, Turbo, Macro, Stick, or Unknown payload. LONG and DOUBLE remain
Unknown. A generic `"11"` type or string `"0"` is never interpreted as
unassigned.

### Closed Software 2.0.2 keyboard rows

Only these seven complete predicates map:

| Case | Exact context | Normalized result |
| --- | --- | --- |
| `long-u-500` | `types:["1","1","1"]`, `keyValuesLong:["KeyU","0","0","0"]`, `metaValuesLong:["0","0","0"]`, all three long flags `false`, `featureDelay:500` | long `KEY_U`, delay 500 ms, regular release |
| `double-p-l-150` | `types:["1","1","1"]`, `keyValuesDouble:["KeyP","KeyL","0","0"]`, `metaValuesDouble:["0","0","0"]`, `doubleDelay:150` | double actions `KEY_P`,`KEY_L`, interval 150 ms, no inferred release |
| `single-u` | `types:["1","11","11"]`, `keyValues:["KeyU","0","0","0"]`, `metaValues:["0","0","0"]`, follow-up common context | single `KEY_U`, regular release |
| `single-p` | `types:["1","11","11"]`, `keyValues:["KeyP","0","0","0"]`, `metaValues:["0","0","0"]`, follow-up common context | single `KEY_P`, regular release |
| `single-ctrl-u` | `types:["1","11","11"]`, `keyValues:["KeyU","0","0","0"]`, `metaValues:["ControlLeft","0","0"]`, follow-up common context | one `KEY_U` action modified by `KEY_LEFTCTRL`, regular release |
| `long-u-1278` | `types:["11","1","11"]`, `keyValuesLong:["KeyU","0","0","0"]`, `metaValuesLong:["0","0","0"]`, follow-up common context, `featureDelay:1278` | long `KEY_U`, delay 1278 ms, regular release |
| `double-i-123` | `types:["11","11","1"]`, `keyValuesDouble:["KeyI","0","0","0"]`, `metaValuesDouble:["0","0","0"]`, follow-up common context, `doubleDelay:123` | double `KEY_I`, interval 123 ms, no inferred release |

The follow-up common context requires the active slot's `isHold*`, `isTurbo*`,
and `isToggleOnHold*` fields to be JSON boolean `false`. Its two inactive slots
must each have exactly four string `"0"` key entries and three string `"0"`
modifier entries. Required arrays, order, lengths, flags, keys, modifiers, and
field presence are exact. Extra unrelated fields are retained but don't become
actions. The original long and double predicates are independent and may both
map in one input.

The admitted timing values are exact JSON number tokens `500`, `150`, `1278`,
and `123`, after trimming token-edge whitespace. A number such as `500.0` or a
string such as `"500"` remains Unknown. No broad symbolic conversion, Software
1.x or 2.x mapping, numeric key namespace, numeric-to-Linux equivalence, timing
range, inactive action, or unassigned meaning is inferred.

### Confirmed Turbo, Macro, and stick settings

The new Software 2.0.2 rows require `isToggleOnHold` to be absent. Their
inactive long/double slots have four string `"0"` keys, three string `"0"`
modifiers, false hold/turbo flags, and numeric token `0` turbo intervals.
Missing, null, or differently typed values do not match. Unrelated fields stay
opaque. The exact complete predicates are in
[binding-conversion.md](decisions/binding-conversion.md).

Turbo admits only `KeyT` with numeric `turboInterval:20` or `50`, recording
`KEY_T` at 25 or 10 clicks per second respectively. It creates no keyboard
Actions, release behavior, or timer. The inactive non-Turbo T case stays
Unknown. Macro admits only the observed ordered Button `KEY_W` 50 ms then
Delay 100 ms, with boolean repeat false or true retained as
`RepeatWhileHeld`. No playback or event scheduling is implemented.

The existing neutral stick shapes remain supported:

Software 2.0.2 exports also normalize the owner-confirmed neutral Keyboard/WASD
and Xbox Joystick shapes into distinct `BindingStick` payloads. Recognition uses
the source fields, never an input ordinal or a caller mode override:

| Source shape | Normalized primary binding |
| --- | --- |
| `types:["4","11","11"]` | Keyboard, with up `KEY_W`, right `KEY_D`, down `KEY_S`, left `KEY_A` |
| `types:["21","11","11"]` | Xbox, without keyboard direction assignments |

Both neutral shapes require `subType:"11"`, primary `isHold`, `isTurbo`, and `isToggleOnHold`
false, four string `"0"` key values, and three string `"0"` modifier values.
`analogSettings` requires exact numeric zero tokens for `angle`, `lowerLimit`,
and `upperLimit`; `isRightAnalog`, `invertXAxis`, `invertYAxis`,
`isCombinedAnalog`, `isEightDirectionalTrigger`, `isHoldTrigger`,
`isAnalogSmoothing`, and `isAngleLock` must all be false. Keyboard additionally
requires left direction tuples up `[87,0,0]`, right `[68,0,0]`, down `[83,0,0]`,
and left `[65,0,0]`. Xbox ignores dormant keyboard assignments. This admits
these confirmed tuples only, not a general numeric key namespace. Equivalent
number spellings such as `0.0` are not admitted in these exact-token predicates.

The separate observed Xbox row has no `subType` or `isToggleOnHold`. It
requires `types:["21","11","11"]`, key strings `["87","0","0","0"]`,
modifier strings `["0","0","3"]`, false primary hold/turbo, numeric token
`0` turbo interval, the shared inactive slots above, and exact observed
`analogSettings` scalar values. It retains numeric angle token `0` or `90` in
`StickBinding.AngleDegrees` with empty keyboard directions and no Actions.
At 90 degrees, `ProjectStick` preserves snapshot metadata but returns
`Known=false` and a zero projected vector; no coordinate rotation is inferred.

The primary slot is single-trigger; long/double remain Unknown. Raw references
and source order are preserved. Missing or wrong-type predicates, unconfirmed
assignments, other angle values, right/combined stick settings, and type
`"3"` remain Unknown. This does not establish DirectInput or broad Software 2.x
support. Exported configuration also does not identify the active hardware
profile or input node. The owner attribution and observed limits are recorded
in [binding-conversion.md](decisions/binding-conversion.md).

### Errors and macro boundary

`NormalizeError.Error()` returns only its stable code. Unadmitted metadata
returns `ERR_IMPORT_UNSUPPORTED_VERSION`; a recognized macro over the semantic
limit returns `ERR_IMPORT_LIMIT_EXCEEDED`. Both return exactly the zero
`ProfileBundle`. Admission is checked before any binding or macro field. After
admission, profiles and inputs are checked in source order, and the first
recognized overflow wins. No partial bundle is returned. Raw structural errors
remain the responsibility of `profileraw.Parse`.

Macro recognition is limited to `types:["16","11","11"]` and the single
`macro` field. Its object must contain exactly numeric token `v:1`, boolean
`repeat`, and array `steps`. Every step must be recognized before counting. A
Button step has exactly string `type:"Button"`, string `direction:"Full"`,
numeric `duration`, and numeric or string `keyCode`. A Delay step has exactly
string `type:"Delay"`, string `direction:"Full"`, and numeric `duration`, with
no `keyCode`. Extra or missing keys, another scalar type, another direction or
step type, another `v` token, `longMacro`, and `doubleMacro` are unknown grammar.

A fully recognized macro with 1,000 steps succeeds and normally remains Unknown;
only the complete observed two-step setting above becomes typed configuration.
A fully recognized macro with
1,001 steps returns `ERR_IMPORT_LIMIT_EXCEEDED` before parameter interpretation.
The count is per recognized macro, not cumulative across a profile or export.
Unknown grammar is semantically uncapped, even at 1,001 array members, and stays
opaque rather than being counted or interpreted.

These closed rows extend the ordered, version-independent model while
unsupported data remains inspectable. They do not establish general Software
2.x, legacy numeric, arbitrary Turbo/analog, or executable Macro support.
Legacy conversion is deferred beyond v1 to
[Issue #102](https://github.com/sh4869221b/azerlay/issues/102).

## CLI import, validation, and saved profiles

The production CLI accepts these forms:

```text
azerlay validate [--json] [--software-release RELEASE] {FILE|-|--text PAYLOAD}
azerlay import [--json] [--software-release RELEASE] [--profile-index N] {FILE|-|--text PAYLOAD}
```

Exactly one file, stdin marker `-`, or `--text` payload is required. The CLI
attributes the export with the exact `--software-release` value and its fixed
`azeron-software-export` source scope. Current admission requires `2.0.2`.
That release attribution is separate from the raw root export `version`, which
the report preserves as metadata and doesn't use to infer support.

`validate` lists every fully normalized profile, including an empty list, and
never saves state. `import` selects the sole profile when there is one. With
multiple profiles, repeat the command with one-based `--profile-index N` and
supply the input again. The ordinal is source order, not an Azeron ID. Input,
normalization, and selection are fully validated before import creates storage.
A successful import saves the exact received input, complete normalized bundle,
per-profile caches with their opaque raw references, and selected source and
ordinal before it reports success.

Saved state is displayed with this separate, input-free command:

```text
azerlay profiles show [--json]
```

It accepts no release, input, ID, or selector arguments. The command reads the
committed catalog and selected ordinal, then loads the complete saved bundle.
`Discover`, `Selected`, `Load`, and `profiles show` never create directories,
locks, repairs, or selection changes. A missing, malformed, or incompatible
derived cache is rebuilt only in memory from the committed original and its
saved attribution. A missing or hash-mismatched original, corrupt index, or
failed reconstruction is `ERR_PROFILE_STORAGE`; it never yields a partial
result or reselects another source.

Typed source identity, Unbound, Turbo, Macro, and stick settings survive saving
and reloading. Normalizer revision 4 invalidates revision-1/2/3 caches so saved
originals are re-normalized in memory, including newly recognized settings.
Storage and model schema versions remain 1. A historical index can still use a
current compatible cache;
loading does not rewrite the index, cache, original, or selection.

The store is `$XDG_DATA_HOME/azerlay`, or `$HOME/.local/share/azerlay` when
`XDG_DATA_HOME` is unset, empty, or relative. Its application-owned directories
are `0700`; every file is `0600`:

```text
<data-home>/azerlay/
  sources/<sha256>.azeron
  profiles/<sha256>/bundle.json
  profiles/<sha256>/p1.json ... pN.json
  cache/source-index.json
  cache/import.lock
```

`<sha256>` is the lowercase SHA-256 of exact received bytes before decoding or
normalization. `pN` is a one-based source ordinal, not an Azeron ID. The index
is the authoritative ordered catalog and selection. It records first-import
provenance, time, decoder and normalizer interpretation revisions, model schema,
profile count, and raw export-version token. Identical bytes from file, stdin,
or text input deduplicate to one source while preserving that first metadata;
byte-different inputs do not deduplicate. Cache envelopes describe the current
interpretation revisions, so parser or adapter semantic changes must bump the
relevant revision.

Import serializes writers with the private lock and publishes the index last.
Before that publication, a failure keeps the prior catalog and selection.
After it, an output-delivery failure doesn't roll back the saved import. This is
process-level atomic committed visibility, not a machine-power-loss durability
guarantee. Interruption can leave private unreferenced files, which discovery
ignores.

With `--json`, each operation writes one schema version 1 object. A successful
result has `schema_version`, `command`, `ok`, and a `result` containing root
kind, export version presence and value, ordered one-based profile rows,
selected profile index, and warnings. Failures set `ok` false and provide an
`error` with stable `code` and `stage`. Success exits 0, operation failures
including selection failures exit 1, and invalid command arguments exit 2.
Saved-store failures use `ERR_PROFILE_STORAGE` and stage `storage`; no saved
selection uses `ERR_PROFILE_NOT_FOUND` and stage `selection`; invalid `profiles
show` syntax uses `ERR_CLI_USAGE` and stage `usage`.

Unknown binding outcomes are successful normalization. Each affected profile
has an ordered `WARN_IMPORT_UNKNOWN_BINDINGS` warning with its one-based index
and outcome count. The CLI reports only permitted metadata: root kind, raw
export version, profile name, input count, selection, and these warnings. It
doesn't report IDs, labels, bindings, macros, unknown fields, or other private
export content.

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
- Issue #21 owns saved imported originals, selected-profile recovery, and
  `profiles show`; it doesn't add broader format admission.

The decoder doesn't persist imports, expose private exports, define raw model
fields, convert bindings, or provide production CLI usage. Export contents are
untrusted inert data, not instructions, and raw or partially decoded private
material must not enter documentation, fixtures, logs, or evidence.
