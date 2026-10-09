"""Offline packaging-structure checks. Stubs do not validate a real Go binary."""
import os
import re
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
LEGAL = ["LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md"]
LICENSE_TEXTS = (
    "LGPL-2.1-or-later.txt",
    "fswatcher-MIT.txt",
    "go-difflib-BSD-3-Clause.txt",
    "go-runtime-BSD-3-Clause.txt",
    "go-runtime-PATENTS.txt",
    "go-spew-ISC.txt",
    "go-toml-MIT.txt",
    "golang-BSD-3-Clause.txt",
    "golang-PATENTS.txt",
    "gotk4-cairo-ISC.txt",
    "gotk4-cairo-swizzle-BSD-3-Clause.txt",
    "gotk4-core-ISC.txt",
    "gotk4-gdkarrayimpl-NOTICE.txt",
    "gotk4-pkg-MPL-2.0.txt",
    "gtk4-layer-shell-MIT.txt",
    "purego-Apache-2.0.txt",
    "testify-MIT.txt",
    "urfave-cli-MIT.txt",
    "xz-BSD-3-Clause.txt",
    "yaml-v3-LICENSE.txt",
)


def required_legal_files(root):
    """A checked-in manifest must not shrink when a license is deleted."""
    required = LEGAL + ["LICENSES/" + name for name in LICENSE_TEXTS]
    for relative in required:
        if not (root / relative).is_file():
            raise FileNotFoundError(relative)
    return required


class LicensePackagingTests(unittest.TestCase):
    def test_source_and_binary_archives_include_exact_notices(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            tools = base / "tools"
            tools.mkdir()
            stubs = {
                "uname": '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; *) exit 1;; esac\n',
                "pkg-config": "#!/bin/sh\nexit 0\n",
                "go": '''#!/bin/sh
if [ "$1" = env ] && [ "$2" = GOVERSION ]; then echo go1.27.2; exit 0; fi
if [ "$1" != build ]; then exit 90; fi
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then shift; printf 'synthetic packaging test, not an executable\\n' > "$1"; exit 0; fi
  shift
done
exit 91
''',
            }
            for name, contents in stubs.items():
                path = tools / name
                path.write_text(contents)
                path.chmod(0o755)
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"])
            output = base / "archives"
            result = subprocess.run(
                ["bash", str(ROOT / "scripts/package.sh"), "0.0.0", str(output)],
                env=env, capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            expected = required_legal_files(ROOT)
            for archive in output.glob("*.tar.gz"):
                with self.subTest(archive=archive.name), tarfile.open(archive) as packed:
                    for relative in expected:
                        member = packed.extractfile("azerlay-0.0.0/" + relative)
                        self.assertIsNotNone(member)
                        self.assertEqual(member.read(), (ROOT / relative).read_bytes())
            self.assertEqual(len(list(output.glob("*.tar.gz"))), 2)

    def test_arch_package_installs_license_texts(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            source = base / "source" / "azerlay-0.0.0"
            source.mkdir(parents=True)
            for name in LEGAL + ["README.md"]:
                shutil.copyfile(ROOT / name, source / name)
            for name in ("LICENSES", "docs", "packaging"):
                shutil.copytree(ROOT / name, source / name)
            (source / "azerlay").write_text("synthetic packaging test, not an executable\n")
            package = base / "package"
            env = dict(os.environ, srcdir=str(source.parent), pkgdir=str(package))
            result = subprocess.run(
                ["bash", "-c", 'source "$1"; package', "test", str(ROOT / "packaging/arch/PKGBUILD")],
                env=env, capture_output=True, text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            for relative in required_legal_files(ROOT):
                self.assertEqual((package / "usr/share/licenses/azerlay" / relative).read_bytes(), (ROOT / relative).read_bytes())
            self.assertFalse((package / "usr/share/applications").exists())

    def test_required_manifest_and_notice_references_exist(self):
        required_legal_files(ROOT)
        self.assertEqual({p.name for p in (ROOT / "LICENSES").iterdir() if p.is_file()}, set(LICENSE_TEXTS))
        notice = (ROOT / "THIRD_PARTY_NOTICES.md").read_text()
        references = set(re.findall(r"LICENSES/([A-Za-z0-9.-]+\.txt)", notice))
        self.assertEqual(references, set(LICENSE_TEXTS))

    def test_missing_required_license_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "LICENSES").mkdir()
            omitted = "LICENSES/gotk4-pkg-MPL-2.0.txt"
            for relative in LEGAL + ["LICENSES/" + name for name in LICENSE_TEXTS]:
                if relative != omitted:
                    (root / relative).write_text("synthetic existence test\n")
            with self.assertRaisesRegex(FileNotFoundError, "gotk4-pkg-MPL-2.0"):
                required_legal_files(root)

    def test_notice_inventory_matches_go_mod(self):
        lines = (ROOT / "go.mod").read_text().splitlines()
        notice = (ROOT / "THIRD_PARTY_NOTICES.md").read_text()
        for line in lines:
            fields = line.strip().split()
            if len(fields) >= 2 and fields[1].startswith("v"):
                self.assertIn(fields[0] + " " + fields[1], notice)


if __name__ == "__main__":
    unittest.main()
