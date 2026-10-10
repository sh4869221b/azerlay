"""Offline Go artifact regression; fixtures are synthetic, never legal evidence."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import zipfile

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('go_notices', ROOT / 'scripts/go_notices.py')
notices = importlib.util.module_from_spec(spec)
spec.loader.exec_module(notices)


@unittest.skipUnless(shutil.which('go'), 'offline Go artifact fixture requires Go')
class GoArtifactNoticesTests(unittest.TestCase):
    def test_final_go_mod_drives_direct_and_indirect_rows(self):
        for indirect in ('v1.1.0', 'v1.1.0-rc.1'):
            with self.subTest(indirect=indirect), tempfile.TemporaryDirectory() as temporary:
                self.check_artifacts(Path(temporary), indirect)

    def check_artifacts(self, root, indirect):
        proxy = root / 'proxy'
        upstream = root / 'upstream'
        upstream.mkdir()
        license_text = 'synthetic license; not legal evidence\n'
        (upstream / 'LICENSE').write_text(license_text)

        def module(name, version, requirement=''):
            directory = proxy / name / '@v'
            directory.mkdir(parents=True, exist_ok=True)
            mod = f'module {name}\n\ngo 1.20\n{requirement}'
            (directory / (version + '.mod')).write_text(mod)
            (directory / (version + '.info')).write_text(json.dumps({
                'Version': version, 'Time': '2026-10-10T00:00:00Z'}))
            package = name.rsplit('/', 1)[1]
            source = 'package ' + package + '\n'
            if requirement:
                source += 'import _ "example.test/b"\n'
            with zipfile.ZipFile(directory / (version + '.zip'), 'w') as archive:
                for relative, content in [('go.mod', mod), (package + '.go', source), ('LICENSE', license_text)]:
                    archive.writestr(name + '@' + version + '/' + relative, content)
            (directory / 'list').write_text('\n'.join(sorted(p.name[:-5] for p in directory.glob('*.info'))) + '\n')

        module('example.test/b', 'v1.0.0')
        module('example.test/b', indirect)
        module('example.test/a', 'v1.0.0', '\nrequire example.test/b v1.0.0\n')
        module('example.test/a', 'v1.1.0', '\nrequire example.test/b ' + indirect + '\n')
        work = root / 'project'
        work.mkdir()
        (work / 'go.mod').write_text('module example.test/app\n\ngo 1.20\n\nrequire example.test/a v1.0.0\nrequire example.test/b v1.0.0 // indirect\n')
        (work / 'main.go').write_text('package main\nimport _ "example.test/a"\nfunc main() {}\n')
        # Disabling sumdb is exclusive to this isolated file:// synthetic proxy.
        # Production generation retains its public checksum verification.
        env = dict(os.environ, GOPROXY=proxy.as_uri(), GOSUMDB='off', GOWORK='off', GOTOOLCHAIN='local',
                   GOPRIVATE='', GONOSUMDB='', GONOPROXY='', GOFLAGS='',
                   GOMODCACHE=str(root / 'gomod'), GOCACHE=str(root / 'gocache'))

        def go(args, directory=work):
            return subprocess.check_output(['go', *args], cwd=directory, env=env, stderr=subprocess.PIPE, text=True)

        go(['mod', 'download', 'all'])
        (work / 'LICENSES').mkdir()
        (work / 'scripts').mkdir()
        modules = {}
        for name in ('a', 'b'):
            relative = 'LICENSES/' + name + '.txt'
            (work / relative).write_text(license_text)
            modules['example.test/' + name] = {
                'terms': 'MIT; `' + relative + '`',
                'source': 'https://example.invalid/' + name + '/tree/{version}',
                'upstream_legal_files': {'LICENSE': notices.digest(upstream / 'LICENSE')},
                'local_files': {relative: notices.digest(work / relative)},
            }
        (work / 'scripts/go-notice-policy.json').write_text(json.dumps({'modules': modules}))
        before = 'synthetic curated notices\n'
        after = '\nsynthetic preserved historical review\n'
        notice = work / 'THIRD_PARTY_NOTICES.md'
        notice.write_text(before + notices.BEGIN + '\nstale\n' + notices.END + after)
        with patch.object(notices, 'run_go', side_effect=lambda args, directory: json.loads(go(args, directory))):
            notice.write_text(notices.generate(work))
            go(['mod', 'edit', '-require=example.test/a@v1.1.0'])
            go(['get', '-d', '-t', './...'])
            required = {r['Path']: r['Version'] for r in json.loads(go(['mod', 'edit', '-json']))['Require']}
            self.assertEqual(required, {'example.test/a': 'v1.1.0', 'example.test/b': indirect})
            inputs = {name: (work / name).read_bytes() for name in ('go.mod', 'go.sum')}
            # The old per-dependency replacement missed the side-effect update.
            generated = notices.generate(work)
            self.assertNotEqual(generated, notice.read_text())
            self.assertIn('| example.test/a v1.1.0 |', generated)
            self.assertIn('| example.test/b ' + indirect + ' |', generated)
            self.assertIn('https://example.invalid/b/tree/' + indirect, generated)
            self.assertTrue(generated.startswith(before))
            self.assertTrue(generated.endswith(after))
            notice.write_text(generated)
            self.assertEqual(notices.generate(work), generated)
            self.assertEqual(inputs, {name: (work / name).read_bytes() for name in inputs})


if __name__ == '__main__':
    unittest.main()
