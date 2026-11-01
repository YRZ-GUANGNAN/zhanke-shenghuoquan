const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')
const postcss = require('postcss')

// CSS geometry contract, not a screenshot/real-device acceptance. A zero-height
// native Swiper can only grow vertically when its immediate flex parent is a
// column; the default row leaves it at height:0 and clips all order content.
const root = path.resolve(__dirname, '..')
const artifact = process.argv.includes('--artifact')
const sheets = []
const visited = new Set()
function readArtifact(relative) {
  const file = path.resolve(root, 'apps/miniapp/dist', relative)
  if (visited.has(file)) return
  visited.add(file)
  const css = postcss.parse(fs.readFileSync(file, 'utf8'), { from: file })
  css.walkAtRules('import', rule => {
    const imported = rule.params.match(/^["']([^"']+)["']/)?.[1]
    assert.ok(imported, `unsupported import: ${rule.params}`)
    readArtifact(path.relative(path.join(root, 'apps/miniapp/dist'), path.resolve(path.dirname(file), imported)))
  })
  sheets.push(css)
}
function compiled(relative) {
  sheets.push(postcss.parse(sass.compile(path.join(root, 'apps/miniapp/src', relative), { logger: sass.Logger.silent }).css))
}
function values(selector) {
  const result = {}
  for (const css of sheets) css.walkRules(rule => {
    if (rule.parent.type !== 'root' || !rule.selectors.includes(selector)) return
    rule.walkDecls(declaration => { result[declaration.prop] = declaration.value })
  })
  return result
}

if (artifact) {
  readArtifact('app.wxss')
  readArtifact('pages/merchant-home/index.wxss')
  readArtifact('pages/delivery-rider/index.wxss')
} else {
  compiled('components/channel-pager.scss')
  compiled('components/refresh-scroll-view.scss')
  compiled('pages/merchant-home/index.scss')
  compiled('pages/delivery-rider/index.scss')
}

const stage = values('.delivery-stage-pager')
const pager = values('.channel-pager')
assert.equal(stage.display, 'flex')
assert.equal(stage['flex-direction'], 'column', '原生Swiper的height:0必须在column主轴增长；默认row会让订单、刷新工具栏和空态全部不可见')
assert.equal(stage.flex, '1 1 auto')
assert.equal(stage.height, '0')
assert.equal(pager.flex, '1 1 auto')
assert.equal(pager.height, '0')
assert.equal(values('.channel-pager__item').height, '100%')
assert.equal(values('.taro-ux-refresh__scroll').height, '100%')
for (const [page, content] of [['merchant-home-page', 'merchant-home-orders'], ['rider-grab-page', 'rider-grab-content']]) {
  assert.equal(values(`.${page}`)['flex-direction'], 'column')
  assert.equal(values(`.${page} .channel-pager__item > .${content}`).height, '100%', `${page}: refresh viewport must fill its SwiperItem`)
}
console.log(`delivery pager layout: PASS (${artifact ? '真实WXSS产物' : '编译SCSS'}纵向flex高度链；不替代微信画面验收)`)
