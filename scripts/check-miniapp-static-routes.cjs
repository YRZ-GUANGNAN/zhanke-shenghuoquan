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
  'pages/legal/index',
  'pages/merchant-home/index',
  'pages/merchant-publish/index',
  'pages/merchant-wallet/index',
  'pages/merchant-messages/index',
  'pages/merchant-order-detail/index',
  'pages/delivery-rider/index',
  'pages/rider-wallet/index',
  'pages/delivery-conversation/index',
  'pages/post-detail/index'
]

const config = fs.readFileSync(configPath, 'utf8')
const registeredRoutes = [...config.matchAll(/'((?:pages\/)[^']+\/index)'/g)].map((match) => match[1])
const registered = new Set(registeredRoutes)
const expected = new Set(expectedRoutes)

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

console.log(`miniapp static route check PASS (${registeredRoutes.length} routes)`)
