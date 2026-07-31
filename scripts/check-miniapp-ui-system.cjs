const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const uiRoot = path.join(root, 'apps', 'miniapp', 'src', 'components', 'ui')

for (const name of [
  'app-button',
  'app-field',
  'app-dialog',
  'app-action-sheet',
  'app-state',
  'app-notice',
  'app-skeleton',
]) {
  assert.ok(fs.existsSync(path.join(uiRoot, `${name}.tsx`)), `${name}.tsx is required`)
  assert.ok(fs.existsSync(path.join(uiRoot, `${name}.scss`)), `${name}.scss is required`)
}

const tokens = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'styles', 'design-tokens.scss'),
  'utf8',
)
for (const token of ['$radius-control', '$radius-card', '$duration-fast', '$z-dialog']) {
  assert.ok(tokens.includes(token), `${token} is required`)
}

const pagesRoot = path.join(root, 'apps', 'miniapp', 'src', 'pages')
const sources = fs.readdirSync(pagesRoot, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .flatMap((entry) => ['index.tsx', 'index.scss'].map((file) => path.join(pagesRoot, entry.name, file)))
  .filter(fs.existsSync)
  .map((file) => ({ file, source: fs.readFileSync(file, 'utf8') }))

for (const { file, source } of sources) {
  assert.doesNotMatch(source, /[a-z0-9-]+-modal__mask/, `${file} must use AppDialog or AppActionSheet`)
  if (!file.endsWith(path.join('settings', 'index.tsx'))) {
    assert.doesNotMatch(source, /Taro\.showModal/, `${file} must not use platform modal`)
  }
}

console.log('miniapp ui system: ok')
