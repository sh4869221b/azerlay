import argparse
import copy
import json
import re
import sys
import uuid
from pathlib import Path


class ReportError(ValueError):
    pass


def check_components(sbom, report):
    packages = [package["SPDXID"] for package in sbom["packages"]]
    rows = [component["SPDXID"] for component in report["components"]]
    if len(set(packages)) != len(packages) or len(set(rows)) != len(rows) or set(packages) != set(rows):
        raise ReportError("SBOM and license report component IDs must match exactly")
    known = set(packages) | {sbom["SPDXID"]}
    for edge in sbom["relationships"]:
        if edge["spdxElementId"] not in known or edge["relatedSpdxElement"] not in known:
            raise ReportError("SBOM relationship references an absent component")


def render(inventory, template, artifact):
    if template.get("spdxVersion") != "SPDX-2.3" or template.get("SPDXID") != "SPDXRef-DOCUMENT":
        raise ReportError("Syft template must be an SPDX 2.3 document")
    scopes = {"shipped-source", "source-dependency"} if artifact == "source" else {"shipped-binary", "host-provided"}
    components = copy.deepcopy([item for item in inventory["components"] if scopes.intersection(item["scopes"])])
    scanned = {(item["name"], item.get("versionInfo", "")): item for item in template.get("packages", [])}
    observed = {(item["name"], item["version"]) for item in components}
    for index, ((name, version), package) in enumerate(scanned.items()):
        if (name, version) in observed or name == "azerlay" or name.endswith("/azerlay"):
            continue
        components.append({"id": f"observed-{index}", "name": name, "version": version,
            "kind": "scanner-observed", "scopes": ["observed-source-dependency", "build-tool"] if artifact == "source" else ["observed-binary-component"],
            "license": "NOASSERTION", "source": package.get("downloadLocation", "NOASSERTION"),
            "notices": [], "exceptions": [], "review_status": "pending",
            "review_reasons": ["scanner observed component; exact role and corresponding source require review"], "files": []})
    ids = {item["id"]: "SPDXRef-" + re.sub(r"[^A-Za-z0-9.-]", "-", item["id"]) for item in components}
    packages = []
    rows = []
    safe_licenses = {"MIT", "ISC", "BSD-3-Clause", "MPL-2.0", "Apache-2.0", "NOASSERTION"}
    for component in components:
        declared = component["license"]
        license_id = declared if declared in safe_licenses else "NOASSERTION"
        if license_id != declared:
            component["review_status"] = "pending"
            component["review_reasons"].append("declared terms require SPDX expression and exception review")
        row = {key: value for key, value in component.items() if key not in ("id", "scanner_matches")}
        row["SPDXID"] = ids[component["id"]]
        row["spdx_license_declared"] = license_id
        row["scanner_observations"] = [{"origin": match["origin"], "type": match["type"],
            "licenses": [license.get("value", "NOASSERTION") for license in match["licenses"]]}
            for match in component.get("scanner_matches", [])]
        rows.append(row)
        package = {"SPDXID": row["SPDXID"], "name": row["name"], "versionInfo": row["version"],
            "downloadLocation": row["source"] or "NOASSERTION", "filesAnalyzed": False,
            "licenseDeclared": license_id, "licenseConcluded": "NOASSERTION", "copyrightText": "NOASSERTION",
            "annotations": [{"annotationType": "OTHER", "annotator": "Tool: azerlay-release-report",
                "annotationDate": template["creationInfo"]["created"],
                "comment": json.dumps({key: row[key] for key in ("scopes", "notices", "exceptions", "review_status", "review_reasons")}, sort_keys=True)}]}
        scanner_package = scanned.get((row["name"], row["version"]), {})
        if scanner_package.get("externalRefs"):
            package["externalRefs"] = scanner_package["externalRefs"]
        packages.append(package)
    edges = [edge for edge in inventory["relationships"] if edge["scope"] == artifact and edge["from"] in ids and edge["to"] in ids]
    relationships = [{"spdxElementId": ids[edge["from"]], "relationshipType": edge["type"],
                      "relatedSpdxElement": ids[edge["to"]]} for edge in edges]
    relationships.insert(0, {"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": ids["application"]})
    archive = inventory["artifacts"][artifact].get("archive_name", inventory["artifacts"][artifact]["name"])
    sbom = {key: copy.deepcopy(template[key]) for key in ("spdxVersion", "dataLicense", "SPDXID", "creationInfo")}
    sbom.update({"name": archive, "documentNamespace": f"https://spdx.org/spdxdocs/{archive}-{uuid.uuid4()}",
                 "packages": packages, "relationships": relationships})
    sbom["creationInfo"]["creators"].append("Tool: azerlay-release-report")
    go = copy.deepcopy(inventory["go"])
    report = {"artifact": archive, "version": inventory["version"], "target": inventory["target"],
        "toolchain": inventory["toolchain"], "build_options": inventory["build_options"],
        "components": rows, "relationships": [{**edge, "from": ids[edge["from"]], "to": ids[edge["to"]]} for edge in edges],
        "go": go, "publication_ready": False}
    if artifact == "binary":
        native = copy.deepcopy(inventory["native"])
        for item in native["objects"]:
            if item["component"] == "application":
                item["path"] = "bin/azerlay"
            item["component"] = ids[item["component"]]
            for dependency in item["dependencies"]:
                dependency["component"] = ids[dependency["component"]]
        report["native"] = native
    report["review_pending"] = [{"SPDXID": row["SPDXID"], "name": row["name"], "reasons": row["review_reasons"]}
                                for row in rows if row["review_status"] == "pending"]
    check_components(sbom, report)
    return sbom, report


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--inventory", type=Path)
    parser.add_argument("--syft-spdx", type=Path)
    parser.add_argument("--artifact", choices=("source", "binary"))
    parser.add_argument("--sbom", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    args = parser.parse_args()
    try:
        if args.check:
            check_components(json.loads(args.sbom.read_text()), json.loads(args.report.read_text()))
            return 0
        if not (args.inventory and args.syft_spdx and args.artifact):
            parser.error("generation requires --inventory, --syft-spdx and --artifact")
        sbom, report = render(json.loads(args.inventory.read_text()), json.loads(args.syft_spdx.read_text()), args.artifact)
        args.sbom.write_text(json.dumps(sbom, indent=2) + "\n")
        args.report.write_text(json.dumps(report, indent=2) + "\n")
        print(f"{report['artifact']}: {len(report['review_pending'])} components require review; publication readiness is not established.")
        for item in report["review_pending"]:
            print(f"- {item['name']}: {'; '.join(item['reasons'])}")
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"release report failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
