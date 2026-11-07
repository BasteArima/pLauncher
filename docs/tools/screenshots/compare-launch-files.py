"""Compare the three right-column cards at equal displayed width; no app CSS overrides."""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

here = Path(__file__).resolve().parent
out = here / 'out'
actual = Image.open(out / 'launch-files-right.png').convert('RGB')
board = Image.open(here.parents[1] / 'design' / 'game-page.png').convert('RGB')
# Preserve the mockup's card sizes, offsets and gaps, excluding unrelated left-column text.
reference = Image.new('RGB', (266, 715), '#0d0f13')
for box in [(666, 74, 932, 446), (697, 468, 932, 663), (697, 676, 932, 789)]:
    reference.paste(board.crop(box), (box[0] - 666, box[1] - 74))
width = actual.width
reference = reference.resize((width, round(reference.height * width / reference.width)), Image.Resampling.LANCZOS)
comparison = Image.new('RGB', (2 * width + 24, max(actual.height, reference.height) + 40), '#0d0f13')
comparison.paste(reference, (0, 40))
comparison.paste(actual, (width + 24, 40))
draw = ImageDraw.Draw(comparison)
try:
    font = ImageFont.truetype('arial.ttf', 16)
except OSError:
    font = ImageFont.load_default()
draw.text((8, 10), 'Mockup (same column width)', fill='white', font=font)
draw.text((width + 32, 10), 'Mock UI', fill='white', font=font)
comparison.save(out / 'launch-files-comparison.png')
print(f'Columns: {width}px; heights: mockup {reference.height}px, app {actual.height}px')
