const assert = require('node:assert/strict')
const { loadSource, hookRenderer, deferred } = require('./test-delivery-stage-workbench.cjs')

const baseOrder = { deliveryOrderPublicId: 'ORDER-A', rowVersion: 1, status: 'pending_fee_payment', deliveryFeeCents: 350 }
const emptyFunds = { deliveryOrderPublicId: 'ORDER-A', payment: null, settlement: null, refund: null }
function setup(options = {}) {
  const calls = []
  let order = { ...baseOrder, ...options.order }, funds = { ...emptyFunds, ...options.funds }, active = true
  const record = (name, fn) => async (...args) => { calls.push([name, ...args]); return fn(...args) }
  const api = loadSource('services/delivery-order-payment.ts', {
    './delivery-merchant': { getMerchantDeliveryOrder: record('readOrder', () => options.readOrder?.() || order) },
    './delivery-payment': {
      getDeliveryOrderFunds: record('readFunds', () => options.readFunds?.() || funds),
      payDeliveryOrderWithWechat: record('wechat', () => options.wechat?.() || { payment: { paymentPublicId: 'PAY-A', provider: 'wechat', status: 'prepay_issued', wechat: { package: 'same-prepay' } } }),
      payDeliveryOrderWithWallet: record('wallet', () => {
        order = { ...order, status: 'ready_for_claim', rowVersion: 2 }
        funds = { ...funds, settlement: { status: 'paid', source: 'wallet' } }
        return { order, settlement: funds.settlement }
      }),
      requestDeliveryOrderRefund: record('refund', () => {
        order = { ...order, status: 'cancelled', rowVersion: order.rowVersion + 1 }
        funds = { ...funds, refund: { status: 'pending' }, settlement: { status: 'refund_pending', source: 'wechat' } }
        return { order, refund: funds.refund }
      }),
      pollDeliveryPayment: record('poll', () => {
        if (options.pending) return { status: 'prepay_issued' }
        order = { ...order, status: 'ready_for_claim', rowVersion: 2 }
        funds = { ...funds, settlement: { status: 'paid', source: 'wechat' } }
        return { status: 'paid' }
      }),
    },
  }, hookRenderer())
  return {
    api, calls, stop: () => { active = false },
    run: (action, overrides = {}) => api.executeManualDeliveryPayment({
      action, order: { ...baseOrder, ...options.order }, requestId: 'ORIGINAL-REQUEST',
      isCurrent: () => active,
      requestPayment: record('native', options.native || (() => {})), ...overrides,
    }),
  }
}

async function run() {
  const { api } = setup()
  assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, emptyFunds), ['wechat', 'wallet'])
  assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, null), [])
  assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, { ...emptyFunds, deliveryOrderPublicId: 'OTHER' }), [])
  for (const status of ['created', 'prepay_issued']) {
    assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, { ...emptyFunds, payment: { provider: 'wechat', status } }), ['wechat'])
  }
  assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, { ...emptyFunds, payment: { provider: 'wechat', status: 'prepay_issued', providerStatus: 'USERPAYING' } }), [])
  assert.deepEqual(api.manualDeliveryPaymentActions({ ...baseOrder, status: 'ready_for_claim' }, { ...emptyFunds, settlement: { status: 'paid' } }), ['refund'])
  assert.deepEqual(api.manualDeliveryPaymentActions({ ...baseOrder, status: 'claimed', riderUserId: 5 }, { ...emptyFunds, settlement: { status: 'paid' } }), [])
  assert.deepEqual(api.manualDeliveryPaymentActions(baseOrder, { ...emptyFunds, payment: { status: 'paid' } }), [])
  console.log('PASS 手动支付动作遵循订单和资金双重状态，未知资金不开放操作')

  const paid = setup()
  const result = await paid.run('wechat')
  assert.equal(result.funds.settlement.status, 'paid')
  assert.equal(result.order.status, 'ready_for_claim')
  assert.deepEqual(paid.calls.find(c => c[0] === 'wechat').slice(1), ['ORDER-A', 1, 'ORIGINAL-REQUEST'])
  console.log('PASS 续付沿用原配送单和请求号，只有服务端核验成功才展示成功')

  const cancelled = setup({ native: () => { throw { errMsg: 'requestPayment:fail cancel' } } })
  assert.equal((await cancelled.run('wechat')).nativeCancelled, true)
  assert.equal(cancelled.calls.filter(c => c[0] === 'poll').length, 0)
  console.log('PASS 微信取消保留原单，不伪造支付成功，不启动新付款')

  const pending = setup({ pending: true })
  assert.equal((await pending.run('wechat')).funds.settlement, null)
  const activeWechat = setup({ funds: { payment: { provider: 'wechat', status: 'prepay_issued' } } })
  await assert.rejects(activeWechat.run('wallet'), /状态|支付/)
  assert.equal(activeWechat.calls.filter(c => c[0] === 'wallet').length, 0)
  console.log('PASS 未决微信付款禁止切换余额扣费，回调未知不冒充已发布')

  const changed = setup({ readOrder: () => ({ ...baseOrder, deliveryFeeCents: 450, rowVersion: 2 }) })
  await assert.rejects(changed.run('wechat'), /变化|更新/)
  assert.equal(changed.calls.filter(c => c[0] === 'wechat').length, 0)
  const claimed = setup({ order: { status: 'ready_for_claim' }, readOrder: () => ({ ...baseOrder, status: 'claimed', rowVersion: 2 }), funds: { settlement: { status: 'paid' } } })
  await assert.rejects(claimed.run('refund'), /变化|更新|状态/)
  assert.equal(claimed.calls.filter(c => c[0] === 'refund').length, 0)
  console.log('PASS 费用或订单版本变化需重新确认，骑手抢单后不能按旧快照退款')

  const refund = setup({ order: { status: 'ready_for_claim' }, funds: { settlement: { status: 'paid' } } })
  const refunded = await refund.run('refund')
  assert.equal(refunded.funds.refund.status, 'pending')
  assert.equal(refunded.order.status, 'cancelled')
  console.log('PASS 取消订单与微信退款到账分别表达，退款申请不冒充到账')

  for (const phase of ['readOrder', 'wechat', 'native']) {
    const gate = deferred()
    const opts = phase === 'readOrder' ? { readOrder: () => gate.promise } : phase === 'wechat' ? { wechat: () => gate.promise } : { native: () => gate.promise }
    const app = setup(opts)
    const running = app.run('wechat')
    const rejected = assert.rejects(running, /页面|会话|中断/)
    for (let i = 0; i < 12; i++) await Promise.resolve()
    app.stop()
    const count = app.calls.length
    gate.resolve(phase === 'readOrder' ? baseOrder : phase === 'wechat' ? { payment: { status: 'prepay_issued', provider: 'wechat', wechat: {}, paymentPublicId: 'PAY-A' } } : undefined)
    await rejected
    assert.equal(app.calls.length, count, `${phase} 后失效会话不得继续请求`)
  }
  console.log('PASS 读取、预支付和原生支付三个异步阶段换账号/卸载均阻断后续操作')
}
run().catch(error => { console.error(error); process.exitCode = 1 })
