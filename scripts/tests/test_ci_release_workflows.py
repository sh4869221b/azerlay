"""Offline, fail-closed guards for daily and pre-release Arch validation.

These tests use Python's standard library plus the hosted runner's Node runtime;
they do not need an unpinned YAML dependency. Gate tests execute the exact
workflow shell and github-script bodies without network access.
"""

import os
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
ARCH = (ROOT / ".github/workflows/arch-ci.yml").read_text()
RELEASE = (ROOT / ".github/workflows/release-validation.yml").read_text()
GATES = {"test", "fuzz", "native-build", "vulnerability", "licenses", "generated-files"}
SHA = "0123456789abcdef0123456789abcdef01234567"


def jobs(workflow):
    return dict(re.findall(
        r"^  ([a-z][a-z-]*):\n(.*?)(?=^  [a-z][a-z-]*:\n|\Z)",
        workflow.split("jobs:\n", 1)[1], re.M | re.S,
    ))


def step_blocks(job):
    return re.findall(r"^      - name: (.*?)(?=^      - name: |\Z)", job, re.M | re.S)


def shell_body(job, step_name):
    matching = [step for step in step_blocks(job) if step.splitlines()[0] == step_name]
    if len(matching) != 1:
        raise AssertionError(f"Expected one step: {step_name}")
    block = matching[0].split("        run: |\n", 1)[1]
    return "\n".join(line[10:] if line.startswith("          ") else line
                     for line in block.splitlines()) + "\n"


ARCH_JOBS = jobs(ARCH)
RELEASE_JOBS = jobs(RELEASE)
RESULT_VARIABLES = (
    "REVISION_RESULT", "TEST_RESULT", "FUZZ_RESULT", "NATIVE_BUILD_RESULT",
    "VULNERABILITY_RESULT", "LICENSES_RESULT", "GENERATED_FILES_RESULT",
)


class ArchWorkflowContracts(unittest.TestCase):
    def test_only_daily_manual_and_reusable_triggers(self):
        triggers = ARCH.split("on:\n", 1)[1].split("\npermissions:", 1)[0]
        self.assertEqual(set(re.findall(r"^  ([a-z_]+):", triggers, re.M)),
                         {"schedule", "workflow_dispatch", "workflow_call"})
        self.assertEqual(re.findall(r"cron: '([^']+)'", triggers), ["17 20 * * *"])
        self.assertIn("05:17 JST", ARCH)
        self.assertIn("commit:\n        description:", triggers)
        self.assertIn("required: true\n        type: string", triggers)
        self.assertIn("value: ${{ jobs.arch-validation.outputs.commit }}", triggers)
        self.assertIn("cancel-in-progress: false", ARCH)

    def test_all_six_gates_run_at_one_commit(self):
        self.assertEqual(set(ARCH_JOBS), GATES | {"revision", "arch-validation"})
        for name in GATES:
            with self.subTest(job=name):
                job = ARCH_JOBS[name]
                self.assertIn("name: Arch " + name, job)
                self.assertIn("needs: revision", job)
                self.assertIn("runs-on: ubuntu-24.04", job)
                self.assertEqual(job.count("uses: actions/checkout@v4"), 1)
                self.assertIn("ref: ${{ github.sha }}", job)
                self.assertIn("persist-credentials: false", job)
                self.assertIn('git -c safe.directory="$GITHUB_WORKSPACE" rev-parse HEAD', job)
                self.assertIn('test "$actual" = "$EXPECTED_COMMIT"', job)
                self.assertLess(job.index("actions/checkout@v4"), job.index("Verify exact checked-out commit"))
                if name in {"fuzz", "generated-files"}:
                    # These tests rely on the non-root hosted-runner identity;
                    # moving them into a default-root container breaks fixtures.
                    self.assertNotIn("container:", job)
                    self.assertNotIn("sudo", job)
                else:
                    self.assertIn("container: archlinux:base", job)
                    self.assertEqual(job.count("pacman -Syu --noconfirm --needed"), 1)
                    self.assertNotIn("snapshot", job)
                    self.assertLess(job.index("pacman -Syu"), job.index("actions/checkout@v4"))

    def test_native_gate_is_complete_and_runs_wayland_nonroot(self):
        job = ARCH_JOBS["test"]
        self.assertIn("timeout-minutes: 60", job)
        for command in (
            "scripts/ci_check_packages.py", "gofmt -l .", "gofmt -d .",
            "go vet ./...", "staticcheck ./...", "go build ./cmd/azerlay",
            "useradd --create-home --user-group azerlay-test",
            "runuser -u azerlay-test", "scripts/ci-native-packages.txt",
            "scripts/test-wayland.sh go test -race -json -shuffle=on -count=1",
        ):
            self.assertIn(command, job)
        for package in ("gtk4", "gtk4-layer-shell", "sway", "dbus", "grim",
                        "noto-fonts-cjk", "noto-fonts-emoji"):
            self.assertRegex(job, rf"pacman -Syu[^\n]*\b{re.escape(package)}\b")
        self.assertNotIn("GTK_A11Y", ARCH)

    def test_native_binary_version_and_elf_smoke_remains(self):
        job = ARCH_JOBS["native-build"]
        for command in ("test \"$(go env CGO_ENABLED)\" = 1",
                        "pkg-config --modversion gtk4 gtk4-layer-shell-0",
                        "CGO_ENABLED=1 go build -o /tmp/azerlay ./cmd/azerlay",
                        "/tmp/azerlay version", "readelf -d /tmp/azerlay"):
            self.assertIn(command, job)

    def test_vulnerability_and_license_checks_remain(self):
        self.assertIn("govulncheck ./...", ARCH_JOBS["vulnerability"])
        self.assertIn('govulncheck@"$GOVULNCHECK_VERSION"', ARCH_JOBS["vulnerability"])
        self.assertIn('go-licenses/v2@"$GO_LICENSES_VERSION"', ARCH_JOBS["licenses"])
        self.assertIn("go-licenses report ./... --include_tests", ARCH_JOBS["licenses"])
        self.assertIn("arch-dependency-licenses", ARCH_JOBS["licenses"])

    def test_seven_exact_fuzz_targets_remain(self):
        targets = re.findall(r"-fuzz='\^([^']+)\$'", ARCH_JOBS["fuzz"])
        self.assertCountEqual(targets, [
            "FuzzNormalizeOuterText", "FuzzDecodeBase64", "FuzzDecodeLZMAEnvelope",
            "FuzzDecodeMsgpackString", "FuzzParseRoot", "FuzzParse", "FuzzNormalize",
        ])
        self.assertEqual(ARCH_JOBS["fuzz"].count("-fuzztime=10s -parallel=1 -timeout=2m"), 7)

    def test_core_race_and_generated_files_remain(self):
        job = ARCH_JOBS["generated-files"]
        for command in (
            "python3 -m unittest discover -s scripts/tests -v",
            "CGO_ENABLED: '0'", "scripts/ci-core-packages.txt",
            "python3 scripts/ci_check_packages.py --core-only",
            "go test -shuffle=on -count=1", "go test -race -json -shuffle=on -count=1",
            'export GOCACHE="$race_cache" CGO_ENABLED=1',
            "go run ./internal/profiledecode/testdata/generate",
            "git diff --exit-code -- internal/profiledecode/testdata/zeros-64mib.lzma internal/profiledecode/testdata/zeros-64mib-plus-one.lzma",
        ):
            self.assertIn(command, job)

    def test_full_native_cache_boundary_is_used_by_every_native_gate(self):
        for name in ("test", "native-build", "vulnerability", "licenses"):
            with self.subTest(job=name):
                job = ARCH_JOBS[name]
                self.assertNotIn("CI_CACHE_JOB:", job)
                self.assertNotIn("CI_NATIVE_WORKFLOW:", job)
                self.assertIn("bash scripts/ci-native-fingerprint.sh > .git/ci-native-cache-input", job)
                self.assertIn("uses: ./.github/actions/native-go-cache", job)
                self.assertIn("include-hidden-files: true", job)
        self.assertEqual(ARCH.count("module-writer: 'true'"), 1)
        self.assertIn("module-writer: 'true'", ARCH_JOBS["native-build"])

    def test_aggregate_requires_every_gate(self):
        job = ARCH_JOBS["arch-validation"]
        self.assertIn("needs: [revision, test, fuzz, native-build, vulnerability, licenses, generated-files]", job)
        self.assertIn("if: ${{ always() }}", job)
        self.assertIn("commit: ${{ steps.validated.outputs.commit }}", job)
        for name in GATES | {"revision"}:
            self.assertIn("${{ needs." + name + ".result }}", job)
        self.assertIn("VALIDATED_COMMIT: ${{ needs.revision.outputs.commit }}", job)


class ArchBootstrapExecution(unittest.TestCase):
    BOOTSTRAPS = {
        "test": "Install native test dependencies",
        "native-build": "Install Arch build dependencies",
        "vulnerability": "Install native build dependencies",
        "licenses": "Install native build dependencies",
    }
    PACKAGES = (
        "python ca-certificates git curl tar gzip zstd base-devel pkgconf gtk4 "
        "gtk4-layer-shell gobject-introspection"
    ).split()
    TEST_PACKAGES = "sway dbus grim noto-fonts-cjk noto-fonts-emoji".split()

    def execute(self, job, statuses, setcap_status=0):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            log = root / "commands.jsonl"
            for command in ("pacman", "sleep", "setcap"):
                stub = root / command
                stub.write_text(f"#!{sys.executable}\n" + '''import json
import os
from pathlib import Path
import sys

log = Path(os.environ["COMMAND_LOG"])
calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
command = Path(sys.argv[0]).name
with log.open("a") as output:
    output.write(json.dumps([command, *sys.argv[1:]]) + "\\n")
if command == "pacman":
    statuses = json.loads(os.environ["PACMAN_STATUSES"])
    attempt = sum(call[0] == "pacman" for call in calls)
    sys.exit(statuses[min(attempt, len(statuses) - 1)])
if command == "setcap":
    sys.exit(int(os.environ["SETCAP_STATUS"]))
''')
                stub.chmod(0o700)
            body = shell_body(ARCH_JOBS[job], self.BOOTSTRAPS[job])
            env = {
                **os.environ, "PATH": str(root) + os.pathsep + os.environ["PATH"],
                "COMMAND_LOG": str(log), "PACMAN_STATUSES": json.dumps(statuses),
                "SETCAP_STATUS": str(setcap_status),
            }
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", body],
                                    env=env, cwd=root, capture_output=True, text=True,
                                    timeout=5)
            return result, [json.loads(line) for line in log.read_text().splitlines()]

    def expected_calls(self, job, attempts, success):
        packages = self.PACKAGES + (self.TEST_PACKAGES if job == "test" else [])
        pacman = ["pacman", "-Syu", "--noconfirm", "--needed", *packages]
        calls = []
        for attempt in range(attempts):
            if attempt:
                calls.append(["sleep", str(10 * attempt)])
            calls.append(pacman)
        if success and job == "test":
            calls.append(["setcap", "-r", "/usr/bin/sway"])
        return calls

    def test_immediate_success_never_waits(self):
        for job in self.BOOTSTRAPS:
            with self.subTest(job=job):
                result, calls = self.execute(job, [0])
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, self.expected_calls(job, 1, True))
                self.assertEqual(result.stderr, "")

    def test_transient_failure_recovers_on_second_or_last_attempt(self):
        for job in self.BOOTSTRAPS:
            for statuses in ([1, 0], [1, 1, 0]):
                with self.subTest(job=job, statuses=statuses):
                    result, calls = self.execute(job, statuses)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(calls, self.expected_calls(job, len(statuses), True))
                    self.assertIn("retrying in 10s", result.stderr)
                    if len(statuses) == 3:
                        self.assertIn("retrying in 20s", result.stderr)
                    self.assertNotIn("failed after", result.stderr)

    def test_exhaustion_preserves_final_status_and_stops_before_setup(self):
        for job in self.BOOTSTRAPS:
            with self.subTest(job=job):
                # A fourth success must never be reached; the last failure wins.
                result, calls = self.execute(job, [1, 2, 23, 0])
                self.assertEqual(result.returncode, 23, result.stderr)
                self.assertEqual(calls, self.expected_calls(job, 3, False))
                self.assertIn("failed after 3 attempts (exit 23)", result.stderr)

    def test_permanent_installation_error_remains_blocking(self):
        for job in self.BOOTSTRAPS:
            with self.subTest(job=job):
                result, calls = self.execute(job, [1])
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual(calls, self.expected_calls(job, 3, False))

    def test_sway_setup_failure_is_not_retried_or_masked(self):
        result, calls = self.execute("test", [0], setcap_status=17)
        self.assertEqual(result.returncode, 17, result.stderr)
        self.assertEqual(calls, self.expected_calls("test", 1, True))


class ReleaseWorkflowContracts(unittest.TestCase):
    def test_prerelease_is_only_explicit_manual_validation(self):
        triggers = RELEASE.split("on:\n", 1)[1].split("\npermissions:", 1)[0]
        self.assertEqual(triggers.strip(), "workflow_dispatch:")
        self.assertEqual(set(RELEASE_JOBS), {"full-arch", "validation-complete"})
        caller = RELEASE_JOBS["full-arch"]
        self.assertIn("uses: ./.github/workflows/arch-ci.yml", caller)
        self.assertIn("commit: ${{ github.sha }}", caller)
        self.assertNotIn("ref:", caller)

    def test_prerelease_success_really_depends_on_same_commit_full_arch(self):
        job = RELEASE_JOBS["validation-complete"]
        self.assertIn("needs: [full-arch]", job)
        self.assertIn("if: ${{ always() }}", job)
        self.assertIn("ARCH_RESULT: ${{ needs.full-arch.result }}", job)
        self.assertIn("VALIDATED_COMMIT: ${{ needs.full-arch.outputs.validated-commit }}", job)
        self.assertIn("EXPECTED_COMMIT: ${{ github.sha }}", job)

    def test_publication_boundary_is_explicit(self):
        self.assertIn("Issues #18 and #39", RELEASE)
        self.assertIn("does not publish, tag, sign, or authorize a release", RELEASE)
        self.assertIn("Manual publication through the GitHub UI is not technically blocked", RELEASE)
        self.assertIn("Validate again if the candidate commit changes", RELEASE)
        self.assertIn("not legal clearance", RELEASE)

    def test_readonly_issue_blockers_are_required_after_arch_success(self):
        job = RELEASE_JOBS["validation-complete"]
        self.assertIn("issues: read", RELEASE)
        self.assertIn("for (const issue_number of [18, 39])", job)
        self.assertIn("github.rest.issues.get", job)
        self.assertIn("issue.state !== 'closed'", job)
        self.assertIn("issue.state_reason !== 'completed'", job)
        self.assertIn("core.setFailed", job)
        self.assertLess(job.index("Require same-commit full Arch success"),
                        job.index("Require resolved publication blockers"))

    def test_no_soft_failures_or_conditional_quality_gates(self):
        for workflow in (ARCH, RELEASE):
            self.assertNotIn("continue-on-error", workflow)
            self.assertNotIn("|| true", workflow)
            self.assertNotIn("|| :", workflow)
            for name, job in jobs(workflow).items():
                if name in {"arch-validation", "validation-complete"}:
                    self.assertEqual(re.findall(r"^    if: (.*)$", job, re.M), ["${{ always() }}"])
                else:
                    self.assertNotRegex(job, r"(?m)^    if:")
                for step in step_blocks(job):
                    if re.search(r"(?m)^        if:", step):
                        self.assertRegex(step.splitlines()[0], r"^Upload .* diagnostics$")
                        self.assertIn("uses: actions/upload-artifact@v4", step)

    def test_no_publishing_credentials_or_write_permissions(self):
        allowed_actions = {
            "actions/checkout@v4", "actions/setup-go@v5", "actions/upload-artifact@v4",
            "actions/github-script@v7",
            "./.github/actions/native-go-cache", "./.github/workflows/arch-ci.yml",
        }
        for workflow in (ARCH, RELEASE):
            expected = "  contents: read\n" + ("  issues: read\n" if workflow == RELEASE else "")
            self.assertEqual(re.findall(r"(?m)^permissions:\n((?:  .*\n)+)", workflow), [expected])
            self.assertNotRegex(workflow, r"(?m)^ +permissions:")
            self.assertNotRegex(workflow, r"(?i)\b(?:contents|actions|packages|id-token):\s*write")
            self.assertNotRegex(workflow, r"secrets[.:]|secrets:\s*inherit|persist-credentials:\s*true")
            self.assertNotRegex(workflow, r"(?m)^ +(?:environment|token|ssh-key):")
            self.assertNotRegex(workflow, r"(?i)\b(?:gh\s+release|git\s+(?:push|tag)|cosign|gpg\s+--sign)\b")
            self.assertNotIn("scripts/package.sh", workflow)
            self.assertNotRegex(workflow, r"github\.rest\.(?!issues\.get\b)")
            self.assertLessEqual(set(re.findall(r"\buses: (\S+)", workflow)), allowed_actions)


class WorkflowGateExecution(unittest.TestCase):
    def execute(self, body, **values):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            summary = Path(directory) / "summary"
            env = {**os.environ, "GITHUB_OUTPUT": str(output), "GITHUB_STEP_SUMMARY": str(summary), **values}
            result = subprocess.run(["bash", "-c", body], env=env, text=True, capture_output=True)
            return result, output.read_text() if output.exists() else "", summary.read_text() if summary.exists() else ""

    def test_each_checked_out_commit_is_verified_before_quality_gates(self):
        bodies = [shell_body(ARCH_JOBS[name], "Verify exact checked-out commit") for name in GATES]
        self.assertEqual(len(set(bodies)), 1)
        with tempfile.TemporaryDirectory() as directory:
            git = Path(directory) / "git"
            git.write_text('#!/bin/sh\n[ "$1" = -c ] || exit 2\n[ "$2" = "safe.directory=$GITHUB_WORKSPACE" ] || exit 2\n[ "$3" = rev-parse ] || exit 2\n[ "$4" = HEAD ] || exit 2\nprintf "%s\\n" "$ACTUAL_COMMIT"\n')
            git.chmod(0o700)
            for actual in (SHA, "f" * 40, ""):
                with self.subTest(actual=actual):
                    result, _, _ = self.execute(
                        bodies[0], ACTUAL_COMMIT=actual, EXPECTED_COMMIT=SHA,
                        GITHUB_WORKSPACE="/a workspace with spaces",
                        PATH=directory + os.pathsep + os.environ["PATH"],
                    )
                    self.assertEqual(result.returncode == 0, actual == SHA)

    def test_revision_accepts_only_exact_immutable_trigger_commit(self):
        body = shell_body(ARCH_JOBS["revision"], "Validate immutable commit")
        result, output, _ = self.execute(body, REQUESTED_COMMIT=SHA, TRIGGER_COMMIT=SHA)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(output, "commit=" + SHA + "\n")
        for requested, trigger in (("main", SHA), ("v1.0.0", SHA), ("f" * 40, SHA),
                                   ("", SHA), ("main", "main"), ("", "")):
            with self.subTest(requested=requested, trigger=trigger):
                result, output, _ = self.execute(body, REQUESTED_COMMIT=requested, TRIGGER_COMMIT=trigger)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(output, "")

    def test_arch_output_exists_only_after_every_gate_succeeds(self):
        body = shell_body(ARCH_JOBS["arch-validation"], "Require every full Arch gate")
        env = {key: "success" for key in RESULT_VARIABLES}
        env.update(EXPECTED_COMMIT=SHA, VALIDATED_COMMIT=SHA)
        result, output, summary = self.execute(body, **env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(output, "commit=" + SHA + "\n")
        self.assertIn(SHA, summary)
        for variable in RESULT_VARIABLES:
            for status in ("failure", "cancelled", "skipped", ""):
                with self.subTest(variable=variable, status=status):
                    result, output, summary = self.execute(body, **{**env, variable: status})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual((output, summary), ("", ""))
        for invalid in ("", "main", "f" * 40):
            result, output, _ = self.execute(body, **{**env, "VALIDATED_COMMIT": invalid})
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(output, "")

    def test_prerelease_success_cannot_reuse_a_skipped_failed_or_other_commit_run(self):
        body = shell_body(RELEASE_JOBS["validation-complete"], "Require same-commit full Arch success")
        env = dict(ARCH_RESULT="success", EXPECTED_COMMIT=SHA, VALIDATED_COMMIT=SHA)
        result, _, summary = self.execute(body, **env)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(SHA, summary)
        for key, values in (("ARCH_RESULT", ("failure", "cancelled", "skipped", "")),
                            ("VALIDATED_COMMIT", ("", "main", "f" * 40)),
                            ("EXPECTED_COMMIT", ("", "main", "f" * 40))):
            for value in values:
                with self.subTest(key=key, value=value):
                    result, _, summary = self.execute(body, **{**env, key: value})
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(summary, "")


class PublicationBlockerExecution(unittest.TestCase):
    def execute(self, issues, error=None):
        step = next(step for step in step_blocks(RELEASE_JOBS["validation-complete"])
                    if step.startswith("Require resolved publication blockers\n"))
        script = "\n".join(line[12:] for line in step.split("          script: |\n", 1)[1].splitlines())
        wrapper = r"""
const fixture = JSON.parse(process.env.FIXTURE);
const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
const requested = [];
const summary = [];
const context = {repo: {owner: 'owner', repo: 'azerlay'}};
const github = {rest: {issues: {get: async (args) => {
  if (args.owner !== 'owner' || args.repo !== 'azerlay') throw new Error('wrong repository');
  requested.push(args.issue_number);
  if (fixture.error) throw new Error(fixture.error);
  if (!Object.hasOwn(fixture.issues, args.issue_number)) throw new Error('404');
  return {data: fixture.issues[args.issue_number]};
}}}};
const core = {
  setFailed: (message) => { process.exitCode = 1; console.error(message); },
  summary: {addRaw: (text) => { summary.push(text); return {write: async () => {}}; }},
};
new AsyncFunction('github', 'context', 'core', process.env.SCRIPT)(github, context, core)
  .then(() => console.log(JSON.stringify({requested, summary})))
  .catch((error) => { console.error(error.message); process.exitCode = 1; });
"""
        env = {**os.environ, "FIXTURE": json.dumps({"issues": issues, "error": error}), "SCRIPT": script}
        return subprocess.run(["node", "-e", wrapper], env=env, text=True, capture_output=True)

    def completed(self):
        return {number: {"number": number, "state": "closed", "state_reason": "completed"}
                for number in (18, 39)}

    def test_only_both_completed_issues_pass(self):
        result = self.execute(self.completed())
        self.assertEqual(result.returncode, 0, result.stderr)
        details = json.loads(result.stdout)
        self.assertEqual(details["requested"], [18, 39])
        self.assertIn("not legal clearance", "".join(details["summary"]))

    def test_open_not_planned_unknown_or_wrong_issue_cannot_pass(self):
        for number in (18, 39):
            for change in ({"state": "open"}, {"state_reason": "not_planned"},
                           {"state_reason": None}, {"number": 100},
                           {"pull_request": {"url": "https://example.invalid"}}):
                with self.subTest(number=number, change=change):
                    issues = self.completed()
                    issues[number].update(change)
                    result = self.execute(issues)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(f"#{number}", result.stderr)

    def test_missing_issue_or_api_failure_cannot_pass(self):
        self.assertNotEqual(self.execute({}).returncode, 0)
        for error in ("403", "404", "500", "rate limited", "network unavailable"):
            with self.subTest(error=error):
                self.assertNotEqual(self.execute(self.completed(), error).returncode, 0)


if __name__ == "__main__":
    unittest.main()
