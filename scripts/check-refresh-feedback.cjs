const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')
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
      throw new Error('Unexpected dependency ' + id)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file, compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, file)
    return loaded.exports
  }
  const Refresh = load('components/refresh-scroll-view.tsx').default
  const app = createRoot(document.querySelector('main'))
  props = { enabled: true, busy: false, refreshKey: 'user-a', onRefresh: () => { count++; const task = defer(); tasks.push(task); return task.promise } }
  const render = () => React.act(async () => app.render(React.createElement(Refresh, props)))
  const event = cb => React.act(async () => { cb() })
  const scroll = () => document.querySelector('[data-kind=ScrollView]').taroProps
  const label = () => document.querySelector('[data-kind=Slot]').textContent
  const wait = ms => React.act(() => new Promise(resolve => setTimeout(resolve, ms)))
  try {
    await render()
    assert.equal(document.querySelector('[data-kind=Slot]').taroProps.name, 'refresher')
    await event(() => scroll().onRefresherPulling({ detail: { dy: 70 } }))
    assert.equal(label(), '松开刷新')
    await event(() => { scroll().onRefresherRefresh(); scroll().onRefresherRefresh() })
    assert.equal(count, 1)
    assert.equal(scroll().refresherTriggered, true)
    props = { ...props, busy: true }; await render()
    assert.equal(scroll().refresherEnabled, true, 'in-flight pull must keep native slot enabled')
    await event(() => scroll().onRefresherAbort())
    assert.equal(label(), '正在更新', 'abort cannot report a pending request as cancelled or successful')
    await event(() => tasks[0].resolve(true))
    assert.equal(label(), '已更新')
    await wait(470)
    assert.equal(scroll().refresherTriggered, false)
    await event(() => scroll().onRefresherRefresh())
    assert.equal(count, 1, 'external paging prevents overlapping pull requests')
    props = { ...props, busy: false }; await render()
    await event(() => scroll().onRefresherRefresh())
    await event(() => tasks[1].reject(new Error('offline')))
    assert.equal(label(), '更新失败，请重试')
    await wait(820)
    await event(() => scroll().onRefresherRefresh())
    props = { ...props, refreshKey: 'user-b' }; await render()
    await event(() => tasks[2].resolve(true))
    assert.equal(label(), '下拉刷新', 'old owner cannot update refresh feedback')
    await event(() => scroll().onRefresherRefresh())
    await event(() => hide())
    await event(() => tasks[3].resolve(true))
    assert.equal(scroll().refresherTriggered, false)
    await event(() => show())
    await event(() => scroll().onRefresherRefresh())
    props = { ...props, enabled: false }; await render()
    await event(() => tasks[4].resolve(true))
    assert.equal(label(), '下拉刷新', 'hidden channel ignores completion')
    props = { ...props, enabled: true }; await render()
    await event(() => scroll().onRefresherRefresh())
    await event(() => app.unmount())
    await event(() => tasks[5].resolve(true))

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
    console.log('refresh feedback: PASS (deduplication, real result, abort, busy, owner change, hide, disable, unmount; real directory rendering)')
  } finally {
    dom.window.close()
    saved.forEach(([key, descriptor]) => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key])
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
