#!/usr/bin/env python3
"""Validate the fixed Nix input and fetch only substituted dependency outputs.

Run after installing the pinned Nix version. This deliberately does not install
Nix, change trust/daemon settings, or configure an external cache. A cache miss
fails rather than compiling GTK, Go, or the rest of nixpkgs on the CI runner.
"""
import argparse
import base64
import hashlib
import json
import os
import re
import subprocess
from pathlib import Path

REVISION = "494ce7fd23ff6a5dff39e1fb11e9b6f2ac74bf25"
NAR_HASH = "sha256-Ni0LBydzaCi8oek12r4mVreuFHqYlM1eTD6SfiENKfw="
FLAKES = {role: ".#devShells.x86_64-linux.ci-" + role for role in ("test", "build")}
EVALUATION_FLAGS = ["--no-update-lock-file", "--no-allow-import-from-derivation"]
EXPECTED_VERSIONS = {
    "system": "x86_64-linux", "go": "1.27.1", "gtk4": "4.22.4",
    "glib": "2.88.3", "layerShell": "1.3.0", "sway": "1.12",
    "cairo": "1.18.4", "pango": "1.57.1", "grim": "1.5.0",
    "atspi": "2.60.6", "dbus": "1.16.2",
}
STORE_PATH = re.compile(r"(?:/nix/store/)?[0123456789abcdfghijklmnpqrsvwxyz]{32}-[^/\s^]+\.drv")
OUTPUT = re.compile(r"[A-Za-z_][A-Za-z0-9_-]*")


def validate_lock(lock):
    if not isinstance(lock, dict) or lock.get("version") != 7 or lock.get("root") != "root":
        raise ValueError("Unexpected flake lock format")
    nodes = lock.get("nodes", {})
    if not isinstance(nodes, dict) or set(nodes) != {"root", "nixpkgs"} or nodes["root"] != {"inputs": {"nixpkgs": "nixpkgs"}}:
        raise ValueError("Unexpected extra or redirected flake inputs")
    node = nodes["nixpkgs"]
    if not isinstance(node, dict) or set(node) != {"locked", "original"}:
        raise ValueError("Unexpected nixpkgs input fields")
    expected = {"type": "github", "owner": "NixOS", "repo": "nixpkgs", "rev": REVISION}
    for field in ("locked", "original"):
        source = node.get(field)
        fields = set(expected) | ({"narHash", "lastModified"} if field == "locked" else set())
        if not isinstance(source, dict) or set(source) != fields or any(source.get(k) != v for k, v in expected.items()):
            raise ValueError("Nixpkgs revision or source changed")
    value = node["locked"].get("narHash", "")
    if not isinstance(value, str) or not value.startswith("sha256-"):
        raise ValueError("Missing SHA-256 NAR hash")
    try:
        decoded = base64.b64decode(value[7:], validate=True)
    except ValueError as exc:
        raise ValueError("Invalid NAR hash") from exc
    if len(decoded) != 32:
        raise ValueError("Invalid NAR hash length")
    if value != NAR_HASH:
        raise ValueError("Pinned nixpkgs NAR hash changed")
    # This validates identity/shape, not the hash independently. Nix fetches and
    # verifies the actual source against this hash during evaluation.
    return value


def dependency_installables(graph):
    if not isinstance(graph, dict) or len(graph) != 1:
        raise ValueError("Expected exactly one devShell derivation")
    drv = next(iter(graph.values()))
    if not isinstance(drv, dict) or drv.get("version", 4) != 4:
        raise ValueError("Unsupported devShell derivation format")
    if "inputs" in drv and not isinstance(drv["inputs"], dict):
        raise ValueError("Invalid devShell inputs")
    inputs = drv.get("inputDrvs", drv.get("inputs", {}).get("drvs"))
    if not isinstance(inputs, dict) or not inputs:
        raise ValueError("Missing devShell dependencies")
    installables = []
    for path, outputs in sorted(inputs.items()):
        if not isinstance(path, str) or not STORE_PATH.fullmatch(path):
            raise ValueError("Invalid dependency store path")
        if isinstance(outputs, dict):
            if outputs.get("dynamicOutputs"):
                raise ValueError("Dynamic outputs are unsupported in this probe")
            outputs = outputs.get("outputs")
        if not isinstance(outputs, list) or not outputs or any(not isinstance(o, str) or not OUTPUT.fullmatch(o) for o in outputs):
            raise ValueError("Invalid dependency outputs")
        path = path if path.startswith("/nix/store/") else "/nix/store/" + path
        installables.append(path + "^" + ",".join(sorted(set(outputs))))
    return installables


def capture(args, destination):
    result = subprocess.run(args, check=True, stdout=subprocess.PIPE, text=True)
    destination.write_text(result.stdout)
    return result.stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True)
    parser.add_argument("--validate-lock-only", action="store_true")
    parser.add_argument("--role", choices=("test", "build"), default=os.environ.get("CI_NIX_ROLE"))
    args = parser.parse_args()
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    lock_bytes = Path("flake.lock").read_bytes()
    validate_lock(json.loads(lock_bytes))
    if args.validate_lock_only:
        return
    if args.role not in FLAKES:
        parser.error("--role or CI_NIX_ROLE must select test or build")
    flake = FLAKES[args.role]
    (output / "role.txt").write_text(args.role + "\n")
    version = capture(["nix", "--version"], output / "nix-version.txt").strip()
    if version != "nix (Nix) 2.35.2":
        raise ValueError("Unexpected Nix installer version")
    versions = json.loads(capture(["nix", "eval", *EVALUATION_FLAGS, "--json", ".#ciVersions"], output / "versions.json"))
    if versions != EXPECTED_VERSIONS:
        raise ValueError("Pinned native package versions changed")
    graph = json.loads(capture(["nix", "derivation", "show", *EVALUATION_FLAGS, flake], output / "devshell-derivation.json"))
    dependencies = dependency_installables(graph)
    (output / "dependency-installables.json").write_text(json.dumps(dependencies, indent=2) + "\n")
    # No fallback build and no remote builder. Only pre-existing local paths or
    # default signed substitutes may satisfy dependency derivations.
    subprocess.run(["nix", "build", "--no-link", "--max-jobs", "0", "--builders", "", *dependencies], check=True)
    # Dependencies are now available; only the small project shell environment
    # itself is built locally, not an application or a native library.
    shell_paths = json.loads(capture(["nix", "build", *EVALUATION_FLAGS, "--builders", "", "--no-link", "--json", flake], output / "devshell-result.json"))
    paths = [path for item in shell_paths for path in item["outputs"].values()]
    if not paths:
        raise ValueError("No devShell output")
    capture(["nix", "path-info", "--recursive", "--json", *paths], output / "closure.json")
    capture(["nix", "path-info", "--recursive", *paths], output / "closure-paths.txt")
    if Path("flake.lock").read_bytes() != lock_bytes:
        raise ValueError("flake.lock changed during preparation")
    (output / "lock-sha256.txt").write_text(hashlib.sha256(lock_bytes).hexdigest() + "\n")


if __name__ == "__main__":
    main()
