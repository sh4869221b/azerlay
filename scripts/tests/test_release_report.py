import copy
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from scripts.release_report import ReportError, check_components, render

ROOT = Path(__file__).resolve().parents[2]


def fixture():
    components = [{"id": "application", "name": "azerlay", "version": "1.2.3", "kind": "application",
        "scopes": ["shipped-source", "shipped-binary"], "license": "MIT", "source": "NOASSERTION",
        "notices": ["LICENSE"], "exceptions": [], "review_status": "recorded", "review_reasons": [], "files": []},
        {"id": "native-example", "name": "example", "version": "2", "kind": "native",
        "scopes": ["host-provided"], "license": "custom:vendor terms", "source": "NOASSERTION",
        "notices": [], "exceptions": [], "review_status": "pending", "review_reasons": ["unknown license"], "files": ["/usr/lib/libexample.so"]},
        {"id": "go-test", "name": "test-module", "version": "v1", "kind": "go-module",
        "scopes": ["source-dependency", "build-or-test-only"], "license": "NOASSERTION", "source": "NOASSERTION",
        "notices": [], "exceptions": [], "review_status": "pending", "review_reasons": ["unknown license"], "files": []}]
    inventory = {"components": components, "version": "1.2.3", "target": "linux/amd64", "toolchain": "go1.27.2",
        "build_options": {"CGO_ENABLED": "1"}, "artifacts": {"source": {"name": "azerlay-1.2.3-source.tar.gz"},
        "binary": {"name": "azerlay", "archive_name": "azerlay-1.2.3-linux-x86_64.tar.gz"}},
        "go": {"modules": [], "target_packages": [], "buildinfo": {"path": "example/azerlay", "toolchain": "go1.27.2"}},
        "native": {"objects": [{"component": "application", "path": "/home/private/azerlay", "dependencies": []}], "dlopen_coverage": "not observed"},
        "relationships": [{"from": "application", "to": "native-example", "type": "DEPENDS_ON", "scope": "binary", "direct": True},
                          {"from": "application", "to": "go-test", "type": "DEPENDS_ON", "scope": "source", "direct": False},
                          {"from": "go-test", "to": "native-example", "type": "DEPENDS_ON", "scope": "binary"}]}
    template = {"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT",
        "creationInfo": {"created": "2026-10-09T00:00:00Z", "creators": ["Tool: syft-1.54.1"]}, "packages": []}
    return inventory, template


class ReleaseReportTests(unittest.TestCase):
    def test_unknown_native_license_remains_pending_and_components_match(self):
        inventory, template = fixture()
        sbom, report = render(inventory, template, "binary")
        check_components(sbom, report)
        self.assertEqual({row["SPDXID"] for row in report["components"]}, {"SPDXRef-application", "SPDXRef-native-example"})
        native = next(package for package in sbom["packages"] if package["name"] == "example")
        self.assertEqual(native["licenseDeclared"], "NOASSERTION")
        row = next(row for row in report["components"] if row["name"] == "example")
        self.assertEqual(row["license"], "custom:vendor terms")
        self.assertEqual(row["review_status"], "pending")
        self.assertFalse(report["publication_ready"])
        self.assertNotIn("/home/private", json.dumps(report))
        self.assertEqual(len(sbom["relationships"]), 2)

    def test_source_retains_observed_scanner_dependency_without_claiming_binary_inclusion(self):
        inventory, template = fixture()
        template["packages"] = [{"SPDXID": "SPDXRef-scanner", "name": "scanner-tool", "versionInfo": "v3"}]
        sbom, report = render(inventory, template, "source")
        row = next(row for row in report["components"] if row["name"] == "scanner-tool")
        self.assertEqual(row["scopes"], ["observed-source-dependency", "build-tool"])
        self.assertEqual(row["review_status"], "pending")
        self.assertNotIn("example", {package["name"] for package in sbom["packages"]})

    def test_component_omission_is_rejected_by_check_cli(self):
        inventory, template = fixture()
        sbom, report = render(inventory, template, "binary")
        report["components"].pop()
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            (base / "sbom.json").write_text(json.dumps(sbom))
            (base / "report.json").write_text(json.dumps(report))
            result = subprocess.run([sys.executable, str(ROOT / "scripts/release_report.py"), "--check",
                "--sbom", str(base / "sbom.json"), "--report", str(base / "report.json")], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("component IDs", result.stderr)

    def test_non_spdx_template_is_rejected(self):
        inventory, template = fixture()
        template["spdxVersion"] = "invalid"
        with self.assertRaises(ReportError):
            render(inventory, template, "source")

    def test_dangling_relationship_is_rejected(self):
        inventory, template = fixture()
        sbom, report = render(inventory, template, "binary")
        corrupted = copy.deepcopy(sbom)
        corrupted["relationships"][0]["relatedSpdxElement"] = "SPDXRef-absent"
        with self.assertRaises(ReportError):
            check_components(corrupted, report)


if __name__ == "__main__":
    unittest.main()
