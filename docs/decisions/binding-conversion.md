# Evidence-backed binding conversion

## Decision and scope

This decision establishes a closed conversion subset for one anonymous,
owner-supplied Azeron Software 2.0.2 UI/export pair. The unchanged production
path, `profiledecode.DecodeReader` followed by `profileraw.Parse`, accepted that
source as a bundle with one profile and 43 opaque inputs. That result admits the
raw boundary only. It does not select or implement a semantic adapter.

The supported subset is intentionally small:

* `KeyU` maps to Linux `KEY_U` with value 22 only in the complete observed long
  trigger context below.
* Ordered `KeyP`, `KeyL` maps to Linux `KEY_P`, value 25, then `KEY_L`, value
  38, only in the complete observed double trigger context below.
* Numeric `featureDelay` value 500 means a long trigger delay of 500
  milliseconds only in that long context.
* Numeric `doubleDelay` value 150 means a double trigger interval of 150
  milliseconds only in that double context.
* The complete long tuple `isHoldLong:false`, `isTurboLong:false`, and
  `isToggleOnHoldLong:false` means regular release behavior in that context.

No row defines a general symbol spelling rule, a meaning for `"0"` or `"1"`,
an array default, an unused-slot convention, or support for Software 2.x as a
family. Filenames, device information, firmware, and export `version` scalars
do not establish the Software release. The release in this decision is owner
attested from the supplied pair. It was not independently read from a displayed
Software version in the admitted UI images.
No separate source-generation identifier was established, so the adapter key
cannot be broader than this exact release and source scope.

## Sources and admission

| Source ID | Public description | Admitted use | Limit |
| --- | --- | --- | --- |
| `owner-software-2.0.2-pair` | Anonymous owner-supplied Software 2.0.2 UI/export pair | Release-scoped root, field, key, timing, and regular-release observations listed here | The export, images, labels, IDs, and complete macro content are private and are not normative dependencies or reproduced artifacts. Release attribution is owner attested, not independently displayed. |
| `linux-uapi-654ae5d` | [Linux input event codes at commit 654ae5d](https://github.com/torvalds/linux/blob/654ae5d73c05bd2943d65636ce6cd0aa46e62f18/include/uapi/linux/input-event-codes.h) | Canonical names and values `KEY_U` 22, `KEY_P` 25, and `KEY_L` 38 | Linux constants do not prove an Azeron raw mapping by numeric equality. |
| `linux-events-654ae5d` | [Linux input event semantics at commit 654ae5d](https://github.com/torvalds/linux/blob/654ae5d73c05bd2943d65636ce6cd0aa46e62f18/Documentation/input/event-codes.rst) | The `EV_KEY` keyboard namespace uses `KEY_<name>` | It does not establish source symbols or physical controls. |
| `raw-boundary` | [Strict profile export decoding](../profile-format.md) | Opaque raw ownership, scalar token preservation, root validation, and the #20/#40/#42 split | Structural acceptance is not semantic admission. |
| `normalized-model` | [Design research](../design-research.md), sections 7.2, 8.5, 9.1, 9.2, and 13.6 | Existing `TriggerBinding`, `Action`, canonical code, Unknown, and duplicate vocabulary | Examples in the design are not conversion evidence. |

Older version-unknown observations and synthetic corpus cases remain negative
controls. They are not retroactively attributed to Software 2.0.2. The exact
new structural observation and its acquisition limits are recorded in the
[export format corpus](export-format-corpus.md).

## Closed conversion rows

Each `mapped` row requires trusted source metadata selecting the exact
Software 2.0.2 source scope and an exact match of every listed field and JSON
type. A near match is not a partial mapping.

| Case and disposition | Exact raw representation and context | Independently observed meaning | Canonical result | Unresolved facts |
| --- | --- | --- | --- | --- |
| Long ordinary U, `mapped` | Trigger is `long`; `types` is the string array `["1","1","1"]`; `keyValuesLong` is `["KeyU","0","0","0"]`; `metaValuesLong` is `["0","0","0"]`; all three of `isHoldLong`, `isTurboLong`, and `isToggleOnHoldLong` are JSON boolean `false`; `featureDelay` is JSON number `500`. Source: `owner-software-2.0.2-pair`. | The matching UI shows ordinary U, a 500 ms long trigger delay, and regular release behavior. | One `TriggerBinding`, trigger `long`, kind `keyboard`, ordered actions `[keyboard KEY_U]`, `trigger_delay_ms:500`, `release_behavior:"regular"`. `KEY_U` is 22 in `linux-uapi-654ae5d`. | The standalone meanings of type `"1"`, string `"0"`, each array position, each false flag, omission, null, other values, valid timing range, precision, and threshold algorithm remain unknown. |
| Double P plus L, `mapped` | Trigger is `double`; `types` is the string array `["1","1","1"]`; `keyValuesDouble` is `["KeyP","KeyL","0","0"]`; `metaValuesDouble` is `["0","0","0"]`; `doubleDelay` is JSON number `150`. Source: `owner-software-2.0.2-pair`. | The matching UI shows a P plus L chord and a 150 ms double trigger interval. | One `TriggerBinding`, trigger `double`, kind `keyboard`, ordered actions `[keyboard KEY_P, keyboard KEY_L]`, `trigger_interval_ms:150`. `KEY_P` is 25 and `KEY_L` is 38 in `linux-uapi-654ae5d`. | Chord arity, unused slots, general ordering rules, other intervals, precision, boundary inclusivity, and recognition algorithm remain unknown. |
| Three trigger identities, `mapped` only as part of the observed input | `types` is exactly the ordered string array `["1","1","1"]`, and the UI shows distinct single, long, and double slots. Source: `owner-software-2.0.2-pair`. | This input has distinct `single`, `long`, and `double` trigger slots. | Preserve those trigger identities and source order. Actions require their own complete conversion row. | Type `"1"` has no standalone or generation-wide meaning. Missing, disabled, reordered, and other type representations are unknown. |

The long and double rows are separate predicates. Combining fields from them is
allowed only when a synthetic fixture intentionally represents the one
correlated input and retains every field required by both predicates.

## Unknown, unsupported, and invalid

Adapter selection and value interpretation are separate decisions:

* `unknown` applies when trusted metadata has selected this admitted Software
  2.0.2 scope, the raw structure remains inspectable, but no complete mapping
  row matches. Preserve the full raw binding object and needed trigger/type
  context in `UnknownBinding`.
* `unsupported_generation` applies when no release-specific adapter may be
  selected. Version-unknown sources, Software 1.x, every other Software 2.x
  release, and a broad `2.x` claim remain unsupported. This is not the result
  for an unfamiliar value encountered inside the admitted 2.0.2 adapter.
* `invalid` applies only when an established structural contract is violated,
  including a known macro grammar exceeding its semantic step limit. An
  unfamiliar value is not malformed merely because it has no mapping.

A recognizable base symbol does not authorize a partial result. If its
modifier representation differs from the complete positive row, the whole
composite is Unknown. The raw key and all modifier residue remain attached.
Missing fields, explicit JSON null, string values, numbers, and booleans stay
distinct. No unknown value becomes zero, default, unassigned, or an empty
modifier list.

Assignments are normalized independently and appended in source order. Two
assignments that produce the same canonical action remain two assignments.
They are not keyed, sorted, or coalesced by canonical code. This says nothing
about their physical origin, which remains outside Issue #40.

## Family dispositions

| Family | Software 2.0.2 admitted source scope | Other releases |
| --- | --- | --- |
| `types` | Only exact `["1","1","1"]` participates in the rows above. No generic discriminant table is established. Other forms are Unknown. | `unsupported_generation` |
| Modern keyboard symbols | Only contextual `KeyU`, ordered `KeyP`, and `KeyL` are mapped. Every other symbol is Unknown. No prefix conversion is allowed. | `unsupported_generation` |
| Modifiers and `metaValues*` | No general grammar. Exact no-modifier UI meaning is part of each whole positive predicate. Any residual or changed representation makes the composite Unknown and is preserved. Masks, enums, combinations, left/right identity, and zero semantics are unresolved. | `unsupported_generation` |
| Legacy numeric `keyValues` and `metaValues` | Unknown if encountered in the admitted adapter. No numeric namespace, number/string coercion, or Linux numeric equivalence is established. | `unsupported_generation` |
| Other keyboard, mouse, and `BTN_*` symbols | Unknown. No additional source symbol or namespace is correlated. | `unsupported_generation` |
| Single trigger | Slot identity only. No single action, timing, or release grammar is established. | `unsupported_generation` |
| Long trigger | Exact U, 500 ms trigger delay, and regular false/false/false tuple only. Other values or combinations are Unknown. | `unsupported_generation` |
| Double trigger | Exact ordered P plus L and 150 ms trigger interval only. Other values or combinations are Unknown. | `unsupported_generation` |
| Macro, sequence, macro hold, macro delay, and repeat | Unknown. A UI editor and ordered example were seen, but no raw container, tag, step, parameter, duration, or repeat grammar was correlated. | `unsupported_generation` |
| Turbo | Unknown except that the complete false/false/false tuple participates in the exact regular long row. Enabled encoding, rate, units, and interactions are unresolved. | `unsupported_generation` |
| Gamepad | Unknown. No button namespace, raw value, or discriminant meaning is correlated. | `unsupported_generation` |
| Analog | Unknown. Axis namespace, mode, sign, direction, center, range, dead zone, inversion, rotation, value type, and analog/WASD relationship are unresolved. No `ABS_*` result is inferred. | `unsupported_generation` |
| Defaults and unused slots | No general meaning. The observed `"0"` entries remain parts of exact predicates only. | `unsupported_generation` |
| Physical controls, pins, device, and firmware | Not decided here. No physical guess is permitted. | Not Issue #40 scope |

## Future machine-readable fixture contract

Issue #42 must express table-driven cases as UTF-8 JSON records. This is a
future fixture requirement, not a production API, schema, registry, or loader.
Each record has these fields:

| Field | Requirement |
| --- | --- |
| `case` | Nonempty synthetic case name. |
| `software_release` | Exact evidenced release for a positive case. A clearly synthetic unadmitted identifier is allowed only for a policy-negative generation or grammar case. It is never copied from an unrelated export `version` scalar. |
| `source_refs` | Nonempty source-row or policy references. Positive mappings cite an independently admitted row. |
| `raw` | Synthetic JSON object using the exact release-specific fields, scalar types, and complete positive predicate where mapped. |
| `context` | Only the parent or discriminant context needed for conversion. An empty object is valid. |
| `expected.disposition` | Exactly `mapped`, `unknown`, `unsupported_generation`, or `invalid`. |
| `expected.bindings` | Ordered array required for `mapped` and `unknown`. Entries use existing binding kind, trigger, actions, canonical code, timing, and release vocabulary only where relevant. Duplicate entries remain separate. |
| `expected.reason` | Required nonempty text for every result except `mapped`. |
| `expected.raw` | Required complete original synthetic raw object for `unknown`. |
| `raw_tokens` | Required only when exact number-token spelling matters. It maps field paths to literal source tokens, independently of JSON reserialization. |

Missing and explicit null are different fixtures. Ordered source arrays and
expected bindings remain ordered. Expected values are written literally, not
calculated from adapter output. Policy-negative records are synthetic and
cannot prove that any Software release emits their inputs.

### Synthetic mapped temporal case

This fixture combines only fields from the same correlated input. Its expected
actions resolve the key and timing rows in one vocabulary.

```json
{
  "case": "synthetic Software 2.0.2 exact long and double subset",
  "software_release": "2.0.2",
  "source_refs": [
    "owner-software-2.0.2-pair:long-u-500ms-regular",
    "owner-software-2.0.2-pair:double-p-l-150ms",
    "linux-uapi-654ae5d"
  ],
  "raw": {
    "types": ["1", "1", "1"],
    "keyValuesLong": ["KeyU", "0", "0", "0"],
    "metaValuesLong": ["0", "0", "0"],
    "keyValuesDouble": ["KeyP", "KeyL", "0", "0"],
    "metaValuesDouble": ["0", "0", "0"],
    "featureDelay": 500,
    "doubleDelay": 150,
    "isHoldLong": false,
    "isTurboLong": false,
    "isToggleOnHoldLong": false
  },
  "context": {"source_scope": "single correlated input"},
  "expected": {
    "disposition": "mapped",
    "bindings": [
      {
        "kind": "keyboard",
        "trigger": "long",
        "actions": [{"kind": "keyboard", "code": "KEY_U"}],
        "trigger_delay_ms": 500,
        "release_behavior": "regular"
      },
      {
        "kind": "keyboard",
        "trigger": "double",
        "actions": [
          {"kind": "keyboard", "code": "KEY_P"},
          {"kind": "keyboard", "code": "KEY_L"}
        ],
        "trigger_interval_ms": 150
      }
    ]
  }
}
```

### Synthetic Unknown modifier case

This policy-negative fixture does not assert that Software 2.0.2 emits the
sentinel modifier. The complete composite stays Unknown, so no partial mapped
action silently discards modifier residue.

```json
{
  "case": "synthetic known key with unknown modifier residue",
  "software_release": "2.0.2",
  "source_refs": ["policy:unknown-modifier-residue", "normalized-model:FR-023"],
  "raw": {
    "types": ["1", "1", "1"],
    "keyValuesLong": ["KeyU", "0", "0", "0"],
    "metaValuesLong": ["SyntheticModifier", "0", "0"],
    "isHoldLong": false,
    "isTurboLong": false,
    "isToggleOnHoldLong": false
  },
  "context": {"trigger": "long", "synthetic_negative": true},
  "expected": {
    "disposition": "unknown",
    "bindings": [{"kind": "unknown", "trigger": "long"}],
    "reason": "The admitted key row does not define this modifier representation, so the complete composite remains Unknown.",
    "raw": {
      "types": ["1", "1", "1"],
      "keyValuesLong": ["KeyU", "0", "0", "0"],
      "metaValuesLong": ["SyntheticModifier", "0", "0"],
      "isHoldLong": false,
      "isTurboLong": false,
      "isToggleOnHoldLong": false
    }
  }
}
```

### Synthetic duplicate case

```json
{
  "case": "synthetic duplicate exact long KeyU assignments",
  "software_release": "2.0.2",
  "source_refs": [
    "owner-software-2.0.2-pair:long-u-500ms-regular",
    "normalized-model:FR-026",
    "normalized-model:13.6"
  ],
  "raw": {
    "inputs": [
      {
        "types": ["1", "1", "1"],
        "keyValuesLong": ["KeyU", "0", "0", "0"],
        "metaValuesLong": ["0", "0", "0"],
        "featureDelay": 500,
        "isHoldLong": false,
        "isTurboLong": false,
        "isToggleOnHoldLong": false
      },
      {
        "types": ["1", "1", "1"],
        "keyValuesLong": ["KeyU", "0", "0", "0"],
        "metaValuesLong": ["0", "0", "0"],
        "featureDelay": 500,
        "isHoldLong": false,
        "isTurboLong": false,
        "isToggleOnHoldLong": false
      }
    ]
  },
  "context": {
    "trigger": "long",
    "assignment_identity": ["synthetic-input-A", "synthetic-input-B"],
    "synthetic_composition": true
  },
  "expected": {
    "disposition": "mapped",
    "bindings": [
      {
        "kind": "keyboard",
        "trigger": "long",
        "actions": [{"kind": "keyboard", "code": "KEY_U"}],
        "trigger_delay_ms": 500,
        "release_behavior": "regular"
      },
      {
        "kind": "keyboard",
        "trigger": "long",
        "actions": [{"kind": "keyboard", "code": "KEY_U"}],
        "trigger_delay_ms": 500,
        "release_behavior": "regular"
      }
    ]
  }
}
```

### Synthetic unsupported generation case

```json
{
  "case": "synthetic version-unknown numeric value is not Linux KEY_U",
  "software_release": "synthetic-unadmitted-version-unknown",
  "source_refs": ["policy:release-admission-required", "export-format-corpus:observed-version-unknown"],
  "raw": {"keyValues": ["22"]},
  "context": {"synthetic_negative": true},
  "expected": {
    "disposition": "unsupported_generation",
    "reason": "No admitted release and source namespace links numeric string 22 to Linux KEY_U."
  }
}
```

### Synthetic unresolved grammar case

An array does not become a macro because it is short enough to pass a limit.

```json
{
  "case": "synthetic generic array under unknown macro grammar",
  "software_release": "synthetic-unadmitted-grammar",
  "source_refs": ["policy:macro-grammar-required-before-step-count"],
  "raw": {"unresolved_payload": ["synthetic-token-a", "synthetic-token-b"]},
  "context": {
    "claimed_family": "macro",
    "generic_array_length": 2,
    "known_step_grammar": false,
    "synthetic_negative": true
  },
  "expected": {
    "disposition": "unknown",
    "bindings": [],
    "reason": "No admitted grammar defines this container or its elements as macro steps, so length does not authorize interpretation.",
    "raw": {"unresolved_payload": ["synthetic-token-a", "synthetic-token-b"]}
  }
}
```

## Macro limit and Issue #42 handoff

Issue #20 continues to preserve opaque macro values under its existing
structural limits. It neither recognizes nor counts steps. Issue #42 first
selects an admitted release adapter, then recognizes a release-specific macro
grammar, then counts syntactically identified steps, and only then interprets
any action, delay, hold, repeat, or other parameter.

For every future known grammar, table-driven coverage must include these
literal boundaries:

| Input after grammar recognition | Required result before interpretation |
| --- | --- |
| One macro with exactly 1,000 identified steps | `accepted_for_interpretation`; this does not guarantee a final mapped result |
| One macro with exactly 1,001 identified steps | `invalid`, reason `macro step limit exceeded`; reject before interpreting step 1 |
| Any array length under an unknown grammar, including 2, 1,000, or 1,001 | `unknown` with raw and context preserved; do not count or interpret it as steps |

Literal 1,000 and 1,001 fixture records cannot be written until an actual raw
step representation is admitted. Inventing a generic grammar to fill those
records would make the boundary test dishonest. Once evidence exists, both
records must use the exact release-specific raw container and differ only in
their literal step count. This ceiling is semantic admission policy, not a new
generic array cap and not evidence for any grammar.

No adapter, fixture file, registry, fingerprint, provenance token system, or
schema is introduced by this decision. Issue #42 may implement only the closed
rows above and the required Unknown behavior. Extending any family or release
requires another release/export/known-meaning chain and a new reviewed row.
