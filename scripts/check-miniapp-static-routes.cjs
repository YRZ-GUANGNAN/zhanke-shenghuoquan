const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps', 'miniapp', 'src')
const configPath = path.join(src, 'app.config.ts')

const expectedRoutes = [
  'pages/home/index',
  'pages/campus-guide/index',
  'pages/search/index',
  'pages/publish-entry/index',
  'pages/publish-post/index',
  'pages/publish-secondhand/index',
  'pages/student-takeout/index',
  'pages/student-secondhand/index',
  'pages/secondhand-detail/index',
  'pages/secondhand-chat/index',
  'pages/profile/index',
  'pages/identity-application/index',
  'pages/user-profile/index',
  'pages/my-content/index',
  'pages/edit-profile/index',
  'pages/settings/index',
  'pages/official-account-bind/index',
  'pages/notifications/index',
  'pages/blacklist/index',
  'pages/feedback/index',
  'pages/my-feedback/index',
  'pages/about/index',
  'pages/legal/index',
  'pages/merchant-home/index',
  'pages/merchant-publish/index',
  'pages/merchant-wallet/index',
  'pages/merchant-messages/index',
  'pages/merchant-order-detail/index',
  'pages/delivery-rider/index',
  'pages/rider-wallet/index',
  'pages/rider-messages/index',
  'pages/delivery-profile/index',
  'pages/delivery-conversation/index',
  'pages/post-detail/index'
]

const expectedLazyRoutes = [
  'pages/search/index',
  'pages/post-detail/index',
  'pages/secondhand-detail/index',
  'pages/user-profile/index',
  'pages/identity-application/index',
  'pages/my-content/index',
  'pages/edit-profile/index',
  'pages/settings/index',
  'pages/official-account-bind/index',
  'pages/notifications/index',
  'pages/my-feedback/index',
  'pages/merchant-home/index',
  'pages/delivery-rider/index',
  'pages/delivery-conversation/index'
]

const expectedExistingSubpackageRoutes = [
  'pages/merchant-publish/index',
  'pages/merchant-order-detail/index',
  'pages/merchant-wallet/index',
  'pages/rider-wallet/index'
]

const expectedSubpackageRoutes = [...new Set([...expectedExistingSubpackageRoutes, ...expectedLazyRoutes])]

const config = fs.readFileSync(configPath, 'utf8')
const mainPagesBlock = config.match(/pages:\s*\[([\s\S]*?)\],\s*subPackages:/)?.[1] || ''
const mainRoutes = [...mainPagesBlock.matchAll(/'([^']+)'/g)].map((match) => match[1])
const subpackageRoutes = [...config.matchAll(/root:\s*'([^']+)'[\s\S]*?pages:\s*\[([\s\S]*?)\]/g)]
  .flatMap(([, packageRoot, packagePages]) => [...packagePages.matchAll(/'([^']+)'/g)]
    .map((match) => `${packageRoot}/${match[1]}`))
const registeredRoutes = [...new Set([...mainRoutes, ...subpackageRoutes])]
const registered = new Set(registeredRoutes)
const expected = new Set(expectedRoutes)
const mainSet = new Set(mainRoutes)
const subpackageSet = new Set(subpackageRoutes)
const forbiddenLegacyRoutes = [
  'pages/chat/index',
  'pages/conversations/index',
  'pages/delivery-merchant/index',
  'pages/delivery-wallet/index',
  'pages/takeout/index',
  'pages/takeout-cart/index',
  'pages/takeout-orders/index',
  'pages/takeout-order-detail/index',
]
const registeredLegacyRoutes = forbiddenLegacyRoutes.filter((route) => registered.has(route))

for (const route of expectedSubpackageRoutes) {
  if (mainSet.has(route)) {
    throw new Error(`subpackage route must not stay in main package: ${route}`)
  }
  if (!subpackageSet.has(route)) {
    throw new Error(`subpackage route must be registered in a subpackage: ${route}`)
  }
}

for (const route of expectedRoutes) {
  if (!expectedSubpackageRoutes.includes(route) && !mainSet.has(route)) {
    throw new Error(`primary route must stay in main package: ${route}`)
  }
}

for (const route of expectedSubpackageRoutes) {
  if (mainSet.has(route) && subpackageSet.has(route)) {
    throw new Error(`route cannot belong to both main package and subpackage: ${route}`)
  }
}

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
sourceFiles.push(path.join(src, 'components', 'delivery-workspace-nav.tsx'))

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
if (/\/pages\/secondhand-chat\/index\?[^`'"\r\n]*(?:conversationId|peerUserId|buyerId|sellerId|peerName|peerAvatar|scene)=/.test(homeSource)) {
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
