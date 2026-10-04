#!/usr/bin/env python3
"""One-command GitHub Release bootstrap for install/update (Python 3 + curl/wget).

Example after independently checking the script source/tag:
  sudo python3 komari-oneclick.py update --tag vX.Y.Z
A Release without the controller scripts/verified assets fails before service changes.
"""
import argparse
from collections import namedtuple
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile

REPO = '3rnn/komari-lite'
Controller = namedtuple('Controller', 'tag manager controller binary_sha theme_sha', defaults=[None])


def require(ok, message):
    if not ok:
        raise ValueError(message)


def trusted_tool(tool):
    executable = shutil.which(tool)
    require(tool in ('curl', 'wget') and executable, 'curl or wget required')
    path = Path(executable).resolve(strict=True)
    for part in (path.parent, *path.parents):
        meta = part.lstat()
        require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                'untrusted downloader path')
    meta = path.stat()
    require(stat.S_ISREG(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
            'untrusted downloader executable')
    return str(path)


def download(url, target, tool, limit):
    require(url.startswith('https://'), 'HTTPS required')
    executable = trusted_tool(tool)
    if tool == 'curl':
        argv = [executable, '--fail', '--location', '--silent', '--show-error',
                '--proto', '=https', '--proto-redir', '=https', '--retry', '2',
                '--connect-timeout', '15', '--max-time', '300', '--max-filesize', str(limit),
                '--output', str(target), url]
    else:
        argv = [executable, '--quiet', '--https-only', '--max-redirect=5',
                '--timeout=20', '--tries=2', '--output-document', str(target), url]
    subprocess.run(argv, check=True, capture_output=True, timeout=330)
    require(target.is_file() and 0 < target.stat().st_size <= limit, 'missing/oversized download')


def sha256(path):
    import hashlib
    h = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def fetch_controller(tag, tool, directory, pinned_manager=None, pinned_controller=None,
                     pinned_binary=None, pinned_theme=None):
    require(tag == 'latest' or re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?', tag),
            'invalid release tag')
    for pin in (pinned_manager, pinned_controller, pinned_binary, pinned_theme):
        require(pin is None or re.fullmatch(r'[0-9a-f]{64}', pin), 'invalid independent SHA-256 pin')
    url = 'https://api.github.com/repos/' + REPO + '/releases/' + ('latest' if tag == 'latest' else 'tags/' + tag)
    metadata = Path(directory)/'release.json'
    download(url, metadata, tool, 1024 * 1024)
    try:
        release = json.loads(metadata.read_text())
    finally:
        metadata.unlink(missing_ok=True)
    resolved = release.get('tag_name')
    require(isinstance(resolved, str) and
            re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+(?:[.-][A-Za-z0-9.-]+)?', resolved) and
            (tag == 'latest' or tag == resolved), 'release tag mismatch')
    require(not release.get('draft') and not release.get('prerelease'), 'draft/prerelease refused')
    assets = release.get('assets')
    require(isinstance(assets, list) and len(assets) <= 128 and
            all(isinstance(x, dict) and isinstance(x.get('name'), str) for x in assets),
            'invalid release asset list')
    names = [x['name'] for x in assets]
    require(len(names) == len(set(names)), 'duplicate asset names')
    index = {x['name']: x for x in assets}
    base = 'https://github.com/' + REPO + '/releases/download/' + resolved + '/'
    checks = {}
    for name in ('komari-manager.py', 'safe_upgrade.py', 'komari'):
        require(name in index, 'release ' + resolved + ' lacks ' + name)
        item = index[name]
        match = re.fullmatch(r'sha256:([0-9a-f]{64})', str(item.get('digest', '')))
        size = item.get('size')
        require(item.get('browser_download_url') == base + name and match and
                type(size) is int and 0 < size <= 150 * 1024 * 1024,
                'invalid URL/digest/size for ' + name)
        checks[name] = (match.group(1), size)
    for name, pin in (('komari-manager.py', pinned_manager),
                      ('safe_upgrade.py', pinned_controller), ('komari', pinned_binary)):
        require(not pin or pin == checks[name][0], 'pinned ' +
                ('manager' if name == 'komari-manager.py' else 'controller' if name == 'safe_upgrade.py' else 'binary') +
                ' SHA-256 differs from release')
    theme_sha = None
    if 'Glass.zip' in index:
        theme = index['Glass.zip']
        match = re.fullmatch(r'sha256:([0-9a-f]{64})', str(theme.get('digest', '')))
        require(theme.get('browser_download_url') == base + 'Glass.zip' and match and
                type(theme.get('size')) is int and 0 < theme['size'] <= 100 * 1024 * 1024,
                'invalid Glass.zip release metadata')
        theme_sha = match.group(1)
    require(not pinned_theme or pinned_theme == theme_sha, 'pinned theme SHA-256 differs from release')
    for name in ('komari-manager.py', 'safe_upgrade.py'):
        expected, size = checks[name]
        dest = Path(directory)/name
        download(base + name, dest, tool, size)
        require(dest.stat().st_size == size and sha256(dest) == expected,
                'controller checksum/size mismatch: ' + name)
        dest.chmod(0o600)
    return Controller(resolved, Path(directory)/'komari-manager.py',
                      Path(directory)/'safe_upgrade.py', checks['komari'][0], theme_sha)


def run_manager(controller, arguments):
    return subprocess.call([sys.executable, str(controller.manager), *arguments])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('install', 'update'))
    parser.add_argument('--tag', required=True, help='concrete release tag or latest (resolved once)')
    parser.add_argument('--downloader', choices=('auto', 'curl', 'wget'), default='auto')
    parser.add_argument('--download-dir', default='/var/lib/komari-downloads')
    parser.add_argument('--sha256', help='independently pinned panel binary hash')
    parser.add_argument('--manager-sha256', help='independently pinned manager script hash')
    parser.add_argument('--controller-sha256', help='independently pinned recovery controller hash')
    parser.add_argument('--theme-sha256', help='independently pinned Glass bundle hash')
    options, forward = parser.parse_known_args()
    try:
        require(os.geteuid() == 0, 'run as root on the intended host')
        require(not any(value.split('=', 1)[0] in
                        ('--tag', '--binary', '--theme-bundle', '--sha256', '--theme-sha256',
                         '--manager-sha256', '--controller-sha256') for value in forward),
                'bootstrap owns release inputs')
        tool = ('curl' if shutil.which('curl') else 'wget') if options.downloader == 'auto' else options.downloader
        trusted_tool(tool)
        parent = Path(options.download_dir)
        require(parent.is_absolute() and re.fullmatch(r'[A-Za-z0-9_./:@-]+', str(parent)),
                'unsafe bootstrap staging path')
        for ancestor in parent.parents:
            meta = ancestor.lstat()
            require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                    'untrusted bootstrap staging ancestor')
        if not parent.exists() and not parent.is_symlink():
            parent.mkdir(mode=0o700)
        for part in (parent, *parent.parents):
            meta = part.lstat()
            require(stat.S_ISDIR(meta.st_mode) and meta.st_uid == 0 and not meta.st_mode & 0o022,
                    'untrusted bootstrap staging path')
        with tempfile.TemporaryDirectory(prefix='bootstrap-', dir=parent) as stage:
            bundle = fetch_controller(options.tag, tool, stage,
                                      pinned_manager=options.manager_sha256,
                                      pinned_controller=options.controller_sha256,
                                      pinned_binary=options.sha256,
                                      pinned_theme=options.theme_sha256)
            args = [options.action, '--tag', bundle.tag, '--sha256', bundle.binary_sha,
                    '--downloader', tool, '--download-dir', str(parent), *forward]
            if bundle.theme_sha and options.action == 'update':
                args.extend(['--theme-sha256', bundle.theme_sha])
            print('Bootstrap verified controller from release ' + bundle.tag, flush=True)
            return run_manager(bundle, args)
    except (Exception, KeyboardInterrupt) as exc:
        print('komari-oneclick: ' + str(exc), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
