import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "ci_check_packages.py"
spec = importlib.util.spec_from_file_location("ci_check_packages", SCRIPT)
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)
MODULE = "example.invalid/azerlay"


class PackageBoundaryTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / "scripts").mkdir()
        self.core = ["./internal/cli", "./internal/config"]
        self.native = ["./cmd/azerlay", "./internal/live"]
        self.write_manifests()
        self.core_imports = [MODULE + package[1:] for package in self.core]
        self.native_imports = [MODULE + package[1:] for package in self.native]
        self.listed = self.native_imports + self.core_imports
        self.dependencies = ["fmt", *self.core_imports,
                             MODULE + "/internal/cli [" + MODULE + "/internal/cli.test]",
                             MODULE + "/internal/cli_test [" + MODULE + "/internal/cli.test]",
                             MODULE + "/internal/cli.test"]
        self.calls = []

    def write_manifests(self):
        for name, packages in (("core", self.core), ("native", self.native)):
            (self.root / f"scripts/ci-{name}-packages.txt").write_text("\n".join(packages) + "\n")

    def fake_go(self, root, *args):
        self.assertEqual(root, self.root)
        self.calls.append(args)
        if args == ("list", "-m", "-f", "{{.Path}}"):
            return [MODULE]
        if args == ("list", "./..."):
            return self.listed
        if args == ("list", "-deps", "-test", "-f", "{{.ImportPath}}", *self.core):
            return self.dependencies
        self.fail(f"unexpected go command: {args}")

    def check(self, core_only=False):
        with patch.object(ci, "run_go", self.fake_go):
            return ci.check(self.root, core_only)

    def test_nested_module_is_not_a_root_manifest_package(self):
        nested = self.root / "internal/live"
        nested.mkdir(parents=True)
        (nested / "go.mod").write_text("module example.invalid/nested\n")
        with self.assertRaisesRegex(ValueError, "nested module"):
            self.check()
        self.assertEqual(self.calls, [])

    def test_exact_partition_and_test_dependencies(self):
        self.assertEqual(self.check(), (2, 2))
        self.assertEqual(len(self.calls), 3)
        self.assertIn(("list", "./..."), self.calls)

    def test_core_only_never_lists_native_packages(self):
        self.listed = None  # Any attempted all-package listing would fail.
        with patch.dict("os.environ", {"CGO_ENABLED": "0"}):
            self.assertEqual(self.check(core_only=True), (2, 2))
        self.assertEqual(len(self.calls), 2)
        self.assertNotIn(("list", "./..."), self.calls)
        self.assertFalse(any(package in call for call in self.calls for package in self.native))

    def test_partition_rejects_unassigned_stale_and_duplicate_output(self):
        for output, diagnostic in ((self.listed + [MODULE + "/internal/new"], "unassigned"),
                                   (self.listed[:-1], "stale"),
                                   (self.listed + self.listed[:1], "exact manifest union")):
            with self.subTest(output=output):
                self.listed = output
                with self.assertRaisesRegex(ValueError, diagnostic):
                    self.check()

    def test_manifests_must_be_disjoint(self):
        self.native.append(self.core[0])
        self.native.sort()
        self.write_manifests()
        with self.assertRaisesRegex(ValueError, "both manifests"):
            self.check()
        self.assertEqual(self.calls, [])

    def test_manifest_rejects_empty_unsorted_duplicate_and_patterns(self):
        path = self.root / "manifest.txt"
        for text in ("", "\n", "./b\n./a\n", "./a\n./a\n", "./...\n", "./internal/...\n", "./internal/cli...\n",
                     "./internal/*\n", "./internal/../live\n", "./internal/./cli\n", "./a b\n",
                     "--tags=hide_native\n", "github.com/example/pkg\n", " ./internal/cli\n"):
            with self.subTest(text=text):
                path.write_text(text)
                with self.assertRaises(ValueError):
                    ci.read_manifest(path)

    def test_native_and_gotk4_leaks_include_test_variants(self):
        for dependency in (MODULE + "/cmd/azerlay", MODULE + "/internal/live",
                           MODULE + "/internal/live/helper", MODULE + "/internal/live_test",
                           "github.com/diamondburned/gotk4/pkg/gtk/v4"):
            for suffix in ("", " [" + MODULE + "/internal/cli.test]"):
                with self.subTest(dependency=dependency, suffix=suffix):
                    original = list(self.dependencies)
                    self.dependencies.append(dependency + suffix)
                    with self.assertRaisesRegex(ValueError, "core imports native/GUI"):
                        self.check(core_only=True)
                    self.dependencies = original

    def test_similarly_named_packages_are_not_native(self):
        self.dependencies += [MODULE + "/internal/lively", "github.com/diamondburned/gotk4extra"]
        self.assertEqual(self.check(), (2, 2))

    def test_empty_or_partial_dependency_listing_fails(self):
        for dependencies in ([], ["fmt"], self.core_imports[:1]):
            with self.subTest(dependencies=dependencies):
                self.dependencies = dependencies
                with self.assertRaisesRegex(ValueError, "omitted packages"):
                    self.check(core_only=True)

    def test_go_failure_is_not_accepted_as_partial_success(self):
        failure = subprocess.CalledProcessError(1, ["go", "list"],
                                                output="fmt\n", stderr="missing GTK\n")
        with patch.object(ci.subprocess, "run", side_effect=failure) as run:
            with self.assertRaisesRegex(ValueError, "missing GTK"):
                ci.run_go(self.root, "list", "./...")
        run.assert_called_once_with(["go", "list", "./..."], cwd=self.root,
                                    check=True, capture_output=True, text=True)

    def test_checked_in_manifests_are_sorted_and_disjoint(self):
        scripts = SCRIPT.parent
        core = ci.read_manifest(scripts / "ci-core-packages.txt")
        native = ci.read_manifest(scripts / "ci-native-packages.txt")
        self.assertFalse(set(core) & set(native))


if __name__ == "__main__":
    unittest.main()
