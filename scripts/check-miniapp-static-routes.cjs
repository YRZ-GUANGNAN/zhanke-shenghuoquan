const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps', 'miniapp', 'src')
const configPath = path.join(src, 'app.config.ts')

const expectedRoutes = [
  'pages/home/index',
  'pages/search/index',
  'pages/publish-entry/index',
  'pages/publish-post/index',
  'pages/publish-secondhand/index',
  'pages/student-takeout/index',
  'pages/student-secondhand/index',
  'pages/secondhand-detail/index',
  'pages/secondhand-chat/index',
  'pages/profile/index',
  'pages/user-profile/index',
  'pages/my-content/index',
  'pages/edit-profile/index',
  'pages/settings/index',
  'pages/blacklist/index',
  'pages/feedback/index',
  'pages/about/index',
  'pages/legal/index',
  'pages/delivery-merchant/index',
  'pages/delivery-wallet/index',
  'pages/delivery-rider/index',
  'pages/delivery-conversation/index',
  'pages/post-detail/index'
]

const config = fs.readFileSync(configPath, 'utf8')
const registeredRoutes = [...config.matchAll(/'((?:pages\/)[^']+\/index)'/g)].map((match) => match[1])
const registered = new Set(registeredRoutes)
const expected = new Set(expectedRoutes)
const forbiddenLegacyRoutes = [
  'pages/chat/index',
  'pages/conversations/index',
  'pages/takeout/index',
  'pages/takeout-cart/index',
  'pages/takeout-orders/index',
  'pages/takeout-order-detail/index',
]
const registeredLegacyRoutes = forbiddenLegacyRoutes.filter((route) => registered.has(route))

if (registeredLegacyRoutes.length > 0) {
  throw new Error(`legacy chat routes must stay unregistered: ${registeredLegacyRoutes.join(', ')}`)
}

const missingFromConfig = expectedRoutes.filter((route) => !registered.has(route))
const unexpectedInConfig = registeredRoutes.filter((route) => !expected.has(route))

if (missingFromConfig.length > 0) {
  throw new Error(`new-shell routes missing from app.config.ts: ${missingFromConfig.join(', ')}`)
}

if (unexpectedInConfig.length > 0) {
  throw new Error(`non-whitelisted routes registered in app.config.ts: ${unexpectedInConfig.join(', ')}`)
}

for (const route of registeredRoutes) {
  const pageFile = path.join(src, `${route}.tsx`)
  if (!fs.existsSync(pageFile)) {
    throw new Error(`registered page missing: ${route}`)
  }
}

const sourceFiles = registeredRoutes.map((route) => path.join(src, `${route}.tsx`))
sourceFiles.push(path.join(src, 'components', 'student-bottom-nav.tsx'))

const routePattern = /\/pages\/[a-z0-9-]+\/index/g
const brokenTargets = []

for (const filePath of sourceFiles) {
  const source = fs.readFileSync(filePath, 'utf8')
  const targets = new Set(source.match(routePattern) || [])

  for (const target of targets) {
    const normalized = target.slice(1)
    if (!registered.has(normalized)) {
      brokenTargets.push(`${path.relative(root, filePath)} -> ${target}`)
    }
  }
}

if (brokenTargets.length > 0) {
  throw new Error(`registered new-shell source points to an unregistered route:\n${brokenTargets.join('\n')}`)
}

const homeSource = fs.readFileSync(path.join(src, 'pages', 'home', 'index.tsx'), 'utf8')
if (/const\s+conversations\s*[:=]/.test(homeSource)) {
  throw new Error('home messages must not use a hard-coded conversations array')
}
if (/conversationId|peerUserId|buyerId|sellerId|peerName|peerAvatar|scene=/.test(homeSource)) {
  throw new Error('home conversation navigation must carry relationId only')
}
const safeRelationTarget = '/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(item.context.relationId))}'
if (!homeSource.includes(safeRelationTarget)) {
  throw new Error('home conversation navigation must use the authorized relationId')
}
if (!/RuntimeSessionGate\s+preserveAuthenticatedChildrenDuringRetry/.test(homeSource)) {
  throw new Error('home must preserve an authenticated mount during one-shot session recovery')
}

console.log(`miniapp static route check PASS (${registeredRoutes.length} routes)`)
