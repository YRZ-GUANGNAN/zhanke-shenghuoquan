#!/usr/bin/env python3
"""Publish only the public-profile cover projection with a verified rollback."""
import argparse
import datetime
import fcntl
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
PREVIOUS = ROOT / 'releases/20261016-delivery-api'
RELEASE = ROOT / 'releases/20261017-public-profile'
PENDING = ROOT / 'current.public-profile-next'
STAGE = Path('/home/admin/campus-public-profile-20261017')
ENV_FILE = Path('/etc/campus-circle/campus-api.env')
RECORD = Path('/var/backups/campus-circle/20261017-public-profile-result.json')
OLD_COMMIT = '302660924ac5'
OLD_HASH = 'e7372f0de4a0c7b7ca3c9334ef31999107a02af2e2c854000243a5b107b97ee5'
VERSION = '2026.10.17.1'
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


def run(command):
    result = subprocess.run(command, capture_output=True, timeout=45)
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
    require(not os.path.lexists(RELEASE) and not os.path.lexists(PENDING) and not os.path.lexists(RECORD), 'release identity already exists')
    require(RECORD.parent.is_dir() and not RECORD.parent.is_symlink() and RECORD.parent.stat().st_uid == 0 and not (RECORD.parent.stat().st_mode & 0o077), 'private record directory required')
    require(shutil.disk_usage(ROOT).free > 512 * 1024 * 1024, 'insufficient free disk space')


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
            result.update(after=after, verified=True)
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
