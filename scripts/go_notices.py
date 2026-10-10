#!/usr/bin/env python3
"""Generate only the Go requirements table; retain reviewed legal prose.

Downloads are checksum-verified by Go. No dependency packages or generators run.
A changed legal-file inventory/text, unknown module, replacement, or gotk4
version requires updating the reviewed policy explicitly before generation.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
BEGIN = '<!-- BEGIN GENERATED GO MODULES -->'
END = '<!-- END GENERATED GO MODULES -->'
LEGAL_NAME = re.compile(r'(?:licen[cs]e|copying|notice|patents)(?:\..*)?', re.I)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def run_go(args, root):
    env = dict(os.environ, GOWORK='off', GOTOOLCHAIN='local',
               GOPROXY='https://proxy.golang.org', GOSUMDB='sum.golang.org',
               GOPRIVATE='', GONOPROXY='', GONOSUMDB='', GOFLAGS='')
    return json.loads(subprocess.check_output(['go', *args], cwd=root, env=env, text=True))


def requirements(root):
    # Work on copies: even Go's module-download bookkeeping cannot modify inputs.
    with tempfile.TemporaryDirectory() as directory:
        work = Path(directory)
        for name in ('go.mod', 'go.sum'):
            (work / name).write_bytes((root / name).read_bytes())
        module = run_go(['mod', 'edit', '-json'], work)
    if module.get('Replace') or module.get('Exclude'):
        raise ValueError('replace/exclude requires a corresponding-source review')
    return [(item['Path'], item['Version']) for item in module.get('Require', [])]


def verify_module(directory, entry, version):
    if entry.get('reviewed_version', version) != version:
        raise ValueError('gotk4 version requires a fresh nested/header license review')
    actual = {str(p.relative_to(directory)): digest(p)
              for p in sorted(directory.rglob('*'))
              if p.is_file() and LEGAL_NAME.fullmatch(p.name)}
    if actual != entry['upstream_legal_files']:
        raise ValueError('upstream legal files changed; review license text/inventory')
    for name, expected in entry.get('exception_files', {}).items():
        if digest(directory / name) != expected:
            raise ValueError('reviewed file-level license exception changed: ' + name)


def render(root, required, policy, download):
    if len(required) != len({path for path, _ in required}):
        raise ValueError('duplicate Go requirement')
    if {path for path, _ in required} != set(policy['modules']):
        raise ValueError('Go module inventory changed; review/add/remove policy entries')
    rows = [BEGIN, '| Component and version | Terms / notice file | Corresponding source |',
            '| --- | --- | --- |']
    for path, version in required:
        entry = policy['modules'][path]
        for name, expected in entry['local_files'].items():
            if digest(root / name) != expected:
                raise ValueError('reviewed local license text changed: ' + name)
        verify_module(download(path, version), entry, version)
        rows.append(f"| {path} {version} | {entry['terms']} | {entry['source'].format(version=version)} |")
    rows.append(END)
    notice = (root / 'THIRD_PARTY_NOTICES.md').read_text()
    if notice.count(BEGIN) != 1 or notice.count(END) != 1 or notice.index(BEGIN) > notice.index(END):
        raise ValueError('expected one generated Go modules section')
    before, rest = notice.split(BEGIN)
    _, after = rest.split(END)
    return before + '\n'.join(rows) + after


def generate(root):
    policy = json.loads((root / 'scripts/go-notice-policy.json').read_text())
    required = requirements(root)
    with tempfile.TemporaryDirectory() as directory:
        work = Path(directory)
        for name in ('go.mod', 'go.sum'):
            (work / name).write_bytes((root / name).read_bytes())

        def download(path, version):
            result = run_go(['mod', 'download', '-json', path + '@' + version], work)
            if result.get('Error') or not result.get('Sum'):
                raise ValueError('module download/checksum failed: ' + path)
            if result['Path'] != path or result['Version'] != version:
                raise ValueError('download did not match exact requirement: ' + path)
            return Path(result['Dir'])

        return render(root, required, policy, download)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--check', action='store_true', help='reject drift without writing')
    args = parser.parse_args()
    try:
        generated = generate(args.root)
        notice = args.root / 'THIRD_PARTY_NOTICES.md'
        if args.check:
            if notice.read_text() != generated:
                raise ValueError('Go notices are stale; run python3 scripts/go_notices.py')
        else:
            notice.write_text(generated)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, str(error) + '\n')


if __name__ == '__main__':
    main()
