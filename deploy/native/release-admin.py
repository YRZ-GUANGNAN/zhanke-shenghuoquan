#!/usr/bin/env python3
"""Publish a verified Admin archive to the existing zhanduiwang.top site."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import tarfile
import urllib.error
import urllib.request


ROOT = Path('/var/www/campus-admin')
DOMAIN = 'https://zhanduiwang.top'


def digest(value):
    return hashlib.sha256(value).hexdigest()


def fetch(path, status=200):
    request = urllib.request.Request(DOMAIN + path, headers={'Cache-Control': 'no-cache'})
    try:
        response = urllib.request.urlopen(request, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        if response.status != status:
            raise RuntimeError(f'{path}: HTTP {response.status}, expected {status}')
        return response.read()


def switch_to(target):
    pending = ROOT / 'current.admin-next'
    if os.path.lexists(pending):
        raise RuntimeError(f'Unexpected pending symlink: {pending}')
    pending.symlink_to(target, target_is_directory=True)
    os.replace(pending, ROOT / 'current')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--sha256', required=True)
    parser.add_argument('--release', required=True)
    parser.add_argument('--deploy', action='store_true')
    args = parser.parse_args()
    if not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}', args.release):
        raise ValueError('Invalid release ID')
    if not re.fullmatch(r'[a-f0-9]{64}', args.sha256):
        raise ValueError('Invalid SHA-256')
    if digest(args.archive.read_bytes()) != args.sha256:
        raise ValueError('Archive checksum mismatch')
    current = ROOT / 'current'
    if not current.is_symlink():
        raise RuntimeError('Existing current must be a symlink')
    previous = current.resolve(strict=True)
    releases = (ROOT / 'releases').resolve(strict=True)
    if previous.parent != releases or not (previous / 'index.html').is_file():
        raise RuntimeError('Previous release is outside the expected directory')
    release = releases / args.release
    if os.path.lexists(release) or os.path.lexists(ROOT / 'current.admin-next'):
        raise RuntimeError('Release or pending symlink already exists')

    files = {}
    with tarfile.open(args.archive, 'r:gz') as archive:
        for member in archive.getmembers():
            name = PurePosixPath(member.name)
            if name.is_absolute() or '..' in name.parts:
                raise ValueError('Unsafe archive path')
            if name == PurePosixPath('.') and member.isdir():
                continue
            if name != PurePosixPath('index.html') and name.parts[0] != 'assets':
                raise ValueError(f'Unexpected Admin file: {name}')
            if not (member.isfile() or member.isdir()):
                raise ValueError('Only regular files and directories are permitted')
            if member.isfile():
                if str(name) in files or member.size > 20 * 1024 * 1024:
                    raise ValueError('Duplicate or excessive archive entry')
                files[str(name)] = archive.extractfile(member).read()
    if not files.get('index.html') or not any(name.endswith('.js') for name in files):
        raise RuntimeError('Incomplete Admin archive')

    before_policy = json.loads(fetch('/api/v1/features'))
    flags = before_policy.get('data', {}).get('flags', {})
    if before_policy.get('code') != 0 or len(flags) != 20 or not all(isinstance(v, bool) for v in flags.values()):
        raise RuntimeError('Feature-policy API is not ready')
    result = {'domain': 'zhanduiwang.top', 'previousRelease': str(previous),
              'release': str(release), 'archiveSHA256': args.sha256,
              'fileCount': len(files), 'deployed': False}
    if not args.deploy:
        print(json.dumps(result, indent=2))
        return

    release.mkdir(mode=0o755)
    for name, data in files.items():
        target = release / name
        target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        target.write_bytes(data)
        target.chmod(0o644)
    # Keep content-addressed assets for browsers holding the previous index.
    old_assets = previous / 'assets'
    if old_assets.is_dir():
        for source in old_assets.rglob('*'):
            if source.is_symlink():
                raise RuntimeError('Unexpected symlink in previous assets')
            if source.is_file():
                target = release / 'assets' / source.relative_to(old_assets)
                if target.exists():
                    if digest(target.read_bytes()) != digest(source.read_bytes()):
                        raise RuntimeError('Asset name collision with different content')
                else:
                    target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
                    shutil.copyfile(source, target)
                    target.chmod(0o644)

    switched = False
    try:
        switch_to(release)
        switched = True
        if digest(fetch('/')) != digest(files['index.html']):
            raise RuntimeError('Served index does not match the release')
        for name, data in files.items():
            if digest(fetch('/' + name)) != digest(data):
                raise RuntimeError(f'Served asset mismatch: {name}')
        if json.loads(fetch('/healthz')).get('code') != 0:
            raise RuntimeError('Backend health check failed')
        fetch('/api/v1/admin/feature-policy', status=401)
        fetch('/api/v1/admin-auth/session', status=401)
        after_policy = json.loads(fetch('/api/v1/features'))
        if after_policy.get('code') != 0 or len(after_policy.get('data', {}).get('flags', {})) != 20:
            raise RuntimeError('Feature-policy API check failed')
        result.update(deployed=True, httpsAssets='verified', backendHealth=200,
                      unauthenticatedAdminPolicy=401, unauthenticatedSession=401,
                      publicFeatureCount=20, nginxChanged=False, dnsChanged=False)
        (release / 'release-receipt.json').write_text(json.dumps(result, indent=2) + '\n')
        print(json.dumps(result, indent=2))
    except Exception:
        if switched:
            switch_to(previous)
        raise


if __name__ == '__main__':
    main()
