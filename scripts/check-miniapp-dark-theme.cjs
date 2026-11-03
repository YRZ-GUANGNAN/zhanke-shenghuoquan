const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { execFileSync } = require('node:child_process')
const sass = require('sass')
const postcss = require('postcss')
const { readMiniappRoutes } = require('./miniapp-routes.cjs')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps/miniapp/src')
const read = file => fs.readFileSync(path.join(src, file), 'utf8')
const appConfig = read('app.config.ts')
const pages = readMiniappRoutes(appConfig)
const themes = JSON.parse(read('theme.json'))
assert.match(appConfig, /darkmode:\s*true/)
assert.match(appConfig, /themeLocation:\s*'theme.json'/)
assert.deepEqual(Object.keys(themes.light).sort(), Object.keys(themes.dark).sort())
assert.equal(themes.light.navigationBackground, '#f7fbff')
assert.equal(themes.light.navigationText, 'black')
assert.equal(themes.light.navigationOnCover, 'white')
assert.equal(themes.dark.navigationText, 'white')
assert.match(fs.readFileSync(path.join(root, 'apps/miniapp/config/index.ts'), 'utf8'), /from:\s*'src\/theme.json',\s*to:\s*'dist\/theme.json'/)
assert.match(read('pages/rider-wallet/index.config.ts'), /backgroundColor:\s*'@backgroundDefault'/)
assert.match(read('pages/rider-wallet/index.config.ts'), /navigationBarTextStyle:\s*'@navigationText'/)
for (const source of [appConfig, ...pages.map(page => read(`${page}.config.ts`))]) {
  for (const match of source.matchAll(/['"]@([a-zA-Z]+)['"]/g)) {
    assert.ok(themes.light[match[1]] && themes.dark[match[1]], `unresolved native theme key: ${match[1]}`)
  }
}

const generated = read('styles/system-dark-colors.generated.scss')
assert.ok(generated.includes(`${pages.length} registered pages;`))
for (const route of ['pages/merchant-wallet/index', 'pages/rider-wallet/index']) {
  assert.ok(pages.includes(route), `missing wallet route: ${route}`)
  assert.ok(generated.includes(`/* ${route}.scss */`), `missing wallet dark styles: ${route}`)
}
const ast = postcss.parse(sass.compileString(generated, { logger: sass.Logger.silent }).css)
const allowed = /^(?:color|background(?:-color|-image)?|border(?:-(?:top|right|bottom|left))?(?:-color)?|outline(?:-color)?|box-shadow|text-shadow|fill|stroke|caret-color|--guide-[\w-]+)$/
const rules = new Map()
let count = 0
ast.walkRules(rule => {
  let parent = rule.parent
  let scoped = false
  while (parent) {
    if (parent.type === 'atrule' && parent.name === 'media' && parent.params === '(prefers-color-scheme: dark)') scoped = true
    assert.ok(parent.type !== 'atrule' || !parent.name.includes('keyframes'), 'dark colors must not replace animations')
    parent = parent.parent
  }
  assert.ok(scoped, 'all generated overrides must be dark-only')
  assert.ok(rule.selector.split(',').every(selector => /^page(?:\b|[.:[#])/.test(selector.trim())), `unscoped selector: ${rule.selector}`)
  for (const decl of rule.nodes.filter(node => node.type === 'decl')) {
    assert.match(decl.prop, allowed, `unexpected layout/animation property: ${decl.prop}`)
    assert.doesNotMatch(decl.value, /url\(|invert\(/i, 'user media and SVG assets must not be rewritten')
    count++
  }
  rules.set(rule.selector, Object.fromEntries(rule.nodes.filter(node => node.type === 'decl').map(decl => [decl.prop, decl.value])))
})
// Runtime token variables carry the repeated light/dark palette once. The
// generated sheet only needs rules for genuinely hard-coded semantic colors.
assert.ok(count >= 100, 'compact dark sheet must retain semantic color coverage')
assert.match(read('styles/design-tokens.scss'), /\$brand-primary-static:/)
assert.match(read('styles/design-tokens.scss'), /\$brand-primary:\s*var\(--c0\)/)
assert.match(read('styles/system-dark-theme.scss'), /--c0:\s*#2E7DD4/)
assert.match(read('styles/system-dark-theme.scss'), /--c0:\s*#2E7DD4\s*!important/)
assert.match(read('components/app-icon.tsx'), /app-icon--\$\{name\}/)
assert.match(read('components/channel-pager.scss'), /\.delivery-stage-pager/)
for (const [selector, declarations] of rules) {
  if (/(?:^|\s)\.(?:student-bottom-nav|delivery-workspace-nav|app-dialog|app-action-sheet|refresh-feedback)$/.test(selector)) {
    assert.equal(declarations.background, undefined, `glass background belongs to semantic surface theme: ${selector}`)
  }
}
const themeAst = postcss.parse(sass.compile(path.join(src, 'styles/system-dark-theme.scss'), { logger: sass.Logger.silent }).css)
let darkVariables = 0
themeAst.walkDecls(decl => {
  if (!decl.prop.startsWith('--')) return
  let parent = decl.parent
  let dark = false
  while (parent) {
    if (parent.type === 'atrule' && parent.name === 'media' && parent.params === '(prefers-color-scheme: dark)') dark = true
    parent = parent.parent
  }
  if (dark) {
    if (/^--(?:c[a-z0-9]+|g\d+|s\d+|p\d+|brand-|text-|bg-|border-|radius-|glass-)/.test(decl.prop)) {
      darkVariables++
      assert.ok(decl.important, 'night variables must survive later page light-token imports')
    }
  }
})
assert.ok(darkVariables >= 30, 'dark theme must define the compact variable table')
execFileSync(process.execPath, [path.join(root, 'scripts/generate-miniapp-dark-theme.cjs'), '--check'], { stdio: 'inherit' })
console.log(`miniapp system dark theme: PASS (${pages.length} native configs, ${count} color declarations; light structure/media/variants preserved)`)
