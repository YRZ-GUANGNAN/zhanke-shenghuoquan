const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const root = path.resolve(__dirname, '..')
const sourcePath = path.join(root, 'apps/miniapp/src/utils/edge-back.ts')
const loaded = new Module(sourcePath, module)
loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText, sourcePath)

const { createEdgeBack } = loaded.exports
const touch = (identifier, clientX, clientY) => ({ identifier, clientX, clientY })
const nativeTouch = Object.create({ identifier: 1, clientX: 10, clientY: 240 })
const native = createEdgeBack()
native.start([nativeTouch])
native.move([touch(1, 180, 242)])
assert.equal(native.end([touch(1, 180, 242)], [], 390), true, '浏览器原生 Touch 原型坐标也可以识别')
const gesture = (startX, moveX, moveY = 0, endX = moveX, viewport = 390) => {
  const tracker = createEdgeBack()
  tracker.start([touch(1, startX, 240)])
  tracker.move([touch(1, moveX, 240 + moveY)])
  return tracker.end([touch(1, endX, 240 + moveY)], [], viewport)
}

assert.equal(gesture(10, 60, 2, 80), false, '短滑不应返回')
assert.equal(gesture(10, 180, 8, 180), true, '390px 屏幕达到 24% 阈值应返回')
assert.equal(gesture(10, 100, 0, 100, 320), true, '320px 屏幕使用受限后的阈值应返回')
assert.equal(gesture(10, 180, 160, 180), false, '纵向优先手势不应返回')
assert.equal(gesture(10, -30, 0, -30), false, '左滑不应返回')
assert.equal(gesture(-1, 180, 0), false, '非法负起点不应返回')
assert.equal(gesture(10, 180, 0, 20), false, '大幅滑动后收回不应返回')
assert.equal(gesture(10, 100, 0, 100, 430), false, '宽屏短滑不误用窄屏阈值')
assert.equal(gesture(10, 140, 0, 140, 768), true, '宽屏阈值不超过 120px')

const wrongEdge = createEdgeBack()
wrongEdge.start([touch(1, 25, 100)])
assert.equal(wrongEdge.end([touch(1, 200, 100)], [], 390), false, '非左边缘起点不应返回')

const multiTouch = createEdgeBack()
multiTouch.start([touch(1, 10, 100), touch(2, 14, 110)])
assert.equal(multiTouch.end([touch(1, 200, 100)], [], 390), false, '多指起点不应返回')

const changedFinger = createEdgeBack()
changedFinger.start([touch(1, 10, 100)])
changedFinger.move([touch(2, 160, 100)])
assert.equal(changedFinger.end([touch(1, 180, 100)], [], 390), false, '触点更换后应取消')

const cancelled = createEdgeBack()
cancelled.start([touch(1, 10, 100)])
cancelled.move([touch(1, 160, 100)])
cancelled.cancel()
assert.equal(cancelled.end([touch(1, 180, 100)], [], 390), false, '取消生命周期后不应返回')

const verticalFirst = createEdgeBack()
verticalFirst.start([touch(1, 10, 100)])
verticalFirst.move([touch(1, 12, 130)])
verticalFirst.move([touch(1, 200, 130)])
assert.equal(verticalFirst.end([touch(1, 200, 130)], [], 390), false, '先纵向滚动后转横向不应抢走手势')
const invalid = createEdgeBack()
invalid.start([touch(1, 10, 100)])
invalid.move([touch(1, NaN, 100)])
assert.equal(invalid.end([touch(1, 200, 100)], [], 390), false, '无效坐标取消整次手势')
const remaining = createEdgeBack()
remaining.start([touch(1, 10, 100)])
remaining.move([touch(1, 200, 100)])
assert.equal(remaining.end([touch(1, 200, 100)], [touch(2, 210, 110)], 390), false, '仍有手指未离开时不能返回')
const duplicate = createEdgeBack()
duplicate.start([touch(1, 10, 100)])
duplicate.move([touch(1, 180, 100)])
assert.equal(duplicate.end([touch(1, 180, 100)], [], 390), true)
assert.equal(duplicate.end([touch(1, 180, 100)], [], 390), false, '重复 touchend 不能返回两次')

async function checkHook() {
  const React = require('react')
  const { JSDOM } = require('jsdom')
  const { createRoot } = require('react-dom/client')
  const dom = new JSDOM('<main></main>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const previous = globals.map(key => Object.getOwnPropertyDescriptor(globalThis, key))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const originalNow = Date.now
  let now = 1000
  Date.now = () => now
  const roots = []
  async function mount({ platform = 'android', depth = 2, failHome = false } = {}) {
    const previousPage = { route: 'pages/home/index', scrollTop: 640, tab: 'secondhand' }
    const profilePage = { route: 'pages/user-profile/index' }
    const stack = depth > 1 ? [previousPage, profilePage] : [profilePage]
    const calls = { back: 0, home: 0, toast: 0 }
    const callbacks = { show: new Set(), hide: new Set(), resize: new Set() }
    const register = name => callback => {
      const ref = React.useRef(callback)
      ref.current = callback
      React.useEffect(() => { callbacks[name].add(ref); return () => callbacks[name].delete(ref) }, [])
    }
    const facade = {
      getDeviceInfo: () => ({ platform }),
      getSystemInfoSync: () => ({ platform }),
      getCurrentPages: () => stack,
      getWindowInfo: () => ({ windowWidth: 390 }),
      navigateBack: async () => { calls.back++; if (stack.length < 2) throw Error('first page'); stack.pop() },
      reLaunch: async ({ url }) => { calls.home++; if (failHome) throw Error('offline'); stack.splice(0, stack.length, { route: url }) },
      showToast: async () => { calls.toast++ },
    }
    const cache = new Map()
    function load(file) {
      file = path.resolve(file)
      if (cache.has(file)) return cache.get(file)
      const source = new Module(file, module)
      source.require = id => {
        if (id === 'react') return React
        if (id === '@tarojs/taro') return { useDidShow: register('show'), useDidHide: register('hide'), useResize: register('resize') }
        if (id.endsWith('/services/taro')) return { default: facade }
        if (id.startsWith('.')) return load(path.resolve(path.dirname(file), id + '.ts'))
        throw Error('Unexpected edge-back test dependency: ' + id)
      }
      source._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {
        compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
      }).outputText, file)
      cache.set(file, source.exports)
      return source.exports
    }
    const { useEdgeBack } = load(path.join(root, 'apps/miniapp/src/hooks/use-edge-back.ts'))
    let enabled = true, api
    function Subject() { api = useEdgeBack(enabled); return null }
    const container = document.createElement('section')
    document.body.appendChild(container)
    const reactRoot = createRoot(container)
    roots.push(reactRoot)
    const event = fn => React.act(async () => { fn() })
    const render = () => event(() => reactRoot.render(React.createElement(Subject)))
    await render()
    const dispatch = (kind, x = 10, y = 240, identifier = 1) => event(() => api.handlers[kind]({
      touches: kind === 'onTouchEnd' ? [] : [touch(identifier, x, y)],
      changedTouches: [touch(identifier, x, y)],
    }))
    const drag = async (x = 160) => { await dispatch('onTouchStart'); await dispatch('onTouchMove', x); await dispatch('onTouchEnd', x) }
    return {
      get api() { return api }, calls, stack, previousPage, dispatch, drag, event,
      enable: async value => { enabled = value; await render() },
      lifecycle: name => event(() => { for (const ref of callbacks[name]) ref.current() }),
      recover: () => { failHome = false },
    }
  }
  try {
    const android = await mount()
    assert.equal(android.api.available, true)
    await android.dispatch('onTouchStart')
    await android.dispatch('onTouchMove', 180)
    assert.equal(android.api.tracking, true)
    assert.equal(android.api.offset, 12, '返回箭头跟手位移有上限')
    assert.equal(android.api.canActivate(), false, '拖动中不能产生返回按钮点击')
    await android.dispatch('onTouchEnd', 180)
    assert.equal(android.calls.back, 1)
    assert.equal(android.stack[0], android.previousPage, '返回保留来源页实例与列表状态')
    await android.dispatch('onTouchEnd', 180)
    assert.equal(android.calls.back, 1)
    assert.equal(android.api.canActivate(), false, '松手后的派生 click 被抑制')
    now += 500
    assert.equal(android.api.canActivate(), true)

    const short = await mount()
    await short.drag(60)
    assert.equal(short.calls.back, 0)
    assert.equal(short.api.offset, 0)
    assert.equal(short.api.canActivate(), false, '短滑取消也不能误触透明返回键')
    now += 500
    await short.dispatch('onTouchStart')
    await short.dispatch('onTouchEnd')
    assert.equal(short.api.canActivate(), true, '普通点按不被屏蔽')

    for (const name of ['hide', 'resize']) {
      const changed = await mount()
      await changed.dispatch('onTouchStart')
      await changed.dispatch('onTouchMove', 160)
      await changed.lifecycle(name)
      await changed.dispatch('onTouchEnd', 160)
      assert.equal(changed.calls.back, 0, `${name} 后遗留的松手不得导航`)
      await changed.lifecycle('show')
      await changed.drag()
      assert.equal(changed.calls.back, 1, '重新显示后可重新发起手势')
    }
    const blocked = await mount()
    await blocked.dispatch('onTouchStart')
    await blocked.dispatch('onTouchMove', 160)
    await blocked.enable(false)
    assert.equal(blocked.api.available, false, '弹窗期间移除自定义边缘层')
    await blocked.dispatch('onTouchEnd', 160)
    await blocked.drag()
    assert.equal(blocked.calls.back, 0, '禁用期间的触摸不能穿透返回')
    await blocked.enable(true)
    await blocked.drag()
    assert.equal(blocked.calls.back, 1)

    const ios = await mount({ platform: 'ios' })
    assert.equal(ios.api.available, false, 'iOS 有上一页时仅由系统拥有侧滑')
    await ios.drag()
    assert.equal(ios.calls.back, 0, '系统侧滑不能再被应用额外退一页')
    for (const platform of ['ios', 'android']) {
      const direct = await mount({ platform, depth: 1 })
      assert.equal(direct.api.available, true, '分享直达也提供手势出口')
      await direct.drag()
      assert.equal(direct.calls.home, 1)
      assert.equal(direct.stack[0].route, '/pages/home/index')
    }
    const retry = await mount({ depth: 1, failHome: true })
    await retry.drag()
    assert.equal(retry.calls.toast, 1)
    retry.recover()
    await retry.drag()
    assert.equal(retry.calls.home, 2, '导航失败后不会永久锁死手势')
    console.log('公开主页边缘返回检查通过：方向/阈值、短滑、多指、取消、弹窗、生命周期、来源状态、分享兜底、iOS 系统归属与失败重试。')
  } finally {
    await React.act(async () => { for (const node of roots) node.unmount() })
    Date.now = originalNow
    globals.forEach((key, i) => previous[i] ? Object.defineProperty(globalThis, key, previous[i]) : delete globalThis[key])
    dom.window.close()
  }
}
checkHook().catch(error => { console.error(error); process.exitCode = 1 })
