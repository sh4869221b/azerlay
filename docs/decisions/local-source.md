# Safe Linux Azeron stored-profile reading

## Decision and scope

Adopt a read-only local source for **individually selected stored Software
profile JSON** in the observed Azeron Software 2.0.2 subset. Azerlay owns the
selection: a device-directory and profile-file reference chosen explicitly or
persisted by Azerlay. Discover lists admitted definitions, Load reads the
selected definition, and Watch reloads that same definition. This contract is
for Issue #22; the implementation now follows the boundary below.

This resolves R-003, R-004 and R-014 within UC-003/UC-006 and FR-031–037's
local-reading requirement. The amended research plan does not require complete
official writer disassembly, a transaction across all profile files, or
automatic following of the official UI's active/favorite selection. Independent
JSON files form a catalog of saved definitions, not a transactional device-state
snapshot. No Software 2.x family, firmware, model, hand, active device mode or
current on-board contents are admitted by this decision.

## Sources and limits

Research date: 2026-10-03; Linux x86-64 AppImage. The installed AppImage was
176718832 bytes. Runtime `--appimage-extract` exited 0; Python standard-library
ASAR reads inspected packaged code without installing dependencies. Real-store
JSON inspection reported only anonymous schema, types, counts and predicates;
no private IDs, labels, macros or raw data were copied or published.

| Primary material | Established evidence | Limit |
| --- | --- | --- |
| Packaged desktop / ASAR `package.json` | Desktop `2.0.2.442`, package `2.0.2`, product `Azeron Software` | Distribution metadata is not evidence of every file's last writer. |
| Renderer `out/renderer/assets/index-cLscnKLH.js`; preload `out/preload/preload.js` | Profile request/save IPC names, state getters/setters and IndexedDB persistence configuration | UI state and persistence expressions do not prove current physical state. |
| Seven real stored JSON objects and eleven history records | The closed admission predicates below hold for all seven; every history record asserts `softwareVersion:"2.0.2"` | Embedded assertions are not authentication, a last-writer guarantee or `.442` build attribution. |
| Regular-file-only IndexedDB WAL research | Reconstructed historical records corroborate `azeron-storage` / `store` / `sw-profile-store:sw-profile-store` and per-device metadata fields | No live DB library open, CRC verification or complete consistent DB view; not a production reader. |
| Owner-approved isolated official runtime | Actual userData path, GlobalStore adapter and main/backup ordering, migration-marker location | Synthetic settings only, USB and external network blocked. Profile save/load handlers were absent without a device; profile writer atomicity remains unknown. |
| Bundled lowdb 7.0.1 / steno 4.0.2 | Dependency APIs use per-file temporary publication; details below | Dependency presence does not establish the official profile writer's selected adapter. |

[Azeron's download page](https://azeron.com/pages/downloads) offers Linux Software
2.0.2. [Electron's app API](https://www.electronjs.org/docs/latest/api/app)
describes `userData` and permits overriding it. These sources provide
distribution/path context, not a local schema contract. Packaged main code is
V8 cached bytecode; bounded offline recovery did not recover profile handlers.
That limitation does not prevent reading stable admitted saved definitions.

## Observed release and store

The real root was `$HOME/.config/Azeron Software`. In isolated execution,
`app.getVersion()` returned `2.0.2` and `app.getPath("userData")` returned
`$XDG_CONFIG_HOME/Azeron Software` with an absolute synthetic XDG base. Desktop
and package labels agree on release; the `.442` suffix is desktop build metadata.

| Root-relative location | Observation and role |
| --- | --- |
| `Storage/DevicesStorage/<device>/ProfileStorage/profile_<id>.json` | One device directory, seven independent Software profile objects. All have integer version 1, `isSoftware:true`, filename/ID agreement, string name, 43 object inputs and nonempty changedLogs. Eleven total history records all assert Software 2.0.2. One stored `isFavorite` flag is true. These files supply the supported saved definitions. |
| `Storage/GlobalStore.json`, `Storage/GlobalStore.bak.json` | Each 151 bytes; equal JSON objects containing only `settings.SCREEN_PROPERTIES`, an object. Settings authority/backup, not the profile catalog. Not required for profile loading. |
| `StorageMigratedV2` | Isolated startup creates it directly under userData, beside `Storage`. Migration marker; not a release or schema admission requirement. Azerlay never creates it. |
| `IndexedDB/file__0.indexeddb.leveldb` | Browser-store directory coexists with JSON profiles. Holds selection/presentation metadata corroborated by static code and historical WAL research; excluded from the production read procedure. |
| `Local Storage/leveldb` | Coexisting browser storage; presence alone does not establish profile authority. |

### Closed local admission

Admit a **single JSON profile**, through the existing bounded decoder/raw
validation rules, only when all of these predicates hold:

* A nonempty string `id`; final basename exactly `profile_` + `id` + `.json`.
* `version` is integer **1**, not boolean, string, fractional number or null.
* `isSoftware` is boolean **true**; `name` is a string; `inputs` is an array of
  objects within existing raw-model limits.
* `metaData` is an object and `metaData.changedLogs` is a nonempty array of
  objects; **each** entry's `softwareVersion` is exactly the string `"2.0.2"`.

Missing, unknown or mixed history versions reject that selected file as
unsupported. History supplies a release-bearing assertion for this supported
subset; it does not authenticate origin, prove the most recent writer or admit
Software build `.442` specifically. File existence, valid JSON, the migration
marker, an installed app version or firmware alone does not satisfy admission.
The observed 43 inputs are evidence, **not** a required count. Optional
`profileIconId` and unknown fields remain owned/preserved by the raw model.

Use the existing [profile format limits](../profile-format.md): at most 16 MiB
source bytes, single-profile root, at most 256 inputs, valid UTF-8, no trailing
JSON or duplicate decoded keys, depth at most 64, decoded strings at most
65,536 UTF-8 bytes, and the existing normalization/macro bounds. A bundle,
wrong-shaped input, overflow or filename mismatch is rejected without partial
adoption. Device-directory metadata does not admit a keypad model, hand,
firmware or hardware mode.

Use a distinct adapter source scope, `azeron-software-local-json`, with Software
release `2.0.2`. Export `Normalize` still admits only
`azeron-software-export`; `NormalizeLocal` provides the separate complete local
admission boundary and reuses only
the existing [closed binding semantics](binding-conversion.md), preserving
Unknown for everything outside those evidenced rules. No new physical-device
matching or inferred binding conversion follows from a local file.

## Stored state semantics

The renderer `ae` store has in-memory `selectedProfileId` and `activeProfileId`.
`selectProfile` changes selection; `activateProfile` / `setActiveHwProfile`
change both. That store is not persisted with `Ls`. The `ht` store does persist
`activeDevicePid` and per-device `deviceData` using `Ls`, `Cs(()=>Xs(...))` and
IndexedDB `K0`: database `azeron-storage`, object store `store`, complete key
`sw-profile-store:sw-profile-store`, a JSON string with `state`/`version` envelope.

| State | Stored representation / update path | Supported interpretation |
| --- | --- | --- |
| Saved profile | The admitted independent JSON; renderer `ho` / `T8` issue `sw-profile-save` / `sw-profiles-save`, receipt uses `sw-profile-request` | Saved Software definition; not proof it is active or on-board. |
| Azerlay selection | Device directory plus final filename / profile ID, explicitly selected or persisted by Azerlay | Load/reload exactly this reference. IDs need not be globally unique across device directories. |
| Stored favorite flag | Profile `isFavorite` boolean; renderer `setProfileFavorite` then `ho` | Report the saved flag only. Do not use it as selection, current mode or complete ordered favorites. |
| Official last Software selection | `deviceData[pid].lastUsedSoftwareProfile` string; `c8` calls `setLastUsedSoftwareProfile` for a found Software profile | Historical last-used metadata, not followed by LocalSource. Missing/dangling values stay unknown. |
| Official favorites/order | `favoriteProfiles:[{profileId,sortIndex}]`, `lastFavoriteProfileId`, `profileOrderList`; favorite setters then `PS` sends `fav-profile-update` | Independent UI metadata; ordered favorites and official UI auto-follow are unsupported without safe external state access. |
| Official last mode | `lastDeviceMode`, `"hw"` or `"sw"`; `Pi` / `wf` call its setter | Saved choice only. Current Software/on-board mode remains unknown. |
| On-board profiles | Separate `EVENT_PROFILES`, hardware-index/state requests and device-originated data | Current contents/active slot unsupported; no HID access is added. |

Historical WAL reconstruction found a later Put for the exact persisted key
with `activeDevicePid:null`. Its device record had mode `sw`, a last-used
reference resolving to one JSON with stored favorite true, an empty
`favoriteProfiles` array, empty last-favorite reference and seven
`{profileId,sortIndex}` order entries. This corroborates the distinct
representations; it does not establish current DB state or device activity and
is not used for source selection.

Discover is a catalog operation: list admitted files with device/file refs.
Load requires an unambiguous explicit or persisted Azerlay reference; a bare
ID spanning multiple devices is ambiguous. Watch remains bound to that ref.
Missing/dangling selection never means “first profile” or “favorite profile”.
An initial auto request without a resolvable local selection uses import
fallback; choosing another local definition requires explicit selection.

## Writer and safe-read strategy

Profile writer adapter, locking, ordering and transaction completion remain
unknown. The strategy therefore admits a stable **saved definition**, without
claiming an official transaction boundary or a coherent snapshot of unrelated
profile/UI state.

Observed GlobalStore read stacks were `LowSync → JSONFileSync → TextFileSync`.
An actual isolated renderer `window.ipcRender.invoke("set-global-zoom", 1)`
completed with: `writeFileSync(.GlobalStore.json.tmp)` → `renameSync` to main →
`copyFileSync(main, GlobalStore.bak.json.tmp)` → `renameSync` to backup.
Bundled `TextFileSync` uses temp+rename; async `TextFile` delegates to
`steno.Writer`, which awaits temp write then rename. Steno's `#locked` is an
in-process flag, not an OS lock; these adapter bodies contain no fsync or
cross-file transaction. This is GlobalStore/dependency evidence, not proof of
profile writer atomicity.

The approved isolated invocation was `sh <task-owned-scratch>/launch.sh` with
`timeout 55s bwrap --unshare-all`, cleared environment, read-only libraries/app,
synthetic writable HOME, minimal `/dev`, private `/proc`/`tmp` and only the
owner's Wayland socket. Preflight confirmed synthetic HOME, real store
unreachable, no USB/hidraw/input, and external TCP errno 101. Passive filesystem
observers delegated original operations unchanged. Controller exit was 0;
the coordinator independently reviewed its ordered paths/stacks. Without USB,
profile save/request handlers were absent, so no profile writer claim is made.

### Selected-file procedure for Issue #22

1. Resolve the selected device/file reference inside the chosen userData root.
   Do not derive a path from an unvalidated arbitrary ID or silently change ref.
2. Open the final JSON with `O_RDONLY|O_CLOEXEC` and confirm with `fstat` that it
   is a regular file. Record descriptor device/inode, size, mtime and ctime;
   read through EOF into owned memory with the existing source bound. Never
   read a writer's temporary file or backup.
3. Compare descriptor metadata before/after and the current pathname's
   device/inode and metadata against the opened file. Reject a changed,
   replaced, missing or oversized candidate. Close the descriptor, then run
   complete strict JSON, local admission and versioned-adapter validation.
4. After the quiet/debounce interval, perform a second complete bounded read
   and validation of the **same ref**. Publish only when the two owned byte
   sequences are directly equal; compare bytes, not hashes or stamps.
5. On failure, use #22's bounded retry and last-known-good policy. Parent
   directory watching follows final-name rename/recreation; unrelated profiles
   do not invalidate a stable selected file or trigger selection changes.

No step creates a source file, lock, backup, copy, repair or DB handle. Ordinary
file reads do not obtain a destructive DB lock. A quiet interval and matching
reads detect observed changes; **they do not prove writer transaction
completion**. A valid intermediate JSON paused beyond the interval may be
accepted as a stable saved definition. Later changes reload the same selected
profile. Cross-file generations may differ because the catalog is independent;
do not fabricate shared-generation metadata or claim global coherence.

Never open live IndexedDB/LevelDB through a library or treat a running DB copy
as a safe snapshot. [LevelDB's documentation](https://github.com/google/leveldb/blob/main/doc/index.md)
describes one-process database access and snapshots within the opened DB; that
API does not authorize an external live-DB open. The research WAL parser is not
part of LocalSource.

## Synthetic checks

The QA executor ran `python3 /tmp/azerlay-issue12-qa.yAFJf4/research.py` on
2026-10-03: exit 0, seven scenarios passed. The coordinator independently read
the complete prototype and its meaningful assertions. All fixtures and IDs
were invented; no real store, official writer, USB or live DB was involved.
The temporary script and fixtures were subsequently removed.

The reader prototype uses `O_RDONLY|O_CLOEXEC`, bounded cap+1 reads, descriptor
and pathname metadata comparison, strict UTF-8/full JSON with duplicate-key
rejection, the selected admission predicates and modeled existing structural
bounds, then two reads with direct byte comparison. Its quiet interval was
**20 ms for the experiment only**; production debounce and bounded retry are
Issue #22 implementation responsibilities, without a new user setting here.
It is not production LocalSource or the full Go raw/semantic normalizer.
Fixtures were regular files; no general filesystem-security coverage is claimed.

| Scenario | Observed result / assertion |
| --- | --- |
| S1: normal directory 0555 / file 0444 | Complete admitted definition accepted using read-only opens. |
| S2: partial JSON, initial failure and recovery | Partial candidate rejected; first failure leaves no LKG; existing LKG remains identical; repaired stable definition accepted. |
| S3: final-name rename during descriptor read | Old candidate rejected after replacement; subsequent stable new definition accepted. |
| S4: same-length in-place change during/between reads | Both changed candidates rejected; no stale or unequal pair published. |
| S5: admission/selection failures | Version 2, boolean version, filename/ID mismatch, Software 2.0.3 history, duplicate keys and missing selected file rejected. |
| S6: unrelated profile changes | Stable selected definition accepted; another profile's update does not invalidate it or change selection. |
| S7: valid intermediate definition quiet for 20 ms | Accepted as a stable saved definition. Explicitly confirms the transaction-completion limitation rather than official atomicity. |

These original research cases validated the selected-file reader boundary and
LKG model. They did not exercise production integration, complete normalization
or parent watching; the implementation now covers those with synthetic tests.
Actual official writer transaction boundaries remain unclaimed.

## Explicit path contract

`local_store_path` denotes the **userData root**, not `Storage`, a
profile, backup or DB directory. Required relative layout is
`Storage/DevicesStorage/<device>/ProfileStorage/profile_<id>.json`; only the
selected file and its containing directories are required. GlobalStore,
backups, migration marker, icons and browser databases are not load dependencies.

Explicit path overrides automatic detection. Resolve relative paths from the
process working directory at initialization via `filepath.Abs`, consistent with
`internal/config/paths.go`; do not expand `~` or environment variables or create
directories. An incorrect/unreadable explicit root reports failure without
searching another root. Device/file components come from validated catalog refs,
not arbitrary path traversal or globally unique-ID assumptions.

Automatic detection checks an absolute `$XDG_CONFIG_HOME/Azeron Software`, then
`$HOME/.config/Azeron Software`. It uses the admitted JSON layout/predicates,
not marker or app installation presence. A persisted device/file ref must
resolve unambiguously; multiple matching roots or selections require explicit
choice. Detection alone never chooses the first/favorite file. Only an admitted,
stably loadable selected local profile enters source priority. Unsupported
diagnostics state the failed release/schema condition without private contents.

## Failure and fallback contract

Source priority is explicit selected imported ID > exact loadable local >
matching durable local last-good > last selected imported source > newest
loadable single-profile imported source when saved selection is unavailable,
following [design section 10.4](../design-research.md). Explicit selection is sticky;
subsequent failures do not silently choose another root, profile or source.

| Mode | Failure | Initial load without local last-good | Failure after successful local load |
| --- | --- | --- | --- |
| `auto` | Absent/unreadable root or unresolved/ambiguous local selection | Use ImportedSource priority; no valid import leaves unavailable/degraded. | Keep local LKG, report degraded. |
| `auto` | Unsupported release/schema/history | Same import fallback; explain unsupported and import procedure. | Keep LKG, report degraded and import procedure. |
| `auto` | Corrupt/partial/changed/missing selected file or dangling ref | Reject whole candidate and use import priority. | Reject candidate, keep LKG and report degraded. |
| Explicit `local` | Absent/unreadable root or unresolved/ambiguous selection | Unavailable; no implicit other-root/profile/source switch. | Keep LKG, report degraded. |
| Explicit `local` | Unsupported release/schema/history | Unavailable; explain unsupported and import procedure. | Keep LKG, report degraded and import procedure. |
| Explicit `local` | Corrupt/partial/changed/missing selected file or dangling ref | Unavailable; no fabricated LKG or choice. | Reject candidate, keep LKG and report degraded. |

An explicit selected source has priority over auto detection; its failure is
not hidden by selecting a different source. Official UI active/favorite/mode
metadata being unavailable does not invalidate an otherwise admitted
Azerlay-selected definition: report those states as unknown/unsupported.

**Current behavior:** local admission, exact configuration selection, stable
two-read loading, parent watching, bounded retries and durable matching
last-good are implemented. Explicit local never falls back; initial auto
fallback retains its local diagnostic. Auto skips multi-profile imports without
a saved ordinal and does not scan orphan originals on index failure. Successful
imported session selection stops local watching. Configuration reload preserves
initial source policy/ref; changed settings apply at the next start. See
[current profile configuration](../config.md#profile) for state paths,
permissions, timing and config examples.

Unsupported local data always offers the official Software export UI and
[current export CLI](../../README.md#profile-export-cli):
`azerlay import --software-release 2.0.2 --profile-index N <export-file> --json`
for an attributed supported multi-profile export; N is one-based. Read saved
imports with `azerlay profiles show --json`. Fallback uses existing valid imports;
it never manufactures an import from local data or bypasses export admission.

## Implementation boundary

Issue #22 implements the local-specific admission/adapter scope, device/file
selection,
Discover/Load/Watch, bounded retries, LKG and source priority described here.
Retain existing raw ownership, closed binding semantics and Unknown handling.
Do not add official UI auto-follow, live DB access, source writes/repair,
hardware queries or a cross-store transaction mechanism for this contract.

The original research was documentation-only. Production regression tests now
cover local admission, stable reads, watching, state and controller behavior;
the run CLI integration exercises update retention, rename repair and restart.
These synthetic scenarios do not qualify broader releases or actual official
writer transaction boundaries. No permanent research script or private dump is
part of the implementation.

Initial static/experimental materials and all approved isolated-runtime
processes, extracted app, controller, synthetic settings and temporary output
were removed after independent review. The real store and original Software
session were never changed, started, stopped or repaired.
