#!/usr/bin/env python3
"""Offline, dependency-free CI measurements. Never authenticates or starts runs."""
import argparse
import json
import math
from collections import Counter, defaultdict
from datetime import datetime
from pathlib import Path

REQUIRED = ("test", "fuzz", "native-build", "vulnerability", "licenses", "generated-files")
P50_BUDGET_SECONDS = 180
P95_BUDGET_SECONDS = 300
MINIMUM_SAMPLES = 20

CONDITIONS = ("cold", "warm", "source-only", "go-refresh", "native-refresh", "tool-refresh", "historical", "diagnostic")


def timestamp(value):
    if value is None:
        return None
    result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if result.utcoffset() is None:
        raise ValueError("timestamps must include a timezone")
    return result


def seconds(start, end):
    if start is None or end is None:
        return None
    result = (timestamp(end) - timestamp(start)).total_seconds()
    if result < 0:
        raise ValueError("negative interval")
    return result


def load(path):
    return json.loads(Path(path).read_text())


def measure(run, jobs_pages, condition, cohort, required=REQUIRED, event_at=None):
    """Preserve non-successes and reject missing, duplicate or paginated-away jobs."""
    if condition not in CONDITIONS:
        raise ValueError("unknown measurement condition")
    if not cohort or not required or len(set(required)) != len(required):
        raise ValueError("cohort and distinct required job names are mandatory")
    jobs = []
    total = None
    for page in jobs_pages:
        if total is None:
            total = page["total_count"]
        if page["total_count"] != total:
            raise ValueError("inconsistent jobs pagination")
        jobs.extend(page["jobs"])
    if total != len(jobs) or len({j["id"] for j in jobs}) != len(jobs):
        raise ValueError("missing or duplicate jobs pages")
    attempt = run["run_attempt"]
    if any(j.get("run_id") != run["id"] or j.get("head_sha") != run["head_sha"] for j in jobs):
        raise ValueError("jobs do not belong to this run/commit")
    if any(j.get("run_attempt") != attempt for j in jobs):
        raise ValueError("use attempt-specific jobs; do not mix reruns")
    counts = Counter(j["name"] for j in jobs)
    errors = [f"required job {name}: expected one, found {counts[name]}" for name in required if counts[name] != 1]
    selected = [j for j in jobs if j["name"] in required]
    if attempt > 1:
        # GitHub can copy earlier successful jobs into a failed-jobs-only rerun,
        # assigning new IDs/attempt numbers but retaining their old intervals.
        # Such recovery is valid quality evidence, not an independent timing run.
        attempt_start = timestamp(run.get("run_started_at"))
        if attempt_start is None:
            raise ValueError("rerun requires its run_started_at timestamp")
        if any(j.get("started_at") and timestamp(j["started_at"]) < attempt_start for j in selected):
            raise ValueError("partial rerun reuses prior job intervals; retain recovery separately")
    rows = []
    for j in selected:
        duration = seconds(j.get("started_at"), j.get("completed_at"))
        if j.get("status") != "completed" or j.get("conclusion") != "success" or duration is None:
            errors.append(f"required job {j['name']}: {j.get('status')}/{j.get('conclusion')}")
        rows.append({
            "id": j["id"], "name": j["name"], "url": j.get("html_url"),
            "status": j.get("status"), "conclusion": j.get("conclusion"),
            "started_at": j.get("started_at"), "completed_at": j.get("completed_at"),
            "seconds": duration,
            # API created_at is a run clock, not each dependent job's enqueue time.
            "run_created_to_job_start_seconds": seconds(run["created_at"], j.get("started_at")),
            "runner_name": j.get("runner_name"), "labels": j.get("labels", []),
            "steps": [{"name": s["name"], "conclusion": s.get("conclusion"),
                       "seconds": seconds(s.get("started_at"), s.get("completed_at"))}
                      for s in j.get("steps", [])],
        })
    if run.get("status") != "completed" or run.get("conclusion") != "success":
        errors.append(f"workflow: {run.get('status')}/{run.get('conclusion')}")
    intervals_complete = len(selected) == len(required) and not any(counts[n] != 1 for n in required) and all(r["seconds"] is not None for r in rows)
    start = min((r["started_at"] for r in rows), key=timestamp) if intervals_complete else None
    end = max((r["completed_at"] for r in rows), key=timestamp) if intervals_complete else None
    return {
        "schema_version": 1, "run_id": run["id"], "attempt": attempt,
        "commit": run["head_sha"], "url": run["html_url"], "condition": condition,
        "cohort": cohort, "required_jobs": list(required), "status": run["status"],
        "conclusion": run.get("conclusion"), "quality_success": not errors, "errors": errors,
        "execution_seconds": seconds(start, end),
        "runner_seconds": sum(r["seconds"] for r in rows) if intervals_complete else None,
        "created_to_required_result_seconds": seconds(run["created_at"], end),
        "created_to_updated_seconds": seconds(run["created_at"], run.get("updated_at")),
        "event_to_required_result_seconds": seconds(event_at, end),
        "event_at": event_at, "jobs": rows,
    }


def quantiles(values):
    values = sorted(values)
    if not values:
        return {"n": 0, "p50": None, "p95": None, "max": None}
    # Nearest rank, fixed before observing candidates. n<20 is descriptive only.
    return {"n": len(values), "p50": values[math.ceil(.50 * len(values)) - 1],
            "p95": values[math.ceil(.95 * len(values)) - 1], "max": values[-1]}


def validate_sample(sample):
    if sample.get("schema_version") != 1 or type(sample.get("quality_success")) is not bool:
        raise ValueError("unsupported sample schema or invalid success boolean")
    if sample.get("condition") not in CONDITIONS or not sample.get("cohort"):
        raise ValueError("invalid condition/cohort")
    required = sample.get("required_jobs")
    if not isinstance(required, list) or not required or not all(isinstance(n, str) and n for n in required) or len(set(required)) != len(required):
        raise ValueError("invalid required jobs")
    if not isinstance(sample.get("errors"), list):
        raise ValueError("invalid errors list")
    for name in ("execution_seconds", "runner_seconds", "created_to_required_result_seconds", "created_to_updated_seconds", "event_to_required_result_seconds"):
        value = sample[name]
        if value is not None and (type(value) not in (int, float) or not math.isfinite(value) or value < 0):
            raise ValueError("invalid nonnegative finite duration")
    if sample["quality_success"] and (sample["errors"] or sample.get("status") != "completed" or sample.get("conclusion") != "success" or sample["execution_seconds"] is None or sample["runner_seconds"] is None):
        raise ValueError("inconsistent successful sample")
    if sample["quality_success"]:
        # Saved samples are inputs too: a stale success flag or edited aggregate
        # must not certify missing/failed jobs or a shorter interval than the evidence.
        jobs = sample.get("jobs")
        if not isinstance(jobs, list) or len(jobs) != len(required) or not all(isinstance(j, dict) for j in jobs):
            raise ValueError("successful sample requires complete job rows")
        names = [j.get("name") for j in jobs]
        ids = [j.get("id") for j in jobs]
        if not all(isinstance(n, str) for n in names) or sorted(names) != sorted(required):
            raise ValueError("successful sample jobs do not match required jobs")
        if not all(type(i) is int and i >= 0 for i in ids) or len(set(ids)) != len(ids):
            raise ValueError("successful sample requires distinct job IDs")
        starts, ends, durations = [], [], []
        for job in jobs:
            if job.get("status") != "completed" or job.get("conclusion") != "success":
                raise ValueError("successful sample contains a non-successful job")
            if not isinstance(job.get("started_at"), str) or not isinstance(job.get("completed_at"), str):
                raise ValueError("successful sample requires complete job intervals")
            start, end = timestamp(job["started_at"]), timestamp(job["completed_at"])
            duration = (end - start).total_seconds()
            if duration < 0 or type(job.get("seconds")) not in (int, float) or job["seconds"] != duration:
                raise ValueError("successful sample has an inconsistent job duration")
            starts.append(start)
            ends.append(end)
            durations.append(duration)
        if sample["execution_seconds"] != (max(ends) - min(starts)).total_seconds() or sample["runner_seconds"] != sum(durations):
            raise ValueError("successful sample aggregates do not match job intervals")


def summarize(samples):
    seen = set()
    groups = defaultdict(list)
    for sample in samples:
        validate_sample(sample)
        key = (sample["run_id"], sample["attempt"])
        if key in seen:
            raise ValueError("duplicate run attempt")
        seen.add(key)
        # Never combine revisions, conditions, environment cohorts or gate sets.
        group = (sample["commit"], sample["condition"], sample["cohort"], tuple(sorted(sample["required_jobs"])))
        groups[group].append(sample)
    output = []
    for (commit, condition, cohort, required), rows in sorted(groups.items()):
        good = [r for r in rows if r["quality_success"]]
        metrics = {name: quantiles([r[name] for r in rows if r[name] is not None])
                   for name in ("execution_seconds", "runner_seconds", "created_to_required_result_seconds", "event_to_required_result_seconds")}
        # Incomplete/failed/cancelled runs remain in the denominator and block a pass.
        enough = len(rows) >= MINIMUM_SAMPLES and len(good) == len(rows) and metrics["execution_seconds"]["n"] == len(rows)
        output.append({"commit": commit, "condition": condition, "cohort": cohort,
                       "required_jobs": required, "attempts": len(rows), "successes": len(good),
                       "non_success_rate": (len(rows) - len(good)) / len(rows),
                       "small_sample_warning": len(rows) < MINIMUM_SAMPLES, "metrics": metrics,
                       "complete_initial_gate_set": set(required) == set(REQUIRED),
                       "budget_applicable": condition in ("warm", "source-only"),
                       "budget": {"p50_seconds": P50_BUDGET_SECONDS, "p95_seconds": P95_BUDGET_SECONDS, "minimum_samples": MINIMUM_SAMPLES},
                       "ordinary_budget_observed": enough and set(required) == set(REQUIRED) and condition in ("warm", "source-only") and metrics["execution_seconds"]["p50"] <= P50_BUDGET_SECONDS and metrics["execution_seconds"]["p95"] <= P95_BUDGET_SECONDS,
                       "runs": [{"run_id": r["run_id"], "attempt": r["attempt"], "url": r["url"], "errors": r["errors"]} for r in rows]})
    return output


def test_times(lines):
    """Keep Go's active elapsed and observed wall intervals distinct.

    Parallel parent tests can report little Elapsed while waiting for children.
    Neither clock can be summed across overlapping tests or packages.
    Missing start events leave wall_seconds unknown, not zero.
    """
    results = []
    starts = {}
    for line in lines:
        event = json.loads(line)
        key = (event.get("Package"), event.get("Test"))
        if event.get("Action") in ("start", "run"):
            starts[key] = event.get("Time")
        if event.get("Action") in ("pass", "fail", "skip"):
            start = starts.pop(key, None)
            results.append({"package": event.get("Package"), "test": event.get("Test"),
                            "action": event["Action"], "seconds": event.get("Elapsed"),
                            "wall_seconds": seconds(start, event.get("Time"))})
    return sorted(results, key=lambda r: (r["wall_seconds"] if r["wall_seconds"] is not None
                                         else r["seconds"] if r["seconds"] is not None else -1), reverse=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    run = sub.add_parser("run", help="measure saved run and attempt-specific jobs API responses")
    run.add_argument("--run", required=True)
    run.add_argument("--jobs", nargs="+", required=True)
    run.add_argument("--condition", choices=CONDITIONS, required=True)
    run.add_argument("--cohort", required=True, help="reviewed environment/measurement protocol ID")
    run.add_argument("--required", nargs="+", default=list(REQUIRED))
    run.add_argument("--event-at", help="verified push/PR event time; omit if unknown")
    summary = sub.add_parser("summary")
    summary.add_argument("samples", nargs="+")
    tests = sub.add_parser("tests")
    tests.add_argument("jsonl")
    args = parser.parse_args()
    try:
        if args.command == "run":
            result = measure(load(args.run), [load(p) for p in args.jobs], args.condition, args.cohort, args.required, args.event_at)
        elif args.command == "summary":
            result = summarize([load(p) for p in args.samples])
        else:
            with open(args.jsonl) as source:
                result = test_times(source)
    except (ValueError, KeyError, TypeError, OSError) as error:
        parser.exit(2, f"invalid measurement input: {error}\n")
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
