const assert = require('node:assert/strict')
const { loadSource, hookRenderer, deferred } = require('./test-delivery-stage-workbench.cjs')
const original = { accountId: 'A', amountCents: 10000, clientRequestId: 'ORIGINAL', paymentPublicId: 'PAY-A', createdAt: 123 }
function harness(options = {}) {
  const calls = []
  let active = true, cancel = false
  const payment = { paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'prepay_issued', wechat: { package: 'original-prepay' }, ...options.payment }
  const api = loadSource('pages/merchant-wallet/wallet-recharge.ts', {
    '../../services/delivery-payment': {
      getDeliveryPayment: async id => { calls.push(['get', id]); return options.get ? options.get() : payment },
      getDeliveryWalletRechargeByRequest: async (id, amount) => { calls.push(['getByRequest', id, amount]); return options.getByRequest ? options.getByRequest() : payment },
      startDeliveryWalletRecharge: async (amount, id) => { calls.push(['start', amount, id]); return options.start ? options.start() : { payment } },
      closeDeliveryWalletRecharge: async id => { calls.push(['close', id]); return options.close ? options.close() : { ...payment, status: 'closed' } },
      closeDeliveryWalletRechargeByRequest: async (id, amount) => { calls.push(['closeByRequest', id, amount]); return options.close ? options.close() : { ...payment, status: 'closed' } },
      pollDeliveryPayment: async id => { calls.push(['poll', id]); return options.poll ? options.poll() : { ...payment, status: options.pending ? 'prepay_issued' : 'paid' } },
    },
    './wallet-state': { savePendingWalletRecharge: value => { calls.push(['save', value]) } },
  }, hookRenderer())
  return { calls, stop: () => { active = false }, cancel: () => { cancel = true }, run: (pending = original, verifyOnly = false, cancelOnly = false) => api.runWalletRecharge({
    pending, verifyOnly, cancelOnly, isCurrent: () => active, shouldCancel: () => cancel,
    onCancelRequested: () => { calls.push(['dismiss']) },
    requestPayment: async parameters => { calls.push(['native', parameters]); return options.native?.() },
  }) }
}

async function run() {
  const fresh = harness()
  assert.equal((await fresh.run({ ...original, paymentPublicId: undefined })).status, 'paid')
  assert.deepEqual(fresh.calls.find(c => c[0] === 'start'), ['start', 10000, 'ORIGINAL'])
  assert.equal(fresh.calls.find(c => c[0] === 'save')[1].wechat, undefined)
  console.log('PASS 充值只持久化账号隔离恢复标识，不保存微信签名')

  const resume = harness()
  await resume.run()
  assert.equal(resume.calls[0][0], 'get')
  assert.deepEqual(resume.calls.find(c => c[0] === 'start'), ['start', 10000, 'ORIGINAL'])
  assert.equal(resume.calls.filter(c => c[0] === 'native').length, 1)
  console.log('PASS 原支付单按同一幂等键恢复，不另建充值请求')

  for (const status of ['paid', 'closed', 'failed']) {
    const terminal = harness({ payment: { status } })
    assert.equal((await terminal.run()).status, status)
    assert.equal(terminal.calls.filter(c => ['start', 'native'].includes(c[0])).length, 0)
  }
  const verify = harness({ pending: true })
  await verify.run(original, true)
  assert.equal(verify.calls.some(c => c[0] === 'start' || c[0] === 'native'), false)
  console.log('PASS 终态只核实原单；主动查询不弹付款、不创建新的充值单')

  const closing = deferred()
  const cancelled = harness({ native: () => { throw { errMsg: 'requestPayment:fail cancel' } }, close: () => closing.promise })
  const cancelledRun = cancelled.run({ ...original, paymentPublicId: undefined })
  await new Promise(resolve => setImmediate(resolve))
  assert.ok(cancelled.calls.some(c => c[0] === 'dismiss'), '等待服务端之前立即收起弹窗')
  assert.equal(cancelled.calls.filter(c => c[0] === 'save').at(-1)[1].cancelRequested, true)
  closing.resolve({ paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'closed' })
  assert.equal((await cancelledRun).status, 'closed')
  assert.equal(cancelled.calls.filter(c => c[0] === 'native').length, 1)
  console.log('PASS 微信取消立即收起弹窗并持久化取消意图，官方关单后返回终态')

  for (const marker of [{ cancelRequested: true }, {}]) {
    const restored = harness(marker.cancelRequested ? {} : { payment: { cancelRequestedAt: '2026-10-30T14:00:00Z' } })
    assert.equal((await restored.run({ ...original, ...marker }, true)).status, 'closed')
    assert.equal(restored.calls.some(c => ['start', 'native'].includes(c[0])), false)
    assert.equal(restored.calls.filter(c => c[0] === 'close').length, 1)
  }
  console.log('PASS 本地或服务端已有取消意图时只能核验关单，绝不重新付款')

  for (const status of ['paid', 'prepay_issued']) {
    const race = harness({ close: async () => ({ paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status, providerStatus: status === 'paid' ? 'SUCCESS' : 'USERPAYING' }) })
    assert.equal((await race.run(original, false, true)).status, status)
    assert.equal(race.calls.some(c => c[0] === 'native'), false)
  }
  const unknown = harness({ close: async () => { throw new Error('timeout') } })
  await assert.rejects(unknown.run(original, false, true), /timeout/)
  assert.equal(unknown.calls.filter(c => c[0] === 'save').at(-1)[1].cancelRequested, true)
  console.log('PASS 关单时已付竞态保留paid、处理中保留未知、超时保留取消记录')

  const lostResponse = harness()
  await lostResponse.run({ ...original, paymentPublicId: undefined, cancelRequested: true }, true)
  assert.deepEqual(lostResponse.calls.find(c => c[0] === 'closeByRequest'), ['closeByRequest', 'ORIGINAL', 10000])
  assert.equal(lostResponse.calls.some(c => ['start', 'get', 'native'].includes(c[0])), false)
  const queryMissingID = harness()
  await queryMissingID.run({ ...original, paymentPublicId: undefined }, true)
  assert.deepEqual(queryMissingID.calls.find(c => c[0] === 'getByRequest'), ['getByRequest', 'ORIGINAL', 10000])
  assert.equal(queryMissingID.calls.some(c => ['start', 'native', 'closeByRequest'].includes(c[0])), false)
  const notFound = harness({ getByRequest: async () => null })
  await assert.rejects(notFound.run({ ...original, paymentPublicId: undefined }, true), /尚未确认/)
  assert.equal(notFound.calls.some(c => ['start', 'native'].includes(c[0])), false)
  console.log('PASS 缺ID取消使用原请求键安全关单，查询缺单不创建订单或预支付')

  const createPending = deferred()
  const creating = harness({ start: () => createPending.promise })
  const creatingRun = creating.run({ ...original, paymentPublicId: undefined })
  creating.cancel()
  createPending.resolve({ payment: { paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'prepay_issued' } })
  assert.equal((await creatingRun).status, 'closed')
  assert.equal(creating.calls.some(c => c[0] === 'native'), false)
  const pollPending = deferred()
  const polling = harness({ payment: { providerStatus: 'USERPAYING' }, poll: () => pollPending.promise })
  const pollingRun = polling.run()
  await new Promise(resolve => setImmediate(resolve))
  polling.cancel()
  pollPending.resolve({ paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'prepay_issued', providerStatus: 'USERPAYING' })
  assert.equal((await pollingRun).status, 'closed')
  assert.equal(polling.calls.some(c => c[0] === 'native'), false)
  console.log('PASS 创建与微信处理中核验期间取消，后续流程只关单不拉起付款')

  const wrongClose = harness({ close: async () => ({ paymentPublicId: 'WRONG', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'closed' }) })
  await assert.rejects(wrongClose.run({ ...original, cancelRequested: true }), /一致/)
  for (const changed of [{ amountCents: 20000 }, { purpose: 'order_fee' }, { provider: 'mock' }, { paymentPublicId: 'ANOTHER' }]) {
    const wrongOrder = harness({ payment: changed })
    await assert.rejects(wrongOrder.run(), /金额|一致|微信/)
    assert.equal(wrongOrder.calls.some(c => ['close', 'native'].includes(c[0])), false)
  }
  console.log('PASS 拒绝关其他ID/用途/金额/渠道订单，关单响应ID不一致不得确认终态')

  const userpaying = harness({ payment: { providerStatus: 'USERPAYING' } })
  await userpaying.run()
  assert.equal(userpaying.calls.some(c => c[0] === 'native'), false)
  const mismatch = harness({ payment: { amountCents: 20000 } })
  await assert.rejects(mismatch.run(), /金额|一致/)
  assert.equal(mismatch.calls.some(c => c[0] === 'native'), false)
  console.log('PASS 微信处理中只查单；金额不一致拒绝拉起付款')

  const waiting = deferred()
  const interrupted = harness({ get: () => waiting.promise })
  const running = interrupted.run()
  const rejected = assert.rejects(running, /会话|页面/)
  interrupted.stop(); waiting.resolve({ status: 'prepay_issued' })
  await rejected
  assert.equal(interrupted.calls.length, 1)
  console.log('PASS 查询期间换账号或卸载，不继续预支付或弹旧账号付款')

  const closePending = deferred()
  const leaveWhileClosing = harness({ close: () => closePending.promise })
  const closeRun = leaveWhileClosing.run(original, false, true)
  assert.equal(leaveWhileClosing.calls.some(c => c[0] === 'get'), false)
  assert.equal(leaveWhileClosing.calls.some(c => c[0] === 'close'), true, '取消请求在退出前同步发起')
  const closeRejected = assert.rejects(closeRun, /会话|页面/)
  leaveWhileClosing.stop(); closePending.resolve({ status: 'closed' }); await closeRejected
  console.log('PASS 已知ID取消直接提交服务端，页面退出不会使请求卡在预先查单')

  const map = new Map()
  const state = loadSource('pages/merchant-wallet/wallet-state.ts', { '../../services/taro': {
    getStorageSync: key => map.get(key), setStorageSync: (key, value) => map.set(key, value), removeStorageSync: key => map.delete(key),
  } }, hookRenderer())
  state.savePendingWalletRecharge(original)
  assert.equal(state.readPendingWalletRecharge('B'), null)
  for (const corrupt of [{ ...original, amountCents: -1 }, { ...original, amountCents: NaN }, { ...original, clientRequestId: '' }, { ...original, accountId: 'B' }, { ...original, cancelRequested: 'yes' }]) {
    map.set('delivery.wallet.recharge.A', corrupt)
    assert.equal(state.readPendingWalletRecharge('A'), null)
  }
  console.log('PASS 恢复缓存校验金额与账户，不读入损坏或其他账号记录')

  const requests = []
  const service = loadSource('services/delivery-payment.ts', {
    './api': { request: async (...args) => { requests.push(args); return { status: 'closed' } } },
    './delivery-merchant': { createDeliveryClientRequestId: () => 'TEST' },
  }, hookRenderer())
  await service.closeDeliveryWalletRecharge('PAY/SAFE')
  assert.equal(requests[0][0], '/delivery/merchant/wallet/recharges/PAY%2FSAFE/close')
  assert.equal(requests[0][1].method, 'POST')
  assert.equal(requests[0][1].data, undefined)
  await service.closeDeliveryWalletRechargeByRequest('REQUEST', 123)
  assert.equal(requests[1][0], '/delivery/merchant/wallet/recharges/close')
  assert.deepEqual(requests[1][1].data, { clientRequestId: 'REQUEST', amountCents: 123 })
  await service.getDeliveryWalletRechargeByRequest('REQ/SAFE', 123)
  assert.equal(requests[2][0], '/delivery/merchant/wallet/recharges/by-request/REQ%2FSAFE?amountCents=123')
  console.log('PASS 关单服务使用编码订单ID和无body POST契约，不提交客户端余额')
}
run().catch(error => { console.error(error); process.exitCode = 1 })
