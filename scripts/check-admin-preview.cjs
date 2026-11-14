const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const admin = path.join(root, 'apps', 'admin')
const preview = path.join(admin, 'src', 'preview')

const requiredFiles = [
  path.join(admin, 'preview.html'),
  path.join(preview, 'main.tsx'),
  path.join(preview, 'App.tsx'),
  path.join(preview, 'data.ts'),
  path.join(preview, 'extra-data.ts'),
  path.join(preview, 'store.ts'),
  path.join(preview, 'ui.tsx'),
  path.join(preview, 'ExtraViews.tsx'),
  path.join(preview, 'styles.css'),
  path.join(preview, 'views', 'DashboardView.tsx'),
  path.join(preview, 'views', 'NotificationsView.tsx'),
  path.join(preview, 'views', 'DeliveryView.tsx'),
  path.join(preview, 'views', 'OperationsView.tsx'),
  path.join(preview, 'views', 'PeopleView.tsx'),
  path.join(preview, 'views', 'SettingsView.tsx'),
  path.join(preview, 'views', 'LegacyTakeoutView.tsx')
]

for (const file of requiredFiles) {
  assert.ok(fs.existsSync(file), `${path.relative(root, file)} is required`)
}

const read = (...parts) => fs.readFileSync(path.join(preview, ...parts), 'utf8')
const app = read('App.tsx')
const store = read('store.ts')
const notifications = read('views', 'NotificationsView.tsx')
const operations = read('views', 'OperationsView.tsx')
const settings = read('views', 'SettingsView.tsx')

for (const label of ['经营总览', '上线流程', '配送与结算', '消息通知', '素材中心', '站点管理', '表单管理', '接单员管理', '推广员管理', '商品库存', '营销管理', '私信管理', '广告管理', '审核中心', '社区内容', '二手商品', '举报处理', '商家骑手认证', '意见反馈', '小程序设置', '服务号设置', '提现管理', '用户与权限', '服务入口', '分享设置', '功能开关', '通用设置', '旧版外卖']) {
  assert.ok(app.includes(label), `preview navigation must include ${label}`)
}

assert.match(store, /persist\s*\(/, 'preview store must persist local changes')
assert.match(store, /resetPreview/, 'preview store must expose resetPreview')
assert.match(notifications, /消息模板/, 'notification view must include message templates')
assert.match(notifications, /Webhook/, 'notification view must include webhooks')
assert.match(notifications, /testSend|testWebhook/, 'notification view must support test actions')
assert.match(notifications, /retryDelivery/, 'notification view must retry failed deliveries')
assert.match(operations, /approveReview|rejectReview/, 'operations view must approve or reject reviews')
assert.match(settings, /saveSettings/, 'settings view must save local settings')

const html = fs.readFileSync(path.join(admin, 'preview.html'), 'utf8')
assert.match(html, /src\/preview\/main\.tsx/, 'preview.html must load the preview entry')

console.log('admin preview contract: ok')
