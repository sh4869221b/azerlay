"""Migration admission and security regressions for active workflow versions."""

from pathlib import Path
import tempfile
import unittest

from workflow_contracts import (
    ACTION_VERSIONS, action_refs, allowed_actions, require_action,
    require_artifact_transfer, require_checkout_boundary, require_hosted_runner,
    inputs, steps, workflow_jobs,
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


def validate_active_workflows(workflow_dir):
    files = {path.name for path in workflow_dir.iterdir() if path.suffix in {".yml", ".yaml"}}
    if files != set(WORKFLOW_ACTIONS):
        raise AssertionError(f"Workflow inventory requires review: {files}")
    runners = set()
    for filename, names in WORKFLOW_ACTIONS.items():
        source = (workflow_dir / filename).read_text()
        approved = {name for name in names if name.startswith("./")}
        approved |= allowed_actions(*(names - approved))
        if not set(action_refs(source)) <= approved:
            raise AssertionError(f"Unreviewed action in {filename}")
        for job in workflow_jobs(source).values():
            if "uses" not in job:
                runners.add(require_hosted_runner(job))
        for step in steps(source):
            refs = action_refs(step)
            if any(ref.startswith("actions/checkout@") for ref in refs):
                require_checkout_boundary(step)
            if any(ref.startswith(("actions/upload-artifact@", "actions/download-artifact@")) for ref in refs):
                require_artifact_transfer(step)
            if any(ref.startswith("actions/setup-go@") for ref in refs):
                require_action(step, "actions/setup-go")
                values = inputs(step)
                if values.get("go-version") != "1.27.2" or values.get("cache") not in {"true", "false"}:
                    raise AssertionError("Go version and cache ownership must be explicit")
                if values["cache"] == "true" and values.get("cache-dependency-path", "").splitlines() != ["go.mod", "go.sum"]:
                    raise AssertionError("Go cache must track both module inputs")
    if len(runners) != 1:
        raise AssertionError("Active gates must use the same approved runner class")


class WorkflowVersionTests(unittest.TestCase):
    def test_active_workflows_use_only_reviewed_actions_and_one_runner_class(self):
        validate_active_workflows(ROOT / ".github/workflows")

    def test_composite_actions_keep_cache_v6_and_explicit_go_cache_ownership(self):
        for kind in ("native", "nix"):
            source = (ROOT / f".github/actions/{kind}-go-cache/action.yml").read_text()
            names = {"actions/cache", "actions/cache/restore", "actions/cache/save"}
            if kind == "native":
                names.add("actions/setup-go")
                setup = next(step for step in steps(source) if any(ref.startswith("actions/setup-go@") for ref in action_refs(step)))
                require_action(setup, "actions/setup-go")
                self.assertEqual(inputs(setup).get("go-version"), "1.27.2")
                self.assertEqual(inputs(setup).get("cache"), "false")
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
                require_action({"steps": [{"uses": name + "@" + next(iter(versions))}] * 2}, name)

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
        step = "uses: actions/checkout@v7\nwith:\n  persist-credentials: false\n"
        require_checkout_boundary(step)
        for unsafe in (step.replace("false", "true"), step.replace("persist-credentials: false", ""),
                       step + "  allow-unsafe-pr-checkout: true\n",
                       step + "  token: ${{ secrets.PAT }}\n"):
            with self.subTest(step=unsafe), self.assertRaises(AssertionError):
                require_checkout_boundary(unsafe)

    def test_artifact_migration_cannot_weaken_transfer_or_digest_contract(self):
        upload = ("uses: actions/upload-artifact@v7\nwith:\n  name: candidate\n"
                  "  path: candidate/*\n  if-no-files-found: error\n")
        download = "uses: actions/download-artifact@v8\nwith:\n  name: candidate\n  path: /tmp/candidate\n"
        require_artifact_transfer(upload)
        require_artifact_transfer(download)
        for unsafe in (upload + "  archive: false\n", upload.replace("error", "warn"),
                       download + "  skip-decompress: true\n",
                       download + "  digest-mismatch: warn\n",
                       download + "  run-id: 123\n",
                       download.replace("  name: candidate\n", "")):
            with self.subTest(step=unsafe), self.assertRaises(AssertionError):
                require_artifact_transfer(unsafe)

    def test_yaml_extension_cannot_escape_workflow_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory)
            for name in WORKFLOW_ACTIONS:
                (target / name).write_text((ROOT / ".github/workflows" / name).read_text())
            validate_active_workflows(target)
            (target / "unreviewed.yaml").write_text("jobs: {escape: {runs-on: self-hosted}}")
            with self.assertRaisesRegex(AssertionError, "inventory"):
                validate_active_workflows(target)

    def test_all_legal_job_identifiers_receive_runner_admission(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory)
            for name in WORKFLOW_ACTIONS:
                (target / name).write_text((ROOT / ".github/workflows" / name).read_text())
            path = target / "ci.yml"
            original = path.read_text()
            for name in ("Test_2", "_job", "test2"):
                with self.subTest(job=name):
                    path.write_text(original + f"\n  {name}:\n    runs-on: self-hosted\n    steps: [{{run: echo escape}}]\n")
                    with self.assertRaisesRegex(AssertionError, "runner"):
                        validate_active_workflows(target)

    def test_inputs_must_belong_to_the_actual_with_mapping(self):
        checkout = "uses: actions/checkout@v7\nenv:\n  DECOY: |\n    persist-credentials: false\n"
        upload = "uses: actions/upload-artifact@v7\nenv:\n  DECOY: |\n    name: candidate\n    path: candidate/*\n    if-no-files-found: error\n"
        download = upload.replace("upload-artifact@v7", "download-artifact@v8")
        for checker, source in ((require_checkout_boundary, checkout),
                                (require_artifact_transfer, upload), (require_artifact_transfer, download)):
            with self.subTest(source=source), self.assertRaises(AssertionError):
                checker(source)
        for value in ("false", "[]", "credentials", '"{persist-credentials: false}"'):
            with self.subTest(value=value), self.assertRaises(AssertionError):
                require_checkout_boundary(f"uses: actions/checkout@v7\nwith: {value}\n")

    def test_checkout_cannot_redirect_default_token_to_another_origin(self):
        with self.assertRaisesRegex(AssertionError, "origin"):
            require_checkout_boundary("uses: actions/checkout@v7\nwith:\n  persist-credentials: false\n  github-server-url: https://attacker.example\n")

    def test_flow_style_actions_and_inputs_receive_the_same_checks(self):
        self.assertEqual(action_refs("steps: [{uses: attacker/action@main}]"), ["attacker/action@main"])
        require_checkout_boundary(steps("steps: [{uses: actions/checkout@v7, with: {persist-credentials: false}}]")[0])
        for source, checker in (
            ("steps: [{uses: actions/checkout@v7, with: {persist-credentials: true}}]", require_checkout_boundary),
            ("steps: [{uses: actions/upload-artifact@v7, with: {name: candidate, path: candidate/*, if-no-files-found: warn}}]", require_artifact_transfer),
            ("steps: [{uses: actions/download-artifact@v8, with: {name: candidate, path: /tmp/candidate, digest-mismatch: warn}}]", require_artifact_transfer),
        ):
            with self.subTest(source=source), self.assertRaises(AssertionError):
                checker(steps(source)[0])
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory)
            for name in WORKFLOW_ACTIONS:
                (target / name).write_text((ROOT / ".github/workflows" / name).read_text())
            path = target / "ci.yml"
            path.write_text(path.read_text() + "\n  flow: {runs-on: ubuntu-24.04, steps: [{uses: attacker/action@main}]}\n")
            with self.assertRaisesRegex(AssertionError, "Unreviewed action"):
                validate_active_workflows(target)

    def test_ambiguous_yaml_must_fail_closed(self):
        for source in ("uses: actions/checkout@v7\nuses: attacker/action@main\n",
                       "steps: {uses: attacker/action@main}",
                       "steps: [{<<: {uses: attacker/action@main}}]",
                       "steps: [&step {uses: actions/checkout@v7}, *step]"):
            with self.subTest(source=source), self.assertRaises(AssertionError):
                action_refs(source)

    def test_ci_installs_only_hashed_test_dependencies_in_a_temporary_target(self):
        script = (ROOT / "scripts/ci-python-tests.sh").read_text()
        for flag in ("--require-hashes", "--no-deps", "--only-binary=:all:", "--no-cache-dir", '--target "$dependencies"'):
            self.assertIn(flag, script)
        self.assertIn("mktemp -d", script)
        self.assertIn('trap \'rm -rf "$dependencies"\' EXIT', script)
        self.assertIn("python3 -m unittest discover -s scripts/tests -v", script)
        requirements = (ROOT / "scripts/tests/requirements.txt").read_text()
        self.assertIn("PyYAML==6.0.3", requirements)
        self.assertIn("--hash=sha256:", requirements)
        for name in ("ci.yml", "arch-ci.yml"):
            self.assertIn("run: bash scripts/ci-python-tests.sh", (ROOT / ".github/workflows" / name).read_text())


if __name__ == "__main__":
    unittest.main()
