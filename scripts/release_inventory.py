#!/usr/bin/env python3
"""Collect release inventory in the native build environment; no license clearance."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile


def run(root, *command):
    result = subprocess.run(command, cwd=root, env={**os.environ, "LC_ALL": "C"},
                            capture_output=True, text=True)
    if result.returncode:
        raise ValueError(f"{' '.join(map(str, command))}: {result.stderr.strip() or result.stdout.strip()}")
    return result.stdout


def json_stream(text):
    decoder = json.JSONDecoder()
    values = []
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        values.append(value)
        text = text.lstrip()[end:]
    return values


def build_info(text):
    result = {"modules": [], "settings": {}}
    for line in text.splitlines():
        fields = line.strip().split("\t")
        if len(fields) == 1 and ": go" in fields[0]:
            result["toolchain"] = fields[0].rsplit(": ", 1)[1]
        elif fields[0] in ("mod", "dep") and len(fields) >= 3:
            result["modules"].append({"Path": fields[1], "Version": fields[2],
                                      "Main": fields[0] == "mod"})
        elif fields[0] == "=>" and result["modules"]:
            result["modules"][-1]["Replace"] = {"Path": fields[1], "Version": fields[2]}
        elif fields[0] == "build" and len(fields) == 2:
            key, value = fields[1].split("=", 1)
            result["settings"][key] = value
        elif fields[0] == "path" and len(fields) == 2:
            result["path"] = fields[1]
    if "toolchain" not in result or not result["modules"]:
        raise ValueError("binary has no usable Go build information")
    return result


def ldd_paths(text):
    paths = {}
    for line in text.splitlines():
        if "=> not found" in line:
            raise ValueError(f"unresolved native dependency: {line.strip()}")
        match = re.match(r"\s*(\S+) => (/.*?) \(0x[0-9a-fA-F]+\)", line)
        if match:
            paths[Path(match[1]).name] = match[2]
        else:
            match = re.match(r"\s*(/\S+) \(0x[0-9a-fA-F]+\)", line)
            if match:
                paths[Path(match[1]).name] = match[1]
    return paths


def package_fields(text):
    fields = {}
    key = None
    for line in text.splitlines():
        match = re.match(r"^([^ :][^:]*?)\s+:\s*(.*)$", line)
        if match:
            key = match[1].strip()
            fields[key] = match[2].strip()
        elif line.startswith(" ") and key:
            fields[key] += " " + line.strip()
    return fields


def notice_modules(root):
    result = {}
    text = (root / "THIRD_PARTY_NOTICES.md").read_text()
    for line in text.splitlines():
        match = re.match(r"\| (\S+) (v\S+) \| (.*?) \| (https?://\S+) \|", line)
        if match:
            path, version, terms, source = match.groups()
            license_match = re.match(r"([A-Za-z0-9.-]+)", terms)
            notices = re.findall(r"`(LICENSES/[^`]+)`", terms)
            result[(path, version)] = {
                "license": license_match[1] if license_match else "NOASSERTION",
                "source": source, "notices": notices,
            }
    return result


def component(identifier, name, version, kind, scopes, metadata=None):
    metadata = metadata or {}
    reasons = list(metadata.get("review_reasons", []))
    if not version or version == "NOASSERTION":
        reasons.append("unknown version")
    for field in ("license", "source"):
        if metadata.get(field, "NOASSERTION") == "NOASSERTION":
            reasons.append(f"unknown {field}")
    return {"id": identifier, "name": name, "version": version or "NOASSERTION",
            "kind": kind, "scopes": scopes, "license": metadata.get("license", "NOASSERTION"),
            "source": metadata.get("source", "NOASSERTION"),
            "notices": metadata.get("notices", []),
            "exceptions": metadata.get("exceptions", []),
            "review_status": "pending" if reasons else "recorded",
            "review_reasons": reasons, "files": []}


def native_inventory(binary, root):
    objects = []
    components = {}
    relationships = []
    queue = [(str(binary.resolve()), "application")]
    visited = set()
    while queue:
        path, owner_id = queue.pop(0)
        if path in visited:
            continue
        visited.add(path)
        dynamic = run(root, "readelf", "-d", path)
        needed = re.findall(r"\(NEEDED\).*?\[(.*?)\]", dynamic)
        program = run(root, "readelf", "-l", path)
        interpreters = re.findall(r"Requesting program interpreter: (.*?)\]", program)
        resolved = ldd_paths(run(root, "ldd", path)) if needed or interpreters else {}
        dependencies = []
        for name in needed + interpreters:
            actual = resolved.get(name) or resolved.get(Path(name).name)
            if not actual or not Path(actual).is_file():
                raise ValueError(f"unresolved native dependency: {path}: {name}")
            actual = str(Path(actual).resolve())
            try:
                package = run(root, "pacman", "-Qqo", actual).strip()
            except ValueError as error:
                package = "unowned-" + Path(actual).name
                components[package] = component(
                    "native-" + package, package, "NOASSERTION", "native", ["host-provided"],
                    {"review_reasons": [f"Arch package ownership unresolved: {error}"]})
            if package not in components:
                fields = package_fields(run(root, "pacman", "-Qi", package))
                license_value = fields.get("Licenses", "None")
                license_value = "NOASSERTION" if license_value == "None" else license_value
                components[package] = component(
                    "native-" + package, package, fields.get("Version"), "native",
                    ["host-provided"], {"license": license_value,
                        "source": fields.get("URL", "NOASSERTION") if fields.get("URL") != "None" else "NOASSERTION",
                        "notices": (["LICENSES/gtk4-layer-shell-MIT.txt"] if package == "gtk4-layer-shell" else
                                    ["LICENSES/LGPL-2.1-or-later.txt"] if package in ("gtk4", "glib2", "pango", "cairo") else []),
                        "review_reasons": ["package metadata does not establish exact corresponding source or file-level exceptions"]})
            target_id = components[package]["id"]
            if actual not in components[package]["files"]:
                components[package]["files"].append(actual)
            dependencies.append({"needed": name, "path": actual, "component": target_id})
            edge = {"from": owner_id, "to": target_id, "type": "DEPENDS_ON", "scope": "binary",
                    "direct": owner_id == "application"}
            if edge not in relationships and owner_id != target_id:
                relationships.append(edge)
            queue.append((actual, target_id))
        objects.append({"path": path, "component": owner_id, "dependencies": dependencies})
    return list(components.values()), relationships, objects


def collect(args):
    root = args.source_root.resolve()
    with tarfile.open(args.source_archive) as archive:
        members = [member for member in archive.getmembers() if member.isfile()]
        files = [member.name for member in members]
        for member in members:
            relative = member.name.split("/", 1)[-1]
            with archive.extractfile(member) as archived:
                if archived.read() != (root / relative).read_bytes():
                    raise ValueError(f"source archive and extracted source root differ: {relative}")
    if not files or len({name.split("/")[0] for name in files}) != 1:
        raise ValueError("source archive must contain one nonempty source root")
    prefix = files[0].split("/")[0] + "/"
    files = sorted(name.removeprefix(prefix) for name in files)
    if not all((root / name).is_file() for name in files):
        raise ValueError("source archive and extracted source root differ")
    syft = {}
    for name in ("source", "binary", "host"):
        data = json.loads(getattr(args, "syft_" + name).read_text())
        if not isinstance(data, dict) or not isinstance(data.get("artifacts"), list):
            raise ValueError(f"{name} scanner input must be Syft JSON with artifacts")
        syft[name] = data
    modules = json_stream(run(root, "go", "list", "-m", "-json", "all"))
    packages = json_stream(run(root, "go", "list", "-deps", "-json", "./cmd/azerlay"))
    info = build_info(run(root, "go", "version", "-m", str(args.binary.resolve())))
    toolchain = run(root, "go", "env", "GOVERSION").strip()
    if info["toolchain"] != toolchain:
        raise ValueError("binary and source inventory Go toolchains differ")
    settings = info["settings"]
    if any(settings.get(key) != value for key, value in
           (("GOOS", "linux"), ("GOARCH", "amd64"), ("CGO_ENABLED", "1"))):
        raise ValueError("binary must be the Linux amd64 CGO-enabled native build")
    main = next(module for module in modules if module.get("Main"))
    if info.get("path") != main["Path"] + "/cmd/azerlay":
        raise ValueError("binary target does not match source module")
    actual_version = run(root, str(args.binary.resolve()), "version").strip()
    if actual_version != "azerlay " + args.version:
        raise ValueError("binary version does not match candidate version")
    linked = {module["Path"]: module for module in info["modules"] if not module["Main"]}
    target = {package["Module"]["Path"] for package in packages if "Module" in package}
    resolved = {module["Path"]: module for module in modules}
    for path, module in linked.items():
        expected = resolved.get(path, {})
        actual_replacement = {key: module.get("Replace", {}).get(key) for key in ("Path", "Version")}
        expected_replacement = {key: expected.get("Replace", {}).get(key) for key in ("Path", "Version")}
        if (path not in target or module["Version"] != expected.get("Version")
                or actual_replacement != expected_replacement):
            raise ValueError(f"binary/source module mismatch: {path}")
    notices = notice_modules(root)
    components = [component("application", main["Path"], args.version, "application",
                            ["shipped-source", "shipped-binary"],
                            {"license": "MIT", "source": main["Path"],
                             "notices": ["LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md"],
                             "review_reasons": ["publication conditions remain unresolved"]})]
    components[0]["artifact_files"] = {"source": files, "binary": ["bin/azerlay"]}
    relationships = []
    for index, module in enumerate(modules):
        if module.get("Main"):
            continue
        path, version = module["Path"], module.get("Version", "NOASSERTION")
        metadata = dict(notices.get((path, version), {}))
        if module.get("Replace"):
            metadata = {"review_reasons": ["replacement source and license require review"]}
        scopes = ["source-dependency"]
        if path in linked:
            scopes.append("shipped-binary")
        elif path in target:
            scopes.append("build-only")
        else:
            scopes.append("build-or-test-only")
            metadata.setdefault("review_reasons", []).append("module is outside target graph; build/test role unresolved")
        item = component(f"go-{index}", path, version, "go-module", scopes, metadata)
        if path.startswith("golang.org/x/"):
            item["notices"].append("LICENSES/golang-PATENTS.txt")
        if "gotk4/pkg" in path:
            item["exceptions"] = ["core ISC", "cairo ISC header/README discrepancy", "cairo/swizzle BSD-3-Clause", "gioutil C LGPL-2.1-or-later"]
            item["notices"] += sorted(name for name in files if name.startswith("LICENSES/gotk4-") or name == "LICENSES/LGPL-2.1-or-later.txt")
            item["review_status"] = "pending"
            item["review_reasons"].append("gioutil C incorporation and file-level reachability require review")
        components.append(item)
        relationships.append({"from": "application", "to": item["id"],
                              "type": "DEPENDS_ON", "scope": "source",
                              "direct": not module.get("Indirect", False)})
        if path in linked:
            relationships.append({"from": "application", "to": item["id"],
                                  "type": "DEPENDS_ON", "scope": "binary",
                                  "direct": not module.get("Indirect", False)})
    package_modules = {package["ImportPath"]: package.get("Module", {}).get("Path") for package in packages}
    module_ids = {item["name"]: item["id"] for item in components}
    for package in packages:
        origin = module_ids.get(package_modules[package["ImportPath"]])
        for imported in package.get("Imports", []):
            destination = module_ids.get(package_modules.get(imported))
            if origin and destination and origin != destination:
                edge = {"from": origin, "to": destination, "type": "DEPENDS_ON", "scope": "binary"}
                if edge not in relationships:
                    relationships.append(edge)
    components.append(component("go-runtime", "Go runtime", toolchain, "runtime",
                                ["shipped-binary"], {"license": "BSD-3-Clause",
                                    "source": "https://github.com/golang/go/tree/" + toolchain,
                                    "notices": ["LICENSES/go-runtime-BSD-3-Clause.txt", "LICENSES/go-runtime-PATENTS.txt"]}))
    relationships.append({"from": "application", "to": "go-runtime", "type": "DEPENDS_ON", "scope": "binary"})
    native, edges, objects = native_inventory(args.binary, root)
    components.extend(native)
    relationships.extend(edges)
    # Scanner labels enrich matching components; unrelated host packages are excluded.
    for item in components:
        item["scanner_matches"] = []
        for origin, data in syft.items():
            for scanned in data["artifacts"]:
                if scanned.get("name") == item["name"] and scanned.get("version") == item["version"]:
                    item["scanner_matches"].append({"origin": origin, "id": scanned.get("id"),
                        "licenses": scanned.get("licenses", []), "type": scanned.get("type")})
    return {"version": args.version, "target": "linux/amd64", "toolchain": toolchain,
            "build_options": settings, "artifacts": {"source": {"name": args.source_archive.name, "files": files},
                "binary": {"name": args.binary.name, "archive_name": f"azerlay-{args.version}-linux-x86_64.tar.gz"}},
            "components": components, "relationships": relationships,
            "go": {"modules": [{**{key: module[key] for key in ("Path", "Version", "Main", "Indirect", "GoVersion") if key in module},
                                **({"Replace": {key: module["Replace"][key] for key in ("Path", "Version") if key in module["Replace"]}} if "Replace" in module else {})} for module in modules],
                   "target_packages": [{key: package[key] for key in ("ImportPath", "Imports", "CgoFiles", "CFiles", "HFiles", "Standard") if key in package} for package in packages], "buildinfo": info},
            "native": {"objects": objects, "dlopen_coverage": "not observed; GTK runtime plugins are outside the ELF closure"}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("source-root", "source-archive", "binary", "syft-source", "syft-binary", "syft-host", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    try:
        inventory = collect(args)
        args.output.write_text(json.dumps(inventory, indent=2) + "\n")
    except (OSError, ValueError, tarfile.TarError) as error:
        print(f"release inventory failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
