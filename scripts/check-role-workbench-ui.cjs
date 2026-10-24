const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...segments) => fs.readFileSync(path.join(root, ...segments), 'utf8')

const merchant = read('apps', 'miniapp', 'src', 'pages', 'merchant-home', 'index.tsx')
const merchantStyle = read('apps', 'miniapp', 'src', 'pages', 'merchant-home', 'index.scss')
const rider = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.tsx')
const riderStyle = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.scss')
const workspaceNav = read('apps', 'miniapp', 'src', 'components', 'delivery-workspace-nav.tsx')
const profile = read('apps', 'miniapp', 'src', 'pages', 'delivery-profile', 'index.tsx')
const profileStyle = read('apps', 'miniapp', 'src', 'pages', 'delivery-profile', 'index.scss')
const messages = read('apps', 'miniapp', 'src', 'pages', 'merchant-messages', 'content.tsx')
const riderWallet = read('apps', 'miniapp', 'src', 'pages', 'rider-wallet', 'index.tsx')
const merchantService = read('apps', 'miniapp', 'src', 'services', 'delivery-merchant.ts')
const riderService = read('apps', 'miniapp', 'src', 'services', 'delivery-rider.ts')

for (const fragment of [
  'CapsuleSystemRow',
  'useDidShow',
  "useIdentityGuard('merchant')",
  "getIdentityApplication('merchant')",
  'merchant-home-metrics',
  'merchant-home-tabs',
  'merchant-order-card',
  'onScrollToLower={() => { if (activeTab === tab && channelInteraction.canActivate()) void loadMoreOrders() }}',
  "listMerchantDeliveryOrders(1, 50, 'pending')",
  "listMerchantDeliveryOrders(1, 50, 'delivering')",
  "listMerchantDeliveryOrders(1, 50, 'completed')",
  'listMerchantDeliveryOrders(nextPage, 50, requestedTab)',
  'ordersTotalByTab[tab.key]',
]) {
  assert.ok(merchant.includes(fragment), `merchant workbench missing: ${fragment}`)
}
assert.doesNotMatch(merchant, /merchant-home-publish/, 'merchant workbench must keep a single publish entry in the workspace navigation')

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
  'getDeliveryRiderTodayStats',
  'listClaimableDeliveryOrders',
  'listAssignedDeliveryOrders',
  'rider-grab-task-summary',
  'rider-grab-tabs',
  'rider-order-card',
  'onScrollToLower={() => { if (activeTab === tab) void loadMoreOrders() }}',
  "listAssignedDeliveryOrders(1, 50, 'pickup')",
  "listAssignedDeliveryOrders(1, 50, 'delivery')",
  "listAssignedDeliveryOrders(1, 1, 'completed')",
  '已结束',
  '今日单量',
  '今日收入',
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

assert.match(merchantStyle, /\.merchant-home-tabs\s*\{[^}]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/, '商家保留三个真实订单分组')
assert.match(merchantStyle, /\.merchant-home-order-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/, '商家订单使用双列队列')
assert.match(riderStyle, /\.rider-grab-tabs\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/, '骑手配送视图显示取货和送达两个阶段')
assert.doesNotMatch(merchantStyle, /merchant-home-hero__orb/, 'merchant workbench must not use decorative hero orbs')
assert.doesNotMatch(riderStyle, /rider-grab-hero__orb/, 'rider workbench must not use decorative hero orbs')
assert.match(merchant, /<View className='merchant-home-page' style=\{capsuleStyle\} \{\.\.\.workspace\.swipe\.handlers\}>/, 'merchant page-level swipe handlers must cover the full workspace')
assert.match(rider, /<View className='rider-grab-page' \{\.\.\.workspace\.swipe\.handlers\}>/, 'rider page-level swipe handlers must cover the full workspace')
assert.match(workspaceNav, /swipeItems = items\.filter\(item => item\.swipe !== false\)/, 'workspace swipe order must exclude independent publish action')
assert.match(workspaceNav, /key: 'publish',[\s\S]*swipe: false/, 'merchant publish must remain an independent action')
assert.match(workspaceNav, /gestureGuard\?: \(\) => boolean/, 'bottom navigation must accept the page gesture guard')
assert.match(workspaceNav, /const canActivate = gestureGuard \?\? swipe\.canActivate/, 'bottom navigation must share the page gesture guard')
assert.doesNotMatch(workspaceNav, /return <View className='delivery-workspace-nav' \{\.\.\.swipe\.handlers\}>/, 'navigation must not install a second nested swipe tracker')
for (const [name, source] of [['merchant', merchant], ['rider', rider], ['profile', profile], ['messages', messages]]) {
  if (name === 'merchant' || name === 'rider') {
    assert.match(source, /gestureGuard=\{canActivateWorkspace\}/, `${name} navigation must share both gesture guards`)
    assert.match(source, /workspace\.swipe\.canActivate\(\) && channelInteraction\.canActivate\(\)/)
    assert.match(source, /<DeliveryStagePager/, `${name} stage gestures must not bubble into workspace navigation`)
    assert.match(source, /onBoundarySwipe=\{workspace\.step\}/)
  } else assert.match(source, /gestureGuard=\{workspace\.swipe\.canActivate\}/, `${name} navigation must share the page gesture guard`)
}
assert.doesNotMatch(merchant, /profile\.cover|profile\.avatar|merchant-home-hero__cover-image|ChatAvatar/, 'merchant order page must keep personal media on the profile page')
assert.doesNotMatch(rider, /profile\.cover|profile\.avatar|rider-grab-hero__cover-image|ChatAvatar/, 'rider order page must keep personal media on the profile page')
assert.match(rider, /todayStatsLoaded && todayStats \? todayStats\.todayOrders : ['"]—['"]/, 'rider daily order count must fail closed until real stats are loaded')
assert.match(rider, /todayStats\.todayIncomeCents/, 'rider daily income must use the stats service cents field')
assert.match(riderService, /stats\/today/, 'rider daily stats must use the dedicated read-only endpoint')
assert.match(merchant, /merchant-home-metric__label.*全部订单|全部订单/, 'merchant order page must show an order-only total')
assert.match(profile, /delivery-profile__hero-image/, 'role profile must keep the real cover image')
assert.match(profileStyle, /\.delivery-profile__hero\s*\{[^}]*overflow:\s*hidden/, 'cover must clip image edges')
assert.match(profileStyle, /\.delivery-profile__identity\s*\{[^}]*background:\s*\$bg-surface/, 'identity must form an opaque cover boundary')
assert.match(profileStyle, /\.delivery-profile__identity--cover\s*\{[^}]*margin-top:\s*-20PX/, 'cover and identity must overlap without a seam')
assert.match(riderWallet, /骑手结算功能尚未开放/, 'rider wallet must expose an honest unavailable state until a real API exists')
assert.doesNotMatch(riderWallet, /ledgerItems|128\.50|静态示意/, 'rider wallet must not present fabricated balances or ledger records')

console.log('role workbench ui contract: ok')
