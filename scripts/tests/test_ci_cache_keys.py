"""Exercise fingerprints and the hashFiles inputs of both composite actions."""
import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class CacheKeyTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.repo = Path(self.directory.name)
        for name in ('scripts', '.github', '.git', 'bin'):
            (self.repo / name).mkdir()
        for name in ('ci-nix-fingerprint.sh', 'ci-native-fingerprint.sh', 'ci-go-build-settings.sh',
                     'ci-tool-fingerprint.sh'):
            shutil.copyfile(ROOT / 'scripts' / name, self.repo / 'scripts' / name)
        shutil.copyfile(ROOT / '.github/ci-tools.env', self.repo / '.github/ci-tools.env')
        (self.repo / 'scripts/ci-staticcheck').mkdir()
        for name in ('go.mod', 'go.sum'):
            shutil.copyfile(ROOT / 'scripts/ci-staticcheck' / name, self.repo / 'scripts/ci-staticcheck' / name)
        self.env = dict(os.environ, PATH=str(self.repo / 'bin') + ':/usr/bin:/bin',
                        AZERLAY_NIX_CI='1', AZERLAY_NIX_BUILD_INPUTS='/nix/store/compiler\n/nix/store/gtk-dev')
        self.mock('go', "printf '%s\\n' \"${TEST_GO_SETTINGS:-go1.27.2 linux amd64 cgo=1}\"")
        self.mock('cc', 'echo "${TEST_CC:-compiler}"; exit "${TEST_CC_STATUS:-0}"')
        self.mock('ld', 'echo "${TEST_LD:-linker}"')
        self.mock('pkg-config', 'echo "${TEST_PKG:-native-flags}"; exit "${TEST_PKG_STATUS:-0}"')
        self.mock('python3', 'echo "${TEST_PACKAGES:-gcc 1 x86_64 gtk4 1 x86_64}"; exit "${TEST_PACKAGE_STATUS:-0}"')
        self.mock('cat', 'echo "${TEST_OS:-ID=arch}"')
        self.mock('uname', 'echo "${TEST_ARCH:-x86_64}"')

    def mock(self, command, body):
        path = self.repo / 'bin' / command
        path.write_text('#!/bin/sh\n' + body + '\n')
        path.chmod(0o755)

    def run_script(self, name, *args, **settings):
        return subprocess.run(['bash', 'scripts/' + name + '.sh', *args], cwd=self.repo,
                              env=dict(self.env, **settings), capture_output=True, text=True)

    def fingerprint(self, kind, **settings):
        result = self.run_script('ci-' + kind + '-fingerprint', **settings)
        self.assertEqual(result.returncode, 0, result.stderr)
        if kind == 'native':
            go = self.run_script('ci-go-build-settings', **settings)
            self.assertEqual(go.returncode, 0, go.stderr)
            return result.stdout + go.stdout
        return result.stdout

    def test_unrelated_repository_and_job_changes_preserve_both_fingerprints(self):
        for kind in ('nix', 'native'):
            original = self.fingerprint(kind)
            for name in ('.github/workflows/ci.yml', '.github/workflows/arch-ci.yml',
                         'flake.nix', 'flake.lock', 'scripts/ci-nix-session.py',
                         'scripts/test-wayland.sh', 'README.md'):
                path = self.repo / name
                path.parent.mkdir(exist_ok=True, parents=True)
                path.write_text('changed comments, uploads or runtime fixture\n')
            self.assertEqual(original, self.fingerprint(kind, CI_CACHE_JOB='renamed', CI_NIX_ROLE='test',
                                                       CI_NATIVE_WORKFLOW='renamed.yml', CI_NIX_OUTPUT='/other'))

    def test_effective_compatibility_changes_invalidate(self):
        for kind in ('nix', 'native'):
            original = self.fingerprint(kind)
            for setting in ('TEST_CC', 'TEST_LD', 'TEST_PKG', 'TEST_GO_SETTINGS',
                            'NIX_CC', 'NIX_CFLAGS_COMPILE', 'NIX_LDFLAGS', 'NIX_ENFORCE_PURITY'):
                with self.subTest(kind=kind, setting=setting):
                    self.assertNotEqual(original, self.fingerprint(kind, **{setting: 'changed'}))
        self.assertNotEqual(self.fingerprint('nix'), self.fingerprint('nix', AZERLAY_NIX_BUILD_INPUTS='/nix/store/new-gtk-dev'))
        for setting in ('TEST_PACKAGES', 'TEST_OS', 'TEST_ARCH'):
            self.assertNotEqual(self.fingerprint('native'), self.fingerprint('native', **{setting: 'changed'}))

    def test_nix_output_order_and_duplicates_do_not_invalidate(self):
        self.assertEqual(self.fingerprint('nix'), self.fingerprint('nix',
                         AZERLAY_NIX_BUILD_INPUTS='/nix/store/gtk-dev\n/nix/store/compiler\n/nix/store/compiler'))

    def test_missing_identity_and_failed_probes_abort(self):
        self.assertNotEqual(self.run_script('ci-nix-fingerprint', AZERLAY_NIX_BUILD_INPUTS='').returncode, 0)
        for kind in ('nix', 'native'):
            for setting in ('TEST_CC_STATUS', 'TEST_PKG_STATUS'):
                self.assertNotEqual(self.run_script('ci-' + kind + '-fingerprint', **{setting: '7'}).returncode, 0)
        self.assertNotEqual(self.run_script('ci-native-fingerprint', TEST_PACKAGE_STATUS='7').returncode, 0)

    def action_files(self, kind, cache):
        action = (ROOT / f'.github/actions/{kind}-go-cache/action.yml').read_text()
        prefix = {'modules': 'azerlay-mod-', 'build': 'azerlay-nix-build-' if kind == 'nix' else 'azerlay-build-',
                  'tools': 'azerlay-tool-mod-'}[cache]
        line = next(line for line in action.splitlines() if 'key: ' + prefix in line)
        return re.findall("'([^']+)'", line.split('hashFiles(', 1)[1])

    def key_digest(self, files):
        return hashlib.sha256(b''.join(name.encode() + b'\0' + (self.repo / name).read_bytes() for name in files)).digest()

    def test_action_keys_change_only_with_their_selected_graph(self):
        for kind in ('nix', 'native'):
            files = self.action_files(kind, 'build')
            self.assertEqual(files, ['go.mod', 'go.sum', '.git/ci-native-cache-input'])
            for name in files:
                (self.repo / name).write_text('original')
            original = self.key_digest(files)
            pins = self.repo / '.github/ci-tools.env'
            saved = pins.read_text()
            for setting in ('STATICCHECK_VERSION', 'STATICCHECK_X_TOOLS_VERSION', 'GOVULNCHECK_VERSION',
                            'GO_LICENSES_VERSION', 'SYFT_VERSION'):
                pins.write_text(saved + '\n# renovate: datasource=go\n' + setting + '=changed\n')
                self.assertEqual(original, self.key_digest(files))
            for name in files:
                (self.repo / name).write_text('changed')
                self.assertNotEqual(original, self.key_digest(files))
                (self.repo / name).write_text('original')
            self.assertEqual(self.action_files(kind, 'modules'), ['go.mod', 'go.sum'])
            self.assertEqual(self.action_files(kind, 'tools'), ['.git/ci-tool-cache-input'])

    def test_tool_pins_only_track_the_selected_analyzer(self):
        pins = self.repo / '.github/ci-tools.env'
        saved = pins.read_text()
        selections = {'staticcheck': ('STATICCHECK_VERSION', 'STATICCHECK_X_TOOLS_VERSION'),
                      'govulncheck': ('GOVULNCHECK_VERSION',), 'go-licenses': ('GO_LICENSES_VERSION',)}
        for tool, relevant in selections.items():
            pins.write_text(saved)
            original = self.run_script('ci-tool-fingerprint', tool).stdout
            for setting in ('STATICCHECK_VERSION', 'STATICCHECK_X_TOOLS_VERSION', 'GOVULNCHECK_VERSION',
                            'GO_LICENSES_VERSION', 'SYFT_VERSION'):
                pins.write_text(saved + '\n# documentation change\n' + setting + '=changed\n')
                result = self.run_script('ci-tool-fingerprint', tool)
                self.assertEqual(result.returncode, 0, result.stderr)
                with self.subTest(tool=tool, setting=setting):
                    if setting in relevant:
                        self.assertNotEqual(original, result.stdout)
                    else:
                        self.assertEqual(original, result.stdout)
        self.assertNotEqual(self.run_script('ci-tool-fingerprint', 'unknown').returncode, 0)

    def test_staticcheck_lock_changes_only_invalidate_staticcheck(self):
        for tool in ('staticcheck', 'govulncheck', 'go-licenses'):
            original = self.run_script('ci-tool-fingerprint', tool).stdout
            for name in ('go.mod', 'go.sum'):
                path = self.repo / 'scripts/ci-staticcheck' / name
                saved = path.read_text()
                path.write_text(saved + '# changed graph\n')
                changed = self.run_script('ci-tool-fingerprint', tool).stdout
                if tool == 'staticcheck':
                    self.assertNotEqual(original, changed)
                else:
                    self.assertEqual(original, changed)
                path.write_text(saved)

    @unittest.skipUnless(shutil.which('go'), 'Go SDK unavailable')
    def test_real_go_effective_settings_track_compatibility(self):
        environment = dict(os.environ, GOENV='off', GOTOOLCHAIN='local', GOOS='linux', GOARCH='amd64',
                           GOAMD64='v1', CGO_ENABLED='1', GOFLAGS='')
        def capture(**settings):
            result = subprocess.run(['bash', str(ROOT / 'scripts/ci-go-build-settings.sh')],
                                    cwd=ROOT, env=dict(environment, **settings),
                                    capture_output=True, text=True, check=True)
            return result.stdout
        original = capture()
        decoded, _ = json.JSONDecoder().raw_decode(original)
        self.assertEqual(decoded['GOOS'], 'linux')
        self.assertEqual(decoded['CGO_ENABLED'], '1')
        for setting, value in {'GOOS': 'windows', 'GOARCH': 'arm64', 'GOAMD64': 'v3',
                               'CGO_ENABLED': '0', 'CC': 'clang', 'CXX': 'clang++',
                               'CGO_CFLAGS': '-O0', 'CGO_CPPFLAGS': '-DTEST',
                               'CGO_CXXFLAGS': '-O0', 'CGO_FFLAGS': '-O0',
                               'CGO_LDFLAGS': '-lm', 'GOFLAGS': '-tags=ci_test',
                               'CPATH': '/different/include', 'PKG_CONFIG_SYSROOT_DIR': '/other'}.items():
            with self.subTest(setting=setting):
                self.assertNotEqual(original, capture(**{setting: value}))

    def test_wiring_has_exact_restore_and_effective_go_settings(self):
        for kind in ('nix', 'native'):
            action = (ROOT / f'.github/actions/{kind}-go-cache/action.yml').read_text()
            self.assertNotIn('restore-keys:', action)
            self.assertNotIn('${{ github.job }}', action)
            self.assertIn('test -s .git/ci-native-cache-input', action)
            self.assertIn('ci-tool-fingerprint.sh "$CI_CACHE_TOOL" > .git/ci-tool-cache-input', action)
            self.assertIn('${{ inputs.build-profile }}-', action)
            self.assertIn("default: 'normal'", action)
        for workflow in ('ci.yml', 'arch-ci.yml'):
            source = (ROOT / '.github/workflows' / workflow).read_text()
            test = source.split('  test:\n', 1)[1].split('\n  fuzz:', 1)[0]
            self.assertIn("build-profile: 'race'", test)
            self.assertEqual(source.count("build-profile: 'race'"), 1)
        native = (ROOT / '.github/actions/native-go-cache/action.yml').read_text()
        self.assertLess(native.index('uses: actions/setup-go'), native.index('ci-go-build-settings.sh'))
        self.assertLess(native.index('ci-go-build-settings.sh'), native.index('Restore native Go build cache'))
        flake = (ROOT / 'flake.nix').read_text()
        identity = flake.split('AZERLAY_NIX_BUILD_INPUTS =', 1)[1].split(';', 1)[0]
        self.assertIn('lib.getDev', identity)
        self.assertIn('lib.getLib', identity)
        self.assertIn('pkgs.stdenv.cc', identity)
        self.assertIn('++ libraries', identity)
        for irrelevant in ('fonts', 'tools', 'withGUI'):
            self.assertNotIn(irrelevant, identity)

    @unittest.skipUnless(shutil.which('go'), 'Go SDK unavailable')
    def test_go_reuses_normal_and_race_entries_and_rebuilds_changed_source(self):
        # A private real GOCACHE verifies content reuse and Go's mode separation.
        # No network or application/native dependencies are needed.
        project = self.repo / 'example'
        project.mkdir()
        (project / 'go.mod').write_text('module example.invalid/cache\n\ngo 1.20\n')
        source = project / 'main.go'
        source.write_text('package main\nfunc main() { println("before") }\n')
        environment = dict(os.environ, GOENV='off', GOTOOLCHAIN='local', GOFLAGS='',
                           GOCACHE=str(self.repo / 'go-cache'), CGO_ENABLED='1')

        def build(race, expected):
            binary = project / ('race' if race else 'normal')
            binary.unlink(missing_ok=True)
            command = ['go', 'build', '-buildvcs=false', '-p=2', '-x',
                       *(['-race'] if race else []), '-o', str(binary), '.']
            result = subprocess.run(command, cwd=project, env=environment,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            execution = subprocess.run([str(binary)], env=environment,
                                       capture_output=True, text=True, check=True)
            self.assertEqual(execution.stderr.strip(), expected)
            return result.stderr

        for race in (False, True):
            self.assertIn('/compile ', build(race, 'before'))
        for race in (False, True):
            self.assertNotIn('/compile ', build(race, 'before'))
        source.write_text('package main\nfunc main() { println("after") }\n')
        for race in (False, True):
            self.assertIn('/compile ', build(race, 'after'))


if __name__ == '__main__':
    unittest.main()
