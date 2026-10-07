import copy
import importlib.util
import json
import unittest
from datetime import timedelta
from pathlib import Path

spec = importlib.util.spec_from_file_location("ci_measure", Path(__file__).parents[1] / "ci_measure.py")
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class MeasurementTests(unittest.TestCase):
    def setUp(self):
        self.run = dict(id=1, run_attempt=1, head_sha="abc", html_url="https://example.invalid/run/1",
                        status="completed", conclusion="success", created_at="2026-01-01T00:00:00Z",
                        updated_at="2026-01-01T00:00:59Z")
        self.jobs = {"total_count": 6, "jobs": [dict(id=i, run_id=1, run_attempt=1, head_sha="abc",
                     name=name, status="completed", conclusion="success", started_at="2026-01-01T00:00:02Z",
                     completed_at="2026-01-01T00:00:58Z", steps=[]) for i, name in enumerate(ci.REQUIRED)]}

    def sample(self):
        return ci.measure(self.run, [self.jobs], "warm", "synthetic-v1")

    def timed_sample(self, duration):
        jobs = copy.deepcopy(self.jobs)
        end = (ci.timestamp(jobs["jobs"][0]["started_at"]) + timedelta(seconds=duration)).isoformat()
        for job in jobs["jobs"]:
            job["completed_at"] = end
        run = dict(self.run, updated_at=end)
        return ci.measure(run, [jobs], "warm", "synthetic-v1")

    def test_distinct_clocks_and_runner_seconds(self):
        result = self.sample()
        self.assertEqual(result["execution_seconds"], 56)
        self.assertEqual(result["created_to_required_result_seconds"], 58)
        self.assertEqual(result["created_to_updated_seconds"], 59)
        self.assertEqual(result["runner_seconds"], 336)
        self.assertIsNone(result["event_to_required_result_seconds"])

    def test_all_gate_failure_modes_are_not_success(self):
        for index in range(6):
            for conclusion in ("failure", "cancelled", "timed_out", "skipped", None):
                with self.subTest(index=index, conclusion=conclusion):
                    jobs = copy.deepcopy(self.jobs)
                    jobs["jobs"][index]["conclusion"] = conclusion
                    result = ci.measure(self.run, [jobs], "warm", "synthetic-v1")
                    self.assertFalse(result["quality_success"])

    def test_missing_and_duplicate_gate(self):
        for mode in ("missing", "duplicate"):
            jobs = copy.deepcopy(self.jobs)
            jobs["jobs"][0]["name"] = "unknown" if mode == "missing" else "test"
            if mode == "duplicate":
                jobs["jobs"][1]["name"] = "test"
            result = ci.measure(self.run, [jobs], "warm", "synthetic-v1")
            self.assertFalse(result["quality_success"])
            self.assertIsNone(result["execution_seconds"])

    def test_incomplete_job_has_no_final_wall_time(self):
        self.jobs["jobs"][0].update(status="in_progress", conclusion=None, completed_at=None)
        self.assertIsNone(self.sample()["execution_seconds"])
        self.assertFalse(self.sample()["quality_success"])

    def test_missing_page_is_error(self):
        self.jobs["total_count"] = 7
        with self.assertRaises(ValueError):
            self.sample()

    def test_mixed_run_commit_attempt_is_error(self):
        for field, value in (("run_id", 2), ("head_sha", "different"), ("run_attempt", 2)):
            jobs = copy.deepcopy(self.jobs)
            jobs["jobs"][0][field] = value
            with self.assertRaises(ValueError):
                ci.measure(self.run, [jobs], "warm", "synthetic-v1")

    def test_paginated_jobs(self):
        pages = [dict(total_count=6, jobs=self.jobs["jobs"][:3]), dict(total_count=6, jobs=self.jobs["jobs"][3:])]
        self.assertTrue(ci.measure(self.run, pages, "warm", "synthetic-v1")["quality_success"])

    def test_partial_rerun_copied_jobs_are_not_a_new_timing_sample(self):
        self.run.update(run_attempt=2, run_started_at="2026-01-01T00:00:30Z")
        for job in self.jobs["jobs"]:
            job["run_attempt"] = 2
        with self.assertRaisesRegex(ValueError, "reuses prior job intervals"):
            self.sample()
        for job in self.jobs["jobs"]:
            job["started_at"] = "2026-01-01T00:00:31Z"
        self.assertTrue(self.sample()["quality_success"])
        del self.run["run_started_at"]
        with self.assertRaisesRegex(ValueError, "run_started_at"):
            self.sample()

    def test_nearest_rank(self):
        self.assertEqual(ci.quantiles(range(1, 21)), dict(n=20, p50=10, p95=19, max=20))
        self.assertIsNone(ci.quantiles([])["p95"])

    def test_single_sample_never_passes_target(self):
        result = ci.summarize([self.sample()])[0]
        self.assertTrue(result["small_sample_warning"])
        self.assertFalse(result["ordinary_budget_observed"])

    def test_failure_stays_in_denominator(self):
        rows = []
        for i in range(20):
            row = self.sample()
            row["run_id"] = i
            rows.append(row)
        self.assertTrue(ci.summarize(rows)[0]["ordinary_budget_observed"])
        rows[0].update(quality_success=False, errors=["cancelled"], execution_seconds=None)
        result = ci.summarize(rows)[0]
        self.assertEqual(result["attempts"], 20)
        self.assertEqual(result["non_success_rate"], .05)
        self.assertFalse(result["ordinary_budget_observed"])

    def test_duplicate_attempt_and_mixed_cohorts(self):
        row = self.sample()
        with self.assertRaises(ValueError):
            ci.summarize([row, row])
        other = copy.deepcopy(row)
        other.update(run_id=2, cohort="other")
        self.assertEqual(len(ci.summarize([row, other])), 2)

    def test_test_json_keeps_failed_skipped_and_package_rows(self):
        result = ci.test_times(['{"Action":"pass","Package":"p","Elapsed":3}',
                                '{"Action":"fail","Package":"p","Test":"T","Elapsed":2}',
                                '{"Action":"skip","Package":"p","Test":"U"}',
                                '{"Action":"output","Output":"synthetic"}'])
        self.assertEqual(len(result), 3)
        self.assertEqual(result[0]["seconds"], 3)
        self.assertEqual(result[1]["action"], "fail")

    def test_reduced_gate_set_cannot_claim_project_target(self):
        sample = ci.measure(self.run, [self.jobs], "warm", "synthetic-v1", ["generated-files"])
        rows = [dict(sample, run_id=i) for i in range(20)]
        self.assertFalse(ci.summarize(rows)[0]["ordinary_budget_observed"])

    def test_parallel_parent_wall_time_is_not_active_elapsed(self):
        events = [
            dict(Action="start", Package="p", Time="2026-01-01T00:00:00Z"),
            dict(Action="run", Package="p", Test="Parent", Time="2026-01-01T00:00:01Z"),
            dict(Action="run", Package="p", Test="Parent/child", Time="2026-01-01T00:00:01Z"),
            dict(Action="pause", Package="p", Test="Parent/child", Time="2026-01-01T00:00:01Z"),
            dict(Action="cont", Package="p", Test="Parent/child", Time="2026-01-01T00:00:02Z"),
            dict(Action="pass", Package="p", Test="Parent/child", Elapsed=2, Time="2026-01-01T00:00:04Z"),
            dict(Action="pass", Package="p", Test="Parent", Elapsed=.04, Time="2026-01-01T00:00:04Z"),
            dict(Action="pass", Package="p", Elapsed=4, Time="2026-01-01T00:00:04Z"),
        ]
        rows = ci.test_times(map(json.dumps, events))
        self.assertEqual(rows[0]["wall_seconds"], 4)
        parent = next(r for r in rows if r["test"] == "Parent")
        self.assertEqual(parent["seconds"], .04)
        self.assertEqual(parent["wall_seconds"], 3)
        self.assertEqual(next(r for r in rows if r["test"] == "Parent/child")["wall_seconds"], 3)

    def test_test_wall_time_repetitions_and_incomplete_streams(self):
        events = []
        for start, end, action in [(1, 3, "pass"), (4, 7, "fail"), (8, 8, "skip")]:
            events += [dict(Action="run", Package="p", Test="T", Time=f"2026-01-01T00:00:0{start}Z"),
                       dict(Action=action, Package="p", Test="T", Time=f"2026-01-01T00:00:0{end}Z")]
        # Terminal without a new start must not reuse the previous repetition.
        events.append(dict(Action="pass", Package="p", Test="T", Elapsed=1,
                           Time="2026-01-01T00:00:09Z"))
        events.append(dict(Action="run", Package="p", Test="unfinished",
                           Time="2026-01-01T00:00:09Z"))
        rows = ci.test_times(map(json.dumps, events))
        self.assertEqual([r["wall_seconds"] for r in rows], [3, 2, None, 0])
        self.assertEqual(len(rows), 4)

    def test_test_wall_time_does_not_mix_packages(self):
        events = [dict(Action="run", Package="p", Test="T", Time="2026-01-01T00:00:00Z"),
                  dict(Action="pass", Package="other", Test="T", Elapsed=2, Time="2026-01-01T00:00:02Z"),
                  dict(Action="pass", Package="p", Test="T", Elapsed=1, Time="2026-01-01T00:00:01Z")]
        rows = ci.test_times(map(json.dumps, events))
        self.assertIsNone(rows[0]["wall_seconds"])
        self.assertEqual(rows[1]["wall_seconds"], 1)

    def test_gate_order_does_not_split_cohort(self):
        first = self.sample()
        second = dict(first, run_id=2, required_jobs=list(reversed(first["required_jobs"])))
        self.assertEqual(len(ci.summarize([first, second])), 1)

    def test_summary_rejects_invalid_inputs(self):
        for field, value in (("schema_version", 2), ("quality_success", "false"),
                             ("execution_seconds", -1), ("execution_seconds", float("nan")),
                             ("execution_seconds", float("inf")), ("execution_seconds", True),
                             ("errors", ["failed"]), ("conclusion", "cancelled")):
            with self.subTest(field=field, value=value):
                sample = dict(self.sample(), **{field: value})
                with self.assertRaises(ValueError):
                    ci.summarize([sample])

    def test_successful_summary_requires_complete_job_rows(self):
        for jobs in (None, [], {}, "jobs", [None] * 6, [{}] * 6,
                     self.sample()["jobs"][:-1], self.sample()["jobs"] * 2):
            with self.subTest(jobs=jobs):
                sample = dict(self.sample(), jobs=jobs)
                with self.assertRaises(ValueError):
                    ci.summarize([sample])
        sample = self.sample()
        del sample["jobs"]
        with self.assertRaises(ValueError):
            ci.summarize([sample])

    def test_successful_summary_checks_job_evidence(self):
        for field, value in (("name", "unknown"), ("name", ci.REQUIRED[1]),
                             ("id", 1), ("id", None), ("id", "0"), ("id", True),
                             ("status", "in_progress"), ("conclusion", "failure"),
                             ("conclusion", "cancelled"), ("conclusion", "skipped"),
                             ("started_at", None), ("completed_at", None),
                             ("started_at", "invalid"), ("started_at", 123),
                             ("started_at", "2026-01-01T00:00:59Z"),
                             ("completed_at", "2026-01-01T00:00:58"),
                             ("seconds", None), ("seconds", True), ("seconds", -1),
                             ("seconds", float("nan")), ("seconds", float("inf")),
                             ("seconds", 1)):
            with self.subTest(field=field, value=value):
                sample = self.sample()
                sample["jobs"][0][field] = value
                with self.assertRaises(ValueError):
                    ci.summarize([dict(sample, run_id=i) for i in range(20)])

    def test_successful_summary_rejects_edited_aggregates(self):
        for field in ("execution_seconds", "runner_seconds"):
            with self.subTest(field=field):
                sample = self.sample()
                sample[field] = 1
                with self.assertRaisesRegex(ValueError, "aggregates"):
                    ci.summarize([sample])

    def test_summary_retains_failed_and_incomplete_job_evidence(self):
        for changes in (dict(conclusion="failure"), dict(conclusion="cancelled"),
                        dict(status="in_progress", conclusion=None, completed_at=None),
                        dict(name="unknown"), dict(name=ci.REQUIRED[1])):
            with self.subTest(changes=changes):
                jobs = copy.deepcopy(self.jobs)
                jobs["jobs"][0].update(changes)
                sample = ci.measure(self.run, [jobs], "warm", "synthetic-v1")
                result = ci.summarize([sample])[0]
                self.assertEqual(result["attempts"], 1)
                self.assertEqual(result["successes"], 0)
                self.assertEqual(result["non_success_rate"], 1)
                self.assertFalse(result["ordinary_budget_observed"])

    def test_workflow_failure_blocks_quality(self):
        self.run["conclusion"] = "failure"
        self.assertFalse(self.sample()["quality_success"])

    def test_duplicate_job_ids_and_inconsistent_page_totals(self):
        self.jobs["jobs"][1]["id"] = self.jobs["jobs"][0]["id"]
        with self.assertRaises(ValueError):
            self.sample()
        with self.assertRaises(ValueError):
            ci.measure(self.run, [dict(total_count=6, jobs=[]), dict(total_count=7, jobs=[])], "warm", "v1")

    def test_positive_event_timestamp(self):
        result = ci.measure(self.run, [self.jobs], "warm", "v1", event_at="2025-12-31T19:00:00-05:00")
        self.assertEqual(result["event_to_required_result_seconds"], 58)

    def test_target_boundary(self):
        rows = [dict(self.timed_sample(180), run_id=i) for i in range(20)]
        self.assertTrue(ci.summarize(rows)[0]["ordinary_budget_observed"])
        rows = [dict(self.timed_sample(180.1), run_id=i) for i in range(20)]
        self.assertFalse(ci.summarize(rows)[0]["ordinary_budget_observed"])
        rows = [dict(self.timed_sample(180 if i < 18 else 300), run_id=i) for i in range(20)]
        self.assertTrue(ci.summarize(rows)[0]["ordinary_budget_observed"])
        rows = [dict(self.timed_sample(180 if i < 18 else 300.1), run_id=i) for i in range(20)]
        self.assertFalse(ci.summarize(rows)[0]["ordinary_budget_observed"])

    def test_ordinary_budget_does_not_apply_to_cold_or_refresh(self):
        for condition in ("cold", "go-refresh", "native-refresh", "tool-refresh", "historical", "diagnostic"):
            with self.subTest(condition=condition):
                rows = [dict(self.sample(), run_id=i, condition=condition) for i in range(20)]
                result = ci.summarize(rows)[0]
                self.assertFalse(result["budget_applicable"])
                self.assertFalse(result["ordinary_budget_observed"])

    def test_invalid_time(self):
        with self.assertRaises(ValueError):
            ci.seconds("2026-01-01T00:00:02Z", "2026-01-01T00:00:01Z")
        with self.assertRaises(ValueError):
            ci.timestamp("2026-01-01T00:00:00")


if __name__ == "__main__":
    unittest.main()
