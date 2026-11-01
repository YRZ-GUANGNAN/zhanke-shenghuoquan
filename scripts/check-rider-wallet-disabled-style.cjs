const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')
const postcss = require('postcss')

const root = path.resolve(__dirname, '..')
const source = path.join(root, 'apps/miniapp/src')
const artifact = process.argv.includes('--dist')
const compile = file => sass.compile(path.join(source, file), { logger: sass.Logger.silent }).css
const walletCss = artifact
  ? fs.readFileSync(path.join(root, 'apps/miniapp/dist/pages/rider-wallet/index.wxss'), 'utf8')
  : compile('pages/rider-wallet/index.scss')
const rules = postcss.parse(walletCss)
const selectors = [
  '.rider-wallet-page .rider-wallet-withdraw.app-button--disabled',
  '.rider-wallet-page .rider-wallet-withdraw[disabled]',
]
const declarations = rule => Object.fromEntries(rule.nodes.filter(node => node.type === 'decl').map(node => [node.prop, node.value]))
const allowed = new Set(['opacity', 'color', 'background', 'border-color'])
const scoped = []
rules.walkRules(rule => {
  if (rule.selector.split(',').some(value => selectors.includes(value.trim()))) scoped.push(rule)
})
assert.ok(scoped.length, '骑手提现禁用态缺少独立的可读性兼容规则')
for (const selector of selectors) {
  assert.ok(scoped.some(rule => rule.selector.split(',').map(value => value.trim()).includes(selector)), `${selector} 必须覆盖组件类及微信原生disabled态`)
}
for (const rule of scoped) {
  assert.equal(rule.parent.type, 'root', '禁用兼容规则不能只在媒体条件下生效')
  assert.ok(rule.selector.split(',').every(value => selectors.includes(value.trim())), '禁用兼容不得扩散到其它按钮或页面')
  const values = declarations(rule)
  for (const property of Object.keys(values)) assert.ok(allowed.has(property), `不得修改布局/行为：${property}`)
  assert.equal(values.opacity, '1', '禁用文字不能与蓝卡一起降低透明度')
  assert.equal(values.color, 'var(--cd)', '使用语义正文token兼容浅深色，不采用原生低透明文字')
  assert.equal(values.background, 'var(--cm)', '使用语义次级表面token')
  assert.equal(values['border-color'], 'var(--cn)', '保留低强调语义边界')
}

// Both selectors have three class/attribute components, outranking the native
// button[disabled]:not([type]) rule (two class/attribute components) without !important.
assert.ok(selectors.every(selector => (selector.match(/\.[\w-]+|\[[^\]]+\]/g) || []).length === 3))

const themes = { light: {}, dark: {} }
postcss.parse(compile('styles/system-dark-theme.scss')).walkDecls(decl => {
  if (!decl.prop.startsWith('--') || decl.parent.selector !== 'page') return
  let parent = decl.parent
  let dark = false
  while (parent) {
    if (parent.type === 'atrule' && parent.name === 'media' && /prefers-color-scheme:\s*dark/.test(parent.params)) dark = true
    parent = parent.parent
  }
  themes[dark ? 'dark' : 'light'][decl.prop] = decl.value
})
function rgb(value) {
  assert.match(value, /^#[0-9a-f]{6}$/i, '对比度计算只接受不透明六位token颜色')
  return value.slice(1).match(/../g).map(part => parseInt(part, 16))
}
function luminance(values) {
  return values.map(value => {
    const channel = value / 255
    return channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4
  }).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0)
}
for (const mode of ['light', 'dark']) {
  const tokens = { ...themes.light, ...(mode === 'dark' ? themes.dark : {}) }
  const foreground = luminance(rgb(tokens['--cd']))
  const background = luminance(rgb(tokens['--cm']))
  const contrast = (Math.max(foreground, background) + .05) / (Math.min(foreground, background) + .05)
  assert.ok(contrast >= 4.5, `${mode} 禁用提现文字对比度不足：${contrast.toFixed(2)}:1`)
  console.log(`PASS ${mode} 骑手提现禁用态对比度 ${contrast.toFixed(2)}:1`)
}
console.log(`rider wallet disabled style: PASS (${artifact ? '真实WXSS' : '编译SCSS'}；仅禁用提现按钮，不改布局或资金行为)`)
