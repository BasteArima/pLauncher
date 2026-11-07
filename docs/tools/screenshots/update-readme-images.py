"""Version README screenshot URLs by their exported content, without changing the images."""
import hashlib
from pathlib import Path
import re
import sys


def update(readme):
    original = readme.read_bytes().decode("utf-8")
    count = 0

    def version(match):
        nonlocal count
        relative = match.group(1)
        digest = hashlib.sha256((readme.parent / relative).read_bytes()).hexdigest()[:12]
        count += 1
        return f'src="{relative}?v={digest}"'

    # Restrict changes to our exported screenshot names; preserve all other README content.
    updated = re.sub(r'src="(docs/images/[\w-]+\.webp)(?:\?[^"\s]*)?"', version, original)
    if updated != original:
        readme.write_bytes(updated.encode("utf-8"))
    return count


if __name__ == "__main__":
    readme = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parents[3] / "README.md"
    print(f"README image versions: {update(readme)} links")
