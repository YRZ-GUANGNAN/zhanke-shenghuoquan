const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { assertHomeToolsDataFlow, assertSyntheticStaticToolsRejected } = require('./lib/home-tools-contract.cjs')

const root = path.resolve(__dirname, '..')
const read = (value) => fs.readFileSync(path.join(root, value), 'utf8')
const readOptional = (value) => fs.existsSync(path.join(root, value)) ? read(value) : ''
const navigation = read('apps/miniapp/src/services/d-navigation.ts')
const navigationValidationSource = read('apps/miniapp/src/services/d-navigation-validation.js')
const home = read('apps/miniapp/src/pages/home/index.tsx')
const homeStyle = read('apps/miniapp/src/pages/home/index.scss')
const headerLayout = readOptional('apps/miniapp/src/pages/home/header-layout.ts')
const taro = read('apps/miniapp/src/services/taro.ts')
const taroShim = read('apps/miniapp/src/taro-shim.d.ts')
const profile = read('apps/miniapp/src/pages/profile/index.tsx')
const profileStyle = read('apps/miniapp/src/pages/profile/index.scss')
const navigationValidation = require(path.join(root, 'apps/miniapp/src/services/d-navigation-validation.js'))

assert.match(navigationValidationSource, /legacy-missing-appid/)
assert.match(navigation, /validMiniProgramAppID/)
assert.match(navigation, /validMiniProgramPath/)
assert.match(navigation, /filter\(isRealTool\)/)
assert.match(home, /getMenuButtonBoundingClientRect/)
assert.match(home, /getWindowInfo/)
assert.match(home, /--home-header-right/)
assert.match(headerLayout, /calculateHomeHeaderLayout/)
assert.match(taro, /getMenuButtonBoundingClientRect/)
assert.match(taro, /getWindowInfo/)
assert.match(taroShim, /getMenuButtonBoundingClientRect/)
assert.match(homeStyle, /var\(--home-header-top/)
assert.match(homeStyle, /var\(--home-header-height/)
assert.match(homeStyle, /var\(--home-header-right/)
assert.match(homeStyle, /@media \(max-width: 360px\)/)
assert.doesNotMatch(homeStyle, /padding:\s*112rpx 36rpx 0/)
assert.match(home, /type HomeTab = 'messages' \| 'discover' \| 'tools'/)
assert.match(home, /key: 'tools', label: '工具'/)
assert.match(home, /暂无可用工具/)
assert.match(home, /工具加载失败，点击重试/)
assert.match(home, /navigateToMiniProgram/)
assert.match(home, /listNavigationItems/)
assert.match(home, /createSelectorQuery/)
assert.match(home, /select\('#home-discover-sentinel'\)/)
assert.match(home, /if \(!rect\) return/)
assert.doesNotMatch(home, /特价影票|学生兼职|生活服务|校园群聊|校园地图/)
assertHomeToolsDataFlow(home)
assertSyntheticStaticToolsRejected(home)

for (const value of [
  'pages/home/index',
  'pages/store/index?id=1&name=hello+world',
  'pages/store/index?id=1&name=hello%20world',
  'pages/store/index?discount=50%25',
  'pages/home/index?redirect=pages/detail/index',
  'pages/home/index?redirect=pages%2Fdetail%2Findex',
  'pages/home/index?target=pages/detail/index',
  'pages/%E5%95%86%E5%AE%B6/index'
]) {
  assert.equal(navigationValidation.validMiniProgramPath(value), true, `expected valid mini-program path: ${value}`)
}
for (const value of [
  '', '/', '/pages/home/index', '%2Fpages/home/index', '../pages/home/index', 'pages/../home/index',
  'pages//home/index', 'https://evil.example/path', 'custom:pages/home/index',
  'pages\\home\\index', 'pages/home/index#fragment', 'pages/home/%zz',
  'pages/home/index?value=%zz',
  'pages/home/index?redirect=pages%252Fdetail', 'pages/home/index?redirect=https://evil.example',
  'pages/home/index?redirect=custom:pages/detail/index',
  'pages/home/index?redirect=javascript:alert(1)',
  'pages/home/index?redirect=%2Fpages/detail/index',
  'pages/home/index?redirect=%252G',
  'pages/home/index?path=%2Fpages/detail/index',
  'pages/home/index?url=custom:pages/detail/index',
  'pages/home/index?target=pages/../detail/index',
  'pages/home/index?page=pages//detail/index',
  'pages/home/index?route=pages%5Cdetail%5Cindex',
  'pages/home/index\u0000'
]) {
  assert.equal(navigationValidation.validMiniProgramPath(value), false, `expected invalid mini-program path: ${value}`)
}
assert.equal(navigationValidation.validMiniProgramAppID(' wx1234567890ABCDEF '), true)
assert.equal(navigationValidation.validMiniProgramAppID('legacy-missing-appid'), false)
assert.equal(navigationValidation.validMiniProgramAppID('wx0000000000000000'), false)

const sentinelCheck = home.indexOf("select('#home-discover-sentinel')")
const observerCreation = home.indexOf('createIntersectionObserver(page', sentinelCheck)
assert.ok(sentinelCheck >= 0 && observerCreation > sentinelCheck, 'discover observer must be created only after the sentinel exists')
const openToolStart = home.indexOf('  const openTool = async')
const openToolEnd = home.indexOf('\n\n  const renderDiscoverCard', openToolStart)
const openTool = home.slice(openToolStart, openToolEnd)
assert.match(openTool, /await Taro\.showToast[\s\S]*catch/)
assert.match(profile, /student-profile-hero[^\n]*onClick=\{openProfileEditor\}/)
assert.match(profile, /const openProfileEditor = \(\) =>/)
assert.match(profile, /profileNavigationLockRef/)
assert.match(profile, /if \(profileNavigationLockRef\.current\) return/)
assert.match(profile, /profileNavigationLockRef\.current = true[\s\S]*Taro\.navigateTo[\s\S]*\.catch\(\(\) => \{[\s\S]*profileNavigationLockRef\.current = false/)
assert.doesNotMatch(profile, /student-profile-user__edit' onClick=/)
assert.match(profileStyle, /\.student-profile-hero:active/)

console.log('home real tools contract: ok')
