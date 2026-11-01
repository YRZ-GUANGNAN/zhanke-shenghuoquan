// Isolated service tests. No login, account data or external network IO.
const assert = require('node:assert/strict')
const { loadSource, hookRenderer, deferred } = require('./test-delivery-stage-workbench.cjs')

async function run() {
  let account = 'test-account-a', calls = [], nativeCalls = 0, nextNative = null, reportFails = false
  const taro = {
    getStorageSync: () => account,
    requestSubscribeMessage: options => {
      nativeCalls++
      assert.deepEqual(Array.from(options.tmplIds), ['unit-test-template'])
      return nextNative ? nextNative.promise : Promise.resolve({ 'unit-test-template': 'accept' })
    }
  }
  const api = {
    request: async (url, options = {}) => {
      calls.push({ url, options })
      if (url === '/notification/subscribe/templates') return { items: [{ templateKey: 'order', templateId: 'unit-test-template', available: true }] }
      if (url === '/notification/subscribe/nonce') return { nonce: 'unit-nonce', expiresAt: new Date(Date.now() + 60000).toISOString() }
      if (url === '/notification/subscribe/authorize') { if (reportFails) throw Error('offline'); return {} }
      if (url.startsWith('/notification/inbox?')) return { items: [{ id: 'one', read: false }], total: 200 }
      throw Error('unexpected test request')
    }
  }
  const service = loadSource('services/notification.ts', { './api': api, './taro': taro }, hookRenderer())
  loadSource('services/notification.test.ts', { './notification': service }, hookRenderer()).runNotificationUnreadSnapshotTests()
  assert.equal((await service.loadNotificationInbox()).unread, null, 'page length is not the global unread total')

  const prepared = await service.prepareNotificationSubscription('order')
  assert.equal(nativeCalls, 0, 'preparation must not open authorization')
  const networkCallsBeforeClick = calls.length
  const response = service.requestNotificationSubscription(prepared)
  assert.equal(nativeCalls, 1, 'native API must run synchronously inside the click, before any network await')
  assert.equal(calls.length, networkCallsBeforeClick)
  assert.equal((await response).reportStatus, 'recorded')
  assert.equal((await service.requestNotificationSubscription(prepared)).status, 'unavailable')
  assert.equal(nativeCalls, 1, 'double click must not request twice')

  const pending = await service.prepareNotificationSubscription('order')
  nextNative = deferred()
  const delayed = service.requestNotificationSubscription(pending)
  const count = calls.length
  account = 'test-account-b'
  nextNative.resolve({ 'unit-test-template': 'accept' })
  assert.equal((await delayed).reportStatus, 'not_sent')
  assert.equal(calls.length, count, 'a stale authorization must not be written under another account')

  nextNative = null
  const unsynced = await service.prepareNotificationSubscription('order')
  reportFails = true
  assert.equal((await service.requestNotificationSubscription(unsynced)).reportStatus, 'failed')
  assert.ok(calls.every(call => !JSON.stringify(call.options).includes('test-account')), 'API data must not contain session credentials')
  console.log('PASS 通知：未知未读、完整快照替换、去重、代次隔离、原生点击授权、nonce单次消费、失败不假成功')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
