const assert = require('node:assert/strict')
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

console.log('miniapp capsule geometry: ok')
