"""Historical guards for the archived Ubuntu comparison candidate."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = (ROOT / "docs/measurements/ci-136-comparison/workflows/ci-ubuntu.yml").read_text()
JOBS = dict(re.findall(r"^  ([a-z][a-z-]*):\n(.*?)(?=^  [a-z][a-z-]*:\n|\Z)", WORKFLOW.split("jobs:\n", 1)[1], re.M | re.S))


class StableWorkflowTests(unittest.TestCase):
    def test_runner_metadata_is_recorded(self):
        for job in JOBS.values():
            self.assertIn("Record runner comparison metadata", job)
            self.assertIn("logical_cpu_count", job)
            self.assertIn("cpu_model=", job)

    def test_exact_gate_set_and_no_soft_failures(self):
        self.assertEqual(set(JOBS), {"test", "fuzz", "native-build", "vulnerability", "licenses", "generated-files", "arch-compatibility"})
        self.assertNotIn("continue-on-error", WORKFLOW)
        self.assertNotIn("schedule:", WORKFLOW)
        self.assertNotIn("restore-keys", (ROOT / ".github/actions/native-go-cache/action.yml").read_text())

    def test_fixed_native_sources_and_verified_bootstrap(self):
        for name in ("test", "native-build", "vulnerability", "licenses"):
            with self.subTest(job=name):
                job = JOBS[name]
                self.assertIn("ubuntu:26.04@sha256:88a381d5b5eeb2b35d3ad70925a362c37ce569daf43ede89ff818ec20e4d3794", job)
                self.assertIn("6077d27c6b6f8b23590cb01ff877ed8c804a67a5442cc32b5a33da10d2bd0e90", job)
                self.assertIn("actual !== expected", job)
                self.assertIn("URIs: https://snapshot.ubuntu.com/ubuntu/20261001T000000Z/", job)
                self.assertIn("Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg", job)
                self.assertIn("APT::Get::Always-Include-Phased-Updates", job)
                self.assertIn("apt-get update --error-on=any", job)
                self.assertIn("sha256sum --check", job)
                self.assertIn("Unexpected non-snapshot APT source", job)
                self.assertLess(job.index("Bootstrap snapshot CA"), job.index("Install pinned Ubuntu"))
                self.assertLess(job.index("Install pinned Ubuntu"), job.index("Check out repository"))
                self.assertIn("ci-native-fingerprint.sh", job)
                self.assertIn("include-hidden-files: true", job)

    def test_native_race_and_arch_contracts_remain(self):
        for name in ("test", "arch-compatibility"):
            job = JOBS[name]
            for command in ("go vet ./...", "staticcheck ./...", "runuser -u azerlay-test", "scripts/test-wayland.sh go test -race -json -shuffle=on -count=1", "scripts/ci-native-packages.txt", "scripts/ci_check_packages.py"):
                self.assertIn(command, job)
        arch = JOBS["arch-compatibility"]
        self.assertIn("container: archlinux:base", arch)
        self.assertIn("pacman -Syu", arch)
        self.assertIn("CGO_ENABLED=1 go build", arch)
        self.assertIn("readelf -d", arch)
        self.assertIn("arch-native-race-json", arch)
        self.assertIn("libcap2-bin at-spi2-core", JOBS["test"])
        self.assertIn("test -f /usr/share/dbus-1/services/org.a11y.Bus.service", JOBS["test"])
        self.assertNotIn("GTK_A11Y", WORKFLOW)

    def test_existing_non_native_gates_remain(self):
        self.assertEqual(JOBS["fuzz"].count("-fuzztime=10s -parallel=1 -timeout=2m"), 7)
        for command in ("CGO_ENABLED: '0'", "go test -shuffle=on -count=1", "go test -race -json -shuffle=on -count=1", "go run ./internal/profiledecode/testdata/generate", "git diff --exit-code"):
            self.assertIn(command, JOBS["generated-files"])
        self.assertIn("govulncheck ./...", JOBS["vulnerability"])
        self.assertIn("go-licenses report ./... --include_tests", JOBS["licenses"])


if __name__ == "__main__":
    unittest.main()
