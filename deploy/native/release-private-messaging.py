#!/usr/bin/env python3
"""Publish private messaging with a database backup and verified rollback."""
import argparse
import datetime
import fcntl
import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request

ROOT = Path('/opt/campus-circle')
CURRENT = ROOT / 'current'
PREVIOUS = ROOT / 'releases/20261017-public-profile'
RELEASE = ROOT / 'releases/20261017-private-messaging'
PENDING = ROOT / 'current.private-messaging-next'
STAGE = Path('/home/admin/campus-private-messaging-20261017')
ENV_FILE = Path('/etc/campus-circle/campus-api.env')
RECORD = Path('/var/backups/campus-circle/20261017-private-messaging-result.json')
OLD_COMMIT = '092ddc352125'
OLD_HASH = '641c456b6ad49cdeb26306098b96ce8b4a0d9a23f2a8cab52514c27328d8b530'
VERSION = '2026.10.17.3'
BACKUP = RECORD.parent / '20261017-private-messaging-before.sql.gz'
MYSQL_CONFIG = Path('/etc/campus-circle/mysql-admin.cnf')
MYSQL = ['mysql', '--defaults-extra-file=' + str(MYSQL_CONFIG), '--batch', '--skip-column-names', 'campus_circle']
PROTECTED = ('/api/v1/community/users/7/profile', '/api/v1/delivery/merchant/orders',
             '/api/v1/delivery/rider/orders', '/api/v1/admin/share-settings')


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def sha(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(chunk)
    return digest.hexdigest()


def value_sha(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def run(command, **kwargs):
    result = subprocess.run(command, capture_output=True, timeout=45, **kwargs)
    require(result.returncode == 0, command[0] + ' failed; sensitive output withheld')
    return result.stdout.decode().strip()


def probe(path, protected=False):
    try:
        response = urllib.request.urlopen('http://127.0.0.1:8080' + path, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        if protected:
            require(response.status in (401, 403), path + ': access protection changed')
            return response.status
        require(response.status == 200, path + ': unexpected HTTP ' + str(response.status))
        body = json.load(response)
        require(body.get('code') == 0, path + ': application error')
        return body['data']


def snapshot():
    return {
        'version': probe('/version'), 'health': probe('/healthz'), 'readiness': probe('/readyz'),
        'featurePolicySha256': value_sha(probe('/api/v1/features')),
        'shareSettingsSha256': value_sha(probe('/api/v1/share-settings')),
        'runtimeEnvironmentSha256': sha(ENV_FILE),
        'protectedEndpoints': {path: probe(path, True) for path in PROTECTED},
        'release': str(CURRENT.resolve(strict=True)),
    }


def sql(statement):
    return run(MYSQL, input=statement.encode())


def verify_dependencies():
    require(MYSQL_CONFIG.is_file() and not MYSQL_CONFIG.is_symlink() and not (MYSQL_CONFIG.stat().st_mode & 0o077), 'private database config required')
    values = {}
    for line in ENV_FILE.read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    require(all(values.get(key, '').strip() for key in ('TENCENT_IM_SDK_APP_ID', 'TENCENT_IM_SECRET_KEY', 'TENCENT_IM_ADMIN_USER')), 'Tencent IM credentials missing')
    match = re.fullmatch(r'([^:]+):(.*)@tcp\(([^:]+):(\d+)\)/([^?]+)(?:\?.*)?', values['MYSQL_DSN'])
    require(match is not None, 'unexpected runtime database configuration')
    user, password, host, port, database = match.groups()
    require(database == 'campus_circle' and host in ('127.0.0.1', 'localhost'), 'unexpected database target')
    runtime_uuid = run(['mysql', '--protocol=TCP', '-h', host, '-P', port, '-u', user, '--batch', '--skip-column-names', database],
                       input=b'SELECT @@server_uuid;', env={**os.environ, 'MYSQL_PWD': password})
    require(runtime_uuid == sql('SELECT @@server_uuid;'), 'runtime and admin database mismatch')
    require(sql("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_type='BASE TABLE' AND engine<>'InnoDB';") == '0', 'nontransactional tables require another backup strategy')
    relation = sql("SELECT column_name,non_unique,COALESCE(sub_part,0) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='im_conversations' AND index_name='idx_im_relation' ORDER BY seq_in_index;")
    require(relation.splitlines() == [name + '\t0\t0' for name in ('buyer_id', 'seller_id', 'reference_type', 'reference_id')], 'conversation unique index missing')
    require(sql("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='im_conversations' AND column_name='reference_type' AND data_type='varchar' AND character_maximum_length>=7;") == '1', 'user relation type unsupported')
    require(sql("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='user_blocks' AND column_name IN ('user_id','blocked_user_id');") == '2', 'block schema missing')


def backup_database():
    with BACKUP.open('xb') as raw:
        os.chmod(BACKUP, 0o600)
        command = ['mysqldump', '--defaults-extra-file=' + str(MYSQL_CONFIG), '--single-transaction', '--quick',
                   '--no-tablespaces', '--routines', '--events', '--triggers', '--set-gtid-purged=OFF', '--hex-blob', 'campus_circle']
        with subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE) as process:
            with gzip.GzipFile(fileobj=raw, mode='wb') as compressed:
                shutil.copyfileobj(process.stdout, compressed)
            process.stdout.close()
            process.communicate(timeout=60)
            require(process.returncode == 0, 'database backup failed; backend unchanged')
    size = 0
    with gzip.open(BACKUP, 'rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            size += len(chunk)
    require(size > 1024, 'backup unexpectedly small')
    return {'path': str(BACKUP), 'sha256': sha(BACKUP), 'uncompressedBytes': size}


def verify_private_routes():
    result = {}
    for method, path in (('PUT', '/api/v1/im/users/7/conversation'), ('GET', '/api/v1/im/conversations'), ('GET', '/api/v1/im/usersig')):
        request = urllib.request.Request('http://127.0.0.1:8080' + path, method=method)
        try:
            response = urllib.request.urlopen(request, timeout=5)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            require(response.status in (401, 403), path + ': authentication protection missing')
            result[method + ' ' + path] = response.status
    return result


def preflight(args):
    require(os.geteuid() == 0, 'sudo required')
    require(re.fullmatch(r'[a-f0-9]{64}', args.sha256) is not None, 'invalid artifact hash')
    require(re.fullmatch(r'[a-f0-9]{12}', args.commit) is not None, 'invalid commit')
    require(CURRENT.is_symlink() and CURRENT.resolve(strict=True) == PREVIOUS, 'production release changed')
    require(probe('/version')['commit'] == OLD_COMMIT, 'running baseline changed')
    require(sha(PREVIOUS / 'bin/campus-api') == OLD_HASH, 'rollback binary changed')
    require(STAGE.is_dir() and not STAGE.is_symlink(), 'unexpected stage')
    artifact = STAGE / 'campus-api'
    require(artifact.is_file() and not artifact.is_symlink() and sha(artifact) == args.sha256, 'artifact mismatch')
    with artifact.open('rb') as stream:
        header = stream.read(64)
    require(header[:5] == b'\x7fELF\x02' and header[18:20] == b'\x3e\x00', 'expected Linux amd64 binary')
    require(not os.path.lexists(RELEASE) and not os.path.lexists(PENDING) and not os.path.lexists(RECORD) and not os.path.lexists(BACKUP), 'release identity already exists')
    require(RECORD.parent.is_dir() and not RECORD.parent.is_symlink() and RECORD.parent.stat().st_uid == 0 and not (RECORD.parent.stat().st_mode & 0o077), 'private record directory required')
    require(shutil.disk_usage(ROOT).free > 1024**3 and shutil.disk_usage(RECORD.parent).free > 1024**3, 'insufficient free disk space')
    verify_dependencies()


def activate(target):
    require(target in (PREVIOUS, RELEASE) and target.is_dir(), 'unexpected activation target')
    require(not os.path.lexists(PENDING), 'another transition is pending')
    PENDING.symlink_to(target, target_is_directory=True)
    os.replace(PENDING, CURRENT)
    run(['systemctl', 'restart', 'campus-api'])


def wait_healthy(commit):
    for attempt in range(12):
        try:
            probe('/healthz')
            probe('/readyz')
            require(probe('/version')['commit'] == commit, 'unexpected running commit')
            require(run(['systemctl', 'is-active', 'campus-api']) == 'active', 'service is not active')
            return
        except Exception:
            if attempt == 11:
                raise RuntimeError('backend did not become healthy at the expected commit')
            time.sleep(2)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sha256', required=True)
    parser.add_argument('--commit', required=True)
    parser.add_argument('--deploy', action='store_true')
    args = parser.parse_args()
    preflight(args)
    before = snapshot()
    result = {'release': str(RELEASE), 'version': VERSION, 'commit': args.commit,
              'binarySha256': args.sha256, 'rollbackBinarySha256': OLD_HASH,
              'previousRelease': str(PREVIOUS), 'before': before, 'deployed': False,
              'verified': False, 'rollbackPerformed': False, 'migrationExecuted': False,
              'databaseWrittenByRelease': False}
    if not args.deploy:
        print(json.dumps(result, sort_keys=True))
        return
    with open('/run/lock/campus-api-native-release.lock', 'a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        preflight(args)
        require(snapshot() == before, 'production changed during preparation')
        result['databaseBackup'] = backup_database()
        require(CURRENT.resolve() == PREVIOUS and probe('/version')['commit'] == OLD_COMMIT, 'production changed during backup')
        try:
            RELEASE.mkdir(mode=0o755)
            (RELEASE / 'bin').mkdir(mode=0o755)
            shutil.copyfile(STAGE / 'campus-api', RELEASE / 'bin/campus-api')
            (RELEASE / 'bin/campus-api').chmod(0o755)
            require(sha(RELEASE / 'bin/campus-api') == args.sha256, 'installed binary mismatch')
            (RELEASE / 'RELEASE').write_text(VERSION + '\n' + args.commit + '\n' + args.sha256 + '\n')
            print(json.dumps({'stage': 'switching_backend', 'previousRelease': str(PREVIOUS)}), flush=True)
            activate(RELEASE)
            result['deployed'] = True
            wait_healthy(args.commit)
            after = snapshot()
            require(after['version']['version'] == VERSION and after['release'] == str(RELEASE), 'release identity mismatch')
            for key in ('featurePolicySha256', 'shareSettingsSha256', 'runtimeEnvironmentSha256', 'protectedEndpoints'):
                require(before[key] == after[key], key + ' changed during release')
            require(sha(PREVIOUS / 'bin/campus-api') == OLD_HASH, 'rollback binary changed')
            result.update(after=after, verified=True, privateRoutes=verify_private_routes())
        except Exception:
            if CURRENT.resolve() == RELEASE:
                activate(PREVIOUS)
                wait_healthy(OLD_COMMIT)
                result.update(rollbackPerformed=True, deployed=False)
            raise
        finally:
            result['recordedAt'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
            with RECORD.open('x') as stream:
                os.chmod(RECORD, 0o600)
                json.dump(result, stream, indent=2, sort_keys=True)
                stream.flush()
                os.fsync(stream.fileno())
            print(json.dumps(result, sort_keys=True), flush=True)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        message = str(error) if isinstance(error, RuntimeError) else 'operation failed; sensitive details withheld'
        print(json.dumps({'success': False, 'errorType': type(error).__name__, 'error': message}), file=sys.stderr)
        raise SystemExit(1)
