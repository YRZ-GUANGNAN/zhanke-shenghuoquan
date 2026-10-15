const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const sass = require('sass')

async function run() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const names = ['window', 'document', 'requestAnimationFrame', 'cancelAnimationFrame', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = names.map(name => Object.getOwnPropertyDescriptor(globalThis, name))
  const frames = new Map()
  let frameId = 0
  Object.assign(globalThis, {
    window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true,
    requestAnimationFrame: callback => { const id = ++frameId; frames.set(id, callback); return id },
    cancelAnimationFrame: id => frames.delete(id),
  })
  const container = document.querySelector('main')
  const root = createRoot(container)
  const primitive = tag => props => React.createElement(tag, {
    className: props.className, style: props.style,
    ref: node => { if (node) node.taroProps = props },
  }, props.children)
  const modules = new Map()
  const sourceRoot = path.resolve(__dirname, '../apps/miniapp/src')
  const load = source => {
    if (modules.has(source)) return modules.get(source).exports
    const loaded = new Module(source, module)
    modules.set(source, loaded)
    loaded.require = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return { View: primitive('div'), Text: primitive('span') }
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('/app-icon-button')) return { AppIconButton: primitive('button') }
      if (id.startsWith('.')) {
        const base = path.resolve(path.dirname(source), id)
        const file = [base + '.ts', base + '.tsx'].find(candidate => fs.existsSync(candidate))
        if (file) return load(file)
      }
      throw new Error(`Unexpected sheet test dependency: ${id}`)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(source, 'utf8'), {
      compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    }).outputText, source)
    return loaded.exports
  }
  const point = (x, y, identifier = 1) => ({ clientX: x, clientY: y, identifier })
  const event = (touches, changedTouches = touches) => ({ touches, changedTouches, stopPropagation() {} })
  const frame = async () => React.act(async () => {
    const pending = [...frames.values()]; frames.clear()
    pending.forEach(callback => callback())
  })
  try {
    const { AppActionSheet } = load(path.join(sourceRoot, 'components/ui/app-action-sheet.tsx'))
    const { createSheetDrag } = load(path.join(sourceRoot, 'utils/sheet-drag.ts'))
    let closes = 0
    const props = { visible: true, title: '测试操作', onClose: () => { closes++ }, children: React.createElement('p', null, '可滚动内容') }
    const render = async patch => React.act(async () => root.render(React.createElement(AppActionSheet, { ...props, ...patch })))
    const handle = () => container.querySelector('.app-action-sheet__drag-handle').taroProps
    const send = async (name, touches, changedTouches) => React.act(async () => handle()[name](event(touches, changedTouches)))
    const start = async () => send('onTouchStart', [point(100, 100)])
    const move = async (y, x = 100) => send('onTouchMove', [point(x, y)])
    const end = async (y, x = 100) => send('onTouchEnd', [], [point(x, y)])

    await render()
    await frame(); await frame()
    const contents = container.querySelector('.app-action-sheet__content').taroProps
    assert.equal(contents.onTouchMove, undefined, '内容滚动区不能认领关闭手势')
    assert.equal(handle().catchMove, true, '只有把手应阻止滚动穿透')
    await start(); await move(130)
    const draggedOffset = Number(container.querySelector('.app-action-sheet').style.transform.match(/translateY\((.+)px\)/)?.[1])
    assert.ok(Math.abs(draggedOffset - 21.6) < 0.001, '拖动应有阻尼位移')
    await end(130)
    assert.equal(closes, 0, '短拖不能关闭')
    assert.equal(container.querySelector('.app-action-sheet').style.transform, '', '短拖释放位移以允许CSS回弹')
    await start(); await move(220); await send('onTouchCancel', [])
    assert.equal(closes, 0, '系统取消必须保留面板')
    assert.equal(container.querySelector('.app-action-sheet').style.transform, '')
    await start(); await move(220)
    await send('onTouchMove', [point(100, 220), point(110, 230, 2)])
    await end(240)
    assert.equal(closes, 0, '第二根手指应取消整次拖动')
    await start(); await move(220)
    await render({ loading: true })
    await end(220)
    await React.act(async () => container.querySelector('.app-action-sheet-layer__mask').taroProps.onClick())
    assert.equal(closes, 0, '拖动期间开始提交应复位且禁止关闭')
    assert.equal(container.querySelector('.app-action-sheet').style.transform, '')
    await render({ loading: false })
    await start(); await move(110, 180); await end(220, 180)
    assert.equal(closes, 0, '最初横向动作不能后续转成关闭')
    await start(); await move(220); await end(220)
    assert.equal(closes, 1, '足够距离的下拉应关闭一次')
    await React.act(async () => {
      container.querySelector('.app-action-sheet-layer__mask').taroProps.onClick()
      container.querySelector('.app-action-sheet__close').taroProps.onClick()
    })
    assert.equal(closes, 1, '关闭请求与其他关闭入口不应重复调用')
    await render({ visible: false })
    assert.ok(container.querySelector('.app-action-sheet-layer--exit'), '关闭必须复用原退出生命周期')
    await render({ visible: true })
    await frame(); await frame()
    assert.equal(container.querySelector('.app-action-sheet').style.transform, '', '退出中快速重开无残留位移')
    await start(); await move(220); await end(220)
    assert.equal(closes, 2, '快速重开应重新允许拖动关闭')

    const tracker = createSheetDrag()
    tracker.start([point(0, 0)])
    tracker.move([point(0, 120)])
    tracker.move([point(0, 10)])
    assert.equal(tracker.end([point(0, 10)], []), false, '反向拖回起点应取消关闭')
    tracker.start([point(0, 0)])
    tracker.move([point(0, 120, 2)])
    assert.equal(tracker.end([point(0, 130, 2)], []), false, '触点标识改变不能串用原手势')
    const css = sass.compile(path.join(sourceRoot, 'components/ui/app-action-sheet.scss'), {
      silenceDeprecations: ['legacy-js-api', 'global-builtin', 'color-functions', 'import'],
    }).css
    assert.match(css, /min-height: 44PX/, '把手触控区至少44物理像素')
    assert.match(css, /prefers-reduced-motion/, '保留减少动态效果支持')
  } finally {
    await React.act(async () => root.unmount())
    dom.window.close()
    names.forEach((name, i) => originals[i] ? Object.defineProperty(globalThis, name, originals[i]) : delete globalThis[name])
  }
  console.log('sheet dismiss: PASS (real sheet, handle-only drag, rebound, cancel, multitouch, loading, close deduplication, fast reopen)')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
