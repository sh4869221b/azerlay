"""Fail-closed guards for the temporary, isolated Staticcheck source pin."""
import os
import re
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class StaticcheckInstallTests(unittest.TestCase):
    def run_installer(self, **changes):
        with tempfile.TemporaryDirectory(prefix='staticcheck test ') as temp:
            root = Path(temp)
            for name in ('scripts/ci-install-staticcheck.sh', '.github/ci-tools.env',
                         'scripts/ci-staticcheck/go.mod', 'scripts/ci-staticcheck/go.sum'):
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / name, target)
            binaries = root / 'bin'
            binaries.mkdir()
            go = binaries / 'go'
            go.write_text('''#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "$GO_COMMAND_LOG"
case "$*" in
  'env GOVERSION') printf '%s\\n' "$TEST_GO_VERSION" ;;
  'env GOTOOLCHAIN') printf '%s\\n' "${GOTOOLCHAIN:-auto}" ;;
  *'{{.Replace.Path}}@{{.Replace.Version}}'*) printf '%s\\n' "$TEST_SOURCE" ;;
  *'{{.Version}}'*) printf '%s\\n' "$TEST_IMPORTER" ;;
  'install -mod=readonly honnef.co/go/tools/cmd/staticcheck')
    if [[ "${TEST_MUTATE:-false}" = true ]]; then printf '\\nchanged\\n' >> go.mod; fi ;;
  'mod verify') exit "${TEST_VERIFY_STATUS:-0}" ;;
  *) exit 99 ;;
esac
''')
            go.chmod(0o755)
            pins = dict(line.split('=', 1) for line in (ROOT / '.github/ci-tools.env').read_text().splitlines()
                        if line and not line.startswith('#'))
            env = dict(os.environ, PATH=str(binaries) + ':' + os.environ['PATH'],
                       GO_COMMAND_LOG=str(root / 'commands'), TEST_GO_VERSION='go1.27.2',
                       GOTOOLCHAIN='local',
                       TEST_SOURCE='github.com/stefanb/go-tools@' + pins['STATICCHECK_VERSION'],
                       TEST_IMPORTER=pins['STATICCHECK_X_TOOLS_VERSION'])
            env.update(changes)
            result = subprocess.run(['bash', str(root / 'scripts/ci-install-staticcheck.sh')],
                                    env=env, capture_output=True, text=True)
            return result, (root / 'commands').read_text().splitlines()

    def test_readonly_install_and_module_verification(self):
        result, commands = self.run_installer()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(commands[-2:], ['install -mod=readonly honnef.co/go/tools/cmd/staticcheck',
                                        'mod verify'])

    def test_wrong_toolchain_source_or_importer_stops_before_install(self):
        for change in ({'TEST_GO_VERSION': 'go1.27.1'}, {'GOTOOLCHAIN': 'auto'},
                       {'TEST_SOURCE': 'github.com/other/go-tools@latest'},
                       {'TEST_SOURCE': 'github.com/stefanb/go-tools@latest'},
                       {'TEST_IMPORTER': 'v0.44.1'}):
            with self.subTest(change=change):
                result, commands = self.run_installer(**change)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(command.startswith('install ') for command in commands))

    def test_arch_and_nix_invocations_select_local_from_default_auto(self):
        arch = (ROOT / '.github/workflows/arch-ci.yml').read_text()
        install = arch.split('      - name: Install Staticcheck\n', 1)[1].split('      - name:', 1)[0]
        setting = re.search(r'^          GOTOOLCHAIN: (\S+)$', install, re.M)
        arch_toolchain = setting.group(1) if setting else 'auto'
        self.assertEqual(arch_toolchain, 'local')

        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        install = workflow.split('      - name: Install Staticcheck\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('--command bash scripts/ci-nix-run.sh', install)
        wrapper = (ROOT / 'scripts/ci-nix-run.sh').read_text()
        export = re.search(r'^export GOTOOLCHAIN=.*$', wrapper, re.M).group(0)
        environment = subprocess.run(
            ['bash', '-c', export + '\nprintf %s "$GOTOOLCHAIN"'],
            env=dict(os.environ, GOTOOLCHAIN='auto'), capture_output=True, text=True, check=True)
        self.assertEqual(environment.stdout, 'local')
        for toolchain in (arch_toolchain, environment.stdout):
            result, commands = self.run_installer(GOTOOLCHAIN=toolchain)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('install -mod=readonly honnef.co/go/tools/cmd/staticcheck', commands)

    def test_checksum_failure_propagates(self):
        result, _ = self.run_installer(TEST_VERIFY_STATUS='8')
        self.assertEqual(result.returncode, 8)

    def test_install_cannot_rewrite_committed_tool_graph(self):
        result, _ = self.run_installer(TEST_MUTATE='true')
        self.assertNotEqual(result.returncode, 0)

    def test_both_workflows_and_caches_use_the_same_tool_lock(self):
        for name in ('ci.yml', 'arch-ci.yml'):
            workflow = (ROOT / '.github/workflows' / name).read_text()
            self.assertIn('bash scripts/ci-install-staticcheck.sh', workflow)
            self.assertIn('staticcheck ./...', workflow)
            self.assertNotIn('go install honnef.co/go/tools/cmd/staticcheck@', workflow)
        for name in ('native-go-cache', 'nix-go-cache'):
            cache = (ROOT / '.github/actions' / name / 'action.yml').read_text()
            self.assertIn("hashFiles('.git/ci-tool-cache-input')", cache)
            self.assertIn('ci-tool-fingerprint.sh "$CI_CACHE_TOOL"', cache)
        fingerprint = (ROOT / 'scripts/ci-tool-fingerprint.sh').read_text()
        self.assertIn('sha256sum scripts/ci-staticcheck/go.mod scripts/ci-staticcheck/go.sum', fingerprint)
