# Evidence-backed binding conversion

## Decision and scope

This decision establishes a closed conversion subset from anonymous,
owner-supplied Azeron Software 2.0.2 UI/export evidence. For the original pair,
the unchanged production
path, `profiledecode.DecodeReader` followed by `profileraw.Parse`, accepted that
source as a bundle with one profile and 43 opaque inputs. That result admits the
raw boundary only. It does not select or implement a semantic adapter.

The original supported subset is intentionally small; the follow-up rows below
extend it without generalizing to unobserved settings:

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
do not establish the Software release. The original pair was owner attested.
The follow-up settings image explicitly displays Software 2.0.2 and Firmware
111; firmware is not used as an adapter selector.
No separate source-generation identifier was established, so the adapter key
cannot be broader than this exact release and source scope.

## Sources and admission

| Source ID | Public description | Admitted use | Limit |
| --- | --- | --- | --- |
| `owner-software-2.0.2-pair` | Anonymous owner-supplied Software 2.0.2 UI/export pair | Release-scoped root, field, key, timing, and regular-release observations listed here | The export, images, labels, IDs, and complete macro content are private and are not normative dependencies or reproduced artifacts. Release attribution is owner attested, not independently displayed. |
| `owner-software-2.0.2-followup` | Seven exports paired in message order with seven configuration images on 2026-09-05, plus a settings image displaying Software 2.0.2 | Complete anonymous empty setting, single U/P, left Ctrl+U, long U, double I, and macro structure below | The owner explicitly confirmed left Ctrl. Full exports, images, labels, IDs, and the complete macro sequence remain private. |
| `owner-software-2.0.2-stick-pair` | Two private exports supplied for Issue #44 on 2026-09-20; owner-confirmed Software 2.0.2, Keyboard versus Xbox Joystick, and up=W/right=D/down=S/left=A | Exact neutral left-stick mode and directional tuples below | Release, UI mode names, and directions are owner attested, not independently inspected settings screenshots. Python inspection did not run production `DecodeReader`/`Parse`. No live Xbox-device or physical-control mapping is established. |
| `owner-software-2.0.2-turbo-20260926` | Owner UI/export comparison supplied 2026-09-26 | Exact T Turbo settings at two displayed rates below | Private inputs are not included; no production Go parsing or physical Turbo behavior was verified. |
| `owner-software-2.0.2-macro-20260926` | Owner UI/export comparison supplied 2026-09-26 | Exact two-step macro setting and repeat toggle below | Only minimum anonymous step observations are recorded; no private full macro, production Go parsing, or playback behavior is included. |
| `owner-software-2.0.2-angle-20260926` | Owner UI/export comparison supplied 2026-09-26 | Exact Xbox Joystick angle 0 and 90 contexts below | Private inputs are not included; no production Go parsing or physical axis behavior was verified. |
| `local-linux-uapi` | Local `/usr/include/linux/input-event-codes.h`, inspected 2026-09-05 | `KEY_W` 17, `KEY_U` 22, `KEY_I` 23, `KEY_P` 25, `KEY_LEFTCTRL` 29, `KEY_LEFTSHIFT` 42 | Constants establish Linux names and values, not Azeron numeric namespaces. |
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

## Follow-up conversion evidence

The seven Base64URL payloads were decoded in memory with complete LZMA-Alone
decompression (`xz`, 64 MiB decompressor memory limit), exact MessagePack str32
length checks, and JSON parsing. Each contains one profile with 43 inputs.
All consecutive JSON differences are confined to the same input; no physical
control identity is inferred from its array position. This inspection did not
run the production decoder or raw parser on these seven payloads.

The source for every following row is `owner-software-2.0.2-followup`. A row
requires its exact `types` string array, key and modifier arrays, and the
trigger-local JSON boolean tuple `isHold*`, `isTurbo*`, `isToggleOnHold*`, all
`false`. The suffix is absent for single, `Long` for long, and `Double` for
double. Other slots have four `"0"` key entries and three `"0"` modifier
entries in these cases; those values alone do not authorize a conversion.

| Case | Exact fields in addition to the common conditions | Admitted result |
| --- | --- | --- |
| Single U | `types:["1","11","11"]`, `keyValues:["KeyU","0","0","0"]`, `metaValues:["0","0","0"]` | Single keyboard action `KEY_U` 22, regular release |
| Single P | Same single context, `keyValues:["KeyP","0","0","0"]`, `metaValues:["0","0","0"]` | Single keyboard action `KEY_P` 25, regular release |
| Single left Ctrl+U | Same single context, `keyValues:["KeyU","0","0","0"]`, `metaValues:["ControlLeft","0","0"]` | Single keyboard action `KEY_U` 22 with left Ctrl (`KEY_LEFTCTRL` 29), regular release; not two sequential actions |
| Long U | `types:["11","1","11"]`, `keyValuesLong:["KeyU","0","0","0"]`, `metaValuesLong:["0","0","0"]`, numeric `featureDelay:1278` | Long keyboard action `KEY_U` 22, `trigger_delay_ms:1278`, regular release |
| Double I | `types:["11","11","1"]`, `keyValuesDouble:["KeyI","0","0","0"]`, `metaValuesDouble:["0","0","0"]`, numeric `doubleDelay:123` | Double keyboard action `KEY_I` 23, `trigger_interval_ms:123`; no release behavior is inferred from the double image |

The first image has an empty SINGLE keyboard field and Regular selected. Its
complete anonymous semantic setting is below. It excludes only `id`,
`pinOne`, `pinTwo`, and `label`; those fields do not participate in this
recognition. Under the admitted release/source metadata, this exact setting
normalizes only the SINGLE slot to `kind:unbound` with no payload. LONG and
DOUBLE stay Unknown. Object key order and insignificant whitespace are ignored;
all semantic fields, JSON types and number tokens, and ordered arrays must
match. Missing, null, changed, or additional semantic fields do not match this
Unbound predicate; separately documented normalization predicates still apply.

```json
{
  "types": ["11", "11", "11"],
  "keyValues": ["0", "0", "0", "0"],
  "metaValues": ["0", "0", "0"],
  "keyValuesLong": ["0", "0", "0", "0"],
  "metaValuesLong": ["0", "0", "0"],
  "keyValuesDouble": ["0", "0", "0", "0"],
  "metaValuesDouble": ["0", "0", "0"],
  "layeringProfileId": "", "isBelkin": false, "isToggleOnHold": false,
  "layeringProfileIdLong": "", "isBelkinLong": false, "isToggleOnHoldLong": false,
  "layeringProfileIdDouble": "", "isBelkinDouble": false, "isToggleOnHoldDouble": false,
  "macro": {"repeat": false, "steps": [], "v": 1},
  "longMacro": {"repeat": false, "steps": [], "v": 1},
  "doubleMacro": {"repeat": false, "steps": [], "v": 1},
  "featureDelay": 500, "doubleDelay": 150, "subType": "11",
  "x": 0, "y": 0, "interval": 0, "yInterval": 0,
  "xLong": 0, "yLong": 0, "xDouble": 0, "yDouble": 0,
  "isHold": false, "isHoldLong": false, "isHoldDouble": false,
  "holdTime": 0, "holdTimeLong": 0, "holdTimeDouble": 0,
  "isTurbo": false, "isTurboLong": false, "isTurboDouble": false,
  "turboInterval": 0, "turboIntervalLong": 0, "turboIntervalDouble": 0,
  "sequenceTriggerSettings": {"isPingPongLoop": false, "sequenceSteps": []},
  "analogSettings": {
    "angle": 0, "lowerLimit": 0, "upperLimit": 0, "sensitivity": 0,
    "analogKeys": {
      "left": {"up": [87,0,0], "right": [68,0,0], "down": [83,0,0], "left": [65,0,0]},
      "right": {"up": ["ArrowUp",0,0], "right": ["ArrowRight",0,0], "down": ["ArrowDown",0,0], "left": ["ArrowLeft",0,0]}
    },
    "diagonalKeys": {
      "left": {"up_right": ["Digit1",0,0], "up_left": ["Digit2",0,0], "down_left": ["Digit3",0,0], "down_right": ["Digit4",0,0]},
      "right": {"up_right": ["Digit1",0,0], "up_left": ["Digit2",0,0], "down_left": ["Digit3",0,0], "down_right": ["Digit4",0,0]}
    },
    "analogCones": {"verticalCone": 45, "horizontalCone": 45},
    "isEightDirectionalTrigger": false, "mouseSensitivity": 5, "analogThrottle": 0,
    "isAnalogSmoothing": false, "triggerMagnitude": 4, "isCombinedAnalog": false,
    "combinedAnalogMagnitude": 6, "isHoldTrigger": false, "holdMagnitude": 9,
    "holdType": "1", "holdKeyValues": ["16","0","0"], "rotateStickButtonId": 0,
    "isAngleLock": false, "lockZoneAngle": 70, "lockZoneSize": 30,
    "isRightAnalog": false, "invertXAxis": false, "invertYAxis": false
  },
  "scrollSpeed": 1, "scrollThreshold": 10, "isSmoothScroll": false
}
```

The paired SINGLE U export changes only `types` to `["1","11","11"]`
and `keyValues` to `["KeyU","0","0","0"]` in that one input. This is an
exact paired observation, not a general rule for type `"11"` or string `"0"`.
The later long and double images retain indicators on other tabs even though
their exported key arrays are empty; tab indicators do not establish active
actions.

The U-to-P comparison changes only the first key entry. Adding left Ctrl
correlates with `ControlLeft` in the first modifier entry. No right Ctrl,
multi-modifier ordering, or general modifier grammar is established.
The long and double exports also change active slots, so these are exact
1278 ms and 123 ms observations, not isolated timing sweeps proving a range.
The double export retains `featureDelay:1278` while its long key array is
empty; an inactive timing value must not create a long action.

### Observed macro grammar

In the macro case `types` is `["16","11","11"]` and the active single container
is `macro`, an object with numeric `v:1`, boolean `repeat`, and ordered array
`steps`. The preceding non-macro cases have `repeat:false` and empty `steps`;
the macro image shows "Repeat (while held down)" enabled, matching `repeat:true`.
This identifies the container and step boundary for this source, not a general
meaning for every `"16"`, a grammar for `longMacro`/`doubleMacro`, or playback
behavior for all repeat values.

The minimum step observations, without reproducing the private sequence, are:

| Step representation | Correlated UI meaning | Limit |
| --- | --- | --- |
| String `type:"Button"`, string `direction:"Full"`, numeric `duration`, and `keyCode` | One Button editor row; numeric `keyCode:87` correlates with W and numeric duration 20 with 20 ms; string `keyCode:"ShiftLeft"` correlates with Shift and numeric duration 50 with 50 ms | W maps contextually to `KEY_W` 17, not Linux code 87. No general numeric namespace or complete press/release scheduling semantics follows. The UI does not distinguish Shift sides, so left-Shift semantics remain unconfirmed despite the raw spelling. |
| String `type:"Delay"`, string `direction:"Full"`, numeric `duration:45`, absent `keyCode` | One Delay editor row showing 45 ms | Other values, missing/null fields, and other directions remain unobserved. |

These rows admit step recognition and the stated UI parameter correlations.
They do not yet admit a fully mapped executable or matchable macro:
`direction:"Full"` scheduling, repeat lifecycle, and Shift-side semantics
remain unresolved. Preserve unknown semantics and raw content rather than
executing or guessing them. Repeating observed step shapes in synthetic
limit fixtures is policy coverage, not evidence of a real 1,000-step export.

## Issue #44 stick conversion evidence

The source is `owner-software-2.0.2-stick-pair`, independently attributed by
the owner to Software 2.0.2 and the UI modes Keyboard and Xbox Joystick.
The owner confirmed Keyboard directions up=W, right=D, down=S, left=A.
The planning inspection used Python `base64`, `lzma`, and `json` in memory;
it did not run production `profiledecode.DecodeReader` or `profileraw.Parse`
on this private pair. Structural observations are recorded in the
[export format corpus](export-format-corpus.md).

Both bundles have one profile and 43 inputs. A recursive JSON comparison
found exactly one changed value: input index 23's `types[0]` is string `"4"`
for Keyboard and string `"21"` for Xbox Joystick. This ordinal is observation
context only; recognition must not depend on input position, ID, pin, or any
private source content. The complete type arrays, not a standalone numeric
discriminant, participate in the following closed predicates.

Both modes require trusted source metadata `2.0.2` / `azeron-software-export`,
string `subType:"11"`, and an `analogSettings` object. The neutral context
requires literal JSON number token `0` for `angle`, `lowerLimit`, and `upperLimit`,
and JSON boolean `false` for `isRightAnalog`, `invertXAxis`, `invertYAxis`,
`isCombinedAnalog`, `isEightDirectionalTrigger`, `isHoldTrigger`,
`isAnalogSmoothing`, and `isAngleLock`. The primary input requires JSON
boolean `false` for `isHold`, `isTurbo`, and `isToggleOnHold`, string array
`keyValues:["0","0","0","0"]`, and string array
`metaValues:["0","0","0"]`. Missing, null, or differently typed required
fields do not satisfy these predicates.

| Complete type context | Owner-confirmed UI meaning | Admitted primary result |
| --- | --- | --- |
| `types:["4","11","11"]`, common neutral predicates, and all four exact left-direction tuples below | Keyboard with up=W, right=D, down=S, left=A | Single trigger, kind `stick`, mode `keyboard`, with the four canonical directions; no generic keyboard actions |
| `types:["21","11","11"]` and common neutral predicates | Xbox Joystick, left stick | Single trigger, kind `stick`, mode `xbox`; no keyboard direction assignments |

| `analogSettings.analogKeys.left` member | Exact JSON numeric tuple | Owner-confirmed key | Canonical direction code |
| --- | --- | --- | --- |
| `up` | `[87,0,0]` | W | `KEY_W` |
| `right` | `[68,0,0]` | D | `KEY_D` |
| `down` | `[83,0,0]` | S | `KEY_S` |
| `left` | `[65,0,0]` | A | `KEY_A` |

These tuples establish only the complete Keyboard context above. They do not
establish arbitrary numeric key conversion, a meaning for the zero slots,
or an extension to ordinary keyboard `keyValues`. The Xbox export retains
the same dormant keyboard assignments, but those must not generate keyboard
directions for Xbox mode. Inactive right/diagonal keys, hold payloads, and
macro bodies are retained opaquely rather than interpreted. Both exact type
arrays preserve single/long/double trigger ordering; their unimplemented
long and double slots remain Unknown.

Type `"3"`, DirectInput, right-stick, combined/eight-direction modes, nonzero
angle or limits, inversion, other enabled transformations, and unconfirmed
direction tuples remain unadmitted. An unsupported contextual variant in
the selected 2.0.2 adapter keeps the entire primary binding Unknown with raw
data intact. Other releases remain `unsupported_generation`. Xbox identity
is not generalized to DirectInput. No export field here establishes kernel
axis codes, signs, ranges, dead-zone units, a selected raw-input source, an active
hardware profile, or successful live Xbox-device operation.

Current input uses only the qualified physical-button hidraw path from #108;
these static stick settings do not establish live analog support or authorize
an evdev fallback.

### Minimal synthetic stick fixtures

Use this neutral single-profile JSON as the Keyboard fixture; its profile ID
is synthetic, and it does not reproduce the private 43-input bundle. Supply
the trusted release/source metadata separately to the adapter.

```json
{
  "id": "synthetic-stick",
  "inputs": [{
    "types": ["4", "11", "11"], "subType": "11",
    "keyValues": ["0", "0", "0", "0"], "metaValues": ["0", "0", "0"],
    "isHold": false, "isTurbo": false, "isToggleOnHold": false,
    "analogSettings": {
      "angle": 0, "lowerLimit": 0, "upperLimit": 0,
      "isRightAnalog": false, "invertXAxis": false, "invertYAxis": false,
      "isCombinedAnalog": false, "isEightDirectionalTrigger": false,
      "isHoldTrigger": false, "isAnalogSmoothing": false, "isAngleLock": false,
      "analogKeys": {"left": {
        "up": [87, 0, 0], "right": [68, 0, 0],
        "down": [83, 0, 0], "left": [65, 0, 0]
      }}
    }
  }]
}
```

The Xbox fixture is exactly this JSON with `types` changed to
`["21","11","11"]`; retain the dormant key tuples to check that they do not
populate Xbox directions. For each fixture, also wrap the same synthetic
profile in `{"profiles":[...]}` to exercise the bundle path. Single-profile
and Raw JSON tests are synthetic protocol coverage, not new observed export
forms. Expected primary results are the two rows above; relocating or
duplicating the synthetic input must not alter its interpretation. Changing
one required predicate to an unadmitted value must preserve an Unknown raw
binding, rather than produce a partial stick result.

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
about their physical origin, which remains outside Issue #40. The separate
Issue #30 candidate index uses typed identity and the evidenced layout scope;
it does not alter these conversion predicates.

## Issue #42 closed settings rows

The sources are the three owner UI/export comparisons listed above, each
showing Software 2.0.2. They admit only these exact contexts. No private
originals, production Go parsing, Turbo execution, macro playback, or physical
axis behavior is claimed. All rows require trusted metadata
`SoftwareRelease="2.0.2"`, `SourceScope="azeron-software-export"`,
`isToggleOnHold` absent, and these shared inactive-slot values: long/double
key arrays each four string `"0"` values; long/double meta arrays each three
string `"0"` values; `isHoldLong`, `isHoldDouble`, `isTurboLong`,
`isTurboDouble` are boolean `false`; `turboIntervalLong` and
`turboIntervalDouble` are number token `0`. Missing, null, and wrong-typed
values do not match. These exact values define no defaults.

| Case and source | Additional exact predicate | UI observation and admitted result | Exclusions |
| --- | --- | --- | --- |
| Turbo T, `owner-software-2.0.2-turbo-20260926` | `types:["1","11","11"]`; `keyValues:["KeyT","0","0","0"]`; `metaValues:["0","0","0"]`; `isHold:false`, `isTurbo:true`; `turboInterval` number token `20` or `50` | UI shows 25 or 10 clicks/second. Map to `TriggerSingle`, kind `turbo`, code `KEY_T`, rate 25 or 10 respectively; no Actions or other payload. | No `500/x` rule, units conversion, arbitrary rate, rounding, release behavior, or execution. Non-Turbo T remains Unknown because Regular was not shown. |
| Macro, `owner-software-2.0.2-macro-20260926` | `types:["16","11","11"]`; key/meta arrays are four/three string zeroes; `isHold:false`, `isTurbo:false`; `turboInterval` number token `0`; shared inactive slots. `macro` has exactly `v` number token `1`, boolean `repeat`, and two steps only: `{type:"Button",direction:"Full",duration:50,keyCode:87}`, then `{type:"Delay",direction:"Full",duration:100}`; no extra step fields. | UI shows W Button 50 ms then Delay 100 ms; repeat is the displayed Repeat (while held down) toggle. Map to `TriggerSingle`, kind `macro`, `repeat_while_held` from `repeat`, and ordered steps `{button,KEY_W,50}`, `{delay,"",100}`; no Actions or other payload. | 87 maps contextually to `KEY_W` only here. Other directions, keys, modifiers, durations, or step shapes keep the whole binding Unknown. No execution or scheduling meaning is inferred for `Full`. |
| Xbox angle, `owner-software-2.0.2-angle-20260926` | `types:["21","11","11"]`; `subType` absent; `keyValues:["87","0","0","0"]`; `metaValues:["0","0","3"]`; `isHold:false`, `isTurbo:false`; `turboInterval` number token `0`; shared inactive slots. `analogSettings.angle` token 0 or 90; limits tokens 0; right-analog, inversion, combined, eight-direction, hold, smoothing, angle-lock flags all false; sensitivity 0, mouseSensitivity 5, analogThrottle 0, triggerMagnitude 4, combinedAnalogMagnitude 6, holdMagnitude 9, holdType `"1"`, rotateStickButtonId 0, lockZoneAngle 70, lockZoneSize 30, with exact JSON types. | UI shows Xbox Joystick at 0 or 90 degrees. Map to `TriggerSingle`, kind `stick`, mode `xbox`, that `AngleDegrees`, empty directions and Actions. | Arrays are contextual predicates, not keyboard assignment or legacy support. No global semantics, physical orientation, or coordinate rotation is inferred. |

The comparisons found only `isTurbo` and `turboInterval` changing for Turbo
activation, only `turboInterval` changing between the two displayed rates,
only `macro.repeat` changing between repeat states, and only
`analogSettings.angle` changing between the two Xbox settings. All unrelated
fields stay opaque. Matchers do not use input index, ID, label, pin, private
profile name, or filename. Preserve a near match as whole-binding Unknown
with raw fields intact. A nonzero-angle projection remains `Known=false` with
a zero vector; this records configuration without claiming transformation.

## Family dispositions

| Family | Software 2.0.2 admitted source scope | Other releases |
| --- | --- | --- |
| `types` | Only the exact original and follow-up contexts above are admitted. No generic discriminant table is established. | `unsupported_generation` |
| Modern keyboard symbols | Contextual `KeyU`, `KeyP`, `KeyL`, `KeyI`, and Turbo `KeyT` only. No prefix conversion is allowed. | `unsupported_generation` |
| Modifiers and `metaValues*` | Exact no-modifier predicates and the single left Ctrl+U row only. Other modifiers, combinations, masks, enums, ordering, and general zero semantics remain unresolved. | `unsupported_generation` |
| Legacy numeric `keyValues` and `metaValues` | Unknown except the exact Xbox stick context above. The macro step's `keyCode:87` is likewise contextual only. No numeric namespace, number/string coercion, or Linux numeric equivalence is established. | `unsupported_generation` |
| Other keyboard, mouse, and `BTN_*` symbols | Unknown. No additional source symbol or namespace is correlated. | `unsupported_generation` |
| Single trigger | Exact U, P, and left Ctrl+U regular rows; neutral Keyboard/Xbox and observed Xbox angle stick rows; exact T Turbo and two-step Macro settings above. Empty-setting observation does not establish a generic unassigned rule. | `unsupported_generation` |
| Long trigger | Exact U rows at 500 ms and 1278 ms with their respective type contexts and regular tuples. Other values or combinations are Unknown. | `unsupported_generation` |
| Double trigger | Exact ordered P plus L at 150 ms and I at 123 ms, each in its own context. Other values or combinations are Unknown. | `unsupported_generation` |
| Macro, sequence, macro hold, macro delay, and repeat | The Issue #40 follow-up admits structure and limited correlations; Issue #42 admits only the exact inert two-step setting above. Other shapes and playback remain Unknown. | `unsupported_generation` |
| Turbo | Issue #42 admits only the exact T settings above. Other keys, rates, release semantics, and execution remain Unknown. | `unsupported_generation` |
| Gamepad | Only the exact neutral and observed-angle Xbox Joystick contexts above are admitted. Button namespaces and other gamepad modes remain Unknown. | `unsupported_generation` |
| Analog and Keyboard stick | Only the exact neutral Keyboard/WASD context, neutral Xbox context, and observed Xbox angle 0/90 contexts above are admitted. Axis namespace, range, sign, coordinate rotation, and live-device mapping remain unresolved. No `ABS_*` result is inferred from the export. | `unsupported_generation` |
| Defaults and unused slots | No general meaning. The observed `"0"` entries remain parts of exact predicates only. | `unsupported_generation` |
| Physical controls, pins, device, and firmware | Not decided here. No physical guess is permitted. | Not Issue #40 scope |

## Illustrative machine-readable fixture vocabulary

Issue #42's implemented regression goldens are the synthetic
`v1-settings.input.json` and literal `v1-settings.expected.json` projection,
alongside existing adapter fixtures. The following record shape is an
illustrative documentation example, not the format of those golden files or a
production API, schema, registry, or loader. Its fields describe how a
standalone case could record the same evidence:

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

For the observed grammar above and every future known grammar, table-driven coverage must include these
literal boundaries:

| Input after grammar recognition | Required result before interpretation |
| --- | --- |
| One macro with exactly 1,000 identified steps | `accepted_for_interpretation`; this does not guarantee a final mapped result |
| One macro with exactly 1,001 identified steps | `invalid`, reason `macro step limit exceeded`; reject before interpreting step 1 |
| Any array length under an unknown grammar, including 2, 1,000, or 1,001 | `unknown` with raw and context preserved; do not count or interpret it as steps |

The follow-up supplies an actual `macro.v:1` container and Button/Delay
step representation. Issue #42 tests synthetic 1,000 and 1,001 step
records using these observed shapes, differing only in literal step count.
Count recognized steps before interpreting key codes, durations, directions,
or repeat behavior, even when final semantics remain Unknown. Unknown
containers or step shapes do not authorize guessing a grammar. This ceiling
is semantic admission policy, not a new generic array cap or proof that the
Software produces such large macros.

The decision introduces no registry, fingerprint, provenance token system, or
schema. Issue #42 implements only the closed rows above and the required
Unknown behavior. The adapter's normalizer revision is 4 while storage and
model schema versions remain 1. Revision-1/2/3 caches reconstruct in memory
from retained originals without writes. Extending any family or release
requires another release/export/known-meaning chain and a new reviewed row.
