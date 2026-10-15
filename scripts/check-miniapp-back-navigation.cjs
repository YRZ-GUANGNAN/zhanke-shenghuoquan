const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const root = path.resolve(__dirname, '..')
const sourcePath = path.join(root, 'apps/miniapp/src/utils/navigation.ts')
const compiled = ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
})

function deferred() {
  let resolve
  const promise = new Promise(accept => { resolve = accept })
  return { promise, resolve }
}

function scenario(pages, failures = {}, gates = {}) {
  const stack = [...pages]
  const notices = []
  const calls = { back: 0, home: 0 }
  let now = 1000
  const loaded = new Module(sourcePath, module)
  loaded.require = (id) => {
    if (id === '../test-clock') return () => now
    assert.equal(id, '../services/taro')
    return { default: {
      async navigateBack({ delta }) {
        calls.back++
        assert.equal(delta, 1)
        if (gates.back) await gates.back.promise
        if (failures.back || stack.length <= 1) throw new Error('navigateBack:fail cannot navigate back at first page')
        stack.pop()
      },
      async reLaunch({ url }) {
        calls.home++
        if (gates.home) await gates.home.promise
        if (failures.home) throw new Error('reLaunch:fail temporary navigation failure')
        stack.splice(0, stack.length, { path: url })
      },
      async showToast(options) {
        if (failures.toast) throw new Error('showToast:fail')
        notices.push(options)
      }
    } }
  }
  loaded._compile("const Date = { now: require('../test-clock') };\n" + compiled.outputText, sourcePath)
  return { back: loaded.exports.navigateBackOrHome, guardMs: loaded.exports.RETURN_TRANSITION_GUARD_MS, stack, notices, failures, calls, advance: ms => { now += ms } }
}

async function run() {
  const sharedPost = { path: '/pages/post-detail/index?id=1' }
  const direct = scenario([sharedPost])
  await direct.back()
  assert.deepEqual(direct.stack, [{ path: '/pages/home/index' }], '分享直达详情必须能回到首页，且清除原详情页')
  assert.deepEqual(direct.notices, [])

  const previousPage = { path: '/pages/home/index', scrollTop: 840, activeTab: 'recommend' }
  const internal = scenario([previousPage, sharedPost])
  await internal.back()
  assert.deepEqual(internal.stack, [previousPage])
  assert.equal(internal.stack[0], previousPage, '站内返回必须保留原页面及其滚动、筛选状态')

  const searchPage = { path: '/pages/search/index', query: '图书馆' }
  const nested = scenario([previousPage, searchPage, sharedPost])
  await nested.back()
  assert.deepEqual(nested.stack, [previousPage, searchPage], '多层进入应仅返回一页')

  const delayed = deferred()
  const rapid = scenario([previousPage, searchPage, sharedPost], {}, { back: delayed })
  const firstReturn = rapid.back()
  assert.equal(rapid.back(), firstReturn, '原生返回未完成时应复用同一任务')
  assert.equal(rapid.calls.back, 1)
  delayed.resolve()
  await firstReturn
  await rapid.back()
  assert.equal(rapid.calls.back, 1, '原生success提前到达不能让同一轮连点再退一层')
  rapid.advance(rapid.guardMs - 1)
  await rapid.back()
  assert.equal(rapid.calls.back, 1, '短过渡保护期应完整保留')
  rapid.advance(1)
  await rapid.back()
  assert.equal(rapid.calls.back, 2, '原生过渡结束后允许用户继续逐层返回')
  assert.deepEqual(rapid.stack, [previousPage])

  const homeGate = deferred()
  const fallbackRapid = scenario([sharedPost], {}, { home: homeGate })
  const goingHome = fallbackRapid.back()
  await Promise.resolve()
  assert.equal(fallbackRapid.back(), goingHome, '首页兜底也应保持同一返回任务')
  assert.equal(fallbackRapid.calls.home, 1)
  homeGate.resolve()
  await goingHome
  await fallbackRapid.back()
  assert.equal(fallbackRapid.calls.home, 1, '首页成功后仍需要覆盖原生退场的防重保护')

  const failedBack = scenario([previousPage, sharedPost], { back: true })
  await failedBack.back()
  assert.deepEqual(failedBack.stack, [{ path: '/pages/home/index' }], '页面栈存在但返回失败时仍要提供首页出口')

  const retry = scenario([sharedPost], { home: true })
  await assert.doesNotReject(retry.back())
  assert.deepEqual(retry.stack, [sharedPost])
  assert.equal(retry.notices.length, 1)
  assert.equal(retry.notices[0].title, '暂时无法返回，请重试')
  retry.failures.home = false
  await retry.back()
  assert.deepEqual(retry.stack, [{ path: '/pages/home/index' }], '导航临时失败后仍可再次返回')

  const failedNotice = scenario([sharedPost], { home: true, toast: true })
  await assert.doesNotReject(failedNotice.back(), '提示失败不得产生未处理的异步错误')

  const pageSource = fs.readFileSync(path.join(root, 'apps/miniapp/src/pages/post-detail/index.tsx'), 'utf8')
  assert.match(pageSource, /className='post-detail-nav__back'\s+onClick=\{\(\) => void navigateBackOrHome\(\)\}/, '帖子顶部返回入口必须接入被验证的导航行为')
  for (const page of ['about', 'legal', 'campus-guide']) {
    const source = fs.readFileSync(path.join(root, `apps/miniapp/src/pages/${page}/index.tsx`), 'utf8')
    assert.match(source, /navigateBackOrHome\(\)/, `${page}返回应接入已验证的共用行为`)
  }
  console.log('分享返回检查通过：单页直达、状态保留、多层返回、并发去重、原生动画期防重、兜底与失败后立即重试。')
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
