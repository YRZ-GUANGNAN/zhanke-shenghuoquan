// Render the actual TSX with isolated hooks. No network, account or payment writes.
const assert = require('node:assert/strict')
const { loadPage, loadSource, hookRenderer, deferred, nodes, text, hasClass, merchantHarness, order } = require('./test-delivery-stage-workbench.cjs')
const button = (tree, label) => {
  const value = nodes(tree, node => node.type === 'AppButton' && text(node) === label)[0]
  assert.ok(value, `缺少按钮 ${label}`)
  return value.props.onClick
}
const section = (tree, title) => nodes(tree, node => hasClass(node, 'rider-wallet-section') && text(node).startsWith(title))[0]
const record = (id) => ({ earningPublicId: id, deliveryOrderPublicId: id, amountCents: 120, settlementStatus: 'pending', createdAt: '2026-10-26T02:00:00Z' })
function withdrawalState(storage = new Map()) {
  return loadSource('pages/rider-wallet/withdrawal-state.ts', {
    '../../services/taro': {
      getStorageSync: key => storage.get(key), setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key),
    },
  }, hookRenderer())
}
function riderWalletHarness(services = {}, storage = new Map()) {
  const renderer = hookRenderer()
  let accountId = 'A', show, hide
  const session = () => ({ profile: { id: accountId }, token: `TOKEN-${accountId}` })
  const store = select => select(session()); store.getState = session
  loadPage('rider-wallet', {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView', Input: 'Input' },
    '@tarojs/taro': { useDidShow: fn => { show = fn }, useDidHide: fn => { hide = fn } },
    '../../components/capsule-page-header': { CapsuleSystemRow: 'CapsuleSystemRow' },
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../components/ui': { AppButton: 'AppButton', AppIconButton: 'AppIconButton', AppNotice: 'AppNotice', AppActionSheet: 'AppActionSheet' },
    '../../hooks/use-identity-guard': { useIdentityGuard: () => true },
    '../../services/delivery-rider': { getDeliveryRiderTodayStats: async () => ({ todayOrders: 3, todayIncomeCents: 360 }) },
    '../../services/delivery-rider-wallet': {
      getRiderWallet: async () => ({ availableBalanceCents: 360, reservedCents: 120, unverifiedCents: 0 }),
      listRiderEarnings: async () => ({ items: [], total: 0 }),
      listRiderLedger: async () => ({ items: [], total: 0 }),
      listRiderPayoutQueue: async () => ({ items: [], total: 0 }),
      listRiderWithdrawals: async () => ({ items: [], total: 0 }),
      createRiderWithdrawal: async amountCents => ({ withdrawPublicId: 'WITHDRAWAL', riderUserId: 1, amountCents, status: 'processing', rowVersion: 1, createdAt: '2026-10-26T02:00:00Z', updatedAt: '2026-10-26T02:00:00Z' }),
      ...services,
    },
    '../../services/delivery-merchant': { createDeliveryClientRequestId: () => '01ARZ3NDEKTSV4RRFFQ69G5FAV' },
    '../../services/api': { isApiError: error => error?.name === 'ApiError' },
    './withdrawal-state': withdrawalState(storage),
    '../../services/taro': { navigateBack: async () => {}, showToast: async () => {} },
    '../../stores/session': { useSessionStore: store }, './index.scss': {},
  }, renderer)
  return {
    ...renderer, get tree() { return renderer.tree },
    async mount() { renderer.render(); show?.(); await renderer.flush() },
    async account(id) { accountId = id; renderer.render(); await renderer.flush() },
    async hide() { hide?.(); await renderer.flush() },
    async show() { show?.(); await renderer.flush() },
  }
}

function rechargeHarness(options = {}) {
  const cache = options.cache || new Map(), calls = []
  const walletState = {
    readPendingWalletRecharge: id => cache.get(id) || null,
    savePendingWalletRecharge: value => cache.set(value.accountId, value),
    clearPendingWalletRecharge: id => cache.delete(id),
  }
  const payment = { paymentPublicId: 'PAY-A', purpose: 'wallet_recharge', amountCents: 10000, provider: 'wechat', status: 'prepay_issued', wechat: {} }
  const flow = loadSource('pages/merchant-wallet/wallet-recharge.ts', {
    '../../services/delivery-payment': {
      getDeliveryPayment: async () => { calls.push('get'); return options.get ? options.get() : payment },
      getDeliveryWalletRechargeByRequest: async () => { calls.push('getByRequest'); return options.getByRequest ? options.getByRequest() : payment },
      startDeliveryWalletRecharge: async () => { calls.push('start'); return options.start ? options.start() : { payment } },
      closeDeliveryWalletRecharge: async () => { calls.push('close'); return options.close ? options.close() : { ...payment, status: 'closed' } },
      closeDeliveryWalletRechargeByRequest: async () => { calls.push('closeByRequest'); return options.close ? options.close() : { ...payment, status: 'closed' } },
      pollDeliveryPayment: async () => { calls.push('poll'); return { ...payment, status: 'paid' } },
    }, './wallet-state': walletState,
  }, hookRenderer())
  const app = merchantHarness('merchant-wallet', {
    walletState, runWalletRecharge: flow.runWalletRecharge,
    requestPayment: async () => { calls.push('native'); return options.native?.() },
  })
  return { app, cache, calls, payment }
}
async function run() {
  {
    const state = withdrawalState()
    for (const value of ['0', '-1', '+1', '1e2', '0.001', '01', '1.', '', 'NaN', '1000000']) assert.equal(state.parseWithdrawalCents(value), null, value)
    for (const [value, cents] of [['0.01', 1], ['1.1', 110], [' 2.34 ', 234], ['999999.99', 99999999]]) assert.equal(state.parseWithdrawalCents(value), cents)
    state.savePendingRiderWithdrawal({ accountId: 'A', amountCents: 120, idempotencyKey: '01ARZ3NDEKTSV4RRFFQ69G5FAV' })
    assert.equal(state.readPendingRiderWithdrawal('A').amountCents, 120)
    assert.equal(state.readPendingRiderWithdrawal('B'), null)
    state.clearPendingRiderWithdrawal('A'); assert.equal(state.readPendingRiderWithdrawal('A'), null)
    console.log('PASS 提现元分精确转换、格式边界、按账号隔离恢复记录')
  }
  const fillWithdrawal = async (app, amount = '1.20') => {
    button(app.tree, '提现')(); await app.flush()
    nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: amount } }); await app.flush()
    return button(app.tree, '提交提现申请')
  }
  const withdrawal = { withdrawPublicId: 'WITHDRAWAL', amountCents: 120, status: 'processing', rowVersion: 1, createdAt: '2026-10-26T02:00:00Z' }
  {
    const pending = deferred(); let calls = 0, balance = 360
    const app = riderWalletHarness({
      getRiderWallet: async () => ({ availableBalanceCents: balance, reservedCents: 120, unverifiedCents: 0 }),
      createRiderWithdrawal: () => { calls++; return pending.promise },
      listRiderWithdrawals: async () => ({ items: balance === 240 ? [withdrawal] : [], total: balance === 240 ? 1 : 0 }),
    })
    await app.mount(); const submit = await fillWithdrawal(app)
    submit(); submit(); await app.flush(); assert.equal(calls, 1)
    assert.match(text(nodes(app.tree, node => hasClass(node, 'rider-wallet-balance-card'))), /3\.60/, '响应前不能本地扣减')
    balance = 240; pending.resolve(withdrawal); await app.flush()
    assert.match(text(nodes(app.tree, node => hasClass(node, 'rider-wallet-balance-card'))), /2\.40/)
    assert.match(text(section(app.tree, '提现记录')), /提现中/)
    app.unmount(); console.log('PASS 提现同帧防重、服务端确认后刷新真实余额和提现中状态')
  }
  {
    const storage = new Map(); const requests = []
    const service = { createRiderWithdrawal: async (...args) => { requests.push(args); throw new Error('timeout') } }
    let app = riderWalletHarness(service, storage); await app.mount()
    const submit = await fillWithdrawal(app); submit(); await app.flush()
    assert.match(text(app.tree), /暂未确认结果/); app.unmount()
    app = riderWalletHarness(service, storage); await app.mount()
    button(app.tree, '继续原提现申请')(); await app.flush()
    assert.equal(nodes(app.tree, node => node.type === 'Input').length, 0, '未知结果不能修改金额')
    button(app.tree, '重试原提现申请')(); await app.flush()
    assert.deepEqual(requests[0], requests[1]); assert.equal(requests[1][0], 120)
    app.unmount(); console.log('PASS 请求超时跨卸载恢复原金额与幂等键，不生成新提现')
  }
  {
    const pending = deferred(); const storage = new Map()
    const app = riderWalletHarness({ createRiderWithdrawal: () => pending.promise }, storage)
    await app.mount(); const submit = await fillWithdrawal(app); submit(); await app.flush()
    await app.account('B'); pending.resolve(withdrawal); await app.flush()
    assert.ok(storage.has('delivery.rider.withdrawal.A')); assert.doesNotMatch(text(section(app.tree, '提现记录')), /提现申请/)
    app.unmount(); console.log('PASS 提现响应换账号隔离，不清理旧账号原申请')
  }
  {
    let calls = 0
    const app = riderWalletHarness({ createRiderWithdrawal: async () => { calls++; throw Object.assign(new Error('尚未开放'), { name: 'ApiError', status: 403 }) } })
    await app.mount(); let submit = await fillWithdrawal(app, '2.41'); submit(); await app.flush(); assert.equal(calls, 0, '结算占用金额不可花费')
    nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: '1.20' } }); await app.flush()
    submit = button(app.tree, '提交提现申请'); submit(); await app.flush()
    assert.equal(calls, 1); assert.match(text(app.tree), /尚未开放/)
    assert.ok(nodes(app.tree, node => node.type === 'Input').length, '明确未提交后允许重新填写')
    app.unmount(); console.log('PASS 结算预留不可提现，服务端明确拒绝不显示成功')
  }
  {
    for (const [amount, expectedCents] of [['0.01', 1], ['0.29', 29], ['1.23', 123], ['100000', 10_000_000]]) {
      const requests = [], saved = []
      let nativeCalls = 0
      const app = merchantHarness('merchant-wallet', {
        walletState: {
          readPendingWalletRecharge: () => null,
          savePendingWalletRecharge: value => saved.push(value),
          clearPendingWalletRecharge: () => {},
        },
        runWalletRecharge: async input => { requests.push(input); return { status: 'paid' } },
        requestPayment: async () => { nativeCalls++; throw new Error('金额测试不得拉起付款') },
      })
      await app.mount(); button(app.tree, '充值')(); await app.flush()
      const input = nodes(app.tree, node => node.type === 'Input' && hasClass(node, 'merchant-wallet-custom-field__input'))[0]
      assert.ok(input, '真实充值页必须渲染自定义金额输入框')
      input.props.onInput({ detail: { value: amount } }); await app.flush()
      assert.equal(nodes(app.tree, node => hasClass(node, 'merchant-wallet-amount--active')).length, 0, '输入自定义金额后预设金额不应仍被选中')
      button(app.tree, `微信支付 ¥${(expectedCents / 100).toFixed(2)}`)(); await app.flush()
      assert.equal(requests.length, 1, amount)
      assert.equal(requests[0].pending.amountCents, expectedCents, `${amount} 元必须精确转换为整数分`)
      assert.equal(saved[0].amountCents, expectedCents, '恢复记录应保存实际提交金额')
      assert.equal(nativeCalls, 0, '测试必须隔离真实微信付款')
      await app.unmount()
    }
    console.log('PASS 商家真实充值页自定义 0.01、0.29、1.23、100000 元精确转换、展示并保存整数分')
  }
  {
    let calls = 0, saves = 0
    const app = merchantHarness('merchant-wallet', {
      walletState: {
        readPendingWalletRecharge: () => null,
        savePendingWalletRecharge: () => { saves++ },
        clearPendingWalletRecharge: () => {},
      },
      runWalletRecharge: async () => { calls++; return { status: 'paid' } },
    })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    for (const amount of ['0', '-1', '1.234', '100000.01', '100001', '1e2', ' ', '\t']) {
      nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: amount } }); await app.flush()
      const previousToasts = app.toasts.length
      button(app.tree, '微信支付')(); await app.flush()
      assert.equal(calls, 0, `${amount} 不得提交充值`)
      assert.equal(saves, 0, `${amount} 不得生成待处理原单`)
      assert.equal(app.toasts.length, previousToasts + 1)
      assert.match(app.toasts.at(-1).title, /0\.01 至 100000/, '无效金额必须给出明确可用范围')
      assert.equal(nodes(app.tree, node => node.type === 'Input')[0].props.value, amount, '校验失败后保留输入方便修改')
    }
    nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: '1.23' } }); await app.flush()
    button(app.tree, '微信支付 ¥1.23')(); await app.flush()
    assert.equal(calls, 1, '校验失败后应释放锁并允许有效金额提交')
    await app.unmount()
    console.log('PASS 商家自定义金额拒绝零、负数、三位小数、超限、指数和纯空白且不创建原单')
  }
  {
    const requests = []
    const app = merchantHarness('merchant-wallet', {
      runWalletRecharge: async input => { requests.push(input); return { status: 'paid' } },
    })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: '1.23' } }); await app.flush()
    nodes(app.tree, node => hasClass(node, 'merchant-wallet-amount') && text(node) === '¥200')[0].props.onClick(); await app.flush()
    assert.equal(nodes(app.tree, node => node.type === 'Input')[0].props.value, '')
    assert.equal(text(nodes(app.tree, node => hasClass(node, 'merchant-wallet-amount--active'))), '¥200')
    button(app.tree, '微信支付 ¥200')(); await app.flush()
    assert.equal(requests.length, 1); assert.equal(requests[0].pending.amountCents, 20000)
    await app.unmount()
    console.log('PASS 重新选择预设金额清空自定义输入，并按选中的预设金额提交')
  }
  {
    const pending = deferred(), requests = [], cache = new Map()
    const app = merchantHarness('merchant-wallet', {
      walletState: {
        readPendingWalletRecharge: id => cache.get(id) || null,
        savePendingWalletRecharge: value => cache.set(value.accountId, value),
        clearPendingWalletRecharge: id => cache.delete(id),
      },
      runWalletRecharge: input => { requests.push(input); return pending.promise },
    })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    nodes(app.tree, node => node.type === 'Input')[0].props.onInput({ detail: { value: '1.23' } }); await app.flush()
    const confirm = button(app.tree, '微信支付 ¥1.23'); confirm(); confirm(); await app.flush()
    assert.equal(requests.length, 1, '自定义金额同帧双击只能提交一次')
    assert.equal(requests[0].pending.amountCents, 123)
    assert.equal(cache.get(101).amountCents, 123)
    assert.equal(nodes(app.tree, node => node.type === 'Input').length, 0, '提交后按原单处理，不允许篡改金额')
    assert.match(text(nodes(app.tree, node => hasClass(node, 'merchant-wallet-balance'))), /10\.00/, '响应前不得本地增加余额')
    pending.resolve({ status: 'paid' }); await app.flush()
    assert.equal(cache.has(101), false, '服务端确认支付后才清除恢复记录')
    await app.unmount()
    console.log('PASS 自定义金额同帧防双击，响应前不改余额，原单金额在处理中保持锁定')
  }
  {
    const cache = new Map([[101, { accountId: 101, amountCents: 5000, clientRequestId: 'ORIGINAL', paymentPublicId: 'PAY-A', createdAt: 1 }]])
    const pending = deferred(); let calls = 0, currentCheck
    const app = merchantHarness('merchant-wallet', {
      walletState: {
        readPendingWalletRecharge: id => cache.get(id) || null,
        savePendingWalletRecharge: value => cache.set(value.accountId, value),
        clearPendingWalletRecharge: id => cache.delete(id),
      },
      runWalletRecharge: input => {
        calls++; currentCheck = input.isCurrent
        assert.equal(input.pending.amountCents, 5000, '恢复原单不能改用当前默认100元')
        assert.equal(input.pending.clientRequestId, 'ORIGINAL')
        return pending.promise
      },
    })
    await app.mount()
    assert.equal(nodes(app.tree, node => node.type === 'Input').length, 0, '恢复原单时不能再修改自定义金额')
    assert.equal(nodes(app.tree, node => hasClass(node, 'merchant-wallet-amount')).length, 0, '恢复原单时不能选择其他预设金额')
    const confirm = button(app.tree, '查询充值状态'); confirm(); confirm(); await app.flush()
    assert.equal(calls, 1)
    await app.setAccount(202); assert.equal(currentCheck(), false)
    pending.resolve({ status: 'paid' }); await app.flush()
    assert.ok(cache.has(101), '失效响应不得清理旧账号恢复标识')
    assert.doesNotMatch(text(app.tree), /充值已确认到账/)
    await app.unmount()
    console.log('PASS 商家充值原金额只查询、防重复提交、换账号后不展示旧付款成功')
  }
  {
    const { app, calls, cache } = rechargeHarness()
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet')[0]
    const oldSubmit = button(app.tree, '微信支付 ¥100')
    sheet.props.onClose(); oldSubmit(); await app.flush()
    assert.equal(nodes(app.tree, node => node.type === 'AppActionSheet')[0].props.visible, false)
    assert.deepEqual(calls, []); assert.equal(cache.size, 0)
    await app.unmount()
    console.log('PASS 选金额后关闭无网络请求，关闭动画中旧提交也不能新建付款')
  }
  {
    const pendingClose = deferred()
    const { app, calls, cache, payment } = rechargeHarness({ native: () => { throw { errMsg: 'requestPayment:fail cancel' } }, close: () => pendingClose.promise })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    button(app.tree, '微信支付 ¥100')(); await app.flush()
    assert.equal(nodes(app.tree, node => node.type === 'AppActionSheet')[0].props.visible, false, '关单未返回也应立即退出弹窗')
    assert.equal(cache.get(101).cancelRequested, true)
    assert.doesNotMatch(text(app.tree), /继续.*支付|继续原单/)
    assert.equal(calls.filter(c => c === 'close').length, 1)
    pendingClose.resolve({ ...payment, status: 'closed' }); await app.flush()
    assert.equal(cache.has(101), false); assert.ok(button(app.tree, '充值'))
    await app.unmount()
    console.log('PASS 微信取消即关弹窗，服务端关单确认后清记录并恢复充值入口')
  }
  {
    const creating = deferred(), closing = deferred()
    const { app, calls, cache, payment } = rechargeHarness({ start: () => creating.promise, close: () => closing.promise })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    button(app.tree, '微信支付 ¥100')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet')[0]
    assert.notEqual(sheet.props.loading, true, '网络请求期间不能锁死面板关闭按钮')
    sheet.props.onClose(); sheet.props.onClose(); await app.flush()
    assert.equal(cache.get(101).cancelRequested, true)
    assert.equal(calls.filter(c => c === 'closeByRequest').length, 1, '不等创建响应，立即按原请求键持久取消')
    creating.resolve({ payment }); await app.flush()
    assert.equal(calls.filter(c => c === 'start').length, 1)
    assert.equal(calls.filter(c => c === 'close').length, 0)
    assert.equal(calls.includes('native'), false)
    closing.resolve({ ...payment, status: 'closed' }); await app.flush()
    assert.equal(cache.has(101), false)
    await app.unmount()
    console.log('PASS 创建充值中关闭立即并行提交取消，迟到原响应不付款且无重复关单')
  }
  {
    const { app, cache, calls } = rechargeHarness({ native: () => { throw new Error('cancel') }, close: () => { throw new Error('timeout') } })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    button(app.tree, '微信支付 ¥100')(); await app.flush()
    assert.equal(cache.get(101).cancelRequested, true)
    assert.match(text(app.tree), /取消结果待核实/)
    assert.doesNotMatch(text(app.tree), /继续.*支付/)
    assert.equal(nodes(app.tree, node => node.type === 'AppActionSheet')[0].props.visible, false)
    button(app.tree, '查询充值状态')(); await app.flush()
    assert.equal(calls.filter(c => c === 'native').length, 1)
    assert.equal(calls.filter(c => c === 'close').length, 2)
    await app.unmount()
    console.log('PASS 关单超时仅待核实，可查询重试关单，不重复付款或卡住页面')
  }
  {
    const cancelled = { accountId: 101, amountCents: 10000, clientRequestId: 'ORIGINAL', paymentPublicId: 'PAY-A', createdAt: 1, cancelRequested: true }
    const closing = deferred(), cache = new Map([[101, cancelled]])
    const { app, payment } = rechargeHarness({ cache, close: () => closing.promise })
    await app.mount(); button(app.tree, '查询充值状态')(); await app.flush()
    await app.setAccount(202)
    closing.resolve({ ...payment, status: 'paid' }); await app.flush()
    assert.equal(cache.has(101), true)
    assert.doesNotMatch(text(app.tree), /充值已确认到账/)
    await app.unmount()
    console.log('PASS 取消核验迟到响应不清其他账号记录、不显示旧账号支付结果')
  }
  {
    const creating = deferred(), closing = deferred()
    const { app, cache, calls, payment } = rechargeHarness({ start: () => creating.promise, close: () => closing.promise })
    await app.mount(); button(app.tree, '充值')(); await app.flush()
    button(app.tree, '微信支付 ¥100')(); await app.flush()
    nodes(app.tree, node => node.type === 'AppActionSheet')[0].props.onClose()
    assert.equal(calls.includes('closeByRequest'), true, '退出页面前取消意图请求已发出')
    await app.hide(); await app.unmount()
    creating.resolve({ payment }); closing.resolve({ ...payment, status: 'closed' }); await app.flush()
    assert.equal(calls.includes('native'), false); assert.equal(cache.get(101).cancelRequested, true)
    console.log('PASS 取消后立即隐藏卸载也已发送关单，迟到创建不能付款且保留本地核验记录')
  }
  {
    const cancelled = { accountId: 101, amountCents: 10000, clientRequestId: 'ORIGINAL', createdAt: 1, cancelRequested: true }
    const cache = new Map([[101, cancelled]])
    const { app, calls } = rechargeHarness({ cache, close: () => { throw new Error('offline') } })
    await app.mount()
    assert.equal(calls.filter(c => c === 'closeByRequest').length, 1)
    assert.equal(calls.some(c => ['get', 'start', 'native'].includes(c)), false)
    await app.hide(); await app.show()
    assert.equal(calls.filter(c => c === 'closeByRequest').length, 2)
    await app.setAccount(202); await app.show()
    assert.equal(calls.filter(c => c === 'closeByRequest').length, 2, '新账号不重发旧账号取消')
    await app.unmount()
    console.log('PASS 重进与重新显示自动恢复取消意图，严格隔离账号且不预支付')
  }
  {
    let calls = 0
    const cancelled = { accountId: 101, amountCents: 10000, clientRequestId: 'ORIGINAL', createdAt: 1, cancelRequested: true }
    const app = merchantHarness('merchant-wallet', {
      deliveryPaymentEnabled: false,
      walletState: { readPendingWalletRecharge: () => cancelled, savePendingWalletRecharge: () => {}, clearPendingWalletRecharge: () => {} },
      runWalletRecharge: async () => { calls++; throw new Error('关闭支付时不得调用') },
    })
    await app.mount(); await app.hide(); await app.show()
    assert.equal(calls, 0)
    await app.unmount()
    console.log('PASS 支付能力关闭时不自动发送取消资金请求')
  }
  {
    const next = deferred(); let calls = 0, failing = false
    const app = riderWalletHarness({ listRiderEarnings: async (page = 1) => {
      if (failing) throw new Error('测试网络失败')
      if (page > 1) { calls++; return next.promise }
      return { items: [record('FIRST')], total: 3 }
    } })
    await app.mount()
    const more = button(section(app.tree, '收入记录'), '加载更多')
    more(); more(); await app.flush()
    assert.equal(calls, 1, '同帧重复分页只能发送一次请求')
    next.resolve({ items: [record('FIRST'), record('SECOND')], total: 3 }); await app.flush()
    assert.equal(nodes(section(app.tree, '收入记录'), node => hasClass(node, 'rider-wallet-list-item')).length, 2)
    failing = true; await app.show()
    assert.match(text(section(app.tree, '收入记录')), /FIRST.*SECOND/)
    assert.match(text(section(app.tree, '收入记录')), /保留上次记录/)
    assert.match(text(app.tree), /今日预计收入/)
    app.unmount()
    console.log('PASS 骑手真实页面分页防双击、按完整ID去重，刷新失败保留原收入记录')
  }
  {
    const waiting = deferred(); let reads = 0
    const app = riderWalletHarness({ listRiderEarnings: async (page = 1) => {
      reads++
      return page > 1 ? waiting.promise : { items: [record('CURRENT')], total: 2 }
    } })
    await app.mount(); const more = button(section(app.tree, '收入记录'), '加载更多')
    more(); await app.flush(); await app.account('B')
    const before = reads; more(); await app.flush(); assert.equal(reads, before)
    waiting.resolve({ items: [record('OLD-ACCOUNT')], total: 2 }); await app.flush()
    assert.doesNotMatch(text(app.tree), /OLD-ACCOUNT/)
    app.unmount()
    console.log('PASS 钱包旧账号分页响应与旧按钮不会串入新账号')
  }
  {
    const read = deferred(); let reads = 0, writes = 0
    let currentOrder = order('ORDER-A', 'pending_fee_payment')
    let funds = { deliveryOrderPublicId: 'ORDER-A', payment: null, settlement: null, refund: null }
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: () => ++reads === 2 ? read.promise : Promise.resolve(currentOrder),
      getDeliveryOrderFunds: async () => funds,
      payDeliveryOrderWithWallet: async () => {
        writes++; currentOrder = { ...currentOrder, rowVersion: 2, status: 'ready_for_claim' }
        funds = { ...funds, settlement: { source: 'wallet', status: 'paid' } }; return {}
      },
    })
    await app.mount(); button(app.tree, '余额支付')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet' && node.props.visible)[0]
    const confirm = nodes(sheet, node => node.type === 'AppButton')[0].props.onClick
    confirm(); confirm(); await app.flush(); assert.equal(reads, 2)
    read.resolve(currentOrder); await app.flush()
    assert.equal(writes, 1); assert.match(text(app.tree), /配送费已确认支付/)
    await app.unmount()
    console.log('PASS 详情页付款二次确认、同帧防重和服务端确认后更新状态')
  }
  {
    let writes = 0
    const currentOrder = order('ORDER-A', 'pending_fee_payment')
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: async () => currentOrder,
      payDeliveryOrderWithWallet: async () => { writes++; throw new Error('响应丢失') },
    })
    await app.mount(); button(app.tree, '余额支付')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet' && node.props.visible)[0]
    nodes(sheet, node => node.type === 'AppButton')[0].props.onClick(); await app.flush()
    assert.equal(writes, 1)
    assert.equal(nodes(app.tree, node => hasClass(node, 'merchant-detail-funds-actions')).length, 0)
    assert.match(text(app.tree), /请刷新核实/)
    await app.unmount()
    console.log('PASS 支付请求结果未知时关闭旧付款按钮，不盲目重发')
  }
  {
    const preflight = deferred(); let reads = 0, writes = 0
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: () => ++reads === 1 ? Promise.resolve(order('ORDER-A', 'pending_fee_payment')) : preflight.promise,
      payDeliveryOrderWithWallet: async () => { writes++ },
    })
    await app.mount(); button(app.tree, '余额支付')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet' && node.props.visible)[0]
    nodes(sheet, node => node.type === 'AppButton')[0].props.onClick(); await app.flush()
    await app.hide(); preflight.resolve(order('ORDER-A', 'pending_fee_payment')); await app.flush()
    assert.equal(writes, 0); await app.unmount()
    console.log('PASS 普通页面离开终止迟到的付款预检，不在其他页面继续扣费')
  }
  {
    let native = 0, polled = 0
    const nativeFinished = deferred()
    let currentOrder = order('ORDER-A', 'pending_fee_payment')
    let funds = { deliveryOrderPublicId: 'ORDER-A', payment: null, settlement: null, refund: null }
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: async () => currentOrder,
      getDeliveryOrderFunds: async () => funds,
      payDeliveryOrderWithWechat: async () => ({ payment: { paymentPublicId: 'PAY-A', provider: 'wechat', status: 'prepay_issued', wechat: {} } }),
      requestPayment: async () => { native++; await app.hide(); await app.show(); nativeFinished.resolve() },
      pollDeliveryPayment: async () => {
        polled++; currentOrder = { ...currentOrder, status: 'ready_for_claim', rowVersion: 2 }
        funds = { ...funds, settlement: { status: 'paid', source: 'wechat' } }; return { status: 'paid' }
      },
    })
    await app.mount(); button(app.tree, '微信支付')(); await app.flush()
    const sheet = nodes(app.tree, node => node.type === 'AppActionSheet' && node.props.visible)[0]
    nodes(sheet, node => node.type === 'AppButton')[0].props.onClick(); await nativeFinished.promise; await app.flush()
    assert.equal(native, 1); assert.equal(polled, 1); assert.match(text(app.tree), /配送费已确认支付/)
    await app.unmount()
    console.log('PASS 系统微信付款临时遮盖页面不会误取消核验或重复拉起付款')
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
