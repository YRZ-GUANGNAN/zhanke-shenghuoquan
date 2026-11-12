#!/usr/bin/env python3
"""Offline-only 055/056 rehearsal evidence verifier. Never connects or deploys.

The supplied manifest/evidence hashes are explicit operator inputs. Validation
proves only that this evidence bundle is internally consistent; it cannot
authenticate an external production observation. Fixture receipts are always
simulations. Even isolated-MySQL receipts never authorize production changes.
"""
import argparse
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import sys


MIGRATIONS = ('055_delivery_rider_availability.sql',
              '056_delivery_rider_withdrawal_allocations.sql')
ASSETS = ('campus-api', *MIGRATIONS)
OFF_FLAGS = ('ENABLE_DELIVERY_RIDER_BROADCAST', 'ENABLE_DELIVERY_RIDER_WITHDRAWAL',
             'ENABLE_DELIVERY_RIDER_PAYOUT_QUEUE', 'ENABLE_NOTIFICATION_SENDING')
PRESERVED_FLAGS = ('ENABLE_DELIVERY_PAYMENT', 'ENABLE_DELIVERY_RIDER_DAILY_SETTLEMENT')
MAX_BACKUP_BYTES = 256 * 1024 * 1024
SPEC = importlib.util.spec_from_file_location('rider_release_inspector',
                                             Path(__file__).with_name('inspect-rider-release.py'))
INSPECTOR = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(INSPECTOR)


class EvidenceError(Exception):
    pass


def require(condition, code):
    if not condition:
        raise EvidenceError(code)


def hexadecimal(value, minimum, maximum):
    return isinstance(value, str) and re.fullmatch('[a-f0-9]{%d,%d}' % (minimum, maximum), value) is not None


def sha256_file(path):
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return digest.hexdigest()


def safe_file(directory, name):
    require(isinstance(name, str) and re.fullmatch('[A-Za-z0-9._-]{1,120}', name) is not None,
            'invalid_asset_name')
    path = directory / name
    require(not directory.is_symlink() and directory.is_dir()
            and not path.is_symlink() and path.is_file()
            and path.resolve().parent == directory.resolve(), 'unsafe_asset_path')
    return path


def read_bound_json(path, expected_hash):
    require(hexadecimal(expected_hash, 64, 64), 'invalid_input_hash')
    require(not path.is_symlink() and path.is_file(), 'unsafe_input_path')
    raw = path.read_bytes()
    require(hashlib.sha256(raw).hexdigest() == expected_hash, 'input_hash_mismatch')
    value = json.loads(raw.decode('utf-8'))
    require(isinstance(value, dict), 'invalid_json_object')
    return value


def validate_backup(path, expected_hash):
    require(hexadecimal(expected_hash, 64, 64) and sha256_file(path) == expected_hash,
            'backup_hash_mismatch')
    total = 0
    found_create = False
    tail = b''
    # Reading through EOF verifies gzip CRC/trailer, not merely the gzip magic.
    with gzip.open(path, 'rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            total += len(block)
            require(total <= MAX_BACKUP_BYTES, 'backup_exceeds_rehearsal_limit')
            combined = tail + block
            found_create = found_create or b'CREATE TABLE' in combined
            tail = combined[-4096:]
    require(total > 0 and found_create and b'-- Dump completed on ' in tail,
            'incomplete_backup_dump')
    return total


def validate_snapshot(snapshot):
    require(isinstance(snapshot, dict) and snapshot, 'missing_original_table_snapshot')
    for table, row in snapshot.items():
        require(re.fullmatch('[A-Za-z0-9_]{1,64}', table) is not None
                and isinstance(row, dict), 'invalid_original_table_snapshot')
        count = row.get('rows')
        require(isinstance(count, int) and not isinstance(count, bool) and count >= 0
                and hexadecimal(row.get('sha256'), 64, 64), 'invalid_original_table_snapshot')


def verify_bundle(stage, evidence_path, expected_manifest_hash, expected_evidence_hash,
                  expected_previous_commit, expected_candidate_commit):
    require(hexadecimal(expected_previous_commit, 12, 40), 'invalid_previous_commit')
    require(hexadecimal(expected_candidate_commit, 40, 40), 'invalid_candidate_commit')
    require(expected_previous_commit != expected_candidate_commit
            and not expected_candidate_commit.startswith(expected_previous_commit),
            'candidate_must_differ_from_previous')
    manifest = read_bound_json(safe_file(stage, 'manifest.json'), expected_manifest_hash)
    evidence = read_bound_json(evidence_path, expected_evidence_hash)
    require(type(manifest.get('schemaVersion')) is int and manifest['schemaVersion'] == 1
            and type(evidence.get('schemaVersion')) is int and evidence['schemaVersion'] == 1,
            'unsupported_evidence_schema')
    require(manifest.get('candidateCommit') == expected_candidate_commit
            and manifest.get('previousCommit') == expected_previous_commit,
            'manifest_commit_mismatch')
    require(evidence.get('candidateCommit') == expected_candidate_commit
            and evidence.get('previousCommit') == expected_previous_commit
            and evidence.get('manifestSha256') == expected_manifest_hash,
            'evidence_identity_mismatch')
    assets = manifest.get('assets')
    require(isinstance(assets, dict) and set(assets) == set(ASSETS), 'unexpected_asset_set')
    for name in ASSETS:
        require(hexadecimal(assets[name], 64, 64)
                and sha256_file(safe_file(stage, name)) == assets[name], 'asset_hash_mismatch')
    with safe_file(stage, 'campus-api').open('rb') as stream:
        require(stream.read(4) == b'\x7fELF', 'candidate_must_be_linux_elf')
    require(manifest.get('migrations') == list(MIGRATIONS), 'migration_order_mismatch')
    mode = evidence.get('mode')
    require(mode in ('fixture', 'isolated_mysql'), 'production_or_unknown_mode_forbidden')
    stopped = evidence.get('stoppedWriters', {})
    for name in ('apiProcesses', 'workerProcesses', 'applicationConnections', 'uninspectedWriters'):
        require(type(stopped.get(name)) is int and stopped[name] == 0, 'writers_not_proven_stopped')
    require(stopped.get('scope') == mode, 'stop_evidence_scope_mismatch')
    before_flags = evidence.get('flagsBefore', {})
    after_flags = evidence.get('flagsAfter', {})
    for name in OFF_FLAGS:
        require(before_flags.get(name) in ('false', '0') and after_flags.get(name) in ('false', '0'),
                'default_off_policy_not_proven')
    for name in PRESERVED_FLAGS:
        require(before_flags.get(name) in ('true', '1', 'false', '0')
                and before_flags.get(name) == after_flags.get(name), 'preserved_flag_changed')
    backup = evidence.get('backup', {})
    require(backup.get('completed') is True, 'backup_not_complete')
    backup_bytes = validate_backup(safe_file(stage, backup.get('file')), backup.get('sha256'))
    original = evidence.get('beforeMigration')
    restored = evidence.get('restoredOriginalTables')
    after = evidence.get('afterMigrationOriginalTables')
    for snapshot in (original, restored, after):
        validate_snapshot(snapshot)
    inventory = manifest.get('originalTables')
    require(isinstance(inventory, list) and all(isinstance(name, str) for name in inventory)
            and inventory == sorted(set(inventory)) and set(original) == set(inventory),
            'original_table_inventory_mismatch')
    require(original == restored == after, 'original_table_digest_changed')
    for name in INSPECTOR.MONEY_TABLES:
        require(name in original, 'money_table_snapshot_missing')
    require(not set(INSPECTOR.MIGRATION_055_TABLES + INSPECTOR.MIGRATION_056_TABLES).intersection(original),
            'migration_already_or_partially_applied')
    snapshot = evidence.get('afterPreflight')
    require(isinstance(snapshot, dict), 'missing_after_preflight')
    preflight = INSPECTOR.evaluate(snapshot, expected_commit=expected_candidate_commit)
    require(preflight['defaultOffCandidateReady'] and preflight['migration055Ready']
            and preflight['migration056Ready'], 'after_preflight_failed')
    for name in INSPECTOR.MIGRATION_056_TABLES:
        require(snapshot.get('counts', {}).get(name) == 0, 'unexpected_new_money_fact')
    require(snapshot.get('counts', {}).get('delivery_rider_availability') == 0,
            'unexpected_new_availability_fact')
    require(snapshot.get('flags') == after_flags, 'after_flags_snapshot_mismatch')
    return {
        'readOnly': True, 'offlineOnly': True, 'mode': mode,
        'simulationOnly': mode == 'fixture', 'evidenceBundleValidated': True,
        'productionReady': False, 'deployed': False, 'featuresActivated': False,
        'candidateCommit': expected_candidate_commit,
        'previousCommit': expected_previous_commit,
        'manifestSha256': expected_manifest_hash, 'evidenceSha256': expected_evidence_hash,
        'backupCompressedSha256': backup['sha256'], 'backupUncompressedBytes': backup_bytes,
        'originalTableCount': len(original), 'originalRowsUnchanged': True,
        'externalEvidenceAuthenticated': False,
        'evidenceClaimsOnly': True,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--stage', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    parser.add_argument('--expected-manifest-sha256', required=True)
    parser.add_argument('--expected-evidence-sha256', required=True)
    parser.add_argument('--expected-previous-commit', required=True)
    parser.add_argument('--expected-candidate-commit', required=True)
    args = parser.parse_args()
    result = verify_bundle(args.stage, args.evidence, args.expected_manifest_sha256,
                           args.expected_evidence_sha256, args.expected_previous_commit,
                           args.expected_candidate_commit)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(json.dumps({'readOnly': True, 'offlineOnly': True, 'failed': True,
                          'productionReady': False, 'deployed': False,
                          'reason': str(error) if isinstance(error, EvidenceError) else 'invalid_evidence_bundle'}))
        raise SystemExit(1)
