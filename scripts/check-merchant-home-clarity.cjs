const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const pageRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'merchant-home')
const source = fs.readFileSync(path.join(pageRoot, 'index.tsx'), 'utf8')
const style = fs.readFileSync(path.join(pageRoot, 'index.scss'), 'utf8')
const workspaceNav = fs.readFileSync(path.join(pageRoot, '../../components/delivery-workspace-nav.tsx'), 'utf8')
const wallet = fs.readFileSync(path.join(pageRoot, '../merchant-wallet/index.tsx'), 'utf8')
const appIcon = fs.readFileSync(path.join(pageRoot, '../../components/app-icon.tsx'), 'utf8')
const iconComponentPath = path.join(pageRoot, 'merchant-home-icon.tsx')
const icons = [
  ['action-wallet', 'walletIcon'],
  ['action-bill', 'billIcon'],
  ['action-message', 'messageIcon'],
]

assert.ok(fs.existsSync(iconComponentPath), 'merchant home icon component is required')
const iconComponent = fs.readFileSync(iconComponentPath, 'utf8')
for (const [icon, binding] of icons) {
  const file = path.join(pageRoot, 'icons', `${icon}.svg`)
  assert.ok(fs.existsSync(file), `${icon}.svg is required`)
  const svg = fs.readFileSync(file, 'utf8')
  assert.match(svg, /viewBox="0 0 48 48"/, `${icon}.svg must use the shared viewBox`)
  assert.match(iconComponent, new RegExp(`\\b${binding}\\b`), `${icon}.svg must be registered`)
}

assert.match(source, /MerchantHomeIcon/)
assert.match(source, /name='message'/)
assert.match(source, /<DeliveryWorkspaceNav role='merchant' active='orders'/, '商家订单使用统一底部导航')
assert.doesNotMatch(source, /name='publish'|merchant-publish\/index/, '订单内容不得重复增加发单入口')
assert.equal((workspaceNav.match(/key:\s*'publish'/g) || []).length, 1, '底部导航只保留一个发单入口')
assert.match(workspaceNav, /key:\s*'publish',\s*label:\s*'发单',\s*icon:\s*'document',\s*url:\s*'\/pages\/merchant-publish\/index'/)
assert.match(workspaceNav, /<AppIcon name=\{item\.icon\}/, '底部导航使用项目统一 SVG 图标')
assert.match(appIcon, /document:\s*\{\s*normal:\s*actionDocument,\s*active:\s*actionDocument\s*\}/, '唯一发单入口使用已注册的统一文档图标')
assert.match(workspaceNav, /if\s*\(item\.key === 'publish'\)\s*return visibility\.delivery && visibility\.merchant/, '发单可见性仍受配送与商家功能开关保护')
assert.match(source, /key: 'pending', label: '待处理'/)
assert.match(source, /key: 'completed', label: '已结束'/)
assert.doesNotMatch(source, /name='empty'/, '订单空态使用最新配送反馈组件')
assert.match(source, /RefreshScrollView/)
assert.match(source, /DeliveryOrderState/)
assert.doesNotMatch(source, /icon:\s*['"](?:\+|¥|楼|账|聊)['"]/)
assert.match(
  style,
  /\.merchant-home-order-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/,
)
assert.match(
  style,
  /\.merchant-home-tabs[\s\S]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/,
)
assert.match(source, /CapsuleSystemRow/)
assert.match(source, /listMerchantDeliveryOrders/)
assert.doesNotMatch(source, /getDeliveryWallet/, '订单页不读取钱包')
assert.match(wallet, /getDeliveryWallet/)
assert.match(wallet, /listDeliveryWalletLedger/)
assert.match(source, /getIdentityApplication\('merchant'\)/)

console.log('merchant home clarity contract: ok')
