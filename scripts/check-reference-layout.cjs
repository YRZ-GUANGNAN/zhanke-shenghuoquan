const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')
const postcss = require('postcss')
const pxtransform = require('postcss-pxtransform')

const root = path.resolve(__dirname, '..')
const src = path.join(root, 'apps/miniapp/src')
const read = (name) => fs.readFileSync(path.join(src, name), 'utf8')
const routes = [...read('app.config.ts').matchAll(/'pages\/([^']+)\/index'/g)].map(match => match[1])
const excluded = ['campus-guide']
const compiled = new Map()
assert.equal(routes.length, 28, '路由变化需要显式复审覆盖表，不得顺带开启历史页面')
for (const route of routes) {
  const source = read(`pages/${route}/index.scss`)
  if (excluded.includes(route)) {
    assert.doesNotMatch(source, /reference-layout|ref\./, '咨询页面保持不变')
    continue
  }
  assert.match(source, /@use ['"]\.\.\/\.\.\/styles\/reference-layout\.scss['"] as ref/, `${route} 必须显式接入规范`)
  assert.match(source, /@include ref\.page-foundation/, `${route} 缺少作用域内控件适配`)
  assert.match(source, /@include ref\.(type|wrap|touch)/, `${route} 不能只接入基础而没有语义适配`)
  const css = sass.compile(path.join(src, `pages/${route}/index.scss`), { silenceDeprecations: ['legacy-js-api', 'global-builtin', 'color-functions', 'import'] }).css
  compiled.set(route, css)
  assert.match(css, /min-height: 44PX/, `${route} 控件保留物理触控尺寸`)
  assert.match(css, /overflow-wrap: anywhere/, `${route} 长文本不能横向溢出`)
}
// Verify final declarations after Taro unit conversion, where a later rpx or
// lowercase px declaration used to shrink these targets on a 320px screen.
const touchCases = {
  'publish-post': ['.publish-post-nav__side', '.publish-post-nav__publish', '.publish-post-topic', '.publish-post-custom-tag__button'],
  'publish-secondhand': ['.publish-secondhand-category', '.publish-secondhand-footer__button'],
  'edit-profile': ['.edit-profile-gender'],
  blacklist: ['.student-blacklist-user__remove'],
  about: ['.about-row'],
  'merchant-messages': ['.merchant-messages-nav__read'],
  'merchant-order-detail': ['.merchant-detail-nav__refresh']
}
function pixels(value, viewport) {
  const unit = value.trim().match(/^([\d.]+)(rpx|px)$/i)
  if (unit) return Number(unit[1]) * (unit[2].toLowerCase() === 'rpx' ? viewport / 750 : 1)
  const maximum = value.match(/^max\((.+)\)$/)
  if (maximum) return Math.max(...maximum[1].split(',').map(part => pixels(part, viewport)))
  throw new Error(`无法验证触控下限: ${value}`)
}
for (const [route, selectors] of Object.entries(touchCases)) {
  const css = postcss([pxtransform({ platform: 'weapp', designWidth: 750 })]).process(compiled.get(route), { from: undefined }).css
  const ast = postcss.parse(css)
  for (const selector of selectors) {
    const declarations = {}
    ast.walkRules(rule => {
      if (rule.selectors.includes(selector)) rule.walkDecls(decl => { declarations[decl.prop] = decl.value })
    })
    for (const width of [320, 390, 430]) {
      for (const property of ['min-width', 'min-height']) {
        assert.ok(declarations[property] && pixels(declarations[property], width) >= 44,
          `${route} ${selector} ${property} 在 ${width}px 屏被旧声明覆盖`)
      }
    }
  }
}
const shared = read('styles/reference-layout.scss')
assert.doesNotMatch(shared, /^\s*(page|:root|\*)\s*\{/m, '不能通过全局选择器修改咨询分支')
const probe = sass.compileString("@use 'reference-layout.scss' as ref; .test-title { @include ref.type(title); } .test-body { @include ref.type(body); } .test-comment { @include ref.type(comment); }", { loadPaths: [path.join(src, 'styles')] }).css
assert.match(probe, /font-size: 36rpx;\s*line-height: 52rpx/)
assert.match(probe, /font-size: 30rpx;\s*line-height: 44rpx/)
assert.match(probe, /font-size: 28rpx;\s*line-height: 42rpx/)
assert.match(read('styles/design-tokens.scss'), /\$brand-primary: #2E7DD4;/, '保持既有蓝色品牌')
assert.ok(/className=['"]post-detail-comment__content['"][^>]*>\{comment\.content\}<\/Text>/.test(read('pages/post-detail/post-comments.tsx')), '评论正文完整显示，不将头像首字截取误判为正文截断')
assert.doesNotMatch(read('pages/delivery-conversation/index.config.ts'), /navigationStyle:\s*['"]custom['"]/, '配送沟通仍由原生导航保留系统区')
assert.doesNotMatch(read('pages/delivery-conversation/index.tsx'), /CapsuleSystemRow/, '原生导航页面不再叠加自定义系统区')
console.log(`参考排版检查通过：${routes.length - excluded.length} 个页面接入，${excluded.length} 个咨询页保留；色调、长文本与物理触控检查通过。`)
