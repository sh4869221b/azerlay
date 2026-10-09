import base64
import copy
import importlib.util
import json
import os
import re
import subprocess
import tempfile
import unittest
from unittest import mock
from pathlib import Path

spec = importlib.util.spec_from_file_location('ci_nix_prepare', Path(__file__).parents[1] / 'ci_nix_prepare.py')
nix = importlib.util.module_from_spec(spec)
spec.loader.exec_module(nix)


def lock_fixture():
    nodes = {'root': {'inputs': {name: name for name in nix.PINNED_INPUTS}}}
    for name, (revision, nar_hash) in nix.PINNED_INPUTS.items():
        source = {'type': 'github', 'owner': 'NixOS', 'repo': 'nixpkgs', 'rev': revision}
        nodes[name] = {'original': dict(source), 'locked': {
            **source, 'narHash': nar_hash, 'lastModified': 1}}
    return {'version': 7, 'root': 'root', 'nodes': nodes}


def derivation_show_fixture():
    # Actual outer/inner shape emitted by Nix 2.35.2 derivation-show.cc and
    # libstore/derivations.cc; package hashes are synthetic privacy-safe values.
    return {'version': 4, 'derivations': {'1' * 32 + '-azerlay-ci-test.drv': {
        'name': 'azerlay-ci-test', 'version': 4,
        'outputs': {'out': {'path': '2' * 32 + '-azerlay-ci-test'}},
        'system': 'x86_64-linux', 'builder': '/nix/store/' + '3' * 32 + '-bash/bin/bash',
        'args': ['-e', '/nix/store/' + '4' * 32 + '-source-stdenv.sh'],
        'env': {'name': 'azerlay-ci-test'},
        'inputs': {'srcs': [], 'drvs': {'0' * 32 + '-gtk4.drv': ['out', 'dev']}},
    }}}


class NixPreparationTests(unittest.TestCase):
    def test_expected_lock_shape(self):
        self.assertTrue(nix.validate_lock(lock_fixture()).startswith('sha256-'))

    def test_revision_source_extra_inputs_and_hash_rejected(self):
        for name in nix.PINNED_INPUTS:
            mutations = [
                lambda x: x['nodes'][name]['locked'].update(rev='main'),
                lambda x: x['nodes'][name]['original'].update(owner='other'),
                lambda x: x['nodes'].update(extra={}),
                lambda x: x['nodes'][name]['locked'].update(narHash='sha256-fake'),
                lambda x: x['nodes'][name]['locked'].update(narHash='sha256-' + base64.b64encode(b'x' * 32).decode()),
                lambda x: x['nodes'][name]['original'].update(dir='redirected'),
                lambda x: x['nodes'][name]['locked'].update(host='other.example'),
                lambda x: x['nodes'].update({name: None}),
                lambda x: x['nodes']['root']['inputs'].update({name: 'other'}),
                lambda x: x['nodes']['root']['inputs'].pop(name),
                lambda x: x.update(version=8),
            ]
            for mutate in mutations:
                with self.subTest(name=name, mutate=mutate):
                    lock = copy.deepcopy(lock_fixture())
                    mutate(lock)
                    with self.assertRaises(ValueError):
                        nix.validate_lock(lock)

    def test_old_and_new_derivation_formats(self):
        path = '0' * 32 + '-gtk4.drv'
        expected = ['/nix/store/' + path + '^dev,out']
        self.assertEqual(nix.dependency_installables({'shell': {'inputDrvs': {'/nix/store/' + path: ['out', 'dev']}}}), expected)
        self.assertEqual(nix.dependency_installables({'shell': {'version': 4, 'inputs': {'drvs': {path: {'outputs': ['dev', 'out'], 'dynamicOutputs': {}}}}}}), expected)
        self.assertEqual(nix.dependency_installables({'shell': {'version': 4, 'inputs': {'drvs': {path: ['dev', 'out']}}}}), expected)

    def test_bad_dependency_graph_rejected(self):
        path = '0' * 32 + '-gtk4.drv'
        invalid = [None, [], {}, {'x': {}, 'y': {}}, {'x': None}, {'x': {'inputs': None}},
                   {'x': {'version': 5, 'inputDrvs': {path: ['out']}}}, {'x': {'inputDrvs': {}}},
                   {'x': {'inputDrvs': {'/tmp/gtk.drv': ['out']}}},
                   {'x': {'inputDrvs': {path: ['out;exec']}}},
                   {'x': {'inputs': {'drvs': {path: {'outputs': ['out'], 'dynamicOutputs': {'x': {}}}}}}}]
        for graph in invalid:
            with self.subTest(graph=graph), self.assertRaises(ValueError):
                nix.dependency_installables(graph)

    def test_pinned_nix_235_versioned_envelope(self):
        self.assertEqual(nix.dependency_installables(derivation_show_fixture()),
                         ['/nix/store/' + '0' * 32 + '-gtk4.drv^dev,out'])

    def test_unknown_or_malformed_envelope_is_rejected(self):
        invalid = [
            {'version': 5, 'derivations': derivation_show_fixture()['derivations']},
            {'version': 4}, {'derivations': {}},
            {'version': 4, 'derivations': None},
            {'version': 4, 'derivations': []},
            {'version': 4, 'derivations': {}},
            {**derivation_show_fixture(), 'extra': {}},
        ]
        for graph in invalid:
            with self.subTest(graph=graph), self.assertRaises(ValueError):
                nix.dependency_installables(graph)

    def test_envelope_still_requires_exactly_one_derivation(self):
        graph = derivation_show_fixture()
        graph['derivations']['5' * 32 + '-extra.drv'] = copy.deepcopy(next(iter(graph['derivations'].values())))
        with self.assertRaisesRegex(ValueError, 'exactly one'):
            nix.dependency_installables(graph)

    def test_flake_pins_toolchain_and_does_not_change_trust(self):
        root = Path(__file__).parents[2]
        flake = (root / 'flake.nix').read_text()
        self.assertIn('github:NixOS/nixpkgs/' + nix.REVISION, flake)
        self.assertIn('github:NixOS/nixpkgs/' + nix.GO_REVISION, flake)
        self.assertIn('go = (import nixpkgs-go { inherit system; }).go_1_27;', flake)
        self.assertIn('pkgs = import nixpkgs { inherit system; };', flake)
        self.assertIn('go = go.version;', flake)
        self.assertNotIn('pkgs.go_1_27', flake)
        self.assertIn('GOTOOLCHAIN = "local"', flake)
        self.assertNotIn('nixConfig', flake)
        self.assertNotIn('trusted-public-keys', flake)
        self.assertNotIn('shellHook =', flake)

    def test_ci_go_versions_and_cache_keys_match(self):
        root = Path(__file__).parents[2]
        version = nix.EXPECTED_VERSIONS['go']
        self.assertEqual(version, '1.27.2')
        for name in ('ci.yml', 'arch-ci.yml'):
            workflow = (root / '.github/workflows' / name).read_text()
            self.assertEqual(set(re.findall(r'go-version: (\S+)', workflow)), {version})
        native = (root / '.github/actions/native-go-cache/action.yml').read_text()
        self.assertIn('go-version: ' + version, native)
        cache = (root / '.github/actions/nix-go-cache/action.yml').read_text()
        self.assertIn('= go' + version, cache)
        keys = [line for line in cache.splitlines() if 'key: ' in line]
        self.assertEqual(len(keys), 4)
        for key in keys:
            if 'cache-primary-key' not in key:
                self.assertIn('-go-' + version + '-', key)

    def test_previous_go_version_is_rejected_before_dependency_fetch(self):
        versions = {**nix.EXPECTED_VERSIONS, 'go': '1.27.1'}
        commands = self.run_preparation(versions=versions)
        self.assertFalse(any(c[1] == 'build' for c in commands))

    def test_committed_lock_has_valid_fixed_identity(self):
        import json
        root = Path(__file__).parents[2]
        self.assertEqual(nix.validate_lock(json.loads((root / 'flake.lock').read_text())),
                         'sha256-Ni0LBydzaCi8oek12r4mVreuFHqYlM1eTD6SfiENKfw=')

    def test_installer_does_not_enable_extra_trust_or_token_access(self):
        source = (Path(__file__).parents[1] / 'ci-nix-install.sh').read_text()
        self.assertIn('0c3960a9792331a22081c3c7a5d8465db9b17c50b3acdf18587fa4c6f2cb1158', source)
        self.assertIn('https://releases.nixos.org/nix/nix-2.35.2/', source)
        self.assertNotIn('GITHUB_TOKEN', source)
        self.assertNotIn('trusted-users =', source)
        self.assertNotIn('trusted-public-keys =', source)
        self.assertNotIn('enable_kvm', source)
        self.assertNotIn('require-sigs = false', source)

    def test_dependency_build_is_substitution_only(self):
        source = (Path(__file__).parents[1] / 'ci_nix_prepare.py').read_text()
        self.assertIn('"--max-jobs", "0", "--builders", ""', source)
        self.assertNotIn('cachix', source)

    def run_preparation(self, fail_substitution=False, versions=None):
        commands = []
        def run(command, **kwargs):
            commands.append(command)
            if command == ['nix', '--version']:
                stdout = 'nix (Nix) 2.35.2\n'
            elif command[1] == 'eval':
                stdout = json.dumps(versions if versions is not None else nix.EXPECTED_VERSIONS)
            elif command[1:3] == ['derivation', 'show']:
                stdout = json.dumps(derivation_show_fixture())
            elif '--max-jobs' in command:
                if fail_substitution:
                    raise subprocess.CalledProcessError(1, command)
                stdout = ''
            elif command[1] == 'build':
                stdout = json.dumps([{'outputs': {'out': '/nix/store/' + '1' * 32 + '-shell'}}])
            elif '--json' in command:
                stdout = '{}'
            else:
                stdout = '/nix/store/' + '1' * 32 + '-shell\n'
            return subprocess.CompletedProcess(command, 0, stdout)
        with tempfile.TemporaryDirectory() as temp:
            old = Path.cwd()
            try:
                os.chdir(temp)
                Path('flake.lock').write_text(json.dumps(lock_fixture()))
                with mock.patch.object(nix.subprocess, 'run', side_effect=run), mock.patch(
                        'sys.argv', ['ci_nix_prepare.py', '--role', 'test', '--output', temp + '/output']):
                    if fail_substitution:
                        with self.assertRaises(subprocess.CalledProcessError):
                            nix.main()
                    elif versions is not None:
                        with self.assertRaisesRegex(ValueError, 'versions changed'):
                            nix.main()
                    else:
                        nix.main()
                        self.assertTrue(Path('output/closure.json').is_file())
                        self.assertTrue(Path('output/lock-sha256.txt').is_file())
            finally:
                os.chdir(old)
        return commands

    def test_prepare_substitutes_before_building_project_shell(self):
        commands = self.run_preparation()
        builds = [c for c in commands if c[1] == 'build']
        self.assertEqual(len(builds), 2)
        self.assertIn('--max-jobs', builds[0])
        self.assertEqual(builds[0][builds[0].index('--max-jobs') + 1], '0')
        self.assertEqual(builds[0][builds[0].index('--builders') + 1], '')
        self.assertEqual(builds[1][-1], nix.FLAKES['test'])
        for command in commands:
            if any(part in command for part in ('eval', 'derivation')) or command == builds[1]:
                self.assertIn('--no-update-lock-file', command)
                self.assertIn('--no-allow-import-from-derivation', command)

    def test_substitution_failure_never_falls_back_to_native_build(self):
        commands = self.run_preparation(fail_substitution=True)
        self.assertEqual(len([c for c in commands if c[1] == 'build']), 1)

    def test_unexpected_versions_fail_before_dependency_fetch(self):
        commands = self.run_preparation(versions={'go': '1.26.9'})
        self.assertFalse(any(c[1] == 'build' for c in commands))


if __name__ == '__main__':
    unittest.main()
