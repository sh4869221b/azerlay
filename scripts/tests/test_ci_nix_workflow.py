"""Offline guard for adopted normal Nix CI and unchanged quality gates."""
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = (ROOT / '.github/workflows/ci.yml').read_text()
JOBS = dict(re.findall(r'^  ([a-z][a-z-]*):\n(.*?)(?=^  [a-z][a-z-]*:\n|\Z)', WORKFLOW.split('jobs:\n', 1)[1], re.M | re.S))

class NixWorkflowTests(unittest.TestCase):
    def test_runner_metadata_is_recorded(self):
        for job in JOBS.values():
            self.assertIn("Record runner comparison metadata", job)
            self.assertIn("logical_cpu_count", job)
            self.assertIn("cpu_model=", job)

    def test_complete_native_set(self):
        self.assertEqual(set(JOBS), {'test', 'native-build', 'vulnerability', 'licenses', 'fuzz', 'generated-files'})
        self.assertNotIn('continue-on-error', WORKFLOW)
        self.assertNotIn('GTK_A11Y', WORKFLOW)
        for name in ('test', 'native-build', 'vulnerability', 'licenses'):
            job = JOBS[name]
            role = 'test' if name == 'test' else 'build'
            self.assertIn('runs-on: ubuntu-24.04', job)
            self.assertIn('CI_NIX_ROLE: ' + role, job)
            self.assertNotIn('CI_NIX_test', job)
            self.assertNotIn('CI_NIX_build', job)
            self.assertIn('--role "$CI_NIX_ROLE"', job)
            self.assertIn('.#ci-$CI_NIX_ROLE', job)
            self.assertIn('ci-nix-fingerprint.sh', job)
            self.assertIn('ci-nix-run.sh', job)
            self.assertIn('include-hidden-files: true', job)
            self.assertNotIn('actions/setup-go', job)

    def test_equivalent_quality_commands(self):
        for command in ('go vet ./...', 'staticcheck ./...', 'scripts/ci_check_packages.py', 'gofmt -l .', 'scripts/test-wayland.sh go test -race -json -shuffle=on -count=1', 'scripts/ci-native-packages.txt', 'go build ./cmd/azerlay'):
            self.assertIn(command, JOBS['test'])
        for command in ('CGO_ENABLED=1 go build', '/tmp/azerlay version', 'readelf -d /tmp/azerlay'):
            self.assertIn(command, JOBS['native-build'])
        self.assertIn('govulncheck ./...', JOBS['vulnerability'])
        self.assertIn('go-licenses report ./... --include_tests', JOBS['licenses'])

    def test_normal_ci_has_no_scheduled_arch_or_duplicate_experiment(self):
        self.assertNotIn('schedule:', WORKFLOW)
        self.assertNotIn('archlinux:', WORKFLOW)
        self.assertNotIn('workflow_run:', WORKFLOW)
        self.assertIn('pull_request:', WORKFLOW)
        self.assertIn('branches: [main]', WORKFLOW)
        self.assertFalse((ROOT / '.github/workflows/ci-nix.yml').exists())
        self.assertEqual(JOBS['fuzz'].count('-fuzztime=10s -parallel=1 -timeout=2m'), 7)
        for command in ('CGO_ENABLED:', 'go test -race -json -shuffle=on -count=1', 'go run ./internal/profiledecode/testdata/generate', 'git diff --exit-code'):
            self.assertIn(command, JOBS['generated-files'])
        self.assertIn("module-writer: 'true'", JOBS['native-build'])
        self.assertEqual(WORKFLOW.count("module-writer: 'true'"), 1)

    def test_build_cache_has_no_cross_environment_fallback(self):
        cache = (ROOT / '.github/actions/nix-go-cache/action.yml').read_text()
        self.assertNotIn('restore-keys:', cache)
        self.assertIn('azerlay-nix-build-v1-', cache)
        self.assertIn('.git/ci-native-cache-input', cache)
        self.assertIn('GOTOOLCHAIN', cache)

if __name__ == '__main__':
    unittest.main()
