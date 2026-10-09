"""Candidate boundary checks; the package stub does not validate a real binary."""
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class ReleaseDryRunTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.repo = self.base / "repo"
        scripts = self.repo / "scripts"
        scripts.mkdir(parents=True)
        shutil.copyfile(ROOT / "scripts/release-dry-run.sh", scripts / "release-dry-run.sh")
        (scripts / "package.sh").write_text(
            '#!/bin/sh\nprintf "%s\\n" "$1" > "$2/package-called"\nexit 42\n'
        )
        (self.repo / ".github").mkdir()
        shutil.copyfile(ROOT / ".github/ci-tools.env", self.repo / ".github/ci-tools.env")
        (self.repo / ".gitignore").write_text(".omo/\n")
        self.git("init", "-q")
        self.git("add", ".")
        self.git("-c", "user.name=Test", "-c", "user.email=test@example.invalid",
                 "commit", "-qm", "candidate fixture")
        self.output = self.base / "output"
        self.tools = self.base / "tools"
        self.tools.mkdir()
        for name, content in {"go": '#!/bin/sh\necho go1.27.2\n',
                              "syft": '#!/bin/sh\necho \'{"version":"1.54.1"}\'\n'}.items():
            tool = self.tools / name
            tool.write_text(content)
            tool.chmod(0o755)
        self.schema = self.base / "schema.json"
        self.schema.write_text("{}")

    def git(self, *args):
        return subprocess.run(["git", "-C", str(self.repo), *args], check=True,
                              capture_output=True, text=True)

    def run_candidate(self, version="1.2.3", ref=None, output=None):
        env = dict(os.environ)
        env.pop("GITHUB_REF", None)
        env["PATH"] = str(self.tools) + os.pathsep + env["PATH"]
        env["SPDX_SCHEMA"] = str(self.schema)
        if ref is not None:
            env["GITHUB_REF"] = ref
        return subprocess.run(
            ["bash", str(self.repo / "scripts/release-dry-run.sh"), version,
             str(output if output is not None else self.output)],
            env=env, capture_output=True, text=True,
        )

    def assert_rejected_before_package(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertFalse((self.output / "package-called").exists())

    def test_branch_and_matching_tags_call_package_with_explicit_version(self):
        for ref in (None, "refs/heads/release", "refs/tags/v1.2.3", "refs/tags/1.2.3"):
            with self.subTest(ref=ref):
                result = self.run_candidate(ref=ref)
                self.assertEqual(result.returncode, 42, result.stderr)
                self.assertEqual((self.output / "package-called").read_text(), "1.2.3\n")
                shutil.rmtree(self.output)

    def test_invalid_version_and_mismatched_tags_fail_before_packaging(self):
        for version, ref in (("../1.2.3", None), ("1.2.4", "refs/tags/v1.2.3"),
                             ("1.2.3", "refs/tags/vv1.2.3")):
            with self.subTest(version=version, ref=ref):
                result = self.run_candidate(version, ref)
                self.assert_rejected_before_package(result)
                self.assertFalse(self.output.exists())

    def test_modified_and_untracked_files_fail_before_packaging(self):
        tracked = self.repo / ".gitignore"
        original = tracked.read_text()
        tracked.write_text(original + "other/\n")
        result = self.run_candidate()
        self.assert_rejected_before_package(result)
        self.assertIn("clean checkout", result.stderr)
        tracked.write_text(original)
        (self.repo / "untracked.txt").write_text("product file\n")
        result = self.run_candidate()
        self.assert_rejected_before_package(result)
        self.assertIn("clean checkout", result.stderr)

    def test_ignored_omo_files_do_not_dirty_candidate(self):
        (self.repo / ".omo").mkdir()
        (self.repo / ".omo" / "local.txt").write_text("local workflow state\n")
        result = self.run_candidate()
        self.assertEqual(result.returncode, 42, result.stderr)

    def test_nonempty_output_including_hidden_files_is_rejected(self):
        self.output.mkdir()
        (self.output / ".existing").write_text("preserve\n")
        result = self.run_candidate()
        self.assert_rejected_before_package(result)
        self.assertIn("must be empty", result.stderr)
        self.assertEqual((self.output / ".existing").read_text(), "preserve\n")

    def test_source_tree_output_and_symlink_into_source_are_rejected(self):
        link = self.base / "source-link"
        link.symlink_to(self.repo, target_is_directory=True)
        for output in (self.repo, self.repo / ".omo" / "output", link / ".omo" / "output"):
            with self.subTest(output=output):
                result = self.run_candidate(output=output)
                self.assert_rejected_before_package(result)
                self.assertIn("outside the source tree", result.stderr)


if __name__ == "__main__":
    unittest.main()
