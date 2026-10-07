import json
import os
from pathlib import Path
import selectors
import signal
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "ci_time_command.py"


@unittest.skipUnless(sys.platform.startswith("linux"), "timing wrapper requires Linux")
class CommandTimingTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.output = self.root / "timing.json"

    def command(self, *command):
        return [sys.executable, str(SCRIPT), "--output", str(self.output), "--", *command]

    def run_child(self, source, **kwargs):
        return subprocess.run(self.command(sys.executable, "-c", source),
                              capture_output=True, text=True, timeout=10, **kwargs)

    def report(self):
        report = json.loads(self.output.read_text())
        self.assertEqual(set(report), {"schema_version", "elapsed_seconds", "child_user_seconds",
                                     "child_system_seconds", "max_child_rss_kib", "returncode",
                                     "wrapper_signal"})
        self.assertEqual(report["schema_version"], 1)
        for field in ("elapsed_seconds", "child_user_seconds", "child_system_seconds", "max_child_rss_kib"):
            self.assertGreaterEqual(report[field], 0)
        return report

    def test_streams_and_input_are_preserved_without_logging_private_arguments(self):
        source = ('import sys; print(sys.stdin.read(), end=""); '
                  'print("synthetic child diagnostic", file=sys.stderr)')
        payload = '{"Action":"pass","Test":"synthetic"}\n'
        result = self.run_child(source, input=payload, env=dict(os.environ, SYNTHETIC_PRIVATE="do-not-log"))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, payload)
        self.assertIn("synthetic child diagnostic\n", result.stderr)
        self.assertNotIn(source, result.stderr)
        self.assertNotIn("do-not-log", result.stderr + self.output.read_text())
        self.assertEqual(self.report()["returncode"], 0)
        self.assertIsNone(self.report()["wrapper_signal"])

    def test_nonzero_exit_status_is_preserved(self):
        for status in (1, 7, 126, 127, 143, 255):
            with self.subTest(status=status):
                result = self.run_child(f"import sys; sys.exit({status})")
                self.assertEqual(result.returncode, status)
                self.assertEqual(self.report()["returncode"], status)

    def test_missing_executable_is_failure(self):
        result = subprocess.run(self.command(str(self.root / "missing")), capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 127)
        self.assertEqual(self.report()["returncode"], 127)
        self.assertEqual(result.stdout, b"")

    def test_unexecutable_command_is_failure(self):
        command = self.root / "not-executable"
        command.write_text("synthetic non-executable file")
        result = subprocess.run(self.command(str(command)), capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 126)
        self.assertEqual(self.report()["returncode"], 126)

    def test_child_signal_is_preserved(self):
        for signum in (signal.SIGTERM, signal.SIGINT, signal.SIGKILL):
            with self.subTest(signal=signum):
                result = self.run_child('import os, signal; '
                                        f'signal.signal({signum}, signal.SIG_DFL); '
                                        f'os.kill(os.getpid(), {signum})'
                                        if signum != signal.SIGKILL else
                                        'import os, signal; os.kill(os.getpid(), signal.SIGKILL)')
                self.assertEqual(result.returncode, -signum)
                self.assertEqual(self.report()["returncode"], -signum)

    def test_wrapper_signals_reach_child_and_cannot_become_success(self):
        for signum in (signal.SIGTERM, signal.SIGINT):
            with self.subTest(signal=signum):
                marker = self.root / "received-signal"
                source = ('import signal, sys; from pathlib import Path; '
                          f'signal.signal({signum}, lambda s, f: '
                          '(Path(sys.argv[1]).write_text(str(s)), sys.exit(0))); '
                          'print("ready", flush=True); signal.pause()')
                child = subprocess.Popen(self.command(sys.executable, "-c", source, str(marker)),
                                         stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                try:
                    with selectors.DefaultSelector() as selector:
                        selector.register(child.stdout, selectors.EVENT_READ)
                        self.assertTrue(selector.select(10), "synthetic child did not become ready")
                    self.assertEqual(child.stdout.readline(), "ready\n")
                    child.send_signal(signum)
                    stdout, stderr = child.communicate(timeout=10)
                    self.assertEqual(child.returncode, -signum, stderr)
                    self.assertEqual(stdout, "")
                    self.assertEqual(marker.read_text(), str(signum))
                    report = self.report()
                    self.assertEqual(report["returncode"], 0)
                    self.assertEqual(report["wrapper_signal"], signum)
                finally:
                    if child.poll() is None:
                        child.kill()
                    child.communicate()

    def test_report_failure_does_not_hide_command_failure(self):
        self.output = self.root / "missing-directory" / "timing.json"
        for status, expected in ((0, 1), (7, 7)):
            with self.subTest(status=status):
                result = self.run_child(f'import sys; print("child ran"); sys.exit({status})')
                self.assertEqual(result.returncode, expected)
                self.assertEqual(result.stdout, "child ran\n")
                self.assertIn("could not write timing report", result.stderr)

    def test_child_timing_and_rss_are_recorded(self):
        result = self.run_child('import time; data = bytearray(16 * 1024 * 1024); time.sleep(.02)')
        self.assertEqual(result.returncode, 0, result.stderr)
        report = self.report()
        self.assertGreaterEqual(report["elapsed_seconds"], .02)
        self.assertGreater(report["max_child_rss_kib"], 0)

    def test_stderr_report_failure_does_not_hide_command_failure(self):
        for status, expected in ((0, 1), (7, 7)):
            with self.subTest(status=status):
                result = subprocess.run(["bash", "-c", 'exec 2>&-; exec "$@"', "synthetic",
                                         *self.command(sys.executable, "-c", f'raise SystemExit({status})')],
                                        capture_output=True, text=True, timeout=10)
                self.assertEqual(result.returncode, expected)
                self.assertEqual(self.report()["returncode"], status)

    def test_pipefail_keeps_original_failure_through_tee(self):
        target = self.root / "tests.jsonl"
        # "$@" preserves argv exactly, matching the workflow's wrapper | tee.
        result = subprocess.run(["bash", "-c", 'set -euo pipefail; "$@" | tee "$JSONL"', "synthetic",
                                 *self.command(sys.executable, "-c", 'print("synthetic"); raise SystemExit(7)')],
                                env=dict(os.environ, JSONL=str(target)), capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 7)
        self.assertEqual(result.stdout, "synthetic\n")
        self.assertEqual(target.read_text(), "synthetic\n")
        self.assertEqual(self.report()["returncode"], 7)

    def test_tee_failure_is_not_accepted(self):
        target = self.root / "missing-directory" / "tests.jsonl"
        result = subprocess.run(["bash", "-c", 'set -euo pipefail; "$@" | tee "$JSONL"', "synthetic",
                                 *self.command(sys.executable, "-c", 'print("synthetic")')],
                                env=dict(os.environ, JSONL=str(target)), capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.report()["returncode"], 0)

    def test_missing_command_is_usage_error(self):
        result = subprocess.run(self.command(), capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 2)
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
