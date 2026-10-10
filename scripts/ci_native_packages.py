#!/usr/bin/env python3
"""Identify Arch's installed native build roots and all their dependencies.

The local pacman database records actual installed versions/architectures and
virtual providers. No network, package installation or optional dependencies.
Keep roots aligned with the compiler and native development libraries used by
the Go/CGo build. Compositors, fonts, download and reporting tools are not roots.
Transitive dependencies are deliberately conservative: a native library's
dependency update must still invalidate the external CGo cache boundary.
"""
import argparse
import re
import subprocess
from pathlib import Path

ROOTS = ("gcc", "binutils", "glibc", "linux-api-headers", "pkgconf")
LIBRARY_MODULES = ("gtk4", "gtk4-layer-shell-0", "glib-2.0", "pango",
                   "gobject-introspection-1.0", "cairo")


def native_roots():
    # Select the actual development-file owners, not generator packages with
    # similar names. Arch may split headers/libraries from introspection tools.
    paths = subprocess.run(["pkg-config", "--path", *LIBRARY_MODULES],
                           check=True, capture_output=True, text=True).stdout.splitlines()
    if len(paths) != len(LIBRARY_MODULES) or any(not path.startswith("/") for path in paths):
        raise ValueError("Missing native pkg-config file identity")
    owners = subprocess.run(["pacman", "-Qoq", "--", *paths],
                            check=True, capture_output=True, text=True).stdout.splitlines()
    if len(owners) != len(paths) or any(not name for name in owners):
        raise ValueError("Missing installed native development-file owner")
    return (*ROOTS, *owners)


def package_name(requirement):
    return re.split(r"[<>=]", requirement, maxsplit=1)[0]


def inventory(database, roots=ROOTS):
    packages = {}
    providers = {}
    for desc in sorted(database.glob("*/desc")):
        fields = {}
        field = None
        for line in desc.read_text().splitlines():
            if line.startswith("%") and line.endswith("%"):
                field = line.strip("%")
                fields[field] = []
            elif line and field:
                fields[field].append(line)
        name, version, arch = (fields[key][0] for key in ("NAME", "VERSION", "ARCH"))
        if name in packages:
            raise ValueError("Duplicate installed package: " + name)
        packages[name] = (version, arch, fields.get("DEPENDS", []))
        for provided in fields.get("PROVIDES", []):
            providers.setdefault(package_name(provided), set()).add(name)
    selected = set()
    pending = list(roots)
    while pending:
        name = package_name(pending.pop())
        if name not in packages:
            choices = providers.get(name, set())
            if len(choices) != 1:
                raise ValueError("Missing or ambiguous installed build dependency: " + name)
            name = next(iter(choices))
        if name in selected:
            continue
        selected.add(name)
        pending.extend(packages[name][2])
    return [(name, *packages[name][:2]) for name in sorted(selected)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database", type=Path, default=Path("/var/lib/pacman/local"))
    args = parser.parse_args()
    for record in inventory(args.database, native_roots()):
        print("\t".join(record))


if __name__ == "__main__":
    main()
