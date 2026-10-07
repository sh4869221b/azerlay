import ctypes.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[2]
# Exercise the actual small configuration writer without needing Nix/Go or
# bypassing ci-nix-run's pinned-compiler checks.
FONT_WRITER = (ROOT / 'scripts/ci-nix-run.sh').read_text().split("python3 - <<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
HOST_RULE = Path('/usr/share/fontconfig/conf.avail/10-scale-bitmap-fonts.conf')
HOST_FONTS = [Path('/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc'),
              Path('/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf')]

# Match prepareLine's Sans/14px, 248px-wide, single-paragraph, ellipsized label.
# This is a host-library regression, not a claim of exact Nix runtime coverage.
PANGO_PROBE = r'''
import ctypes as C
import ctypes.util
import json
pc = C.CDLL(ctypes.util.find_library('pangocairo-1.0'))
p = C.CDLL(ctypes.util.find_library('pango-1.0'))
c = C.CDLL(ctypes.util.find_library('cairo'))
def call(lib, name, args, result, *values):
    fn = getattr(lib, name)
    fn.argtypes, fn.restype = args, result
    return fn(*values)
ptr, integer = C.c_void_p, C.c_int
surface = call(c, 'cairo_image_surface_create', [integer] * 3, ptr, 0, 1, 1)
context = call(c, 'cairo_create', [ptr], ptr, surface)
layout = call(pc, 'pango_cairo_create_layout', [ptr], ptr, context)
font = call(p, 'pango_font_description_from_string', [C.c_char_p], ptr, b'Sans')
call(p, 'pango_font_description_set_absolute_size', [ptr, C.c_double], None, font, 14 * 1024)
call(p, 'pango_layout_set_font_description', [ptr, ptr], None, layout, font)
call(p, 'pango_layout_set_single_paragraph_mode', [ptr, integer], None, layout, True)
call(p, 'pango_layout_set_ellipsize', [ptr, integer], None, layout, 3)
call(p, 'pango_layout_set_width', [ptr, integer], None, layout, 248 * 1024)
text = '日本語 e\u0301 👾 <b>literal</b> next part 長いラベル長いラベル長いラベル'.encode()
call(p, 'pango_layout_set_text', [ptr, C.c_char_p, integer], None, layout, text, -1)
width, height = integer(), integer()
call(p, 'pango_layout_get_pixel_size', [ptr, C.POINTER(integer), C.POINTER(integer)], None,
     layout, C.byref(width), C.byref(height))
unknown = call(p, 'pango_layout_get_unknown_glyphs_count', [ptr], integer, layout)
print(json.dumps({'width': width.value, 'height': height.value, 'unknown': unknown}))
'''


class NixFontTests(unittest.TestCase):
    def write_config(self, directory, rules, fonts=()):
        env = dict(os.environ, FONTCONFIG_FILE=str(directory / 'fonts.conf'),
                   FONTCONFIG_PATH=str(directory), XDG_CACHE_HOME=str(directory / 'cache'),
                   AZERLAY_NIX_FONT_DIRS=':'.join(map(str, fonts)),
                   AZERLAY_NIX_FONT_RULES=':'.join(map(str, rules)))
        subprocess.run([sys.executable, '-c', FONT_WRITER], env=env, check=True, capture_output=True)
        return env

    def test_pinned_standard_rule_is_test_role_only(self):
        flake = (ROOT / 'flake.nix').read_text()
        self.assertIn('AZERLAY_NIX_FONT_RULES = lib.optionalString withGUI "${pkgs.fontconfig.out}/share/fontconfig/conf.avail/10-scale-bitmap-fonts.conf";', flake)

    def test_writer_includes_only_explicit_rules_and_font_directories(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            rule = root / '10-scale-bitmap-fonts.conf'
            rule.write_text('<fontconfig/>')
            self.write_config(root, [rule], [root / 'pinned-fonts'])
            config = ET.parse(root / 'fonts.conf').getroot()
            self.assertEqual([x.text for x in config.findall('include')], [str(rule)])
            self.assertEqual(config.find('include').get('ignore_missing'), 'no')
            self.assertEqual([x.text for x in config.findall('dir')], [str(root / 'pinned-fonts')])
            self.assertNotIn('/etc/fonts', (root / 'fonts.conf').read_text())

    def test_missing_scaling_rule_fails_closed(self):
        with tempfile.TemporaryDirectory() as temp:
            with self.assertRaises(subprocess.CalledProcessError):
                self.write_config(Path(temp), [Path(temp) / 'missing.conf'])

    def test_build_role_needs_no_font_rules(self):
        with tempfile.TemporaryDirectory() as temp:
            self.write_config(Path(temp), [])
            self.assertEqual(ET.parse(Path(temp) / 'fonts.conf').getroot().findall('include'), [])

    @unittest.skipUnless(HOST_RULE.is_file() and all(p.is_file() for p in HOST_FONTS)
                         and all(ctypes.util.find_library(x) for x in ('pango-1.0', 'pangocairo-1.0', 'cairo')),
                         'Host Pango/Cairo and Noto CJK/emoji regression fixtures unavailable')
    def test_bitmap_scaling_keeps_unicode_label_inside_renderer_box(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            # Limit discovery to the same two font families as the Nix shell.
            font_dirs = []
            for i, font in enumerate(HOST_FONTS):
                target = root / str(i)
                target.mkdir()
                (target / font.name).symlink_to(font)
                font_dirs.append(target)
            dimensions = []
            for rules in ([], [HOST_RULE]):
                env = self.write_config(root, rules, font_dirs)
                result = subprocess.run([sys.executable, '-c', PANGO_PROBE],
                                        env=env, check=True, capture_output=True, text=True)
                dimensions.append(json.loads(result.stdout))
            bare, fixed = dimensions
            self.assertGreater(bare['height'], 70, bare)
            self.assertGreater(fixed['height'], 0, fixed)
            self.assertLess(fixed['height'], 35, fixed)
            self.assertEqual(fixed['unknown'], 0, fixed)
            self.assertLessEqual(fixed['width'], 249, fixed)


if __name__ == '__main__':
    unittest.main()
