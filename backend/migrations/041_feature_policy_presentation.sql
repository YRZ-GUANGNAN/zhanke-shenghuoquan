-- Preserve every original module flag and its audit/version metadata.
-- Older binaries ignore this additive column. Rollback never drops user data.
ALTER TABLE feature_policy_configs
    ADD COLUMN presentation_mode VARCHAR(24) NOT NULL DEFAULT 'standard',
    ADD CONSTRAINT chk_feature_policy_presentation_mode
        CHECK (presentation_mode IN ('standard', 'campus_info'));
