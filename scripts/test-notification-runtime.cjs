// Executes real runtime/SDK adapter functions with isolated accounts, fake SDK and fake clocks.
const assert = require('node:assert/strict')
const { loadSource, hookRenderer, deferred } = require('./test-delivery-stage-workbench.cjs')

const renderer = hookRenderer()
const notification = loadSource('services/notification.ts', { './api': { request: async () => ({}) }, './taro': {} }, renderer)
let conversationRows = [{ conversationID: 'C2Ccampus_2', unreadCount: 2 }]
const sdkListeners = new Map()
const chat = {
  getConversationList: async () => {
    // Some SDK versions emit list events after the read API itself.
    sdkListeners.get('conversation')?.({ data: conversationRows.slice(0, 1) })
    sdkListeners.get('conversation')?.({ data: conversationRows.slice(1) })
    return { data: { conversationList: conversationRows } }
  },
  on: (event, listener) => sdkListeners.set(event, listener),
  off: (event, listener) => { if (sdkListeners.get(event) === listener) sdkListeners.delete(event) }
}
const moduleRuntime = loadSource('services/notification-runtime.ts', {
  '@tencentcloud/chat': { EVENT: { CONVERSATION_LIST_UPDATED: 'conversation', SDK_READY: 'ready' } },
  './notification': notification,
  './e-im': {
    ensureTencentIMReady: async () => chat, getIMCredentials: async () => ({}),
    listIMConversationPage: async () => ({ items: [{ conversationId: 'C2Ccampus_2' }, { conversationId: 'C2Ccampus_3' }], page: 1, pageSize: 100, total: 2 }),
    listenTencentMessageEvents: () => () => {}, withTimeout: promise => promise
  },
  './delivery-im': { listDeliveryConversations: async () => ({ items: [], page: 1, pageSize: 100, total: 0 }) },
  './feature-visibility': { getFeatureVisibility: async () => ({ messages: true }) },
  './api': { request: async () => ({}) }
}, renderer)

async function drain() { for (let i = 0; i < 8; i++) await Promise.resolve() }

async function run() {
  const original = { setTimeout, clearTimeout, setInterval, clearInterval }
  const timers = new Map(), intervals = new Map()
  let serial = 0
  global.setTimeout = fn => { const id = ++serial; timers.set(id, fn); return id }
  global.clearTimeout = id => timers.delete(id)
  global.setInterval = fn => { const id = ++serial; intervals.set(id, fn); return id }
  global.clearInterval = id => intervals.delete(id)
  const tick = async () => { const pending = [...timers.values()]; timers.clear(); pending.forEach(fn => fn()); await drain() }
  const ownerA = { key: 'secret-test-account-a', userId: '1', role: 'student' }
  const ownerB = { key: 'secret-test-account-b', userId: '2', role: 'rider' }
  const inbox = () => Promise.resolve({ items: [], nextCursor: '', unread: 0 })
  try {
    let calls = 0, binds = 0, unbinds = 0, changed, response = deferred()
    const runtime = moduleRuntime.createNotificationRuntime({
      loadCounts: () => { calls++; return response.promise }, loadInbox: inbox,
      listen: async (_, listener) => { binds++; changed = listener; return () => { unbinds++ } }
    })
    runtime.start(ownerA); runtime.start(ownerA)
    await drain()
    assert.equal(binds, 1, 'same account must not register another listener')
    assert.equal(calls, 1)
    assert.ok(!JSON.stringify(runtime.getState()).includes('secret-test'), 'public state must never contain the owner token')
    const oldChanged = changed
    const previous = response
    response = deferred()
    runtime.start(ownerB)
    await drain()
    previous.resolve({ old: 99 }); response.resolve({ current: 3 })
    await drain()
    assert.equal(runtime.getState().unread.total, 3)
    assert.deepEqual(Object.keys(runtime.getState().unread.conversations), ['current'])
    const revision = runtime.getState().messageRevision
    oldChanged(); await tick()
    assert.equal(runtime.getState().messageRevision, revision, 'old listeners cannot mutate the new account')
    assert.equal(unbinds, 1)

    response = deferred(); runtime.foreground(false)
    changed(); intervals.forEach(fn => fn()); await tick()
    assert.equal(calls, 2, 'background events and poll timers must perform no IO')
    runtime.foreground(true); await drain()
    assert.equal(calls, 3)
    const hiddenReply = response
    runtime.foreground(false)
    response = deferred(); runtime.foreground(true)
    hiddenReply.resolve({ hiddenOld: 80 }); response.resolve({ fresh: 4 }); await drain()
    assert.equal(runtime.getState().unread.total, 4, 'a hidden request cannot win over a new foreground request')

    const beforeRole = runtime.getState().unread.generation
    response = deferred(); runtime.start({ ...ownerB, role: 'merchant' })
    await drain()
    assert.ok(runtime.getState().unread.generation > beforeRole)
    assert.equal(runtime.getState().unread.total, null, 'role change must clear old counts')
    response.reject(Error('offline')); await drain()
    assert.equal(runtime.getState().unread.total, null, 'failed snapshot is unknown rather than zero')
    runtime.stop(); await drain()
    assert.equal(intervals.size, 0)
    assert.equal(runtime.getState().ownerKey, '')

    let resolves = deferred(), stoppedLate = 0
    const late = moduleRuntime.createNotificationRuntime({ loadCounts: async () => ({}), loadInbox: inbox, listen: () => resolves.promise })
    late.start(ownerA); late.stop(); resolves.resolve(() => { stoppedLate++ }); await drain()
    assert.equal(stoppedLate, 1, 'late listener setup must be immediately disposed after logout')

    const ports = moduleRuntime.notificationRuntimePorts
    await assert.rejects(ports.loadCounts(ownerA), /快照尚未完整/, 'missing SDK rows must not become invented zeros')
    conversationRows = [{ conversationID: 'C2Ccampus_2', unreadCount: 2 }, { conversationID: 'C2Ccampus_3', unreadCount: -1 }]
    await assert.rejects(ports.loadCounts(ownerA), /快照无效/)
    conversationRows[1].unreadCount = 0
    let notifications = 0
    const stop = await ports.listen(ownerA, () => { notifications++ })
    await ports.loadCounts(ownerA)
    const first = notifications
    assert.equal(first, 2)
    await ports.loadCounts(ownerA); await ports.loadCounts(ownerA)
    assert.equal(notifications, first, 'reading identical SDK chunked snapshots must not create a feedback loop')
    stop()
    assert.equal(sdkListeners.size, 0)

    assert.equal(notification.notificationTargetPath({ targetPath: 'https://attacker.invalid' }), null)
    assert.equal(notification.notificationTargetPath({ targetPath: '/pages/admin/index' }), null)
    assert.equal(notification.notificationTargetPath({ targetPath: '/pages/merchant-order-detail/index?id=public-id' }), '/pages/merchant-order-detail/index?id=public-id')

    const context = { relationId: 1, conversationId: 'C2Ccampus_2', peerUserId: 'campus_2', buyerId: 1, sellerId: 2 }
    const imEvents = new Map()
    let readCalls = 0, authWait = null
    const oldMessage = { ID: 'old', conversationID: context.conversationId, from: 'campus_2', to: 'campus_1' }
    const imChat = {
      setLogLevel() {}, isReady: () => true, getLoginUser: () => 'campus_1',
      getConversationProfile: async () => ({ data: {} }),
      getMessageList: async () => ({ data: { messageList: [oldMessage], isCompleted: true } }),
      setMessageRead: async () => { readCalls++ }, destroy: async () => {},
      on: (event, fn) => imEvents.set(event, fn), off: (event, fn) => { if (imEvents.get(event) === fn) imEvents.delete(event) }
    }
    const im = loadSource('services/e-im.ts', {
      '@tencentcloud/chat': { create: () => imChat, EVENT: { MESSAGE_RECEIVED: 'messages' } },
      './feature-visibility': { getFeatureVisibility: async () => ({ messages: true, secondhand_chat: true }) },
      './api': { request: async url => url === '/im/usersig' ? { sdkAppId: '1', userId: 'campus_1', userSig: 'test-only' } : authWait ? authWait.promise : context }
    }, renderer)
    await im.loadTencentMessages(context, 'campus_1')
    assert.equal(readCalls, 0, 'fetching history is never a read receipt')
    const stopMessages = await im.subscribeTencentMessages(context, () => {}, 'campus_1')
    const stopAppMessages = im.listenTencentMessageEvents(imChat, () => {})
    assert.equal(imEvents.size, 1, 'page and app share the same physical SDK message listener')
    authWait = deferred()
    const reading = im.markTencentConversationRead(context, 'campus_1', () => true, ['old'])
    imEvents.get('messages')({ data: [{ ...oldMessage, ID: 'new-not-rendered' }] })
    authWait.resolve(context); await reading; authWait = null
    assert.equal(readCalls, 0, 'new incoming messages not rendered while auth awaits cannot be marked read')
    await im.markTencentConversationRead(context, 'campus_1', () => true, ['old', 'new-not-rendered'])
    assert.equal(readCalls, 1)
    await im.markTencentConversationRead(context, 'campus_1', () => false, ['old', 'new-not-rendered'])
    assert.equal(readCalls, 1, 'hidden conversations never mark read')
    stopMessages(); assert.equal(imEvents.size, 1)
    stopAppMessages(); assert.equal(imEvents.size, 0)

    const hook = hookRenderer()
    let hideHook, showHook, key = 'a', receiptGate = deferred(), hookReads = 0
    const useRead = loadSource('hooks/use-visible-conversation-read.ts', {
      '@tarojs/taro': { useDidHide: fn => { hideHook = fn }, useDidShow: fn => { showHook = fn } }
    }, hook).useVisibleConversationRead
    hook.setComponent(() => useRead(key, key, async visible => { await receiptGate.promise; if (visible()) hookReads++ }, true))
    hook.render(); hideHook(); receiptGate.resolve(); await drain()
    assert.equal(hookReads, 0)
    receiptGate = deferred(); showHook(); hook.render(); key = 'b'; hook.render()
    receiptGate.resolve(); await drain()
    assert.equal(hookReads, 1, 'the prior rendered account effect is invalidated before the new account receipt')
    hook.unmount()
    console.log('PASS 通知运行时：唯一监听、切号/角色/隐藏隔离、迟到响应、失败未知、完整快照、SDK反馈去重、跳转白名单')
    console.log('PASS 已读边界：读取历史不标记、页面与App共享SDK监听、未渲染新消息/隐藏/旧账号不标已读')
  } finally {
    Object.assign(global, original)
  }
}

run().catch(error => { console.error(error); process.exitCode = 1 })
