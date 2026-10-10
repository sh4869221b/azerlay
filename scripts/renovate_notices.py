#!/usr/bin/env python3
"""Trusted-base-only Renovate notice preparation and SHA-guarded publication.

Never check out or execute PR code. prepare uses a read-only token; publish uses
an approved ephemeral write token, and never invokes Go or dependency code.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
BEGIN = '<!-- BEGIN GENERATED GO MODULES -->'
END = '<!-- END GENERATED GO MODULES -->'
AUTHOR_EMAIL = 'azerlay-notices@users.noreply.github.com'
ALLOWED = {'go.mod', 'go.sum', 'THIRD_PARTY_NOTICES.md'}
SHA = re.compile(r'[0-9a-f]{40}')
REF = re.compile(r'renovate/[A-Za-z0-9._/-]+')


def api(repo, suffix):
    request = Request('https://api.github.com/repos/' + repo + suffix, headers={
        'Authorization': 'Bearer ' + os.environ['GH_TOKEN'],
        'Accept': 'application/vnd.github+json', 'X-GitHub-Api-Version': '2022-11-28',
    })
    with urlopen(request, timeout=30) as response:
        return json.load(response)


def validate_pr(pr, repo, expected_head):
    if (repo != 'sh4869221b/azerlay' or pr['state'] != 'open'
            or pr['user']['login'] != 'renovate[bot]'
            or pr['head']['repo']['full_name'] != repo
            or pr['base']['repo']['full_name'] != repo
            or pr['base']['ref'] != 'main'
            or not REF.fullmatch(pr['head']['ref'])
            or '..' in pr['head']['ref'] or '@{' in pr['head']['ref']
            or not SHA.fullmatch(expected_head)
            or pr['head']['sha'] != expected_head
            or not SHA.fullmatch(pr['base']['sha'])):
        raise ValueError('not an open same-repository Renovate PR at the expected head')


def git(*args, input=None, env=None):
    return subprocess.check_output(['git', *args], cwd=ROOT, input=input, env=env)


def fetch(ref, token=None):
    env = dict(os.environ)
    if token:
        # Environment-only auth: never persist credentials or interpolate them
        # into a URL/command/log. Disable hooks for all publisher git operations.
        authorization = base64.b64encode(('x-access-token:' + token).encode()).decode()
        env.update(GIT_CONFIG_COUNT='2', GIT_CONFIG_KEY_0='http.https://github.com/.extraheader',
                   GIT_CONFIG_VALUE_0='AUTHORIZATION: basic ' + authorization,
                   GIT_CONFIG_KEY_1='core.hooksPath', GIT_CONFIG_VALUE_1='/dev/null')
    git('fetch', '--no-tags', '--depth=1', 'origin', ref, env=env)
    return git('rev-parse', 'FETCH_HEAD').decode().strip(), env


def blob(commit, name):
    return git('show', commit + ':' + name)


def outside_table(data):
    text = data.decode('utf-8')
    if text.count(BEGIN) != 1 or text.count(END) != 1 or text.index(BEGIN) > text.index(END):
        raise ValueError('expected one generated Go modules section')
    before, rest = text.split(BEGIN)
    _, after = rest.split(END)
    return before, after


def fingerprint(data):
    return hashlib.sha256(data).hexdigest()


def prepare(event, directory):
    repo = event['repository']['full_name']
    number = event['pull_request']['number']
    expected_head = event['pull_request']['head']['sha']
    pr = api(repo, '/pulls/' + str(number))
    validate_pr(pr, repo, expected_head)
    changed = set()
    for page in range(1, 12):
        files = api(repo, f'/pulls/{number}/files?per_page=100&page={page}')
        changed.update(item['filename'] for item in files)
        changed.update(item['previous_filename'] for item in files if 'previous_filename' in item)
        if len(files) < 100:
            break
    else:
        raise ValueError('PR file inventory exceeds synchronization limit')
    if not changed & {'go.mod', 'go.sum'}:
        return False  # GitHub Actions/tool updates need no application notice write.
    if changed - ALLOWED:
        raise ValueError('Go update changes code/policy/workflows; manual review required')
    head, _ = fetch(expected_head)
    validate_pr(api(repo, '/pulls/' + str(number)), repo, expected_head)
    if head != expected_head or git('rev-parse', 'HEAD').decode().strip() != pr['base']['sha']:
        raise ValueError('checkout/fetch does not match trusted base and final PR head')
    inputs = {name: blob(head, name) for name in ALLOWED}
    trusted_notice = (ROOT / 'THIRD_PARTY_NOTICES.md').read_bytes()
    if outside_table(inputs['THIRD_PARTY_NOTICES.md']) != outside_table(trusted_notice):
        raise ValueError('curated notice prose differs from trusted base; rebase/review first')
    directory.mkdir(parents=True, exist_ok=False)
    for name in ('go.mod', 'go.sum'):
        (directory / name).write_bytes(inputs[name])
    (directory / 'THIRD_PARTY_NOTICES.md').write_bytes(trusted_notice)
    shutil.copytree(ROOT / 'LICENSES', directory / 'LICENSES')
    (directory / 'scripts').mkdir()
    shutil.copyfile(ROOT / 'scripts/go-notice-policy.json', directory / 'scripts/go-notice-policy.json')
    manifest = {'repository': repo, 'number': number, 'head_sha': head,
                'base_sha': pr['base']['sha'], 'ref': pr['head']['ref'],
                'inputs': {name: fingerprint(data) for name, data in inputs.items()}}
    (directory / 'manifest.json').write_text(json.dumps(manifest, sort_keys=True) + '\n')
    return True


def validate_candidate(manifest, current, candidate, trusted_notice):
    for name, data in current.items():
        if fingerprint(data) != manifest['inputs'][name]:
            raise ValueError('input does not match prepared PR head: ' + name)
    if (outside_table(candidate) != outside_table(trusted_notice)
            or outside_table(current['THIRD_PARTY_NOTICES.md']) != outside_table(trusted_notice)):
        raise ValueError('candidate changes curated prose outside generated table')


def push_candidate(head, ref, candidate, env):
    # Build exactly one-file child commit without checking out PR source. An
    # explicit lease atomically guards the remote head; ancestry is verified
    # before publication, so the lease never authorizes a history rewrite.
    with tempfile.TemporaryDirectory() as temp:
        commit_env = dict(env, GIT_INDEX_FILE=str(Path(temp) / 'index'),
                          GIT_AUTHOR_NAME='Azerlay notices automation', GIT_AUTHOR_EMAIL=AUTHOR_EMAIL,
                          GIT_COMMITTER_NAME='Azerlay notices automation', GIT_COMMITTER_EMAIL=AUTHOR_EMAIL)
        git('read-tree', head, env=commit_env)
        sha = git('hash-object', '-w', '--stdin', input=candidate, env=commit_env).decode().strip()
        git('update-index', '--add', '--cacheinfo', '100644,' + sha + ',THIRD_PARTY_NOTICES.md', env=commit_env)
        tree = git('write-tree', env=commit_env).decode().strip()
        commit = git('commit-tree', tree, '-p', head, '-m', 'chore: synchronize Go dependency notices', env=commit_env).decode().strip()
        parent = git('rev-parse', commit + '^', env=commit_env).decode().strip()
        files = git('diff-tree', '--no-commit-id', '--name-only', '-r', commit, env=commit_env).decode().splitlines()
        if parent != head or files != ['THIRD_PARTY_NOTICES.md']:
            raise ValueError('publication must be exactly one-file child of expected head')
        git('push', '--force-with-lease=refs/heads/' + ref + ':' + head,
            'origin', commit + ':refs/heads/' + ref, env=commit_env)
        return commit


def publish(event, directory):
    repo = event['repository']['full_name']
    number = event['pull_request']['number']
    expected = event['pull_request']['head']['sha']
    manifest = json.loads((directory / 'manifest.json').read_text())
    pr = api(repo, '/pulls/' + str(number))
    validate_pr(pr, repo, expected)
    if (manifest['repository'] != repo or manifest['number'] != number
            or manifest['head_sha'] != expected or manifest['base_sha'] != pr['base']['sha']
            or manifest['ref'] != pr['head']['ref']
            or git('rev-parse', 'HEAD').decode().strip() != manifest['base_sha']):
        raise ValueError('artifact/event/trusted base does not match current PR')
    head, env = fetch('refs/heads/' + pr['head']['ref'], os.environ['GH_TOKEN'])
    if head != expected:
        raise ValueError('Renovate head advanced; do not publish stale output')
    current = {name: blob(head, name) for name in ALLOWED}
    candidate = (directory / 'THIRD_PARTY_NOTICES.md').read_bytes()
    validate_candidate(manifest, current, candidate, (ROOT / 'THIRD_PARTY_NOTICES.md').read_bytes())
    if candidate == current['THIRD_PARTY_NOTICES.md']:
        return None  # No commit, dispatch or recurring write when already synchronized.
    validate_pr(api(repo, '/pulls/' + str(number)), repo, expected)
    return push_candidate(head, pr['head']['ref'], candidate, env)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['prepare', 'publish'])
    parser.add_argument('--directory', required=True, type=Path)
    args = parser.parse_args()
    event = json.loads(Path(os.environ['GITHUB_EVENT_PATH']).read_text())
    if args.mode == 'prepare':
        eligible = prepare(event, args.directory)
        with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
            output.write('eligible=' + str(eligible).lower() + '\n')
    else:
        commit = publish(event, args.directory)
        with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
            output.write('changed=' + str(commit is not None).lower() + '\n')
            if commit:
                output.write('commit=' + commit + '\n')


if __name__ == '__main__':
    main()
