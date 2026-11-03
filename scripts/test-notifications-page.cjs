const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function fixture(communityOverride = {}) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(name => [name, Object.getOwnPropertyDescriptor(globalThis, name)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const lifecycle = {}
  const state = { token: 'token-1', identity: { currentIdentity: 'student' } }
  const reads = [], navigations = [], toasts = [], refreshes = []
  const communityItem = { id: 'same-id', type: 'post_commented', actor: '同学', postId: 'post-1', read: false, createdAt: '2026-11-03T00:00:00Z', ...communityOverride }
  const deliveryItem = { id: 'same-id', title: '订单更新', summary: '配送中', read: false, targetPath: '/pages/delivery-rider/index' }
  const sessionStore = selector => selector(state)
  sessionStore.getState = () => state
  const markRead = kind => id => {
    const pending = deferred(); reads.push({ kind, id, pending }); return pending.promise
  }
  const SessionGate = () => null
  const imports = {
    '../../components/feature-page-gate': { withFeaturePage: (Page, gate) => {
      assert.equal(gate, SessionGate, '通知中心必须使用真实会话守卫'); return Page
    } },
    '../../components/runtime-session-gate': { default: SessionGate },
    '@tarojs/components': {
      View: ({ children, ...props }) => React.createElement('div', props, children),
      Text: ({ children, ...props }) => React.createElement('span', props, children),
    },
    '@tarojs/taro': {
      useDidHide: callback => { lifecycle.hide = callback },
      useDidShow: callback => { lifecycle.show = callback },
    },
    '../../services/b-community': {
      listCommunityNotifications: async () => ({ items: [communityItem], unread: 1 }),
      markCommunityNotificationRead: markRead('community'),
    },
    '../../services/notification': {
      loadNotificationInbox: async () => ({ items: [deliveryItem], nextCursor: '', unread: 1 }),
      markNotificationRead: markRead('delivery'),
      notificationTargetPath: item => item.targetPath,
    },
    '../../services/notification-runtime': { notificationRuntime: { refresh: async () => { refreshes.push(true) } } },
    '../../stores/session': { useSessionStore: sessionStore },
    '../../services/taro': { default: {
      navigateTo: async value => { navigations.push(value) },
      showToast: async value => { toasts.push(value) },
    } },
    './index.scss': {},
  }
  const filename = path.resolve(__dirname, '../apps/miniapp/src/pages/notifications/index.tsx')
  const loaded = new Module(filename, module)
  loaded.filename = filename
  loaded.require = id => {
    assert.ok(id === 'react' || id === 'react/jsx-runtime' || id in imports, `测试必须显式隔离依赖：${id}`)
    return id === 'react' || id === 'react/jsx-runtime' ? require(id) : imports[id]
  }
  loaded._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 },
    fileName: filename,
  }).outputText, filename)
  const container = document.createElement('main')
  document.body.appendChild(container)
  const root = createRoot(container)
  const act = callback => React.act(async () => { await callback() })
  const flush = async () => { await act(async () => { await new Promise(setImmediate) }) }
  const render = async () => { await act(async () => root.render(React.createElement(loaded.exports.default))) }
  const item = kind => container.querySelectorAll('.notification')[kind === 'community' ? 0 : 1]
  let cleaned = false
  const cleanup = async () => {
    if (cleaned) return
    cleaned = true
    await act(async () => root.unmount())
    container.remove(); dom.window.close()
    for (const [name, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor)
      else delete globalThis[name]
    }
  }
  return { lifecycle, state, reads, navigations, toasts, refreshes, render, flush, item, cleanup, act }
}

async function run() {
  for (const commentId of ['comment&other=value', undefined]) {
    const app = fixture({ type: 'product_commented', productId: 'product?x=1', productCommentId: commentId })
    try {
      await app.render(); app.item('community').click(); await app.flush()
      app.reads[0].pending.resolve(); await app.flush()
      assert.deepEqual(app.navigations, [{ url: `/pages/secondhand-detail/index?id=product%3Fx%3D1&commentId=${encodeURIComponent(commentId || '')}` }])
    } finally { await app.cleanup() }
  }
  console.log('PASS 商品通知保留商品与评论坐标并逐个编码，缺评论坐标安全回退')
  for (const kind of ['community', 'delivery']) {
    for (const boundary of ['hide', 'unmount', 'account', 'role']) {
      const app = fixture()
      try {
        await app.render()
        app.item(kind).click(); await app.flush()
        assert.equal(app.reads.length, 1)
        if (boundary === 'hide') app.lifecycle.hide()
        if (boundary === 'unmount') await app.cleanup()
        if (boundary === 'account') app.state.token = 'token-2'
        if (boundary === 'role') app.state.identity.currentIdentity = 'merchant'
        app.reads[0].pending.resolve()
        if (boundary === 'unmount') await new Promise(setImmediate)
        else await app.flush()
        assert.equal(app.navigations.length, 0, `${kind} ${boundary} 后旧已读结果不得跳转`)
        assert.equal(app.refreshes.length, 0, `${kind} ${boundary} 后不得刷新新上下文`)
        assert.equal(app.toasts.length, 0)
      } finally { await app.cleanup() }
      console.log(`PASS ${kind} ${boundary} 后丢弃旧点击结果`)
    }
    {
      const app = fixture()
      try {
        await app.render()
        app.item(kind).click(); app.item(kind).click(); await app.flush()
        assert.equal(app.reads.length, 1, `${kind} 连点只读一次`)
        app.reads[0].pending.resolve(); await app.flush()
        assert.equal(app.navigations.length, 1)
        assert.equal(app.item(kind).classList.contains('notification--unread'), false, '已读完成立即更新当前卡片')
      } finally { await app.cleanup() }
      console.log(`PASS ${kind} 连点只导航一次并更新已读状态`)
    }
    {
      const app = fixture()
      try {
        await app.render(); app.item(kind).click(); await app.flush()
        app.reads[0].pending.reject(new Error('网络失败')); await app.flush()
        assert.equal(app.toasts.length, 1)
        app.item(kind).click(); await app.flush()
        assert.equal(app.reads.length, 2)
        app.reads[1].pending.resolve(); await app.flush()
        assert.equal(app.navigations.length, 1)
      } finally { await app.cleanup() }
      console.log(`PASS ${kind} 已读失败后释放点击锁并可重试`)
    }
    {
      const app = fixture()
      try {
        await app.render(); app.item(kind).click(); await app.flush()
        app.lifecycle.hide()
        app.reads[0].pending.reject(new Error('网络失败')); await app.flush()
        assert.equal(app.toasts.length, 0, '隐藏页迟到错误不得在其他页面弹提示')
        await app.act(async () => app.lifecycle.show())
        app.item(kind).click(); await app.flush()
        assert.equal(app.reads.length, 2)
        app.reads[1].pending.resolve(); await app.flush()
        assert.equal(app.navigations.length, 1)
      } finally { await app.cleanup() }
      console.log(`PASS ${kind} 隐藏错误不弹窗，重新显示后可操作`)
    }
  }
  console.log('通知中心生命周期与真实会话守卫：PASS')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
