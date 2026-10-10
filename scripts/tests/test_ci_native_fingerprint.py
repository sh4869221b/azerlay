"""Offline regression tests for installed Arch build dependency identities."""
import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('ci_native_packages', ROOT / 'scripts/ci_native_packages.py')
packages = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packages)


def write_package(database, name, version='1', arch='x86_64', depends=(), provides=(), optional=()):
    directory = database / name
    directory.mkdir(exist_ok=True)
    fields = {'NAME': [name], 'VERSION': [version], 'ARCH': [arch],
              'DEPENDS': depends, 'PROVIDES': provides, 'OPTDEPENDS': optional}
    (directory / 'desc').write_text('\n\n'.join('%' + key + '%\n' + '\n'.join(values)
                                               for key, values in fields.items()) + '\n')


class NativeFingerprintTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.database = Path(self.directory.name)
        self.roots = (*packages.ROOTS, 'gtk4', 'gtk4-layer-shell', 'girepository')
        for name in self.roots:
            write_package(self.database, name)
        write_package(self.database, 'gtk4', depends=('libgraphics>=1',), optional=('fonts: optional',))
        write_package(self.database, 'graphics', depends=('glibc',), provides=('libgraphics=1',))
        write_package(self.database, 'sway')
        write_package(self.database, 'fonts')
        write_package(self.database, 'curl')

    def test_roots_transitive_dependencies_and_providers_are_sorted(self):
        rows = packages.inventory(self.database, self.roots)
        self.assertEqual([row[0] for row in rows], sorted([*self.roots, 'graphics']))
        self.assertTrue(all(row[1:] == ('1', 'x86_64') for row in rows))

    def test_native_updates_invalidate_including_transitive_abi_and_arch(self):
        original = packages.inventory(self.database, self.roots)
        for name in (*self.roots, 'graphics'):
            with self.subTest(package=name):
                file = self.database / name / 'desc'
                saved = file.read_text()
                file.write_text(saved.replace('%VERSION%\n1', '%VERSION%\n2'))
                self.assertNotEqual(original, packages.inventory(self.database, self.roots))
                file.write_text(saved.replace('%ARCH%\nx86_64', '%ARCH%\naarch64'))
                self.assertNotEqual(original, packages.inventory(self.database, self.roots))
                file.write_text(saved)

    def test_unrelated_and_optional_packages_do_not_invalidate(self):
        original = packages.inventory(self.database, self.roots)
        for name in ('sway', 'fonts', 'curl', 'syft', 'pacman', 'gobject-introspection'):
            write_package(self.database, name, version='2')
        self.assertEqual(original, packages.inventory(self.database, self.roots))

    def test_missing_and_ambiguous_dependencies_fail_closed(self):
        (self.database / 'graphics' / 'desc').unlink()
        with self.assertRaisesRegex(ValueError, 'Missing or ambiguous'):
            packages.inventory(self.database, self.roots)
        write_package(self.database, 'provider-a', provides=('libgraphics=1',))
        write_package(self.database, 'provider-b', provides=('libgraphics=1',))
        with self.assertRaisesRegex(ValueError, 'Missing or ambiguous'):
            packages.inventory(self.database, self.roots)

    def test_library_roots_follow_pkg_config_file_owners(self):
        paths = [f'/usr/lib/pkgconfig/{name}.pc' for name in packages.LIBRARY_MODULES]
        owners = ['gtk4', 'gtk4-layer-shell', 'glib2', 'pango', 'girepository', 'cairo']
        with mock.patch.object(packages.subprocess, 'run', side_effect=[
                subprocess.CompletedProcess([], 0, '\n'.join(paths) + '\n'),
                subprocess.CompletedProcess([], 0, '\n'.join(owners) + '\n')]) as run:
            self.assertEqual(packages.native_roots(), (*packages.ROOTS, *owners))
        self.assertEqual(run.call_args_list[0].args[0], ['pkg-config', '--path', *packages.LIBRARY_MODULES])
        self.assertEqual(run.call_args_list[1].args[0], ['pacman', '-Qoq', '--', *paths])

    def test_root_discovery_fails_on_missing_files_owners_or_command_failure(self):
        paths = '\n'.join('/usr/lib/pkgconfig/' + name + '.pc' for name in packages.LIBRARY_MODULES)
        for results in ([subprocess.CompletedProcess([], 0, '')],
                        [subprocess.CompletedProcess([], 0, paths), subprocess.CompletedProcess([], 0, '')]):
            with mock.patch.object(packages.subprocess, 'run', side_effect=results):
                with self.assertRaises(ValueError):
                    packages.native_roots()
        with mock.patch.object(packages.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'pkg-config')):
            with self.assertRaises(subprocess.CalledProcessError):
                packages.native_roots()

    def test_dependency_cycles_are_finite(self):
        write_package(self.database, 'graphics', depends=('gtk4',), provides=('libgraphics=1',))
        self.assertEqual(len(packages.inventory(self.database, self.roots)), len(self.roots) + 1)


if __name__ == '__main__':
    unittest.main()
