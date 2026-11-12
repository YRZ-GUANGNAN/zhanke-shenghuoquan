const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const postcss = require('postcss')

// Inspect the real WeChat output, not Sass-only output or the browser bridge.
// Run after `npm run build:miniapp`; this script never builds or rewrites dist.
const dist = path.resolve(__dirname, '../apps/miniapp/dist')
const styles = []
const visited = new Set()
const failures = []
const read = relative => fs.readFileSync(path.join(dist, relative), 'utf8')
function loadStyles(relative) {
  const file = path.resolve(dist, relative)
  assert.ok(file.startsWith(dist + path.sep), 'WXSS import must remain inside dist')
  if (visited.has(file)) return
  visited.add(file)
  const css = postcss.parse(fs.readFileSync(file, 'utf8'), { from: file })
  css.walkAtRules('import', rule => {
    const imported = rule.params.match(/^["']([^"']+)["']/)?.[1]
    assert.ok(imported, `unsupported WXSS import: ${rule.params}`)
    loadStyles(path.relative(dist, path.resolve(path.dirname(file), imported)))
  })
  styles.push(css)
}
function matchingRules(selector, condition = () => true) {
  const matches = []
  for (const css of styles) css.walkRules(rule => {
    if (rule.selector.split(',').some(value => value.trim() === selector) && condition(rule)) matches.push(rule)
  })
  return matches
}
function declarations(rule) {
  return Object.fromEntries(rule.nodes.filter(node => node.type === 'decl').map(node => [node.prop, node.value]))
}
function inCondition(rule, name, pattern) {
  for (let parent = rule.parent; parent; parent = parent.parent) {
    if (parent.type === 'atrule' && parent.name === name && pattern.test(parent.params)) return true
  }
  return false
}
function check(name, run) {
  try { run(); console.log(`PASS ${name}`) } catch (error) {
    failures.push(name)
    console.error(`FAIL ${name}: ${error.message}`)
  }
}
function hasBothFilters(rule, expected) {
  const values = declarations(rule)
  return values['backdrop-filter'] === expected && values['-webkit-backdrop-filter'] === expected
}

try {
  loadStyles('app.wxss')
  loadStyles('pages/home/index.wxss')
  const supported = rule => inCondition(rule, 'supports', /-webkit-backdrop-filter/)
  const reduced = rule => inCondition(rule, 'media', /prefers-reduced-motion\s*:\s*reduce/)
  check('bottom nav keeps real glass with a translucent no-blur fallback', () => {
    for (const selector of ['page .student-bottom-nav', 'page .delivery-workspace-nav']) {
      assert.ok(matchingRules(selector, rule => rule.parent.type === 'root').some(rule => /var\(--glass-nav/.test(declarations(rule).background || '')), `${selector}: translucent fallback missing`)
      assert.ok(matchingRules(selector, rule => rule.parent.type === 'root').some(rule => declarations(rule)['backdrop-filter'] === 'blur(24rpx)'), `${selector}: blur enhancement missing`)
      assert.ok(matchingRules(selector, supported).some(rule => /var\(--glass-nav/.test(declarations(rule).background || '') && hasBothFilters(rule, 'blur(24rpx)')), `${selector}: supported glass rule incomplete`)
    }
  })
  check('sheet and native refresh glass retain both filter declarations', () => {
    for (const [selector, blur] of [
      ['page .app-action-sheet-layer .app-action-sheet', 'blur(18PX)'],
      ['page .app-dialog-layer .app-dialog', 'blur(18PX)'],
      ['page .refresh-feedback--pulling', 'blur(10PX)'],
      ['page .refresh-feedback--ready', 'blur(10PX)'],
      ['page .refresh-feedback--refreshing', 'blur(10PX)'],
    ]) assert.ok(matchingRules(selector, supported).some(rule => hasBothFilters(rule, blur)), `${selector}: WebKit filter missing from built WXSS`)
    assert.ok(matchingRules('.app-action-sheet-layer__mask').some(rule => hasBothFilters(rule, 'blur(4rpx)')), 'sheet mask must keep its existing blur on prefixed WebKit')
  })
  check('three-dot refresh is viewport-contained and reduced-motion-safe', () => {
    const overlay = matchingRules('.taro-ux-loading--overlay', rule => !reduced(rule))
    assert.ok(overlay.some(rule => {
      const values = declarations(rule)
      return values.position === 'absolute' && values['pointer-events'] === 'none' && hasBothFilters(rule, 'var(--blur)')
    }), 'refresh overlay must retain prefixed blur without intercepting gestures')
    assert.ok(matchingRules('.taro-ux-loading--overlay', reduced).some(rule => hasBothFilters(rule, 'none')), 'reduced motion must disable both prefixed and unprefixed blur')
    assert.ok(matchingRules('.taro-ux-loading__dot', reduced).some(rule => declarations(rule).animation === 'none'))
    const frames = []
    styles.forEach(css => css.walkAtRules('keyframes', rule => {
      if (rule.params === 'taro-ux-loading-wave') frames.push(rule)
    }))
    assert.ok(frames.some(rule => rule.nodes.filter(node => node.type === 'rule').map(node => [node.selector, declarations(node).height]).join('|') === '0%,16rpx|50%,36rpx|100%,16rpx'), 'original three-dot keyframes missing')
    const javascript = read('common.js') + read('pages/home/index.js')
    assert.equal((javascript.match(/className:["']taro-ux-loading__dot["']/g) || []).length, 3, 'built real component must render all three dots')
    assert.ok(matchingRules('.taro-ux-refresh').some(rule => declarations(rule).overflow === 'hidden'))
    assert.ok(matchingRules('.taro-ux-refresh__scroll').some(rule => declarations(rule).height === '100%'))
    assert.ok(matchingRules('.taro-ux-refresh__content').some(rule => /var\(--refresh-content-bottom/.test(declarations(rule)['padding-bottom'] || '')))
    assert.ok(matchingRules('.student-home-content').some(rule => !declarations(rule)['padding-bottom'] && /136rpx/.test(declarations(rule)['--refresh-content-bottom'] || '')), 'home clearance must scroll with content rather than shorten the viewport')
  })
  check('real WXSS retains fixed rounded nav and sheet exit states', () => {
    for (const [selector, radius] of [['.student-bottom-nav', '24PX'], ['.delivery-workspace-nav', '18PX']]) {
      assert.ok(matchingRules(selector).some(rule => declarations(rule)['border-radius'] === radius))
      assert.equal(matchingRules(`${selector}--hidden`).length, 0, `${selector} must not ship a scroll-collapse state`)
      assert.equal(matchingRules(`${selector}--hidden::before`).length, 0, `${selector} must not ship a collapsed handle`)
    }
    assert.ok(matchingRules('.app-action-sheet-layer--exit .app-action-sheet').some(rule => declarations(rule).transform === 'translateY(100%)'))
  })
  check('native dark theme and native gesture event bindings ship together', () => {
    const config = JSON.parse(read('app.json'))
    assert.equal(config.darkmode, true)
    const theme = JSON.parse(read(config.themeLocation))
    assert.equal(theme.light.navigationText, 'black')
    assert.equal(theme.dark.navigationText, 'white')
    assert.notEqual(theme.light.backgroundSurface, theme.dark.backgroundSurface)
    assert.ok(matchingRules('page', rule => inCondition(rule, 'media', /prefers-color-scheme\s*:\s*dark/)).length, 'dark CSS variables missing')
    const templates = read('base.wxml')
    for (const event of ['bindscroll', 'bindrefresherpulling', 'bindrefresherrefresh', 'bindrefresherrestore', 'bindrefresherabort', 'bindtouchstart', 'catchtouchmove', 'bindtouchend', 'bindtouchcancel']) {
      assert.ok(templates.includes(`${event}="eh"`), `${event} must remain wired to Taro's native event dispatcher`)
    }
  })
} catch (error) {
  failures.push('required build output')
  console.error(`FAIL required build output: ${error.message}`)
}
if (failures.length) {
  console.error(`miniapp UX artifact: FAIL (${failures.join('; ')})`)
  process.exitCode = 1
} else {
  console.log('miniapp UX artifact: PASS (real Taro WXSS/JS/WXML/theme; not a device visual acceptance)')
}
