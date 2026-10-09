# Public-release license and safety decision

Decision record updated: 2026-10-09 JST. Original review baseline: main at
`63e1c4da271029ed758ebad97cce6745897467e8`, reviewed 2026-10-07. Task 3
integration baseline: checkout `e2da64c`, reviewed 2026-10-09 JST.

## Decision

The owner selected MIT for project-owned material, conditional on compatibility.
MIT is compatible in principle with the documented combination of permissive,
MPL and LGPL dependencies when their separate conditions are met. This decision
supersedes the earlier MPL-2.0 recommendation and the outdated statement in
[Issue #18](https://github.com/sh4869221b/azerlay/issues/18) that the license is
undecided, including its historical instruction not to add a root LICENSE
before this decision. It does not relicense third-party code, certify all
rights, or authorize publication, CI execution or a release.

`LICENSE` supplies the MIT terms. `NOTICE`, `THIRD_PARTY_NOTICES.md` and
`LICENSES/` retain third-party notices, source locations and reviewed license
texts. The source and binary archive recipes and the Arch package include
these materials. Source-file headers may be added when appropriate; do not
invent ownership or replace upstream headers. Existing original project
material is covered by the root license; future contributions should be
submitted on the same terms, with their provenance and any exceptions
identified during review.

## Name and trademark review (2026-10-09 JST)

The live GitHub repository-name search
(`gh api -X GET search/repositories -f q='azerlay in:name' --jq '{total_count,names:[.items[].full_name]}'`)
returned `{"names":["sh4869221b/azerlay"],"total_count":1}`. The repository
metadata query
(`gh api repos/sh4869221b/azerlay --jq '{full_name,private,license:.license.spdx_id}'`)
returned `{"full_name":"sh4869221b/azerlay","license":"MIT","private":false}`.
The source repository is
already public. This search is limited to GitHub repository names; it is not a
general name-availability or trademark-register search, and it does not clear
the Azerlay name. Existing source visibility does not authorize or qualify a
future binary release.

On 2026-10-09 JST, the owner approved retaining the Azerlay name and the existing
unofficial-project wording in both README and NOTICE:

> Azerlay is an independent, unofficial project and is not affiliated with,
> endorsed by, or sponsored by Azeron SIA. Azeron and Cyborg are trademarks
> of their respective owner.

This records the owner's project decision, not permission from the trademark
owner or legal clearance. The separate limits in NOTICE remain in force: the
Azerlay license grants no rights to third-party trademarks, official artwork,
software or user-provided profiles.

The primary [Azeron Terms of Service](https://azeron.com/policies/terms-of-service)
was reviewed on 2026-10-09 JST. It identifies Azeron SIA as operator of its
website and defines its Service around that site. Section 2 requires express
written permission to reproduce, copy, sell, resell or exploit any portion of
that Service or access to it. The official
[Azeron Software page](https://azeron.com/pages/software) advertises and links
to Software 2.0.2 downloads, but its reviewed page text contains no EULA or
software license (`EULA` and `license` searches returned no matches). These
website terms do not establish a Software EULA or grant rights to reproduce
official software, artwork, marks or schematic expression. The installer and
any click-through terms were not inspected, so software terms, if any, remain
unverified.

## Read-only findings at the baseline

- The exact Go requirements were inspected at their upstream version tags.
  gotk4's generated `pkg` is MPL-2.0, distinct from its AGPL generator. Its cairo
  README label and one source header differ; both facts are documented rather
  than silently rewriting the actual notice.
- Native archive packaging uses separately installed shared libraries. GTK
  4.22.5 and gtk4-layer-shell 1.3.0 are the documented tested tuple; other exact
  native component versions have not been established for a final release.
- The recursive main tree contained no files with SVG, PNG, JPEG, GIF, TTF, OTF,
  WOFF or XML extensions. This is an inventory observation, not proof that no
  protected material is embedded in another format.
- The layout comprises original JSON schematic geometry informed by the
  observed official Software layout and physical mapping. The existing research
  documents record inspection of an installed official application. No claim is
  made that this automatically establishes all copyright, contract or design
  rights needed for public distribution.
- Existing fixture policy requires minimal synthetic structures, not private
  exports. Research provenance should remain factual, without copying official
  implementation code, images or unnecessary user data into the release.

## Dependency license report and bounded SBOM draft (2026-10-09)

This inventory is for the current source checkout and the dependencies declared
by the Arch recipe. It is a review aid, not a machine-readable SBOM or proof of
what a final binary contains. Rows marked pending require the release build and
package set to be inspected before distribution.

The current-checkout scenario used `go list -m all` (exit 0) to resolve the
module graph and `go list -deps ./cmd/azerlay` (exit 0) to observe packages
reachable from the executable target. That target lists packages from gotk4,
fswatcher, go-toml, xz, urfave/cli and x/sync. It does not list packages from
purego, x/sys or x/text. The result is Go package reachability for this source
target, not final-binary inspection. The module graph also contains x/mod
v0.35.0 and x/tools v0.44.0 through x/text; neither appeared in the target
package list. `go list -deps -test ./...` (exit 0) did not list package paths
for the four additional go.sum entries below. CI pins go-licenses v2.0.1 in
`.github/ci-tools.env` and runs `go-licenses report ./... --include_tests`; its
CSV is a tests-inclusive Go license report, not a native dependency inventory
or artifact-specific SBOM.

The table uses exact requirement versions from `go.mod`, checks the existing
license copies against the cached source at those versions, and records the
role observed by the commands above. All compared module license copies match
their corresponding source files except gotk4's MPL text, whose only difference
is one trailing space in the upstream copy. gotk4's separate ISC and BSD files
match their source copies. Its cairo header discrepancy and LGPL C exception
remain described in `THIRD_PARTY_NOTICES.md`; whether that C file is in the
final binary is still pending.

| Component | Version | Role | License / exception | Upstream source | Notice location | Verified / pending |
| --- | --- | --- | --- | --- | --- | --- |
| `github.com/diamondburned/gotk4/pkg` | v0.4.1 | Go packages reachable from the source target (generated bindings) | MPL-2.0; `pkg/core` ISC; cairo ISC header; cairo/swizzle BSD-3-Clause; `gioutil/gdkarrayimpl.c` LGPL-2.1-or-later exception. Upstream says the AGPL `gir/` generator license does not apply to generated output. | [pkg v0.4.1](https://github.com/diamondburned/gotk4/tree/pkg/v0.4.1/pkg) | `THIRD_PARTY_NOTICES.md` gotk4 sections; `LICENSES/gotk4-pkg-MPL-2.0.txt`, `gotk4-core-ISC.txt`, `gotk4-cairo-ISC.txt`, `gotk4-cairo-swizzle-BSD-3-Clause.txt`, `gotk4-gdkarrayimpl-NOTICE.txt`, `LGPL-2.1-or-later.txt` | Exact-version license files compared; MPL copy differs only by trailing whitespace. The source package graph was observed; final-binary composition and whether the LGPL C exception applies remain pending. |
| `github.com/fswatcher/fswatcher` | v0.1.0 | Go packages reachable from the source target | MIT | [v0.1.0](https://github.com/fswatcher/fswatcher/tree/v0.1.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/fswatcher-MIT.txt` | Exact-version license copy compared; source package reachability observed. |
| `github.com/pelletier/go-toml/v2` | v2.4.3 | Go packages reachable from the source target | MIT | [v2.4.3](https://github.com/pelletier/go-toml/tree/v2.4.3) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/go-toml-MIT.txt` | Exact-version license copy compared; source package reachability observed. |
| `github.com/ulikunitz/xz` | v0.5.16 | Go packages reachable from the source target | BSD-3-Clause | [v0.5.16](https://github.com/ulikunitz/xz/tree/v0.5.16) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/xz-BSD-3-Clause.txt` | Exact-version license copy compared; source package reachability observed. |
| `github.com/urfave/cli/v3` | v3.11.0 | Go packages reachable from the source target | MIT | [v3.11.0](https://github.com/urfave/cli/tree/v3.11.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/urfave-cli-MIT.txt` | Exact-version license copy compared; source package reachability observed. |
| `golang.org/x/sys` | v0.44.0 | Module graph only; no package in target list | BSD-3-Clause | [v0.44.0](https://github.com/golang/sys/tree/v0.44.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/golang-BSD-3-Clause.txt` | Exact-version license copy compared; executable inclusion not observed and final artifact remains pending. |
| `github.com/ebitengine/purego` | v0.10.0 | Indirect module graph only; no package in target list | Apache-2.0 | [v0.10.0](https://github.com/ebitengine/purego/tree/v0.10.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/purego-Apache-2.0.txt` | Exact-version license copy compared; executable inclusion not observed and final artifact remains pending. |
| `golang.org/x/sync` | v0.20.0 | Go packages reachable from the source target through gotk4 core | BSD-3-Clause | [v0.20.0](https://github.com/golang/sync/tree/v0.20.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/golang-BSD-3-Clause.txt` | Exact-version license copy compared; source package reachability observed. |
| `golang.org/x/text` | v0.37.0 | Indirect module graph only; no package in target list | BSD-3-Clause | [v0.37.0](https://github.com/golang/text/tree/v0.37.0) | `THIRD_PARTY_NOTICES.md` Go modules; `LICENSES/golang-BSD-3-Clause.txt` | Exact-version license copy compared; executable inclusion not observed and final artifact remains pending. |
| Go runtime | Local `go version`: `go1.27.1-X:nodwarf5 linux/amd64`; release version pending | Runtime | BSD-3-Clause and Go patent grant | [Go 1.27.2 source cited by current notice](https://github.com/golang/go/tree/go1.27.2) | `THIRD_PARTY_NOTICES.md` Go runtime paragraph; `LICENSES/go-runtime-BSD-3-Clause.txt`, `go-runtime-PATENTS.txt` | Local toolchain observation recorded. It differs from the notice's 1.27.2 reference; release toolchain and corresponding runtime source remain pending. |
| `github.com/davecgh/go-spew` | v1.1.1 | go.sum source/test provenance; no package in target list | ISC | [v1.1.1](https://github.com/davecgh/go-spew/tree/v1.1.1) | `THIRD_PARTY_NOTICES.md` Additional entries; `LICENSES/go-spew-ISC.txt` | Existing notice and exact version recorded; not observed in the executable target or repository test package list. |
| `github.com/pmezard/go-difflib` | v1.0.0 | go.sum source/test provenance; no package in target list | BSD-3-Clause | [v1.0.0](https://github.com/pmezard/go-difflib/tree/v1.0.0) | `THIRD_PARTY_NOTICES.md` Additional entries; `LICENSES/go-difflib-BSD-3-Clause.txt` | Existing notice and exact version recorded; not observed in the executable target or repository test package list. |
| `github.com/stretchr/testify` | v1.11.1 | go.sum source/test provenance; no package in target list | MIT | [v1.11.1](https://github.com/stretchr/testify/tree/v1.11.1) | `THIRD_PARTY_NOTICES.md` Additional entries; `LICENSES/testify-MIT.txt` | Existing notice and exact version recorded; not observed in the executable target or repository test package list. |
| `gopkg.in/yaml.v3` | v3.0.1 | go.sum source/test provenance; no package in target list | MIT / Apache-2.0 upstream notices | [v3.0.1](https://github.com/go-yaml/yaml/tree/v3.0.1) | `THIRD_PARTY_NOTICES.md` Additional entries; `LICENSES/yaml-v3-LICENSE.txt` | Existing notice and exact version recorded; not observed in the executable target or repository test package list. |
| `go-licenses` | v2.0.1 | CI reporting tool only | Tool's own terms do not become application runtime dependencies | [v2.0.1](https://github.com/google/go-licenses/tree/v2.0.1) | `.github/ci-tools.env`; CI uploads a Go license CSV; not packaged as an Azerlay notice | CI tool version and command observed; it is not part of the application dependency inventory. |
| GTK 4 | 4.22.5 documented qualification; 4.24.1 observed by `pkg-config`; release version pending | Native shared library | LGPL-2.1-or-later | [GTK 4.22.5](https://github.com/GNOME/gtk/tree/4.22.5) | `THIRD_PARTY_NOTICES.md` Native shared libraries; `LICENSES/LGPL-2.1-or-later.txt` | Qualification license/version and current host version recorded; actual release package, linkage and LGPL replacement behavior pending. |
| gtk4-layer-shell | 1.3.0 documented and observed by `pkg-config`; release package version pending | Native shared library | MIT | [v1.3.0](https://github.com/wmww/gtk4-layer-shell/tree/v1.3.0) | `THIRD_PARTY_NOTICES.md` Native shared libraries; `LICENSES/gtk4-layer-shell-MIT.txt` | Exact documented and current host version recorded; final package and binary linkage pending. |
| Pango | 1.58.2 observed by `pkg-config`; release version pending | Native shared library | LGPL-2.1-or-later | [Pango source](https://gitlab.gnome.org/GNOME/pango) | `THIRD_PARTY_NOTICES.md` Native shared libraries; `LICENSES/LGPL-2.1-or-later.txt` | License family recorded; exact release package and final linkage pending. |
| Cairo | 1.18.6 observed by `pkg-config`; release version pending | Native shared library | LGPL-2.1 or MPL-1.1; this preparation follows the LGPL route | [Cairo project](https://cairographics.org/) | `THIRD_PARTY_NOTICES.md` Native shared libraries; `LICENSES/LGPL-2.1-or-later.txt` | Selected notice route recorded; exact release version, terms and final linkage pending. |
| Other Arch `depends`: `libgirepository`, `glib2`, `harfbuzz`, `gdk-pixbuf2`, `vulkan-icd-loader`, `graphene`, `glibc` | Current host pkg-config: gobject-introspection 1.86.0, GLib 2.90.1, HarfBuzz 14.6.0, GdkPixbuf 2.44.8, Vulkan 1.4.363, Graphene 1.10.8; glibc version unavailable through pkg-config | Native package dependencies | Exact package license expressions and file-level exceptions remain to verify for the release package set | [gobject-introspection](https://gitlab.gnome.org/GNOME/gobject-introspection); [GLib](https://gitlab.gnome.org/GNOME/glib); [HarfBuzz](https://github.com/harfbuzz/harfbuzz); [GdkPixbuf](https://gitlab.gnome.org/GNOME/gdk-pixbuf); [Vulkan Loader](https://github.com/KhronosGroup/Vulkan-Loader); [Graphene](https://github.com/ebassi/graphene); [glibc](https://sourceware.org/git/glibc.git) | `packaging/arch/PKGBUILD` `depends`; `THIRD_PARTY_NOTICES.md` Native shared libraries identifies the pending native scope | Package names and available host pkg-config versions observed. Exact release versions, terms, notices, transitive native closure and binary relationships pending. |

The pkg-config observation used `pkg-config --modversion gtk4
gtk4-layer-shell-0 glib-2.0 gobject-introspection-1.0 pango harfbuzz
gdk-pixbuf-2.0 cairo vulkan graphene-1.0` and returned, in that order,
`4.24.1`, `1.3.0`, `2.90.1`, `1.86.0`, `1.58.2`, `14.6.0`, `2.44.8`,
`1.18.6`, `1.4.363`, and `1.10.8`. A separate `pkg-config --modversion glibc`
query reported that no `glibc.pc` file is available. These are observations of
the current host, not the final Arch package or a release dependency closure.

## Issue #18 decision and acceptance map (2026-10-09 JST)

The live `gh issue view 18 --json body,state,url` query returned `state: OPEN`;
`gh api repos/sh4869221b/azerlay/issues/18/dependencies/blocked_by --jq '[.[]|{number,state}]'`
returned Issue #10 as `CLOSED`. The live `gh issue view 39 --json body,state,url`
query returned `state: OPEN`. These are tracker observations from this review,
not closure or release authorization.

| Issue #18 acceptance / decision | Evidence and result in this record | Remaining state |
| --- | --- | --- |
| The safety decision is recorded | MIT is selected for project-owned material in the Decision section; the 2026-10-07 main review baseline and 2026-10-09 integration baseline are distinguished above. | This decision record supplies the required research artifact; delivery verifies its committed state and the tracker status separately. |
| License status is decided or explicitly remains blocked | The owner-selected MIT decision supersedes the old MPL recommendation and stale “undecided” wording. Third-party conditions and the limits of this decision are stated. | The project license choice is decided; compatibility and rights conditions for actual distribution remain scoped to the components and artifact being distributed. |
| Dependency license report and SBOM expectations are listed | The source-checkout dependency report and bounded SBOM draft above identify reviewed and pending entries. The #39 contract below specifies the release artifact evidence still required. | Report and draft criteria are met as document content; there is no final, machine-readable SBOM for a release artifact. |
| Trademark disclaimer wording is approved for docs | The owner approved retaining the existing wording on 2026-10-09 JST; the quoted text is present in README and NOTICE. | The project wording decision is recorded. It grants no trademark permission and is not legal clearance. |

The research and decision criteria for #18 are recorded here. Delivery verifies
the required commit and tracker state separately; Issue status is not inferred
from this document. The source repository is already public; this decision
neither changes that fact nor qualifies a future binary distribution.

## Issue #39 release-artifact NOTICE and SBOM contract

For each release artifact actually produced, its work must select either SPDX
or CycloneDX at generation time and produce a machine-readable document valid
for that format's schema. This bounded source
inventory above is not that deliverable. A tests-inclusive `go-licenses` CSV
does not establish the contents of a binary or its native dependency closure.

Alongside the SBOM, #39 must produce a license report for the same artifact and
build. It must list each shipped Go and native component with its version,
license and exceptions, source location, applicable notice/license text, and
any unresolved review status, and reconcile its component set with the SBOM.
The existing `go-licenses` CSV may inform the Go portion, but it includes test
packages and does not cover the native graph, so it cannot stand alone as this
release report.

The artifact record must identify the exact artifact version and target, Go
toolchain and relevant build configuration, resolved Go module graph, and the
actual native shared-library/package graph for that build. It must identify each
component and version, source or supplier location, applicable license and
file-level exception, and direct or transitive relationship to the application.
It must distinguish shipped application/runtime components from build-only,
test-only and host-provided components rather than treating every module or
installed host package as shipped content.

Unknown versions, licenses, file exceptions, copied or generated source
provenance, and uncertain component relationships must remain marked for review
until resolved against the artifact and its sources. The generated record and
the actual source/binary/package contents must be checked together. The release
must carry `NOTICE`, `THIRD_PARTY_NOTICES.md` and `LICENSES/` in the relevant
source archive, binary archive and Arch package, as the current packaging code
does. Where an applicable license requires corresponding source, modification
information, replacement libraries or relinking materials, the release must
include the required materials for the actual build and distribution form.
Static or bundled native libraries, a changed linkage model, or other material
packaging changes require a fresh license and relinking review; the current
shared-library notice set is not approval for those cases.

The present packaging paths were checked directly: `scripts/package.sh` copies
the legal files into both source and binary archive roots, and
`packaging/arch/PKGBUILD` installs them below `/usr/share/licenses/azerlay/`.
This confirms the current recipes' inclusion rules, not the contents of any
release archive or package, which have not been built and inspected here.

## Remaining conditions before public distribution

The source repository is already public. The rights/provenance, official-software
layout, trademark presentation and separate privacy/history review below are
unresolved conditions for formal public distribution; they are not prerequisites
to the repository's existing visibility. Final native SBOM and executable
relinking checks apply to binary distribution.

### Checks before binary distribution, plus publication-safety review

1. Record the exact release tree, Go toolchain, module graph and actual native
   dependencies; prepare an artifact-specific SBOM and review unknown licenses,
   file-level exceptions, bundled data and generator/copied-code provenance.
   A successful go-licenses CSV job alone is insufficient.
2. The gotk4 cairo label discrepancy and nested ISC/BSD/file-level LGPL notices
   are documented and retained. Preserve their actual terms; do not flatten them
   to the module root license. Retain corresponding MPL source
   availability for the actual linked version, including any modifications.
3. Verify native LGPL terms, replacement behavior and any required source or
   relinking materials for the actual artifact, including the gotk4 gioutil C
   exception if the actual build incorporates it. Its import was not found in
   the current source inspection; binary reachability remains to be recorded.
   New static/bundled distribution
   needs a fresh review. The present notices are not blanket approval for it.
4. Confirm ownership/permission for external contributions and third-party
   expression, including AI-assisted copied material if any. Review the relevant
   official-software terms and the schematic's provenance; do not assume fair
   use or infer permission merely from public accessibility. Prefer independently
   designed geometry with documented provenance or verified permission where
   protected third-party expression is reused. A missing public Software EULA
   is not a grant of rights.
5. Review the Azerlay name and trademark risk. Keep the existing unofficial
   disclaimer in README, notices and any future About/distribution page. A GitHub
   name or disclaimer alone is not trademark clearance. Desktop identity remains
   unresolved and its template must not be installed as a finished launcher.
6. Complete the separate privacy/history/artifact audit and inspect the actual
   final source and binary archives, checksums and license inclusion. Publication
   and release require a separate explicit action; this decision does not
   authorize release publication.

Issue #18 and Issue #39 were both OPEN when queried on 2026-10-09 JST. These are
dated tracker observations; delivery verifies tracker completion separately.
This decision does not authorize release publication or claim public-release
readiness. It is an engineering review, not a legal opinion or guarantee of
non-infringement.
