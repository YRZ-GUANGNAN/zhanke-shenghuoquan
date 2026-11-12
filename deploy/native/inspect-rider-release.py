#!/usr/bin/env python3
"""Read-only rider release inventory and 054/055/056 activation preflight.

The script never mutates the database, environment file, release symlink or
service. By default it inspects the live host. ``--fixture`` evaluates a
sanitised JSON snapshot and exists only so the fail-closed rules can be tested
without production access.

An inventory without arguments is not a release acceptance. Use
``--expected-commit`` to bind the runtime to the frozen candidate, and
``--require-ready`` to require a specific preflight gate with a nonzero exit
status on failure. These options remain read-only and never activate a feature.
"""
import argparse
import json
from pathlib import Path
import re
import subprocess
import urllib.request


ROOT = Path('/opt/campus-circle')
MYSQL = ['--defaults-extra-file=/etc/campus-circle/mysql-admin.cnf']
MONEY_TABLES = [
    'delivery_rider_earning_records', 'delivery_rider_wallets',
    'delivery_rider_ledger', 'delivery_rider_withdrawals',
    'delivery_rider_payout_records', 'delivery_rider_payout_queue',
    'delivery_rider_payout_reservations',
]
MIGRATION_055_TABLES = [
    'delivery_rider_availability', 'delivery_rider_broadcast_config',
]
MIGRATION_056_TABLES = [
    'delivery_rider_withdrawal_allocations',
    'delivery_rider_withdrawal_claims',
    'delivery_rider_withdrawal_events',
    'delivery_rider_withdrawal_payment_references',
]
NOTIFICATION_TABLES = [
    'delivery_notification_deliveries', 'im_notification_receipts',
]
BOOLEAN_FLAG_NAMES = [
    'ENABLE_DELIVERY', 'ENABLE_DELIVERY_PAYMENT',
    'ENABLE_DELIVERY_RIDER_BROADCAST',
    'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT',
    'ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE',
    'ENABLE_DELIVERY_RIDER_WITHDRAWAL', 'ENABLE_USER_NOTIFICATIONS',
    'ENABLE_WECHAT_SUBSCRIBE_NOTIFICATIONS',
    'ENABLE_NOTIFICATION_SENDING',
]
FLAG_NAMES = BOOLEAN_FLAG_NAMES + ['WECHAT_SUBSCRIBE_MINIPROGRAM_STATE']
BOOLEAN_DEFAULTS = {'ENABLE_DELIVERY': True}
ROW_COLUMNS = {'id': 'bigint unsigned', 'created_at': 'datetime(3)'}
MUTABLE_ROW_COLUMNS = {**ROW_COLUMNS, 'updated_at': 'datetime(3)'}
COLUMNS_055 = {
    'delivery_rider_availability': {
        **MUTABLE_ROW_COLUMNS, 'rider_user_id': 'bigint unsigned', 'state': 'varchar(16)',
        'scope': 'varchar(16)', 'eligibility_revision': 'varchar(128)', 'row_version': 'bigint unsigned',
    },
    'delivery_rider_broadcast_config': {
        **MUTABLE_ROW_COLUMNS, 'config_scope': 'varchar(16)',
        'max_in_flight_per_rider': 'int unsigned', 'row_version': 'bigint unsigned',
    },
}
COLUMNS_056 = {
    'delivery_rider_withdrawal_allocations': {
        **MUTABLE_ROW_COLUMNS, 'allocation_public_id': 'char(26)', 'withdraw_public_id': 'char(26)',
        'earning_public_id': 'char(26)', 'delivery_order_public_id': 'char(26)',
        'rider_user_id': 'bigint unsigned', 'wallet_public_id': 'char(26)',
        'amount_cents': 'bigint unsigned', 'currency': 'varchar(8)', 'status': 'varchar(16)',
        'source_policy_version': 'varchar(32)', 'source_evidence_hash': 'char(64)',
        'earning_row_version': 'bigint unsigned',
    },
    'delivery_rider_withdrawal_claims': {
        **MUTABLE_ROW_COLUMNS, 'claim_public_id': 'char(26)', 'withdraw_public_id': 'char(26)',
        'operator_user_id': 'bigint unsigned', 'claim_key': 'char(26)',
        'expected_row_version': 'bigint unsigned', 'status': 'varchar(16)',
        'reason': 'varchar(256)', 'payment_reference': 'varchar(128)', 'idempotency_key': 'char(26)',
    },
    'delivery_rider_withdrawal_events': {
        **ROW_COLUMNS, 'event_public_id': 'char(26)', 'withdraw_public_id': 'char(26)',
        'event_type': 'varchar(24)', 'operator_user_id': 'bigint unsigned',
        'reason': 'varchar(256)', 'payment_reference': 'varchar(128)', 'claim_key': 'varchar(26)',
        'request_hash': 'char(64)', 'idempotency_key': 'char(26)', 'expected_row_version': 'bigint unsigned',
    },
    'delivery_rider_withdrawal_payment_references': {
        **ROW_COLUMNS, 'payment_reference': 'varchar(128)', 'withdraw_public_id': 'char(26)',
    },
}

# These are the actual migration predicates, not merely constraint names.
# The metadata collector also checks ENFORCED: a same-named CHECK (1), or a
# disabled CHECK, must never satisfy a money-safety migration gate.
CHECKS_055 = {
    'delivery_rider_availability': {
        'chk_delivery_rider_availability_state': "state IN ('off_duty', 'on_duty')",
        'chk_delivery_rider_availability_scope': "scope = 'campus'",
        'chk_delivery_rider_availability_revision': 'CHAR_LENGTH(eligibility_revision) BETWEEN 1 AND 128',
        'chk_delivery_rider_availability_identity': 'rider_user_id > 0 AND row_version > 0',
    },
    'delivery_rider_broadcast_config': {
        'chk_delivery_rider_broadcast_config_scope': "config_scope = 'global'",
        'chk_delivery_rider_broadcast_config_limit': 'max_in_flight_per_rider BETWEEN 1 AND 100',
        'chk_delivery_rider_broadcast_config_version': 'row_version > 0',
    },
}
CHECKS_056 = {
    'delivery_rider_withdrawal_allocations': {
        'chk_delivery_rider_withdrawal_allocation_amount': 'amount_cents > 0 AND amount_cents <= 9007199254740991',
        'chk_delivery_rider_withdrawal_allocation_currency': "currency = 'CNY'",
        'chk_delivery_rider_withdrawal_allocation_status': "status IN ('in_flight', 'paid', 'released')",
        'chk_delivery_rider_withdrawal_allocation_source': 'CHAR_LENGTH(TRIM(source_policy_version)) > 0 AND CHAR_LENGTH(source_evidence_hash) = 64',
    },
    'delivery_rider_withdrawal_claims': {
        'chk_delivery_rider_withdrawal_claim_status': "status IN ('active', 'released', 'completed', 'exception')",
        'chk_delivery_rider_withdrawal_claim_operator': 'operator_user_id > 0 AND expected_row_version > 0',
    },
    'delivery_rider_withdrawal_events': {
        'chk_delivery_rider_withdrawal_event_type': "event_type IN ('claimed', 'released', 'paid', 'rejected', 'exception')",
        'chk_delivery_rider_withdrawal_event_operator': 'operator_user_id > 0 AND expected_row_version > 0 AND CHAR_LENGTH(request_hash) = 64',
    },
    'delivery_rider_withdrawal_payment_references': {
        'chk_delivery_rider_withdrawal_payment_reference': 'CHAR_LENGTH(TRIM(payment_reference)) > 0',
    },
}


def run(command):
    result = subprocess.run(command, capture_output=True, text=True, timeout=15)
    if result.returncode:
        raise RuntimeError('Read-only command failed; details withheld')
    return result.stdout.strip()


def bool_flag(value, fallback=False):
    # Match config.envBool exactly. Invalid case/whitespace is an error in Go,
    # not permission to silently interpret the deployment differently.
    if value == '':
        return fallback
    if value in ('true', '1'):
        return True
    if value in ('false', '0'):
        return False
    raise ValueError('invalid boolean flag')


def table_shape(sql, table):
    columns = sql(
        "SELECT COLUMN_NAME,DATA_TYPE,IS_NULLABLE,COALESCE(DATETIME_PRECISION,''),"
        "COALESCE(CHARACTER_MAXIMUM_LENGTH,''),COLUMN_TYPE "
        "FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() "
        "AND TABLE_NAME='" + table + "' ORDER BY ORDINAL_POSITION"
    ).splitlines()
    indexes = sql(
        "SELECT INDEX_NAME,NON_UNIQUE,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) "
        "FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() "
        "AND TABLE_NAME='" + table + "' GROUP BY INDEX_NAME,NON_UNIQUE ORDER BY INDEX_NAME"
    ).splitlines()
    constraints = sql(
        "SELECT t.CONSTRAINT_NAME,t.CONSTRAINT_TYPE,t.ENFORCED,COALESCE(c.CHECK_CLAUSE,'') "
        "FROM information_schema.TABLE_CONSTRAINTS t "
        "LEFT JOIN information_schema.CHECK_CONSTRAINTS c "
        "ON c.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA AND c.CONSTRAINT_NAME=t.CONSTRAINT_NAME "
        "WHERE t.TABLE_SCHEMA=DATABASE() AND t.TABLE_NAME='" + table + "' "
        "ORDER BY t.CONSTRAINT_NAME"
    ).splitlines()
    return {'columns': columns, 'indexes': indexes, 'constraints': constraints}


def commit_prefix_matches(left, right):
    return (
        isinstance(left, str)
        and isinstance(right, str)
        and len(left) >= 12
        and len(right) >= 12
        and (left.startswith(right) or right.startswith(left))
    )


def release_metadata(current, version_commit):
    target = current.resolve()
    safe_target = current.is_symlink() and target.parent == ROOT / 'releases'
    candidates = []
    release_file = target / 'RELEASE'
    if safe_target and release_file.is_file() and not release_file.is_symlink():
        lines = release_file.read_text(encoding='utf-8').splitlines()
        if len(lines) >= 2:
            candidates.append({'source': 'RELEASE', 'release': lines[0], 'commit': lines[1]})
    if safe_target:
        for receipt in sorted(target.glob('*-receipt.json')):
            if receipt.is_symlink() or not receipt.is_file():
                continue
            try:
                value = json.loads(receipt.read_text(encoding='utf-8'))
            except (OSError, ValueError):
                continue
            release = value.get('release', '')
            commit = value.get('commit', '')
            if not commit and isinstance(value.get('version'), dict):
                commit = value['version'].get('commit', '')
            if isinstance(release, str) and isinstance(commit, str):
                candidates.append({'source': receipt.name, 'release': release, 'commit': commit})
    match = next((item for item in candidates
                  if item['release'] == target.name
                  and len(item['commit']) >= 12
                  and commit_prefix_matches(version_commit, item['commit'])), None)
    return {
        'currentIsSafeReleaseSymlink': safe_target,
        'targetName': target.name if safe_target else '',
        'identitySource': match['source'] if match else '',
        'identityMatchesRuntime': match is not None,
    }


def collect_live_snapshot():
    pid = run(['systemctl', 'show', 'campus-api', '--property=MainPID', '--value'])
    if not pid.isdigit() or pid == '0':
        raise RuntimeError('API process unavailable')
    entries = Path('/proc/' + pid + '/environ').read_bytes().split(b'\0')
    values = dict(item.decode().split('=', 1) for item in entries if b'=' in item)
    match = re.search(r'@tcp\([^)]+\)/([A-Za-z0-9_]+)\?', values.get('MYSQL_DSN', ''))
    if not match:
        raise RuntimeError('Unrecognized database configuration; details withheld')
    db = match.group(1)

    def sql(query):
        return run(['mysql', *MYSQL, '--batch', '--skip-column-names', db, '-e', query])

    tables = set(sql('SHOW TABLES').splitlines())
    tracked = MONEY_TABLES + MIGRATION_055_TABLES + MIGRATION_056_TABLES + NOTIFICATION_TABLES
    counts = {name: int(sql('SELECT COUNT(*) FROM `' + name + '`')) if name in tables else None
              for name in tracked}
    shapes = {name: table_shape(sql, name) for name in tracked if name in tables}
    with urllib.request.urlopen('http://127.0.0.1:8080/version', timeout=8) as response:
        version = json.load(response)
    with urllib.request.urlopen('http://127.0.0.1:8080/readyz', timeout=8) as response:
        ready = response.status
    version_data = version.get('data', {}) if isinstance(version, dict) else {}
    metadata = release_metadata(ROOT / 'current', str(version_data.get('commit', '')))
    quota = (sql("SELECT template_key,status,SUM(remaining_quota),COUNT(*) "
                 "FROM notification_subscriptions GROUP BY template_key,status")
             if 'notification_subscriptions' in tables else '')
    order_counts = (sql('SELECT status,COUNT(*) FROM delivery_orders GROUP BY status')
                    if 'delivery_orders' in tables else '')
    notification_delivery_summary = (sql(
        "SELECT channel,purpose,status,last_error_code,COUNT(*) "
        "FROM delivery_notification_deliveries "
        "GROUP BY channel,purpose,status,last_error_code "
        "ORDER BY channel,purpose,status,last_error_code"
    ) if 'delivery_notification_deliveries' in tables else '')
    im_receipt_summary = (sql(
        "SELECT status,conversation_type,COUNT(*) "
        "FROM im_notification_receipts GROUP BY status,conversation_type "
        "ORDER BY status,conversation_type"
    ) if 'im_notification_receipts' in tables else '')
    template_keys = [name for name in values if 'SUBSCRIBE' in name and 'TEMPLATE' in name]
    allowlist_count = 0
    for name in values:
        if 'NOTIFICATION' in name and ('ALLOWLIST' in name or 'WHITELIST' in name):
            allowlist_count += len([item for item in values[name].split(',') if item.strip()])
    return {
        'version': version, 'readyHTTP': ready, 'releaseMetadata': metadata,
        # Preserve absent values; evaluate applies the backend defaults
        # (delivery=true, money/broadcast switches=false, target=formal).
        'flags': {name: values.get(name, '') for name in FLAG_NAMES},
        'counts': counts, 'shapes': shapes,
        'configuredTemplateVariables': {name: bool(values[name]) for name in template_keys},
        'imCallbackTokenConfigured': bool(values.get('NOTIFICATION_IM_CALLBACK_TOKEN')),
        'notificationAllowlistCount': allowlist_count,
        'subscriptionSummary': quota.splitlines(), 'orderStatusCounts': order_counts.splitlines(),
        # Aggregate-only summaries deliberately omit recipient/provider IDs and
        # payloads, while exposing enough state to distinguish accepted,
        # delivered, unknown, retry and deadletter outcomes during rollout.
        'notificationDeliverySummary': notification_delivery_summary.splitlines(),
        'imReceiptSummary': im_receipt_summary.splitlines(),
    }


def parsed_shape(snapshot, table):
    raw = snapshot.get('shapes', {}).get(table, {})
    columns = {}
    for line in raw.get('columns', []):
        values = line.split('\t')
        if len(values) >= 6:
            columns[values[0]] = values[1:]
    indexes = {}
    for line in raw.get('indexes', []):
        values = line.split('\t')
        if len(values) == 3:
            indexes[values[0]] = {'nonUnique': values[1], 'columns': values[2]}
    constraints = {}
    for line in raw.get('constraints', []):
        values = line.split('\t', 3)
        if len(values) == 4:
            constraints[values[0]] = {
                'type': values[1], 'enforced': values[2], 'clause': values[3],
            }
    return columns, indexes, constraints


def has_columns(snapshot, table, names):
    columns, _, _ = parsed_shape(snapshot, table)
    return all(name in columns for name in names)


def has_typed_columns(snapshot, table, expected):
    columns, _, _ = parsed_shape(snapshot, table)
    # Every 055/056 field is NOT NULL. A same-named nullable or lossy numeric
    # column is not the approved schema, even if its CHECK/unique names match.
    return all(name in columns and columns[name][1] == 'NO'
               and columns[name][4].lower() == column_type
               for name, column_type in expected.items())


def has_indexes(snapshot, table, expected):
    _, indexes, _ = parsed_shape(snapshot, table)
    return all(name in indexes and indexes[name]['columns'] == columns
               and indexes[name]['nonUnique'] == ('0' if unique else '1')
               for name, (columns, unique) in expected.items())


def check_expression(expression):
    """Parse only the comparison/IN/BETWEEN/AND grammar used by 055/056.

    MySQL adds identifier quoting, charset introducers and parentheses. Compare
    syntax trees, not stripped strings: removing all parentheses would accept
    a different expression such as a > (0 AND b) <= 100. Unknown SQL fails closed.
    """
    # MySQL 8.0.46 can escape the quote after a charset introducer with one or
    # more backslashes, while 8.4 returns the same expression unescaped.
    expression = re.sub(r"\\+'", "'", expression)
    token_pattern = re.compile(r"\s+|`[A-Za-z_][A-Za-z_0-9]*`|'(?:''|[^'])*'|[A-Za-z_][A-Za-z_0-9]*|[0-9]+|<=|>=|<>|[(),=<>]")
    tokens = []
    cursor = 0
    for match in token_pattern.finditer(expression):
        if match.start() != cursor:
            raise ValueError('unsupported check expression')
        cursor = match.end()
        token = match.group()
        if not token.isspace():
            tokens.append(token if token.startswith("'") else token.strip('`').lower())
    if cursor != len(expression) or not tokens:
        raise ValueError('unsupported check expression')
    position = 0

    def peek():
        return tokens[position] if position < len(tokens) else None

    def consume(expected=None):
        nonlocal position
        token = peek()
        if token is None or (expected is not None and token != expected):
            raise ValueError('unsupported check expression')
        position += 1
        return token

    def primary():
        token = consume()
        if token == '(':
            value = conjunction()
            consume(')')
            return value
        # MySQL metadata can render ASCII literals with _ascii/_utf8mb4.
        if token.startswith('_') and peek() is not None and peek().startswith("'"):
            token = consume()
        if token.startswith("'") or token.isdecimal():
            return ('literal', token)
        if not re.fullmatch('[a-z_][a-z_0-9]*', token):
            raise ValueError('unsupported check expression')
        if peek() == '(':
            consume('(')
            argument = conjunction()
            consume(')')
            return ('call', token, argument)
        return ('column', token)

    def predicate():
        left = primary()
        operator = peek()
        if operator in ('=', '>', '<', '>=', '<=', '<>'):
            consume()
            return (operator, left, primary())
        if operator == 'between':
            consume()
            lower = primary()
            consume('and')
            return ('between', left, lower, primary())
        if operator == 'in':
            consume()
            consume('(')
            values = [primary()]
            while peek() == ',':
                consume(',')
                values.append(primary())
            consume(')')
            return ('in', left, tuple(values))
        return left

    def conjunction():
        values = [predicate()]
        while peek() == 'and':
            consume('and')
            value = predicate()
            values.extend(value[1:] if value[0] == 'and' else [value])
        if values[0][0] == 'and':
            values = [*values[0][1:], *values[1:]]
        return values[0] if len(values) == 1 else ('and', *values)

    result = conjunction()
    if position != len(tokens):
        raise ValueError('unsupported check expression')
    return result


def has_constraints(snapshot, table, expected):
    _, _, constraints = parsed_shape(snapshot, table)
    try:
        return all(name in constraints and constraints[name]['type'] == 'CHECK'
                   and constraints[name]['enforced'] == 'YES'
                   and check_expression(constraints[name]['clause']) == check_expression(clause)
                   for name, clause in expected.items())
    except ValueError:
        return False


def expected_commit_matches(snapshot, expected_commit):
    if expected_commit is None:
        return None
    if not isinstance(expected_commit, str) or not re.fullmatch('[a-f0-9]{12,40}', expected_commit):
        return False
    version = snapshot.get('version', {})
    data = version.get('data', {}) if isinstance(version, dict) else {}
    runtime_commit = data.get('commit') if isinstance(data, dict) else None
    return (
        isinstance(runtime_commit, str)
        and re.fullmatch('[a-f0-9]{12,40}', runtime_commit) is not None
        and commit_prefix_matches(runtime_commit, expected_commit)
    )


def evaluate(snapshot, expected_commit=None):
    flags = snapshot.get('flags', {})
    counts = snapshot.get('counts', {})
    release = snapshot.get('releaseMetadata', {})
    candidate_matches = expected_commit_matches(snapshot, expected_commit)
    expected_valid = (expected_commit is None or
                      isinstance(expected_commit, str) and
                      re.fullmatch('[a-f0-9]{12,40}', expected_commit) is not None)
    try:
        parsed_flags = {name: bool_flag(flags.get(name, ''), BOOLEAN_DEFAULTS.get(name, False))
                        for name in BOOLEAN_FLAG_NAMES}
        state = flags.get('WECHAT_SUBSCRIBE_MINIPROGRAM_STATE', '') or 'formal'
        if state not in ('developer', 'trial', 'formal'):
            raise ValueError('invalid mini program state')
    except ValueError:
        return {
            'readOnly': True, 'migration054Ready': False,
            'migration055Ready': False, 'migration056Ready': False,
            'releaseIdentityReady': False, 'flagConflicts': ['invalid_configuration_flag'],
            'defaultOffCandidateReady': False, 'broadcastActivationReady': False,
            'withdrawalActivationPreflightReady': False,
            'expectedCommitMatches': candidate_matches,
        }
    earnings, earning_indexes, _ = parsed_shape(snapshot, 'delivery_rider_earning_records')
    migration_054 = (
        'settle_at' in earnings and earnings['settle_at'][:3] == ['datetime', 'NO', '3']
        and 'settled_at' in earnings and earnings['settled_at'][:3] == ['datetime', 'YES', '3']
        and earning_indexes.get('idx_delivery_rider_earning_due')
        == {'columns': 'settlement_status,settle_at,id', 'nonUnique': '1'}
    )
    migration_055 = (
        all(counts.get(name) is not None for name in MIGRATION_055_TABLES)
        and all(has_typed_columns(snapshot, table, columns)
                and has_indexes(snapshot, table, {'PRIMARY': ('id', True)})
                for table, columns in COLUMNS_055.items())
        and has_columns(snapshot, 'delivery_rider_availability',
                        ['rider_user_id', 'state', 'scope', 'eligibility_revision', 'row_version'])
        and has_indexes(snapshot, 'delivery_rider_availability',
                        {'uq_delivery_rider_availability_rider': ('rider_user_id', True),
                         'idx_delivery_rider_availability_state': ('state,scope,rider_user_id', False)})
        and has_constraints(snapshot, 'delivery_rider_availability',
                            CHECKS_055['delivery_rider_availability'])
        and has_columns(snapshot, 'delivery_rider_broadcast_config',
                        ['config_scope', 'max_in_flight_per_rider', 'row_version'])
        and has_indexes(snapshot, 'delivery_rider_broadcast_config',
                        {'uq_delivery_rider_broadcast_config_scope': ('config_scope', True)})
        and has_constraints(snapshot, 'delivery_rider_broadcast_config',
                            CHECKS_055['delivery_rider_broadcast_config'])
        and counts.get('delivery_rider_broadcast_config') == 1
    )
    migration_056 = (
        all(counts.get(name) is not None for name in MIGRATION_056_TABLES)
        and all(has_typed_columns(snapshot, table, columns)
                and has_indexes(snapshot, table, {'PRIMARY': ('id', True)})
                for table, columns in COLUMNS_056.items())
        and has_columns(snapshot, 'delivery_rider_withdrawal_allocations',
                        ['withdraw_public_id', 'earning_public_id', 'rider_user_id',
                         'wallet_public_id', 'amount_cents', 'status', 'source_evidence_hash'])
        and has_indexes(snapshot, 'delivery_rider_withdrawal_allocations',
                        {'uq_delivery_rider_withdrawal_allocation_public_id': ('allocation_public_id', True),
                         'uq_delivery_rider_withdrawal_allocation_pair':
                         ('withdraw_public_id,earning_public_id', True)})
        and has_constraints(snapshot, 'delivery_rider_withdrawal_allocations',
                            CHECKS_056['delivery_rider_withdrawal_allocations'])
        and has_columns(snapshot, 'delivery_rider_withdrawal_claims',
                        ['withdraw_public_id', 'operator_user_id', 'claim_key', 'status',
                         'idempotency_key'])
        and has_indexes(snapshot, 'delivery_rider_withdrawal_claims',
                        {'uq_delivery_rider_withdrawal_claim_public_id': ('claim_public_id', True),
                         'uq_delivery_rider_withdrawal_claim_withdraw': ('withdraw_public_id', True),
                         'uq_delivery_rider_withdrawal_claim_key': ('claim_key', True),
                         'uq_delivery_rider_withdrawal_claim_idempotency': ('idempotency_key', True)})
        and has_constraints(snapshot, 'delivery_rider_withdrawal_claims',
                            CHECKS_056['delivery_rider_withdrawal_claims'])
        and has_columns(snapshot, 'delivery_rider_withdrawal_events',
                        ['withdraw_public_id', 'event_type', 'operator_user_id',
                         'request_hash', 'idempotency_key'])
        and has_indexes(snapshot, 'delivery_rider_withdrawal_events',
                        {'uq_delivery_rider_withdrawal_event_public_id': ('event_public_id', True),
                         'uq_delivery_rider_withdrawal_event_idempotency': ('idempotency_key', True)})
        and has_constraints(snapshot, 'delivery_rider_withdrawal_events',
                            CHECKS_056['delivery_rider_withdrawal_events'])
        and has_columns(snapshot, 'delivery_rider_withdrawal_payment_references',
                        ['payment_reference', 'withdraw_public_id', 'created_at'])
        and has_indexes(snapshot, 'delivery_rider_withdrawal_payment_references',
                        {'uq_delivery_rider_withdrawal_payment_reference': ('payment_reference', True),
                         'uq_delivery_rider_withdrawal_payment_withdraw': ('withdraw_public_id', True)})
        and has_constraints(snapshot, 'delivery_rider_withdrawal_payment_references',
                            CHECKS_056['delivery_rider_withdrawal_payment_references'])
    )
    delivery = parsed_flags['ENABLE_DELIVERY']
    broadcast = parsed_flags['ENABLE_DELIVERY_RIDER_BROADCAST']
    withdrawal = parsed_flags['ENABLE_DELIVERY_RIDER_WITHDRAWAL']
    payout_queue = parsed_flags['ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE']
    daily = parsed_flags['ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT']
    conflicts = []
    if not expected_valid:
        conflicts.append('invalid_expected_commit')
    elif candidate_matches is False:
        conflicts.append('expected_candidate_commit_mismatch')
    if withdrawal and payout_queue:
        conflicts.append('withdrawal_and_payout_queue_both_enabled')
    if withdrawal and not delivery:
        conflicts.append('withdrawal_enabled_without_delivery')
    if broadcast and not migration_055:
        conflicts.append('broadcast_enabled_without_migration_055')
    if withdrawal and not migration_056:
        conflicts.append('withdrawal_enabled_without_migration_056')
    if daily and not migration_054:
        conflicts.append('daily_settlement_enabled_without_migration_054')
    if delivery and not migration_054:
        conflicts.append('delivery_enabled_without_required_migration_054')
    identity_ready = (
        release.get('currentIsSafeReleaseSymlink') is True
        and release.get('identityMatchesRuntime') is True
        and snapshot.get('readyHTTP') == 200
        and candidate_matches is not False
    )
    return {
        'readOnly': True,
        'expectedCommitMatches': candidate_matches,
        'migration054Ready': migration_054,
        'migration055Ready': migration_055,
        'migration056Ready': migration_056,
        'releaseIdentityReady': identity_ready,
        'flagConflicts': conflicts,
        'defaultOffCandidateReady': identity_ready and migration_054 and not conflicts
        and not broadcast and not withdrawal and not payout_queue
        and not parsed_flags['ENABLE_NOTIFICATION_SENDING'],
        'broadcastActivationReady': identity_ready and delivery and migration_054 and migration_055
        and not conflicts,
        # Full source consistency is rechecked by CheckWithdrawalReadiness at
        # server startup; this flag only proves schema and configuration gates.
        'withdrawalActivationPreflightReady': identity_ready and delivery and migration_054 and migration_056
        and not conflicts and not payout_queue,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', help='evaluate a sanitised JSON snapshot instead of the live host')
    parser.add_argument('--expected-commit', help='frozen candidate commit, 12..40 lowercase hexadecimal characters')
    parser.add_argument('--require-ready', choices=('candidate', 'broadcast', 'withdrawal'),
                        help='require the selected preflight gate; needs --expected-commit and never activates it')
    args = parser.parse_args()
    if args.require_ready and args.expected_commit is None:
        parser.error('--require-ready requires --expected-commit')
    if args.expected_commit is not None and not re.fullmatch('[a-f0-9]{12,40}', args.expected_commit):
        parser.error('--expected-commit must contain 12..40 lowercase hexadecimal characters')
    snapshot = json.loads(Path(args.fixture).read_text(encoding='utf-8')) if args.fixture else collect_live_snapshot()
    result = dict(snapshot)
    result['readOnly'] = True
    result['preflight'] = evaluate(snapshot, args.expected_commit)
    if args.require_ready:
        gate = {
            'candidate': 'defaultOffCandidateReady',
            'broadcast': 'broadcastActivationReady',
            'withdrawal': 'withdrawalActivationPreflightReady',
        }[args.require_ready]
        result['requiredGate'] = {'name': args.require_ready,
                                  'passed': result['preflight'][gate] is True}
    print(json.dumps(result, ensure_ascii=False))
    if result['preflight']['expectedCommitMatches'] is False:
        return 1
    if args.require_ready and not result['requiredGate']['passed']:
        return 1
    return 0


if __name__ == '__main__':
    try:
        raise SystemExit(main())
    except Exception as error:
        print(json.dumps({'readOnly': True, 'failed': True, 'errorType': type(error).__name__,
                          'reason': str(error) if isinstance(error, RuntimeError) else 'details withheld'}))
        raise SystemExit(1)
