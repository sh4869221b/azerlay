# Third-party notices and source availability

Azerlay's MIT license covers only material its authors have the right to license.
It does not replace the licenses of dependencies, copied notices or user data.
This inventory was prepared on 2026-10-07 against Azerlay main
`63e1c4da271029ed758ebad97cce6745897467e8`. It is not a completed, artifact-specific
SBOM or a claim that every native transitive component has been audited.

The license texts under `LICENSES/` are part of both source and binary archives
and the Arch package. Preserve this file, `NOTICE` and those texts when
redistributing. Upstream source files retain their own copyright notices.

## Go modules

These are the exact requirements in `go.mod`, including indirect requirements.
The links identify corresponding upstream source, not a floating latest release.
This table is maintained separately from the historical review above; see
[the dependency-notice update and review policy](docs/ci.md#go-dependency-notices).

<!-- BEGIN GENERATED GO MODULES -->
| Component and version | Terms / notice file | Corresponding source |
| --- | --- | --- |
| github.com/diamondburned/gotk4/pkg v0.4.1 | MPL-2.0, with core/cairo and LGPL C exceptions below; `LICENSES/gotk4-pkg-MPL-2.0.txt` | https://github.com/diamondburned/gotk4/tree/pkg/v0.4.1/pkg |
| github.com/fswatcher/fswatcher v0.1.0 | MIT, Yasuhiro Matsumoto; `LICENSES/fswatcher-MIT.txt` | https://github.com/fswatcher/fswatcher/tree/v0.1.0 |
| github.com/pelletier/go-toml/v2 v2.4.3 | MIT, Thomas Pelletier; `LICENSES/go-toml-MIT.txt` | https://github.com/pelletier/go-toml/tree/v2.4.3 |
| github.com/ulikunitz/xz v0.5.16 | BSD-3-Clause, Ulrich Kunitz; `LICENSES/xz-BSD-3-Clause.txt` | https://github.com/ulikunitz/xz/tree/v0.5.16 |
| github.com/urfave/cli/v3 v3.11.0 | MIT, urfave/cli maintainers; `LICENSES/urfave-cli-MIT.txt` | https://github.com/urfave/cli/tree/v3.11.0 |
| golang.org/x/sys v0.49.0 | BSD-3-Clause, The Go Authors; `LICENSES/golang-BSD-3-Clause.txt` | https://github.com/golang/sys/tree/v0.49.0 |
| github.com/ebitengine/purego v0.11.1 | Apache-2.0; `LICENSES/purego-Apache-2.0.txt` | https://github.com/ebitengine/purego/tree/v0.11.1 |
| golang.org/x/sync v0.20.0 | BSD-3-Clause, The Go Authors; `LICENSES/golang-BSD-3-Clause.txt` | https://github.com/golang/sync/tree/v0.20.0 |
| golang.org/x/text v0.37.0 | BSD-3-Clause, The Go Authors; `LICENSES/golang-BSD-3-Clause.txt` | https://github.com/golang/text/tree/v0.37.0 |
<!-- END GENERATED GO MODULES -->

The Go project additionally provides the patent grant reproduced in
`LICENSES/golang-PATENTS.txt`. Runtime source and notices for the currently
specified Go 1.27.2 toolchain are available at
https://github.com/golang/go/tree/go1.27.2 ; see
`LICENSES/go-runtime-BSD-3-Clause.txt` and `LICENSES/go-runtime-PATENTS.txt`.
Record the actual toolchain when producing a release and update this entry if
it differs. Check notices for bundled code/data inside modules as well as their
root license files; a module-level inventory is not a file-level clearance.

### gotk4 MPL-covered source

The MPL-covered portions incorporated into Azerlay executables are available in
Source Code Form under MPL-2.0 at the exact gotk4 source link above. No local
modification or `replace` of that module is specified in this checkout. Recipients
may obtain that source from the public repository without charge. If a distributor
modifies these portions, it must provide the corresponding modified source under
MPL-2.0 and update this notice. Do not impose terms restricting recipients' MPL
source rights. For a release, retain an accessible corresponding-source copy so
this offer does not depend solely on the continued availability of upstream.

The upstream generator under `gir/` is AGPL-licensed; upstream explicitly says
that this generator license does not apply to its generated output. Azerlay
requires the separate `pkg` module. This does not grant permission to redistribute
or incorporate generator code under MIT.

### gotk4 nested licenses

The module also contains separately licensed subdirectories at the same version:

- `pkg/core` is generally ISC, copyright (c) 2021 diamondburned, except for
  the file-level notices described below, under
  `LICENSES/gotk4-core-ISC.txt`; original:
  https://github.com/diamondburned/gotk4/blob/pkg/v0.4.1/pkg/core/LICENSE .
- `pkg/cairo/swizzle` is BSD-3-Clause, copyright (c) 2009 The Go Authors,
  under `LICENSES/gotk4-cairo-swizzle-BSD-3-Clause.txt`; original:
  https://github.com/diamondburned/gotk4/blob/pkg/v0.4.1/pkg/cairo/swizzle/LICENSE .

The swizzle source headers additionally carry Copyright 2015 The Go Authors.
Preserve that attribution alongside the directory license's 2009 notice.

`pkg/core/glib/glib.go.h` carries the same Conformal Systems ISC notice as
`pkg/cairo/cairo.go`; `LICENSES/gotk4-cairo-ISC.txt` preserves the notice for
both files. Source:
https://github.com/diamondburned/gotk4/blob/pkg/v0.4.1/pkg/core/glib/glib.go.h .

`pkg/core/gioutil/gdkarrayimpl.c` is an LGPL-2.1-or-later exception, copyright
2020 Benjamin Otte; `pkg/core/gioutil/listmodel.c` textually includes it.
Its notice is retained in `LICENSES/gotk4-gdkarrayimpl-NOTICE.txt`, with full
terms in `LICENSES/LGPL-2.1-or-later.txt`. Corresponding source:
https://github.com/diamondburned/gotk4/tree/pkg/v0.4.1/pkg/core/gioutil .
This exception must not be relabeled ISC or MPL based on directory metadata.

The reviewed Azerlay source does not import `core/gioutil`, and the inspected
upstream imports did not establish an incoming dependency on it outside its
own example tests. This is source inspection, not a final binary reachability
result. If the actual build incorporates this C code, OS shared-library
replacement alone is insufficient evidence of compliance: provide the LGPL
corresponding source and a compliant way to rebuild/relink the application
with a modified library component, preserving the recipient's modification
rights. Section 6(a) permits adequate rebuildable source as an alternative to
application object files. Merely pointing to an arbitrary public source tree
does not prove those distribution requirements are met. Record the actual
build graph before removing this conservative qualification.

The recursive upstream package tree was checked for LICENSE/COPYING/NOTICE
files, and headers of all 100 Go/C/header/assembly files were inspected. The
three license files are the root MPL, core ISC and swizzle BSD files above.
The additional source-header exceptions are preserved here. These findings
also cover the gotk4 subdirectory licenses reported in the retained historical
Go license CSVs; those CSVs alone do not detect every source-file exception.

### gotk4 cairo binding: upstream notice discrepancy

At `pkg/v0.4.1`, upstream README describes `pkg/cairo` as MIT, but `cairo.go`
contains the ISC-form permission and disclaimer attributed to Conformal Systems
(2013-2014). The actual header is reproduced unaltered as text in
`LICENSES/gotk4-cairo-ISC.txt` and remains available at
https://github.com/diamondburned/gotk4/blob/pkg/v0.4.1/pkg/cairo/cairo.go .
Preserve the actual upstream notices; do not relabel or remove them on the basis
of a scanner's single module label. Other reviewed cairo headers do not resolve the README-versus-header
wording discrepancy; preserve the original notices and do not claim one
uniform file-level expression without resolving it. The native Cairo library is
a different component, discussed below.

## Additional entries present in go.sum

These entries are retained for source/test provenance. Their presence in go.sum
does not establish that they are linked into the distributed executable.

| Component | License text | Source |
| --- | --- | --- |
| github.com/davecgh/go-spew v1.1.1 | `LICENSES/go-spew-ISC.txt` | https://github.com/davecgh/go-spew/tree/v1.1.1 |
| github.com/pmezard/go-difflib v1.0.0 | `LICENSES/go-difflib-BSD-3-Clause.txt` | https://github.com/pmezard/go-difflib/tree/v1.0.0 |
| github.com/stretchr/testify v1.11.1 | `LICENSES/testify-MIT.txt` | https://github.com/stretchr/testify/tree/v1.11.1 |
| gopkg.in/yaml.v3 v3.0.1 | `LICENSES/yaml-v3-LICENSE.txt` (upstream MIT/Apache notices) | https://github.com/go-yaml/yaml/tree/v3.0.1 |

## Native shared libraries

The native archive does not bundle GTK or other native shared libraries. They
are installed separately on the destination. This distinction does not remove
obligations arising from linking or any native code actually included in a binary.

- GTK 4.22.5 is the documented qualification version. Its package declares
  LGPL-2.1-or-later: https://github.com/GNOME/gtk/blob/4.22.5/meson.build .
  Source: https://github.com/GNOME/gtk/tree/4.22.5 . The applicable LGPL 2.1 text
  is included in `LICENSES/LGPL-2.1-or-later.txt`.
- gtk4-layer-shell 1.3.0 is the documented qualification version and is MIT:
  https://github.com/wmww/gtk4-layer-shell/tree/v1.3.0 . Its copyright and full
  license are included in `LICENSES/gtk4-layer-shell-MIT.txt`.
- Pango upstream declares LGPLv2.1+; the exact version supplied with a release
  environment must be recorded. Source: https://gitlab.gnome.org/GNOME/pango .
- Native Cairo offers LGPL 2.1 or MPL 1.1: https://cairographics.org/ . This
  preparation uses the LGPL compliance route; confirm the exact installed
  version and its notices before release. MPL 1.1 is not MPL 2.0.
- The Arch recipe also depends on GLib, HarfBuzz, GdkPixbuf, Graphene, the Vulkan
  loader and glibc. Native transitive dependencies, enabled optional features,
  component versions and additional notices remain to be inventoried against
  the actual final binary and distribution packages.

When distributing an executable linked to LGPL libraries, give notice of their
use and provide the applicable license text. Preserve an appropriate shared
library mechanism that lets recipients use an interface-compatible modified
library. Do not prohibit the modifications or reverse engineering needed to debug
those modifications. If redistributing the libraries themselves, modifying them,
statically linking, or changing to a self-contained bundle, reassess corresponding
source and relinking obligations before distribution. This file does not certify
that a new packaging mode complies merely because it includes these notices.

## Assets, profiles and trademarks

No official Azeron artwork, logos or fonts are intentionally included. The shipped
layout is a JSON schematic; its factual provenance and remaining rights questions
are recorded in `docs/decisions/cyborg-ii-physical-map.md` and
`docs/decisions/public-release-safety.md`. MIT applies only to original expression
that the authors may license, not to third-party rights or trademarks.
User-provided exports, labels, macros and private settings are not relicensed by
this project and must not be added to public fixtures or release artifacts.
