const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const root = path.resolve(__dirname, '..')
const sourcePath = path.join(root, 'apps/miniapp/src/hooks/use-viewport-refresh.ts')
const source = fs.readFileSync(sourcePath, 'utf8')
assert.match(source, /useResize\(refresh\)/, '窗口变化必须触发几何刷新')
assert.match(source, /useDidShow\(refresh\)/, '回到前台必须重新读取几何')
assert.match(source, /sameStyle\(current, next\)/, '相同尺寸不能产生无意义重渲染')

async function run() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map((name) => Object.getOwnPropertyDescriptor(globalThis, name))
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  let show
  let resize
  let geometry = 375
  let renders = 0
  const loaded = new Module(sourcePath, module)
  loaded.require = (id) => {
    if (id === 'react') return React
    if (id === '@tarojs/taro') return {
      useDidShow: (callback) => { show = callback },
      useResize: (callback) => { resize = callback },
    }
    if (id.endsWith('/capsule-safe')) return {
      getCapsuleSafeStyle: () => ({
        '--capsule-system-row-height': `${geometry}px`,
        '--capsule-business-row-top': `${geometry + 10}px`,
        '--capsule-business-row-height': '44px',
        '--capsule-total-header-height': `${geometry + 54}px`,
      }),
    }
    throw new Error(`Unexpected viewport hook dependency: ${id}`)
  }
  loaded._compile(ts.transpileModule(source, {
    compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, sourcePath)

  let result
  function Probe() {
    renders += 1
    result = loaded.exports.useViewportRefresh()
    return null
  }
  const rootNode = document.querySelector('main')
  const root = createRoot(rootNode)
  try {
    await React.act(async () => root.render(React.createElement(Probe)))
    assert.equal(result['--capsule-total-header-height'], '429px')
    const initialRenders = renders
    await React.act(async () => resize())
    assert.equal(renders, initialRenders, '相同窗口尺寸不应重复渲染')
    geometry = 430
    await React.act(async () => resize())
    assert.equal(result['--capsule-system-row-height'], '430px')
    assert.equal(result['--capsule-total-header-height'], '484px')
    geometry = 390
    await React.act(async () => show())
    assert.equal(result['--capsule-total-header-height'], '444px')
    console.log('viewport refresh: PASS (resize/show refreshes geometry without remount)')
  } finally {
    await React.act(async () => root.unmount())
    dom.window.close()
    previous.forEach((descriptor, index) => {
      const name = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'][index]
      if (descriptor) Object.defineProperty(globalThis, name, descriptor)
      else delete globalThis[name]
    })
  }
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
