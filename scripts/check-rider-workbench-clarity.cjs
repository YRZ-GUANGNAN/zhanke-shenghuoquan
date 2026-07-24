const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const pageRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'delivery-rider')
const source = fs.readFileSync(path.join(pageRoot, 'index.tsx'), 'utf8')
const style = fs.readFileSync(path.join(pageRoot, 'index.scss'), 'utf8')
const iconComponentPath = path.join(pageRoot, 'rider-workbench-icon.tsx')
const icons = [
  ['action-refresh', 'refreshIcon'],
  ['action-message', 'messageIcon'],
  ['action-phone', 'phoneIcon'],
  ['empty-route', 'emptyIcon'],
]

assert.ok(fs.existsSync(iconComponentPath), 'rider workbench icon component is required')
const iconComponent = fs.readFileSync(iconComponentPath, 'utf8')

for (const [icon, binding] of icons) {
  const file = path.join(pageRoot, 'icons', `${icon}.svg`)
  assert.ok(fs.existsSync(file), `${icon}.svg is required`)
  const svg = fs.readFileSync(file, 'utf8')
  assert.match(svg, /viewBox="0 0 48 48"/, `${icon}.svg must use the shared viewBox`)
  assert.match(iconComponent, new RegExp(`\\b${binding}\\b`), `${icon}.svg must be registered`)
}

assert.match(source, /RiderWorkbenchIcon/)
assert.match(source, /name='refresh'/)
assert.match(source, /name='message'/)
assert.match(source, /name='phone'/)
assert.match(source, /name='empty'/)
assert.match(source, /className='rider-grab-task-summary'/)
assert.match(source, /rider-grab-refresh/)
assert.match(source, /className='rider-empty-state'/)
assert.match(source, /CapsuleSystemRow/)
assert.match(source, /getDeliveryRiderMe/)
assert.match(source, /listClaimableDeliveryOrders/)
assert.match(source, /listAssignedDeliveryOrders/)
assert.doesNotMatch(source, /<Text>骑<\/Text>/)
assert.doesNotMatch(source, /rider-grab-hero__orb/)
assert.doesNotMatch(source, /rider-grab-list-head/)

assert.match(style, /\.rider-grab-task-summary\s*\{/)
assert.match(style, /\.rider-grab-refresh\s*\{/)
assert.match(style, /\.rider-grab-tabs\s*\{[\s\S]*grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\)/)
assert.match(style, /\.rider-empty-state\s*\{/)
assert.match(style, /@keyframes\s+rider-refresh-spin/)
assert.doesNotMatch(
  style.match(/\.rider-grab-content\s*\{[^}]*\}/)?.[0] || '',
  /\bpadding\s*:/,
  'ScrollView must not use unsupported padding',
)

console.log('rider workbench clarity contract: ok')
