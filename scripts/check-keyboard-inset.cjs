const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

function load(relative) {
  const sourcePath = path.resolve(__dirname, '..', relative)
  const loaded = new Module(sourcePath, module)
  loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, sourcePath)
  return loaded.exports
}

const { normalizeKeyboardHeight, resolveKeyboardInset } = load('apps/miniapp/src/utils/keyboard-inset.ts')

assert.equal(normalizeKeyboardHeight(280), 280)
assert.equal(normalizeKeyboardHeight(-1), 0)
assert.equal(normalizeKeyboardHeight('bad'), 0)
assert.equal(normalizeKeyboardHeight(900, 667), 667)
assert.equal(resolveKeyboardInset(280, { baselineHeight: 667, windowHeight: 667 }), 280, '窗口未缩短时使用键盘事件高度')
assert.equal(resolveKeyboardInset(280, { baselineHeight: 667, windowHeight: 387 }), 0, '平台已缩短完整键盘高度时不重复扣除')
assert.equal(resolveKeyboardInset(300, { baselineHeight: 667, windowHeight: 500 }), 133, '平台已部分缩短时只补偿剩余遮挡')
assert.equal(resolveKeyboardInset(300, { baselineHeight: 0, windowHeight: 0 }), 300, '缺少窗口信息时保留事件高度')
assert.equal(resolveKeyboardInset(0, { baselineHeight: 667, windowHeight: 667 }), 0)

for (const reported of [0, 120, 280, 420, Number.NaN, -20]) {
  for (const baseline of [0, 320, 667, 932]) {
    for (const current of [0, 200, 320, 500, 667, 932]) {
      const value = resolveKeyboardInset(reported, { baselineHeight: baseline, windowHeight: current })
      assert.ok(Number.isInteger(value) && value >= 0, `inset must be a non-negative integer: ${reported}/${baseline}/${current}`)
    }
  }
}

for (const relative of [
  'apps/miniapp/src/pages/post-detail/index.tsx',
  'apps/miniapp/src/pages/secondhand-chat/index.tsx',
  'apps/miniapp/src/pages/delivery-conversation/index.tsx',
]) {
  const source = fs.readFileSync(path.resolve(__dirname, '..', relative), 'utf8')
  assert.match(source, /useKeyboardInset/, `${relative} 应使用统一键盘来源`)
  assert.match(source, /onKeyboardHeightChange/, `${relative} 应监听键盘高度变化`)
  assert.match(source, /adjustPosition=\{false\}/, `${relative} 应关闭平台自动顶起以避免双重避让`)
}

async function lifecycle() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = globals.map((key) => Object.getOwnPropertyDescriptor(globalThis, key))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  let show, hide, resize, result
  let windowHeight = 667
  const sourcePath = path.resolve(__dirname, '../apps/miniapp/src/hooks/use-keyboard-inset.ts')
  const hook = new Module(sourcePath, module)
  hook.require = (id) => {
    if (id === 'react') return React
    if (id === '@tarojs/taro') return {
      useDidShow: (callback) => { show = callback },
      useDidHide: (callback) => { hide = callback },
      useResize: (callback) => { resize = callback },
    }
    if (id.endsWith('/services/taro')) return { default: { getWindowInfo: () => ({ windowHeight }) } }
    if (id.endsWith('/utils/keyboard-inset')) return { resolveKeyboardInset }
    throw new Error('Unexpected hook dependency: ' + id)
  }
  hook._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
    compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, sourcePath)
  function Probe() {
    result = hook.exports.useKeyboardInset()
    return null
  }
  const root = createRoot(document.querySelector('main'))
  try {
    await React.act(async () => root.render(React.createElement(Probe)))
    await React.act(async () => result.onFocus())
    windowHeight = 387
    await React.act(async () => resize())
    await React.act(async () => result.onKeyboardHeightChange({ detail: { height: 280 } }))
    assert.equal(result.inset, 0, 'windowHeight 缩短时不双重扣减')
    // reset is the public lifecycle cleanup used by all three pages.
    await React.act(async () => result.reset())
    await React.act(async () => result.onKeyboardHeightChange({ detail: { height: 280 } }))
    assert.equal(result.inset, 0, '清理后迟到的键盘事件不能恢复旧高度')
    windowHeight = 667
    await React.act(async () => result.onFocus())
    await React.act(async () => result.onKeyboardHeightChange({ detail: { height: 280 } }))
    assert.equal(result.inset, 280, '窗口未缩短时使用键盘事件高度')
    await React.act(async () => hide())
    assert.equal(result.inset, 0, '切后台清理旧键盘高度')
    await React.act(async () => show())
    assert.equal(result.inset, 0, '回前台不恢复旧键盘高度')
  } finally {
    await React.act(async () => root.unmount())
    dom.window.close()
    globals.forEach((key, index) => originals[index] ? Object.defineProperty(globalThis, key, originals[index]) : delete globalThis[key])
  }
}

lifecycle()
  .then(() => console.log('keyboard inset: PASS (normalization, viewport compensation, lifecycle wiring)'))
  .catch((error) => { console.error(error); process.exitCode = 1 })
