"""Explicit migration policy for active workflows, independent of gate behavior.

Only the current main and reviewed Renovate destinations are admitted. Future
majors, mutable branches, third-party actions and local paths need review.
Archived measurement workflows keep their original version contracts.
"""

import re
import textwrap

import yaml


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


class WorkflowLoader(yaml.BaseLoader):
    """Keep GitHub scalar spellings; reject ambiguous mappings and aliases."""

    def compose_node(self, parent, index):
        if self.check_event(yaml.AliasEvent):
            raise AssertionError("Workflow aliases require explicit policy review")
        return super().compose_node(parent, index)

    def construct_mapping(self, node, deep=False):
        result = {}
        for key_node, value_node in node.value:
            key = self.construct_object(key_node, deep=deep)
            if not isinstance(key, str) or key == "<<" or key in result:
                raise AssertionError("Workflow mapping keys must be unique scalar keys without merges")
            result[key] = self.construct_object(value_node, deep=deep)
        return result


def mapping(source):
    if isinstance(source, str):
        try:
            source = yaml.load(textwrap.dedent(source), Loader=WorkflowLoader)
        except yaml.YAMLError as error:
            raise AssertionError("Workflow must be valid YAML") from error
    return require_mapping(source)


def require_mapping(source):
    if not isinstance(source, dict):
        raise AssertionError("Expected a workflow mapping")
    return source


def workflow_jobs(source):
    jobs = require_mapping(mapping(source).get("jobs"))
    if not jobs or any(not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_-]*", key) for key in jobs):
        raise AssertionError("Expected valid workflow job IDs")
    return {key: require_mapping(value) for key, value in jobs.items()}


def steps(source):
    source = mapping(source)
    if "jobs" in source:
        return [step for job in workflow_jobs(source).values() for step in steps(job)]
    if "runs" in source:
        return steps(require_mapping(source["runs"]))
    values = source.get("steps", [])
    if not isinstance(values, list) or any(not isinstance(step, dict) for step in values):
        raise AssertionError("Steps must be a sequence of mappings")
    return values


def action_refs(source):
    source = mapping(source)
    if "jobs" in source:
        return [ref for job in workflow_jobs(source).values() for ref in action_refs(job)]
    if "runs" in source:
        return action_refs(require_mapping(source["runs"]))
    refs = [source["uses"]] if "uses" in source else []
    refs += [ref for step in steps(source) for ref in action_refs(step)]
    if any(not isinstance(ref, str) or not ref for ref in refs):
        raise AssertionError("Action references must be nonempty scalar strings")
    return refs


def require_action(source, name, count=1):
    refs = [ref for ref in action_refs(source) if ref.partition("@")[0] == name]
    if len(refs) != count or not set(refs) <= allowed_actions(name):
        raise AssertionError(f"Expected {count} approved {name} reference(s), got {refs}")
    return refs[0]


def require_hosted_runner(job):
    runner = mapping(job).get("runs-on")
    if not isinstance(runner, str) or runner not in HOSTED_RUNNERS:
        raise AssertionError(f"Expected one approved fixed Ubuntu runner, got {runner}")
    return runner


def inputs(step):
    """Use the runner's ASCII INPUT_* identity and reject shadowing inputs."""
    values = require_mapping(mapping(step).get("with", {}))
    normalized = {}
    for key, value in values.items():
        if not isinstance(key, str) or not key or not key.isascii():
            raise AssertionError("Action input names must be nonempty ASCII strings")
        # Runner Handler.AddInputsToEnvironment replaces spaces and uppercases;
        # lowercasing ASCII gives the same identity for policy comparisons.
        name = key.replace(" ", "_").lower()
        if name in normalized:
            raise AssertionError("Action input names collide after runtime normalization")
        normalized[name] = value
    return normalized


def require_checkout_boundary(step):
    require_action(step, "actions/checkout")
    values = inputs(step)
    if values.get("persist-credentials") != "false":
        raise AssertionError("Checkout must not persist credentials")
    if {"allow-unsafe-pr-checkout", "token", "ssh-key", "github-server-url"} & values.keys():
        raise AssertionError("Checkout must keep the trusted origin and credential boundary")


def require_artifact_transfer(step):
    values = inputs(step)
    if any(ref.startswith("actions/upload-artifact@") for ref in action_refs(step)):
        require_action(step, "actions/upload-artifact")
        # v7 archive:false changes artifact names and accepts only a single file.
        if "archive" in values:
            raise AssertionError("Keep the default ZIP artifact contract")
        if values.get("if-no-files-found") != "error":
            raise AssertionError("Missing artifact files must fail")
    else:
        require_action(step, "actions/download-artifact")
        # v8 defaults to a blocking digest mismatch for current-run artifacts.
        forbidden = {"skip-decompress", "digest-mismatch", "github-token", "repository",
                     "run-id", "pattern", "artifact-ids", "merge-multiple"}
        if forbidden & values.keys():
            raise AssertionError("Keep named, extracted, current-run artifacts and digest verification")
    for key in ("name", "path"):
        if not isinstance(values.get(key), str) or not values[key].strip():
            raise AssertionError(f"Artifact transfer needs an explicit {key}")
