const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const sourcePath = path.resolve(__dirname, '../apps/miniapp/src/utils/gallery-presentation.ts')
const loaded = new Module(sourcePath, module)
loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText, sourcePath)
const { galleryViewportHeight, createGalleryCounter, GALLERY_COUNTER_TIMING } = loaded.exports

function height(media, viewport, mode, actionbar) {
  return galleryViewportHeight(media, viewport, mode, actionbar)
}

assert.equal(height({ width: 600, height: 800 }, { width: 320, height: 568, headerHeight: 64, safeBottom: 0 }, 'post', 112), 360)
assert.equal(height({ width: 600, height: 800 }, { width: 375, height: 667, headerHeight: 88, safeBottom: 0 }, 'post', 112), 427)
assert.equal(height({ width: 600, height: 800 }, { width: 430, height: 932, headerHeight: 96, safeBottom: 24 }, 'post', 112), 573)
assert.equal(height({ width: 600, height: 800 }, { width: 667, height: 375, headerHeight: 64, safeBottom: 0 }, 'post', 0), 217)
assert.equal(height(undefined, { width: 375, height: 667, headerHeight: 88, safeBottom: 0 }, 'product', 120), 375, '商品媒体保持规则正方形')
assert.equal(height({ width: 0, height: 10 }, { width: 0, height: Number.NaN, headerHeight: -1, safeBottom: -2 }, 'post', Number.NaN), 281, '非法尺寸回退为逻辑窗口，不读取设备截图像素')
const viewport = { width: 375, height: 667, headerHeight: 88, safeBottom: 0 }
const portrait = { width: 600, height: 800 }
assert.equal(height(portrait, viewport, 'post', 0), 483, '无操作栏时释放真实可用空间')
assert.equal(height(portrait, { ...viewport, safeBottom: 34 }, 'post', 112), 393, '安全区不能被媒体占用')
assert.equal(height({ width: 6000, height: 10 }, viewport, 'post', 0), 187, '极宽图仍有可查看窗口')
assert.equal(height({ width: 10, height: 6000 }, viewport, 'post', 0), 483, '长图不侵占保留的正文空间')
assert.equal(height(portrait, { ...viewport, height: 80 }, 'post', 112), 0, '不足的视口不能产生负高度')
for (const width of [320, 375, 430, 667]) {
  for (const windowHeight of [320, 568, 932]) {
    for (const safeBottom of [0, 34]) {
      for (const actionbar of [0, 112, 120]) {
        for (const mode of ['post', 'product']) {
          const result = height(portrait, { width, height: windowHeight, headerHeight: 88, safeBottom }, mode, actionbar)
          const contentSpace = Math.max(0, windowHeight - 88 - safeBottom - actionbar * width / 750)
          assert.ok(Number.isInteger(result) && result >= 0 && result <= contentSpace, '媒体高度必须处于窗口可用区域内')
        }
      }
    }
  }
}

const events = []
const timers = []
const clock = {
  set(callback, delay) {
    const task = { callback, delay, cleared: false }
    timers.push(task)
    return task
  },
  clear(task) { task.cleared = true },
}
const counter = createGalleryCounter((visible) => events.push(visible), clock)
counter.reveal()
assert.deepEqual(events, [true])
assert.equal(timers[0].delay, GALLERY_COUNTER_TIMING.fadeMs + GALLERY_COUNTER_TIMING.holdMs)
timers[0].callback()
assert.deepEqual(events, [true, false], '淡入完成后保持 5000ms，再进入淡出状态')
counter.reveal()
const stale = timers[1]
counter.hold()
assert.equal(stale.cleared, true)
stale.callback()
assert.deepEqual(events, [true, false, true, true], '重复触摸会使旧计时任务失效')
counter.reveal()
counter.hide()
timers[2].callback()
assert.deepEqual(events, [true, false, true, true, true, false], '隐藏页面会清理页码计时')
counter.reveal()
counter.dispose()
assert.equal(timers[3].cleared, true)
timers[3].callback()
assert.deepEqual(events, [true, false, true, true, true, false, true], '销毁后迟到回调不能改变状态')

async function hookLifecycle() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = globals.map((key) => Object.getOwnPropertyDescriptor(globalThis, key))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  let show, hide, resize, result
  let windowInfo = { windowWidth: 375, windowHeight: 667, screenHeight: 667, safeArea: { bottom: 633 } }
  const hookTimers = []
  const hookClock = {
    set(callback, delay) { const task = { callback, delay, cleared: false }; hookTimers.push(task); return task },
    clear(task) { task.cleared = true },
  }
  const hookPath = path.resolve(__dirname, '../apps/miniapp/src/hooks/use-gallery-presentation.ts')
  const hook = new Module(hookPath, module)
  hook.require = (id) => {
    if (id === 'react') return React
    if (id === '@tarojs/taro') return { useDidShow: (callback) => { show = callback }, useDidHide: (callback) => { hide = callback }, useResize: (callback) => { resize = callback } }
    if (id.endsWith('/services/taro')) return { default: { getWindowInfo: () => windowInfo } }
    if (id.endsWith('/capsule-safe')) return { getCapsuleLayout: () => ({ totalHeaderHeight: 88 }) }
    if (id.endsWith('/gallery-presentation')) return { ...loaded.exports, createGalleryCounter: (callback) => createGalleryCounter(callback, hookClock) }
    throw new Error('Unexpected hook dependency: ' + id)
  }
  hook._compile(ts.transpileModule(fs.readFileSync(hookPath, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, hookPath)
  function Probe(props) {
    result = hook.exports.useGalleryPresentation(props.id, props.count, portrait, 'post', true)
    return null
  }
  const root = createRoot(document.querySelector('main'))
  try {
    await React.act(async () => root.render(React.createElement(Probe, { id: 'a', count: 2 })))
    assert.equal(result.height, 393, 'hook 读取实际逻辑窗口与安全区')
    assert.equal(result.counterVisible, true)
    const beforeHide = hookTimers.at(-1)
    await React.act(async () => hide())
    assert.equal(result.counterVisible, false)
    assert.equal(beforeHide.cleared, true)
    await React.act(async () => root.render(React.createElement(Probe, { id: 'b', count: 3 })))
    assert.equal(result.counterVisible, false, '后台到达的新媒体不能重新开启页码任务')
    await React.act(async () => show())
    assert.equal(result.counterVisible, true)
    await React.act(async () => beforeHide.callback())
    assert.equal(result.counterVisible, true, '前后台切换后旧任务不能隐藏新页码')
    windowInfo = { windowWidth: 430, windowHeight: 932, screenHeight: 932, safeArea: { bottom: 908 } }
    await React.act(async () => resize())
    assert.equal(result.height, 573)
    const beforeSingle = hookTimers.at(-1)
    await React.act(async () => root.render(React.createElement(Probe, { id: 'b', count: 1 })))
    assert.equal(result.counterVisible, false, '只剩单图不显示页码')
    assert.equal(beforeSingle.cleared, true)
    await React.act(async () => root.render(React.createElement(Probe, { id: 'c', count: 2 })))
    const beforeUnmount = hookTimers.at(-1)
    await React.act(async () => root.unmount())
    assert.equal(beforeUnmount.cleared, true, '卸载时清理最后一个计时任务')
    beforeUnmount.callback()
  } finally {
    await React.act(async () => root.unmount())
    dom.window.close()
    globals.forEach((key, index) => originals[index] ? Object.defineProperty(globalThis, key, originals[index]) : delete globalThis[key])
  }
}

hookLifecycle().then(() => console.log('gallery presentation: PASS (responsive height matrix, counter and page lifecycle)'))
  .catch((error) => { console.error(error); process.exitCode = 1 })
