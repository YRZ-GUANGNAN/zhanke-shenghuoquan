#!/usr/bin/env python3
"""Offline fixtures; these tests never authenticate a production observation."""
import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


MODULE_PATH = ROOT / 'deploy/native/rehearse-rider-release.py'
MODULE = load_module('rider_rehearsal', MODULE_PATH)
FIXTURES = load_module('rider_preflight_fixtures', ROOT / 'scripts/test-rider-release-preflight.py')
CANDIDATE = '203af3154df3526579410f5506fea89c5cec1152'
PREVIOUS = '972d3f343745'


def digest(value):
    return hashlib.sha256(value).hexdigest()


class RiderReleaseRehearsalTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix='rider-release-rehearsal-')
        self.addCleanup(self.temporary.cleanup)
        self.stage = Path(self.temporary.name)
        assets = {'campus-api': b'\x7fELFfixture-not-an-executable'}
        for name in MODULE.MIGRATIONS:
            assets[name] = (ROOT / 'backend/migrations' / name).read_bytes()
        for name, contents in assets.items():
            (self.stage / name).write_bytes(contents)
        self.manifest = {
            'schemaVersion': 1, 'candidateCommit': CANDIDATE, 'previousCommit': PREVIOUS,
            'migrations': list(MODULE.MIGRATIONS),
            'assets': {name: digest(contents) for name, contents in assets.items()},
            'originalTables': sorted(MODULE.INSPECTOR.MONEY_TABLES),
        }
        dump = b'-- fixture only\nCREATE TABLE fixture_table (id BIGINT PRIMARY KEY);\n-- Dump completed on fixture\n'
        backup = gzip.compress(dump)
        (self.stage / 'database.sql.gz').write_bytes(backup)
        preflight = FIXTURES.ready_snapshot()
        preflight['flags'].update({'ENABLE_NOTIFICATION_SENDING': 'false',
                                   'ENABLE_DELIVERY_PAYMENT': 'true'})
        snapshots = {name: {'rows': 0, 'sha256': digest(b'')}
                     for name in MODULE.INSPECTOR.MONEY_TABLES}
        self.evidence = {
            'schemaVersion': 1, 'mode': 'fixture', 'candidateCommit': CANDIDATE,
            'previousCommit': PREVIOUS,
            'stoppedWriters': {'scope': 'fixture', 'apiProcesses': 0, 'workerProcesses': 0,
                               'applicationConnections': 0, 'uninspectedWriters': 0},
            'flagsBefore': copy.deepcopy(preflight['flags']),
            'flagsAfter': copy.deepcopy(preflight['flags']),
            'backup': {'file': 'database.sql.gz', 'sha256': digest(backup), 'completed': True},
            'beforeMigration': copy.deepcopy(snapshots),
            'restoredOriginalTables': copy.deepcopy(snapshots),
            'afterMigrationOriginalTables': copy.deepcopy(snapshots),
            'afterPreflight': preflight,
        }

    def bound_files(self):
        manifest = json.dumps(self.manifest).encode()
        (self.stage / 'manifest.json').write_bytes(manifest)
        self.evidence['manifestSha256'] = digest(manifest)
        evidence = json.dumps(self.evidence).encode()
        evidence_path = self.stage / 'evidence.json'
        evidence_path.write_bytes(evidence)
        return evidence_path, digest(manifest), digest(evidence)

    def verify(self, **overrides):
        evidence_path, manifest_hash, evidence_hash = self.bound_files()
        arguments = {
            'stage': self.stage, 'evidence_path': evidence_path,
            'expected_manifest_hash': manifest_hash, 'expected_evidence_hash': evidence_hash,
            'expected_previous_commit': PREVIOUS, 'expected_candidate_commit': CANDIDATE,
        }
        arguments.update(overrides)
        return MODULE.verify_bundle(**arguments)

    def assert_rejected(self, reason, **overrides):
        with self.assertRaisesRegex(MODULE.EvidenceError, '^' + reason + '$'):
            self.verify(**overrides)

    def test_valid_fixture_is_never_a_production_acceptance(self):
        receipt = self.verify()
        self.assertTrue(receipt['evidenceBundleValidated'])
        self.assertTrue(receipt['simulationOnly'])
        for name in ('productionReady', 'deployed', 'featuresActivated', 'externalEvidenceAuthenticated'):
            self.assertFalse(receipt[name])

    def test_cli_requires_explicit_identity_and_hash_parameters(self):
        process = subprocess.run([sys.executable, '-B', str(MODULE_PATH)],
                                 text=True, capture_output=True, timeout=10)
        self.assertEqual(process.returncode, 2)
        self.assertIn('--expected-candidate-commit', process.stderr)

    def test_cli_success_and_mismatched_evidence_exit_status(self):
        evidence_path, manifest_hash, evidence_hash = self.bound_files()
        arguments = [sys.executable, '-B', str(MODULE_PATH), '--stage', str(self.stage),
                     '--evidence', str(evidence_path), '--expected-manifest-sha256', manifest_hash,
                     '--expected-evidence-sha256', evidence_hash, '--expected-previous-commit', PREVIOUS,
                     '--expected-candidate-commit', CANDIDATE]
        success = subprocess.run(arguments, text=True, capture_output=True, timeout=10)
        self.assertEqual(success.returncode, 0, success.stderr)
        self.assertFalse(json.loads(success.stdout)['productionReady'])
        evidence_path.write_text('secret-untrusted-content', encoding='utf-8')
        failed = subprocess.run(arguments, text=True, capture_output=True, timeout=10)
        self.assertEqual(failed.returncode, 1)
        self.assertNotIn('secret-untrusted-content', failed.stdout + failed.stderr)

    def test_manifest_and_evidence_hashes_are_independently_bound(self):
        self.assert_rejected('input_hash_mismatch', expected_manifest_hash='a' * 64)
        self.assert_rejected('input_hash_mismatch', expected_evidence_hash='b' * 64)

    def test_candidate_and_previous_commits_are_bound(self):
        self.assert_rejected('invalid_candidate_commit', expected_candidate_commit=CANDIDATE[:12])
        self.assert_rejected('manifest_commit_mismatch', expected_candidate_commit='a' * 40)
        self.assert_rejected('manifest_commit_mismatch', expected_previous_commit='b' * 12)

    def test_asset_tampering_and_unexpected_assets_are_rejected(self):
        (self.stage / MODULE.MIGRATIONS[0]).write_bytes(b'ALTER TABLE secret;')
        self.assert_rejected('asset_hash_mismatch')
        self.manifest['assets']['unexpected'] = 'a' * 64
        self.assert_rejected('unexpected_asset_set')

    def test_reverse_migration_order_is_rejected(self):
        self.manifest['migrations'].reverse()
        self.assert_rejected('migration_order_mismatch')

    def test_production_mode_is_always_rejected(self):
        for mode in ('production', 'live', '', None):
            with self.subTest(mode=mode):
                self.evidence['mode'] = mode
                self.assert_rejected('production_or_unknown_mode_forbidden')

    def test_all_writer_counters_must_be_explicit_zero_in_scope(self):
        original = copy.deepcopy(self.evidence['stoppedWriters'])
        for name in ('apiProcesses', 'workerProcesses', 'applicationConnections', 'uninspectedWriters'):
            for value in (None, 1, False, '0'):
                with self.subTest(name=name, value=value):
                    self.evidence['stoppedWriters'] = copy.deepcopy(original)
                    self.evidence['stoppedWriters'][name] = value
                    self.assert_rejected('writers_not_proven_stopped')
        self.evidence['stoppedWriters'] = {**original, 'scope': 'production'}
        self.assert_rejected('stop_evidence_scope_mismatch')

    def test_default_off_and_preserved_flags_are_checked(self):
        for name in MODULE.OFF_FLAGS:
            with self.subTest(name=name):
                self.evidence['flagsAfter'][name] = 'true'
                self.assert_rejected('default_off_policy_not_proven')
                self.evidence['flagsAfter'][name] = 'false'
        self.evidence['flagsAfter']['ENABLE_DELIVERY_PAYMENT'] = 'false'
        self.assert_rejected('preserved_flag_changed')

    def test_backup_hash_integrity_and_dump_completion_are_checked(self):
        backup = self.stage / 'database.sql.gz'
        backup.write_bytes(b'not-the-bound-backup')
        self.assert_rejected('backup_hash_mismatch')
        invalid_dump = gzip.compress(b'CREATE TABLE fixture (id BIGINT);')
        backup.write_bytes(invalid_dump)
        self.evidence['backup']['sha256'] = digest(invalid_dump)
        self.assert_rejected('incomplete_backup_dump')

    def test_backup_crc_is_read_through_eof(self):
        backup = self.stage / 'database.sql.gz'
        contents = bytearray(backup.read_bytes())
        contents[-8] ^= 1
        backup.write_bytes(contents)
        self.evidence['backup']['sha256'] = digest(contents)
        with self.assertRaises(gzip.BadGzipFile):
            self.verify()

    def test_backup_path_traversal_is_rejected(self):
        self.evidence['backup']['file'] = '../database.sql.gz'
        self.assert_rejected('invalid_asset_name')

    def test_restore_or_migration_changes_to_original_tables_are_rejected(self):
        for field in ('restoredOriginalTables', 'afterMigrationOriginalTables'):
            with self.subTest(field=field):
                table = MODULE.INSPECTOR.MONEY_TABLES[0]
                self.evidence[field][table]['sha256'] = 'a' * 64
                self.assert_rejected('original_table_digest_changed')
                self.evidence[field][table]['sha256'] = digest(b'')

    def test_missing_money_snapshot_or_preexisting_migration_is_rejected(self):
        table = MODULE.INSPECTOR.MONEY_TABLES[0]
        for name in ('beforeMigration', 'restoredOriginalTables', 'afterMigrationOriginalTables'):
            del self.evidence[name][table]
        self.assert_rejected('original_table_inventory_mismatch')
        for name in ('beforeMigration', 'restoredOriginalTables', 'afterMigrationOriginalTables'):
            self.evidence[name][table] = {'rows': 0, 'sha256': digest(b'')}
            self.evidence[name]['delivery_rider_availability'] = {'rows': 0, 'sha256': digest(b'')}
        self.manifest['originalTables'].append('delivery_rider_availability')
        self.manifest['originalTables'].sort()
        self.assert_rejected('migration_already_or_partially_applied')

    def test_manifest_original_inventory_is_exact_not_a_money_only_subset(self):
        self.manifest['originalTables'].append('delivery_orders')
        self.manifest['originalTables'].sort()
        self.assert_rejected('original_table_inventory_mismatch')

    def test_after_preflight_and_new_money_facts_are_checked(self):
        self.evidence['afterPreflight']['counts']['delivery_rider_withdrawal_claims'] = 1
        self.assert_rejected('unexpected_new_money_fact')
        self.evidence['afterPreflight']['counts']['delivery_rider_withdrawal_claims'] = 0
        self.evidence['afterPreflight']['releaseMetadata']['identityMatchesRuntime'] = False
        self.assert_rejected('after_preflight_failed')


if __name__ == '__main__':
    unittest.main()
