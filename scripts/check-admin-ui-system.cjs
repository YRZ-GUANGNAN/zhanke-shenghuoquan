const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const src = path.join(root, 'apps', 'admin', 'src')
const componentRoot = path.join(src, 'components')

for (const name of ['AdminButton', 'AdminField', 'AdminSelect', 'AdminIcon', 'AdminState', 'AdminBadge', 'AdminNotice', 'AdminDialog', 'AdminDialogProvider', 'AdminPageHeader', 'AdminTableFrame']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}
for (const name of ['AdminPanel', 'AdminToolbar', 'AdminPagination', 'AdminStatusBadge', 'AdminRowActions']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}

const navigationSource = fs.readFileSync(path.join(src, 'admin-navigation.ts'), 'utf8')
for (const group of ['工作台', '业务运营', '内容与用户', '平台设置']) {
  assert.match(navigationSource, new RegExp(group), `admin navigation must include ${group}`)
}
assert.match(navigationSource, /key:\s*'notifications'/, 'admin navigation must expose notification operations')
assert.match(navigationSource, /label:\s*'经营总览'/, 'dashboard must use revenue-oriented business language')

const sources = fs.readdirSync(src, { withFileTypes: true })
  .filter((entry) => entry.isFile() && entry.name.endsWith('.tsx'))
  .map((entry) => fs.readFileSync(path.join(src, entry.name), 'utf8'))
  .join('\n')

// 线上后台入口是 src/preview/App.tsx；旧 src/App.tsx 已删除。
const previewRoot = path.join(src, 'preview')
const previewFiles = []
;(function collect(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name)
    if (entry.isDirectory()) collect(target)
    else if (/\.(ts|tsx)$/.test(entry.name)) previewFiles.push(target)
  }
})(previewRoot)
const previewSources = previewFiles.map(file => fs.readFileSync(file, 'utf8')).join('\n')

assert.doesNotMatch(sources + previewSources, /window\.(confirm|prompt|alert)\s*\(/)
const adminAPI = fs.readFileSync(path.join(src, 'services', 'f-admin.ts'), 'utf8')

assert.match(adminAPI, /reviews:\s*\(query[^)]*=\s*\{\}\)\s*=>/,
  'review API must accept a query object instead of a status-only argument')
for (const field of ['status', 'page', 'pageSize']) {
  assert.match(adminAPI, new RegExp(`type ReviewQuery=[^\\n]*${field}`), `review API query must include ${field}`)
}
assert.match(previewSources, /adminAPI\.reviews\(\{\s*status/,
  'review views must call the query-object review API with a status')
assert.match(previewSources, /adminAPI\.reviews\(\{[^}]*pageSize/,
  'review views must request an explicit page size')
assert.match(previewSources, />上一页</)
assert.match(previewSources, />下一页</)
assert.match(previewSources, /function FeedbackPeople\(/, 'admin must expose the existing feedback list')
assert.match(previewSources, /NotificationsView/, 'admin must route the notification operations page')
assert.match(previewSources, /deliveryAdminAPI\.summary\(\)/, 'dashboard must load real delivery revenue facts')
for (const field of ['platformFeeIncomeCents', 'todayPlatformFeeCents', 'todayDeliveryFeeCents', 'pendingRiderFeeCents']) {
  assert.ok(previewSources.includes(field), `dashboard must display ${field}`)
}
assert.match(adminAPI, /feedbacks:\s*\(query/, 'feedback API must accept pagination')
assert.match(adminAPI, /createNavigationCategory:/, 'navigation category create API must be wired')
assert.match(adminAPI, /deleteNavigationItem:/, 'navigation item delete API must be wired')

const styles = fs.readFileSync(path.join(src, 'styles.css'), 'utf8')
for (const token of ['--color-primary', '--radius-control', '--shadow-dialog', '--duration-fast']) {
  assert.match(styles, new RegExp(token))
}
assert.ok(fs.existsSync(path.join(src, 'NotificationConsole.tsx')), 'NotificationConsole.tsx is required')
for (const contract of ['.notification-channel-grid', '.notification-detail-grid', '.notification-binding-progress']) {
  assert.ok(styles.includes(contract), `notification admin styles must include ${contract}`)
}
for (const contract of ['focus-visible', '@media (max-width: 900px)', 'overflow-x: auto']) {
  assert.ok(styles.includes(contract), `styles must include ${contract}`)
}
console.log('admin ui system: ok')
