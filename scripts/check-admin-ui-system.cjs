const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const src = path.join(root, 'apps', 'admin', 'src')
const componentRoot = path.join(src, 'components')

for (const name of ['AdminButton', 'AdminField', 'AdminSelect', 'AdminIcon', 'AdminState', 'AdminBadge', 'AdminNotice', 'AdminDialog', 'AdminDialogProvider']) {
  assert.ok(fs.existsSync(path.join(componentRoot, `${name}.tsx`)), `${name}.tsx is required`)
}

const sources = fs.readdirSync(src, { withFileTypes: true })
  .filter((entry) => entry.isFile() && entry.name.endsWith('.tsx'))
  .map((entry) => fs.readFileSync(path.join(src, entry.name), 'utf8'))
  .join('\n')

assert.doesNotMatch(sources, /window\.(confirm|prompt|alert)\s*\(/)
assert.doesNotMatch(fs.readFileSync(path.join(src, 'App.tsx'), 'utf8'), /icon:\s*'[A-Z]'/)

const styles = fs.readFileSync(path.join(src, 'styles.css'), 'utf8')
for (const token of ['--color-primary', '--radius-control', '--shadow-dialog', '--duration-fast']) {
  assert.match(styles, new RegExp(token))
}
console.log('admin ui system: ok')
