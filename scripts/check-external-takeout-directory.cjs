const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')
const takeoutPage = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.tsx')
const navigationClient = read('apps', 'miniapp', 'src', 'services', 'd-navigation.ts')
const adminClient = read('apps', 'admin', 'src', 'services', 'f-admin.ts')
const adminPanel = read('apps', 'admin', 'src', 'ExternalTakeoutMerchants.tsx')
const appSource = read('apps', 'admin', 'src', 'App.tsx')

function assertIncludes(source, value) {
  if (!source.includes(value)) throw new Error(`external takeout contract is missing: ${value}`)
}

function assertExcludes(source, value) {
  if (source.includes(value)) throw new Error(`external takeout contract contains forbidden value: ${value}`)
}

for (const required of [
  'listExternalTakeoutMerchants',
  'navigateToMiniProgram',
  "merchant.operatingStatus === 'resting'",
  '/pages/search/index?scope=takeout',
  '商家加载失败，点击重试',
]) assertIncludes(takeoutPage, required)

for (const forbidden of [
  'const merchants', '茶知味', '华莱士', '正新鸡排', '一点点', '塔斯汀', '蜜雪冰城',
  'g-takeout', 'stores/cart', 'pages/takeout/', 'getCurrentInstance',
]) assertExcludes(takeoutPage, forbidden)

for (const required of [
  'external_takeout_merchants', 'listNavigationItems',
  'category.systemKey!==externalTakeoutSystemKey', 'value.systemKey===externalTakeoutSystemKey',
]) assertIncludes(navigationClient, required)
for (const forbidden of ['g-takeout', 'delivery', 'payment', 'a-user']) assertExcludes(navigationClient, forbidden)

for (const required of [
  '/admin/navigation/external-takeout-merchants', '/operating-status', '/visibility', '/category-visibility',
]) assertIncludes(adminClient, required)
for (const forbidden of ['TakeoutConsole', 'takeout-admin']) assertExcludes(adminPanel, forbidden)
for (const required of [
  'setExternalTakeoutCategoryVisibility', 'setExternalTakeoutOperatingStatus',
  'setExternalTakeoutVisibility', 'directory.categoryEnabled', 'await load()',
]) assertIncludes(adminPanel, required)

const guardIndex = takeoutPage.indexOf("merchant.operatingStatus === 'resting'")
const jumpIndex = takeoutPage.indexOf('navigateToMiniProgram')
if (guardIndex < 0 || jumpIndex < 0 || guardIndex > jumpIndex) {
  throw new Error('resting guard must run before navigateToMiniProgram')
}

for (const required of ['<ExternalTakeoutMerchants', 'legacyTakeoutEnabled', '<TakeoutConsole']) {
  assertIncludes(appSource, required)
}

console.log('external takeout directory check PASS')
