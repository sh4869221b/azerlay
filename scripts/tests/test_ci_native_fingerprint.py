"""Offline regression tests for installed Arch build dependency identities."""
import importlib.util
import shutil
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
        # These fixtures use a small explicit ordering. Production delegates
        # Arch version semantics to vercmp, covered separately below.
        versions = {'1': 1, '2': 2, '3-64': 3, '4-64': 4, '99': 99}
        comparison = mock.patch.object(packages, 'version_compare',
                                       side_effect=lambda a, b: (versions[a] > versions[b]) -
                                       (versions[a] < versions[b]))
        comparison.start()
        self.addCleanup(comparison.stop)
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

    def test_openssl_abis_select_only_satisfying_providers(self):
        # The rolling Arch OpenSSL 4 migration installs both packages. They
        # provide the same library name but incompatible SONAME versions.
        write_package(self.database, 'openssl', version='4.1.0-1',
                      provides=('libcrypto.so=4-64', 'libssl.so=4-64'))
        write_package(self.database, 'openssl-3', version='3.5.9-2',
                      provides=('libcrypto.so=3-64', 'libssl.so=3-64'))
        for abi, expected in (('3-64', 'openssl-3'), ('4-64', 'openssl')):
            with self.subTest(abi=abi):
                write_package(self.database, 'graphics',
                              depends=(f'libcrypto.so={abi}', f'libssl.so={abi}'),
                              provides=('libgraphics=1',))
                names = {row[0] for row in packages.inventory(self.database, self.roots)}
                self.assertEqual(names & {'openssl', 'openssl-3'}, {expected})

        write_package(self.database, 'graphics',
                      depends=('libcrypto.so=3-64', 'libcrypto.so=4-64'),
                      provides=('libgraphics=1',))
        names = {row[0] for row in packages.inventory(self.database, self.roots)}
        self.assertTrue({'openssl', 'openssl-3'} <= names)

    def test_unversioned_provision_does_not_satisfy_versioned_dependency(self):
        write_package(self.database, 'graphics', version='99', provides=('libgraphics',))
        with self.assertRaisesRegex(ValueError, 'libgraphics>=1'):
            packages.inventory(self.database, self.roots)
        write_package(self.database, 'gtk4', depends=('libgraphics',))
        self.assertIn('graphics', [row[0] for row in packages.inventory(self.database, self.roots)])

    def test_provided_version_not_owner_version_controls_matching(self):
        write_package(self.database, 'graphics', version='99', provides=('libgraphics=1',))
        write_package(self.database, 'gtk4', depends=('libgraphics>=2',))
        with self.assertRaisesRegex(ValueError, 'libgraphics>=2'):
            packages.inventory(self.database, self.roots)

    def test_direct_package_must_satisfy_constraint_before_taking_precedence(self):
        write_package(self.database, 'libgraphics', version='2')
        names = {row[0] for row in packages.inventory(self.database, self.roots)}
        self.assertIn('libgraphics', names)
        self.assertNotIn('graphics', names)
        write_package(self.database, 'gtk4', depends=('libgraphics=1',))
        names = {row[0] for row in packages.inventory(self.database, self.roots)}
        self.assertIn('graphics', names)
        self.assertNotIn('libgraphics', names)

    def test_selected_package_cannot_hide_an_unsatisfied_requirement(self):
        write_package(self.database, 'gtk4', depends=('glibc>=2', 'glibc'))
        with self.assertRaisesRegex(ValueError, 'glibc>=2'):
            packages.inventory(self.database, self.roots)

    def test_duplicate_matching_provisions_count_as_one_provider(self):
        write_package(self.database, 'graphics', provides=('libgraphics=1', 'libgraphics=2'))
        names = {row[0] for row in packages.inventory(self.database, self.roots)}
        self.assertIn('graphics', names)

    def test_unversioned_and_multi_match_dependencies_remain_ambiguous(self):
        write_package(self.database, 'other-graphics', provides=('libgraphics=2',))
        for requirement in ('libgraphics', 'libgraphics>=1'):
            with self.subTest(requirement=requirement):
                write_package(self.database, 'gtk4', depends=(requirement,))
                with self.assertRaisesRegex(ValueError, 'Missing or ambiguous'):
                    packages.inventory(self.database, self.roots)

    def test_all_comparison_operators(self):
        for operator, expected in (('=', {2}), ('>=', {2, 99}), ('<=', {1, 2}),
                                   ('>', {99}), ('<', {1})):
            for actual in (1, 2, 99):
                with self.subTest(operator=operator, actual=actual):
                    self.assertEqual(packages.satisfies(str(actual), operator, '2'),
                                     actual in expected)
    def test_invalid_dependencies_and_provisions_fail_closed(self):
        for requirement in ('', '=1', 'graphics>=', 'graphics==1', 'graphics<>1'):
            with self.subTest(requirement=requirement):
                with self.assertRaises(ValueError):
                    packages.dependency(requirement)
        write_package(self.database, 'graphics', provides=('libgraphics>=1',))
        with self.assertRaisesRegex(ValueError, 'Invalid installed package provision'):
            packages.inventory(self.database, self.roots)

    def test_library_roots_follow_pkg_config_file_owners(self):
        paths = [f'/usr/lib/pkgconfig/{name}.pc' for name in packages.LIBRARY_MODULES]
        owners = ['gtk4', 'gtk4-layer-shell', 'glib2', 'pango', 'girepository', 'cairo']
        results = [subprocess.CompletedProcess([], 0, value + '\n')
                   for pair in zip(paths, owners) for value in pair]
        with mock.patch.object(packages.subprocess, 'run', side_effect=results) as run:
            self.assertEqual(packages.native_roots(), (*packages.ROOTS, *owners))
        for index, (module, path) in enumerate(zip(packages.LIBRARY_MODULES, paths)):
            self.assertEqual(run.call_args_list[2 * index].args[0], ['pkg-config', '--path', module])
            self.assertEqual(run.call_args_list[2 * index + 1].args[0], ['pacman', '-Qoq', '--', path])

    def test_root_queries_handle_single_result_batch_queries_and_shared_owners(self):
        def query(args, **kwargs):
            if args[0] == 'pkg-config':
                # pkgconf 3.0.7 returns only the first path in a batch.
                # Individual queries retain every requested identity.
                output = '/usr/lib/pkgconfig/' + args[2] + '.pc\n'
            else:
                output = 'shared-development-package\n'
            return subprocess.CompletedProcess(args, 0, output)
        with mock.patch.object(packages.subprocess, 'run', side_effect=query):
            self.assertEqual(packages.native_roots(),
                             (*packages.ROOTS, *(['shared-development-package'] * len(packages.LIBRARY_MODULES))))

    def test_root_discovery_fails_on_missing_files_owners_or_command_failure(self):
        paths = '/usr/lib/pkgconfig/gtk4.pc'
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


class VersionComparisonTests(unittest.TestCase):
    def test_arch_versions_are_passed_unchanged_to_vercmp(self):
        with mock.patch.object(packages.subprocess, 'run',
                               return_value=subprocess.CompletedProcess([], 0, '1\n')) as run:
            self.assertEqual(packages.version_compare('2:1.0rc1-3', '1:99.0-9'), 1)
        run.assert_called_once_with(['vercmp', '2:1.0rc1-3', '1:99.0-9'],
                                    check=True, capture_output=True, text=True)

    def test_vercmp_failure_and_invalid_output_fail_closed(self):
        for error in (FileNotFoundError(), subprocess.CalledProcessError(1, 'vercmp')):
            with self.subTest(error=error), mock.patch.object(packages.subprocess, 'run', side_effect=error):
                with self.assertRaises(type(error)):
                    packages.version_compare('1', '2')
        for output in ('', 'unexpected', '2'):
            with self.subTest(output=output), mock.patch.object(
                    packages.subprocess, 'run', return_value=subprocess.CompletedProcess([], 0, output)):
                with self.assertRaises(ValueError):
                    packages.version_compare('1', '2')

    @unittest.skipUnless(shutil.which('vercmp'), 'Arch vercmp is not installed')
    def test_real_arch_version_semantics(self):
        for actual, required, expected in (('3-64', '4-64', -1), ('4-64', '3-64', 1),
                                           ('2:1-1', '1:99-9', 1), ('1.0-2', '1.0-1', 1),
                                           ('1.0-2', '1.0', 0), ('1.10', '1.9', 1),
                                           ('1.0rc1', '1.0', -1)):
            with self.subTest(actual=actual, required=required):
                self.assertEqual(packages.version_compare(actual, required), expected)


if __name__ == '__main__':
    unittest.main()
