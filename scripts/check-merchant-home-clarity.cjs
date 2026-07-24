const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const pageRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'merchant-home')
const source = fs.readFileSync(path.join(pageRoot, 'index.tsx'), 'utf8')
const style = fs.readFileSync(path.join(pageRoot, 'index.scss'), 'utf8')
const iconComponentPath = path.join(pageRoot, 'merchant-home-icon.tsx')
const icons = [
  ['action-publish', 'publishIcon'],
  ['action-wallet', 'walletIcon'],
  ['action-bill', 'billIcon'],
  ['action-message', 'messageIcon'],
  ['empty-orders', 'emptyIcon'],
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
assert.match(source, /icon:\s*'publish'/)
assert.match(source, /icon:\s*'wallet'/)
assert.match(source, /icon:\s*'bill'/)
assert.match(source, /icon:\s*'message'/)
assert.match(source, /name='empty'/)
assert.doesNotMatch(source, /icon:\s*['"](?:\+|¥|楼|账|聊)['"]/)
assert.match(
  style,
  /\.merchant-home-shortcuts[\s\S]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/,
)
assert.match(
  style,
  /\.merchant-home-tabs[\s\S]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/,
)
assert.match(source, /CapsuleSystemRow/)
assert.match(source, /listMerchantDeliveryOrders/)
assert.match(source, /getDeliveryWallet/)
assert.match(source, /getIdentityApplication\('merchant'\)/)

console.log('merchant home clarity contract: ok')
