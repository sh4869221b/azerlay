"""Explicit migration policy for active workflows, independent of gate behavior.

Only the current main and reviewed Renovate destinations are admitted. Future
majors, mutable branches, third-party actions and local paths need review.
Archived measurement workflows keep their original version contracts.
"""

import re


ACTION_VERSIONS = {
    "actions/checkout": {"v4", "v7"},
    "actions/github-script": {"v7", "v9"},
    "actions/setup-go": {"v5", "v7"},
    "actions/upload-artifact": {"v4", "v7"},
    "actions/download-artifact": {"v4", "v8"},
    "actions/cache": {"v6"},
    "actions/cache/restore": {"v6"},
    "actions/cache/save": {"v6"},
}
HOSTED_RUNNERS = {"ubuntu-24.04", "ubuntu-26.04"}


def allowed_actions(*names):
    return {f"{name}@{version}" for name in names for version in ACTION_VERSIONS[name]}


def action_refs(source):
    values = re.findall(r"(?m)^ +(?:- )?uses:[ \t]*(.*)$", source)
    return [re.split(r"[ \t]+#", value, maxsplit=1)[0].strip() for value in values]


def require_action(source, name, count=1):
    refs = [ref for ref in action_refs(source) if ref.partition("@")[0] == name]
    if len(refs) != count or not set(refs) <= allowed_actions(name):
        raise AssertionError(f"Expected {count} approved {name} reference(s), got {refs}")
    return refs[0]


def require_hosted_runner(job):
    runners = re.findall(r"(?m)^    runs-on: (.+)$", job)
    if len(runners) != 1 or runners[0] not in HOSTED_RUNNERS:
        raise AssertionError(f"Expected one approved fixed Ubuntu runner, got {runners}")
    return runners[0]


def require_checkout_boundary(step):
    require_action(step, "actions/checkout")
    if not re.search(r"(?m)^ +persist-credentials: false$", step):
        raise AssertionError("Checkout must not persist credentials")
    if re.search(r"(?m)^ +(?:allow-unsafe-pr-checkout|token|ssh-key):", step):
        raise AssertionError("Checkout must not opt into unsafe PR code or extra credentials")


def require_artifact_transfer(step):
    if any(ref.startswith("actions/upload-artifact@") for ref in action_refs(step)):
        require_action(step, "actions/upload-artifact")
        # v7 archive:false changes artifact names and accepts only a single file.
        if re.search(r"(?m)^ +archive:", step):
            raise AssertionError("Keep the default ZIP artifact contract")
        if not re.search(r"(?m)^ +if-no-files-found: error$", step):
            raise AssertionError("Missing artifact files must fail")
    else:
        require_action(step, "actions/download-artifact")
        # Named, current-run artifacts must be extracted. v8 defaults to a
        # blocking digest mismatch; do not override that safety improvement.
        if re.search(r"(?m)^ +(?:skip-decompress|digest-mismatch|github-token|repository|run-id|pattern|artifact-ids|merge-multiple):", step):
            raise AssertionError("Keep named, extracted, current-run artifacts and digest verification")
    for key in ("name", "path"):
        if not re.search(rf"(?m)^ +{key}: \S", step):
            raise AssertionError(f"Artifact transfer needs an explicit {key}")
