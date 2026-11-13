// 校验每个迁移都有回滚脚本。
// 下面这批是规则建立之前就已合并的历史迁移，暂时豁免；新增迁移必须自带回滚，
// 否则本脚本失败。豁免清单只允许缩小，不允许新增。
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const migrationsDir = path.join(root, 'backend', 'migrations')
const rollbackDir = path.join(migrationsDir, 'rollback')

const LEGACY_WITHOUT_ROLLBACK = new Set([
  '001_init',
  '002_seed_dev',
  '003_domain_convergence',
  '004_auth_hardening',
  '005_community_loop',
  '006_transaction_closure',
  '007_media_pipeline',
  '008_media_dimensions_and_audit',
  '031_external_takeout_merchant_directory',
  '032_domain_search_and_community_tags',
  '033_user_settings_feedback_account_closure',
  '034_identity_application_review',
  '035_admin_password_login',
  '040_share_settings',
  '041_feature_policy_presentation',
  '042_user_profile_cover',
  '044_media_review_policy',
  '045_delivery_order_source',
  '046_delivery_external_inbox'
])

const migrations = fs.readdirSync(migrationsDir)
  .filter(name => name.endsWith('.sql'))
  .map(name => name.replace(/\.sql$/, ''))
  .sort()

if (migrations.length === 0) {
  console.error('没有找到任何迁移文件')
  process.exit(1)
}

const rollbacks = new Set(fs.readdirSync(rollbackDir))
const missing = []

for (const migration of migrations) {
  if (LEGACY_WITHOUT_ROLLBACK.has(migration)) continue
  if (!rollbacks.has(`${migration}.sql`) && !rollbacks.has(`${migration}.down.sql`)) {
    missing.push(migration)
  }
}

if (missing.length > 0) {
  console.error('以下迁移缺少回滚脚本（backend/migrations/rollback/）：')
  for (const name of missing) console.error(`  - ${name}`)
  process.exit(1)
}

const covered = migrations.length - LEGACY_WITHOUT_ROLLBACK.size
console.log(`migrations: ${migrations.length} 个迁移，${covered} 个有回滚脚本，${LEGACY_WITHOUT_ROLLBACK.size} 个历史迁移已豁免`)
