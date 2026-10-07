#!/usr/bin/env python3
"""Compare complete CI profiles across two same-head runs, without starting CI."""
import argparse
import json
import re
from collections import Counter

import ci_measure as ci

UBUNTU_NATIVE = ("test", "native-build", "vulnerability", "licenses")
SHARED = ("fuzz", "generated-files", "arch-compatibility")
NIX_NATIVE = tuple("nix-" + name for name in UBUNTU_NATIVE)


def check_pair(ubuntu, nix):
    for run, path in ((ubuntu, ".github/workflows/ci.yml"), (nix, ".github/workflows/ci-nix.yml")):
        if run.get("path") != path:
            raise ValueError("unexpected workflow path")
        if run.get("event") != "pull_request":
            raise ValueError("comparison requires the paired pull-request workflows")
    if ubuntu.get("id") == nix.get("id"):
        raise ValueError("distinct workflow runs are required")
    for field in ("head_sha", "head_branch"):
        if not ubuntu.get(field) or ubuntu[field] != nix.get(field):
            raise ValueError("workflow runs must use the same head and branch")
    repository = ubuntu.get("repository", {}).get("full_name")
    if not repository or repository != nix.get("repository", {}).get("full_name"):
        raise ValueError("workflow runs must use the same repository")
    pull_requests = []
    for run in (ubuntu, nix):
        prs = run.get("pull_requests", [])
        if len(prs) != 1:
            raise ValueError("comparison requires one identified pull request per run")
        pr = prs[0]
        identity = (pr.get("number"), pr.get("head", {}).get("sha"), pr.get("base", {}).get("sha"),
                    pr.get("head", {}).get("repo", {}).get("id"), pr.get("base", {}).get("repo", {}).get("id"))
        if any(value is None for value in identity) or identity[1] != run["head_sha"]:
            raise ValueError("incomplete pull-request source provenance")
        pull_requests.append(identity)
    if pull_requests[0] != pull_requests[1]:
        raise ValueError("pull-request number/base/head/repository provenance differs")
    # Failed-jobs-only reruns are not independent timing observations. The
    # ordinary measurement parser additionally validates every job's attempt.
    if ubuntu.get("run_attempt") != 1 or nix.get("run_attempt") != 1:
        raise ValueError("paired timing requires initial full runs, not reruns")


def profile(rows, expected, condition, runs):
    names = Counter(row["name"] for row in rows)
    errors = [f"expected one job {name}, found {names[name]}" for name in expected if names[name] != 1]
    selected = [row for row in rows if row["name"] in expected]
    for row in selected:
        if row["status"] != "completed" or row["conclusion"] != "success" or row["seconds"] is None:
            errors.append(f"job {row['name']}: {row['status']}/{row['conclusion']}")
    # Do not call an unfinished comparison complete when its shared workflow
    # still has work, even if one technology's native subset finished early.
    if any(run.get("status") != "completed" for run in runs):
        errors.append("comparison workflow still running")
    complete = len(selected) == len(expected) and not any(names[name] != 1 for name in expected) and all(row["seconds"] is not None for row in selected)
    start = min((row["started_at"] for row in selected), key=ci.timestamp) if complete else None
    end = max((row["completed_at"] for row in selected), key=ci.timestamp) if complete else None
    return {
        "condition": condition, "required_jobs": list(expected),
        "quality_success": not errors, "errors": errors,
        "execution_seconds": ci.seconds(start, end),
        "runner_seconds": sum(row["seconds"] for row in selected) if complete else None,
        "first_job_started_at": start, "last_job_completed_at": end,
        "jobs": selected,
    }


def checkout_commit(log):
    lines = log.splitlines()
    found = []
    for index, line in enumerate(lines[:-1]):
        if re.search(r"(?:##)?\[command\].*\bgit log -1 --format=%H$", line):
            match = re.fullmatch(r"\S+ ([0-9a-f]{40})", lines[index + 1])
            if match:
                found.append(match[1])
    if len(found) != 1:
        raise ValueError("expected exactly one actual Actions checkout commit in each job log")
    return found[0]


def verify_jobs_and_checkout(run, pages, expected, logs):
    jobs = [job for page in pages for job in page["jobs"]]
    if Counter(job["name"] for job in jobs) != Counter(expected):
        raise ValueError("unexpected/missing/duplicate workflow jobs; refusing to omit experiment work")
    if set(logs) != {job["id"] for job in jobs}:
        raise ValueError("checkout logs must cover every job exactly once")
    return {checkout_commit(logs[job["id"]]) for job in jobs}


def compare(ubuntu, ubuntu_pages, nix, nix_pages, ubuntu_condition, nix_condition, ubuntu_logs, nix_logs):
    check_pair(ubuntu, nix)
    checkouts = verify_jobs_and_checkout(ubuntu, ubuntu_pages, ci.REQUIRED, ubuntu_logs)
    checkouts |= verify_jobs_and_checkout(nix, nix_pages, NIX_NATIVE, nix_logs)
    if len(checkouts) != 1:
        raise ValueError("actual checked-out commits differ between jobs/workflows")
    tested_commit = next(iter(checkouts))
    a = ci.measure(ubuntu, ubuntu_pages, ubuntu_condition, "ubuntu-nix-paired", required=ci.REQUIRED)
    b = ci.measure(nix, nix_pages, nix_condition, "ubuntu-nix-paired", required=NIX_NATIVE)
    rows = []
    for run, result in ((ubuntu, a), (nix, b)):
        rows += [dict(row, source_run_id=run["id"], source_run_url=run["html_url"],
                      source_workflow=run["path"], source_head_sha=run["head_sha"],
                      source_attempt=run["run_attempt"]) for row in result["jobs"]]
    if any("ubuntu-24.04" not in row.get("labels", []) for row in rows):
        raise ValueError("comparison requires the same ubuntu-24.04 hosted runner class for every gate")
    runs = (ubuntu, nix)
    ubuntu_profile = profile(rows, ci.REQUIRED, ubuntu_condition, runs)
    nix_profile = profile(rows, NIX_NATIVE + SHARED, nix_condition, runs)
    total = profile(rows, ci.REQUIRED + NIX_NATIVE, "paired-experiment", runs)
    # Preserve workflow-level failures (including unexpected bootstrap jobs)
    # separately; a passing native subset never makes the experiment green.
    total["workflow_errors"] = a["errors"] + b["errors"]
    total["quality_success"] = total["quality_success"] and not total["workflow_errors"]
    if b["errors"]:
        nix_profile["quality_success"] = False
        nix_profile["errors"] += b["errors"]
    if a["errors"]:
        ubuntu_profile["quality_success"] = False
        ubuntu_profile["errors"] += a["errors"]
    return {
        "schema_version": 1, "head_commit": ubuntu["head_sha"], "checkout_commit": tested_commit,
        "runner_class": "ubuntu-24.04",
        "ubuntu_profile": ubuntu_profile, "nix_profile": nix_profile,
        "whole_experiment": total,
        "native_only_diagnostics": {
            "ubuntu": profile(rows, UBUNTU_NATIVE, ubuntu_condition, runs),
            "nix": profile(rows, NIX_NATIVE, nix_condition, runs),
        },
        "shared_jobs": list(SHARED),
        "notes": [
            "Profile runner-seconds each include shared work; never add the two profiles. Whole-experiment cost counts every job once.",
            "Native-only diagnostics are not full quality-gate latency or budget evidence.",
            "Same source and runner class do not remove distro/package/version or host variability. No pure package-manager causal claim.",
            "One paired cold/warm experiment establishes neither p50 nor p95.",
        ],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("ubuntu", "nix"):
        parser.add_argument(f"--{name}-run", required=True)
        parser.add_argument(f"--{name}-jobs", nargs="+", required=True)
        parser.add_argument(f"--{name}-condition", choices=ci.CONDITIONS, required=True)
        parser.add_argument(f"--{name}-logs", required=True, help="directory containing unmodified JOB_ID.log files from GitHub")
    args = parser.parse_args()
    try:
        from pathlib import Path
        a_pages = [ci.load(path) for path in args.ubuntu_jobs]
        b_pages = [ci.load(path) for path in args.nix_jobs]
        def logs(directory, pages):
            return {job["id"]: (Path(directory) / f"{job['id']}.log").read_text()
                    for page in pages for job in page["jobs"]}
        result = compare(ci.load(args.ubuntu_run), a_pages, ci.load(args.nix_run), b_pages,
                         args.ubuntu_condition, args.nix_condition,
                         logs(args.ubuntu_logs, a_pages), logs(args.nix_logs, b_pages))
        print(json.dumps(result, indent=2, sort_keys=True))
    except (ValueError, KeyError, TypeError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
