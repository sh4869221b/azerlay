# Linux Azeron local-store research

## Decision and scope

**Status: incomplete research; Issue #12 and the LocalSource implementation
blocker remain open.** This records the Linux AppImage release observed on
2026-10-03 and a proposed future source policy. It does not admit a local-store
adapter. The evidence identifies profile-shaped JSON files and separately
persisted selection metadata. An owner-approved isolated runtime observation
establishes GlobalStore's adapter and publication/backup ordering, but not the
profile adapter, profile update ordering, or a coherent view across stores.
Those gaps prevent a safe, supported LocalSource procedure from being specified.

This decision covers R-003, R-004, and R-014. LocalSource, configuration changes,
watching, controller changes, hardware reads, and real Software setting changes are
outside this research. No version-wide Software 2.x support follows from the
observed release. Current Azerlay behavior is described separately below.

## Sources and limits

The installed AppImage was inspected with `stat -c '%s'` (176718832 bytes), then
its runtime `--appimage-extract` operation in a task-owned temporary directory
(exit 0). Python standard-library `struct`, `json`, and file reads inspected the
ASAR header and extracted packaged static files. The initial static phase
executed no application entry point or bytecode loader. An explicitly approved
isolated runtime phase later executed the official application with synthetic
settings, blocked USB access and external networking, as described below.
No dependencies were installed.
Only anonymous structure, counts, types, and relationship predicates were
reported from the real store; its contents were never copied into scratch.

| Primary material | What it establishes | Limit |
| --- | --- | --- |
| AppImage `azeron-software.desktop`; ASAR `package.json` | Build label `2.0.2.442`, package release `2.0.2`, product name `Azeron Software` | Metadata identifies this distribution, not the version that last wrote every file. |
| ASAR `out/main/main-process.js`, `bytecode-loader.cjs`, `main-process.jsc` | Main source is packaged as V8 cached bytecode; loader calls its compiled wrapper | Plain strings in cached data do not establish calls, arguments, branches, or update order. |
| ASAR `out/preload/preload.js` | IPC channels for Software profile request/save/delete/activate and favorite updates | A channel name does not establish its main-process handler's storage behavior. |
| ASAR `out/renderer/assets/index-cLscnKLH.js` | Readable state getters/setters, IndexedDB wrapper and persistence configuration | Main handlers and device truth are separate; this is a minified bundle, so named functions and nearby expressions identify the evidence. |
| Bundled `lowdb` 7.0.1 and `steno` 4.0.2 | Exact dependency read/write APIs below | Presence and bytecode imports do not establish the adapter chosen for each official store. |
| Anonymous real-store structural inspection | JSON profile files coexist with Electron browser stores | No live DB was opened, repaired, copied, or decoded. |
| Isolated official application and passive filesystem observer | Runtime `app.getVersion()` / `app.getPath("userData")`, GlobalStore read/write adapter, main/backup publication ordering, marker location, and disconnected IPC registration | All writable settings were synthetic. No real profiles, device state or private store entered the sandbox. Profile save/load handlers were unavailable without USB devices. |

[Electron's app API](https://www.electronjs.org/docs/latest/api/app) describes
`userData` as app-specific configuration under `appData`, and permits an app to
change it. Isolated `app.getPath("userData")` returned the expected root under
the synthetic absolute `XDG_CONFIG_HOME`. The full main `getPath`/`setPath`
sequence remains unreadable, so this is an observed resulting path, not proof
that every environment or release uses an unmodified Electron default.
[Azeron's download page](https://azeron.com/pages/downloads) offers a Linux
Software 2.0.2 distribution; it is distribution context, not a schema contract.

A bounded offline recovery attempt used the existing, dependency-free
[v8asm parser](https://github.com/aynakeya/v8asm), which parses cached data without
loading V8. `python3 -m disassembler <main-process.jsc> --snapshot-blob
<snapshot_blob.bin> --format json` exited 1 because this V8 version has no bundled
profile. The extracted executable's static version strings identify Electron
40.9.2 / V8 14.4.258.32, consistent with the
[official Electron release](https://releases.electronjs.org/release/v40.9.2).
The tool's existing `generate_profiles.build_profile` was then given headers
from the official `v8/v8` tag `14.4.258.32`, using standard-library HTTP reads.
The first generation attempt exited 1 because its runtime-name extraction
macro accepted three arguments while this V8 source supplies four. A scratch-only
change made the unused tail variadic, preserving the extraction of the first
(name) argument; exact-version metadata generation then succeeded. The offline
parser still exited 1: with tagged size 4 it found no `BytecodeArray` candidates;
with tagged size 8 it encountered unsupported serializer tag `0x0f` at `0x12`.
No object-layout/parser repair, older-layout override, application execution or
runtime installation followed. No recovered handler body is available as
evidence.

## Observed release and store

Observation: Linux x86-64, AppImage distribution, 2026-10-03. Package release
`2.0.2` and desktop `2.0.2.442` agree on the release; package metadata does not
independently carry the `.442` build suffix.

The observed root corresponds to `$HOME/.config/Azeron Software` on this host.
The isolated official app returned `$XDG_CONFIG_HOME/Azeron Software` when
`XDG_CONFIG_HOME` was an absolute synthetic directory, and `app.getVersion()`
returned `2.0.2`. Other environment/base-path combinations remain unobserved.

| Root-relative location | Anonymous observation | Authority conclusion |
| --- | --- | --- |
| `Storage/GlobalStore.json`, `Storage/GlobalStore.bak.json` | Each 151 bytes in the real-store observation, JSON object with `settings` object | Isolated official runtime reads the final main file through `LowSync` / `JSONFileSync` / `TextFileSync` and refreshes the backup. A real renderer setting update publishes the main file before the backup. Corrupt-store restoration was not exercised; Azerlay must never restore it. |
| `Storage/DevicesStorage/<device>/ProfileStorage/profile_<profile>.json` | One device directory; seven JSON objects. All have string `id`, boolean `isSoftware`/`isFavorite`, integer `version`, and 43-element `inputs` arrays; every `isSoftware` is true. Every filename equals `profile_` plus its contained ID plus `.json`. Profile root key sets are not identical. | Strong candidate for persisted Software profile bodies, consistent with main constants `DevicesStorage`, `ProfileStorage`, `profile_` and `.json` and renderer IPC saves. Official path construction, reader and writer handlers still need confirmation. |
| `Local Storage/leveldb` | Directory exists | Presence does not establish profile authority. The persistence middleware's default is localStorage, but the relevant state overrides that default. |
| `IndexedDB/file__0.indexeddb.leveldb` | Directory exists | Renderer explicitly persists per-device Software selection/favorite metadata in IndexedDB. Association of this physical directory with that database was not decoded. |
| `StorageMigratedV2` | Isolated startup creates this file directly under `userData`, alongside `Storage` | A migration marker, not a demonstrated exact-release/schema admission token. Marker contents and lifetime do not establish which release last wrote a profile. |

The profile `version` values share a type and value in the observed files, but
that scalar is not a Software release identifier. No published local schema
revision or reliable marker binding this root to this exact build was
established. Runtime establishes the location and startup creation of
`StorageMigratedV2`, but not its full migration semantics or version-admission
role. Azerlay must neither create it nor use its existence as sole admission.

### Admission limits

A future detector must establish exact supported release attribution, its
root/schema predicate, and the complete read-only consistency procedure before
parsing local content. File presence, a successful JSON parse, a profile
`version` scalar, firmware, or an installed AppImage with matching metadata is
insufficient to establish which writer produced a store. A malformed synthetic
`"{"` failed `json.loads`; valid JSON without release/schema evidence remains
unadmitted as well.

The existing export normalizer in `internal/profileadapter/normalize.go` admits
only Software release `2.0.2` with scope `azeron-software-export`. That admission
and the [binding conversion evidence](binding-conversion.md) do not admit local
files with a similar shape. Local provenance and semantics need their own
established source scope. No local support claim is made here.

## Stored state semantics

The renderer's `ae` profile store starts with `profiles:[]`,
`selectedProfileId:"0"`, and `activeProfileId:"0"`. It uses `Mi` state middleware
without `Ls` persistence. `selectProfile` changes only the selected ID;
`activateProfile` and `setActiveHwProfile` change active and selected IDs. These
in-memory IDs are not an authoritative persisted active-device state.

The `ht` Software metadata store uses `Ls` with name `sw-profile-store`, storage
`Cs(()=>Xs("sw-profile-store"))`, and `partialize` retaining `activeDevicePid`
and `deviceData`. `Xs` prefixes the key with its namespace; `K0` opens IndexedDB
`azeron-storage`, object store `store`. The complete saved key is therefore
`sw-profile-store:sw-profile-store`, containing a JSON string with the middleware
`state`/`version` envelope (default middleware version 0).

| State | Stored type/location and updater | Meaning and missing-state handling |
| --- | --- | --- |
| Software profile body | Candidate per-profile JSON above; renderer `ho` invokes `sw-profile-save`, `T8` invokes `sw-profiles-save`, loading uses `sw-profile-request` | Persisted Software definitions are suggested, but main read/write authority remains unverified. Missing/invalid definitions cannot be replaced with synthetic defaults. |
| UI selected/active profile | In-memory `ae.selectedProfileId` / `ae.activeProfileId`; their setters and `EVENT_REPORT_ACTIVE_PROFILE` subscriber | Current Software session state. Persistence of the reported active ID in main is unverified. Never call a stored last-used ID the actual active device profile. |
| Last Software selection | `deviceData[pid].lastUsedSoftwareProfile`, string default `""`; `setLastUsedSoftwareProfile`, called by `c8` only for a found Software profile | Last saved Software choice. Startup Software-profile receipt uses it when saved mode is `sw`. Missing, duplicate or dangling references mean unknown/unavailable; do not infer selection from favorite status or first file. |
| Last mode | `deviceData[pid].lastDeviceMode`, string default `"hw"`; `setLastDeviceMode`; `Qo.HW="hw"`, `Qo.SW="sw"` | Software remembers a mode choice during `Pi`/`wf` flows. A default on hydration is not an observation of current hardware mode. |
| Favorite list | `deviceData[pid].favoriteProfiles`, array of `{profileId,sortIndex}`; `setFavProfileList`, `addFavProfile`, `removeFavProfile`, then `PS` invokes `fav-profile-update` | Ordered favorite choices. No favorite implies active selection. References require a unique matching profile. |
| Profile favorite flag / last favorite | JSON `isFavorite` boolean; metadata `lastFavoriteProfileId` string. `NS` has device-dependent branches updating either favorite list or `setProfileFavorite` + `ho` | Separate representations used by different device branches; they are not a universal selection authority. Applicable branch/device identity needs main/device evidence. |
| On-board profiles | `EVENT_PROFILES` adds hardware profiles with `addHwProfiles`; active hardware-index/state requests are separate IPC channels | Device-originated data. Persisted OBM icons/labels are presentation metadata; local files do not establish current on-board contents or active slot. No HID read is part of this decision. |

`getActiveDeviceData` returns null if `activeDevicePid` is null or absent.
Hydration merges stored device records with `ww()` defaults. The stored PID and
last-used reference therefore need validation and cannot substitute for an
observed connected-device state. The real IndexedDB values were not opened to
confirm a current selection, favorite count, or mode; static persistence
expressions establish their format, not their values or successful persistence
in this installation.

### Missing and unknown states

A neutral synthetic relation uses profiles `[{"id":"neutral-a","inputs":[]}]`
and `{"lastUsedSoftwareProfile":"neutral-a"}`. Exactly one match resolves.
An absent reference or `"neutral-absent"` resolves to unavailable/unknown, not a
favorite or the first profile. This is a proposed reader rule, not a report of
official startup behavior: readable `wf` can choose the first Software profile
when a last-used choice is missing. Azerlay must not silently reproduce that
mutation or present it as recorded state.

## Writer and safe-read strategy

Bundled `lowdb/lib/adapters/node/JSONFile.js` selects `JSON.parse` and
`JSON.stringify`; `DataFile.js` delegates to its text adapter. `TextFileSync`
reads the final filename and writes its same-directory `.<filename>.tmp` using
`writeFileSync`, then `renameSync` to the final filename. Async `TextFile` reads
the final filename and delegates writes to `steno.Writer`; steno writes the temp
file, awaits it, then awaits rename (with bounded rename retries).

The steno `#locked` flag is in-process serialization, not an OS file lock. These
adapter bodies contain no `fsync`, directory flush, or cross-file transaction.
They establish per-file publication behavior **if selected by a caller**, not
crash durability or bundle-wide atomicity. Isolated runtime below establishes
that GlobalStore selects the synchronous adapter and its backup ordering.
The corresponding profile adapter/handler remains unverified. Main's
`writeFileSync`, `writeFile`, `renameSync`, `lowdb` and `lowdb/node` constants
alone cannot establish the profile writer.

### Isolated official runtime observation

The owner explicitly approved this additional observation on 2026-10-03.
Invocation was `sh <task-owned-scratch>/launch.sh`: `timeout 55s bwrap
--unshare-all` with read-only system libraries and extracted app, writable
synthetic HOME and temporary output, minimal `/dev`, private `/proc` and `/tmp`,
and only the owner's Wayland display socket. The environment was cleared and
HOME / XDG_CONFIG_HOME set to synthetic paths. No host HOME, store, DBus socket,
USB bus, hidraw, input device or external network was mounted/exposed.

A Python preflight ran before each launch. Its captured predicates were
`home=/home/synthetic`, `real_store_reachable=false`, no `hidraw`/`input`/`bus`
entries under `/dev`, runtime sockets exactly `["wayland-1"]`, and external TCP
connect errno 101 (`ENETUNREACH`). The app then ran with `--no-sandbox
--disable-gpu --ozone-platform=wayland --inspect-brk=127.0.0.1:9229`; inspector
and its controller shared only the isolated loopback namespace.

Before main execution resumed, inspector installed passive filesystem
observers that recorded call/completion paths and stacks for synthetic paths.
Each wrapper delegated the original operation, arguments and return value; no
storage implementation, adapter, handler, device state or app package was
replaced. Read/write stacks and ordered operations were recorded in temporary
`operations.jsonl`; runtime paths/registration and preflight were captured
separately for the coordinator's independent review. Public evidence is retained
here after temporary cleanup.

Observed startup reads `GlobalStore.json` through `LowSync.read`,
`JSONFileSync.read`, and `TextFileSync.read`. On an existing valid synthetic
store it calls `copyFileSync(main, backup.tmp)`, then
`renameSync(backup.tmp, backup)`. Through an actual BrowserWindow renderer,
`window.ipcRender.invoke("set-global-zoom", 1)` completed successfully and
produced this exact call/completion order:

1. `TextFileSync.write` / `JSONFileSync.write` / `LowSync.write` call
   `writeFileSync(Storage/.GlobalStore.json.tmp, ...)`, then complete it.
2. `renameSync(Storage/.GlobalStore.json.tmp, Storage/GlobalStore.json)`
   completes main publication.
3. Main helper `Di`, called after `Si` writes, performs
   `copyFileSync(Storage/GlobalStore.json, Storage/GlobalStore.bak.json.tmp)`.
4. `renameSync(Storage/GlobalStore.bak.json.tmp, Storage/GlobalStore.bak.json)`
   completes backup publication.

The completed controller invocation exited 0. This proves the observed
GlobalStore setting path, not profile publication. With USB absent, 45 actual
IPC invoke handlers were registered, including `sw-profile-activate` and
`fav-profile-update`; `sw-profile-save`, `sw-profiles-save` and
`sw-profile-request` were absent. Read-only reflection of the activation
handler's closures established `Storage`, `DevicesStorage` and `ProfileStorage`
path bindings, but did not identify an independently callable profile storage
class/API. A final bounded inspection found 54 early function locations; cached
dummy source and obfuscated names did not identify their profile semantics.
No function was invoked by guessing its offset or signature, and no device was
fabricated. Profile save/load authority and ordering therefore remain blocked
on a reachable actual API or readable main implementation.

A candidate per-file procedure was evaluated: open only a final JSON file with
`O_RDONLY`, record `fstat`, read through EOF into owned memory, compare the
opened descriptor before/after and its identity with the current pathname,
close, then parse the entire JSON and validate the required profile shape.
Changes, missing files, invalid JSON and unresolved state references reject the
candidate and leave last-known-good untouched. It creates no file, lock, temp
file, or backup in the source root. Metadata comparisons are change detection,
not a proof that content or multiple files share a generation.

**This procedure is insufficient for a complete local snapshot.** Each profile
can parse correctly while a collection contains different writer generations;
selection metadata resides in a separate database. Neither two directory
listings, debounce, copying files, repeated mtime/size checks nor a read-only
open establishes a transaction across those stores. No such transaction or
published coherent snapshot API was found. Do not ship this candidate as a
LocalSource implementation.

A single explicitly selected profile need not inherently share a generation
with every independent profile file. Such a restricted future contract could
avoid unrelated cross-file state, but still needs proven profile authority,
writer behavior and release/schema admission. It would not supply the
unavailable active/favorite state or silently satisfy the complete candidate
procedure required by this approved plan. Read-only access prevents source
writes; it does not establish the candidate's authority or consistency.

Do not open the live LevelDB/IndexedDB through a database library, take its
lock, repair it, copy a running database as an alleged snapshot, or start an
Electron renderer to read it. [LevelDB's documentation](https://github.com/google/leveldb/blob/main/doc/index.md)
limits a database to one process and describes snapshots within the opened
DB; that snapshot API is not evidence for a safe external live-DB open.

## Synthetic checks

Invocation: `python3 <task-owned-scratch>/safe_read_experiment.py`, 2026-10-03,
exit 0. The temporary script used only `json`, `os`, `pathlib`, and `tempfile`.
All IDs, profile bodies and values were invented. No real store was copied.
`neutral_revision` was solely an experiment oracle, not a proposed local schema
field. Results below are the captured public evidence; they do not test the
official writer, a DB snapshot, or production Azerlay code.

| Scenario | Binary observable/result |
| --- | --- |
| Stable normal file with directory 0555 and file 0444 | `normal_readonly: accepted=true`; file opened `O_RDONLY`. |
| Truncate the open candidate during its read | `truncation: rejected=true last_good_revision=1`; descriptor change rejected. |
| Replace pathname with another synthetic file during its read | `rename_during_read: rejected=true last_good_revision=1`; descriptor change rejected. |
| Exactly one neutral last-used reference | `valid_reference: resolved=true`. |
| Missing / dangling neutral last-used reference | `missing_reference: resolved=false` and `dangling_reference: resolved=false`; last-good revision stayed 1. |
| Read first profile, update both synthetic files, then read second | `mixed_files: per_file_checks_accept=true mixed_revision=True coherent_snapshot_proven=false`. This is a failing consistency criterion, not a passing snapshot test. |
| Malformed detector JSON | `malformed_detector_input: parse_rejected=true`. |

The permission case ran as the ordinary user. It demonstrates that this reader
needs only file/directory read access; it does not prove writes are impossible
for the directory owner or cover inaccessible paths. The mixed-file experiment
models the observed multi-file structure; it does not assert that the official
writer uses the experiment's update operations or ordering. Complete candidate
adoption and last-good preservation for undetectable mixed generations remain
unverified because a valid coherence predicate is missing.

## Explicit path contract

This is a proposed future contract for #22, conditional on resolving admission
and coherent-read blockers. No `local_store_path` key is implemented today.

* An explicit path denotes the **Azeron `userData` root**, not `Storage`, one
  profile JSON, a backup, or a LevelDB directory. Candidate relative locations
  are those listed above; their required/optional roles cannot be finalized
  before main authority and supported state coverage are established.
* Resolve an explicit absolute path as supplied, or a relative path against the
  process working directory at initialization, using `filepath.Abs`, consistent
  with `internal/config/paths.go`. Do not expand `~`, environment variables, or
  infer a root from an arbitrary child file. Reading must not create the root.
* Explicit path overrides automatic detection. Missing, wrong-root,
  unreadable, unsupported or corrupt explicit content must report that result;
  do not hide it by scanning another root. Automatic detection can consider an
  absolute `$XDG_CONFIG_HOME/Azeron Software`, then
  `$HOME/.config/Azeron Software` under a version-specific detector. The exact
  valid predicate and multiple-root ambiguity handling are still blockers.
* Unknown build/schema never proceeds through guessed generic parsing. A
  supported diagnostic requires a verified release/schema and coherent read;
  unsupported explains which admission evidence is missing and offers export
  import. Diagnostics omit private root contents, IDs, labels and macros.

## Failure and fallback contract

The following is recommended **future** behavior after a supported detector and
reader exist. It does not make the currently unadmitted local files supported.

| Mode | Failure class | First load without local last-good | Failure after successful local load |
| --- | --- | --- | --- |
| `auto` | Absent/unreadable root | Try ImportedSource priority; no valid import leaves unavailable/degraded. | Retain local last-good, report degraded; do not silently change sources. |
| `auto` | Unsupported release/schema | Same import fallback; explain unsupported and import procedure. | Retain last-good, report degraded and import procedure. |
| `auto` | Corrupt/partial/update-in-progress/dangling state | Same import fallback; reject entire candidate. | Retain last-good, report degraded; reject entire candidate. |
| explicit `local` | Absent/unreadable root | Unavailable; no implicit imported or other-root switch. | Retain last-good, report degraded. |
| explicit `local` | Unsupported release/schema | Unavailable; explain unsupported and import procedure. | Retain last-good, report degraded and import procedure. |
| explicit `local` | Corrupt/partial/update-in-progress/dangling state | Unavailable; no fabricated last-good or selection. | Retain last-good, report degraded; reject entire candidate. |

Future default priority follows [design section 10.4](../design-research.md):
explicit selected source, admitted detected local source, last selected imported
source, then most recent valid imported source. Explicit selection failures are
not silently replaced with another selection. “Most recent valid” fallback is
future policy, not a claim that current initialization implements that search.

**Current implementation:** `internal/control/selection.go` immediately returns
`ERR_PROFILE_SOURCE_UNAVAILABLE` for `source="local"`; it never falls back.
`auto`/`imported` initialize from an exact configured `selected_id`, or the saved
import selection. A missing/corrupt selection does not currently search for a
new import or local source. `profile.watch` and local-store path remain future
behavior; configuration reload does not switch the initialized source policy.
See [current profile configuration](../config.md#profile).

For any unsupported local mode, use the official Software's export UI, then
follow [the current export CLI](../../README.md#profile-export-cli), for example
`azerlay import --software-release 2.0.2 --profile-index N <export-file> --json`
for a genuinely attributed supported multi-profile export (`N` is one-based).
Use `azerlay profiles show --json` to read the saved import. Import is an
explicit user operation; fallback does not manufacture an import from private
local-store data or bypass the export adapter's admission.

## Implementation boundary

This docs-only research changes no product code, configuration or dependencies.
Local Go tests and builds were not run; existing hosted CI may run for the PR.
`git diff --check` must pass before delivery. This
single decision is the evidence artifact; no separate ledger, manifest,
permanent script, private dump, or routine digest is part of the change.

Remaining requirements before #12 can close and #22 can implement LocalSource:

1. Exact-release main code, readable offline disassembly or an actual reachable
   profile API must establish profile read/write authority, selected adapter,
   migration behavior and update order. Runtime established the observed
   `userData` root and GlobalStore path, adapter and backup order; those findings
   do not establish the profile writer.
2. Establish local-store release/schema admission, including roots left behind
   by upgrades; installed binary metadata alone is insufficient.
3. Establish a coherent external read-only view of all required profiles and
   persisted selection/favorites, or an official atomic snapshot interface.
   Confirm whether a documented restricted subset can omit DB state while
   preserving the required source-selection contract. No live DB open or
   application mutation is authorized as a shortcut.
4. Exercise that actual complete procedure with synthetic concurrent update,
   corruption and dangling-reference cases, rejecting mixed/partial candidates
   without replacing last-good. The current mixed-file result fails this
   criterion, so IS-1/IS-3 and overall plan completion are not claimed.

The initial static extract/experiment/tool sources were removed after
independent review and rerun of the synthetic experiment. After the coordinator
independently reviewed the runtime evidence, all owned runtime processes were
confirmed absent and the additional extract, controller, synthetic settings
and temporary runtime evidence were removed. The real source store and original
Software session were never changed, started, stopped or repaired; only the
explicitly approved isolated application used synthetic settings.
