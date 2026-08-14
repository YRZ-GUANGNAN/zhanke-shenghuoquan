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
  if (icon === 'action-publish') {
    assert.match(svg, /stroke="#2476D6"/, 'publish icon must remain legible on a light shortcut tile')
  }
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
assert.doesNotMatch(source, /merchant-home-shop__green-dot/)
assert.doesNotMatch(style, /\.merchant-home-shop__green-dot/)
assert.match(source, /merchant-order-card__identity/)
assert.match(source, /merchant-order-card__time/)
assert.match(source, /merchant-home-tab__count--empty/)
assert.match(source, /merchant-order-route__step/)
assert.match(style, /\.merchant-order-route__step/)
assert.match(style, /\.merchant-home-switch[\s\S]*min-height:\s*44PX/)
assert.match(style, /\.merchant-home-tab[\s\S]*min-height:\s*44PX/)
assert.match(style, /\.merchant-order-button[\s\S]*min-height:\s*44PX/)
assert.match(style, /@media\s*\(max-width:\s*340px\)/)
assert.match(source, /key:\s*'publish'[^\n]*url:\s*'\/pages\/merchant-publish\/index'/)
assert.match(source, /key:\s*'wallet'[^\n]*url:\s*'\/pages\/merchant-wallet\/index'/)
assert.match(source, /key:\s*'bill'[^\n]*url:\s*'\/pages\/merchant-wallet\/index'/)
assert.match(source, /key:\s*'message'[^\n]*url:\s*'\/pages\/merchant-messages\/index'/)
assert.match(source, /pending:\s*pendingResult\.ok\s*\?\s*pendingResult\.value\.total/)
assert.match(source, /delivering:\s*deliveringResult\.ok\s*\?\s*deliveringResult\.value\.total/)
assert.match(source, /completed:\s*completedResult\.ok\s*\?\s*completedResult\.value\.total/)
assert.match(
  source,
  /new Map\(current\[requestedTab\]\.map\(\(order\)\s*=>\s*\[order\.deliveryOrderPublicId,\s*order\]\)\)/,
)
assert.match(source, /\[requestedTab\]:\s*result\.total/)
assert.match(source, /onScrollToLower=\{\(\)\s*=>\s*void loadMoreOrders\(\)\}/)
assert.match(source, /encodeURIComponent\(order\.deliveryOrderPublicId\)/)
assert.match(source, /role=merchant/)

console.log('merchant home clarity contract: ok')
