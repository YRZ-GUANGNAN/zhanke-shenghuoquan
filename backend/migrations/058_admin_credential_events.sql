-- Multi-administrator support records who assigned or removed a login credential.
-- The audit table keeps its existing rows; only the allowed event set grows.
ALTER TABLE admin_auth_audit_logs DROP CHECK chk_admin_auth_audit_event;
ALTER TABLE admin_auth_audit_logs ADD CONSTRAINT chk_admin_auth_audit_event CHECK (
  event_type IN (
    'login','logout','password_changed','credential_bootstrap','credential_reset',
    'credential_set','credential_revoke','account_locked','sessions_revoked'
  )
);
