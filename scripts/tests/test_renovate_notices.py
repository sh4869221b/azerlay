"""Trusted publisher boundaries and atomic git behavior; no GitHub mutations."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('renovate_notices', ROOT / 'scripts/renovate_notices.py')
sync = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sync)
REPO = 'sh4869221b/azerlay'
HEAD = 'a' * 40
BASE = 'b' * 40


def pr():
    return {'state': 'open', 'user': {'login': 'renovate[bot]'},
            'head': {'sha': HEAD, 'ref': 'renovate/example-1.x', 'repo': {'full_name': REPO}},
            'base': {'sha': BASE, 'ref': 'main', 'repo': {'full_name': REPO}}}


def event():
    return {'repository': {'full_name': REPO}, 'pull_request': {'number': 158, 'head': {'sha': HEAD}}}


def notice(row):
    return ('trusted prose\n' + sync.BEGIN + '\n' + row + '\n' + sync.END + '\nhistorical review\n').encode()


class RenovateNoticeBoundaryTests(unittest.TestCase):
    def test_only_open_same_repo_renovate_head_is_accepted(self):
        sync.validate_pr(pr(), REPO, HEAD)
        for change in ('user', 'fork', 'closed', 'base', 'branch', 'stale', 'unsafe-ref'):
            value = pr()
            if change == 'user': value['user']['login'] = 'other-bot'
            if change == 'fork': value['head']['repo']['full_name'] = 'someone/azerlay'
            if change == 'closed': value['state'] = 'closed'
            if change == 'base': value['base']['ref'] = 'other'
            if change == 'branch': value['head']['ref'] = 'main'
            if change == 'stale': value['head']['sha'] = 'c' * 40
            if change == 'unsafe-ref': value['head']['ref'] = 'renovate/../main'
            with self.subTest(change=change), self.assertRaises(ValueError):
                sync.validate_pr(value, REPO, HEAD)

    def test_prepare_skips_tool_pr_and_rejects_code_or_renames(self):
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp) / 'input'
            with patch.object(sync, 'api', side_effect=[pr(), [{'filename': '.github/ci-tools.env'}]]):
                self.assertFalse(sync.prepare(event(), directory))
                self.assertFalse(directory.exists())
            for files in ([{'filename': 'go.mod'}, {'filename': 'scripts/go_notices.py'}],
                          [{'filename': 'go.mod', 'previous_filename': 'scripts/go_notices.py'}]):
                with patch.object(sync, 'api', side_effect=[pr(), files]):
                    with self.assertRaisesRegex(ValueError, 'code/policy/workflows'):
                        sync.prepare(event(), directory)
                self.assertFalse(directory.exists())

    def test_prepare_uses_trusted_policy_and_final_mod_as_data(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'base'
            root.mkdir()
            (root / 'scripts').mkdir()
            (root / 'LICENSES').mkdir()
            (root / 'scripts/go-notice-policy.json').write_text('{"trusted": true}')
            (root / 'LICENSES/example.txt').write_text('trusted text')
            (root / 'THIRD_PARTY_NOTICES.md').write_bytes(notice('base'))
            inputs = {'go.mod': b'final direct and indirect requirements', 'go.sum': b'final sums',
                      'THIRD_PARTY_NOTICES.md': notice('PR row')}
            directory = Path(temp) / 'input'
            with patch.object(sync, 'ROOT', root), patch.object(sync, 'api', side_effect=[pr(), [{'filename': 'go.mod'}], pr()]), \
                 patch.object(sync, 'fetch', return_value=(HEAD, {})), patch.object(sync, 'git', return_value=(BASE + '\n').encode()), \
                 patch.object(sync, 'blob', side_effect=lambda _, name: inputs[name]):
                self.assertTrue(sync.prepare(event(), directory))
            self.assertEqual((directory / 'go.mod').read_bytes(), inputs['go.mod'])
            self.assertEqual((directory / 'scripts/go-notice-policy.json').read_text(), '{"trusted": true}')
            self.assertEqual((directory / 'THIRD_PARTY_NOTICES.md').read_bytes(), notice('base'))
            self.assertEqual(json.loads((directory / 'manifest.json').read_text())['head_sha'], HEAD)

    def test_candidate_rejects_wrong_inputs_and_curated_prose(self):
        current = {'go.mod': b'final modules', 'go.sum': b'sums', 'THIRD_PARTY_NOTICES.md': notice('old')}
        manifest = {'inputs': {name: sync.fingerprint(data) for name, data in current.items()}}
        sync.validate_candidate(manifest, current, notice('new'), notice('base'))
        altered = dict(current, **{'go.mod': b'different modules'})
        with self.assertRaisesRegex(ValueError, 'input does not match'):
            sync.validate_candidate(manifest, altered, notice('new'), notice('base'))
        with self.assertRaisesRegex(ValueError, 'curated prose'):
            sync.validate_candidate(manifest, current, notice('new').replace(b'trusted prose', b'changed prose'), notice('base'))
        with self.assertRaisesRegex(ValueError, 'one generated'):
            sync.outside_table(notice('new') + notice('new'))

    def test_publish_rejects_foreign_artifact_stale_head_and_noop_does_not_write(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'base'; root.mkdir()
            directory = Path(temp) / 'artifact'; directory.mkdir()
            current = {'go.mod': b'final', 'go.sum': b'sums', 'THIRD_PARTY_NOTICES.md': notice('same')}
            (root / 'THIRD_PARTY_NOTICES.md').write_bytes(notice('base'))
            (directory / 'THIRD_PARTY_NOTICES.md').write_bytes(notice('same'))
            manifest = {'repository': REPO, 'number': 158, 'head_sha': HEAD, 'base_sha': BASE,
                        'ref': pr()['head']['ref'], 'inputs': {n: sync.fingerprint(d) for n, d in current.items()}}
            with patch.object(sync, 'ROOT', root), patch.dict(os.environ, GH_TOKEN='synthetic'), \
                 patch.object(sync, 'api', return_value=pr()), patch.object(sync, 'git', return_value=(BASE + '\n').encode()), \
                 patch.object(sync, 'blob', side_effect=lambda _, name: current[name]), patch.object(sync, 'push_candidate') as push:
                bad = dict(manifest, number=159)
                (directory / 'manifest.json').write_text(json.dumps(bad))
                with self.assertRaisesRegex(ValueError, 'artifact/event'):
                    sync.publish(event(), directory)
                (directory / 'manifest.json').write_text(json.dumps(manifest))
                with patch.object(sync, 'fetch', return_value=('c' * 40, {})):
                    with self.assertRaisesRegex(ValueError, 'head advanced'):
                        sync.publish(event(), directory)
                with patch.object(sync, 'fetch', return_value=(HEAD, {})):
                    self.assertIsNone(sync.publish(event(), directory))
                push.assert_not_called()
                (directory / 'THIRD_PARTY_NOTICES.md').write_bytes(notice('new'))
                push.return_value = 'c' * 40
                with patch.object(sync, 'fetch', return_value=(HEAD, {})):
                    self.assertEqual(sync.publish(event(), directory), 'c' * 40)
                push.assert_called_once_with(HEAD, pr()['head']['ref'], notice('new'), {})

    def test_real_git_push_changes_one_file_and_atomic_lease_rejects_race(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / 'local'; root.mkdir()
            remote = Path(temp) / 'remote.git'
            subprocess.run(['git', 'init', '--bare', str(remote)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            def git(*args, input=None):
                return subprocess.check_output(['git', *args], cwd=root, input=input, stderr=subprocess.PIPE)
            git('init', '-b', 'main')
            git('config', 'user.name', 'Synthetic test')
            git('config', 'user.email', 'test@example.invalid')
            (root / 'THIRD_PARTY_NOTICES.md').write_bytes(notice('old'))
            (root / 'protected.txt').write_text('protected')
            git('add', '.')
            git('commit', '-m', 'synthetic base')
            git('remote', 'add', 'origin', str(remote))
            ref = 'renovate/example'
            git('push', 'origin', 'HEAD:refs/heads/' + ref)
            head = git('rev-parse', 'HEAD').decode().strip()
            with patch.object(sync, 'ROOT', root):
                commit = sync.push_candidate(head, ref, notice('new'), dict(os.environ))
                self.assertEqual(git('rev-parse', commit + '^').decode().strip(), head)
                self.assertEqual(git('show', commit + ':protected.txt'), b'protected')
                self.assertEqual(git('log', '-1', '--format=%ae', commit).decode().strip(), sync.AUTHOR_EMAIL)
                with self.assertRaises(subprocess.CalledProcessError):
                    sync.push_candidate(head, ref, notice('stale competing output'), dict(os.environ))
                actual = subprocess.check_output(['git', '--git-dir=' + str(remote), 'rev-parse', 'refs/heads/' + ref]).decode().strip()
                self.assertEqual(actual, commit)

    def test_workflow_has_trusted_checkout_separate_tokens_and_dispatch(self):
        text = (ROOT / '.github/workflows/renovate-notices.yml').read_text()
        self.assertIn('pull_request_target:', text)
        self.assertEqual(text.count('ref: ${{ github.event.pull_request.base.sha }}'), 2)
        self.assertNotIn('ref: ${{ github.event.pull_request.head.sha }}', text)
        generate, publish = text.split('  publish:')
        self.assertNotIn('contents: write', generate)
        self.assertNotIn('actions: write', generate)
        self.assertIn('contents: write', publish)
        self.assertIn('actions: write', publish)
        self.assertIn('gh workflow run ci.yml', publish)
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        self.assertEqual(ci.count('Reject a moved dispatched branch'), 6)
        self.assertIn('workflow_dispatch:', ci)
        for trigger, expected, success in ((HEAD, HEAD, True), ('c' * 40, HEAD, False)):
            result = subprocess.run(['bash', '-c', 'test "$EXPECTED_HEAD" = "$TRIGGER_HEAD"'],
                                    env=dict(os.environ, EXPECTED_HEAD=expected, TRIGGER_HEAD=trigger))
            self.assertEqual(result.returncode == 0, success)


if __name__ == '__main__':
    unittest.main()
