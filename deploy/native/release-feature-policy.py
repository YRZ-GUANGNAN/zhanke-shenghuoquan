#!/usr/bin/env python3
"""Guarded additive release on the existing native host; no DNS changes."""
import argparse
import gzip
import hashlib
import json
import os
import re
import shutil
import subprocess
import time
import urllib.request
from pathlib import Path

ROOT = Path('/opt/campus-circle')
ENV_FILE = Path('/etc/campus-circle/campus-api.env')
KEYS = {'forum', 'post', 'comment', 'secondhand', 'secondhand_publish',
        'secondhand_chat', 'takeout', 'campus_tools', 'delivery', 'merchant',
        'rider', 'messages', 'following', 'search', 'likes', 'favorites',
        'notifications', 'identity_application', 'publish', 'secondhand_comments'}


def run(command, **kwargs):
    result = subprocess.run(command, capture_output=True, **kwargs)
    if result.returncode:
        # Command output can include configuration values. Keep it on the host.
        raise RuntimeError(f'{command[0]} failed (exit {result.returncode}); output withheld')
    return result.stdout


def database_config():
    values = {}
    for line in ENV_FILE.read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    if values.get('ENABLE_FEATURE_POLICY', 'true').lower() not in ('true', '1'):
        raise RuntimeError('Feature policy is explicitly disabled in the runtime configuration')
    match = re.fullmatch(r'([^:]+):(.*)@tcp\(([^:]+):(\d+)\)/([^?]+)(?:\?.*)?', values['MYSQL_DSN'])
    if not match:
        raise RuntimeError('Unsupported database configuration')
    user, password, host, port, database = match.groups()
    if not re.fullmatch(r'[A-Za-z0-9_]+', database):
        raise RuntimeError('Unexpected database name')
    return ['--protocol=TCP', '-h', host, '-P', port, '-u', user], database, {**os.environ, 'MYSQL_PWD': password}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--stage', required=True)
    parser.add_argument('--release', required=True)
    parser.add_argument('--sha256', required=True)
    parser.add_argument('--commit', required=True)
    parser.add_argument('--deploy', action='store_true')
    args = parser.parse_args()
    if os.geteuid() != 0:
        raise RuntimeError('Run with sudo on the production host')
    if not re.fullmatch(r'[A-Za-z0-9._-]{1,80}', args.release) or not re.fullmatch(r'[a-f0-9]{12,40}', args.commit) or not re.fullmatch(r'[a-f0-9]{64}', args.sha256):
        raise RuntimeError('Invalid release identity')
    stage = Path(args.stage)
    if str(stage) != f'/home/admin/campus-feature-{args.release}' or stage.is_symlink():
        raise RuntimeError('Unexpected staging directory')
    for name in ('campus-api', '038_feature_policy.sql', '039_feature_policy_modules.sql'):
        target = stage / name
        if target.is_symlink() or not target.is_file():
            raise RuntimeError('Missing regular release asset')
    if hashlib.sha256((stage / 'campus-api').read_bytes()).hexdigest() != args.sha256:
        raise RuntimeError('Release binary checksum mismatch')
    current = ROOT / 'current'
    old = current.resolve(strict=True)
    if old.parent != ROOT / 'releases' or not current.is_symlink():
        raise RuntimeError('Unexpected current release target')
    options, database, process_env = database_config()

    def sql(query):
        return run(['mysql', *options, '-N', database], input=query, env=process_env, text=True).strip()

    column = sql("SELECT IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='feature_policy_configs' AND COLUMN_NAME='extra_flags';")
    if shutil.disk_usage(ROOT).free < 512 * 1024 * 1024:
        raise RuntimeError('Insufficient disk space for a safe release')
    result = {'release': args.release, 'previousRelease': str(old), 'database': database,
              'schemaExtraFlags': column or 'missing', 'sha256': args.sha256, 'deployed': False}
    if not args.deploy:
        print(json.dumps(result))
        return

    backup_dir = Path('/var/backups/campus-circle')
    backup_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(backup_dir, 0o700)
    backup = backup_dir / f'{args.release}-before-feature-policy.sql.gz'
    if backup.exists():
        raise RuntimeError('Backup already exists; use a new release identity')
    with backup.open('xb') as raw:
        os.chmod(backup, 0o600)
        dump = subprocess.Popen(['mysqldump', *options, '--single-transaction', '--no-tablespaces', database], env=process_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        with gzip.GzipFile(fileobj=raw, mode='wb') as compressed:
            shutil.copyfileobj(dump.stdout, compressed)
        dump.stdout.close()
        dump.communicate()
        if dump.returncode:
            raise RuntimeError('Database backup failed; migrations were not attempted')
    with gzip.open(backup, 'rb') as check:
        if not check.read(128):
            raise RuntimeError('Empty database backup; migrations were not attempted')
    # Only these two additive migrations are allowed. Never replay dev seed or
    # the full historical migration set against an existing production database.
    sql((stage / '038_feature_policy.sql').read_text())
    extension = (stage / '039_feature_policy_modules.sql').read_text()
    if not column:
        sql(extension)
    elif column == 'YES':
        # Resume an interrupted 039 after its first DDL statement.
        sql(extension.split(';', 1)[1])
    if sql("SELECT IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='feature_policy_configs' AND COLUMN_NAME='extra_flags';") != 'NO':
        raise RuntimeError('Feature policy migration did not finish')

    release = ROOT / 'releases' / args.release
    release.mkdir(mode=0o755)
    (release / 'bin').mkdir(mode=0o755)
    shutil.copyfile(stage / 'campus-api', release / 'bin/campus-api')
    os.chmod(release / 'bin/campus-api', 0o755)
    (release / 'RELEASE').write_text(f'{args.release}\n{args.commit}\n{args.sha256}\n')
    os.chmod(release / 'RELEASE', 0o644)

    def activate(target):
        pending = ROOT / 'current.feature-next'
        if pending.exists() or pending.is_symlink():
            raise RuntimeError('A release transition is already pending')
        pending.symlink_to(target, target_is_directory=True)
        pending.replace(current)
        run(['systemctl', 'restart', 'campus-api'])

    def probe(path):
        with urllib.request.urlopen(f'http://127.0.0.1:8080{path}', timeout=4) as response:
            return json.load(response)['data']

    switched = False
    try:
        # Mark before restart: a failed restart must also restore the symlink.
        switched = True
        activate(release)
        failure = None
        for _ in range(12):
            try:
                probe('/readyz')
                version = probe('/version')
                policy = probe('/api/v1/features')
                if version['commit'] != args.commit or policy.get('enabled') is not True or set(policy.get('flags', {})) != KEYS or any(type(value) is not bool for value in policy['flags'].values()):
                    raise RuntimeError('Running release metadata or feature policy is inconsistent')
                result.update(deployed=True, version=version, rowVersion=policy['rowVersion'], backup=str(backup))
                break
            except Exception as error:
                failure = type(error).__name__
                time.sleep(2)
        if not result['deployed']:
            raise RuntimeError(f'Release health checks failed ({failure})')
    except Exception:
        if switched and current.resolve() == release:
            activate(old)
        # Additive policy tables and audit history are deliberately retained.
        raise
    print(json.dumps(result))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'deployed': False, 'error': str(error)}))
        raise SystemExit(1)
