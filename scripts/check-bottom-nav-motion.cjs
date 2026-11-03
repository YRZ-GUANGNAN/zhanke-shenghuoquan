const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')

const sourceRoot = path.resolve(__dirname, '../apps/miniapp/src')
const read = file => fs.readFileSync(path.join(sourceRoot, file), 'utf8')

function checkTransparentFixedSurface() {
  const sass = require('sass')
  const postcss = require('postcss')
  const css = postcss.parse(sass.compile(path.join(sourceRoot, 'components/surface-enhancements.scss')).css)
  let fallback = false
  let glass = false
  let icon = false
  let activeIcon = false
  css.walkRules(rule => {
    const declarations = Object.fromEntries((rule.nodes || [])
      .filter(node => node.type === 'decl')
      .map(node => [node.prop, node.value]))
    if (rule.selector.split(',').map(selector => selector.trim()).join(',') === 'page .student-bottom-nav,page .delivery-workspace-nav') {
      fallback ||= rule.parent.type === 'root' && declarations.background.includes('--bg-surface')
      glass ||= rule.parent.name === 'supports'
        && declarations.background === 'transparent'
        && declarations['backdrop-filter'] === 'blur(24px)'
    }
    if (rule.selector.includes('page .student-bottom-nav__icon-asset') && !rule.selector.includes('--active')) {
      icon ||= declarations.opacity === '0.2'
    }
    if (rule.selector.includes('page .student-bottom-nav__item--active') && rule.selector.includes('page .delivery-workspace-nav__item--active')) {
      activeIcon ||= declarations.opacity === '1'
    }
  })
  assert.ok(fallback, '不支持 backdrop-filter 时必须保留可读实色底')
  assert.ok(glass, '学生、商家与骑手底栏必须保留透明底和 24px 毛玻璃')
  assert.ok(icon && activeIcon, '底栏图标透明度与激活态必须保持现有契约')

  for (const [component, style, radius] of [
    ['components/student-bottom-nav.tsx', 'components/student-bottom-nav.scss', 24],
    ['components/delivery-workspace-nav.tsx', 'components/delivery-workspace-nav.scss', 18],
  ]) {
    const source = read(component)
    const scss = read(style)
    assert.doesNotMatch(source, /useBottomNavVisibility|--hidden/, `${component} 不得再根据滚动收起`)
    assert.doesNotMatch(scss, /--hidden|translateY\(calc\(100% - 12PX\)\)/, `${style} 不得保留收起位移`)
    assert.match(scss, new RegExp(`border-radius:\\s*${radius}PX`), `${style} 保留现有圆角`)
  }
}

function checkScrollBridgeIsStateFree() {
  const source = read('utils/bottom-nav-motion.ts')
  assert.match(source, /export function useBottomNavScroll/)
  assert.doesNotMatch(source, /useBottomNavVisibility|setTimeout|useState|usePageScroll|listeners/,
    '滚动桥不得再保存、广播或定时恢复底栏状态')

  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    reportDiagnostics: true,
  })
  assert.equal((compiled.diagnostics || []).length, 0, '底栏兼容桥必须可以独立编译')
  const loaded = { exports: {} }
  new Function('module', 'exports', compiled.outputText)(loaded, loaded.exports)
  const onScroll = loaded.exports.useBottomNavScroll(true, 'feed')
  assert.equal(typeof onScroll, 'function')
  for (const top of [0, 72, 1000, NaN, Infinity]) assert.equal(onScroll(top), undefined)

  const refresh = read('components/refresh-scroll-view.tsx')
  assert.match(refresh, /navScroll\(Number\(event\.detail\?\.scrollTop\)\)/,
    '兼容桥保持原调用点，页面自己的滚动回调不受影响')
}

checkTransparentFixedSurface()
checkScrollBridgeIsStateFree()
console.log('bottom nav fixed glass: PASS（固定显示、透明毛玻璃、圆角、图标与滚动回调兼容）')
