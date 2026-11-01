const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

const sourceRoot = path.resolve(__dirname, '../apps/miniapp/src')
const read = file => fs.readFileSync(path.join(sourceRoot, file), 'utf8')

function checkConfirmedSurface() {
  const sass = require('sass')
  const postcss = require('postcss')
  const css = postcss.parse(sass.compile(path.join(sourceRoot, 'components/surface-enhancements.scss')).css)
  let fallback = false, glass = false, icon = false, activeIcon = false
  css.walkRules(rule => {
    const declarations = Object.fromEntries((rule.nodes || []).filter(node => node.type === 'decl').map(node => [node.prop, node.value]))
    if (rule.selector.split(',').map(selector => selector.trim()).join(',') === 'page .student-bottom-nav,page .delivery-workspace-nav') {
      fallback ||= rule.parent.type === 'root' && declarations.background.includes('--bg-surface')
      glass ||= rule.parent.name === 'supports' && declarations.background === 'transparent' && declarations['backdrop-filter'] === 'blur(24px)'
    }
    if (rule.selector.includes('page .student-bottom-nav__icon-asset') && !rule.selector.includes('--active')) icon ||= declarations.opacity === '0.2'
    if (rule.selector.includes('page .student-bottom-nav__item--active') && rule.selector.includes('page .delivery-workspace-nav__item--active')) activeIcon ||= declarations.opacity === '1'
  })
  assert.ok(fallback, '不支持backdrop-filter时保留可读实色底')
  assert.ok(glass, '真实SCSS必须包含已确认的透明底+上游24px模糊，不靠预览覆盖')
  assert.ok(icon && activeIcon, '上游图标透明度 .2 / 1 同时应用于学生与工作区')
  assert.match(read('components/student-bottom-nav.tsx'), /hoverClass='student-bottom-nav__item--pressed'/)
  for (const [name, radius] of [['student-bottom-nav', 24], ['delivery-workspace-nav', 18]]) {
    const scss = read(`components/${name}.scss`)
    assert.match(scss, new RegExp(`border-radius:\\s*${radius}PX`))
    assert.match(scss, new RegExp(`${name}--hidden::before`), '收起把手必须在产品样式中')
    assert.doesNotMatch(scss, /content:\s*['"]上滑/, '不恢复用户已删的提示文字')
  }
}

async function run() {
  checkConfirmedSurface()
  const dom = new JSDOM('<body></body>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const PageContext = React.createContext('')
  const lifecycle = new Map()
  const roots = new Set()
  const modules = new Map()
  const timers = new Map()
  let route = '', now = 0, timerId = 0, forwarded = 0
  let campusLoads = 0
  const campusGuides = [
    { id: 'guide-library', category: '学习', title: '图书馆指南', summary: '图书借阅说明', verifiedAt: '2026-10-29' },
    { id: 'guide-campus', category: '生活', title: '校园生活指南', summary: '校园服务说明', verifiedAt: '2026-10-29' },
  ]
  const clock = {
    setTimeout: (callback, delay) => { const id = ++timerId; timers.set(id, { callback, at: now + delay }); return id },
    clearTimeout: id => timers.delete(id),
  }
  const tick = async delay => React.act(async () => {
    now += delay
    for (const [id, timer] of Array.from(timers)) {
      if (timer.at <= now) { timers.delete(id); timer.callback() }
    }
  })
  const useLifecycle = (kind, callback) => {
    const page = React.useContext(PageContext)
    const latest = React.useRef(callback)
    latest.current = callback
    React.useLayoutEffect(() => {
      const key = `${page}:${kind}`
      const callbacks = lifecycle.get(key) || new Set()
      const listener = (...args) => latest.current(...args)
      callbacks.add(listener); lifecycle.set(key, callbacks)
      return () => callbacks.delete(listener)
    }, [])
  }
  const taro = {
    getCurrentInstance: () => ({ router: route ? { $taroPath: route } : undefined }),
    usePageScroll: callback => useLifecycle('scroll', callback),
    useDidHide: callback => useLifecycle('hide', callback),
    useDidShow: callback => useLifecycle('show', callback),
  }
  const component = name => props => React.createElement('div', {
    'data-kind': name,
    ref: node => { if (node) node.taroProps = props },
  }, props.children)
  function load(relative) {
    if (modules.has(relative)) return modules.get(relative)
    const file = path.join(sourceRoot, relative)
    const loaded = { exports: {} }
    const requireSource = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/taro') return { ...taro, default: taro }
      if (id === '@tarojs/components') return Object.fromEntries(['ScrollView', 'Slot', 'View', 'Text', 'Button'].map(name => [name, component(name)]))
      if (id.endsWith('status-glyph')) return { StatusGlyph: () => null }
      if (id.endsWith('bottom-nav-motion')) return load('utils/bottom-nav-motion.ts')
      if (id.endsWith('taro-ux-loading')) return load('components/ui/taro-ux-loading.tsx')
      if (id.endsWith('taro-ux-blur')) return load('utils/taro-ux-blur.ts')
      if (id.endsWith('capsule-page-header') || id.endsWith('app-icon')) return { default: () => null }
      if (id.endsWith('components/ui')) return { AppState: () => null }
      if (id.endsWith('student-bottom-nav')) return { default: () => React.createElement(Nav, { enabled: true }) }
      if (id.endsWith('services/campus-info')) return { listCampusGuides: async () => { campusLoads++; return campusGuides } }
      if (id.endsWith('hooks/use-page-share')) return { usePageShare: () => {} }
      if (id.endsWith('services/taro')) return { default: { navigateTo: async () => { throw Error('底栏测试不执行业务导航') } } }
      if (id.endsWith('.scss')) return {}
      throw new Error(`未隔离的依赖: ${id}`)
    }
    const compiled = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 },
      fileName: file,
    }).outputText
    new Function('require', 'module', 'exports', 'setTimeout', 'clearTimeout', compiled)(requireSource, loaded, loaded.exports, clock.setTimeout, clock.clearTimeout)
    modules.set(relative, loaded.exports)
    return loaded.exports
  }
  const motion = load('utils/bottom-nav-motion.ts')
  const Refresh = load('components/refresh-scroll-view.tsx').default
  const CampusInfoHome = load('components/campus-info-home/index.tsx').default
  function Nav({ enabled }) { return React.createElement('output', null, String(motion.useBottomNavVisibility(enabled))) }
  const act = callback => React.act(async () => { callback() })
  async function page(id, initial = {}) {
    const container = document.createElement('main')
    document.body.appendChild(container)
    const app = createRoot(container)
    roots.add(app)
    let props = { navEnabled: true, enabled: true, sourceKey: 'first', ...initial }
    const render = () => act(() => app.render(React.createElement(PageContext.Provider, { value: id },
      React.createElement(Nav, { enabled: props.navEnabled }),
      React.createElement(Refresh, { enabled: props.enabled, refreshKey: props.sourceKey, onRefresh: async () => true, onScroll: () => forwarded++ }),
    )))
    route = id
    await render()
    const scroll = () => container.querySelector('[data-kind=ScrollView]').taroProps.onScroll
    const fire = (kind, value) => act(() => { for (const callback of Array.from(lifecycle.get(`${id}:${kind}`) || [])) callback(value) })
    return {
      id,
      hidden: () => container.querySelector('output').textContent === 'true',
      scroll,
      emit: top => act(() => scroll()({ detail: { scrollTop: top } })),
      native: top => fire('scroll', { scrollTop: top }),
      hide: () => fire('hide'),
      show: () => fire('show'),
      update: async values => { props = { ...props, ...values }; await render() },
      unmount: async () => { await act(() => app.unmount()); roots.delete(app); container.remove() },
    }
  }
  try {
    const first = await page('home@1', { enabled: false })
    await first.emit(0); await first.emit(100)
    assert.equal(first.hidden(), false, '离屏频道 enabled=false 不得收起当前底栏')
    assert.equal(forwarded, 2, '底栏桥不能吞掉页面自己的 onScroll')

    await first.update({ enabled: true })
    await first.emit(0); await first.emit(71)
    assert.equal(first.hidden(), false, '未到72阈值不收起')
    await first.emit(72)
    assert.equal(first.hidden(), true)
    await tick(599)
    assert.equal(first.hidden(), true, '停止后599ms仍保持收起')
    await tick(1)
    assert.equal(first.hidden(), false, '停止600ms恢复')
    await first.emit(100); await tick(400); await first.emit(130); await tick(400)
    assert.equal(first.hidden(), true, '继续滚动重新计算600ms')
    await first.emit(120)
    assert.equal(first.hidden(), false, '反向滚动立即恢复')
    await first.emit(140); await first.emit(8)
    assert.equal(first.hidden(), false, '回顶部立即恢复')
    const validTimerCount = timers.size
    await first.emit(NaN); await first.emit(Infinity)
    assert.equal(timers.size, validTimerCount, '异常偏移不能安排恢复计时器')
    console.log('PASS 底栏真实hook：阈值、方向、600ms去抖、顶部及异常偏移')

    await first.emit(150)
    const oldChannel = first.scroll()
    await first.update({ sourceKey: 'second' })
    await tick(600)
    assert.equal(first.hidden(), false)
    await act(() => { oldChannel({ detail: { scrollTop: 180 } }) })
    assert.equal(first.hidden(), false, '频道切换后的旧回调不能再隐藏底栏')
    assert.equal(timers.size, 0, '旧频道回调不能重置当前页计时器')
    await first.emit(700)
    assert.equal(first.hidden(), false, '新的滚动源首帧建立独立基线，不继承前一频道')
    await first.emit(710)
    assert.equal(first.hidden(), true)
    const oldLifetime = first.scroll()
    await first.hide()
    assert.equal(first.hidden(), false, '页面隐藏时恢复底栏')
    assert.equal(timers.size, 0, '页面隐藏必须清理计时器')
    await act(() => oldLifetime({ detail: { scrollTop: 740 } }))
    assert.equal(first.hidden(), false, '隐藏后迟到原生事件无效')
    await first.show()
    await act(() => oldLifetime({ detail: { scrollTop: 760 } }))
    assert.equal(first.hidden(), false, '重新显示不能复活旧生命周期回调')
    await first.emit(800); await first.emit(810)
    assert.equal(first.hidden(), true)
    await first.update({ navEnabled: false })
    assert.equal(first.hidden(), false)
    assert.equal(timers.size, 0)
    await first.update({ navEnabled: true })
    console.log('PASS 频道来源、旧回调、hide/show与禁用生命周期隔离')

    await first.hide()
    const second = await page('home@2')
    await second.emit(0); await second.emit(100)
    assert.equal(second.hidden(), true)
    await tick(600)
    await first.emit(820)
    assert.equal(second.hidden(), false, '旧页面事件不能驱动新页面底栏')
    route = first.id
    await first.show()
    await second.emit(140)
    assert.equal(first.hidden(), false, '非当前路由事件不能广播给当前页面')
    await first.native(100); await first.native(110)
    assert.equal(first.hidden(), true, '普通page滚动仍驱动底栏')
    await first.native(90)
    assert.equal(first.hidden(), false)
    const detached = first.scroll()
    const detachedShow = Array.from(lifecycle.get(`${first.id}:show`) || [])
    await first.unmount()
    route = second.id
    await second.show()
    await act(() => { detachedShow.forEach(callback => callback()); detached({ detail: { scrollTop: 900 } }) })
    assert.equal(second.hidden(), false, '卸载后旧show和scroll都不得复活')
    assert.equal(timers.size, 0)
    await second.unmount()
    const missing = await page('')
    await missing.emit(0); await missing.emit(100); await missing.native(100)
    assert.equal(missing.hidden(), false, '无路由时保持可用底栏，不跨页fallback广播')
    await missing.unmount()
    console.log('PASS 跨页面、原生page滚动、卸载、缺少router安全降级')

    // Mount the actual page component: hook-only tests cannot detect a missing
    // native ScrollView onScroll connection in an otherwise correct navbar.
    route = 'campus-info@1'
    const campusContainer = document.createElement('main')
    document.body.appendChild(campusContainer)
    const campusApp = createRoot(campusContainer)
    roots.add(campusApp)
    await act(() => campusApp.render(React.createElement(PageContext.Provider, { value: route }, React.createElement(CampusInfoHome))))
    const guideScroll = () => campusContainer.querySelector('[data-kind=ScrollView]').taroProps.onScroll
    const guideHidden = () => campusContainer.querySelector('output').textContent === 'true'
    const guideEmit = top => act(() => guideScroll()({ detail: { scrollTop: top } }))
    assert.equal(typeof guideScroll(), 'function', '真实CampusInfoHome必须连接ScrollView onScroll，不能仅测试独立hook')
    assert.equal(campusLoads, 1)
    assert.ok(campusContainer.textContent.includes('图书馆指南') && campusContainer.textContent.includes('校园生活指南'))
    await guideEmit(0); await guideEmit(71)
    assert.equal(guideHidden(), false, '校园指南未到72阈值保持底栏')
    await guideEmit(72)
    assert.equal(guideHidden(), true, '校园指南原生ScrollView事件实际收起底栏')
    await tick(599); assert.equal(guideHidden(), true)
    await tick(1); assert.equal(guideHidden(), false, '校园指南停止600ms恢复')
    const oldCategoryScroll = guideScroll()
    const categoryButton = Array.from(campusContainer.querySelectorAll('[data-kind=Button]')).find(node => node.textContent === '学习')
    assert.ok(categoryButton, '实际分类按钮必须存在')
    await act(() => categoryButton.taroProps.onClick())
    assert.equal(campusLoads, 1, '分类筛选不应触发新请求')
    assert.ok(campusContainer.textContent.includes('图书馆指南'))
    assert.ok(!campusContainer.textContent.includes('校园生活指南'), '保留原分类内容筛选')
    await act(() => oldCategoryScroll({ detail: { scrollTop: 300 } }))
    assert.equal(guideHidden(), false, '分类切换后迟到的旧ScrollView回调不能收起底栏')
    assert.equal(timers.size, 0, '旧分类事件不能安排恢复计时器')
    await guideEmit(500)
    assert.equal(guideHidden(), false, '新分类首帧建立独立基线')
    await guideEmit(510)
    assert.equal(guideHidden(), true, '新分类后续滚动正常驱动底栏')
    await tick(600)
    assert.equal(guideHidden(), false)
    await act(() => campusApp.unmount()); roots.delete(campusApp); campusContainer.remove()
    assert.equal(timers.size, 0)
    console.log('PASS 真实CampusInfoHome：原生滚动72阈值、600ms恢复、分类筛选与迟到回调隔离')

    for (const file of ['components/student-bottom-nav.scss', 'components/delivery-workspace-nav.scss']) {
      const scss = read(file)
      assert.match(scss, /border-radius:\s*(?:24|18)PX/)
      assert.match(scss, /translateY\(calc\(100% - 12PX\)\)/)
      assert.match(scss, /prefers-reduced-motion/)
    }
    console.log('bottom nav motion: PASS (真实React hook/生命周期行为；圆角与减弱动效样式契约)')
  } finally {
    for (const root of roots) await React.act(async () => root.unmount())
    dom.window.close()
    for (const [key, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor)
      else delete globalThis[key]
    }
  }
}

run().catch(error => { console.error(error); process.exitCode = 1 })
