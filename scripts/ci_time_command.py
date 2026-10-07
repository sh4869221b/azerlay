#!/usr/bin/env python3
"""Time one Linux CI command without capturing its streams or hiding failures."""
import argparse
import json
import os
from pathlib import Path
import resource
import signal
import subprocess
import sys
import time


def diagnostic(message):
    """A missing/broken stderr must not replace the command's failure status."""
    encoded = ("ci timing: " + message + "\n").encode()
    try:
        return os.write(2, encoded) == len(encoded)
    except OSError:
        return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, help="timing JSON file, separate from command stdout")
    parser.add_argument("command", nargs=argparse.REMAINDER, help="-- COMMAND [ARG...]")
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("a command is required")
    if not sys.platform.startswith("linux"):
        parser.error("Linux is required for the documented maximum-child RSS units")

    child = None
    received_signal = None

    def forward(signum, _frame):
        nonlocal received_signal
        if received_signal is None:
            received_signal = signum
        if child is not None:
            # Keep the existing process group and launcher's signal handling.
            # runuser in the native command forwards termination to its child.
            try:
                child.send_signal(signum)
            except ProcessLookupError:
                pass

    for signum in (signal.SIGINT, signal.SIGTERM):
        signal.signal(signum, forward)
    started = time.monotonic()
    try:
        # Inherit stdin/stdout/stderr; only the caller's tee captures test JSON.
        # Do not log argv, environment, working directory, or private paths.
        child = subprocess.Popen(command)
        if received_signal is not None:
            forward(received_signal, None)
        returncode = child.wait()
    except FileNotFoundError:
        diagnostic("command executable not found")
        returncode = 127
    except OSError:
        diagnostic("command could not be started")
        returncode = 126
    elapsed = time.monotonic() - started
    usage = resource.getrusage(resource.RUSAGE_CHILDREN)
    report = {
        "schema_version": 1,
        "elapsed_seconds": elapsed,
        "child_user_seconds": usage.ru_utime,
        "child_system_seconds": usage.ru_stime,
        # Linux reports KiB. This is the largest child high-water mark,
        # including resource usage propagated from waited-for descendants,
        # NOT the simultaneous peak of the whole process tree.
        "max_child_rss_kib": usage.ru_maxrss,
        "returncode": returncode,
        "wrapper_signal": received_signal,
    }
    report_failed = False
    try:
        encoded = json.dumps(report, sort_keys=True)
        Path(args.output).write_text(encoded + "\n")
        report_failed = not diagnostic(encoded)
    except OSError:
        # A diagnostics failure must not turn a failing test command green or
        # replace its original nonzero status. A successful command fails closed.
        report_failed = True
        diagnostic("could not write timing report")

    signum = received_signal or (-returncode if returncode < 0 else None)
    if signum is not None:
        if signum not in (signal.SIGKILL, signal.SIGSTOP):
            signal.signal(signum, signal.SIG_DFL)
        os.kill(os.getpid(), signum)
        return 128 + signum  # Fallback if the platform does not terminate us.
    return returncode or int(report_failed)


if __name__ == "__main__":
    sys.exit(main())
