import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "release-notes.sh"


class ReleaseNotesTests(unittest.TestCase):
    def test_generates_candidate_notes_from_unreleased_changelog(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "release-notes.md"
            result = subprocess.run(
                ["bash", str(SCRIPT), "0.0.0-rc.1", str(output)],
                capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(output.is_file())
            self.assertGreater(output.stat().st_size, 0)

    def test_invalid_version_fails_without_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "release-notes.md"
            result = subprocess.run(
                ["bash", str(SCRIPT), "v1.2.3", str(output)],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("VERSION must be numeric and dotted", result.stderr)
            self.assertFalse(output.exists())

    def test_missing_changelog_fails_without_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            scripts = root / "scripts"
            scripts.mkdir()
            copied_script = scripts / "release-notes.sh"
            shutil.copyfile(SCRIPT, copied_script)
            output = root / "release-notes.md"
            result = subprocess.run(
                ["bash", str(copied_script), "1.2.3", str(output)],
                capture_output=True, text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("Required reviewed changelog is missing", result.stderr)
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
