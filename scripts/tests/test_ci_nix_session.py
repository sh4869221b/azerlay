import importlib.util
import io
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('ci_nix_session', ROOT / 'scripts/ci-nix-session.py')
session = importlib.util.module_from_spec(spec)
spec.loader.exec_module(session)


class NixSessionTests(unittest.TestCase):
    def make_config(self, root):
        atspi, dbus = root / 'atspi', root / 'dbus'
        for path in (atspi / 'libexec/.at-spi-bus-launcher-wrapped',
                     atspi / 'libexec/at-spi2-registryd',
                     dbus / 'bin/dbus-daemon', dbus / 'bin/dbus-run-session'):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text('#!/bin/sh\nexit 0\n')
            path.chmod(0o755)
        state = root / 'state with spaces & XML'
        session.configure(state, atspi, dbus, '/bin/bash', sys.executable, spec.origin)
        return state, atspi, dbus

    def test_only_pinned_accessibility_service_is_activated(self):
        with tempfile.TemporaryDirectory() as temp:
            state, atspi, dbus = self.make_config(Path(temp))
            config = ET.parse(state / 'dbus-session.conf').getroot()
            self.assertEqual(config.findtext('servicedir'), str(state / 'dbus-services'))
            self.assertIsNone(config.find('include'))
            self.assertIsNone(config.find('includedir'))
            self.assertIsNone(config.find('standard_session_servicedirs'))
            service = (state / 'dbus-services/org.a11y.Bus.service').read_text()
            self.assertIn('Exec=' + str(atspi / 'libexec/.at-spi-bus-launcher-wrapped'), service)
            self.assertNotIn('SystemdService', service)
            shim = (state / 'session-bin/dbus-run-session').read_text()
            self.assertIn('--dbus-daemon=' + str(dbus / 'bin/dbus-daemon'), shim)
            subprocess.run(['bash', '-n', state / 'session-bin/dbus-run-session'], check=True)

    def test_missing_unwrapped_launcher_fails_without_host_fallback(self):
        with tempfile.TemporaryDirectory() as temp:
            state, atspi, dbus = self.make_config(Path(temp))
            (atspi / 'libexec/.at-spi-bus-launcher-wrapped').unlink()
            with self.assertRaisesRegex(ValueError, 'Missing pinned'):
                session.configure(state, atspi, dbus, '/bin/bash', sys.executable, spec.origin)

    def test_probe_requires_bus_and_registry_responses(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with mock.patch.object(session.subprocess, 'run', side_effect=[
            subprocess.CompletedProcess([], 0, "('unix:path=/tmp/private-atspi',)\n"),
            subprocess.CompletedProcess([], 0, '()\n'),
        ]) as run, mock.patch('sys.stdout', stdout), mock.patch('sys.stderr', stderr):
            session.check_accessibility()
        self.assertEqual(run.call_count, 2)
        self.assertIn('org.a11y.Bus.GetAddress', run.call_args_list[0].args[0])
        self.assertIn('org.a11y.atspi.Registry', run.call_args_list[1].args[0])
        self.assertIn('unix:path=/tmp/private-atspi', run.call_args_list[1].args[0])
        self.assertEqual(stdout.getvalue(), '')
        self.assertIn('registry are responsive', stderr.getvalue())

    def test_probe_failure_blocks_the_command(self):
        with mock.patch.object(session.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, ['gdbus'])):
            with mock.patch('sys.argv', [spec.origin, '--check-and-run', '--', 'true']), mock.patch.object(session.os, 'execvpe') as execute:
                with self.assertRaises(subprocess.CalledProcessError):
                    session.main()
                execute.assert_not_called()

    def test_probe_rejects_nonlocal_or_malformed_address(self):
        for value in ('()', "('tcp:host=example.com',)", "('unix:path=x', 'extra')", '[1]'):
            with self.subTest(value=value), mock.patch.object(session.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, value)):
                with self.assertRaises(ValueError):
                    session.check_accessibility()

    def test_command_and_exit_semantics_are_preserved(self):
        with mock.patch.object(session, 'check_accessibility'), mock.patch.object(session.os, 'execvpe') as execute:
            with mock.patch('sys.argv', [spec.origin, '--check-and-run', '--', 'test-command', 'arg with spaces']):
                session.main()
        execute.assert_called_once_with('test-command', ['test-command', 'arg with spaces'], os.environ)

    def test_wrapper_keeps_accessibility_enabled(self):
        source = (ROOT / 'scripts/ci-nix-run.sh').read_text()
        self.assertIn('export GTK_A11Y=atspi ATSPI_DBUS_IMPLEMENTATION=dbus-daemon', source)
        self.assertIn('AT_SPI_BUS_ADDRESS NO_AT_BRIDGE', source)
        self.assertNotIn('GTK_A11Y=none', source)
        self.assertIn('ci-nix-session.py', (ROOT / 'scripts/ci-nix-fingerprint.sh').read_text())

    @unittest.skipUnless(shutil.which('dbus-run-session') and shutil.which('dbus-send'), 'Host D-Bus tools unavailable')
    def test_generated_config_starts_a_real_private_session(self):
        # This verifies config syntax/permissions using host D-Bus. It does not
        # claim that the pinned Nix closure or accessibility runtime was run.
        with tempfile.TemporaryDirectory() as temp:
            state, _, _ = self.make_config(Path(temp))
            result = subprocess.run([
                'dbus-run-session', '--config-file=' + str(state / 'dbus-session.conf'), '--',
                'dbus-send', '--session', '--print-reply', '--dest=org.freedesktop.DBus',
                '/org/freedesktop/DBus', 'org.freedesktop.DBus.ListNames',
            ], capture_output=True, text=True, timeout=15)
            if result.returncode == 127 and 'Failed to open socket: Operation not permitted' in result.stderr:
                self.skipTest('Executor sandbox prohibits private D-Bus sockets')
            result.check_returncode()
            self.assertIn('org.freedesktop.DBus', result.stdout)


if __name__ == '__main__':
    unittest.main()
