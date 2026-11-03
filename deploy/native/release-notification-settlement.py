#!/usr/bin/env python3
"""Guarded 052/053/054 release for the inspected empty-rider-ledger baseline.

Default is read-only. No automatic schema or binary rollback after migration.
Secrets and row contents never enter receipts or stdout.
"""
import argparse
import fcntl
import gzip
import hashlib
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
ENV = Path('/etc/campus-circle/campus-api.env')
MYSQL = ['--defaults-extra-file=/etc/campus-circle/mysql-admin.cnf']
OLD_RELEASE = '20261030-wechat-payment-7d22416'
OLD_COMMIT = '7d22416948be'
MIGRATIONS = ['052_delivery_recharge_cancellation.sql',
              '053_delivery_notifications.sql', '054_delivery_rider_daily_settlement.sql']
NEW_TABLES = {'delivery_notification_events', 'delivery_notification_inbox',
              'delivery_notification_deliveries', 'notification_subscriptions', 'im_notification_receipts'}
EMPTY_MONEY = ['delivery_rider_earning_records', 'delivery_rider_wallets',
               'delivery_rider_ledger', 'delivery_rider_withdrawals',
               'delivery_rider_payout_reservations', 'delivery_rider_payout_queue']


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def run(cmd, **kwargs):
    p = subprocess.run(cmd, capture_output=True, **kwargs)
    require(p.returncode == 0, cmd[0] + ' failed; sensitive output withheld')
    return p.stdout


def sql(db, query):
    return run(['mysql', *MYSQL, '--default-character-set=utf8mb4', '--batch',
                '--skip-column-names', db, '-e', query], text=True).strip()


def env_values():
    values = {}
    for line in ENV.read_text().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            k, v = line.split('=', 1)
            require(k.strip() not in values, 'Duplicate environment key')
            values[k.strip()] = v.strip().strip('"').strip("'")
    return values


def database(values):
    m = re.search(r'@tcp\([^)]+\)/([A-Za-z0-9_]+)\?(.*)$', values['MYSQL_DSN'])
    require(m is not None, 'Unrecognized DSN; details withheld')
    params = urllib.parse.parse_qs(m.group(2))
    require(params.get('loc') == ['Local'] and params.get('parseTime', [''])[0].lower() == 'true',
            'Inspected Local parseTime storage contract changed')
    require(params.get('time_zone') in (None, ["'+08:00'"]), 'Unexpected existing session clock')
    return m.group(1)


def probe(path, expected=200):
    try:
        response = urllib.request.urlopen('http://127.0.0.1:8080' + path, timeout=5)
    except urllib.error.HTTPError as e:
        response = e
    with response:
        require(response.status == expected, path + ': unexpected HTTP ' + str(response.status))
        return json.load(response)


def scalar(db, query):
    return int(sql(db, query))


def assert_empty_money(db):
    for table in EMPTY_MONEY:
        require(scalar(db, 'SELECT COUNT(*) FROM ' + table) == 0,
                'Money baseline changed; inspect before proceeding: ' + table)


def columns(db, table):
    require(re.fullmatch('[A-Za-z0-9_]+', table) is not None, 'Invalid table identifier')
    return sql(db, "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='" + table + "' ORDER BY ORDINAL_POSITION").splitlines()


def snapshot(db, schema=None):
    if schema is None:
        tables = sql(db, 'SHOW TABLES').splitlines()
        schema = {t: columns(db, t) for t in tables}
    digests = {}
    for table, cols in schema.items():
        require(all(re.fullmatch('[A-Za-z0-9_]+', c) for c in cols), 'Invalid column identifier')
        pk_rows = sql(db, "SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='" + table + "' AND INDEX_NAME='PRIMARY' ORDER BY SEQ_IN_INDEX").splitlines()
        require(pk_rows, 'Cannot create stable digest without a primary key: ' + table)
        # HEX preserves binary values, NULL marker differs from an empty string.
        parts = ["IF(`" + c + "` IS NULL,'N',CONCAT('V',HEX(`" + c + "`)))" for c in cols]
        order_by = ','.join('`' + c + '`' for c in pk_rows)
        rows = run(['mysql', *MYSQL, '--default-character-set=utf8mb4', '--batch',
                    '--skip-column-names', db, '-e', 'SELECT CONCAT_WS(\'|\',' + ','.join(parts) + ') AS r FROM `' + table + '` ORDER BY ' + order_by])
        digests[table] = hashlib.sha256(rows).hexdigest()
    return schema, digests


def schema_before(db):
    require('cancel_requested_at' not in columns(db, 'delivery_payment_attempts'), '052 already/partially applied')
    require('settle_at' not in columns(db, 'delivery_rider_earning_records'), '054 already/partially applied')
    require(not NEW_TABLES.intersection(sql(db, 'SHOW TABLES').splitlines()), '053 already/partially applied')
    assert_empty_money(db)


def schema_after(db):
    require('cancel_requested_at' in columns(db, 'delivery_payment_attempts'), '052 incomplete')
    require(sql(db, "SELECT DATA_TYPE,IS_NULLABLE,DATETIME_PRECISION FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='delivery_payment_attempts' AND COLUMN_NAME='cancel_requested_at'") == 'datetime\tYES\t3', '052 cancel_requested_at definition mismatch')
    require(NEW_TABLES.issubset(sql(db, 'SHOW TABLES').splitlines()), '053 incomplete')
    for column, nullable in [('settle_at', 'NO'), ('settled_at', 'YES')]:
        require(sql(db, "SELECT DATA_TYPE,IS_NULLABLE,DATETIME_PRECISION FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='delivery_rider_earning_records' AND COLUMN_NAME='" + column + "'") == 'datetime\t' + nullable + '\t3', '054 ' + column + ' definition mismatch')
    require(sql(db, "SELECT GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='delivery_rider_earning_records' AND INDEX_NAME='idx_delivery_rider_earning_due'") == 'settlement_status,settle_at,id', '054 index incomplete')


def patch_env(changes):
    lines = ENV.read_text().splitlines()
    remaining = dict(changes)
    for i, line in enumerate(lines):
        key = line.split('=', 1)[0].strip()
        if key in remaining and not line.lstrip().startswith('#'):
            lines[i] = key + '=' + remaining.pop(key)
    lines.extend(k + '=' + v for k, v in remaining.items())
    temp = ENV.with_name('campus-api.env.notification-next')
    require(not temp.exists(), 'Pending environment transition exists')
    stat = ENV.stat()
    with temp.open('x') as f:
        os.fchmod(f.fileno(), stat.st_mode & 0o777)
        os.fchown(f.fileno(), stat.st_uid, stat.st_gid)
        f.write('\n'.join(lines) + '\n')
        f.flush()
        os.fsync(f.fileno())
    os.replace(temp, ENV)


def await_ready(commit):
    for attempt in range(15):
        try:
            probe('/readyz')
            data = probe('/version')['data']
            require(data['commit'] == commit, 'Unexpected running candidate')
            return data
        except Exception:
            if attempt == 14:
                raise
            time.sleep(2)


def process_flags():
    pid = run(['systemctl', 'show', 'campus-api', '--property=MainPID', '--value'], text=True).strip()
    raw = Path('/proc/' + pid + '/environ').read_bytes().split(b'\0')
    keys = ('ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT', 'ENABLE_USER_NOTIFICATIONS',
            'ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS', 'ENABLE_NOTIFICATION_SENDING',
            'ENABLE_DELIVERY_PAYMENT')
    values = {x.split(b'=', 1)[0].decode(): x.split(b'=', 1)[1].decode() for x in raw if b'=' in x}
    return {k: values.get(k, 'UNSET') for k in keys}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--stage', required=True)
    p.add_argument('--deploy', action='store_true')
    p.add_argument('--enable-settlement', action='store_true')
    args = p.parse_args()
    require(os.geteuid() == 0, 'sudo required')
    lock = open('/run/campus-notification-settlement-release.lock', 'a')
    fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    stage = Path(args.stage)
    require(stage.parent == Path('/home/admin') and re.fullmatch('campus-notification-[A-Za-z0-9._-]+', stage.name), 'Invalid stage')
    require(not stage.is_symlink(), 'Unexpected stage symlink')
    manifest = json.loads((stage / 'manifest.json').read_text())
    release_id, commit = manifest['release'], manifest['commit']
    require(re.fullmatch('[A-Za-z0-9._-]{1,80}', release_id) and re.fullmatch('[a-f0-9]{12}', commit), 'Invalid release identity')
    for name in ['campus-api', *MIGRATIONS]:
        asset = stage / name
        require(not asset.is_symlink() and asset.is_file(), 'Invalid asset')
        require(hashlib.sha256(asset.read_bytes()).hexdigest() == manifest['sha256'][name], 'Asset checksum mismatch: ' + name)
    values = env_values()
    db = database(values)
    require(run(['timedatectl', 'show', '--property=Timezone', '--value'], text=True).strip() == 'Asia/Shanghai', 'Host clock changed')
    require(sql(db, 'SELECT TIMESTAMPDIFF(MINUTE,UTC_TIMESTAMP(),NOW())') == '480', 'DB clock changed')
    release = ROOT / 'releases' / release_id
    current = ROOT / 'current'
    receipt_path = release / 'notification-settlement-receipt.json'
    if args.enable_settlement:
        require(not args.deploy, 'Choose one mutation phase')
        require(current.resolve() == release and receipt_path.is_file(), 'No verified deployed candidate')
        require(probe('/version')['data']['commit'] == commit, 'Candidate changed')
        receipt = json.loads(receipt_path.read_text())
        require(receipt.get('deployed') is True and not receipt.get('settlementEnabled'), 'Invalid enable phase')
        schema_after(db)
        assert_empty_money(db)
        flags_before = process_flags()
        require(flags_before['ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT'] == 'false' and flags_before['ENABLE_USER_NOTIFICATIONS'] == 'true' and flags_before['ENABLE_NOTIFICATION_SENDING'] == 'false', 'Enable phase runtime flags are not the verified pre-settlement state')
        before = snapshot(db, {t: columns(db, t) for t in EMPTY_MONEY})
        patch_env({'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT': 'true'})
        run(['systemctl', 'restart', 'campus-api'])
        receipt['version'] = await_ready(commit)
        time.sleep(6)
        require(snapshot(db, before[0])[1] == before[1], 'Unexpected ledger change on empty baseline')
        flags = process_flags()
        require(flags['ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT'] == 'true' and flags['ENABLE_NOTIFICATION_SENDING'] == 'false', 'Runtime flag mismatch')
        receipt.update(settlementEnabled=True, flags=flags, emptyLedgerUnchanged=True)
        receipt_path.write_text(json.dumps(receipt, indent=2) + '\n')
        print(json.dumps(receipt))
        return
    require(current.is_symlink() and current.resolve() == ROOT / 'releases' / OLD_RELEASE, 'Production release changed')
    require(probe('/version')['data']['commit'] == OLD_COMMIT, 'Production commit changed')
    probe('/readyz')
    require(not release.exists(), 'Release already exists')
    require(shutil.disk_usage(ROOT).free > 1024**3, 'Insufficient backup space')
    schema_before(db)
    for name in ['EVENTS', 'TRIGGERS', 'ROUTINES']:
        column = {'EVENTS': 'EVENT_SCHEMA', 'TRIGGERS': 'TRIGGER_SCHEMA', 'ROUTINES': 'ROUTINE_SCHEMA'}[name]
        require(scalar(db, 'SELECT COUNT(*) FROM information_schema.' + name + ' WHERE ' + column + '=DATABASE()') == 0, 'Uninspected database writer')
    for key in ['ALLOW_MOCK_WECHAT_LOGIN', 'ALLOW_MOCK_DELIVERY_PAYMENT', 'ALLOW_MOCK_PAYMENT', 'ALLOW_MOCK_DELIVERY_FEE_CONFIRMATION']:
        require(values.get(key, 'false').lower() == 'false', 'Mock policy mismatch')
    require(values.get('ENABLE_DELIVERY_PAYMENT') == 'true', 'Payment baseline changed')
    require(subprocess.run(['pgrep', '-c', '-x', 'campus-api'], capture_output=True, text=True).stdout.strip() == '1', 'Unexpected API process count')
    result = {'release': release_id, 'commit': commit, 'deployed': False, 'settlementEnabled': False}
    if not args.deploy:
        print(json.dumps(dict(result, preflight='PASS')))
        return
    previous_features = probe('/api/v1/features')['data']
    backup_dir = Path('/var/backups/campus-circle') / release_id
    backup_dir.mkdir(mode=0o700)
    shutil.copy2(ENV, backup_dir / 'campus-api.env')
    os.chmod(backup_dir / 'campus-api.env', 0o600)
    run(['systemctl', 'stop', 'campus-api'])
    require(subprocess.run(['pgrep', '-x', 'campus-api'], capture_output=True).returncode == 1, 'API writer remains')
    require(scalar(db, 'SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE() AND ID<>CONNECTION_ID()') == 0, 'Database connections remain; leave stopped')
    schema_before(db)
    schema, before = snapshot(db)
    dump = run(['mysqldump', *MYSQL, '--single-transaction', '--no-tablespaces',
                '--hex-blob', '--routines', '--events', '--default-character-set=utf8mb4', db])
    require(b'-- Dump completed on ' in dump[-1000:], 'Incomplete backup dump')
    backup = backup_dir / 'database.sql.gz'
    with backup.open('xb') as raw:
        os.fchmod(raw.fileno(), 0o600)
        with gzip.GzipFile(fileobj=raw, mode='wb') as out:
            out.write(dump)
    with gzip.open(backup, 'rb') as stream:
        require(stream.read() == dump, 'Full backup CRC/content mismatch')
    verify_db = 'campus_verify_notify_' + commit
    require(scalar(db, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='" + verify_db + "'") == 0, 'Verification database already exists')
    sql(db, 'CREATE DATABASE `' + verify_db + '` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci')
    run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verify_db], input=dump)
    require(snapshot(verify_db, schema)[1] == before, 'Restored backup differs; leave stopped')
    for name in MIGRATIONS:
        run(['mysql', *MYSQL, '--default-character-set=utf8mb4', verify_db], input=(stage / name).read_bytes())
    schema_after(verify_db)
    require(snapshot(verify_db, schema)[1] == before, 'Rehearsal changed old rows')
    # Only this freshly-created, ungranted verification database is removed.
    sql(db, 'DROP DATABASE `' + verify_db + '`')
    require(snapshot(db, schema)[1] == before, 'Production changed despite stopped writers')
    for name in MIGRATIONS:
        run(['mysql', *MYSQL, '--default-character-set=utf8mb4', db], input=(stage / name).read_bytes())
    schema_after(db)
    require(snapshot(db, schema)[1] == before, 'Migration changed baseline rows; leave stopped')
    release.mkdir(mode=0o755)
    (release / 'bin').mkdir(mode=0o755)
    shutil.copyfile(stage / 'campus-api', release / 'bin/campus-api')
    (release / 'bin/campus-api').chmod(0o755)
    dsn = values['MYSQL_DSN']
    if 'time_zone=' not in dsn:
        dsn += '&time_zone=%27%2B08%3A00%27'
    patch_env({'MYSQL_DSN': dsn, 'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT': 'false',
               'ENABLE_USER_NOTIFICATIONS': 'true', 'ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS': 'false',
               'ENABLE_NOTIFICATION_SENDING': 'false'})
    pending = ROOT / 'current.notification-next'
    require(not os.path.lexists(pending), 'Pending release transition exists')
    pending.symlink_to(release, target_is_directory=True)
    os.replace(pending, current)
    run(['systemctl', 'start', 'campus-api'])
    version = await_ready(commit)
    require(probe('/api/v1/features')['data'] == previous_features, 'Unexpected public feature policy change')
    probe('/api/v1/notification/subscribe/templates', 401)
    probe('/api/v1/admin/share-settings', 401)
    result.update(deployed=True, version=version, flags=process_flags(),
                  backup=str(backup), backupSha256=hashlib.sha256(backup.read_bytes()).hexdigest(),
                  restoredBackupVerified=True, migrationRehearsalPassed=True,
                  originalTableCount=len(schema), originalRowsUnchanged=True,
                  migrations=MIGRATIONS, storageClock='Local/+08:00 unchanged',
                  wechatSendingEnabled=False)
    receipt_path.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Never automatically return to the pre-054 real-time-credit binary.
        print(json.dumps({'failed': True, 'errorType': type(error).__name__,
                          'reason': str(error) if isinstance(error, RuntimeError) else 'details withheld',
                          'automaticRollback': False}))
        raise SystemExit(1)
