"""Migration admission and security regressions for active workflow versions."""

from pathlib import Path
import re
import unittest

from test_ci_release_workflows import jobs
from workflow_contracts import (
    ACTION_VERSIONS, action_refs, allowed_actions, require_action,
    require_artifact_transfer, require_checkout_boundary, require_hosted_runner,
)


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_ACTIONS = {
    "ci.yml": {"actions/checkout", "actions/setup-go", "actions/upload-artifact",
               "./.github/actions/nix-go-cache"},
    "arch-ci.yml": {"actions/checkout", "actions/setup-go", "actions/upload-artifact",
                    "./.github/actions/native-go-cache"},
    "release-validation.yml": {"actions/github-script", "./.github/workflows/arch-ci.yml"},
    "release-dry-run.yml": {"actions/checkout", "actions/setup-go", "actions/upload-artifact",
                            "actions/download-artifact", "./.github/workflows/arch-ci.yml"},
    "renovate-notices.yml": {"actions/checkout", "actions/setup-go",
                             "actions/upload-artifact", "actions/download-artifact"},
}


def steps(source):
    return re.findall(r"^ +- [a-z][a-z-]*:.*?(?=^ +- [a-z][a-z-]*:|\Z)", source, re.M | re.S)


class WorkflowVersionTests(unittest.TestCase):
    def test_active_workflows_use_only_reviewed_actions_and_one_runner_class(self):
        runners = set()
        workflow_dir = ROOT / ".github/workflows"
        self.assertEqual({path.name for path in workflow_dir.glob("*.yml")}, set(WORKFLOW_ACTIONS))
        for filename, names in WORKFLOW_ACTIONS.items():
            source = (workflow_dir / filename).read_text()
            with self.subTest(workflow=filename):
                approved = {name for name in names if name.startswith("./")}
                approved |= allowed_actions(*(names - approved))
                self.assertLessEqual(set(action_refs(source)), approved)
                for job in jobs(source).values():
                    if not re.search(r"(?m)^    uses:", job):
                        runners.add(require_hosted_runner(job))
                for step in steps(source):
                    refs = action_refs(step)
                    if any(ref.startswith("actions/checkout@") for ref in refs):
                        require_checkout_boundary(step)
                    if any(ref.startswith(("actions/upload-artifact@", "actions/download-artifact@")) for ref in refs):
                        require_artifact_transfer(step)
                    if any(ref.startswith("actions/setup-go@") for ref in refs):
                        require_action(step, "actions/setup-go")
                        self.assertIn("go-version: 1.27.2", step)
                        self.assertRegex(step, r"(?m)^ +cache: (?:true|false)$")
                        if "cache: true" in step:
                            self.assertRegex(step, r"cache-dependency-path: \|\n +go.mod\n +go.sum")
        self.assertEqual(len(runners), 1, "Active gates must use the same approved runner class")

    def test_composite_actions_keep_cache_v6_and_explicit_go_cache_ownership(self):
        for kind in ("native", "nix"):
            source = (ROOT / f".github/actions/{kind}-go-cache/action.yml").read_text()
            names = {"actions/cache", "actions/cache/restore", "actions/cache/save"}
            if kind == "native":
                names.add("actions/setup-go")
                setup = next(step for step in steps(source) if "actions/setup-go@" in step)
                require_action(setup, "actions/setup-go")
                self.assertIn("go-version: 1.27.2", setup)
                self.assertIn("cache: false", setup)
            self.assertLessEqual(set(action_refs(source)), allowed_actions(*names))
            self.assertNotIn("restore-keys:", source)
            for name in names - {"actions/setup-go"}:
                self.assertIn(name + "@v6", source)

    def test_action_admission_rejects_unreviewed_versions_owners_and_paths(self):
        for name, versions in ACTION_VERSIONS.items():
            for version in versions:
                require_action(f"        uses: {name}@{version}\n", name)
                require_action(f"        uses: {name}@{version} # reviewed\n", name)
            for ref in (name + "@main", name + "@v999", name + "@v7.0.0",
                        name.replace("actions/", "attacker/", 1) + "@v7",
                        "./.github/actions/unreviewed"):
                with self.subTest(ref=ref), self.assertRaises(AssertionError):
                    require_action(f"        uses: {ref}\n", name)
                with self.subTest(ref=ref), self.assertRaises(AssertionError):
                    require_action(f"        uses: {ref} # must still be checked\n", name)
            with self.assertRaises(AssertionError):
                require_action(f"        uses: {name}@{next(iter(versions))}\n" * 2, name)

    def test_runner_admission_rejects_mutable_other_arch_and_self_hosted_labels(self):
        for runner in ("ubuntu-24.04", "ubuntu-26.04"):
            require_hosted_runner(f"    runs-on: {runner}\n")
        for runner in ("ubuntu-latest", "ubuntu-28.04", "ubuntu-26.04-arm",
                       "windows-2025", "[self-hosted, linux]", "${{ inputs.runner }}"):
            with self.subTest(runner=runner), self.assertRaises(AssertionError):
                require_hosted_runner(f"    runs-on: {runner}\n")
        with self.assertRaises(AssertionError):
            require_hosted_runner("")

    def test_checkout_migration_cannot_disable_trusted_code_boundary(self):
        step = "        uses: actions/checkout@v7\n          persist-credentials: false\n"
        require_checkout_boundary(step)
        for unsafe in (step.replace("false", "true"), step.replace("persist-credentials: false", ""),
                       step + "          allow-unsafe-pr-checkout: true\n",
                       step + "          token: ${{ secrets.PAT }}\n"):
            with self.subTest(step=unsafe), self.assertRaises(AssertionError):
                require_checkout_boundary(unsafe)

    def test_artifact_migration_cannot_weaken_transfer_or_digest_contract(self):
        upload = ("        uses: actions/upload-artifact@v7\n          name: candidate\n"
                  "          path: candidate/*\n          if-no-files-found: error\n")
        download = "        uses: actions/download-artifact@v8\n          name: candidate\n          path: /tmp/candidate\n"
        require_artifact_transfer(upload)
        require_artifact_transfer(download)
        for unsafe in (upload + "          archive: false\n", upload.replace("error", "warn"),
                       download + "          skip-decompress: true\n",
                       download + "          digest-mismatch: warn\n",
                       download + "          run-id: 123\n",
                       download.replace("          name: candidate\n", "")):
            with self.subTest(step=unsafe), self.assertRaises(AssertionError):
                require_artifact_transfer(unsafe)


if __name__ == "__main__":
    unittest.main()
