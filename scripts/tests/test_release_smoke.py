import os
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "release-smoke.sh"


class ReleaseSmokeTests(unittest.TestCase):
    def run_smoke(self, binary_version="1.2.3", linkage="libc.so => /usr/lib/libc.so (0x1)"):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary = root / "azerlay-1.2.3" / "bin" / "azerlay"
            binary.parent.mkdir(parents=True)
            binary.write_text(
                "#!/bin/bash\n"
                "test -z \"${INHERITED_SECRET:-}\" || exit 8\n"
                "test -d \"$HOME\" && test -z \"$(ls -A \"$HOME\")\" || exit 9\n"
                "case $1 in\n"
                f"version) printf 'azerlay {binary_version}\\n';;\n"
                "--help) printf 'Usage: azerlay\\n';;\n"
                "*) exit 2;;\nesac\n"
            )
            binary.chmod(0o755)
            archive = root / "binary.tar.gz"
            with tarfile.open(archive, "w:gz") as output:
                output.add(binary.parent.parent, arcname="azerlay-1.2.3")
            commands = root / "commands"
            commands.mkdir()
            for name, output in (("readelf", "Dynamic section"), ("ldd", linkage)):
                command = commands / name
                command.write_text(f"#!/bin/bash\nprintf '%s\\n' '{output}'\n")
                command.chmod(0o755)
            return subprocess.run(
                ["bash", str(SCRIPT), str(archive), "1.2.3"],
                env={**os.environ, "PATH": f"{commands}:{os.environ['PATH']}", "INHERITED_SECRET": "private"},
                capture_output=True, text=True,
            )

    def test_extracted_binary_version_help_and_clean_home(self):
        result = self.run_smoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("azerlay 1.2.3\nUsage: azerlay\n", result.stdout)
        self.assertIn("Release CLI startup and loader smoke passed.", result.stdout)

    def test_binary_version_mismatch_fails(self):
        result = self.run_smoke(binary_version="1.2.2")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Binary version does not match", result.stderr)
        self.assertNotIn("smoke passed", result.stdout)

    def test_unresolved_runtime_dependency_fails(self):
        result = self.run_smoke(linkage="libgtk-4.so.1 => not found")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unresolved runtime dependencies", result.stderr)
        self.assertNotIn("smoke passed", result.stdout)


if __name__ == "__main__":
    unittest.main()
