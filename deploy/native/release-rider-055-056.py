#!/usr/bin/env python3
"""Guarded 055/056 deployment for the frozen 1b919e5 rider release.

The default mode is read-only. --deploy is the only mutation path. Database
rollback is never automatic after additive migrations; a failed new binary can
only fall back to the old binary while retaining the migrated schema.
"""
import argparse
import fcntl
import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path('/opt/campus-circle')
ENV_FILE = Path('/etc/campus-circle/campus-api.env')
MYSQL = ['--defaults-extra-file=/etc/campus-circle/mysql-admin.cnf']
OLD_RELEASE = '20261103-notification-settlement-972d3f3'
OLD_COMMIT = '972d3f343745'
OLD_BINARY_SHA256 = '70d5f389eeb0b46a98704e3b369d0e6e0da84ba7300af7d02a795e727aca84a8'
RELEASE_ID = '20261112-rider-wallet-1b919e5'
RELEASE_VERSION = '2026.11.12.candidate.2'
CANDIDATE_COMMIT = '1b919e5ce5017373cdf10a22da1f503f5e1f5c50'
CANDIDATE_BINARY_SHA256 = 'ec5c8695db133cc3c98b78e5ba2cbf8aef82cb3fde1b843e04190e3fd2b2eb61'
MANIFEST_SHA256 = '060d018e05640359685b21b5620f635f5da6383ffd09549e5f9850fda7d1a1f1'
MIGRATIONS = [
    '055_delivery_rider_availability.sql',
    '056_delivery_rider_withdrawal_allocations.sql',
]
MIGRATION_SHA256 = {
    MIGRATIONS[0]: 'cc0bc1f59a5c230d14a9f81bca1b17d302a25539809380fe05a53e38f186b5ab',
    MIGRATIONS[1]: '97eeb45cc4a8fe4e1fa6a17f05f4a3e188b3b7313dba14ecb07bb070a56af6a6',
}
INSPECTOR_SHA256 = 'a0e0392b2ebb050cb8d6d53ba2a94932d2a37cc6fe3fa19baf5bb89958e24811'
OFF_FLAGS = (
    'ENABLE_DELIVERY_RIDER_BROADCAST',
    'ENABLE_DELIVERY_RIDER_WITHDRAWAL',
    'ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE',
    'ENABLE_NOTIFICATION_SENDING',
)
EMPTY_MONEY_TABLES = (
    'delivery_rider_earning_records',
    'delivery_rider_wallets',
    'delivery_rider_ledger',
    'delivery_rider_withdrawals',
    'delivery_rider_payout_records',
    'delivery_rider_payout_queue',
    'delivery_rider_payout_reservations',
)
NEW_TABLES = {
    'delivery_rider_availability',
    'delivery_rider_broadcast_config',
    'delivery_rider_withdrawal_allocations',
    'delivery_rider_withdrawal_claims',
    'delivery_rider_withdrawal_events',
    'delivery_rider_withdrawal_payment_references',
}


def require(condition, reason):
    if not condition:
        raise RuntimeError(reason)


def run(command, **kwargs):
    result = subprocess.run(command, capture_output=True, **kwargs)
    if result.returncode:
        raise RuntimeError(command[0] + ' failed; sensitive output withheld')
    return result.stdout


def sha256_file(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def load_inspector(path):
    require(not path.is_symlink() and path.is_file(), 'invalid inspector')
    require(sha256_file(path) == INSPECTOR_SHA256, 'inspector checksum mismatch')
    spec = importlib.util.spec_from_file_location('rider_release_inspector', path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def env_values():
    values = {}
    for line in ENV_FILE.read_text(encoding='utf-8').splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            key = key.strip()
            require(key not in values, 'duplicate environment key')
            values[key] = value.strip().strip('"').strip("'")
    return values


def database(values):
    match = re.search(r'@tcp\([^)]+\)/([A-Za-z0-9_]+)\?(.*)$', values.get('MYSQL_DSN', ''))
    require(match is not None, 'unrecognized database configuration')
    params = urllib.parse.parse_qs(match.group(2))
    require(params.get('parseTime', [''])[0].lower() == 'true', 'parseTime contract changed')
    require(params.get('loc') == ['Local'], 'Local time contract changed')
    require(params.get('time_zone') in (None, ["'+08:00'"], ['+08:00']),
            'unexpected session clock')
    return match.group(1)


def sql(db, query, input_text=None):
    return run(
        ['mysql', *MYSQL, '--default-character-set=utf8mb4', '--batch',
         '--skip-column-names', db, '-e', query] if input_text is None else
        ['mysql', *MYSQL, '--default-character-set=utf8mb4', db],
        input=input_text,
        text=True,
    ).strip()


def sql_bytes(db, query):
    return run([
        'mysql', *MYSQL, '--default-character-set=utf8mb4', '--batch',
        '--skip-column-names', db, '-e', query,
    ])


def scalar(db, query):
    value = sql(db, query)
    require(re.fullmatch(r'-?[0-9]+', value) is not None, 'invalid scalar result')
    return int(value)


def probe(path, expected=200):
    try:
        response = urllib.request.urlopen('http://127.0.0.1:8080' + path, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        require(response.status == expected, path + ' returned unexpected HTTP status')
        return json.load(response)


def read_inspector(path, *arguments):
    result = subprocess.run(
        ['python3', '-B', str(path), *arguments],
        capture_output=True,
        text=True,
        timeout=45,
    )
    require(result.returncode == 0, 'release inspector gate failed')
    try:
        value = json.loads(result.stdout)
    except ValueError as error:
        raise RuntimeError('invalid release inspector output') from error
    require(isinstance(value, dict), 'invalid release inspector output')
    return value


def columns(db, table):
    require(re.fullmatch(r'[A-Za-z0-9_]+', table) is not None, 'invalid table identifier')
    return sql(
        db,
        "SELECT COLUMN_NAME FROM information_schema.COLUMNS "
        "WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='" + table +
        "' ORDER BY ORDINAL_POSITION",
    ).splitlines()


def snapshot(db, schema=None):
    if schema is None:
        tables = sql(db, 'SHOW TABLES').splitlines()
        schema = {table: columns(db, table) for table in tables}
    digests = {}
    for table, table_columns in schema.items():
        require(re.fullmatch(r'[A-Za-z0-9_]+', table) is not None, 'invalid table identifier')
        require(all(re.fullmatch(r'[A-Za-z0-9_]+', column) for column in table_columns),
                'invalid column identifier')
        primary = sql(
            db,
            "SELECT COLUMN_NAME FROM information_schema.STATISTICS "
            "WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='" + table +
            "' AND INDEX_NAME='PRIMARY' ORDER BY SEQ_IN_INDEX",
        ).splitlines()
        require(primary, 'missing primary key: ' + table)
        parts = [
            "CONCAT('" + column + "=',IF(`" + column +
            "` IS NULL,'N',CONCAT('V',HEX(`" + column + "`))))"
            for column in table_columns
        ]
        order_by = ','.join('`' + column + '`' for column in primary)
        rows = sql_bytes(
            db,
            "SELECT CONCAT_WS('|'," + ','.join(parts) + ") AS r FROM `" +
            table + "` ORDER BY " + order_by,
        )
        digests[table] = hashlib.sha256(rows).hexdigest()
    return schema, digests


def flag_values(values):
    result = {}
    for name in (
        'ENABLE_DELIVERY',
        'ENABLE_DELIVERY_PAYMENT',
        'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT',
        'ENABLE_NOTIFICATION_SENDING',
        'WECHAT_SUBSCRIBE_MINIPROGRAM_STATE',
        *OFF_FLAGS,
    ):
        result[name] = values.get(name, '')
    return result


def bool_value(value, fallback=False):
    if value == '':
        return fallback
    if value in ('true', '1'):
        return True
    if value in ('false', '0'):
        return False
    raise RuntimeError('invalid boolean flag')


def assert_default_off(values):
    for name in OFF_FLAGS:
        require(not bool_value(values.get(name, ''), False), 'default-off flag changed: ' + name)


def assert_database_preconditions(db):
    require(sql(db, 'SELECT TIMESTAMPDIFF(MINUTE,UTC_TIMESTAMP(),NOW())') == '480',
            'database clock changed')
    require(sql(
        db,
        "SELECT COUNT(*) FROM information_schema.TABLES "
        "WHERE TABLE_SCHEMA=DATABASE() AND ENGINE<>'InnoDB'",
    ) == '0', 'non-InnoDB table found')
    for kind, schema_column in (
        ('EVENTS', 'EVENT_SCHEMA'),
        ('TRIGGERS', 'TRIGGER_SCHEMA'),
        ('ROUTINES', 'ROUTINE_SCHEMA'),
    ):
        require(scalar(
            db,
            'SELECT COUNT(*) FROM information_schema.' + kind +
            ' WHERE ' + schema_column + '=DATABASE()',
        ) == 0, 'uninspected database writer found')
    tables = set(sql(db, 'SHOW TABLES').splitlines())
    require(not NEW_TABLES.intersection(tables), '055/056 table already exists')
    for table in EMPTY_MONEY_TABLES:
        require(scalar(db, 'SELECT COUNT(*) FROM `' + table + '`') == 0,
                'money baseline changed: ' + table)


def validate_new_schema(db, inspector, flags):
    tables = set(sql(db, 'SHOW TABLES').splitlines())
    require(NEW_TABLES.issubset(tables), '055/056 tables incomplete')
    tracked = (
        inspector.MIGRATION_055_TABLES +
        inspector.MIGRATION_056_TABLES +
        ['delivery_rider_earning_records']
    )
    shapes = {table: inspector.table_shape(
        lambda query: sql(db, query), table
    ) for table in tracked}
    counts = {
        table: scalar(db, 'SELECT COUNT(*) FROM `' + table + '`')
        for table in inspector.MIGRATION_055_TABLES + inspector.MIGRATION_056_TABLES
    }
    require(counts.get('delivery_rider_broadcast_config') == 1,
            'broadcast config seed missing')
    require(counts.get('delivery_rider_availability') == 0,
            'migration created unexpected availability fact')
    for table in inspector.MIGRATION_056_TABLES:
        require(counts.get(table) == 0, 'migration created unexpected business fact: ' + table)
    snapshot_value = {
        'version': {'data': {'commit': OLD_COMMIT}},
        'readyHTTP': 200,
        'releaseMetadata': {
            'currentIsSafeReleaseSymlink': True,
            'identityMatchesRuntime': True,
        },
        'flags': flags,
        'counts': counts,
        'shapes': shapes,
    }
    require(all(
        inspector.has_typed_columns(snapshot_value, table, expected)
        and inspector.has_indexes(snapshot_value, table, {'PRIMARY': ('id', True)})
        for table, expected in inspector.COLUMNS_055.items()
    ), 'migration 055 typed columns or primary keys differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_availability',
        {
            'uq_delivery_rider_availability_rider': ('rider_user_id', True),
            'idx_delivery_rider_availability_state': ('state,scope,rider_user_id', False),
        },
    ), 'migration 055 availability indexes differ')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_availability',
        inspector.CHECKS_055['delivery_rider_availability'],
    ), 'migration 055 availability checks differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_broadcast_config',
        {'uq_delivery_rider_broadcast_config_scope': ('config_scope', True)},
    ), 'migration 055 broadcast index differs')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_broadcast_config',
        inspector.CHECKS_055['delivery_rider_broadcast_config'],
    ), 'migration 055 broadcast checks differ')
    require(all(
        inspector.has_typed_columns(snapshot_value, table, expected)
        and inspector.has_indexes(snapshot_value, table, {'PRIMARY': ('id', True)})
        for table, expected in inspector.COLUMNS_056.items()
    ), 'migration 056 typed columns or primary keys differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_withdrawal_allocations',
        {
            'uq_delivery_rider_withdrawal_allocation_public_id': (
                'allocation_public_id', True
            ),
            'uq_delivery_rider_withdrawal_allocation_pair': (
                'withdraw_public_id,earning_public_id', True
            ),
        },
    ), 'migration 056 allocation indexes differ')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_withdrawal_allocations',
        inspector.CHECKS_056['delivery_rider_withdrawal_allocations'],
    ), 'migration 056 allocation checks differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_withdrawal_claims',
        {
            'uq_delivery_rider_withdrawal_claim_public_id': ('claim_public_id', True),
            'uq_delivery_rider_withdrawal_claim_withdraw': ('withdraw_public_id', True),
            'uq_delivery_rider_withdrawal_claim_key': ('claim_key', True),
            'uq_delivery_rider_withdrawal_claim_idempotency': ('idempotency_key', True),
        },
    ), 'migration 056 claim indexes differ')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_withdrawal_claims',
        inspector.CHECKS_056['delivery_rider_withdrawal_claims'],
    ), 'migration 056 claim checks differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_withdrawal_events',
        {
            'uq_delivery_rider_withdrawal_event_public_id': ('event_public_id', True),
            'uq_delivery_rider_withdrawal_event_idempotency': ('idempotency_key', True),
        },
    ), 'migration 056 event indexes differ')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_withdrawal_events',
        inspector.CHECKS_056['delivery_rider_withdrawal_events'],
    ), 'migration 056 event checks differ')
    require(inspector.has_indexes(
        snapshot_value,
        'delivery_rider_withdrawal_payment_references',
        {
            'uq_delivery_rider_withdrawal_payment_reference': ('payment_reference', True),
            'uq_delivery_rider_withdrawal_payment_withdraw': ('withdraw_public_id', True),
        },
    ), 'migration 056 payment-reference indexes differ')
    require(inspector.has_constraints(
        snapshot_value,
        'delivery_rider_withdrawal_payment_references',
        inspector.CHECKS_056['delivery_rider_withdrawal_payment_references'],
    ), 'migration 056 payment-reference checks differ')
    result = inspector.evaluate(snapshot_value)
    require(result['migration055Ready'], 'migration 055 schema validation failed')
    require(result['migration056Ready'], 'migration 056 schema validation failed')
    require(not result['flagConflicts'], 'new migration flag conflict')


def patch_env(changes):
    lines = ENV_FILE.read_text(encoding='utf-8').splitlines()
    remaining = dict(changes)
    for index, line in enumerate(lines):
        key = line.split('=', 1)[0].strip()
        if key in remaining and not line.lstrip().startswith('#'):
            lines[index] = key + '=' + remaining.pop(key)
    lines.extend(key + '=' + value for key, value in remaining.items())
    temporary = ENV_FILE.with_name('campus-api.env.rider-next')
    require(not temporary.exists() and not temporary.is_symlink(), 'pending environment exists')
    status = ENV_FILE.stat()
    with temporary.open('x', encoding='utf-8') as stream:
        os.fchmod(stream.fileno(), status.st_mode & 0o777)
        os.fchown(stream.fileno(), status.st_uid, status.st_gid)
        stream.write('\n'.join(lines) + '\n')
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, ENV_FILE)


def await_ready(commit):
    last_error = None
    for _ in range(20):
        try:
            probe('/readyz')
            version = probe('/version')['data']
            actual = version.get('commit', '')
            require(
                isinstance(actual, str)
                and len(actual) >= 12
                and (actual.startswith(commit) or commit.startswith(actual)),
                'unexpected running commit',
            )
            return version
        except Exception as error:
            last_error = error
            time.sleep(2)
    raise RuntimeError('candidate readiness failed') from last_error


def current_release():
    current = ROOT / 'current'
    require(current.is_symlink(), 'current is not a symlink')
    target = current.resolve(strict=True)
    require(target.parent == ROOT / 'releases', 'unsafe current release target')
    return current, target


def switch_current(current, target, suffix):
    pending = ROOT / ('current.' + suffix)
    require(not pending.exists() and not pending.is_symlink(), 'pending release transition exists')
    pending.symlink_to(target, target_is_directory=True)
    os.replace(pending, current)


def stage_assets(stage):
    require(stage == Path('/home/admin/campus-rider-1b919e5'), 'unexpected stage path')
    require(stage.is_dir() and not stage.is_symlink(), 'unsafe stage')
    manifest_path = stage / 'manifest.json'
    require(manifest_path.is_file() and not manifest_path.is_symlink(), 'manifest missing')
    require(sha256_file(manifest_path) == MANIFEST_SHA256, 'manifest checksum mismatch')
    manifest = json.loads(manifest_path.read_text(encoding='utf-8'))
    require(manifest.get('sourceCommit') == CANDIDATE_COMMIT, 'candidate commit mismatch')
    require(manifest.get('backendVersion') == RELEASE_VERSION, 'candidate version mismatch')
    binary = stage / 'campus-api'
    require(binary.is_file() and not binary.is_symlink(), 'candidate binary missing')
    require(sha256_file(binary) == CANDIDATE_BINARY_SHA256, 'candidate binary checksum mismatch')
    for name in MIGRATIONS:
        path = stage / name
        require(path.is_file() and not path.is_symlink(), 'migration missing')
        require(sha256_file(path) == MIGRATION_SHA256[name], 'migration checksum mismatch')
    return manifest


def latest_backup():
    backup_dir = Path('/var/backups/campus-circle') / RELEASE_ID
    require(backup_dir.is_dir() and not backup_dir.is_symlink(), 'backup directory missing')
    backups = sorted(backup_dir.glob('database-*.sql.gz'), key=lambda path: path.stat().st_mtime)
    require(backups, 'pre-migration backup missing')
    backup = backups[-1]
    require(backup.is_file() and not backup.is_symlink(), 'latest backup is unsafe')
    with gzip.open(backup, 'rb') as stream:
        dump = stream.read()
    require(dump and b'-- Dump completed on ' in dump[-4096:], 'latest backup is incomplete')
    return backup, dump


def finish_activated(stage, inspector, values, db):
    release = ROOT / 'releases' / RELEASE_ID
    require(release.is_dir() and not release.is_symlink(), 'migrated release directory missing')
    require(sha256_file(release / 'bin/campus-api') == CANDIDATE_BINARY_SHA256,
            'migrated release binary mismatch')
    require(run(['pgrep', '-c', '-x', 'campus-api'], text=True).strip() == '1',
            'unexpected API process count')
    running = probe('/version')['data']
    await_ready(CANDIDATE_COMMIT)
    assert_default_off(values)
    validate_new_schema(db, inspector, flag_values(values))

    backup, dump = latest_backup()
    verification_db = 'campus_finish_rider_1b919e5'
    require(scalar(
        db,
        "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='" +
        verification_db + "'",
    ) == 0, 'finish verification database already exists')
    try:
        sql(db, 'CREATE DATABASE `' + verification_db +
            '` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
        run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verification_db], input=dump)
        schema, before = snapshot(verification_db)
        require(len(schema) >= 90, 'unexpected backup table inventory')
        for name in MIGRATIONS:
            run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verification_db],
                input=(stage / name).read_bytes())
        validate_new_schema(verification_db, inspector, flag_values(values))
        require(snapshot(verification_db, schema)[1] == before,
                'finish rehearsal changed original rows')
    finally:
        if scalar(
            db,
            "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='" +
            verification_db + "'",
        ) == 1:
            sql(db, 'DROP DATABASE `' + verification_db + '`')

    current, _ = current_release()
    old = ROOT / 'releases' / OLD_RELEASE
    require(old.is_dir() and not old.is_symlink(), 'old release directory missing')
    switched = False
    try:
        if current.resolve(strict=True) != release:
            switch_current(current, release, 'rider-resume')
        switched = True
        run(['systemctl', 'restart', 'campus-api'])
        version = await_ready(CANDIDATE_COMMIT)
        after_inspector = read_inspector(
            stage / 'inspect-rider-release.py',
            '--expected-commit',
            CANDIDATE_COMMIT,
            '--require-ready',
            'candidate',
        )
        after_preflight = after_inspector.get('preflight', {})
        require(after_preflight.get('migration055Ready') and
                after_preflight.get('migration056Ready'), 'finish migration gate failed')
        require(after_preflight.get('defaultOffCandidateReady') is True,
                'finish default-off gate failed')
        require(not after_preflight.get('flagConflicts'), 'finish flag conflict')
    except Exception:
        if switched and current.resolve(strict=False) == release:
            switch_current(current, old, 'rider-finish-rollback')
            run(['systemctl', 'restart', 'campus-api'])
            await_ready(OLD_COMMIT)
        raise

    result = {
        'release': RELEASE_ID,
        'previousRelease': str(old),
        'commit': CANDIDATE_COMMIT,
        'version': RELEASE_VERSION,
        'candidateManifestSha256': MANIFEST_SHA256,
        'binarySha256': CANDIDATE_BINARY_SHA256,
        'deployed': True,
        'databaseMigrated': True,
        'featuresActivated': False,
        'resumedAfterPostStartIdentityCheck': True,
        'runningVersionBeforeResume': running,
        'versionData': version,
        'backup': str(backup),
        'backupSha256': sha256_file(backup),
        'backupUncompressedBytes': len(dump),
        'restoredBackupVerified': True,
        'migrationRehearsalPassed': True,
        'originalTableCount': len(schema),
        'originalRowsUnchanged': True,
        'migrations': MIGRATIONS,
        'flags': after_inspector['flags'],
        'notificationDeliveryCount': after_inspector['counts'].get(
            'delivery_notification_deliveries'
        ),
        'withdrawalRows': sum(
            after_inspector['counts'].get(table) or 0
            for table in inspector.MIGRATION_056_TABLES
        ),
    }
    receipt = release / 'rider-release-receipt.json'
    receipt.write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
    receipt.chmod(0o644)
    print(json.dumps(result))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--stage', required=True)
    parser.add_argument('--deploy', action='store_true')
    parser.add_argument('--finish-activated', action='store_true')
    args = parser.parse_args()
    require(os.geteuid() == 0, 'sudo required')
    require(not (args.deploy and args.finish_activated),
            'choose one mutation phase')
    lock = open('/run/campus-rider-055-056-release.lock', 'a', encoding='utf-8')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)

    stage = Path(args.stage)
    this_stage_manifest = stage_assets(stage)
    inspector_path = stage / 'inspect-rider-release.py'
    inspector = load_inspector(inspector_path)
    values = env_values()
    db = database(values)
    require(run(['timedatectl', 'show', '--property=Timezone', '--value'], text=True).strip()
            == 'Asia/Shanghai', 'host clock changed')
    if args.finish_activated:
        finish_activated(stage, inspector, values, db)
        return
    require(run(['pgrep', '-c', '-x', 'campus-api'], text=True).strip() == '1',
            'unexpected API process count')
    current, old = current_release()
    require(old == ROOT / 'releases' / OLD_RELEASE, 'production release changed')
    require(sha256_file(old / 'bin/campus-api') == OLD_BINARY_SHA256,
            'old binary checksum changed')
    require(probe('/version')['data'].get('commit') == OLD_COMMIT, 'production commit changed')
    probe('/readyz')
    before_inspector = read_inspector(
        inspector_path, '--expected-commit', OLD_COMMIT, '--require-ready', 'candidate'
    )
    before_preflight = before_inspector.get('preflight', {})
    require(before_preflight.get('defaultOffCandidateReady') is True,
            'candidate default-off gate failed')
    require(before_preflight.get('migration054Ready') is True,
            'migration 054 gate failed')
    require(before_preflight.get('migration055Ready') is False
            and before_preflight.get('migration056Ready') is False,
            '055/056 already or partially applied')
    require(not before_preflight.get('flagConflicts'), 'runtime flag conflict')
    assert_database_preconditions(db)
    assert_default_off(values)
    require(values.get('ENABLE_DELIVERY_PAYMENT') in ('true', '1'),
            'payment baseline changed')

    release = ROOT / 'releases' / RELEASE_ID
    require(not release.exists() and not release.is_symlink(), 'release already exists')
    require(shutil.disk_usage(ROOT).free > 2 * 1024 ** 3, 'insufficient disk space')
    result = {
        'release': RELEASE_ID,
        'previousRelease': str(old),
        'commit': CANDIDATE_COMMIT,
        'version': RELEASE_VERSION,
        'candidateManifestSha256': MANIFEST_SHA256,
        'binarySha256': CANDIDATE_BINARY_SHA256,
        'deployed': False,
        'databaseMigrated': False,
        'featuresActivated': False,
    }
    if not args.deploy:
        print(json.dumps(dict(result, preflight='PASS')))
        return

    backup_dir = Path('/var/backups/campus-circle') / RELEASE_ID
    backup_dir.mkdir(mode=0o700, exist_ok=True)
    backup_dir.chmod(0o700)
    attempt = time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())
    env_backup = backup_dir / ('campus-api.env-' + attempt)
    shutil.copy2(ENV_FILE, env_backup)
    env_backup.chmod(0o600)
    previous_features = probe('/api/v1/features')['data']
    api_stopped = False
    stage_switched = False
    production_mutated = False
    verification_db = 'campus_verify_rider_1b919e5'
    try:
        run(['systemctl', 'stop', 'campus-api'])
        api_stopped = True
        require(subprocess.run(['pgrep', '-x', 'campus-api'], capture_output=True).returncode == 1,
                'API writer remains after stop')
        require(scalar(
            db,
            "SELECT COUNT(*) FROM information_schema.PROCESSLIST "
            "WHERE DB=DATABASE() AND ID<>CONNECTION_ID()",
        ) == 0, 'database writer connection remains')
        assert_database_preconditions(db)
        schema, before = snapshot(db)
        require(len(schema) >= 90, 'unexpected production table inventory')

        dump = run([
            'mysqldump', *MYSQL, '--single-transaction', '--no-tablespaces',
            '--hex-blob', '--routines', '--events', '--triggers',
            '--default-character-set=utf8mb4', db,
        ])
        require(b'-- Dump completed on ' in dump[-4096:], 'incomplete backup dump')
        backup = backup_dir / ('database-' + attempt + '.sql.gz')
        with backup.open('xb') as raw:
            os.fchmod(raw.fileno(), 0o600)
            with gzip.GzipFile(fileobj=raw, mode='wb') as compressed:
                compressed.write(dump)
            raw.flush()
            os.fsync(raw.fileno())
        with gzip.open(backup, 'rb') as stream:
            require(stream.read() == dump, 'backup gzip content mismatch')

        require(scalar(
            db,
            "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='" +
            verification_db + "'",
        ) == 0, 'verification database already exists')
        sql(db, 'CREATE DATABASE `' + verification_db +
            '` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
        run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verification_db], input=dump)
        require(snapshot(verification_db, schema)[1] == before,
                'restored backup differs from stopped production')
        for name in MIGRATIONS:
            run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verification_db],
                input=(stage / name).read_bytes())
        validate_new_schema(verification_db, inspector, flag_values(values))
        require(snapshot(verification_db, schema)[1] == before,
                'migration rehearsal changed original rows')
        sql(db, 'DROP DATABASE `' + verification_db + '`')

        require(snapshot(db, schema)[1] == before,
                'production rows changed while API was stopped')
        for name in MIGRATIONS:
            run(['mysql', *MYSQL, '--default-character-set=utf8mb4', db],
                input=(stage / name).read_bytes())
            production_mutated = True
        validate_new_schema(db, inspector, flag_values(values))
        require(snapshot(db, schema)[1] == before,
                'migration changed original production rows')

        release.mkdir(mode=0o755)
        (release / 'bin').mkdir(mode=0o755)
        shutil.copyfile(stage / 'campus-api', release / 'bin/campus-api')
        (release / 'bin/campus-api').chmod(0o755)
        (release / 'RELEASE').write_text(
            RELEASE_ID + '\n' + CANDIDATE_COMMIT + '\n' + CANDIDATE_BINARY_SHA256 + '\n',
            encoding='utf-8',
        )
        (release / 'RELEASE').chmod(0o644)
        shutil.copy2(stage / 'manifest.json', release / 'candidate-manifest.json')
        shutil.copy2(inspector_path, release / 'inspect-rider-release.py')

        patch_env({
            'ENABLE_DELIVERY_RIDER_BROADCAST': 'false',
            'ENABLE_DELIVERY_RIDER_WITHDRAWAL': 'false',
            'ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE': 'false',
            'ENABLE_NOTIFICATION_SENDING': 'false',
        })
        switch_current(current, release, 'rider-next')
        stage_switched = True
        run(['systemctl', 'start', 'campus-api'])
        version = await_ready(CANDIDATE_COMMIT)
        require(probe('/api/v1/features')['data'] == previous_features,
                'public feature policy changed')
        after_inspector = read_inspector(
            inspector_path,
            '--expected-commit',
            CANDIDATE_COMMIT,
            '--require-ready',
            'candidate',
        )
        after_preflight = after_inspector.get('preflight', {})
        require(after_preflight.get('migration055Ready') and
                after_preflight.get('migration056Ready'), 'deployed migration gate failed')
        require(after_preflight.get('defaultOffCandidateReady') is True,
                'deployed default-off gate failed')
        require(not after_preflight.get('flagConflicts'), 'deployed flag conflict')
        result.update(
            deployed=True,
            databaseMigrated=True,
            versionData=version,
            backup=str(backup),
            backupSha256=sha256_file(backup),
            backupUncompressedBytes=len(dump),
            restoredBackupVerified=True,
            migrationRehearsalPassed=True,
            originalTableCount=len(schema),
            originalRowsUnchanged=True,
            migrations=MIGRATIONS,
            flags=after_inspector['flags'],
            notificationDeliveryCount=after_inspector['counts'].get(
                'delivery_notification_deliveries'
            ),
            withdrawalRows=sum(
                after_inspector['counts'].get(table) or 0
                for table in inspector.MIGRATION_056_TABLES
            ),
        )
        receipt = release / 'rider-release-receipt.json'
        receipt.write_text(json.dumps(result, indent=2) + '\n', encoding='utf-8')
        receipt.chmod(0o644)
        print(json.dumps(result))
    except Exception:
        if stage_switched and current.resolve(strict=False) == release:
            switch_current(current, old, 'rider-rollback')
            run(['systemctl', 'start', 'campus-api'])
            await_ready(OLD_COMMIT)
        elif api_stopped and not production_mutated:
            run(['systemctl', 'start', 'campus-api'])
            await_ready(OLD_COMMIT)
        raise
    finally:
        if re.fullmatch(r'campus_verify_rider_[a-f0-9]+', verification_db):
            existing = scalar(
                db,
                "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='" +
                verification_db + "'",
            )
            if existing == 1:
                sql(db, 'DROP DATABASE `' + verification_db + '`')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        safe_reason = (
            str(error)
            if isinstance(error, RuntimeError)
            else 'missing key: ' + str(error.args[0])
            if isinstance(error, KeyError) and len(error.args) == 1
            else 'details withheld'
        )
        print(json.dumps({
            'deployed': False,
            'failed': True,
            'errorType': type(error).__name__,
            'reason': safe_reason,
            'automaticDatabaseRollback': False,
        }))
        raise SystemExit(1)
