import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/ci-native-fingerprint.sh"


class NativeFingerprintTests(unittest.TestCase):
    def capture(self, package_version="1", compiler_status=0, package_status=0, job="test"):
        with tempfile.TemporaryDirectory() as directory:
            binaries = Path(directory)
            commands = {
                "cat": "case \"$1\" in /etc/os-release) echo ID=ubuntu;; /etc/apt/*) echo snapshot=fixed;; *) /bin/cat \"$@\";; esac",
                "dpkg-query": f"printf 'z-runtime\\t{package_version}\\tamd64\\na-compiler\\t1\\tamd64\\n'; exit {package_status}",
                "cc": f"echo compiler; exit {compiler_status}",
                "ld": "echo linker",
                "pkg-config": "echo pkg-config-result",
            }
            for command, body in commands.items():
                path = binaries / command
                path.write_text("#!/bin/sh\n" + body + "\n")
                path.chmod(0o700)
            env = dict(os.environ, PATH=str(binaries) + ":/usr/bin:/bin", CI_CACHE_JOB=job, CI_NATIVE_WORKFLOW=".github/workflows/arch-ci.yml")
            return subprocess.run(["bash", str(SCRIPT)], cwd=ROOT, env=env, capture_output=True, text=True)

    def test_inventory_is_complete_sorted_and_changes_boundary(self):
        first, changed = self.capture(), self.capture(package_version="2")
        self.assertEqual(first.returncode, 0, first.stderr)
        self.assertIn("a-compiler\t1\tamd64\nz-runtime\t1\tamd64", first.stdout)
        self.assertNotEqual(first.stdout, changed.stdout)
        self.assertIn("snapshot=fixed", first.stdout)
        self.assertIn(".github/workflows/arch-ci.yml", first.stdout)

    def test_jobs_never_share_native_builds(self):
        self.assertNotEqual(self.capture().stdout, self.capture(job="arch-compatibility").stdout)

    def test_native_failures_propagate(self):
        for kwargs in ({"compiler_status": 7}, {"package_status": 8}):
            result = self.capture(**kwargs)
            self.assertNotEqual(result.returncode, 0)

    def test_empty_job_is_rejected(self):
        self.assertNotEqual(self.capture(job="").returncode, 0)


if __name__ == "__main__":
    unittest.main()
