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

for (const fragment of [
  'CapsuleSystemRow',
  "useIdentityGuard('merchant')",
  "getIdentityApplication('merchant')",
  'merchant-home-metrics',
  'merchant-home-shortcuts',
  'merchant-home-tabs',
  'merchant-order-card',
]) {
  assert.ok(merchant.includes(fragment), `merchant workbench missing: ${fragment}`)
}

for (const fragment of [
  'CapsuleSystemRow',
  "useIdentityGuard('rider')",
  'getDeliveryRiderMe',
  'listClaimableDeliveryOrders',
  'listAssignedDeliveryOrders',
  'rider-grab-task-summary',
  'rider-grab-tabs',
  'rider-order-card',
]) {
  assert.ok(rider.includes(fragment), `rider workbench missing: ${fragment}`)
}

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
