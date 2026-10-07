# Public-release license and safety decision

Date: 2026-10-07. Review baseline: main
`63e1c4da271029ed758ebad97cce6745897467e8`.

## Decision

The owner selected MIT for project-owned material, conditional on compatibility.
MIT is compatible in principle with the documented combination of permissive,
MPL and LGPL dependencies when their separate conditions are met. This decision
supersedes the earlier MPL-2.0 recommendation. It does not relicense third-party
code, certify all rights, or authorize publication, CI execution or a release.

`LICENSE` supplies the MIT terms. `NOTICE`, `THIRD_PARTY_NOTICES.md` and
`LICENSES/` retain third-party notices, source locations and reviewed license
texts. Source and binary archives and the Arch package include them. Source-file
headers may be added when appropriate; do not invent ownership or replace
upstream headers. Existing original project material is covered by the root
license; future contributions should be submitted on the same terms, with their
provenance and any exceptions identified during review.

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

## Remaining source-publication checks

The source-publication questions are rights/provenance, the official-software
layout, trademark presentation and the separate exposure audit described below.
Final native SBOM and executable relinking checks apply to binary distribution;
they do not by themselves prohibit publishing original MIT source.

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
   and release require a later explicit action; no Actions run is requested here.

Issues #18 and #39 remain open; this preparation does not mark their release
acceptance criteria complete. This is a practical engineering review, not a legal
opinion or guarantee of non-infringement.
