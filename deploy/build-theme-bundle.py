#!/usr/bin/env python3
"""Package two trusted stock Glass source trees for an offline upgrade."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sys
import uuid
import zipfile

from safe_upgrade import safe_tree


def inventory(root):
    if root.name != 'Glass':
        raise ValueError('source directory must be named Glass')
    safe_tree(root)
    files = sorted((p for p in root.rglob('*') if p.is_file()), key=lambda p: p.relative_to(root).as_posix())
    if not files or len(files) > 4096 or sum(p.stat().st_size for p in files) > 512 * 1024 * 1024:
        raise ValueError('empty or excessively large theme source')
    for required in ('komari-theme.json', 'dist/index.html'):
        if not (root/required).is_file():
            raise ValueError('missing source file: ' + required)
    json.loads((root/'komari-theme.json').read_text())
    return files


def build(old, new, output):
    if old.resolve() == new.resolve() or output.resolve().is_relative_to(old.resolve()) or output.resolve().is_relative_to(new.resolve()):
        raise ValueError('distinct stock sources and an external output directory are required')
    old_files, new_files = inventory(old), inventory(new)
    stock = {p.relative_to(old).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest() for p in old_files}
    if not output.parent.is_dir() or output.parent.is_symlink():
        raise ValueError('output parent must be an existing non-symlink directory')
    temporary = output.with_name('.' + output.name + '.' + uuid.uuid4().hex)
    try:
        with open(temporary, 'xb') as f:
            os.chmod(temporary, 0o600)
            with zipfile.ZipFile(f, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
                archive.writestr('stock.json', json.dumps(stock, sort_keys=True) + '\n')
                for path in new_files:
                    archive.write(path, 'Glass/' + path.relative_to(new).as_posix())
            f.flush(); os.fsync(f.fileno())
        # Hard-link creation is atomic and refuses to replace an approved bundle.
        os.link(temporary, output)
        fd = os.open(output.parent, os.O_RDONLY | os.O_DIRECTORY)
        try: os.fsync(fd)
        finally: os.close(fd)
    finally:
        temporary.unlink(missing_ok=True)
    print(json.dumps({'bundle': str(output), 'sha256': hashlib.sha256(output.read_bytes()).hexdigest(),
                      'old_files': len(old_files), 'new_files': len(new_files)}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--old', required=True, type=Path, help='trusted prior release bundledThemes/Glass')
    parser.add_argument('--new', required=True, type=Path, help='trusted candidate release bundledThemes/Glass')
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    try:
        build(args.old, args.new, args.output)
    except (OSError, ValueError, RuntimeError, zipfile.BadZipFile) as exc:
        print('theme-bundle: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__': sys.exit(main())
