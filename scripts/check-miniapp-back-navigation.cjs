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

function scenario(pages, failures = {}) {
  const stack = [...pages]
  const notices = []
  const loaded = new Module(sourcePath, module)
  loaded.require = (id) => {
    assert.equal(id, '../services/taro')
    return { default: {
      async navigateBack({ delta }) {
        assert.equal(delta, 1)
        if (failures.back || stack.length <= 1) throw new Error('navigateBack:fail cannot navigate back at first page')
        stack.pop()
      },
      async reLaunch({ url }) {
        if (failures.home) throw new Error('reLaunch:fail temporary navigation failure')
        stack.splice(0, stack.length, { path: url })
      },
      async showToast(options) {
        if (failures.toast) throw new Error('showToast:fail')
        notices.push(options)
      }
    } }
  }
  loaded._compile(compiled.outputText, sourcePath)
  return { back: loaded.exports.navigateBackOrHome, stack, notices, failures }
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
  console.log('分享返回检查通过：单页直达、原页面状态保留、多层返回、导航失败兜底与重试。')
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
