const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')

const root = path.resolve(__dirname, '../apps/miniapp/src')
const css = sass.compileString('@use "profile-cover-surface" as cover; .cover { @include cover.surface(true); }', {
  loadPaths: [path.join(root, 'styles')], logger: sass.Logger.silent,
}).css
assert.doesNotMatch(css, /rgba\(var\(--cl\)/, '十六进制颜色 token 不能当作 rgba 通道')
assert.match(css, /rgba\(var\(--profile-cover-rgb, 255, 255, 255\), 0\)/)
assert.match(css, /rgba\(var\(--profile-cover-rgb, 255, 255, 255\), 0\.3\) 38%/)
assert.match(css, /rgba\(var\(--profile-cover-rgb, 255, 255, 255\), 0\.84\) 78%/)
assert.match(css, /var\(--cl\) 94%/)
assert.match(css, /bottom: -1PX/)
const theme = fs.readFileSync(path.join(root, 'styles/system-dark-theme.scss'), 'utf8')
assert.match(theme, /--profile-cover-rgb:\s*255,\s*255,\s*255;/)
assert.match(theme, /--profile-cover-rgb:\s*24,\s*33,\s*44\s*!important;/)
for (const file of ['pages/profile/index.scss', 'pages/user-profile/index.scss', 'components/profile-cover-editor.scss']) {
  assert.match(fs.readFileSync(path.join(root, file), 'utf8'), /profile-cover-surface/)
}
console.log('profile cover theme: PASS (light/dark RGB, original fade stops and seam guard)')
