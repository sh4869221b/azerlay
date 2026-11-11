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
    # Query separately: pkgconf 3.0.7's multi-module --path query does not
    # return a record for every requested module. File ownership can also
    # repeat when several development files belong to one package.
    owners = []
    for module in LIBRARY_MODULES:
        paths = subprocess.run(["pkg-config", "--path", module],
                               check=True, capture_output=True, text=True).stdout.splitlines()
        if len(paths) != 1 or not paths[0].startswith("/"):
            raise ValueError("Missing native pkg-config file identity: " + module)
        names = subprocess.run(["pacman", "-Qoq", "--", paths[0]],
                               check=True, capture_output=True, text=True).stdout.splitlines()
        if len(names) != 1 or not names[0]:
            raise ValueError("Missing installed native development-file owner: " + module)
        owners.extend(names)
    return (*ROOTS, *owners)


def dependency(requirement):
    match = re.fullmatch(r"([^<>=\s]+)(?:(>=|<=|=|>|<)([^<>=\s]+))?", requirement)
    if match is None:
        raise ValueError("Invalid installed build dependency: " + requirement)
    return match.groups()


def version_compare(actual, required):
    # Use pacman's own ordering, including epochs, pkgrel and alphanumeric
    # versions. Python/semver/string ordering is not equivalent to libalpm.
    result = subprocess.run(["vercmp", actual, required], check=True,
                            capture_output=True, text=True)
    comparison = int(result.stdout.strip())
    if comparison not in (-1, 0, 1):
        raise ValueError("Invalid vercmp result: " + result.stdout.strip())
    return comparison


def satisfies(actual, operator, required):
    if operator is None:
        return True
    # An unversioned PROVIDES entry cannot satisfy a versioned dependency;
    # its owner's package version is not the version of the virtual library.
    if actual is None:
        return False
    comparison = version_compare(actual, required)
    return {"=": comparison == 0, ">=": comparison >= 0, "<=": comparison <= 0,
            ">": comparison > 0, "<": comparison < 0}[operator]


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
            virtual_name, operator, provided_version = dependency(provided)
            if operator not in (None, "="):
                raise ValueError("Invalid installed package provision: " + provided)
            providers.setdefault(virtual_name, set()).add((name, provided_version))
    selected = set()
    pending = list(roots)
    while pending:
        requirement = pending.pop()
        name, operator, required_version = dependency(requirement)
        # Prefer a satisfying literal package before considering providers,
        # matching pacman. Keep the full constraint even for selected packages:
        # another dependency on the same name may require a different ABI.
        if name not in packages or not satisfies(packages[name][0], operator, required_version):
            choices = {owner for owner, version in providers.get(name, set())
                       if satisfies(version, operator, required_version)}
            if len(choices) != 1:
                raise ValueError("Missing or ambiguous installed build dependency: " + requirement)
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
