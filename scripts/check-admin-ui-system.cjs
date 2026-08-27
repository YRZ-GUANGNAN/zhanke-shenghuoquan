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
for (const group of ['运营中心', '内容治理', '平台管理']) {
  assert.match(navigationSource, new RegExp(group), `admin navigation must include ${group}`)
}

const sources = fs.readdirSync(src, { withFileTypes: true })
  .filter((entry) => entry.isFile() && entry.name.endsWith('.tsx'))
  .map((entry) => fs.readFileSync(path.join(src, entry.name), 'utf8'))
  .join('\n')

assert.doesNotMatch(sources, /window\.(confirm|prompt|alert)\s*\(/)
const appSource = fs.readFileSync(path.join(src, 'App.tsx'), 'utf8')
const adminAPI = fs.readFileSync(path.join(src, 'services', 'f-admin.ts'), 'utf8')
assert.doesNotMatch(appSource, /icon:\s*'[A-Z]'/)

assert.match(adminAPI, /reviews:\s*\(query[^)]*=\s*\{\}\)\s*=>/,
  'review API must accept a query object instead of a status-only argument')
for (const field of ['status', 'page', 'pageSize']) {
  assert.match(adminAPI, new RegExp(`type ReviewQuery=[^\\n]*${field}`), `review API query must include ${field}`)
}
assert.match(appSource, /const\s*\[reviewStatus,\s*setReviewStatus\]\s*=\s*useState/,
  'review page must keep the active status filter')
assert.match(appSource, /const\s*\[reviewPage,\s*setReviewPage\]\s*=\s*useState/,
  'review page must keep the current server page')
assert.match(appSource, /adminAPI\.reviews\(\{status:reviewStatus,page:reviewPage,pageSize:REVIEW_PAGE_SIZE\}\)/,
  'review refresh must preserve status, page, and pageSize')
assert.match(appSource, /data\?\.total/,
  'review page must render the server total rather than the visible row count')
assert.match(appSource, /value\.items\.length===0&&reviewPage>1/,
  'review refresh must step back when an action empties the current page')
assert.match(appSource, />上一页</)
assert.match(appSource, />下一页</)
assert.match(appSource, /function\s+Feedbacks\(/, 'admin must expose the existing feedback list')
assert.match(adminAPI, /feedbacks:\s*\(query/, 'feedback API must accept pagination')
assert.match(adminAPI, /createNavigationCategory:/, 'navigation category create API must be wired')
assert.match(adminAPI, /deleteNavigationItem:/, 'navigation item delete API must be wired')

const styles = fs.readFileSync(path.join(src, 'styles.css'), 'utf8')
for (const token of ['--color-primary', '--radius-control', '--shadow-dialog', '--duration-fast']) {
  assert.match(styles, new RegExp(token))
}
for (const contract of ['focus-visible', '@media (max-width: 900px)', 'overflow-x: auto']) {
  assert.ok(styles.includes(contract), `styles must include ${contract}`)
}
console.log('admin ui system: ok')
