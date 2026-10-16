const fs = require('node:fs')
const path = require('node:path')
const { assertSyntheticStaticToolsRejected, homeToolsDataFlowViolations } = require('./lib/home-tools-contract.cjs')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps', 'miniapp', 'src')
const config = fs.readFileSync(path.join(src, 'app.config.ts'), 'utf8')
const routes = [...config.matchAll(/'((?:pages\/)[^']+\/index)'/g)].map((match) => match[1])

const forbiddenCopy = /静态(?:预览|占位|示意)|后续接入|下一阶段接入|TODO|128\.50|86\.50/
const violations = []
for (const route of routes) {
  const file = path.join(src, `${route}.tsx`)
  let source = fs.readFileSync(file, 'utf8')
  if (route === 'pages/merchant-messages/index' || route === 'pages/rider-messages/index') {
    source += fs.readFileSync(path.join(src, 'pages/merchant-messages/content.tsx'), 'utf8')
  }
  if (forbiddenCopy.test(source)) violations.push(`${route}: contains placeholder or fake-value copy`)
}

const mustContain = [
  ['pages/user-profile/index.tsx', 'getPublicCommunityProfile'],
  ['pages/user-profile/index.tsx', 'getPublicUserProducts'],
  ['pages/my-content/index.tsx', 'getMyFavoritePosts'],
  ['pages/edit-profile/index.tsx', 'updateProfile'],
  ['pages/delivery-rider/index.tsx', 'listClaimableDeliveryOrders'],
  ['pages/delivery-conversation/index.tsx', 'loadDeliveryTencentMessages'],
  ['pages/merchant-home/index.tsx', 'listMerchantDeliveryOrders'],
  ['pages/merchant-home/index.tsx', 'getDeliveryWallet'],
  ['pages/merchant-publish/index.tsx', 'createMerchantDeliveryOrder'],
  ['pages/merchant-wallet/index.tsx', 'listDeliveryWalletLedger'],
  ['pages/merchant-messages/content.tsx', 'listDeliveryConversations'],
  ['pages/merchant-order-detail/index.tsx', 'getMerchantDeliveryOrder']
]
for (const [relative, token] of mustContain) {
  const source = fs.readFileSync(path.join(src, relative), 'utf8')
  if (!source.includes(token)) violations.push(`${relative}: missing ${token}`)
}

const riderPage = fs.readFileSync(path.join(src, 'pages/delivery-rider/index.tsx'), 'utf8')
for (const token of [
  'CapsuleSystemRow',
  "className='rider-grab-hero'",
  'RiderWorkbenchIcon',
  "className='rider-grab-task-summary'",
  "className='rider-empty-state'",
  "label: '待抢单'",
  "label: '待取货'",
  "label: '待送达'",
  'claimDeliveryOrder(order)',
  "chooseAndUploadImages('delivery_proof'",
  'openConversation(order.deliveryOrderPublicId)'
]) {
  if (!riderPage.includes(token)) {
    violations.push(`pages/delivery-rider/index.tsx: missing restored real-data rider workbench contract ${token}`)
  }
}
if (/<Text>骑<\/Text>|rider-grab-hero__orb|rider-grab-list-head/.test(riderPage)) {
  violations.push('pages/delivery-rider/index.tsx: legacy character icons or duplicate rider list controls must stay removed')
}

const home = fs.readFileSync(path.join(src, 'pages/home/index.tsx'), 'utf8')
if (/静态占位|showTodoToast|特价影票|学生兼职|校园群聊|校园地图/.test(home)) {
  violations.push('pages/home/index.tsx: static fake tools must not be registered')
}
for (const violation of homeToolsDataFlowViolations(home)) {
  violations.push(`pages/home/index.tsx: ${violation}`)
}
assertSyntheticStaticToolsRejected(home)

const identityGuard = fs.readFileSync(path.join(src, 'hooks/use-identity-guard.ts'), 'utf8')
if (!identityGuard.includes("merchant: '/pages/merchant-home/index'")) {
  violations.push('merchant identity must route to the new real-data merchant workbench')
}
const merchantHome = fs.readFileSync(path.join(src, 'pages/merchant-home/index.tsx'), 'utf8')
for (const token of [
  "getIdentityApplication('merchant')",
  "applicationResult.value.status !== 'approved'",
  "setApprovalState('blocked')"
]) {
  if (!merchantHome.includes(token)) {
    violations.push(`pages/merchant-home/index.tsx: missing reviewed application gate ${token}`)
  }
}

const merchantPages = [
  'merchant-home',
  'merchant-publish',
  'merchant-wallet',
  'merchant-messages',
  'merchant-order-detail'
]
for (const pageName of merchantPages) {
  const route = `pages/${pageName}/index`
  if (!routes.includes(route)) violations.push(`${route}: new merchant page must be registered`)
  const source = fs.readFileSync(path.join(src, `${route}.tsx`), 'utf8')
  if (!source.includes("useIdentityGuard('merchant')")) {
    violations.push(`${route}: direct route must require approved merchant identity`)
  }
  if (/delivery-static-data|StaticDeliveryOrder|resolveStaticDeliveryOrder|initialLedger|pendingOrders|deliveringOrders|completedOrders/.test(source)) {
    violations.push(`${route}: static merchant fixtures must be removed`)
  }
}

const merchantPaymentGateContracts = [
  [
    'merchant-home',
    /deliveryPaymentEnabled\s*\?\s*settle\(getDeliveryWallet\(\)\)\s*:\s*null/,
    'merchant home must not request the gated wallet endpoint when delivery payment is disabled'
  ],
  [
    'merchant-wallet',
    /if\s*\(\s*!deliveryPaymentEnabled\s*\)/,
    'merchant wallet must render a disabled state without requesting gated payment endpoints'
  ],
  [
    'merchant-publish',
    /if\s*\(\s*!deliveryPaymentEnabled\s*\)/,
    'merchant publish must fail closed before creating an unpaid order when delivery payment is disabled'
  ],
  [
    'merchant-order-detail',
    /deliveryPaymentEnabled\s*\?\s*settle\(getDeliveryOrderFunds\(orderId\)\)\s*:\s*null/,
    'merchant order detail must not request gated funds when delivery payment is disabled'
  ]
]
for (const [pageName, contract, message] of merchantPaymentGateContracts) {
  const source = fs.readFileSync(path.join(src, `pages/${pageName}/index.tsx`), 'utf8')
  if (!source.includes('deliveryPaymentEnabled')) {
    violations.push(`pages/${pageName}/index.tsx: missing deliveryPaymentEnabled gate`)
  } else if (!contract.test(source)) {
    violations.push(`pages/${pageName}/index.tsx: ${message}`)
  }
}

const merchantPublish = fs.readFileSync(path.join(src, 'pages/merchant-publish/index.tsx'), 'utf8')
if (!/const\s+testPublishEnabled\s*=\s*deliveryMockFeeEnabled\s*&&\s*!deliveryPaymentEnabled/.test(merchantPublish)) {
  violations.push('pages/merchant-publish/index.tsx: mock fee publishing must require the explicit mock fee flag and disabled real payment')
}
if (!/if\s*\(\s*testPublishEnabled\s*\)\s*\{\s*void publishWithMockFee\(\)/.test(merchantPublish)) {
  violations.push('pages/merchant-publish/index.tsx: mock fee publishing must stay behind the explicit test gate')
}
if (!merchantPublish.includes('await confirmMockDeliveryFee(order)')) {
  violations.push('pages/merchant-publish/index.tsx: test publishing must confirm the server-created delivery fee before success')
}

const communityClient = fs.readFileSync(path.join(src, 'services/b-community.ts'), 'utf8')
for (const token of [
  '/community/posts/${encodeURIComponent(postId)}/like',
  '/community/posts/${encodeURIComponent(postId)}/favorite',
  '/community/users/${encodeURIComponent(userId)}/follow',
  "method: liked ? 'PUT' : 'DELETE'",
  "method: favorited ? 'PUT' : 'DELETE'",
  "method: following ? 'PUT' : 'DELETE'"
]) {
  if (!communityClient.includes(token)) violations.push(`services/b-community.ts: missing idempotent interaction contract ${token}`)
}

if (fs.existsSync(path.join(src, 'pages/user-profile/data.ts'))) {
  violations.push('pages/user-profile/data.ts: hard-coded public profile fixtures must stay removed')
}

if (violations.length > 0) {
  throw new Error(`miniapp business closure check failed:\n${violations.join('\n')}`)
}

console.log(`miniapp business closure check PASS (${routes.length} registered routes)`)
