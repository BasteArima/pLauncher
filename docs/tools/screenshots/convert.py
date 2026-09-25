"""PNG-скриншоты → WebP для README (+ social-preview.png из баннера).

python convert.py <папка с png> <docs/images>
"""
import os
import sys

from PIL import Image

src, dst = sys.argv[1], sys.argv[2]
for name in sorted(os.listdir(src)):
    if not name.endswith('.png'):
        continue
    base = name[:-4]
    im = Image.open(os.path.join(src, name)).convert('RGB')
    im.save(os.path.join(dst, base + '.webp'), 'WEBP', quality=90 if base == 'banner' else 88, method=6)
    if base == 'banner':
        # Картинка для превью ссылок (Settings → Social preview на GitHub): PNG до 1 МБ
        im.save(os.path.join(dst, 'social-preview.png'), optimize=True)
    print(f'{base}.webp  {os.path.getsize(os.path.join(dst, base + ".webp")) // 1024} KB')
