import importlib.util
import sys
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SCRIPTS))
spec = importlib.util.spec_from_file_location("ci_compare_profiles", SCRIPTS / "ci_compare_profiles.py")
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)


class ProfileComparisonTests(unittest.TestCase):
    def setUp(self):
        self.ubuntu = dict(id=1, path=".github/workflows/ci.yml", event="pull_request", head_sha="abc",
                           head_branch="candidate", pull_requests=[{"number": 141, "head": {"sha": "abc", "repo": {"id": 1}},
                           "base": {"sha": "base", "repo": {"id": 1}}}], repository={"full_name": "owner/repo"}, run_attempt=1,
                           status="completed", conclusion="success", created_at="2026-01-01T00:00:00Z",
                           updated_at="2026-01-01T00:04:00Z", html_url="https://example.invalid/1")
        self.nix = dict(self.ubuntu, id=2, path=".github/workflows/ci-nix.yml", html_url="https://example.invalid/2")
        self.a = self.jobs(1, comparison.COMPARISON_REQUIRED, "2026-01-01T00:03:00Z")
        self.b = self.jobs(2, comparison.NIX_NATIVE, "2026-01-01T00:04:00Z")

    def jobs(self, run, names, end):
        return {"total_count": len(names), "jobs": [dict(id=100 * run + index, run_id=run,
                head_sha="abc", run_attempt=1, name=name, status="completed", conclusion="success",
                started_at="2026-01-01T00:00:00Z", completed_at=end, steps=[], labels=["ubuntu-24.04"])
                for index, name in enumerate(names)]}

    def compare(self):
        return comparison.compare(self.ubuntu, [self.a], self.nix, [self.b], "warm", "cold", self.logs(self.a), self.logs(self.b))

    def logs(self, page, sha="a" * 40):
        return {job["id"]: "2026-01-01T00:00:00Z [command]/usr/bin/git log -1 --format=%H\n"
                + "2026-01-01T00:00:00Z " + sha + "\n" for job in page["jobs"]}

    def test_full_profiles_and_union_have_distinct_clocks_costs(self):
        result = self.compare()
        self.assertEqual(result["ubuntu_profile"]["execution_seconds"], 180)
        self.assertEqual(result["nix_profile"]["execution_seconds"], 240)
        self.assertEqual(result["whole_experiment"]["runner_seconds"], 7 * 180 + 4 * 240)
        self.assertEqual(len(result["nix_profile"]["jobs"]), 7)
        self.assertEqual(len(result["whole_experiment"]["jobs"]), 11)
        self.assertTrue(result["whole_experiment"]["quality_success"])
        self.assertEqual(result["nix_profile"]["condition"], "cold")
        self.assertIn("source_run_id", result["nix_profile"]["jobs"][0])

    def test_shared_failure_blocks_both_profiles(self):
        next(row for row in self.a["jobs"] if row["name"] == "arch-compatibility")["conclusion"] = "failure"
        result = self.compare()
        for name in ("ubuntu_profile", "nix_profile", "whole_experiment"):
            self.assertFalse(result[name]["quality_success"])

    def test_ubuntu_native_failure_does_not_fabricate_nix_failure(self):
        self.a["jobs"][0]["conclusion"] = "failure"
        self.ubuntu["conclusion"] = "failure"
        result = self.compare()
        self.assertFalse(result["ubuntu_profile"]["quality_success"])
        self.assertTrue(result["nix_profile"]["quality_success"])
        self.assertFalse(result["whole_experiment"]["quality_success"])

    def test_nix_failure_blocks_only_nix_and_whole(self):
        self.b["jobs"][0]["conclusion"] = "failure"
        result = self.compare()
        self.assertTrue(result["ubuntu_profile"]["quality_success"])
        self.assertFalse(result["nix_profile"]["quality_success"])
        self.assertFalse(result["whole_experiment"]["quality_success"])

    def test_missing_or_duplicate_gate_is_not_a_success(self):
        for field in ("missing", "duplicate"):
            with self.subTest(field=field):
                self.setUp()
                self.b["jobs"][0]["name"] = "unknown" if field == "missing" else comparison.NIX_NATIVE[1]
                with self.assertRaises(ValueError):
                    self.compare()

    def test_wrong_head_branch_workflow_repository_or_attempt_rejected(self):
        for field, value in (("head_sha", "different"), ("head_branch", "other"), ("run_attempt", 2),
                             ("path", ".github/workflows/other.yml"), ("repository", {"full_name": "other/repo"})):
            with self.subTest(field=field):
                self.setUp()
                self.nix[field] = value
                with self.assertRaises(ValueError):
                    self.compare()

    def test_unfinished_other_workflow_prevents_completed_comparison(self):
        self.ubuntu["status"] = "in_progress"
        self.assertFalse(self.compare()["nix_profile"]["quality_success"])

    def test_unexpected_successful_job_cannot_disappear_from_cost(self):
        extra = dict(self.a["jobs"][0], id=999, name="bootstrap", completed_at="2026-01-01T00:10:00Z")
        self.a["jobs"].append(extra)
        self.a["total_count"] += 1
        with self.assertRaises(ValueError):
            self.compare()

    def test_pr_base_provenance_must_match(self):
        import copy
        self.nix["pull_requests"] = copy.deepcopy(self.nix["pull_requests"])
        self.nix["pull_requests"][0]["base"]["sha"] = "different"
        with self.assertRaises(ValueError):
            self.compare()

    def test_actual_merge_checkout_must_match_across_all_jobs(self):
        with self.assertRaises(ValueError):
            comparison.compare(self.ubuntu, [self.a], self.nix, [self.b], "warm", "cold",
                               self.logs(self.a), self.logs(self.b, sha="b" * 40))

    def test_missing_or_ambiguous_checkout_log_is_rejected(self):
        for value in ("", next(iter(self.logs(self.a).values())) * 2):
            logs = self.logs(self.a)
            logs[self.a["jobs"][0]["id"]] = value
            with self.assertRaises(ValueError):
                comparison.compare(self.ubuntu, [self.a], self.nix, [self.b], "warm", "cold", logs, self.logs(self.b))

    def test_different_or_missing_runner_class_is_rejected(self):
        self.b["jobs"][0]["labels"] = ["ubuntu-latest"]
        with self.assertRaises(ValueError):
            self.compare()

    def test_job_provenance_and_pagination_are_checked(self):
        self.b["jobs"][0]["run_id"] = 1
        with self.assertRaises(ValueError):
            self.compare()
        self.setUp()
        self.b["total_count"] += 1
        with self.assertRaises(ValueError):
            self.compare()


if __name__ == "__main__":
    unittest.main()
