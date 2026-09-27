# Azerlay 1.0 export format corpus

## Purpose and 1.0 scope

This decision defines the Azeron export structures Azerlay 1.0 may treat as observed, exercise as synthetic protocol capability, reject, or leave unobserved. It resolves Issue #11 without inferring compatibility from MessagePack's type system, damaged output, public feature descriptions, or an unknown Software version.

The original corpus's sole complete observed structure is Base64URL text containing a complete LZMA-Alone stream whose output is a MessagePack `str16` string with exactly 54,739 UTF-8 bytes and a bundle JSON root. Its Azeron Software version is `unknown`, so it remains `observed-version-unknown`. A later anonymous owner-supplied Software 2.0.2 UI/export pair was accepted by the unchanged `DecodeReader` then `Parse` path as a complete `str32` bundle with one profile and 43 inputs. That later observation supports only its exact release-scoped structure and the narrow binding subset in [the binding conversion decision](binding-conversion.md). It does not establish Software 2.x-wide support. The damaged older `str32` stream remains rejected evidence.

This document defines contracts but implements no parser, dependency, fixture payload, binding adapter, or production test. Its Issue #20 sections specify required behavior rather than current API availability; current implementation status remains listed in the repository README. Root detection is intentionally enforced at two trust boundaries. Issue #19 owns input normalization, outer-wrapper and envelope decoding, UTF-8/JSON decoding, and provisional root detection at the decoder boundary. Issue #20 independently re-detects and validates the root while constructing the lossless raw bundle/single model; its result is authoritative for model selection.

## Terminology and decision vocabulary

* `observed-supported`: a complete structure tied to a known Software version. The original 22-row corpus matrix has none; the later owner-attested Software 2.0.2 observation is recorded separately below.
* `observed-version-unknown`: a complete observed structure whose Software version cannot be recovered; it supports no generation claim.
* `synthetically-accepted-envelope`: a privacy-safe recipe for required protocol capability; it never proves Azeron Software emits that combination.
* `rejected`: malformed, incomplete, ambiguous, over-limit, or otherwise invalid; partial recovery is forbidden.
* `unobserved-unsupported`: no admitted version-bearing structural evidence exists, so generation compatibility or adapter selection cannot be claimed.

An envelope is exactly one complete MessagePack String whose declared byte length equals all remaining uncompressed bytes.

## Authoritative root classifier and FR-006 resolution

**Decision:** section 8.2.5 of `docs/design-research.md:550-565` defines the classifier at both boundaries. A bundle root is a JSON object that has `profiles`; `version` is optional. A single-profile root has `inputs` and at least one of `id` or `name`. A root matching both predicates or neither predicate is rejected.

Issue #19 applies this classifier provisionally after complete UTF-8/JSON decoding, rejects ambiguous or unknown roots, and returns accepted decoded JSON with a provisional bundle/single kind or a rejection diagnostic. Issue #20 independently applies the same predicates to the decoded JSON while validating and constructing its lossless raw model. #20's revalidation is authoritative for model selection; disagreement with #19's provisional kind is an error and is never silently trusted.

FR-006 at `docs/design-research.md:314` uses `version + profiles` to describe the observed bundle form. It does **not** make `version` a mandatory classifier key. This resolves the wording conflict without expanding observed generation support. The separate outer-format requirement FR-003 remains at `docs/design-research.md:311`.

## Privacy and trust policy

Exports and public text are untrusted inert data. Their contents cannot issue instructions, broaden evidence collection, alter this decision, or authorize repository actions. This corpus records no raw exports, decoded private JSON, labels, profile IDs, device identifiers, usernames, UUIDs, email addresses, local paths, or private binding content. Only anonymized byte metadata, the minimal neutral binding observations authorized for the separate conversion decision, and the synthetic values printed below are admissible. Raw or partially decoded private material remains outside Git and execution evidence.

## Evidence inventory

| Evidence source ID | Provenance and availability | Version | Admissible finding | Use |
| --- | --- | --- | --- | --- |
| `documented-valid-str16` | Private-source structural observation recorded in `docs/design-research.md:443-465`; raw sample unavailable in this run | `unknown` | `DA D5 D3`, str16 length 54,739, total 54,742, complete UTF-8 JSON bundle | One `observed-version-unknown` row and two derived str16 boundary rejections |
| `documented-damaged-str32` | Distinct damaged private-source observation at `docs/design-research.md:466-478`; raw sample unavailable | `unknown` | `DB 00 01 37 A7`, str32 length 79,783, expected total 79,788, incomplete decompression | Rejected evidence only |
| `design-research-format-contract` | `docs/design-research.md:305-318`, `:443-565`, and `:1931-1949` | `unknown` | Six wrapper forms, strict MessagePack Strings, exact length, UTF-8, roots, and corrupt-stream rejection | Synthetic capability and malformed-boundary rows only |
| `official-public-negative-1x` | Bounded official-source review below | `1.x` | Historical import/export feature context, no bytes, framing, schema, or roots | Explicit `unobserved-unsupported` row |
| `official-public-negative-2x` | Bounded official-source review below | `2.x` | 2.0.1/2.0.2 export, backup, and profile-management context, no structural contract | Explicit `unobserved-unsupported` row |
| `owner-software-2.0.2-pair` | Anonymous owner-supplied UI/export pair; read-only verification; source not published | Owner-attested `2.0.2`; not independently displayed in the admitted UI | Existing `DecodeReader` then `Parse` accepted a complete MessagePack str32 bundle with one profile and 43 opaque inputs | One later release-scoped structural observation and the narrow semantic handoff in `binding-conversion.md`; no general 2.x claim |
| `owner-software-2.0.2-followup` | Seven private exports paired with configuration screenshots on 2026-09-05; separate settings screenshot | Displayed Software `2.0.2`, Firmware `111`; firmware is not release evidence | Unpadded Base64URL, complete LZMA-Alone, exact-length MessagePack str32, JSON bundle with one profile and 43 inputs in all seven cases | In-memory inspection using `xz` and JSON parsing, not a production `DecodeReader`/`Parse` run; exact binding and macro-structure observations in `binding-conversion.md` |
| `owner-software-2.0.2-stick-pair` | Two private exports supplied for Issue #44 on 2026-09-20, labelled Keyboard and joystick; owner subsequently confirmed release, Xbox Joystick UI name, and W/D/S/A directions | Owner-attested `2.0.2`; no independently inspected settings screenshot for this pair | Both are unpadded Base64URL, complete LZMA-Alone, exact-length MessagePack str32 JSON bundles with one profile and 43 inputs | Planning inspection used Python `base64`/`lzma`/`json` in memory, not production `DecodeReader`/`Parse`; exact neutral Keyboard and Xbox contexts are admitted in `binding-conversion.md` |
| `owner-software-2.0.2-turbo-20260926` | Owner UI/export comparison supplied 2026-09-26; private pair retained by owner | UI displays Software `2.0.2` | T Turbo enabled; only `isTurbo` and `turboInterval` differ on activation, and only `turboInterval` differs between displayed 25 and 10 clicks/second | Exact contextual settings only, in `binding-conversion.md`; private exports/images are absent, production parsing and physical Turbo behavior were not verified |
| `owner-software-2.0.2-macro-20260926` | Owner UI/export comparison supplied 2026-09-26; private pair retained by owner | UI displays Software `2.0.2` | One Button W 50 ms followed by Delay 100 ms; repeat comparison changes only `macro.repeat` | Minimum anonymous observations only, in `binding-conversion.md`; no full macro copy, production parsing, or playback verification |
| `owner-software-2.0.2-angle-20260926` | Owner UI/export comparison supplied 2026-09-26; private pair retained by owner | UI displays Software `2.0.2` | Xbox Joystick angle 0 versus 90 degrees; only `analogSettings.angle` differs | Exact contextual settings only, in `binding-conversion.md`; private exports/images are absent, production parsing and physical axis behavior were not verified |

The follow-up str32 UTF-8 payload lengths, in supplied order, are 90,074,
90,076, 90,076, 90,086, 90,077, 90,077, and 90,260 bytes; each decompressed
stream is exactly five bytes longer. Consecutive JSON changes are confined
to one input. The settings screenshot now independently displays the Software
release for this follow-up; it does not retroactively identify older
version-unknown exports. These seven observations sit outside the fixed
22-row matrix and do not change its counts.

The later stick pair has compressed lengths of 1,753 bytes (Keyboard) and
1,752 bytes (Xbox Joystick), and str32 UTF-8 JSON lengths of 90,260 and 90,261
bytes respectively. Planning inspection checked decompressor EOF, absence of
trailing compressed data, and exact string byte lengths. Recursive JSON
comparison found exactly one changed value: input index 23's `types[0]` is
string `"4"` versus string `"21"`. This index is an observation only, never
a conversion or physical-control rule. The full tuples are
`["4","11","11"]` and `["21","11","11"]`, with `subType:"11"` and neutral
left-stick settings. The owner confirmed the Keyboard left-direction tuples
`up:[87,0,0]`, `right:[68,0,0]`, `down:[83,0,0]`, `left:[65,0,0]` as W/D/S/A.
The complete conversion predicates and tiny synthetic fixtures are in
[the binding conversion decision](binding-conversion.md#issue-44-stick-conversion-evidence).
This pair also sits outside the fixed 22-row matrix. It provides no production
decoder run against these private payloads, no additional export form, no
generic numeric key namespace, and no live Xbox-device validation.

The owner source was inspected only for the minimum neutral structural and binding observations authorized for publication. Its raw export, images, labels, IDs, and complete macro content remain private. Missing facts are literal `unknown`, never inferred. Filenames, device data, firmware, and export `version` values do not establish the Software release.

## Auditable official-source review

Retrieval date was `2026-09-02`. Each direct URL returned `HTTP 200 / curl exit 0`. These first-party pages are **contextual/negative only**: transport success, titles, release labels, feature prose, and `.json` filename advice do not establish accepted bytes, framing, schema, root keys, or Software-version support.

| Exact URL | Observed page title | Bounded claim and result |
| --- | --- | --- |
| `https://azeron.com/pages/downloads` | `Azeron Downloads - Software, Manuals & Quick Guides` | `Azeron Software v2.0.2` and `Import/export profiles`; no structural evidence |
| `https://azeron.com/pages/software` | `Azeron Software 2.0 \| Free Gaming Keypad Software \| Windows & Linux` | 2.0.1 migration FAQ recommends a `.json` backup; extension context only |
| `https://azeron.com/pages/software-changelog-2-0-1` | `Software Changelog 2.0.1 – Azeron` | 2.0.1 hotfix mentions profile-management bugs; no export representation |
| `https://azeron.com/pages/software-changelog-2-0-2` | `Software Changelog 2.0.1 – Azeron` | 2.0.2 hotfix prose mentions profile monitoring, macro execution, and stability; observed HTML title itself says 2.0.1; no export representation |
| `https://www.azeron.eu/update-history/` | `Azeron - Update History` | Single-profile import/export plus v1.2.5/v0.7.0 behavior context; no bytes, framing, schema, or root keys |

The two exact searches were `site:azeron.com Azeron Software export profile format` and `site:azeron.eu Azeron Software export profile format`. Each returned `HTTP 200 / curl exit 0` but only an enable-JavaScript retry page, with no attributable result listing. The bounded result is negative: these exact five pages and two searches yielded no version-bearing structural evidence. It does not prove absence outside this fixed source set.

## Azerlay 1.0 support matrix

The 22 rows below reproduce Todo 4 JSON exactly. They include all six Issue #19 input forms: Raw JSON, raw LZMA-Alone, Base64URL padded and unpadded, and standard Base64 padded and unpadded. Existing positive MessagePack rows use Base64URL unpadded; the five named canonical-bundle rows cover the other forms.

| `id` | `evidenceSourceId` | `softwareVersion` | `outerEncoding` | `lzmaFormat` | `messagePackTag` | `declaredLength` | `jsonRoot` | `evidenceStrength` | `decision` | `downstreamImplication` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `observed-str16-bundle-version-unknown` | `documented-valid-str16` | `unknown` | Base64URL text | LZMA-Alone; complete documented decode; raw header fields and dictionary unknown | 0xDA str16 with two-byte big-endian length | 54739 | bundle object with profiles; version optional | documented structural observation; raw reproduction unavailable | `observed-version-unknown` | Accept only as the documented version-unknown shape; do not attribute it to Software 1.x or 2.x. |
| `rejected-damaged-str32` | `documented-damaged-str32` | `unknown` | Base64URL text | damaged LZMA-Alone stream; expected uncompressed size 79788; complete output unavailable | 0xDB str32 with four-byte big-endian length | 79783 | unknown | documented damaged observation; raw reproduction unavailable | `rejected` | Reject incomplete decompression; never recover partial JSON or infer support from the observed header. |
| `rejected-str16-trailing-byte` | `documented-valid-str16` | `unknown` | Base64URL text | synthetic exact-length boundary derived from the documented complete LZMA-Alone observation | 0xDA str16 declaring 54738 bytes with 54739 bytes remaining | 54738 | not evaluated | deterministic in-memory malformed-boundary check from Todo 2 | `rejected` | Reject before UTF-8 or JSON parsing because one trailing byte violates exact length equality. |
| `rejected-str16-short-payload` | `documented-valid-str16` | `unknown` | Base64URL text | synthetic exact-length boundary derived from the documented complete LZMA-Alone observation | 0xDA str16 declaring 54740 bytes with 54739 bytes remaining | 54740 | not evaluated | deterministic in-memory malformed-boundary check from Todo 2 | `rejected` | Reject before UTF-8 or JSON parsing because the payload is shorter than its declaration. |
| `rejected-str8-trailing-byte` | `design-research-format-contract` | `unknown` | synthetic Base64URL text without padding | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 declaring 38 bytes with 39 bytes remaining | 38 | not evaluated | deterministic privacy-safe malformed str8 recipe | `rejected` | Reject before UTF-8 or JSON parsing because one trailing byte violates exact length equality. |
| `rejected-str8-short-payload` | `design-research-format-contract` | `unknown` | synthetic Base64URL text without padding | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 declaring 40 bytes with 39 bytes remaining | 40 | not evaluated | deterministic privacy-safe malformed str8 recipe | `rejected` | Reject before UTF-8 or JSON parsing because the payload is shorter than its declaration. |
| `rejected-lzma-corrupt` | `design-research-format-contract` | `unknown` | synthetic Base64URL text without padding | truncated valid synthetic LZMA-Alone stream after its valid 13-byte header | not reached; valid str8 bytes must not be partially adopted | unknown | not evaluated | deterministic privacy-safe corrupt-stream recipe required by docs/design-research.md:1931-1949 | `rejected` | Return ERR_IMPORT_LZMA_CORRUPT and expose no partial JSON. |
| `synthetic-fixstr-bundle` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xA0..0xBF fixstr | unknown | synthetic bundle object with profiles; version optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict fixstr and bundle decoding without claiming any Software generation emits it. |
| `synthetic-fixstr-single` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xA0..0xBF fixstr | unknown | synthetic single-profile object with inputs and id or name | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict fixstr and single-profile decoding without claiming any Software generation emits it. |
| `synthetic-str8-bundle` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | unknown | synthetic bundle object with profiles; version optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str8 and bundle decoding without claiming any Software generation emits it. |
| `synthetic-str8-single` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | unknown | synthetic single-profile object with inputs and id or name | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str8 and single-profile decoding without claiming any Software generation emits it. |
| `synthetic-str16-bundle` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xDA str16 with two-byte big-endian length | unknown | synthetic bundle object with profiles; version optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str16 and bundle decoding separately from the version-unknown observed sample. |
| `synthetic-str16-single` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xDA str16 with two-byte big-endian length | unknown | synthetic single-profile object with inputs and id or name | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str16 and single-profile decoding without converting protocol coverage to compatibility. |
| `synthetic-str32-bundle` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xDB str32 with four-byte big-endian length | unknown | synthetic bundle object with profiles; version optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str32 and bundle decoding without treating the damaged observation as support. |
| `synthetic-str32-single` | `design-research-format-contract` | `unknown` | synthetic Base64URL text | synthetic complete LZMA-Alone stream within documented limits | 0xDB str32 with four-byte big-endian length | unknown | synthetic single-profile object with inputs and id or name | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Test strict str32 and single-profile decoding without treating the damaged observation as support. |
| `synthetic-raw-json-bundle` | `design-research-format-contract` | `unknown` | synthetic Raw JSON bytes | not applicable; Raw JSON bypasses compressed-envelope stages | not applicable; Raw JSON bypasses MessagePack envelope | unknown | synthetic canonical bundle object with profiles and observed-form version field; classifier requires profiles and treats version as optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Issue #19 detects Raw JSON and supplies the complete JSON bytes to Issue #20 without claiming Azeron generation provenance. |
| `synthetic-raw-lzma-bundle` | `design-research-format-contract` | `unknown` | synthetic raw LZMA-Alone bytes | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | 39 | synthetic canonical bundle object with profiles and observed-form version field; classifier requires profiles and treats version as optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Issue #19 detects raw LZMA-Alone and strictly decodes its MessagePack String envelope without claiming Azeron generation provenance. |
| `synthetic-base64url-padded-bundle` | `design-research-format-contract` | `unknown` | synthetic Base64URL text with padding | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | 39 | synthetic canonical bundle object with profiles and observed-form version field; classifier requires profiles and treats version as optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Issue #19 strictly detects and decodes padded Base64URL input without claiming Azeron generation provenance. |
| `synthetic-base64-standard-padded-bundle` | `design-research-format-contract` | `unknown` | synthetic standard Base64 text with padding | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | 39 | synthetic canonical bundle object with profiles and observed-form version field; classifier requires profiles and treats version as optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Issue #19 strictly detects and decodes padded standard Base64 input without claiming Azeron generation provenance. |
| `synthetic-base64-standard-unpadded-bundle` | `design-research-format-contract` | `unknown` | synthetic standard Base64 text without padding | synthetic complete LZMA-Alone stream within documented limits | 0xD9 str8 with one-byte length | 39 | synthetic canonical bundle object with profiles and observed-form version field; classifier requires profiles and treats version as optional | normative protocol requirement and synthetic coverage only; not observed Azeron evidence | `synthetically-accepted-envelope` | Issue #19 strictly detects and decodes unpadded standard Base64 input without claiming Azeron generation provenance. |
| `unobserved-software-1x` | `official-public-negative-1x` | `1.x` | unknown | unknown | unknown | unknown | unknown | bounded official-source wave found versioned export behavior context but no accepted bytes or root structure | `unobserved-unsupported` | Do not claim Software 1.x compatibility until version-bearing structural evidence is admitted. |
| `unobserved-software-2x` | `official-public-negative-2x` | `2.x` | unknown | unknown | unknown | unknown | unknown | bounded official-source wave found 2.0.1/2.0.2 export context but no accepted bytes or root structure | `unobserved-unsupported` | Do not claim Software 2.x compatibility until version-bearing structural evidence is admitted. |

The original matrix remains 22 rows: zero `observed-supported`, one `observed-version-unknown`, thirteen `synthetically-accepted-envelope`, six `rejected`, and two `unobserved-unsupported`. Those rows and their synthetic status are unchanged. The later Software 2.0.2 observation is deliberately outside that fixed matrix because its available evidence establishes an owner-attested release subset, not a replacement for the original wrapper corpus or support for the broad `2.x` row. In the reproduced row wording, Issue #19 supplies complete JSON bytes only after provisional root detection; Issue #20 then independently re-detects the root rather than trusting that provisional kind.

## Rejected and unobserved cases

1. Strictly distinguish Raw JSON, raw LZMA-Alone, Base64URL padded/unpadded, and standard Base64 padded/unpadded. Reject characters outside the selected alphabet and do not silently repair anything except allowed missing padding.
2. Reject invalid, incomplete, or over-limit LZMA-Alone. Caps are 8 MiB compressed input, 64 MiB dictionary, and 64 MiB complete output. `lzma-corrupt` preserves a valid synthetic 13-byte LZMA-Alone header then truncates the stream; it must produce `ERR_IMPORT_LZMA_CORRUPT` and no partial JSON.
3. Accept only MessagePack fixstr, str8, str16, and str32 String envelopes. Do not reinterpret Map or Binary.
4. Require `declared_string_length == remaining_uncompressed_bytes`; reject trailing bytes, short payloads, and overflow before UTF-8/JSON.
5. Reject invalid UTF-8, invalid JSON, ambiguous roots, and unsupported roots.
6. Never scan for the first `{`, accept partial decompression, or save a partial profile.

The documented damaged str32 stream remains rejected evidence-only. Synthetic str32 capability cannot rehabilitate it. Synthetic wrapper success carries no Software-generation provenance.

## Synthetic fixture recipes

These 22 specifications reproduce Todo 5 JSON exactly; no encoded or compressed payload is stored. The canonical bundle is `{"version":"synthetic-1","profiles":[]}`, and the canonical single is `{"id":"synthetic","name":"Synthetic","inputs":[]}`. Unless a row says otherwise, UTF-8 encode `jsonInput`, add its MessagePack String header, LZMA `FORMAT_ALONE` compress within limits, then apply the named outer encoding. Reverse each stage strictly and stop at the named failure boundary.

| `id` | `matrixRowId` | `purpose` | `jsonInput` | `outerEncoding` | `lzmaFormat` | `messagePackTag` | `declaredLengthRule` | `mutation` | `expectedClassification` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `fixstr-bundle` | `synthetic-fixstr-bundle` | Exercise the accepted fixstr bundle envelope with a minimal synthetic root. | `{"profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | fixstr 0xAF; low five bits declare 15 bytes | Low five tag bits equal all 15 remaining uncompressed bytes. | None; UTF-8 encode jsonInput, prefix 0xAF, LZMA-Alone compress, and Base64URL encode. | `synthetically-accepted-envelope` |
| `fixstr-single` | `synthetic-fixstr-single` | Exercise the accepted fixstr single-profile envelope with a minimal synthetic root. | `{"id":"s","inputs":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | fixstr 0xB6; low five bits declare 22 bytes | Low five tag bits equal all 22 remaining uncompressed bytes. | None; UTF-8 encode jsonInput, prefix 0xB6, LZMA-Alone compress, and Base64URL encode. | `synthetically-accepted-envelope` |
| `str8-bundle` | `synthetic-str8-bundle` | Exercise str8 bundle decoding with the exact canonical positive bundle input. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The byte after 0xD9 equals all 39 remaining uncompressed bytes. | None; prefix UTF-8 jsonInput with bytes 0xD9, 0x27 before compression and encoding. | `synthetically-accepted-envelope` |
| `str8-single` | `synthetic-str8-single` | Exercise str8 single-profile decoding with the exact canonical positive single input. | `{"id":"synthetic","name":"Synthetic","inputs":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The byte after 0xD9 equals all 49 remaining uncompressed bytes. | None; prefix UTF-8 jsonInput with bytes 0xD9, 0x31 before compression and encoding. | `synthetically-accepted-envelope` |
| `str16-bundle` | `synthetic-str16-bundle` | Exercise str16 bundle decoding independently of observed Software provenance. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str16 0xDA plus two-byte big-endian length | The uint16 after 0xDA equals all 39 remaining uncompressed bytes. | None; use explicit bytes 0xDA, 0x00, 0x27 even though this synthetic payload fits str8. | `synthetically-accepted-envelope` |
| `str16-single` | `synthetic-str16-single` | Exercise str16 single-profile decoding independently of observed Software provenance. | `{"id":"synthetic","name":"Synthetic","inputs":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str16 0xDA plus two-byte big-endian length | The uint16 after 0xDA equals all 49 remaining uncompressed bytes. | None; use explicit bytes 0xDA, 0x00, 0x31 even though this synthetic payload fits str8. | `synthetically-accepted-envelope` |
| `str32-bundle` | `synthetic-str32-bundle` | Exercise str32 bundle decoding without adopting the documented damaged stream. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str32 0xDB plus four-byte big-endian length | The uint32 after 0xDB equals all 39 remaining uncompressed bytes. | None; use explicit bytes 0xDB, 0x00, 0x00, 0x00, 0x27 before compression and encoding. | `synthetically-accepted-envelope` |
| `str32-single` | `synthetic-str32-single` | Exercise str32 single-profile decoding without adopting the documented damaged stream. | `{"id":"synthetic","name":"Synthetic","inputs":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str32 0xDB plus four-byte big-endian length | The uint32 after 0xDB equals all 49 remaining uncompressed bytes. | None; use explicit bytes 0xDB, 0x00, 0x00, 0x00, 0x31 before compression and encoding. | `synthetically-accepted-envelope` |
| `raw-json-bundle` | `synthetic-raw-json-bundle` | Exercise Issue #19 Raw JSON detection and Issue #20 bundle-root parsing with the canonical synthetic bundle. | `{"version":"synthetic-1","profiles":[]}` | Raw JSON bytes | Not applicable; do not compress jsonInput. | Not applicable; do not add a MessagePack header. | Not applicable; pass the complete UTF-8 JSON bytes after strict Raw JSON detection. | UTF-8 encode jsonInput exactly and pass it directly; do not apply LZMA, MessagePack, or Base64. | `synthetically-accepted-envelope` |
| `raw-lzma-bundle` | `synthetic-raw-lzma-bundle` | Exercise Issue #19 raw LZMA-Alone detection and strict envelope decoding with the canonical synthetic bundle. | `{"version":"synthetic-1","profiles":[]}` | Raw LZMA-Alone bytes | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The str8 declaration equals all 39 remaining bytes before JSON parsing. | Prefix UTF-8 jsonInput with bytes 0xD9, 0x27 and LZMA-Alone compress; do not Base64 encode. | `synthetically-accepted-envelope` |
| `base64url-padded-bundle` | `synthetic-base64url-padded-bundle` | Exercise Issue #19 padded Base64URL detection and strict envelope decoding with the canonical synthetic bundle. | `{"version":"synthetic-1","profiles":[]}` | Base64URL with padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The str8 declaration equals all 39 remaining bytes before JSON parsing. | Construct the valid compressed str8 bundle and Base64URL encode while retaining required = padding. | `synthetically-accepted-envelope` |
| `base64-standard-padded-bundle` | `synthetic-base64-standard-padded-bundle` | Exercise Issue #19 padded standard Base64 detection and strict envelope decoding with the canonical synthetic bundle. | `{"version":"synthetic-1","profiles":[]}` | Standard Base64 with padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The str8 declaration equals all 39 remaining bytes before JSON parsing. | Construct the valid compressed str8 bundle and standard Base64 encode while retaining required = padding. | `synthetically-accepted-envelope` |
| `base64-standard-unpadded-bundle` | `synthetic-base64-standard-unpadded-bundle` | Exercise Issue #19 unpadded standard Base64 detection and strict envelope decoding with the canonical synthetic bundle. | `{"version":"synthetic-1","profiles":[]}` | Standard Base64 without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one unsigned-byte length | The str8 declaration equals all 39 remaining bytes before JSON parsing. | Construct the valid compressed str8 bundle, standard Base64 encode, and remove only trailing = padding. | `synthetically-accepted-envelope` |
| `lzma-corrupt` | `rejected-lzma-corrupt` | Reject an incomplete synthetic LZMA-Alone stream as ERR_IMPORT_LZMA_CORRUPT and expose no partial JSON. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | Truncated LZMA FORMAT_ALONE stream with its original valid 13-byte header intact | Valid str8 0xD9 bytes are compressed but must never be partially adopted | Unreached because complete LZMA decompression must succeed before MessagePack parsing. | Construct a valid compressed str8 bundle, assert its 13-byte LZMA-Alone header is intact, truncate the final byte, then Base64URL encode without padding. | `rejected:lzma-corrupt` |
| `invalid-base64` | `synthetic-str8-bundle` | Reject a character outside both Base64 alphabets. | `{"version":"synthetic-1","profiles":[]}` | Malformed Base64URL text | Unreached LZMA FORMAT_ALONE stage | Unreached str8 0xD9 stage | Unreached because strict outer decoding fails first. | Construct the valid str8 bundle text in memory, then replace its first encoded character with !. | `rejected:invalid-base64` |
| `lzma-dictionary-limit` | `synthetic-str8-bundle` | Reject an LZMA-Alone dictionary request above 64 MiB. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE with an over-limit dictionary header | Valid str8 0xD9 bytes must remain undecompressed | Unreached because the dictionary cap is checked first. | Replace compressed header bytes 1 through 4 with struct.pack('<I', 134217728), then encode. | `rejected:lzma-dictionary-limit` |
| `msgpack-short` | `rejected-str8-short-payload` | Reject a declaration one byte longer than its payload. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 declaring 40 bytes with 39 remaining | Require declared_length == len(remaining_bytes); 40 != 39. | Increment the positive str8 declaration from 39 to 40 without changing payload bytes. | `rejected:short-payload` |
| `msgpack-trailing` | `rejected-str8-trailing-byte` | Reject a declaration that leaves one trailing byte. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 declaring 38 bytes with 39 remaining | Require declared_length == len(remaining_bytes); 38 != 39. | Decrement the positive str8 declaration from 39 to 38 without changing payload bytes. | `rejected:trailing-bytes` |
| `invalid-utf8` | `synthetic-str8-bundle` | Reject exact-length MessagePack bytes that are invalid UTF-8. | `{"version":"synthetic-1","profiles":[]}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 declaring one byte | Declaration 1 equals the one remaining byte before UTF-8 validation. | Replace the payload with byte 0xFF and set the declaration to 1 before compression. | `rejected:invalid-utf8` |
| `ambiguous-root` | `synthetic-str8-bundle` | Reject an object matching both supported root shapes. | `{"profiles":[],"inputs":[],"id":"synthetic"}` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus exact one-byte length | Declaration equals all remaining UTF-8 bytes before root detection. | Encode jsonInput exactly; profiles matches bundle while inputs and id match single-profile. | `rejected:ambiguous-root` |
| `unsupported-root` | `synthetic-str8-bundle` | Reject valid JSON matching neither supported object root. | `[]` | Base64URL without padding | LZMA FORMAT_ALONE within the 64 MiB dictionary and output limits | str8 0xD9 plus one-byte length | Declaration 2 equals both remaining UTF-8 bytes before root detection. | Encode the exact empty JSON array instead of a bundle or single-profile object. | `rejected:unsupported-root` |
| `unobserved-generation` | `unobserved-software-2x` | Keep an unobserved Software generation unsupported despite synthetic protocol capability. | `{"version":"synthetic-1","profiles":[]}` | Unknown for the unobserved generation; synthetic Base64URL is not provenance | Unknown for the unobserved generation; synthetic LZMA is not provenance | Unknown for the unobserved generation; synthetic str8 is not provenance | No generation-specific rule is observed; do not infer one from synthetic bytes. | Present the synthetic recipe as a Software 2.x compatibility claim; reject the unsupported provenance claim. | `unobserved-unsupported` |

The five newly named positive fixtures plus existing Base64URL-unpadded positive rows cover the complete six-form outer-wrapper contract. `msgpack-short` and `msgpack-trailing` reference distinct str8 rejection rows. `lzma-corrupt` returns `rejected:lzma-corrupt` / `ERR_IMPORT_LZMA_CORRUPT`, and decompressed partial bytes are never exposed as JSON. The reproduced `raw-json-bundle` purpose spans both intentional checks: Issue #19 performs provisional decoder-boundary detection, and Issue #20 independently re-detects and validates the bundle root during lossless raw-model construction.

## Known limitations and admission rules

* The original 22 matrix rows contain no version-bearing structural support. The later original Software 2.0.2 pair and Issue #44 stick pair were owner attested; the seven-export follow-up includes a displayed Software version. These establish only their exact str32 bundle observations and the narrow conversion subset documented separately.
* During the original Issue #11 corpus investigation, raw private samples for its rows were unavailable, so those observed byte facts are documented observations rather than fresh reproduction. The later owner-supplied Software 2.0.2 sample was directly decoded and decompressed during its separate admission, as described above.
* Within the original 22 rows, no real Raw JSON, raw LZMA, padded Base64URL, standard Base64, fixstr, str8, str32-complete, or single-profile export was observed. Those rows remain synthetic capability only. The separate later Software 2.0.2 observation is a real complete str32 bundle, with outer representation details intentionally unpublished.
* Official sources expose context but no admitted structural mapping.
* This decision does not characterize bindings, semantic adapters, physical controls, or device behavior.

Future evidence must record exact provenance and a known Software version or literal `unknown`; traverse every strict stage with complete output; retain no private content; independently reproduce byte/length claims; link every matrix row to an inventory source; separate synthetic capability from observed compatibility; and treat public pages as contextual/negative unless an exact citation itself exposes the claimed bytes or structure. Wrapper acceptance is a protocol contract, not an invented Software support claim.

## R-013 and source traceability

The original R-013 result at `docs/design-research.md:2121` remains accurate for the fixed 22-row corpus: no requested real 2.x, bundle/single, fixstr, or str8 evidence became version-bearing support during that work. The later owner observation now supplies a complete Software 2.0.2 str32 bundle for its exact source scope. It does not establish single-profile, fixstr, str8, other wrapper, or broad 2.x behavior. Requirements derive from `docs/design-research.md:305-318`, `:443-565`, `:626-640`, and `:1931-1949`; product context is `README.md:5-7`. Todo 1 through Todo 5 records contain the original evidence ledger, observations, bounded public review, exact 22-row matrix, and exact 22 fixture recipes.

## Deterministic downstream handoffs

### Issue #14: physical mapping

Issue #14 receives **no physical map**. For v1, it remains blocked until separate privacy-safe, anonymized, version-bearing annotated exports are paired with left-hand physical press tests and revision/firmware notes. Right-hand physical press tests and support are deferred beyond v1 and do not block the left-hand mapping. Neither root shape, MessagePack tag, wrapper form, nor synthetic profile establishes an input-ID/pin-to-physical-control mapping.

### Issue #19: decoder-boundary root detection

Issue #19 owns input normalization; detection of Raw JSON, raw LZMA-Alone, Base64URL padded/unpadded, and standard Base64 padded/unpadded; strict Base64 validation; bounded complete LZMA-Alone decompression; MessagePack fixstr/str8/str16/str32 exact-length decoding; and UTF-8/JSON decoding. At this decoder boundary it detects bundle, single, ambiguous, or unknown root kind; rejects ambiguous/unknown roots; and, for accepted input, returns the decoded JSON plus a provisional bundle/single kind. Failures return a diagnostic and no adopted JSON. It owns stable errors for these stages, receives all 22 recipes including `lzma-corrupt`, and never exposes partial decompression as JSON.

### Issue #20: model-boundary root revalidation

Issue #20 receives Issue #19's strictly decoded JSON and provisional kind, then independently re-detects and validates the root shape while constructing the validated lossless raw bundle/single model. This revalidation is authoritative for model selection. A disagreement with #19's provisional kind is `ERR_IMPORT_ROOT`, never a silently trusted hint. #20 preserves unknown fields, raw scalar tokens, and opaque macro values under fixed structural bounds. It rejects duplicate decoded keys and malformed, invalidly typed, or ambiguous structures with `ERR_IMPORT_ROOT`; exact structural overflow with `ERR_IMPORT_LIMIT_EXCEEDED`; and an object or array `version` with `ERR_IMPORT_UNSUPPORTED_VERSION`. Missing, null, bool, string, and number versions remain raw evidence, not generation admission. A failure returns no partial model.

The model boundary allows at most 512 profiles and 256 inputs per profile, container depth 64, and 65,536 UTF-8 bytes per decoded string, all inclusive. It doesn't add another outer envelope or decoder policy. Both live issue bodies require bundle/single root-detection tests: Issue #19 covers the decoder boundary, while Issue #20 covers authoritative model selection and `RawScalar`. This overlap is deliberate defense-in-depth across separate trust boundaries.

### Issue #40: binding and macro grammar evidence

Issue #40's original Software 2.0.2 observation and the seven-export follow-up
are recorded in [the binding conversion decision](binding-conversion.md).
The latter adds exact single U/P and owner-confirmed left Ctrl+U contexts,
long U at 1278 ms, double I at 123 ms, and an observed `macro.v:1` container
with Button/Delay step shapes. Macro step counting can now use that grammar,
but complete macro semantics remain Unknown. Neither source establishes a
generic symbol rule, numeric namespace, general modifier or analog grammar,
other release, or Software 2.x-wide adapter. Original version-unknown and
source-candidate rows remain unadmitted for semantic conversion.

Issue #42 owns semantic adapter admission and must enforce the 1,000-step macro ceiling before interpretation. Issue #20 neither counts macro steps nor treats that ceiling as a raw array-length bound. This ownership split does not broaden support beyond the exact admitted Software 2.0.2 subset and introduces no separate version-validation API.

### Issue #44: configured stick mode

The separate `owner-software-2.0.2-stick-pair` extends the semantic evidence
only to the exact neutral Keyboard/WASD and Xbox Joystick contexts in the
binding conversion decision. Type `"3"`, other numeric assignments, enabled
combined/eight-direction settings, right-stick, DirectInput, nonzero
transformations, and other Software releases remain unadmitted. Synthetic
fixtures may exercise decoding, normalization, persistence, and input
projection without private source access; they do not establish physical
control identity, active hardware mode, raw-input source selection, or live Xbox
device accessibility.

The evdev-based live stick projection originally implemented for #44 was
superseded by #108. Configured stick metadata remains supported within its
admitted export predicates; the current raw-only path does not provide live
analog state.

### Issue #42: normalized settings and legacy deferral

The three owner UI/export comparisons dated 2026-09-26 are recorded by source
ID in [the binding conversion decision](binding-conversion.md). They admit only
the exact Software 2.0.2 Turbo T rates, two-step inert Macro configuration,
and Xbox Joystick angle 0/90 contexts. The comparison descriptions are the
evidence available to implementation; private source files and images are
not copied or required. They do not establish production Go parsing,
Turbo/Macro execution, stick coordinate rotation, or physical behavior.
The source comparison used bounded in-memory Python LZMA-Alone decompression,
exact MessagePack str32 length checks, and JSON inspection of the complete
stream; it did not run the production parser.

Legacy numeric binding conversion is outside v1. Track its research and any
later support in Issue #102, after v1 is complete. Numeric values remain
Unknown within the admitted adapter, and version-unknown or other-release
sources remain unsupported.

## Issue #11 acceptance checklist

| Criterion | Result |
| --- | --- |
| Decision document committed | PASS: sole tracked deliverable, finalized as one commit ahead of `origin/main`. |
| Each admitted structural capability has evidence and a synthetic fixture specification | PASS: one observed-version-unknown structure is evidence-linked; thirteen synthetic capability rows have exact recipes, including all six wrapper forms. |
| Unsupported and unobserved variants are explicit | PASS: six rejected rows include corrupt LZMA and exact str8/str16 mismatch boundaries; Software 1.x/2.x are unobserved-unsupported. |
| No private export contents committed | PASS: only anonymized metadata and explicit synthetic sentinels appear. |
| Consumer work can proceed without ownership guessing | PASS: #19 performs provisional decoder-boundary root detection/rejection and returns JSON plus kind/diagnostic; #20 independently re-detects and authoritatively validates model selection, rejecting disagreement. Both retain root-detection tests as defense-in-depth. #14 remains evidence-blocked; #40 now has only the later exact Software 2.0.2 subset described above. |
