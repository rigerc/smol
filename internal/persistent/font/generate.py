#!/usr/bin/env python3
"""Regenerate the bundled glyph atlas; runtime uses only Go's standard library.

Requires Pillow, fonttools, ttf-dejavu, noto-fonts and noto-fonts-cjk.
Run from the repository root: python internal/persistent/font/generate.py
"""
import glob
import pathlib
import subprocess

from PIL import Image, ImageDraw, ImageFont
from fontTools.ttLib import TTFont

DEST = pathlib.Path(__file__).parent
cache = subprocess.check_output(['go', 'env', 'GOMODCACHE'], text=True).strip()
version = subprocess.check_output(
    ['go', 'list', '-m', '-f', '{{.Version}}', 'github.com/rigerc/faceroll/go'], text=True
).strip()
source = pathlib.Path(cache) / ('github.com/rigerc/faceroll/go@' + version)
chars = set(chr(c) for c in range(32, 127))
for file in source.glob('*.go'):
    chars.update(c for c in file.read_text() if ord(c) >= 160)
fonts = ['/usr/share/fonts/TTF/DejaVuSansMono.ttf',
         '/usr/share/fonts/TTF/DejaVuSans.ttf',
         '/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc']
fonts += sorted(glob.glob('/usr/share/fonts/noto/NotoSans*-Regular.ttf'))
fonts += ['/usr/share/fonts/noto/NotoSerifTibetan-Regular.ttf']
loaded = []
for path in fonts:
    data = TTFont(path, fontNumber=0, lazy=True)
    loaded.append((path, set(data.getBestCmap()), ImageFont.truetype(path, 28)))
    data.close()
chars = ''.join(sorted(chars))
canvas = Image.new('RGBA', (40 * 16, 40 * ((len(chars)+15)//16)))
draw = ImageDraw.Draw(canvas)
used = set()
for i, char in enumerate(chars):
    selected = next(((p, f) for p, cmap, f in loaded if ord(char) in cmap), None)
    if selected is None:
        raise ValueError(f'No font for U+{ord(char):04X} {char}')
    path, font = selected
    used.add(path)
    # Each tile contains the unscaled glyph, centered in a 40x40 square.
    # Runtime centers/scales it into the rune's one- or two-cell slot.
    box = draw.textbbox((0, 0), char, font=font)
    x, y = (i % 16)*40, (i // 16)*40
    draw.text((x+20-(box[0]+box[2])/2, y+20-(box[1]+box[3])/2),
              char, font=font, fill='white')
canvas.save(DEST / 'atlas.png', optimize=True)
(DEST / 'chars.txt').write_text(chars)
(DEST / 'SOURCES.txt').write_text(
    'Faceroll: '+version+'\nGlyph tiles: 40x40, 16 columns, 28px source fonts.\n'
    'See generate.py for regeneration.\n\n'+
    '\n'.join(pathlib.Path(p).name for p in sorted(used))+'\n')
print(f'{len(chars)} glyphs, {len(used)} source fonts, {(DEST / "atlas.png").stat().st_size} bytes')
