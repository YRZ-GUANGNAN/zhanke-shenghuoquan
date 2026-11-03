const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

const sourceRoot = path.resolve(__dirname, '../apps/miniapp/src')

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function fixture() {
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(name => [name, Object.getOwnPropertyDescriptor(globalThis, name)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const lifecycle = { hide: null, show: null }
  let accountId = 1
  let role = 'rider'
  let enabled = true
  const reads = []
  const navigations = []
  const toasts = []
  const inbox = [{ id: 'notice-1', title: '订单更新', summary: '配送中', read: false, createdAt: '2026-11-02T10:00:00Z', targetPath: '/pages/delivery-rider/index' }]
  const state = { token: 'token-1', identity: { currentIdentity: role }, profile: { id: accountId } }
  const readQueue = []
  const notificationState = { ownerKey: 'token-1:1:rider', inboxUnread: 1, inboxRevision: 'r1' }
  const runtime = { refresh: async () => {} }
  const loadNotificationInbox = async () => {
    const pending = deferred(); readQueue.push(pending); return pending.promise
  }
  const markNotificationRead = async id => {
    const pending = deferred(); reads.push({ id, pending }); return pending.promise
  }
  const sessionStore = selector => selector(state)
  sessionStore.getState = () => state
  const primitives = {
    View: ({ children, ...props }) => React.createElement('div', props, children),
    Text: ({ children, ...props }) => React.createElement('span', props, children),
  }
  const imports = {
    '@tarojs/components': primitives,
    '@tarojs/taro': {
      useDidHide: callback => { lifecycle.hide = callback },
      useDidShow: callback => { lifecycle.show = callback },
    },
    '../hooks/use-notification-runtime': { useNotificationState: () => notificationState },
    '../services/notification': {
      loadNotificationInbox,
      markNotificationRead,
      notificationTargetPath: item => item.targetPath || null,
    },
    '../services/notification-runtime': { notificationRuntime: runtime },
    '../stores/session': { useSessionStore: sessionStore },
    '../services/taro': { default: {
      navigateTo: async value => { navigations.push(value) },
      showToast: async value => { toasts.push(value) },
    } },
  }
  const filename = path.join(sourceRoot, 'components/notification-inbox.tsx')
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
  const Inbox = loaded.exports.default
  const container = document.createElement('main')
  document.body.appendChild(container)
  const root = createRoot(container)
  const act = callback => React.act(async () => { await callback() })
  const flush = async () => { await act(async () => { await new Promise(setImmediate) }) }
  const render = async value => { enabled = value; await act(async () => root.render(React.createElement(Inbox, { enabled: value }))) }
  const item = () => container.querySelector('.merchant-message-item')
  const cleanup = async () => {
    await act(async () => root.unmount())
    container.remove(); dom.window.close()
    for (const [name, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor)
      else delete globalThis[name]
    }
  }
  return { lifecycle, reads, navigations, toasts, readQueue, state, notificationState, render, flush, item, cleanup, setAccount: id => { accountId = id; state.profile.id = id; state.token = `token-${id}`; state.identity = { currentIdentity: role } }, setRole: value => { role = value; state.identity = { currentIdentity: value } }, inbox }
}

async function run() {
  {
    const app = fixture()
    try {
      await app.render(true)
      app.lifecycle.show()
      app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 })
      await app.flush()
      assert.ok(app.item(), '初次 show 不得废弃仍在加载的请求并留下 busy 锁')
    } finally { await app.cleanup() }
    console.log('PASS 首次 show 与初始加载交错不锁死通知列表')
  }
  {
    const app = fixture()
    await app.render(true)
    app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 })
    await app.flush()
    app.item().click(); await app.flush()
    assert.equal(app.reads.length, 1)
    app.lifecycle.hide()
    app.reads[0].pending.resolve()
    await app.flush()
    assert.equal(app.navigations.length, 0, '离开页面后迟到的已读回执不得导航')
    await app.cleanup()
    console.log('PASS hide 后旧通知点击不导航')
  }
  {
    const app = fixture()
    await app.render(true); app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 }); await app.flush()
    app.item().click(); app.item().click(); await app.flush()
    assert.equal(app.reads.length, 1, '连点同一通知只允许一个已读请求')
    app.reads[0].pending.resolve(); await app.flush()
    assert.equal(app.navigations.length, 1, '同一通知只导航一次')
    await app.cleanup(); console.log('PASS 通知连点只读一次且只导航一次')
  }
  {
    const app = fixture()
    await app.render(true); app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 }); await app.flush()
    app.item().click(); await app.flush(); await app.cleanup()
    app.reads[0].pending.resolve(); await new Promise(setImmediate)
    assert.equal(app.navigations.length, 0, '卸载后迟到回执不得导航')
    console.log('PASS 卸载后迟到通知回执被丢弃')
  }
  {
    const app = fixture()
    await app.render(true); app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 }); await app.flush()
    app.item().click(); await app.flush(); app.setAccount(2); await app.render(true)
    app.reads[0].pending.resolve(); await app.flush()
    assert.equal(app.navigations.length, 0, '切换账号后旧通知不得导航')
    await app.cleanup(); console.log('PASS 切换账号后旧通知回执被丢弃')
  }
  {
    const app = fixture()
    await app.render(true); app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 }); await app.flush()
    app.item().click(); await app.flush(); await app.render(false)
    app.reads[0].pending.resolve(); await app.flush()
    assert.equal(app.navigations.length, 0, '禁用通知列表后旧点击不得导航')
    await app.cleanup(); console.log('PASS enabled 关闭后旧通知回执被丢弃')
  }
  {
    const app = fixture()
    await app.render(true); app.readQueue[0].resolve({ items: app.inbox, nextCursor: '', unread: 1 }); await app.flush()
    app.item().click(); await app.flush(); app.reads[0].pending.reject(new Error('网络失败')); await app.flush()
    assert.equal(app.toasts.length, 1)
    app.item().click(); await app.flush()
    assert.equal(app.reads.length, 2, '失败后应允许再次点击重试')
    app.reads[1].pending.resolve(); await app.flush(); assert.equal(app.navigations.length, 1)
    await app.cleanup(); console.log('PASS 通知已读失败后可重试')
  }
  console.log('notification inbox lifecycle: PASS')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
