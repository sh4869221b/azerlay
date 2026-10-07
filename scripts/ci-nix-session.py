#!/usr/bin/env python3
"""Configure and verify a job-local D-Bus/AT-SPI session from pinned binaries."""
import ast
import os
from pathlib import Path
import shlex
import subprocess
import sys
import xml.etree.ElementTree as ET


def configure(state, atspi, dbus, bash, python, helper):
    state, atspi, dbus = Path(state), Path(atspi), Path(dbus)
    launcher = atspi / 'libexec/.at-spi-bus-launcher-wrapped'
    # makeWrapper at this locked nixpkgs revision uses this exact hidden path.
    # Do not fall back to its public wrapper: that prepends host /usr/bin.
    required = [launcher, atspi / 'libexec/at-spi2-registryd',
                dbus / 'bin/dbus-daemon', dbus / 'bin/dbus-run-session',
                Path(bash), Path(python)]
    if any(not p.is_file() or not os.access(p, os.X_OK) for p in required):
        raise ValueError('Missing pinned D-Bus/AT-SPI runtime executable')
    if not state.is_absolute():
        raise ValueError('CI_NIX_STATE must be absolute')
    services = state / 'dbus-services'
    services.mkdir(parents=True, exist_ok=True)
    # The original service also has a SystemdService key. CI uses activation by
    # this private bus only; no host systemd user service participates.
    (services / 'org.a11y.Bus.service').write_text(
        '[D-BUS Service]\nName=org.a11y.Bus\nExec=' + str(launcher) + '\n')
    config = ET.Element('busconfig')
    ET.SubElement(config, 'type').text = 'session'
    ET.SubElement(config, 'listen').text = 'unix:tmpdir=/tmp'
    ET.SubElement(config, 'auth').text = 'EXTERNAL'
    ET.SubElement(config, 'servicedir').text = str(services)
    policy = ET.SubElement(config, 'policy', context='default')
    ET.SubElement(policy, 'allow', send_destination='*', eavesdrop='true')
    ET.SubElement(policy, 'allow', eavesdrop='true')
    ET.SubElement(policy, 'allow', own='*')
    config_path = state / 'dbus-session.conf'
    ET.ElementTree(config).write(config_path, encoding='utf-8', xml_declaration=True)
    shim = state / 'session-bin/dbus-run-session'
    shim.parent.mkdir(parents=True, exist_ok=True)
    command = [str(dbus / 'bin/dbus-run-session'),
               '--dbus-daemon=' + str(dbus / 'bin/dbus-daemon'),
               '--config-file=' + str(config_path), '--',
               str(python), str(helper), '--check-and-run']
    shim.write_text('#!' + str(bash) + '\nset -euo pipefail\nexec ' +
                    shlex.join(command) + ' "$@"\n')
    shim.chmod(0o755)


def check_accessibility():
    # Probe in the same bus session that will run the Go tests. Presence of a
    # .service file is not enough: GetAddress must launch a real accessibility
    # bus, and its registry service must answer on that bus.
    result = subprocess.run([
        'gdbus', 'call', '--session', '--timeout', '10',
        '--dest', 'org.a11y.Bus', '--object-path', '/org/a11y/bus',
        '--method', 'org.a11y.Bus.GetAddress',
    ], check=True, text=True, stdout=subprocess.PIPE, timeout=15)
    value = ast.literal_eval(result.stdout.strip())
    if not isinstance(value, tuple) or len(value) != 1 or not isinstance(value[0], str) or not value[0].startswith('unix:'):
        raise ValueError('AT-SPI did not return a local accessibility bus address')
    subprocess.run([
        'gdbus', 'call', '--address', value[0], '--timeout', '10',
        '--dest', 'org.a11y.atspi.Registry',
        '--object-path', '/org/a11y/atspi/registry',
        '--method', 'org.freedesktop.DBus.Peer.Ping',
    ], check=True, stdout=subprocess.PIPE, timeout=15)
    print('Pinned AT-SPI accessibility bus and registry are responsive', file=sys.stderr, flush=True)


def main():
    if sys.argv[1:2] == ['--check-and-run']:
        command = sys.argv[2:]
        if command[:1] == ['--']:
            command = command[1:]
        if not command:
            raise ValueError('A session command is required')
        check_accessibility()
        os.execvpe(command[0], command, os.environ)
    elif len(sys.argv) == 1:
        configure(os.environ['CI_NIX_STATE'], os.environ['AZERLAY_NIX_ATSPI'],
                  os.environ['AZERLAY_NIX_DBUS'], os.environ['AZERLAY_NIX_BASH'],
                  sys.executable, Path(__file__).resolve())
    else:
        raise ValueError('Unexpected session-helper arguments')


if __name__ == '__main__':
    main()
