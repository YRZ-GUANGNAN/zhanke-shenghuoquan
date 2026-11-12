#!/usr/bin/env python3
"""Pure fixture tests for deploy/native/inspect-rider-release.py."""
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / 'deploy' / 'native' / 'inspect-rider-release.py'
SPEC = importlib.util.spec_from_file_location('inspect_rider_release', MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def column(name, data_type='varchar', nullable='NO', precision='', length='', column_type='varchar(16)'):
    return '\t'.join([name, data_type, nullable, precision, length, column_type])


def shape(columns, indexes=(), constraints=()):
    return {
        'columns': columns,
        'indexes': ['\t'.join(value) for value in indexes],
        'constraints': ['\t'.join([name, 'CHECK', 'YES', clause]) for name, clause in constraints],
    }


def migration_shapes(filename):
    """Build metadata fixtures from the real DDL, independently of inspector constants."""
    sql = (ROOT / 'backend' / 'migrations' / filename).read_text(encoding='utf-8')
    result = {}
    for table, body in re.findall(r'CREATE TABLE(?: IF NOT EXISTS)? (\w+) \((.*?)\) ENGINE=', sql, re.S):
        columns = []
        for match in re.finditer(r'^  (\w+) (CHAR|VARCHAR|BIGINT|INT|DATETIME)(?:\((\d+)\))?( UNSIGNED)?(.*)$', body, re.M):
            name, kind, size, unsigned, rest = match.groups()
            kind = kind.lower()
            columns.append(column(name, kind, 'NO' if 'NOT NULL' in rest else 'YES',
                                  size or '' if kind == 'datetime' else '',
                                  size or '' if kind in ('char', 'varchar') else '',
                                  kind + (f'({size})' if size else '') + (' unsigned' if unsigned else '')))
        indexes = []
        primary = re.search(r'PRIMARY KEY \(([^)]+)\)', body)
        if primary:
            indexes.append(('PRIMARY', '0', primary.group(1).replace(' ', '')))
        for unique, name, keys in re.findall(r'(UNIQUE )?INDEX (\w+) \(([^)]+)\)', body):
            indexes.append((name, '0' if unique else '1', keys.replace(' ', '')))
        constraints = []
        for match in re.finditer(r'CONSTRAINT (\w+) CHECK \(', body):
            start = match.end()
            position, depth = start, 1
            while depth:
                if body[position] == '(':
                    depth += 1
                elif body[position] == ')':
                    depth -= 1
                position += 1
            clause = ' '.join(body[start:position - 1].split())
            constraints.append((match.group(1), clause))
        result[table] = shape(columns, indexes, constraints)
    return result


def ready_snapshot():
    counts = {name: 0 for name in MODULE.MONEY_TABLES + MODULE.MIGRATION_055_TABLES
              + MODULE.MIGRATION_056_TABLES + MODULE.NOTIFICATION_TABLES}
    counts['delivery_rider_broadcast_config'] = 1
    return {
        'version': {'data': {'commit': '203af3154df3526579410f5506fea89c5cec1152'}},
        'readyHTTP': 200,
        'releaseMetadata': {
            'currentIsSafeReleaseSymlink': True,
            'identityMatchesRuntime': True,
            'targetName': 'candidate',
            'identitySource': 'release-receipt.json',
        },
        'flags': {
            'ENABLE_DELIVERY': 'true',
            'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT': 'true',
            'ENABLE_DELIVERY_RIDER_BROADCAST': 'false',
            'ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE': 'false',
            'ENABLE_DELIVERY_RIDER_WITHDRAWAL': 'false',
            'WECHAT_SUBSCRIBE_MINIPROGRAM_STATE': 'formal',
        },
        'counts': counts,
        'shapes': {
            'delivery_rider_earning_records': shape([
                column('settlement_status'),
                column('settle_at', 'datetime', 'NO', '3', '', 'datetime(3)'),
                column('settled_at', 'datetime', 'YES', '3', '', 'datetime(3)'),
                column('id', 'bigint', 'NO', '', '', 'bigint unsigned'),
            ], [('idx_delivery_rider_earning_due', '1', 'settlement_status,settle_at,id')]),
            **migration_shapes('055_delivery_rider_availability.sql'),
            **migration_shapes('056_delivery_rider_withdrawal_allocations.sql'),
        },
    }


class RiderReleasePreflightTest(unittest.TestCase):
    def test_expected_commit_binds_runtime_candidate(self):
        for expected in ('203af3154df3', '203af3154df3526579410f5506fea89c5cec1152'):
            with self.subTest(expected=expected):
                result = MODULE.evaluate(ready_snapshot(), expected_commit=expected)
                self.assertTrue(result['expectedCommitMatches'])
                self.assertTrue(result['defaultOffCandidateReady'])

    def test_short_runtime_commit_matches_full_expected_commit(self):
        snapshot = ready_snapshot()
        snapshot['version'] = {'data': {'commit': '203af3154df3'}}
        result = MODULE.evaluate(
            snapshot,
            expected_commit='203af3154df3526579410f5506fea89c5cec1152',
        )
        self.assertTrue(result['expectedCommitMatches'])
        self.assertTrue(result['defaultOffCandidateReady'])

    def test_commit_prefix_matching_rejects_short_or_ambiguous_values(self):
        for left, right in (
            ('203af3154df3', '203af3154df3'),
            ('203af3154df3526579410f5506fea89c5cec1152', '203af3154df3'),
        ):
            with self.subTest(left=left, right=right):
                self.assertTrue(MODULE.commit_prefix_matches(left, right))
        for left, right in (
            ('203af31', '203af3154df3'),
            ('203af3154df3', '203af31'),
            ('203af3154df3', 'a' * 12),
            (None, '203af3154df3'),
        ):
            with self.subTest(left=left, right=right):
                self.assertFalse(MODULE.commit_prefix_matches(left, right))

    def test_wrong_missing_or_ambiguous_runtime_commit_blocks_all_gates(self):
        for version in ({}, {'data': {}}, {'data': {'commit': '203af31'}},
                        {'data': {'commit': 'a' * 40}}):
            with self.subTest(version=version):
                snapshot = ready_snapshot()
                snapshot['version'] = version
                result = MODULE.evaluate(snapshot, expected_commit='203af3154df3526579410f5506fea89c5cec1152')
                self.assertFalse(result['expectedCommitMatches'])
                for gate in ('releaseIdentityReady', 'defaultOffCandidateReady',
                             'broadcastActivationReady', 'withdrawalActivationPreflightReady'):
                    self.assertFalse(result[gate])
                self.assertIn('expected_candidate_commit_mismatch', result['flagConflicts'])

    def test_invalid_expected_commit_fails_closed(self):
        for expected in ('', '203af31', '203AF3154DF3', ' 203af3154df3', 'g' * 40,
                         'a' * 41, 203, True):
            with self.subTest(expected=expected):
                result = MODULE.evaluate(ready_snapshot(), expected_commit=expected)
                self.assertFalse(result['releaseIdentityReady'])
                self.assertFalse(result['defaultOffCandidateReady'])
                self.assertIn('invalid_expected_commit', result['flagConflicts'])

    def run_cli(self, snapshot, *arguments):
        with tempfile.TemporaryDirectory(prefix='rider-release-fixture-') as temporary:
            fixture = Path(temporary) / 'snapshot.json'
            fixture.write_text(json.dumps(snapshot), encoding='utf-8')
            return subprocess.run([sys.executable, '-B', str(MODULE_PATH), '--fixture', str(fixture),
                                   *arguments], capture_output=True, text=True, timeout=15)

    def test_cli_inventory_remains_read_only_without_acceptance_claim(self):
        result = self.run_cli(ready_snapshot())
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        self.assertTrue(output['readOnly'])
        self.assertIsNone(output['preflight']['expectedCommitMatches'])
        self.assertNotIn('requiredGate', output)

    def test_cli_required_gate_needs_expected_candidate(self):
        result = self.run_cli(ready_snapshot(), '--require-ready', 'candidate')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('--expected-commit', result.stderr)

    def test_cli_unreadable_or_malformed_snapshot_is_redacted_failure(self):
        with tempfile.TemporaryDirectory(prefix='rider-release-invalid-fixture-') as temporary:
            fixture = Path(temporary) / 'snapshot.json'
            for invalid_json in (None, '{invalid-private-fixture-marker'):
                with self.subTest(invalid_json=invalid_json):
                    if invalid_json is not None:
                        fixture.write_text(invalid_json, encoding='utf-8')
                    result = subprocess.run(
                        [sys.executable, '-B', str(MODULE_PATH), '--fixture', str(fixture),
                         '--expected-commit', '203af3154df3', '--require-ready', 'candidate'],
                        capture_output=True, text=True, timeout=15)
                    self.assertEqual(result.returncode, 1, result.stderr)
                    output = json.loads(result.stdout)
                    self.assertTrue(output['failed'])
                    self.assertTrue(output['readOnly'])
                    self.assertEqual(output['reason'], 'details withheld')
                    self.assertNotIn('private-fixture-marker', result.stdout + result.stderr)
                    self.assertNotIn(str(fixture), result.stdout + result.stderr)

    def test_cli_required_gate_binds_candidate_and_exit_status(self):
        for gate in ('candidate', 'broadcast', 'withdrawal'):
            with self.subTest(gate=gate):
                valid = self.run_cli(ready_snapshot(), '--require-ready', gate,
                                     '--expected-commit', '203af3154df3')
                self.assertEqual(valid.returncode, 0, valid.stderr)
                output = json.loads(valid.stdout)
                self.assertTrue(output['requiredGate']['passed'])
                self.assertEqual(output['requiredGate']['name'], gate)
                invalid = self.run_cli(ready_snapshot(), '--require-ready', gate,
                                       '--expected-commit', 'a' * 12)
                self.assertEqual(invalid.returncode, 1, invalid.stderr)
                self.assertFalse(json.loads(invalid.stdout)['requiredGate']['passed'])

    def test_cli_bound_inspection_and_schema_failure_exit_nonzero(self):
        mismatch = self.run_cli(ready_snapshot(), '--expected-commit', 'a' * 12)
        self.assertEqual(mismatch.returncode, 1)
        snapshot = ready_snapshot()
        snapshot['counts']['delivery_rider_availability'] = None
        failed = self.run_cli(snapshot, '--expected-commit', '203af3154df3', '--require-ready', 'broadcast')
        self.assertEqual(failed.returncode, 1)
        self.assertFalse(json.loads(failed.stdout)['requiredGate']['passed'])

    def test_default_off_and_both_activation_schemas_are_ready(self):
        result = MODULE.evaluate(ready_snapshot())
        self.assertTrue(result['defaultOffCandidateReady'])
        self.assertTrue(result['broadcastActivationReady'])
        self.assertTrue(result['withdrawalActivationPreflightReady'])
        self.assertEqual(result['flagConflicts'], [])

    def test_default_off_candidate_rejects_enabled_payout_queue(self):
        snapshot = ready_snapshot()
        snapshot['flags']['ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE'] = 'true'
        result = MODULE.evaluate(snapshot, expected_commit='203af3154df3')
        self.assertFalse(result['defaultOffCandidateReady'])
        self.assertTrue(result['broadcastActivationReady'])
        self.assertFalse(result['withdrawalActivationPreflightReady'])
        self.assertEqual(result['flagConflicts'], [])

    def test_default_off_candidate_rejects_enabled_notification_sending(self):
        snapshot = ready_snapshot()
        snapshot['flags']['ENABLE_NOTIFICATION_SENDING'] = 'true'
        result = MODULE.evaluate(snapshot, expected_commit='203af3154df3')
        self.assertFalse(result['defaultOffCandidateReady'])
        # The explicit activation preflights are not default-off deployments.
        self.assertTrue(result['broadcastActivationReady'])
        self.assertTrue(result['withdrawalActivationPreflightReady'])
        self.assertEqual(result['flagConflicts'], [])

    def test_cli_candidate_gate_rejects_non_default_off_queue_or_sending(self):
        for flag in ('ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE', 'ENABLE_NOTIFICATION_SENDING'):
            with self.subTest(flag=flag):
                snapshot = ready_snapshot()
                snapshot['flags'][flag] = 'true'
                result = self.run_cli(snapshot, '--expected-commit', '203af3154df3',
                                      '--require-ready', 'candidate')
                self.assertEqual(result.returncode, 1, result.stderr)
                output = json.loads(result.stdout)
                self.assertFalse(output['requiredGate']['passed'])
                self.assertTrue(output['preflight']['expectedCommitMatches'])

    def test_missing_schema_fails_closed_when_flag_is_enabled(self):
        snapshot = ready_snapshot()
        snapshot['counts']['delivery_rider_availability'] = None
        snapshot['flags']['ENABLE_DELIVERY_RIDER_BROADCAST'] = 'true'
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['migration055Ready'])
        self.assertFalse(result['defaultOffCandidateReady'])
        self.assertFalse(result['broadcastActivationReady'])
        self.assertIn('broadcast_enabled_without_migration_055', result['flagConflicts'])

    def test_withdrawal_and_queue_conflict_is_rejected(self):
        snapshot = ready_snapshot()
        snapshot['flags']['ENABLE_DELIVERY_RIDER_WITHDRAWAL'] = 'true'
        snapshot['flags']['ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE'] = 'true'
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['withdrawalActivationPreflightReady'])
        self.assertIn('withdrawal_and_payout_queue_both_enabled', result['flagConflicts'])

    def test_untraceable_release_blocks_every_gate(self):
        snapshot = ready_snapshot()
        snapshot['releaseMetadata']['identityMatchesRuntime'] = False
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['releaseIdentityReady'])
        self.assertFalse(result['defaultOffCandidateReady'])
        self.assertFalse(result['broadcastActivationReady'])
        self.assertFalse(result['withdrawalActivationPreflightReady'])

    def test_numeric_true_flags_are_parsed_like_backend(self):
        snapshot = ready_snapshot()
        snapshot['counts']['delivery_rider_availability'] = None
        snapshot['flags']['ENABLE_DELIVERY_RIDER_BROADCAST'] = '1'
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['migration055Ready'])
        self.assertIn('broadcast_enabled_without_migration_055', result['flagConflicts'])

    def test_non_unique_withdrawal_index_fails_closed(self):
        # Change only NON_UNIQUE of each required unique index. Do not remove a
        # different index accidentally, which would let a broken checker pass.
        for table in MODULE.MIGRATION_056_TABLES:
            for index, raw in enumerate(ready_snapshot()['shapes'][table]['indexes']):
                name, non_unique, keys = raw.split('\t')
                if non_unique != '0':
                    continue
                with self.subTest(table=table, index=name):
                    snapshot = ready_snapshot()
                    snapshot['shapes'][table]['indexes'][index] = '\t'.join([name, '1', keys])
                    result = MODULE.evaluate(snapshot)
                    self.assertFalse(result['migration056Ready'])
                    self.assertFalse(result['withdrawalActivationPreflightReady'])

    def test_all_wechat_target_enums_are_distinct_from_booleans(self):
        for state in ('developer', 'trial', 'formal', ''):
            with self.subTest(state=state):
                snapshot = ready_snapshot()
                snapshot['flags']['WECHAT_SUBSCRIBE_MINIPROGRAM_STATE'] = state
                result = MODULE.evaluate(snapshot)
                self.assertTrue(result['defaultOffCandidateReady'])
                self.assertTrue(result['withdrawalActivationPreflightReady'])

    def test_unknown_target_fails_closed(self):
        for value in ('production', 'true', 'FORMAL', ' trial '):
            with self.subTest(value=value):
                snapshot = ready_snapshot()
                snapshot['flags']['WECHAT_SUBSCRIBE_MINIPROGRAM_STATE'] = value
                self.assertFalse(MODULE.evaluate(snapshot)['defaultOffCandidateReady'])

    def test_boolean_loader_matches_backend_exactly(self):
        for value in ('true', '1'):
            self.assertTrue(MODULE.bool_flag(value))
        for value in ('false', '0'):
            self.assertFalse(MODULE.bool_flag(value, True))
        self.assertFalse(MODULE.bool_flag(''))
        self.assertTrue(MODULE.bool_flag('', True))
        for value in ('TRUE', 'False', ' true ', '1 ', 'yes', 'UNSET', None, 1, True):
            with self.subTest(value=value), self.assertRaises(ValueError):
                MODULE.bool_flag(value)

    def test_invalid_boolean_blocks_every_gate(self):
        snapshot = ready_snapshot()
        snapshot['flags']['ENABLE_NOTIFICATION_SENDING'] = 'yes'
        result = MODULE.evaluate(snapshot)
        for gate in ('defaultOffCandidateReady', 'broadcastActivationReady', 'withdrawalActivationPreflightReady'):
            self.assertFalse(result[gate])
        self.assertEqual(result['flagConflicts'], ['invalid_configuration_flag'])

    def test_delivery_defaults_true_even_when_env_is_missing(self):
        for value in (None, ''):
            snapshot = ready_snapshot()
            snapshot['flags'].pop('ENABLE_DELIVERY')
            if value is not None:
                snapshot['flags']['ENABLE_DELIVERY'] = value
            snapshot['flags']['ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT'] = 'false'
            snapshot['shapes']['delivery_rider_earning_records']['columns'] = []
            result = MODULE.evaluate(snapshot)
            self.assertIn('delivery_enabled_without_required_migration_054', result['flagConflicts'])

    def test_disabled_delivery_cannot_claim_activatable_features(self):
        snapshot = ready_snapshot()
        snapshot['flags']['ENABLE_DELIVERY'] = '0'
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['broadcastActivationReady'])
        self.assertFalse(result['withdrawalActivationPreflightReady'])
        snapshot['flags']['ENABLE_DELIVERY_RIDER_WITHDRAWAL'] = '1'
        self.assertIn('withdrawal_enabled_without_delivery', MODULE.evaluate(snapshot)['flagConflicts'])

    def test_missing_payment_reference_schema_is_not_withdrawal_ready(self):
        snapshot = ready_snapshot()
        snapshot['counts']['delivery_rider_withdrawal_payment_references'] = None
        result = MODULE.evaluate(snapshot)
        self.assertFalse(result['migration056Ready'])
        self.assertFalse(result['withdrawalActivationPreflightReady'])

    def test_check_constraints_must_be_enforced_with_real_predicates(self):
        for table in MODULE.MIGRATION_055_TABLES + MODULE.MIGRATION_056_TABLES:
            for index, raw in enumerate(ready_snapshot()['shapes'][table]['constraints']):
                name, kind, enforced, clause = raw.split('\t', 3)
                for modified in ((name, kind, 'NO', clause), (name, kind, enforced, '1'),
                                 (name, 'UNIQUE', enforced, clause), (name, kind)):
                    with self.subTest(table=table, constraint=name, variant=modified):
                        snapshot = ready_snapshot()
                        snapshot['shapes'][table]['constraints'][index] = '\t'.join(modified)
                        gate = 'migration055Ready' if table in MODULE.MIGRATION_055_TABLES else 'migration056Ready'
                        self.assertFalse(MODULE.evaluate(snapshot)[gate])

    def test_required_column_types_and_non_null_are_checked(self):
        for table, field, data_type in (
            ('delivery_rider_withdrawal_allocations', 'amount_cents', 'varchar(16)'),
            ('delivery_rider_withdrawal_allocations', 'amount_cents', 'bigint'),
            ('delivery_rider_withdrawal_claims', 'claim_key', 'char(8)'),
            ('delivery_rider_withdrawal_events', 'request_hash', 'char(26)'),
            ('delivery_rider_withdrawal_payment_references', 'payment_reference', 'varchar(16)'),
            ('delivery_rider_availability', 'eligibility_revision', 'varchar(16)'),
        ):
            for nullable in ('YES', 'NO'):
                with self.subTest(table=table, field=field, nullable=nullable):
                    snapshot = ready_snapshot()
                    rows = snapshot['shapes'][table]['columns']
                    for index, raw in enumerate(rows):
                        cells = raw.split('\t')
                        if cells[0] == field:
                            cells[2] = nullable
                            # Test nullability independently from type changes.
                            if nullable == 'NO':
                                cells[5] = data_type
                            rows[index] = '\t'.join(cells)
                    gate = 'migration055Ready' if table in MODULE.MIGRATION_055_TABLES else 'migration056Ready'
                    self.assertFalse(MODULE.evaluate(snapshot)[gate])

    def test_mysql_parentheses_charset_and_identifier_format_is_supported(self):
        pairs = [
            ("((`state` in (_ascii'off_duty',_ascii'on_duty')))", "state IN ('off_duty', 'on_duty')"),
            ("((`currency` = _utf8mb4'CNY'))", "currency = 'CNY'"),
            ("((`state` in (_utf8mb4\\'off_duty\\',_utf8mb4\\'on_duty')))",
             "state IN ('off_duty', 'on_duty')"),
            ("((`state` in (_utf8mb4\\\\'off_duty\\\\',_utf8mb4\\\\'on_duty')))",
             "state IN ('off_duty', 'on_duty')"),
            ('((`amount_cents` > 0) and (`amount_cents` <= 9007199254740991))',
             'amount_cents > 0 AND amount_cents <= 9007199254740991'),
            ('((char_length(trim(`source_policy_version`)) > 0) and (char_length(`source_evidence_hash`) = 64))',
             'CHAR_LENGTH(TRIM(source_policy_version)) > 0 AND CHAR_LENGTH(source_evidence_hash) = 64'),
            ('((`operator_user_id` > 0) and ((`expected_row_version` > 0) and (char_length(`request_hash`) = 64)))',
             'operator_user_id > 0 AND expected_row_version > 0 AND CHAR_LENGTH(request_hash) = 64'),
            ('(`max_in_flight_per_rider` between 1 and 100)', 'max_in_flight_per_rider BETWEEN 1 AND 100'),
        ]
        for metadata, source in pairs:
            with self.subTest(metadata=metadata):
                self.assertEqual(MODULE.check_expression(metadata), MODULE.check_expression(source))

    def test_check_parser_does_not_strip_meaningful_parentheses_or_accept_extra_sql(self):
        approved = MODULE.check_expression('amount_cents > 0 AND amount_cents <= 9007199254740991')
        for malformed in ('amount_cents > (0 AND amount_cents) <= 9007199254740991',
                          'amount_cents > 0 AND amount_cents <= 9007199254740991 OR 1',
                          'amount_cents > 0 /* wrong */ AND amount_cents <= 9007199254740991',
                          ''):
            with self.subTest(malformed=malformed):
                try:
                    actual = MODULE.check_expression(malformed)
                except ValueError:
                    continue
                self.assertNotEqual(actual, approved)

    def test_metadata_collector_reads_enforcement_and_expression(self):
        queries = []

        def sql(query):
            queries.append(query)
            return ''

        MODULE.table_shape(sql, 'delivery_rider_withdrawal_allocations')
        constraints = queries[-1]
        self.assertIn('t.ENFORCED', constraints)
        self.assertIn('c.CHECK_CLAUSE', constraints)
        self.assertIn('information_schema.CHECK_CONSTRAINTS', constraints)
        self.assertIn('c.CONSTRAINT_SCHEMA=t.CONSTRAINT_SCHEMA', constraints)
        self.assertTrue(all(query.startswith('SELECT ') for query in queries))


if __name__ == '__main__':
    unittest.main()
