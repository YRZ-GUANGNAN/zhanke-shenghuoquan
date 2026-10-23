const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const sass = require('sass')
const postcss = require('postcss')

async function run() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const names = ['window', 'document', 'requestAnimationFrame', 'cancelAnimationFrame', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = names.map((name) => Object.getOwnPropertyDescriptor(globalThis, name))
  const frames = new Map()
  let frameId = 0
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    requestAnimationFrame: (callback) => { const id = ++frameId; frames.set(id, callback); return id },
    cancelAnimationFrame: (id) => frames.delete(id),
    IS_REACT_ACT_ENVIRONMENT: true,
  })
  const roots = new Set()
  const errors = []
  const check = (name, test) => {
    try { test() } catch (error) { errors.push(`${name}: ${error.message}`) }
  }
  const component = (tag) => ({ children, mode, catchMove, ariaLabel, ...props }) => React.createElement(tag, { ...props, ...(ariaLabel ? { 'aria-label': ariaLabel } : {}) }, children)
  const primitives = { View: component('div'), Text: component('span'), Button: component('button'), Image: component('img') }
  const hides = new Set(), shows = new Set()
  const lifecycle = callbacks => handler => {
    const latest = React.useRef(handler)
    latest.current = handler
    React.useEffect(() => {
      const callback = () => latest.current()
      callbacks.add(callback)
      return () => callbacks.delete(callback)
    }, [])
  }
  const modules = new Map()
  const loadSource = (sourcePath) => {
    if (modules.has(sourcePath)) return modules.get(sourcePath).exports
    const loaded = new Module(sourcePath, module)
    modules.set(sourcePath, loaded)
    loaded.require = (id) => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return primitives
      if (id === '@tarojs/taro') return { useDidHide: lifecycle(hides), useDidShow: lifecycle(shows) }
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('.svg')) return id
      if (id.startsWith('.')) {
        const base = path.resolve(path.dirname(sourcePath), id)
        const resolved = [base, `${base}.ts`, `${base}.tsx`].find((candidate) => fs.existsSync(candidate))
        if (resolved) return loadSource(resolved)
      }
      throw new Error(`Unexpected layer test dependency: ${id}`)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX },
    }).outputText, sourcePath)
    return loaded.exports
  }
  const sourceRoot = path.resolve(__dirname, '../apps/miniapp/src')
  const { useLayerPresence, LAYER_EXIT_MS } = loadSource(path.join(sourceRoot, 'hooks/use-layer-presence.ts'))
  const { AppDialog } = loadSource(path.join(sourceRoot, 'components/ui/app-dialog.tsx'))
  const { AppActionSheet } = loadSource(path.join(sourceRoot, 'components/ui/app-action-sheet.tsx'))
  const { useSheetDismiss } = loadSource(path.join(sourceRoot, 'hooks/use-sheet-dismiss.ts'))
  assert.equal(typeof useLayerPresence, 'function', 'useLayerPresence 必须导出 hook')
  assert.equal(Number.isInteger(LAYER_EXIT_MS), true, '退出时长应为明确的整数毫秒')
  assert.ok(LAYER_EXIT_MS >= 220 && LAYER_EXIT_MS <= 320, '退出时长必须覆盖现有面板动效且保持短时范围')
  const dialogSource = fs.readFileSync(path.join(sourceRoot, 'components/ui/app-dialog.tsx'), 'utf8')
  const sheetSource = fs.readFileSync(path.join(sourceRoot, 'components/ui/app-action-sheet.tsx'), 'utf8')
  assert.match(dialogSource, /interactive && !loading/, '对话框遮罩关闭必须受交互阶段与 loading 双重保护')
  assert.match(dialogSource, /disabled=\{!interactive\}/, '对话框确认按钮在退出期必须禁用')
  assert.match(sheetSource, /interactive && !loading/, '操作面板关闭必须受交互阶段与 loading 双重保护')
  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
  const frame = async () => React.act(async () => {
    const pending = [...frames.values()]
    frames.clear()
    pending.forEach((callback) => callback(performance.now()))
  })
  const fixture = () => {
    const container = document.createElement('main')
    document.body.appendChild(container)
    const root = createRoot(container)
    roots.add(root)
    return {
      container,
      render: async (element) => React.act(async () => root.render(element)),
      unmount: async () => { await React.act(async () => root.unmount()); roots.delete(root); container.remove() },
      click: async (selector, times = 1) => React.act(async () => {
        const node = container.querySelector(selector)
        assert.ok(node, `缺少点击对象 ${selector}`)
        for (let i = 0; i < times; i++) node.click()
      }),
    }
  }

  try {
    const states = []
    function Probe({ visible }) {
      const state = useLayerPresence(visible)
      states.push({ visible, ...state })
      return React.createElement('output', { 'data-mounted': String(state.mounted), 'data-phase': state.phase })
    }
    const probe = fixture()
    await probe.render(React.createElement(Probe, { visible: false }))
    check('初始关闭', () => assert.deepEqual(states.at(-1), { visible: false, mounted: false, phase: 'closed' }))
    await probe.render(React.createElement(Probe, { visible: true }))
    await React.act(async () => wait(5))
    check('进入首帧', () => assert.equal(states.at(-1).phase, 'enter', '绘制前不能靠0ms定时器直接进入open'))
    await frame()
    check('起始样式帧', () => assert.equal(states.at(-1).phase, 'enter', '应让浏览器绘制起始样式'))
    await frame()
    check('进入完成', () => assert.deepEqual(states.at(-1), { visible: true, mounted: true, phase: 'open' }))
    await probe.render(React.createElement(Probe, { visible: false }))
    check('退出保留', () => assert.deepEqual(states.at(-1), { visible: false, mounted: true, phase: 'exit' }))
    await React.act(async () => wait(LAYER_EXIT_MS + 20))
    check('退出完成', () => assert.deepEqual(states.at(-1), { visible: false, mounted: false, phase: 'closed' }))
    await probe.render(React.createElement(Probe, { visible: true }))
    await frame(); await frame()
    await probe.render(React.createElement(Probe, { visible: false }))
    await probe.render(React.createElement(Probe, { visible: true }))
    check('退场途中反转', () => assert.equal(states.at(-1).phase, 'open', '尚未卸载时重开应反向过渡，不能跳回enter起点'))
    await frame(); await frame()
    await React.act(async () => wait(LAYER_EXIT_MS + 20))
    check('快速重开', () => assert.deepEqual(states.at(-1), { visible: true, mounted: true, phase: 'open' }))
    await probe.render(React.createElement(Probe, { visible: false }))
    await probe.unmount()
    await frame()
    await React.act(async () => wait(LAYER_EXIT_MS + 20))
    check('卸载清理', () => assert.equal(frames.size, 0))

    let confirms = 0
    let cancels = 0
    let outside = 0
    const dialog = fixture()
    const dialogProps = { visible: true, title: '确认操作', onConfirm: () => { confirms++ }, onCancel: () => { cancels++ } }
    const renderDialog = (props = {}) => dialog.render(React.createElement('section', { onClick: () => { outside++ } },
      React.createElement(AppDialog, { ...dialogProps, ...props })))
    await renderDialog()
    await frame(); await frame()
    await dialog.click('.app-button--primary', 2)
    check('确认同帧防重', () => assert.equal(confirms, 1, '真实确认按钮同一task连点只能提交一次'))
    await renderDialog({ loading: true })
    await dialog.click('.app-button--primary')
    await dialog.click('.app-dialog-layer__mask')
    await dialog.click('.app-button--secondary')
    check('提交中保护', () => { assert.equal(confirms, 1); assert.equal(cancels, 0) })
    await renderDialog({ loading: false, error: '网络异常，请重试' })
    await dialog.click('.app-button--primary')
    check('失败重试', () => assert.equal(confirms, 2, 'loading结束后仍应允许确认'))
    await frame()
    await dialog.click('.app-button--primary')
    check('非loading回调重试', () => assert.equal(confirms, 3, '普通回调在下一task应允许重试'))
    await renderDialog({ visible: false })
    await dialog.click('.app-button--primary')
    await dialog.click('.app-dialog-layer__mask')
    await dialog.click('.app-dialog-layer')
    check('退出不交互', () => { assert.equal(confirms, 3); assert.equal(cancels, 0) })
    check('事件不冒泡背景', () => assert.equal(outside, 0, '弹层不应把点击传给外层页面'))
    await renderDialog()
    await frame(); await frame()
    await dialog.click('.app-button--primary')
    check('重开解除确认锁', () => assert.equal(confirms, 4))
    await dialog.unmount()
    await frame()

    let closes = 0
    const sheet = fixture()
    const sheetProps = { visible: true, title: '操作菜单', onClose: () => { closes++ }, children: React.createElement('button', null, '原有操作') }
    const renderSheet = (props = {}) => sheet.render(React.createElement(AppActionSheet, { ...sheetProps, ...props }))
    await renderSheet({ loading: true })
    await frame(); await frame()
    await sheet.click('.app-action-sheet-layer__mask')
    await sheet.click('.app-action-sheet__close')
    check('面板loading不关闭', () => assert.equal(closes, 0))
    await renderSheet({ visible: false })
    await sheet.click('.app-action-sheet-layer__mask')
    await sheet.click('.app-action-sheet__close')
    check('面板退出不关闭', () => assert.equal(closes, 0))
    await renderSheet()
    await frame(); await frame()
    await sheet.click('.app-action-sheet__close')
    check('面板重开可关闭', () => assert.equal(closes, 1))
    await sheet.unmount()

    let gesture, dismissed = 0
    function DragProbe() { gesture = useSheetDismiss(true, () => { dismissed++ }); return null }
    const dragProbe = fixture()
    const event = (y, ended = false) => ({ stopPropagation() {}, touches: ended ? [] : [{ identifier: 1, clientX: 100, clientY: y }], changedTouches: [{ identifier: 1, clientX: 100, clientY: y }] })
    await dragProbe.render(React.createElement(DragProbe))
    await React.act(async () => gesture.handlers.onTouchStart(event(0)))
    await React.act(async () => gesture.handlers.onTouchMove(event(100)))
    check('面板拖动跟手', () => assert.ok(gesture.dragging && gesture.offset > 0))
    await React.act(async () => hides.forEach(hide => hide()))
    check('隐藏时取消拖动', () => assert.equal(gesture.offset, 0, '页面隐藏后不能保留拖动位移'))
    await React.act(async () => shows.forEach(show => show()))
    await React.act(async () => gesture.handlers.onTouchEnd(event(100, true)))
    check('返回后忽略旧抬手', () => assert.equal(dismissed, 0, '回前台的旧手势不能关闭面板'))
    await React.act(async () => gesture.handlers.onTouchStart(event(0)))
    await React.act(async () => gesture.handlers.onTouchMove(event(100)))
    await React.act(async () => gesture.handlers.onTouchEnd(event(100, true)))
    check('返回后新手势可用', () => assert.equal(dismissed, 1))
    await React.act(async () => gesture.handlers.onTouchStart(event(0)))
    await React.act(async () => gesture.handlers.onTouchMove(event(100)))
    await dragProbe.unmount()
    await React.act(async () => gesture.handlers.onTouchEnd(event(100, true)))
    check('卸载后忽略旧抬手', () => assert.equal(dismissed, 1))

    // Sass解析后的实际规则：jsdom没有命中测试，背景点穿由浏览器另行验证。
    for (const name of ['app-dialog', 'app-action-sheet']) {
      const css = sass.compile(path.join(sourceRoot, `components/ui/${name}.scss`), {
        silenceDeprecations: ['legacy-js-api', 'global-builtin', 'color-functions', 'import'],
      }).css
      const rules = postcss.parse(css)
      let longest = 0
      let exitBlocksContent = false
      rules.walkRules((rule) => {
        rule.walkDecls(/^transition/, (declaration) => {
          for (const duration of declaration.value.matchAll(/(\d+(?:\.\d+)?)(ms|s)\b/g)) {
            longest = Math.max(longest, Number(duration[1]) * (duration[2] === 's' ? 1000 : 1))
          }
        })
        rule.walkDecls('pointer-events', (declaration) => {
          const selector = rule.selector.trim()
          if (selector === `.${name}-layer--exit`) {
            check(`${name}退出遮挡层`, () => assert.notEqual(declaration.value, 'none', '全屏遮挡层必须保留命中'))
          }
          if (selector.includes(`.${name}-layer--exit `) && declaration.value === 'none') exitBlocksContent = true
        })
      })
      check(`${name}退出时长`, () => assert.ok(LAYER_EXIT_MS >= longest, `卸载${LAYER_EXIT_MS}ms不能短于最长动效${longest}ms`))
      check(`${name}退出内容锁定`, () => assert.equal(exitBlocksContent, true, '退场内容应关闭命中，同时保留外层遮挡'))
    }

    delete globalThis.requestAnimationFrame
    delete globalThis.cancelAnimationFrame
    const fallback = fixture()
    await fallback.render(React.createElement(Probe, { visible: true }))
    await React.act(async () => wait(60))
    check('无rAF运行时', () => assert.equal(states.at(-1).phase, 'open', '小程序无rAF环境也应完成进入'))
    await fallback.unmount()
    assert.equal(errors.length, 0, errors.join('\n'))
  } finally {
    for (const root of roots) await React.act(async () => root.unmount())
    dom.window.close()
    names.forEach((name, index) => {
      if (originals[index]) Object.defineProperty(globalThis, name, originals[index])
      else delete globalThis[name]
    })
  }
}

run().then(() => console.log('layer presence: PASS (real dialog/sheet, paint frames, exit interception, fast reopen, hidden/unmounted drag cancellation, fallback and cleanup)'))
  .catch((error) => { console.error(error); process.exitCode = 1 })
