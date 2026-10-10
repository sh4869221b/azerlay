"""Fail-closed generation tests; synthetic files are not legal evidence."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('go_notices', ROOT / 'scripts/go_notices.py')
notices = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notices)


class GoNoticesTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.module = self.root / 'upstream'
        self.module.mkdir()
        (self.module / 'LICENSE').write_text('synthetic license\n')
        (self.root / 'LICENSES').mkdir()
        (self.root / 'LICENSES/example.txt').write_text('synthetic license\n')
        self.entry = {
            'terms': 'MIT; `LICENSES/example.txt`',
            'source': 'https://example.invalid/tree/{version}',
            'upstream_legal_files': {'LICENSE': notices.digest(self.module / 'LICENSE')},
            'local_files': {'LICENSES/example.txt': notices.digest(self.root / 'LICENSES/example.txt')},
        }
        self.policy = {'modules': {'example.invalid/module': self.entry}}
        self.original = ('historical review\n' + notices.BEGIN + '\nstale\n' + notices.END
                         + '\nmanual native / nested / provenance notices\n')
        (self.root / 'THIRD_PARTY_NOTICES.md').write_text(self.original)

    def render(self, version='v1.2.3'):
        return notices.render(self.root, [('example.invalid/module', version)],
                              self.policy, lambda path, version: self.module)

    def test_updates_version_source_and_is_idempotent(self):
        for version in ('v0.5.17', 'v3.14.0', 'v1.2.4-0.20261010101718-abcdefabcdef'):
            generated = self.render(version)
            self.assertIn('example.invalid/module ' + version, generated)
            self.assertIn('/tree/' + version, generated)
            self.assertTrue(generated.startswith('historical review\n'))
            self.assertTrue(generated.endswith('\nmanual native / nested / provenance notices\n'))
            (self.root / 'THIRD_PARTY_NOTICES.md').write_text(generated)
            self.assertEqual(self.render(version), generated)

    def test_changed_upstream_text_fails_without_writing(self):
        (self.module / 'LICENSE').write_text('changed terms\n')
        with self.assertRaisesRegex(ValueError, 'upstream legal files changed'):
            self.render()
        self.assertEqual((self.root / 'THIRD_PARTY_NOTICES.md').read_text(), self.original)

    def test_new_and_removed_upstream_notices_fail(self):
        nested = self.module / 'nested'
        nested.mkdir()
        (nested / 'NOTICE.txt').write_text('new attribution\n')
        with self.assertRaisesRegex(ValueError, 'upstream legal files changed'):
            self.render()
        (nested / 'NOTICE.txt').unlink()
        (self.module / 'LICENSE').unlink()
        with self.assertRaisesRegex(ValueError, 'upstream legal files changed'):
            self.render()

    def test_changed_local_text_fails(self):
        (self.root / 'LICENSES/example.txt').write_text('changed local license\n')
        with self.assertRaisesRegex(ValueError, 'local license text changed'):
            self.render()

    def test_new_removed_and_duplicate_modules_fail(self):
        for required in ([], [('example.invalid/new', 'v1.0.0')],
                         [('example.invalid/module', 'v1.0.0')] * 2):
            with self.subTest(required=required), self.assertRaises(ValueError):
                notices.render(self.root, required, self.policy, lambda *_: self.module)

    def test_gotk4_version_and_exception_changes_require_review(self):
        self.entry['reviewed_version'] = 'v1.2.3'
        with self.assertRaisesRegex(ValueError, 'fresh nested/header license review'):
            self.render('v1.2.4')
        header = self.module / 'binding.go'
        header.write_text('synthetic copyright\n')
        self.entry['exception_files'] = {'binding.go': notices.digest(header)}
        header.write_text('changed header\n')
        with self.assertRaisesRegex(ValueError, 'file-level license exception changed'):
            self.render()

    def test_missing_duplicate_reversed_markers_fail(self):
        for text in ('no markers', self.original * 2, notices.END + notices.BEGIN):
            (self.root / 'THIRD_PARTY_NOTICES.md').write_text(text)
            with self.assertRaisesRegex(ValueError, 'one generated'):
                self.render()

    def test_replace_and_exclude_fail_and_inputs_stay_identical(self):
        for name in ('go.mod', 'go.sum'):
            (self.root / name).write_text('synthetic unchanged input\n')
        for field in ('Replace', 'Exclude'):
            with patch.object(notices, 'run_go', return_value={field: [{'Path': 'example'}]}):
                with self.assertRaisesRegex(ValueError, 'corresponding-source review'):
                    notices.requirements(self.root)
        for name in ('go.mod', 'go.sum'):
            self.assertEqual((self.root / name).read_text(), 'synthetic unchanged input\n')

    def test_both_ci_suites_check_real_upstream_notices(self):
        for workflow in ('ci.yml', 'arch-ci.yml'):
            text = (ROOT / '.github/workflows' / workflow).read_text()
            self.assertIn('python3 scripts/go_notices.py --check', text.split('  generated-files:')[1])

    def test_policy_matches_checked_in_main_requirement_inventory(self):
        policy = json.loads((ROOT / 'scripts/go-notice-policy.json').read_text())
        for path, entry in policy['modules'].items():
            self.assertIn('| ' + path + ' v', (ROOT / 'THIRD_PARTY_NOTICES.md').read_text())
            for relative, expected in entry['local_files'].items():
                self.assertEqual(notices.digest(ROOT / relative), expected)


if __name__ == '__main__':
    unittest.main()
