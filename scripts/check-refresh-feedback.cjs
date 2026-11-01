const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')
const sass = require('sass')
const postcss = require('postcss')
const root = path.resolve(__dirname, '..')
const defer = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }

async function run() {
  const dom = new JSDOM('<main></main>')
  const saved = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  let hide, show, props, count = 0
  const tasks = []
  const component = name => p => React.createElement('div', { 'data-kind': name, className: p.className,
    ref: node => { if (node) node.taroProps = p } }, p.children)
  function load(relative, extra = {}) {
    const file = path.join(root, 'apps/miniapp/src', relative)
    const loaded = new Module(file, module)
    loaded.require = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return Object.fromEntries(['ScrollView', 'Slot', 'View', 'Text'].map(name => [name, component(name)]))
      if (id === '@tarojs/taro') return { useDidHide: cb => { hide = cb }, useDidShow: cb => { show = cb } }
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('status-glyph')) return { StatusGlyph: component('StatusGlyph') }
      if (id.endsWith('app-icon')) return { default: component('AppIcon') }
      if (id === './ui') return { AppState: p => React.createElement('div', { 'data-state': p.kind }, p.title), AppSkeleton: component('Skeleton') }
      if (extra[id]) return extra[id]
      if (id.startsWith('.')) {
        const target = path.resolve(path.dirname(file), id)
        const resolved = ['.tsx', '.ts'].map(ext => target + ext).find(candidate => fs.existsSync(candidate))
        if (resolved) return load(path.relative(path.join(root, 'apps/miniapp/src'), resolved), extra)
      }
      throw new Error('Unexpected dependency ' + id)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file, compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, file)
    return loaded.exports
  }
  const Refresh = load('components/refresh-scroll-view.tsx').default
  const app = createRoot(document.querySelector('main'))
  let forwardedScroll
  props = { enabled: true, busy: false, refreshKey: 'user-a', className: 'test-layout', style: { height: '100%' },
    id: 'native-list', scrollY: true, scrollIntoView: 'last-item', onScroll: value => { forwardedScroll = value },
    children: [React.createElement('article', { id: 'first-item', key: 'first' }, 'first-content'), React.createElement('article', { id: 'last-item', key: 'last' }, 'original-list-content')],
    onRefresh: () => { count++; const task = defer(); tasks.push(task); return task.promise } }
  const render = () => React.act(async () => app.render(React.createElement(Refresh, props)))
  const event = cb => React.act(async () => { cb() })
  const scroll = () => document.querySelector('[data-kind=ScrollView]').taroProps
  const label = () => document.querySelector('[data-kind=Slot]').textContent
  const overlay = () => document.querySelector('.taro-ux-loading--overlay')
  const wait = ms => React.act(() => new Promise(resolve => setTimeout(resolve, ms)))
  try {
    await render()
    assert.equal(document.querySelector('[data-kind=Slot]').taroProps.name, 'refresher')
    assert.equal(scroll().refresherThreshold, 60)
    await event(() => scroll().onRefresherPulling({ detail: { dy: 39 } }))
    assert.equal(label(), '下拉刷新')
    assert.equal(overlay(), null)
    await event(() => scroll().onRefresherPulling({ detail: { dy: 70 } }))
    assert.equal(label(), '松开刷新')
    assert.ok(overlay(), 'the approved upstream blur must be mounted in the real refresh component')
    assert.equal(overlay().taroProps.style['--blur'], 'blur(10px)')
    assert.ok(overlay().classList.contains('taro-ux-loading--hide-dots'), 'pull below upstream END keeps dots hidden')
    assert.equal(overlay().querySelectorAll('.taro-ux-loading__dot').length, 3)
    const frame = document.querySelector('.taro-ux-refresh')
    assert.equal(overlay().parentElement, frame, 'overlay belongs to the viewport, not the long scrolling content')
    assert.equal(frame.querySelector('[data-kind=ScrollView]').parentElement, frame)
    assert.ok(frame.classList.contains('test-layout'), 'caller flex/height class stays on the same parent-level root')
    assert.deepEqual(frame.taroProps.style, props.style)
    assert.equal(scroll().id, 'native-list')
    assert.equal(scroll().scrollY, true)
    assert.equal(scroll().scrollIntoView, 'last-item')
    assert.ok(document.querySelector('[data-kind=ScrollView] #last-item'), 'original children remain inside native ScrollView')
    const content = document.querySelector('.taro-ux-refresh__content')
    assert.ok(content, 'bottom navigation clearance must belong to scrollable content, not the fixed viewport')
    assert.equal(content.parentElement, frame.querySelector('[data-kind=ScrollView]'))
    assert.equal(content.previousElementSibling, document.querySelector('[data-kind=Slot]'), 'native refresher slot stays before the original content')
    assert.deepEqual(Array.from(content.children, child => child.id), ['first-item', 'last-item'], 'business content keeps its original order')
    const scrollEvent = { detail: { scrollTop: 110 } }
    await event(() => scroll().onScroll(scrollEvent))
    assert.equal(forwardedScroll, scrollEvent, 'original onScroll receives the unchanged native event')
    await event(() => scroll().onRefresherPulling({ detail: { dy: 101 } }))
    assert.equal(overlay().classList.contains('taro-ux-loading--hide-dots'), false)
    assert.equal(overlay().taroProps.style['--blur'], 'blur(20px)')
    await event(() => scroll().onRefresherPulling({ detail: { dy: Infinity } }))
    assert.equal(overlay(), null, 'invalid native distance must never create an unbounded or NaN blur')
    await event(() => scroll().onRefresherPulling({ detail: { dy: 70 } }))
    await event(() => { scroll().onRefresherRefresh(); scroll().onRefresherRefresh() })
    assert.equal(count, 1)
    assert.equal(scroll().refresherTriggered, true)
    assert.equal(overlay().taroProps.style['--blur'], 'blur(6px)', 'request uses upstream startLoading blur=60')
    assert.equal(overlay().classList.contains('taro-ux-loading--hide-dots'), false)
    props = { ...props, busy: true }; await render()
    assert.equal(scroll().refresherEnabled, true, 'in-flight pull must keep native slot enabled')
    await event(() => scroll().onRefresherAbort())
    assert.equal(label(), '正在更新', 'abort cannot report a pending request as cancelled or successful')
    assert.ok(overlay(), 'native abort cannot erase an in-flight request decoration')
    await event(() => tasks[0].resolve(true))
    assert.equal(label(), '已更新')
    await wait(320)
    assert.equal(overlay(), null)
    assert.equal(label(), '已更新')
    await wait(150)
    assert.equal(scroll().refresherTriggered, false)
    await event(() => scroll().onRefresherRefresh())
    assert.equal(count, 1, 'external paging prevents overlapping pull requests')
    props = { ...props, busy: false }; await render()
    await event(() => scroll().onRefresherRefresh())
    await event(() => tasks[1].reject(new Error('offline')))
    assert.equal(label(), '更新失败，请重试')
    await wait(320)
    assert.equal(overlay(), null, 'failed request also clears blur after upstream 300ms')
    assert.equal(label(), '更新失败，请重试', 'error explanation stays after decorative blur clears')
    await wait(500)
    await event(() => scroll().onRefresherRefresh())
    const previousOwner = scroll()
    props = { ...props, refreshKey: 'user-b' }; await render()
    await event(() => tasks[2].resolve(true))
    assert.equal(label(), '下拉刷新', 'old owner cannot update refresh feedback')
    assert.equal(overlay(), null, 'old owner cannot leave a blur layer')
    await event(() => {
      previousOwner.onRefresherPulling({ detail: { dy: 70 } })
      previousOwner.onRefresherRefresh()
    })
    assert.equal(count, 3, 'late native events from the previous owner must not refresh the new owner')
    assert.equal(label(), '下拉刷新')
    await event(() => scroll().onRefresherRefresh())
    await event(() => hide())
    await event(() => scroll().onRefresherPulling({ detail: { dy: 70 } }))
    assert.equal(label(), '下拉刷新', 'hidden native pulling events must not leave a ready label on return')
    assert.equal(overlay(), null)
    await event(() => scroll().onRefresherRefresh())
    assert.equal(count, 4, 'hidden native events cannot start another request')
    await event(() => tasks[3].resolve(true))
    assert.equal(scroll().refresherTriggered, false)
    await event(() => show())
    await event(() => scroll().onRefresherRefresh())
    props = { ...props, enabled: false }; await render()
    await event(() => tasks[4].resolve(true))
    assert.equal(label(), '下拉刷新', 'hidden channel ignores completion')
    assert.equal(overlay(), null)
    props = { ...props, enabled: true }; await render()
    await event(() => scroll().onRefresherPulling({ detail: { dy: 80 } }))
    assert.equal(label(), '松开刷新')
    await event(() => scroll().onRefresherAbort())
    assert.equal(label(), '下拉刷新', 'cancelled pull restores the native feedback immediately')
    assert.equal(overlay(), null)
    await event(() => scroll().onRefresherRefresh())
    const detached = scroll()
    const detachedShow = show
    await event(() => app.unmount())
    await event(() => tasks[5].resolve(true))
    await event(() => {
      detachedShow()
      detached.onRefresherPulling({ detail: { dy: 70 } })
      detached.onRefresherRestore()
      detached.onRefresherAbort()
      detached.onRefresherRefresh()
    })
    assert.equal(count, 6, 'unmounted component cannot restart requests even if an old show callback arrives')

    const blur = load('utils/taro-ux-blur.ts')
    assert.equal(blur.UX_BLUR_START, 40)
    assert.equal(blur.UX_BLUR_END, 100)
    for (const invalid of [NaN, Infinity, -Infinity, -1, 39]) assert.equal(blur.taroUxBlurStyle(invalid), undefined)
    assert.deepEqual(blur.taroUxBlurStyle(40), { '--blur': 'blur(0px)' })
    assert.deepEqual(blur.taroUxBlurStyle(70), { '--blur': 'blur(10px)' })
    const source = fs.readFileSync(path.join(root, 'apps/miniapp/src/components/refresh-scroll-view.tsx'), 'utf8')
    assert.doesNotMatch(source, /onTouch(?:Start|Move|End)=/, 'native scrolling keeps exclusive ownership of touch direction')
    const refreshStyles = fs.readFileSync(path.join(root, 'apps/miniapp/src/components/refresh-scroll-view.scss'), 'utf8')
    assert.match(refreshStyles, /\.taro-ux-refresh\s*\{[^}]*position:\s*relative[^}]*overflow:\s*hidden/s)
    assert.match(refreshStyles, /\.taro-ux-refresh__scroll\s*\{[^}]*width:\s*100%[^}]*height:\s*100%/s)
    // JSDOM has no layout engine: inspect real compiled SCSS declarations
    // instead of assigning fabricated clientHeight/getBoundingClientRect values.
    const declarations = (relative, selector) => {
      const css = sass.compile(path.join(root, 'apps/miniapp/src', relative), { logger: sass.Logger.silent }).css
      const result = {}
      postcss.parse(css).walkRules(rule => {
        if (rule.selector !== selector) return
        rule.nodes.filter(node => node.type === 'decl').forEach(node => { result[node.prop] = node.value })
      })
      return result
    }
    const homeViewport = declarations('pages/home/index.scss', '.student-home-content')
    assert.equal(homeViewport.height, '100%')
    assert.equal(homeViewport['padding-bottom'], undefined, 'viewport padding must not remove space behind the translucent bottom nav')
    assert.equal(homeViewport['--refresh-content-bottom'], 'calc(136rpx + env(safe-area-inset-bottom))')
    const contentBox = declarations('components/refresh-scroll-view.scss', '.taro-ux-refresh__content')
    assert.equal(contentBox['padding-bottom'], 'var(--refresh-content-bottom, 0PX)', 'last item clearance must scroll with content; other pages default to zero')
    const nativeViewport = declarations('components/refresh-scroll-view.scss', '.taro-ux-refresh__scroll')
    assert.equal(nativeViewport.height, '100%', 'native scroll viewport retains full wrapper height')
    const loadingStyles = fs.readFileSync(path.join(root, 'apps/miniapp/src/components/ui/taro-ux-loading.scss'), 'utf8')
    assert.match(loadingStyles, /pointer-events:\s*none/)
    assert.match(loadingStyles, /prefers-reduced-motion:\s*reduce/)
    assert.match(loadingStyles, /\.6s linear/)

    const Directory = load('components/tool-directory.tsx').default
    const container = document.createElement('main')
    document.body.appendChild(container)
    const directoryRoot = createRoot(container)
    const opened = []
    const tools = [
      { id: 'test-1', title: '测试工具甲', subtitle: '测试用途', iconText: '', color: '', categoryId: 'a', categoryName: '测试分类甲' },
      { id: 'test-2', title: '测试工具乙', subtitle: '', iconText: '工具', color: '#2E7DD4', categoryId: 'b', categoryName: '测试分类乙' }
    ]
    await React.act(async () => directoryRoot.render(React.createElement(Directory, { tools, status: 'ready', onOpen: tool => opened.push(tool), onRetry() {} })))
    assert.equal(container.querySelectorAll('.tool-entry').length, 2, 'no invented tool entries')
    assert.equal(container.querySelectorAll('.tool-directory__group').length, 2)
    await event(() => container.querySelector('.tool-entry').taroProps.onClick())
    assert.equal(opened[0], tools[0], 'jump uses the unchanged service target')
    await React.act(async () => directoryRoot.render(React.createElement(Directory, { tools: [], status: 'empty', onOpen() {}, onRetry() {} })))
    assert.equal(container.querySelectorAll('.tool-entry').length, 0)
    assert.ok(container.textContent.includes('暂无可用工具'))
    await React.act(async () => directoryRoot.unmount())
    console.log('refresh feedback: PASS (upstream three-dot/blur thresholds, viewport containment, 300ms cleanup, native props, deduplication, real result, abort, busy, stale owner events, hidden pulling, disable, detached callbacks; real directory rendering)')
  } finally {
    dom.window.close()
    saved.forEach(([key, descriptor]) => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key])
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
