#!/usr/bin/env python3
"""Add migration 040 and publish an identified backend without changing feature policy."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.request
import urllib.error

ROOT = Path('/opt/campus-circle')


def run(command, **kwargs):
    result = subprocess.run(command, capture_output=True, **kwargs)
    if result.returncode:
        raise RuntimeError(f'{command[0]} failed (exit {result.returncode}); sensitive output withheld')
    return result.stdout


def probe(path, status=200):
    try:
        response = urllib.request.urlopen('http://127.0.0.1:8080' + path, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        if response.status != status:
            raise RuntimeError(f'{path}: unexpected HTTP {response.status}')
        return json.load(response)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('release', 'sha256', 'migration-sha256', 'commit', 'expected-release', 'expected-commit'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--deploy', action='store_true')
    args = parser.parse_args()
    if os.geteuid() != 0:
        raise RuntimeError('Run with sudo')
    for identity in (args.release, args.expected_release):
        if not re.fullmatch(r'[A-Za-z0-9._-]{1,80}', identity):
            raise ValueError('Invalid release identity')
    for value in (args.sha256, args.migration_sha256):
        if not re.fullmatch(r'[a-f0-9]{64}', value):
            raise ValueError('Invalid checksum')
    for value in (args.commit, args.expected_commit):
        if not re.fullmatch(r'[a-f0-9]{12,40}', value):
            raise ValueError('Invalid source commit')
    stage = Path('/home/admin') / ('campus-share-' + args.release)
    if stage.is_symlink():
        raise RuntimeError('Unexpected staging symlink')
    for name, digest in (('campus-api', args.sha256), ('040_share_settings.sql', args.migration_sha256)):
        target = stage / name
        if target.is_symlink() or not target.is_file() or hashlib.sha256(target.read_bytes()).hexdigest() != digest:
            raise RuntimeError('Missing or mismatched release asset: ' + name)
    current = ROOT / 'current'
    previous = current.resolve(strict=True)
    if not current.is_symlink() or previous != ROOT / 'releases' / args.expected_release:
        raise RuntimeError('Production release changed; inspect before proceeding')
    if probe('/version')['data']['commit'] != args.expected_commit:
        raise RuntimeError('Running build no longer matches the inspected baseline')
    before_policy = probe('/api/v1/features')['data']
    probe('/readyz')
    release = ROOT / 'releases' / args.release
    pending = ROOT / 'current.share-next'
    if release.exists() or os.path.lexists(pending):
        raise RuntimeError('Release ID or pending transition already exists')
    if shutil.disk_usage(ROOT).free < 512 * 1024 * 1024:
        raise RuntimeError('Insufficient free space')
    migration_config = Path('/etc/campus-circle/mysql-admin.cnf')
    if not migration_config.is_file() or migration_config.is_symlink() or migration_config.stat().st_mode & 0o077:
        raise RuntimeError('Private migration configuration required')
    values = {}
    for line in Path('/etc/campus-circle/campus-api.env').read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    match = re.fullmatch(r'[^:]+:.*@tcp\([^)]+\)/([A-Za-z0-9_]+)(?:\?.*)?', values['MYSQL_DSN'])
    if not match:
        raise RuntimeError('Unrecognized database name')
    database = match.group(1)
    options = [f'--defaults-extra-file={migration_config}']
    run(['mysql', *options, '-N', database], input='SELECT 1;', text=True)
    result = {'release': args.release, 'previousRelease': str(previous), 'commit': args.commit, 'deployed': False}
    if not args.deploy:
        print(json.dumps(result))
        return

    backup_dir = Path('/var/backups/campus-circle')
    backup_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    backup = backup_dir / (args.release + '-before-share-settings.sql.gz')
    with backup.open('xb') as raw:
        os.chmod(backup, 0o600)
        dump = subprocess.Popen(['mysqldump', *options, '--single-transaction', '--no-tablespaces', database], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        with gzip.GzipFile(fileobj=raw, mode='wb') as compressed:
            shutil.copyfileobj(dump.stdout, compressed)
        dump.stdout.close()
        dump.communicate()
        if dump.returncode:
            raise RuntimeError('Backup failed; migration not attempted')
    with gzip.open(backup, 'rb') as check:
        if not check.read(128):
            raise RuntimeError('Backup is empty')
    run(['mysql', *options, '--default-character-set=utf8mb4', database], input=(stage / '040_share_settings.sql').read_text(), text=True)
    # Recheck before switching; an independent operator may have published meanwhile.
    if current.resolve() != previous or probe('/version')['data']['commit'] != args.expected_commit:
        raise RuntimeError('Production changed during preparation; additive schema retained')
    release.mkdir(mode=0o755)
    (release / 'bin').mkdir(mode=0o755)
    shutil.copyfile(stage / 'campus-api', release / 'bin/campus-api')
    (release / 'bin/campus-api').chmod(0o755)
    (release / 'RELEASE').write_text(f'{args.release}\n{args.commit}\n{args.sha256}\n')

    def activate(target):
        if os.path.lexists(pending):
            raise RuntimeError('Another transition is pending')
        pending.symlink_to(target, target_is_directory=True)
        os.replace(pending, current)
        run(['systemctl', 'restart', 'campus-api'])

    try:
        activate(release)
        for attempt in range(12):
            try:
                probe('/readyz')
                version = probe('/version')['data']
                settings = probe('/api/v1/share-settings')['data']
                if version['commit'] != args.commit or settings['rowVersion'] < 1:
                    raise RuntimeError('New backend identity or settings mismatch')
                if probe('/api/v1/features')['data'] != before_policy:
                    raise RuntimeError('Feature policy changed during release')
                probe('/api/v1/admin/share-settings', 401)
                result.update(deployed=True, backup=str(backup), version=version, shareSettingsVersion=settings['rowVersion'], featurePolicyUnchanged=True)
                break
            except Exception:
                if attempt == 11:
                    raise
                time.sleep(2)
    except Exception:
        if current.resolve() == release:
            activate(previous)
        # Keep additive tables and audit history; the old binary ignores them.
        raise
    (release / 'share-settings-receipt.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
