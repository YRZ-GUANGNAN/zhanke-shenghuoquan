const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...segments) => fs.readFileSync(path.join(root, ...segments), 'utf8')

const merchant = read('apps', 'miniapp', 'src', 'pages', 'merchant-home', 'index.tsx')
const merchantStyle = read('apps', 'miniapp', 'src', 'pages', 'merchant-home', 'index.scss')
const rider = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.tsx')
const riderStyle = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.scss')
const riderWallet = read('apps', 'miniapp', 'src', 'pages', 'rider-wallet', 'index.tsx')
const merchantService = read('apps', 'miniapp', 'src', 'services', 'delivery-merchant.ts')
const riderService = read('apps', 'miniapp', 'src', 'services', 'delivery-rider.ts')

for (const fragment of [
  'CapsuleSystemRow',
  'useDidShow',
  "useIdentityGuard('merchant')",
  "getIdentityApplication('merchant')",
  'merchant-home-metrics',
  'merchant-home-shortcuts',
  'merchant-home-tabs',
  'merchant-order-card',
  'onScrollToLower={() => void loadMoreOrders()}',
  "listMerchantDeliveryOrders(1, 50, 'pending')",
  "listMerchantDeliveryOrders(1, 50, 'delivering')",
  "listMerchantDeliveryOrders(1, 50, 'completed')",
  'listMerchantDeliveryOrders(nextPage, 50, requestedTab)',
  'ordersTotalByTab[tab.key]',
]) {
  assert.ok(merchant.includes(fragment), `merchant workbench missing: ${fragment}`)
}

assert.match(
  merchant,
  /loadedByTab\[tab\.key\]\s*\?\s*ordersTotalByTab\[tab\.key\]\s*:\s*['"]—['"]/,
  'merchant tabs must show real totals only after that group has loaded',
)
assert.match(
  merchant,
  /setLoadedByTab\(\(current\)\s*=>\s*\(\{\s*pending:\s*pendingResult\.ok\s*\|\|\s*current\.pending,\s*delivering:\s*deliveringResult\.ok\s*\|\|\s*current\.delivering,\s*completed:\s*completedResult\.ok\s*\|\|\s*current\.completed/,
  'merchant loading state must follow each group result and preserve previously loaded data',
)

for (const fragment of [
  'CapsuleSystemRow',
  'useDidShow',
  "useIdentityGuard('rider')",
  'getDeliveryRiderMe',
  'listClaimableDeliveryOrders',
  'listAssignedDeliveryOrders',
  'rider-grab-task-summary',
  'rider-grab-tabs',
  'rider-order-card',
  'onScrollToLower={() => void loadMoreOrders()}',
  "listAssignedDeliveryOrders(1, 50, 'pickup')",
  "listAssignedDeliveryOrders(1, 50, 'delivery')",
  "listAssignedDeliveryOrders(1, 1, 'completed')",
  '已结束',
  "const statusGroup = requestedTab === 'pickup' ? 'pickup' : 'delivery'",
  'listClaimableDeliveryOrders(nextPage, 50)',
  'listAssignedDeliveryOrders(nextPage, 50, statusGroup)',
  'setLoadMoreError',
]) {
  assert.ok(rider.includes(fragment), `rider workbench missing: ${fragment}`)
}

assert.match(merchantService, /statusGroup=\$\{encodeURIComponent\(statusGroup\)\}/, 'merchant order service must forward the status group')
assert.match(riderService, /statusGroup=\$\{encodeURIComponent\(statusGroup\)\}/, 'rider order service must forward the status group')
assert.doesNotMatch(rider, /setError\(errorMessage\(value, '更多订单加载失败/, 'rider load-more failures must not duplicate the initial-load error')

for (const [name, source] of [['merchant', merchant], ['rider', rider]]) {
  assert.doesNotMatch(source, /window\.(?:alert|confirm|prompt)/, `${name} workbench must use app feedback primitives`)
  assert.doesNotMatch(source, /mock(?:Orders?|Data)|fake(?:Orders?|Data)|demo(?:Orders?|Data)/i, `${name} workbench must not introduce fabricated records`)
}

assert.match(merchantStyle, /grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/, 'merchant metrics must keep three balanced columns')
assert.match(merchantStyle, /grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/, 'merchant shortcuts must keep a compact two-column grid')
assert.match(riderStyle, /grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/, 'rider tabs must keep three balanced columns')
assert.doesNotMatch(merchantStyle, /merchant-home-hero__orb/, 'merchant workbench must not use decorative hero orbs')
assert.doesNotMatch(riderStyle, /rider-grab-hero__orb/, 'rider workbench must not use decorative hero orbs')
assert.match(riderWallet, /骑手结算功能尚未开放/, 'rider wallet must expose an honest unavailable state until a real API exists')
assert.doesNotMatch(riderWallet, /ledgerItems|128\.50|静态示意/, 'rider wallet must not present fabricated balances or ledger records')

console.log('role workbench ui contract: ok')
