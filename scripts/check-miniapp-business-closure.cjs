const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps', 'miniapp', 'src')
const config = fs.readFileSync(path.join(src, 'app.config.ts'), 'utf8')
const routes = [...config.matchAll(/'((?:pages\/)[^']+\/index)'/g)].map((match) => match[1])

const forbiddenCopy = /静态(?:预览|占位|示意)|后续接入|下一阶段接入|TODO|128\.50|86\.50/
const violations = []
for (const route of routes) {
  const file = path.join(src, `${route}.tsx`)
  const source = fs.readFileSync(file, 'utf8')
  if (forbiddenCopy.test(source)) violations.push(`${route}: contains placeholder or fake-value copy`)
}

const mustContain = [
  ['pages/user-profile/index.tsx', 'getPublicCommunityProfile'],
  ['pages/user-profile/index.tsx', 'getPublicUserProducts'],
  ['pages/my-content/index.tsx', 'getMyFavoritePosts'],
  ['pages/edit-profile/index.tsx', 'updateProfile'],
  ['pages/delivery-rider/index.tsx', 'listClaimableDeliveryOrders'],
  ['pages/delivery-conversation/index.tsx', 'loadDeliveryTencentMessages'],
  ['pages/delivery-merchant/index.tsx', "useIdentityGuard('merchant')"]
]
for (const [relative, token] of mustContain) {
  const source = fs.readFileSync(path.join(src, relative), 'utf8')
  if (!source.includes(token)) violations.push(`${relative}: missing ${token}`)
}

const home = fs.readFileSync(path.join(src, 'pages/home/index.tsx'), 'utf8')
if (/静态占位|showTodoToast|特价影票|学生兼职|校园群聊|校园地图/.test(home)) {
  violations.push('pages/home/index.tsx: static fake tools must not be registered')
}
if (/HomeTab\s*=.*tools/.test(home) && !home.includes('listNavigationItems')) {
  violations.push('pages/home/index.tsx: tools tab must consume the real navigation service')
}

const identityGuard = fs.readFileSync(path.join(src, 'hooks/use-identity-guard.ts'), 'utf8')
if (!identityGuard.includes("merchant: '/pages/delivery-merchant/index'")) {
  violations.push('merchant identity must route to the real delivery merchant workbench')
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
