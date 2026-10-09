import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

from test_ci_release_workflows import SHA, jobs, shell_body, step_blocks

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = (ROOT / ".github/workflows/release-dry-run.yml").read_text()
JOBS = jobs(WORKFLOW)


class ReleaseDryRunWorkflowTests(unittest.TestCase):
    def execute(self, body, **values):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            summary = Path(directory) / "summary"
            result = subprocess.run(
                ["bash", "-c", body], capture_output=True, text=True,
                env={**os.environ, "GITHUB_OUTPUT": str(output),
                     "GITHUB_STEP_SUMMARY": str(summary), **values},
            )
            return result, output.read_text() if output.exists() else "", summary.read_text() if summary.exists() else ""

    def test_only_manual_version_and_same_commit_reusable_validation(self):
        triggers = WORKFLOW.split("on:\n", 1)[1].split("\npermissions:", 1)[0]
        self.assertEqual(re.findall(r"^  ([a-z_]+):", triggers, re.M), ["workflow_dispatch"])
        self.assertEqual(re.findall(r"^      ([a-z_]+):", triggers, re.M), ["version"])
        self.assertIn("required: true", triggers)
        self.assertEqual(set(JOBS), {"full-arch", "build", "runtime", "dry-run-complete"})
        self.assertIn("uses: ./.github/workflows/arch-ci.yml", JOBS["full-arch"])
        self.assertIn("commit: ${{ github.sha }}", JOBS["full-arch"])
        self.assertIn("needs: [full-arch]", JOBS["build"])
        self.assertIn("ref: ${{ github.sha }}", JOBS["build"])
        self.assertIn("VALIDATED_COMMIT: ${{ needs.full-arch.outputs.validated-commit }}", JOBS["build"])

    def test_build_guard_rejects_failed_skipped_other_commit_and_wrong_tag(self):
        body = shell_body(JOBS["build"], "Require same-commit full Arch success")
        env = dict(ARCH_RESULT="success", VALIDATED_COMMIT=SHA, EXPECTED_COMMIT=SHA,
                   VERSION="1.2.3", GITHUB_REF="refs/heads/main")
        for ref in ("refs/heads/main", "refs/tags/v1.2.3", "refs/tags/1.2.3"):
            result, output, _ = self.execute(body, **{**env, "GITHUB_REF": ref})
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output, f"commit={SHA}\nversion=1.2.3\n")
        failures = {
            "ARCH_RESULT": ("failure", "skipped", "cancelled", ""),
            "VALIDATED_COMMIT": ("f" * 40, "", "main"),
            "EXPECTED_COMMIT": ("f" * 40, "", "main"),
            "VERSION": ("v1.2.3", "1.2.3\nother", ""),
            "GITHUB_REF": ("refs/tags/v1.2.4", "refs/tags/vv1.2.3"),
        }
        for key, values in failures.items():
            for value in values:
                with self.subTest(key=key, value=value):
                    result, output, _ = self.execute(body, **{**env, key: value})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(output, "")
        self.assertLess(JOBS["build"].index("Require same-commit full Arch success"),
                        JOBS["build"].index("Install candidate build dependencies"))

    def test_aggregate_rejects_any_incomplete_job_or_other_commit(self):
        job = JOBS["dry-run-complete"]
        self.assertIn("needs: [full-arch, build, runtime]", job)
        self.assertIn("if: ${{ always() }}", job)
        for name in ("full-arch", "build", "runtime"):
            self.assertIn("${{ needs." + name + ".result }}", job)
        body = shell_body(job, "Require every candidate job at the same commit")
        env = dict(ARCH_RESULT="success", BUILD_RESULT="success", RUNTIME_RESULT="success",
                   VALIDATED_COMMIT=SHA, BUILT_COMMIT=SHA, EXPECTED_COMMIT=SHA)
        result, _, summary = self.execute(body, **env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(SHA, summary)
        for key in env:
            values = ("failure", "skipped", "cancelled", "") if key.endswith("RESULT") else ("", "main", "f" * 40)
            for value in values:
                with self.subTest(key=key, value=value):
                    result, _, summary = self.execute(body, **{**env, key: value})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(summary, "")

    def test_explicit_complete_candidate_upload_and_separate_helper(self):
        steps = step_blocks(JOBS["build"])
        candidate = next(step for step in steps if step.startswith("Upload candidate outputs\n"))
        paths = re.findall(r"^            (.+)$", candidate, re.M)
        prefix = "${{ runner.temp }}/release-candidate/"
        name = "azerlay-${{ steps.candidate.outputs.version }}"
        self.assertCountEqual(paths, [prefix + name + suffix for suffix in (
            "-source.tar.gz", "-linux-x86_64.tar.gz", "-source.spdx.json",
            "-linux-x86_64.spdx.json", "-source.licenses.json", "-linux-x86_64.licenses.json",
        )] + [prefix + "release-notes.md", prefix + "SHA256SUMS"])
        for step in steps:
            if "uses: actions/upload-artifact@v4" in step:
                self.assertIn("if-no-files-found: error", step)
                self.assertIn("retention-days: 7", step)
        self.assertIn('"$RUNNER_TEMP/release-candidate"', JOBS["build"])
        self.assertIn("path: scripts/release-smoke.sh", JOBS["build"])

    def test_clean_runtime_has_only_runtime_packages_and_no_checkout_or_build(self):
        runtime = JOBS["runtime"]
        self.assertIn("needs: [build]", runtime)
        self.assertNotRegex(runtime, r"checkout@|setup-go@|native-go-cache|GOCACHE|GOMODCACHE")
        install = shell_body(runtime, "Install runtime dependencies and inspection tool")
        packages = install.split("pacman -Syu --noconfirm --needed ", 1)[1].strip().split()
        pkgbuild = (ROOT / "packaging/arch/PKGBUILD").read_text()
        depends = re.findall(r"'([^']+)'", re.search(r"^depends=\((.*?)\)", pkgbuild, re.M | re.S)[1])
        self.assertCountEqual(packages, depends + ["binutils"])
        smoke = shell_body(runtime, "Verify checksums and smoke as ordinary user")
        self.assertIn("runuser -u azerlay-smoke -- sha256sum -c SHA256SUMS", smoke)
        self.assertIn("runuser -u azerlay-smoke -- bash", smoke)
        self.assertLess(smoke.index("sha256sum -c"), smoke.index("release-smoke.sh"))
        self.assertIn("name: release-candidate", runtime)
        self.assertIn("name: release-smoke-helper", runtime)

    def test_build_tools_are_pinned_and_publication_permissions_absent(self):
        build = JOBS["build"]
        self.assertIn("go-version: 1.27.2", build)
        self.assertIn("source .github/ci-tools.env", build)
        self.assertIn("python-jsonschema", build)
        self.assertIn('sha256sum -c -', build)
        self.assertIn('https://github.com/anchore/syft/releases/download/$SYFT_VERSION', build)
        self.assertIn('https://raw.githubusercontent.com/anchore/syft/$SYFT_VERSION/schema/spdx-json/spdx-schema-2.3.json', build)
        self.assertEqual(re.findall(r"(?m)^permissions:\n((?:  .*\n)+)", WORKFLOW), ["  contents: read\n"])
        self.assertNotRegex(WORKFLOW, r"(?m)^ +permissions:")
        self.assertNotRegex(WORKFLOW, r"secrets[.:]|secrets:\s*inherit|persist-credentials:\s*true")
        self.assertNotRegex(WORKFLOW, r"(?i)\b(?:gh\s+release|git\s+(?:push|tag)|cosign|id-token|continue-on-error)\b")
        self.assertNotIn("issues.get", WORKFLOW)
        self.assertLessEqual(set(re.findall(r"\buses: (\S+)", WORKFLOW)), {
            "./.github/workflows/arch-ci.yml", "actions/checkout@v4", "actions/setup-go@v5",
            "actions/upload-artifact@v4", "actions/download-artifact@v4",
        })


if __name__ == "__main__":
    unittest.main()
