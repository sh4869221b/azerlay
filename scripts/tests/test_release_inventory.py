import argparse
import importlib.util
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / "release_inventory.py"
spec = importlib.util.spec_from_file_location("release_inventory", SCRIPT)
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)


class InventoryParsingTests(unittest.TestCase):
    def test_go_buildinfo_records_target_options_and_replacement(self):
        result = inventory.build_info("""/tmp/azerlay: go1.27.2
\tpath\texample.org/azerlay/cmd/azerlay
\tmod\texample.org/azerlay\t(devel)\t
\tdep\texample.org/dependency\tv1.0.0\th1:ignored
\t=>\texample.org/replacement\tv1.0.1\th1:ignored
\tbuild\tGOOS=linux
\tbuild\tCGO_ENABLED=1
""")
        self.assertEqual(result["toolchain"], "go1.27.2")
        self.assertEqual(result["settings"], {"GOOS": "linux", "CGO_ENABLED": "1"})
        self.assertEqual(result["modules"][1]["Replace"],
                         {"Path": "example.org/replacement", "Version": "v1.0.1"})
        self.assertNotIn("ignored", json.dumps(result))

    def test_missing_required_native_library_fails(self):
        with self.assertRaisesRegex(ValueError, "unresolved native dependency.*libmissing"):
            inventory.ldd_paths("\tlibmissing.so.1 => not found\n")

    def test_ldd_preserves_loader_and_resolved_paths(self):
        result = inventory.ldd_paths(""" linux-vdso.so.1 (0x00007fffd000)
 libgtk-4.so.1 => /usr/lib/libgtk-4.so.1 (0x00007fe00000)
 /lib64/ld-linux-x86-64.so.2 (0x00007fa00000)
""")
        self.assertEqual(result, {"libgtk-4.so.1": "/usr/lib/libgtk-4.so.1",
                                 "ld-linux-x86-64.so.2": "/lib64/ld-linux-x86-64.so.2"})

    def test_ldd_absolute_loader_alias_matches_needed_soname(self):
        result = inventory.ldd_paths("/usr/lib/ld-linux-x86-64.so.2 => /usr/lib64/ld-linux-x86-64.so.2 (0x00001234)")
        self.assertEqual(result["ld-linux-x86-64.so.2"], "/usr/lib64/ld-linux-x86-64.so.2")

    def test_unknown_license_and_source_remain_pending(self):
        result = inventory.component("native-unknown", "unknown", "1.0", "native", ["host-provided"])
        self.assertEqual(result["license"], "NOASSERTION")
        self.assertEqual(result["source"], "NOASSERTION")
        self.assertEqual(result["review_status"], "pending")
        self.assertEqual(result["review_reasons"], ["unknown license", "unknown source"])


class NativeClosureTests(unittest.TestCase):
    def test_resolved_library_without_arch_owner_remains_pending(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary, library = root / "azerlay", root / "libcustom.so"
            binary.touch()
            library.touch()

            def command(cwd, *args):
                if args[:2] == ("readelf", "-d"):
                    return "(NEEDED) Shared library: [libcustom.so]" if args[2] == str(binary) else ""
                if args[:2] == ("readelf", "-l"):
                    return ""
                if args[0] == "ldd":
                    return f"libcustom.so => {library} (0x00001234)"
                if args[:2] == ("pacman", "-Qqo"):
                    raise ValueError("No package owns libcustom.so")
                self.fail(f"unexpected command {args}")

            with patch.object(inventory, "run", command):
                components, _, _ = inventory.native_inventory(binary, root)
            self.assertEqual(components[0]["version"], "NOASSERTION")
            self.assertEqual(components[0]["review_status"], "pending")
            self.assertEqual(components[0]["files"], [str(library)])

    def test_recursive_library_edges_and_unknown_package_license(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary, library, transitive = [root / name for name in ("azerlay", "libone.so", "libtwo.so")]
            for path in (binary, library, transitive):
                path.touch()

            def command(cwd, *args):
                if args[:2] == ("readelf", "-d"):
                    name = {str(binary): "libone.so", str(library): "libtwo.so"}.get(args[2])
                    return f" (NEEDED) Shared library: [{name}]" if name else ""
                if args[:2] == ("readelf", "-l"):
                    return ""
                if args[0] == "ldd":
                    path = library if args[1] == str(binary) else transitive
                    return f" {path.name} => {path} (0x00001234)"
                if args[:2] == ("pacman", "-Qqo"):
                    return "one" if args[2] == str(library) else "two"
                if args[:2] == ("pacman", "-Qi"):
                    return f"Name : {args[2]}\nVersion : 1.0\nLicenses : None\nURL : https://example.org\n"
                self.fail(f"unexpected command {args}")

            with patch.object(inventory, "run", command):
                components, edges, objects = inventory.native_inventory(binary, root)
            self.assertEqual(len(objects), 3)
            self.assertEqual({(edge["from"], edge["to"]) for edge in edges},
                             {("application", "native-one"), ("native-one", "native-two")})
            self.assertTrue(all(item["review_status"] == "pending" for item in components))
            self.assertTrue(all(item["license"] == "NOASSERTION" for item in components))


class InventoryCollectionTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / "go.mod").write_text("module example.org/azerlay\n")
        (self.root / "THIRD_PARTY_NOTICES.md").write_text(
            "| example.org/linked v1.0.0 | MIT; `LICENSES/linked.txt` | https://example.org/linked/v1.0.0 |\n")
        self.archive = self.root / "azerlay-0.0.0-source.tar.gz"
        with tarfile.open(self.archive, "w:gz") as archive:
            archive.add(self.root / "go.mod", arcname="azerlay-0.0.0/go.mod")
            archive.add(self.root / "THIRD_PARTY_NOTICES.md", arcname="azerlay-0.0.0/THIRD_PARTY_NOTICES.md")
        self.binary = self.root / "azerlay"
        self.binary.touch()
        self.raw = self.root / "syft.json"
        self.raw.write_text(json.dumps({"artifacts": [{"name": "unrelated-host", "version": "9", "id": "scanner-host"}]}))
        self.args = argparse.Namespace(source_root=self.root, source_archive=self.archive,
                                       binary=self.binary, version="0.0.0", syft_source=self.raw,
                                       syft_binary=self.raw, syft_host=self.raw)
        self.modules = [{"Path": "example.org/azerlay", "Main": True},
                        {"Path": "example.org/linked", "Version": "v1.0.0"},
                        {"Path": "example.org/test", "Version": "v2.0.0", "Indirect": True}]
        self.packages = [{"ImportPath": "example.org/azerlay/cmd/azerlay", "Module": self.modules[0], "Imports": ["example.org/linked"]},
                         {"ImportPath": "example.org/linked", "Module": self.modules[1]}]
        self.build_version = "v1.0.0"

    def command(self, root, *command):
        if command == ("go", "list", "-m", "-json", "all"):
            return "\n".join(json.dumps(module) for module in self.modules)
        if command == ("go", "list", "-deps", "-json", "./cmd/azerlay"):
            return "\n".join(json.dumps(package) for package in self.packages)
        if command == ("go", "version", "-m", str(self.binary)):
            return ("azerlay: go1.27.2\n\tpath\texample.org/azerlay/cmd/azerlay\n"
                    "\tmod\texample.org/azerlay\t(devel)\n"
                    f"\tdep\texample.org/linked\t{self.build_version}\n"
                    "\tbuild\tGOOS=linux\n\tbuild\tGOARCH=amd64\n\tbuild\tCGO_ENABLED=1\n")
        if command == ("go", "env", "GOVERSION"):
            return "go1.27.2\n"
        if command == (str(self.binary), "version"):
            return "azerlay 0.0.0\n"
        self.fail(f"unexpected command: {command}")

    def collect(self):
        with patch.object(inventory, "run", self.command), patch.object(inventory, "native_inventory", return_value=([], [], [])):
            return inventory.collect(self.args)

    def test_source_dependencies_are_not_mistaken_for_shipped_binary_or_host_packages(self):
        result = self.collect()
        components = {item["name"]: item for item in result["components"]}
        self.assertNotIn("unrelated-host", components)
        self.assertEqual(components["example.org/linked"]["scopes"], ["source-dependency", "shipped-binary"])
        self.assertEqual(components["example.org/test"]["scopes"], ["source-dependency", "build-or-test-only"])
        self.assertEqual(components["example.org/test"]["review_status"], "pending")
        self.assertEqual(result["artifacts"]["source"]["files"], ["THIRD_PARTY_NOTICES.md", "go.mod"])

    def test_binary_module_version_must_match_resolved_source(self):
        self.build_version = "v1.1.0"
        with self.assertRaisesRegex(ValueError, "binary/source module mismatch"):
            self.collect()

    def test_modified_extracted_source_cannot_describe_another_archive(self):
        (self.root / "go.mod").write_text("module example.org/other\n")
        with self.assertRaisesRegex(ValueError, "source archive and extracted source root differ"):
            self.collect()

    def test_malformed_syft_input_fails_instead_of_silently_ignoring_scanner(self):
        self.raw.write_text('{"packages": []}')
        with self.assertRaisesRegex(ValueError, "must be Syft JSON"):
            self.collect()


if __name__ == "__main__":
    unittest.main()
