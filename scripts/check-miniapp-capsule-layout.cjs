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

console.log('miniapp capsule geometry: ok')
