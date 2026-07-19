const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const layoutModule = path.join(
  __dirname,
  '..',
  'apps',
  'miniapp',
  'src',
  'utils',
  'capsule-layout.js',
)
const { calculateCapsuleLayout } = require(layoutModule)

const cases = [
  {
    name: '320px Android',
    windowInfo: { windowWidth: 320, statusBarHeight: 24 },
    menu: { top: 28, right: 312, bottom: 60, left: 225, width: 87, height: 32 },
    expectedSystemRowHeight: 76,
  },
  {
    name: '375px iPhone',
    windowInfo: { windowWidth: 375, statusBarHeight: 47 },
    menu: { top: 51, right: 367, bottom: 83, left: 280, width: 87, height: 32 },
    expectedSystemRowHeight: 99,
  },
  {
    name: '430px iPhone Max',
    windowInfo: { windowWidth: 430, statusBarHeight: 54 },
    menu: { top: 58, right: 422, bottom: 90, left: 335, width: 87, height: 32 },
    expectedSystemRowHeight: 106,
  },
]

for (const value of cases) {
  const layout = calculateCapsuleLayout(value.windowInfo, value.menu)
  assert.equal(layout.systemRowHeight, value.expectedSystemRowHeight, value.name)
  assert.equal(layout.businessRowTop, layout.systemRowHeight, value.name)
  assert.equal(layout.businessRowHeight, 48, value.name)
  assert.equal(layout.totalHeaderHeight, layout.systemRowHeight + 48, value.name)
  assert.ok(layout.businessRowTop > value.menu.bottom, value.name)
}

const fallback = calculateCapsuleLayout({ windowWidth: 375, statusBarHeight: 24 })
assert.deepEqual(fallback, {
  systemRowHeight: 76,
  businessRowTop: 76,
  businessRowHeight: 48,
  totalHeaderHeight: 124,
})

const totalFallback = calculateCapsuleLayout({ windowWidth: 375, statusBarHeight: 0 })
assert.equal(totalFallback.systemRowHeight, 72)

const componentPath = path.join(
  __dirname,
  '..',
  'apps',
  'miniapp',
  'src',
  'components',
  'capsule-page-header.tsx',
)
const stylePath = componentPath.replace(/\.tsx$/, '.scss')
assert.ok(fs.existsSync(componentPath), 'CapsulePageHeader component is required')
assert.ok(fs.existsSync(stylePath), 'CapsulePageHeader styles are required')

const componentSource = fs.readFileSync(componentPath, 'utf8')
const styleSource = fs.readFileSync(stylePath, 'utf8')
assert.match(componentSource, /export function CapsuleSystemRow/)
assert.match(componentSource, /export default function CapsulePageHeader/)
assert.match(styleSource, /var\(--capsule-system-row-height\)/)
assert.match(styleSource, /var\(--capsule-business-row-height\)/)
assert.match(styleSource, /min-width:\s*44px/)
assert.match(styleSource, /min-height:\s*44px/)
assert.match(styleSource, /capsule-page-header__slot\s*>\s*\*/)
assert.match(styleSource, /capsule-page-header__slot\s*>\s*\*[\s\S]*min-width:\s*100%/)
assert.match(styleSource, /capsule-page-header__slot\s*>\s*\*[\s\S]*min-height:\s*44px/)
assert.match(styleSource, /pointer-events:\s*none/)

const miniappPagesRoot = path.join(__dirname, '..', 'apps', 'miniapp', 'src', 'pages')
const standardPages = [
  'about',
  'blacklist',
  'feedback',
  'edit-profile',
  'legal',
  'settings',
  'my-content',
  'identity-application',
]

for (const pageName of standardPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
}

const publishingAndDetailPages = [
  'publish-entry',
  'publish-post',
  'publish-secondhand',
  'post-detail',
  'secondhand-detail',
  'secondhand-chat',
]

for (const pageName of publishingAndDetailPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  const style = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.scss'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
  assert.doesNotMatch(style, /--capsule-safe-right/, `${pageName} must not squeeze business navigation beside the capsule`)
}

const mainPages = ['home', 'search', 'student-secondhand', 'student-takeout']

for (const pageName of mainPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  const style = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.scss'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
  assert.doesNotMatch(style, /--capsule-safe-right/, `${pageName} must not squeeze business navigation beside the capsule`)
}

const heroPages = ['profile', 'user-profile', 'merchant-home', 'merchant-wallet', 'rider-wallet']

for (const pageName of heroPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsuleSystemRow/, `${pageName} must use CapsuleSystemRow`)
}

const merchantPages = ['merchant-messages', 'merchant-order-detail', 'merchant-publish']

for (const pageName of merchantPages) {
  const source = fs.readFileSync(path.join(miniappPagesRoot, pageName, 'index.tsx'), 'utf8')
  assert.match(source, /CapsulePageHeader/, `${pageName} must use CapsulePageHeader`)
}

const customNavigationPages = fs.readdirSync(miniappPagesRoot, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => entry.name)
  .filter((pageName) => {
    const configPath = path.join(miniappPagesRoot, pageName, 'index.config.ts')
    return fs.existsSync(configPath) && /navigationStyle:\s*'custom'/.test(fs.readFileSync(configPath, 'utf8'))
  })

assert.equal(customNavigationPages.length, 26, 'custom navigation page inventory changed')

for (const pageName of customNavigationPages) {
  const sourcePath = path.join(miniappPagesRoot, pageName, 'index.tsx')
  assert.ok(fs.existsSync(sourcePath), `${pageName} custom navigation page source is missing`)
  const source = fs.readFileSync(sourcePath, 'utf8')
  assert.match(source, /CapsulePageHeader|CapsuleSystemRow/, `${pageName} custom navigation is not capsule-safe`)
}

const allCustomNavigationSource = customNavigationPages
  .flatMap((pageName) => ['index.tsx', 'index.scss'].map((fileName) => path.join(miniappPagesRoot, pageName, fileName)))
  .filter(fs.existsSync)
  .map((fileName) => fs.readFileSync(fileName, 'utf8'))
  .join('\n')

assert.doesNotMatch(allCustomNavigationSource, /--capsule-safe-right|--home-header-right/)

assert.equal(fs.existsSync(path.join(miniappPagesRoot, 'home', 'header-layout.ts')), false)
const studentTakeoutSource = fs.readFileSync(path.join(miniappPagesRoot, 'student-takeout', 'index.tsx'), 'utf8')
const studentTakeoutStyle = fs.readFileSync(path.join(miniappPagesRoot, 'student-takeout', 'index.scss'), 'utf8')
assert.match(studentTakeoutSource, /style=\{getCapsuleSafeStyle\(\)\}/)
assert.match(studentTakeoutStyle, /calc\(var\(--capsule-total-header-height\)/)
assert.doesNotMatch(studentTakeoutStyle, /204rpx/)

const postDetailSource = fs.readFileSync(path.join(miniappPagesRoot, 'post-detail', 'index.tsx'), 'utf8')
const postDetailStyle = fs.readFileSync(path.join(miniappPagesRoot, 'post-detail', 'index.scss'), 'utf8')
assert.match(postDetailSource, /style=\{getCapsuleSafeStyle\(\)\}/)
assert.match(postDetailStyle, /padding-top:\s*var\(--capsule-total-header-height\)/)
assert.match(postDetailStyle, /\.capsule-page-header\.post-detail-nav[\s\S]*position:\s*fixed/)
assert.doesNotMatch(postDetailStyle, /184px/)

console.log('miniapp capsule geometry: ok')
