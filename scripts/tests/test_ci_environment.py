import os
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "ci-measure-environment.sh"


class CaptureTests(unittest.TestCase):
    def test_existing_destination_is_not_modified(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "marker"
            marker.write_text("keep")
            result = subprocess.run(["bash", str(SCRIPT), directory], capture_output=True)
            self.assertEqual(result.returncode, 2)
            self.assertEqual(marker.read_text(), "keep")

    def test_required_metadata_failures_propagate(self):
        for failed in ("date", "git", "getconf"):
            with self.subTest(failed=failed), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                binaries = root / "bin"
                binaries.mkdir()
                # Synthetic command stubs test failure plumbing, not native readiness.
                for command in ("date", "git", "getconf", "go", "pkg-config", "pacman", "sha256sum", "uname", "lscpu"):
                    path = binaries / command
                    path.write_text("#!/bin/sh\nexit " + ("7" if command == failed else "0") + "\n")
                    path.chmod(0o700)
                env = dict(os.environ, PATH=str(binaries) + ":/usr/bin:/bin")
                result = subprocess.run(["bash", str(SCRIPT), str(root / "output")], env=env, capture_output=True)
                self.assertEqual(result.returncode, 7, result.stderr.decode())
                self.assertIn(b"capture failed", result.stderr)


if __name__ == "__main__":
    unittest.main()
