const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const sourcePath = path.resolve(__dirname, '../apps/miniapp/src/hooks/use-layer-presence.ts')
const source = fs.existsSync(sourcePath) ? fs.readFileSync(sourcePath, 'utf8') : ''
const loaded = new Module(sourcePath, module)
loaded.require = (id) => id === 'react'
  ? require(path.resolve(__dirname, '../node_modules/react'))
  : Module.prototype.require.call(loaded, id)
loaded._compile(ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX },
}).outputText, sourcePath)
const { useLayerPresence, LAYER_EXIT_MS } = loaded.exports

assert.equal(typeof useLayerPresence, 'function', 'useLayerPresence 必须导出 hook')
assert.equal(Number.isInteger(LAYER_EXIT_MS), true, '退出时长应为明确的整数毫秒')
assert.ok(LAYER_EXIT_MS >= 120 && LAYER_EXIT_MS <= 260, '退出动效应保持短时范围')
const dialogSource = fs.readFileSync(path.resolve(__dirname, '../apps/miniapp/src/components/ui/app-dialog.tsx'), 'utf8')
const sheetSource = fs.readFileSync(path.resolve(__dirname, '../apps/miniapp/src/components/ui/app-action-sheet.tsx'), 'utf8')
assert.match(dialogSource, /interactive && !loading/, '对话框遮罩关闭必须受交互阶段与 loading 双重保护')
assert.match(dialogSource, /disabled=\{!interactive\}/, '对话框确认按钮在退出期必须禁用')
assert.match(sheetSource, /interactive && !loading/, '操作面板关闭必须受交互阶段与 loading 双重保护')

async function run() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const original = {
    window: globalThis.window,
    document: globalThis.document,
    IS_REACT_ACT_ENVIRONMENT: globalThis.IS_REACT_ACT_ENVIRONMENT,
  }
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const states = []
  let root

  function Probe({ visible }) {
    const state = useLayerPresence(visible)
    states.push({ visible, ...state })
    return React.createElement('output', { 'data-mounted': String(state.mounted), 'data-phase': state.phase })
  }

  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
  try {
    root = createRoot(document.querySelector('main'))
    await React.act(async () => root.render(React.createElement(Probe, { visible: false })))
    assert.deepEqual(states.at(-1), { visible: false, mounted: false, phase: 'closed' }, '初始关闭应不挂载')

    await React.act(async () => root.render(React.createElement(Probe, { visible: true })))
    assert.equal(states.some((state) => state.visible && state.phase === 'enter'), true, '打开应先进入 enter 阶段')
    await React.act(async () => wait(0))
    assert.deepEqual(states.at(-1), { visible: true, mounted: true, phase: 'open' }, '进入阶段完成后应保持 open')

    await React.act(async () => root.render(React.createElement(Probe, { visible: false })))
    assert.deepEqual(states.at(-1), { visible: false, mounted: true, phase: 'exit' }, '关闭期间应保留挂载并标记 exit')
    await React.act(async () => wait(LAYER_EXIT_MS + 20))
    assert.deepEqual(states.at(-1), { visible: false, mounted: false, phase: 'closed' }, '退出完成后才卸载')

    await React.act(async () => root.render(React.createElement(Probe, { visible: true })))
    await React.act(async () => wait(0))
    await React.act(async () => root.render(React.createElement(Probe, { visible: false })))
    await React.act(async () => root.render(React.createElement(Probe, { visible: true })))
    await React.act(async () => wait(LAYER_EXIT_MS + 20))
    assert.equal(states.at(-1).mounted, true, '快速重开不能被旧退出任务卸载')
    assert.equal(states.at(-1).phase, 'open', '快速重开最终回到 open')

    await React.act(async () => root.render(React.createElement(Probe, { visible: false })))
    await React.act(async () => root.unmount())
    await wait(LAYER_EXIT_MS + 20)
    assert.equal(document.querySelector('output'), null, '卸载后旧定时器不能回写 DOM')
  } finally {
    if (root) {
      try { await React.act(async () => root.unmount()) } catch {}
    }
    dom.window.close()
    if (original.window === undefined) delete globalThis.window
    else globalThis.window = original.window
    if (original.document === undefined) delete globalThis.document
    else globalThis.document = original.document
    if (original.IS_REACT_ACT_ENVIRONMENT === undefined) delete globalThis.IS_REACT_ACT_ENVIRONMENT
    else globalThis.IS_REACT_ACT_ENVIRONMENT = original.IS_REACT_ACT_ENVIRONMENT
  }
}

run().then(() => console.log('layer presence: PASS (enter/open/exit, fast reopen and unmount cleanup)'))
  .catch((error) => { console.error(error); process.exitCode = 1 })
