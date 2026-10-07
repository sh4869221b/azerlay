#!/usr/bin/env python3
"""Check the explicit CI partition and GUI-free core dependencies, including tests.

Run the default check on the native runner with CGO_ENABLED=1. On the core
runner, use CGO_ENABLED=0 python3 scripts/ci_check_packages.py --core-only.
The latter deliberately does not enumerate native packages. Neither command
runs tests: the CGO=0 test and CGO=1 race test are separate verification steps.
"""
import argparse
from pathlib import Path
import re
import subprocess
import sys


def read_manifest(path):
    packages = path.read_text().splitlines()
    if not packages:
        raise ValueError(f"{path.name}: empty manifest")
    for package in packages:
        if (not re.fullmatch(r"\./[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*", package)
                or "..." in package
                or any(part in (".", "..") for part in package[2:].split("/"))):
            raise ValueError(f"{path.name}: expected one explicit ./package per line: {package!r}")
    if len(packages) != len(set(packages)):
        raise ValueError(f"{path.name}: duplicate package")
    if packages != sorted(packages):
        raise ValueError(f"{path.name}: packages must be sorted")
    return packages


def run_go(root, *args):
    try:
        result = subprocess.run(["go", *args], cwd=root, check=True,
                                capture_output=True, text=True)
    except subprocess.CalledProcessError as error:
        raise ValueError(f"go {' '.join(args)} failed: {error.stderr.strip()}") from error
    return result.stdout.splitlines()


def check(root, core_only=False):
    core = read_manifest(root / "scripts/ci-core-packages.txt")
    native = read_manifest(root / "scripts/ci-native-packages.txt")
    overlap = sorted(set(core) & set(native))
    if overlap:
        raise ValueError("packages appear in both manifests: " + ", ".join(overlap))

    modules = run_go(root, "list", "-m", "-f", "{{.Path}}")
    if len(modules) != 1 or not modules[0]:
        raise ValueError("expected exactly one module path")
    module = modules[0]
    core_imports = {module + package[1:] for package in core}
    native_imports = {module + package[1:] for package in native}
    if not core_only:
        listed = run_go(root, "list", "./...")
        expected = sorted(core_imports | native_imports)
        if sorted(listed) != expected:
            missing = sorted(set(listed) - set(expected))
            stale = sorted(set(expected) - set(listed))
            raise ValueError(f"package manifest mismatch: unassigned={missing}, stale={stale}; "
                             "go list output must match the exact manifest union")

    # -test includes external tests, test variants and their transitive imports.
    # Go decorates test variants as "import/path [tested/import/path.test]".
    dependencies = {line.split(" [", 1)[0] for line in
                    run_go(root, "list", "-deps", "-test", "-f", "{{.ImportPath}}", *core)}
    missing_core = sorted(core_imports - dependencies)
    if missing_core:
        raise ValueError("core dependency listing omitted packages: " + ", ".join(missing_core))
    forbidden = native_imports | {"github.com/diamondburned/gotk4"}
    leaks = sorted(dependency for dependency in dependencies
                   if any(dependency in (prefix, prefix + ".test", prefix + "_test")
                          or dependency.startswith(prefix + "/") for prefix in forbidden))
    if leaks:
        raise ValueError("core imports native/GUI packages (including test dependencies): "
                         + ", ".join(leaks))
    return len(core), len(native)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--core-only", action="store_true",
                        help="check core dependencies without listing native packages")
    args = parser.parse_args()
    try:
        core_count, native_count = check(Path(__file__).resolve().parents[1], args.core_only)
    except (OSError, ValueError) as error:
        print(f"package boundary check failed: {error}", file=sys.stderr)
        return 1
    if args.core_only:
        print(f"core boundary OK: {core_count} packages; native partition not enumerated")
    else:
        print(f"package partition and core boundary OK: {core_count} core, {native_count} native")
    return 0


if __name__ == "__main__":
    sys.exit(main())
