-- Additive extension of the existing module policy, without touching business data.
ALTER TABLE feature_policy_configs ADD COLUMN extra_flags JSON NULL;
UPDATE feature_policy_configs SET extra_flags = '{"search":true,"likes":true,"favorites":true,"notifications":true,"identity_application":true,"publish":true,"secondhand_comments":true}' WHERE extra_flags IS NULL;
ALTER TABLE feature_policy_configs MODIFY COLUMN extra_flags JSON NOT NULL;
