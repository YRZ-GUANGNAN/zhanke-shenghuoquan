// Local-only regressions omitted by the historical verify chain. Each child
// runs real source with isolated ports; MySQL and dist gates remain explicit.
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const checks = [
  'check-bottom-nav-motion',
  'check-content-tab-gestures',
  'check-profile-cover-theme',
  'check-public-profile-gestures',
  'check-sheet-dismiss',
  'check-taro-ux-sheet-drag',
  'check-delivery-pager-layout',
  'check-rider-wallet-disabled-style',
  'test-post-media-picker',
  'test-session-welcome',
  'test-manual-delivery-payment',
  'test-wallet-recovery',
  'test-wallet-pages',
  'test-rider-daily-settlement',
  'check-admin-identity-applications-contract',
  'test-miniapp-routes',
  'test-miniapp-package-size',
  'check-miniapp-dark-theme',
  'check-notification-contract',
  'test-notification-contract',
  'test-notification-runtime',
  'test-notification-inbox',
  'test-notifications-page',
  'test-workbench-release-guards',
]

const failures = []
for (const name of checks) {
  const result = spawnSync(process.execPath, [path.join(__dirname, `${name}.cjs`)], {
    cwd: root,
    stdio: 'inherit',
  })
  const code = result.status ?? 1
  console.log(`RESULT ${name}: exit ${code}`)
  if (result.error || code !== 0) failures.push(name)
}
if (failures.length) {
  console.error(`上线回归失败 ${failures.length}/${checks.length}：${failures.join(', ')}`)
  process.exitCode = 1
} else {
  console.log(`上线回归 PASS：${checks.length} 项；不代表生产或真机验收`)
}
