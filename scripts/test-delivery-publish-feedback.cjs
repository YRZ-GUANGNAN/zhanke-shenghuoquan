// Run the registered merchant page against isolated service responses; no money,
// production account, order or network request is used by these scenarios.
const assert = require('node:assert/strict')
const { deferred, nodes, text, hasClass, hookRenderer, loadPage } = require('./test-delivery-stage-workbench.cjs')

class ApiError extends Error {
  constructor(message, status) { super(message); this.status = status }
}
const deliveryOrder = { deliveryOrderPublicId: 'ORDER-A', rowVersion: 1, status: 'pending_fee_payment', deliveryFeeCents: 150 }
const inputValues = ['取货人', '13800138000', '一号食堂', '收货人', '13900139000', '三栋201']

function publishHarness(options = {}) {
  const renderer = hookRenderer()
  let identityAllowed = true, requestSequence = 0
  const profile = { id: 'MERCHANT-A', nickname: '测试商家', campus: '', school: '' }
  const calls = { create: [], wallet: [], wechat: [], mock: [], poll: [], navigate: [], toasts: [] }
  const invoke = (name, fallback) => async (...args) => {
    calls[name].push(args)
    return (options[name] || fallback)(...args)
  }
  const imports = {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView', Input: 'Input', Textarea: 'Textarea' },
    '../../components/feature-page-gate': { withFeaturePage: (value) => value },
    '../../components/capsule-page-header': 'Header',
    '../../components/app-icon': 'AppIcon',
    '../../components/ui': { AppIconButton: 'AppIconButton', AppActionSheet: 'AppActionSheet', AppButton: 'AppButton' },
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../feature-flags': { deliveryPaymentEnabled: true, deliveryMockFeeEnabled: false, ...options.flags },
    '../../hooks/use-identity-guard': { useIdentityGuard: () => identityAllowed },
    '../../services/a-user': { getIdentityApplication: options.application || (async () => ({})) },
    '../../services/api': { isApiError: (error) => error instanceof ApiError },
    '../../services/delivery-merchant': {
      getMerchantOrderIntake: options.intake || (async () => ({ manualAvailable: true, externalAvailable: false, platforms: ['taobao', 'jd', 'meituan'].map((platform) => ({ platform, connected: false })) })),
      createDeliveryClientRequestId: () => `REQUEST-${++requestSequence}`,
      createMerchantDeliveryOrder: invoke('create', async () => deliveryOrder),
      confirmMockDeliveryFee: invoke('mock', async () => ({ ...deliveryOrder, status: 'ready_for_claim' })),
    },
    '../../services/delivery-payment': {
      getDeliveryWallet: options.getWallet || (async () => ({ availableBalanceCents: 10000 })),
      payDeliveryOrderWithWallet: invoke('wallet', async () => ({ settlement: { status: 'paid' } })),
      payDeliveryOrderWithWechat: invoke('wechat', async () => ({ payment: { provider: 'wechat', paymentPublicId: 'PAY-A', status: 'prepay_issued', wechat: {} } })),
      pollDeliveryPayment: invoke('poll', async () => ({ status: 'paid' })),
    },
    '../../services/taro': {
      showToast: async (value) => { calls.toasts.push(value); await options.toast?.(value) },
      requestPayment: options.requestPayment || (async () => {}),
      redirectTo: invoke('navigate', async () => {}),
      navigateBack: async () => {},
      setClipboardData: async () => {}, getClipboardData: options.getClipboard || (async () => ({ data: options.clipboard || '' })),
    },
    '../../stores/session': { useSessionStore: (select) => select({ profile }) },
    '../../utils/delivery-order-clipboard': { deliveryClipboardTemplate: '', parseDeliveryOrderClipboard: options.parseClipboard || (() => ({ values: {}, recognizedFields: [] })) },
    '../../utils/delivery-order-source': { deliverySourceLabel: (platform) => ({ local: '校内配送', taobao: '淘宝闪购', jd: '京东外卖', meituan: '美团外卖' }[platform] || '校内配送') },
    './index.scss': {},
  }
  loadPage('merchant-publish', imports, renderer)
  const one = (className) => nodes(renderer.tree, (node) => hasClass(node, className))[0]
  const confirm = () => nodes(renderer.tree, (node) => node.type === 'AppButton' && text(node) === '确认发布并支付')[0]
  return {
    calls, one, confirm, flush: renderer.flush,
    get tree() { return renderer.tree },
    async mount() {
      renderer.render(); await renderer.flush()
      nodes(renderer.tree, (node) => hasClass(node, 'merchant-publish-field__input')).forEach((node, index) => node.props.onInput({ detail: { value: inputValues[index] } }))
      await renderer.flush()
    },
    async method(value) {
      const option = nodes(renderer.tree, (node) => hasClass(node, 'merchant-publish-payment') && text(node).includes(value === 'wechat' ? '微信支付' : '钱包余额'))[0]
      option.props.onClick(); await renderer.flush()
    },
    async source(label) {
      nodes(renderer.tree, (node) => hasClass(node, 'merchant-publish-source__option') && text(node) === label)[0].props.onClick()
      await renderer.flush()
    },
    async pickupCode(value) {
      one('merchant-publish-source__code').props.onInput({ detail: { value } }); await renderer.flush()
    },
    async mode(label) {
      nodes(renderer.tree, (node) => hasClass(node, 'merchant-publish-intake-tab') && text(node) === label)[0].props.onClick()
      await renderer.flush()
    },
    async submit() { confirm().props.onClick(); await renderer.flush() },
    async primary() { one('merchant-publish-footer__button').props.onClick(); await renderer.flush() },
    async identity(value) { identityAllowed = value; renderer.render(); await renderer.flush() },
    async account(value) { profile.id = value; renderer.render(); await renderer.flush() },
    async unmount() { renderer.unmount(); await renderer.flush() },
  }
}

async function run() {
  {
    const app = publishHarness()
    await app.mount(); await app.submit()
    assert.equal(app.calls.create[0][0].sourcePlatform, 'local')
    assert.equal(app.calls.create[0][0].pickupCode, '')
    assert.equal('sourceOrderId' in app.calls.create[0][0], false, '手工发单不能伪造外部全局订单 ID')
    console.log('PASS 旧发单默认校内来源，完整外部编号不由手工页面生成')
  }
  {
    const app = publishHarness()
    await app.mount(); await app.source('淘宝闪购'); await app.submit()
    assert.equal(app.calls.create.length, 0)
    assert.match(app.calls.toasts.map((item) => item.title).join(''), /取餐号/)
    await app.pickupCode(' A0022 '); await app.submit()
    assert.equal(app.calls.create[0][0].sourcePlatform, 'taobao')
    assert.equal(app.calls.create[0][0].pickupCode, 'A0022')
    console.log('PASS 平台手工订单要求取餐号，保留前导零与字母')
  }
  {
    const app = publishHarness()
    await app.mount(); await app.source('京东外卖'); await app.pickupCode('12345678901234567'); await app.submit()
    assert.equal(app.calls.create.length, 0)
    assert.match(app.calls.toasts.map((item) => item.title).join(''), /16/)
    console.log('PASS 超长取餐号不会发出创建请求')
  }
  {
    let attempts = 0
    const app = publishHarness({ create: async () => { if (++attempts === 1) throw new ApiError('超时', 0); return deliveryOrder } })
    await app.mount(); await app.source('美团外卖'); await app.pickupCode('0028'); await app.submit()
    await app.source('京东外卖'); await app.pickupCode('15'); await app.mode('平台接入')
    assert.match(text(app.tree), /订单来源/)
    assert.equal(app.one('merchant-publish-source__code').props.value, '0028')
    await app.primary()
    assert.deepEqual(app.calls.create[0], app.calls.create[1])
    assert.equal(app.calls.create[1][0].sourcePlatform, 'meituan')
    assert.equal(app.calls.wallet.length, 1)
    console.log('PASS 未知创建锁住来源、取餐号与接入方式，重试沿用原快照')
  }
  {
    const app = publishHarness()
    await app.mount(); await app.source('淘宝闪购'); await app.pickupCode('0022'); await app.account('MERCHANT-B')
    assert.equal(app.one('merchant-publish-source__code').props.value, '')
    assert.equal(text(nodes(app.tree, (node) => hasClass(node, 'merchant-publish-source__option--active'))[0]), '校内配送')
    console.log('PASS 换账号清空来源取餐号，不串用上个商家信息')
  }
  {
    const app = publishHarness()
    await app.mount(); await app.mode('平台接入'); await app.submit()
    assert.equal(app.calls.create.length, 0)
    assert.equal(app.calls.wallet.length, 0)
    assert.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-publish-intake__status') && text(node) === '未接入').length, 3)
    assert.match(text(app.tree), /当前尚未开放自动接单/)
    assert.doesNotMatch(text(app.tree), /已接入|接入成功/)
    await app.primary()
    assert.ok(app.one('merchant-publish-source__code'))
    console.log('PASS 平台未接入只显示能力状态，可返回自行发单，不创建或支付')
  }
  {
    let attempts = 0
    const app = publishHarness({ intake: async () => { if (++attempts === 1) throw new Error('断网'); return { manualAvailable: true, externalAvailable: false, platforms: [{ platform: 'jd', connected: false }] } } })
    await app.mount(); await app.mode('平台接入')
    assert.match(text(app.tree), /暂时无法确认平台接入状态/)
    assert.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-publish-intake__status') && text(node) === '状态未确认').length, 3)
    const retry = nodes(app.tree, (node) => node.type === 'AppButton' && text(node) === '重新确认接入状态')[0]
    retry.props.onClick(); await app.flush()
    assert.doesNotMatch(text(app.tree), /暂时无法确认平台接入状态/)
    assert.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-publish-intake__status') && text(node) === '未接入').length, 1)
    assert.equal(app.calls.create.length, 0)
    console.log('PASS 能力读取失败保持未确认，重试读取真实状态')
  }
  {
    const pending = deferred()
    let calls = 0
    const app = publishHarness({ intake: async () => ++calls === 1 ? pending.promise : { manualAvailable: true, externalAvailable: false, platforms: [{ platform: 'taobao', connected: false }] } })
    await app.mount(); await app.account('MERCHANT-B'); await app.mode('平台接入')
    pending.resolve({ manualAvailable: true, externalAvailable: true, platforms: [{ platform: 'taobao', connected: true }] }); await app.flush()
    assert.doesNotMatch(text(app.tree), /已接入|已开放外部订单接入/)
    console.log('PASS 旧账号接入能力迟到响应不会覆盖当前账号')
  }
  {
    const app = publishHarness({ intake: async () => ({ manualAvailable: false, externalAvailable: false, platforms: [] }) })
    await app.mount(); await app.submit()
    assert.equal(app.calls.create.length, 0)
    assert.match(app.calls.toasts.map((item) => item.title).join(''), /自行发单暂未开放/)
    console.log('PASS 服务端关闭自行发单后不创建配送单')
  }
  {
    const parsed = { values: { sourcePlatform: 'taobao', pickupCode: '0022' }, recognizedFields: ['sourcePlatform', 'pickupCode'] }
    const app = publishHarness({ clipboard: '淘宝0022号', parseClipboard: () => parsed })
    await app.mount()
    app.one('merchant-publish-import-card__primary').props.onClick(); await app.flush()
    assert.equal(app.one('merchant-publish-source__code').props.value, '', '读取不等于确认导入')
    assert.match(text(app.tree), /淘宝闪购/)
    nodes(app.tree, (node) => node.type === 'AppButton' && text(node) === '确认导入')[0].props.onClick(); await app.flush()
    assert.equal(app.one('merchant-publish-source__code').props.value, '0022')
    await app.submit()
    assert.equal(app.calls.create[0][0].sourcePlatform, 'taobao')
    assert.equal(app.calls.create[0][0].pickupCode, '0022')
    console.log('PASS 剪贴板来源与取餐号需手工确认才写入发单快照')
  }
  {
    const pending = deferred()
    const app = publishHarness({ getClipboard: () => pending.promise, parseClipboard: () => ({ values: { pickupCode: '0022' }, recognizedFields: ['pickupCode'] }) })
    await app.mount()
    app.one('merchant-publish-import-card__primary').props.onClick(); await app.flush()
    await app.mode('平台接入')
    pending.resolve({ data: '取餐号：0022' }); await app.flush()
    assert.ok(nodes(app.tree, (node) => node.type === 'AppActionSheet').every((node) => !node.props.visible))
    assert.equal(app.calls.create.length, 0)
    console.log('PASS 切换平台接入后迟到的剪贴板响应不会重新弹出导入框')
  }
  {
    const pending = deferred()
    const app = publishHarness({ create: () => pending.promise })
    await app.mount()
    const click = app.confirm().props.onClick
    click(); click(); await app.flush()
    assert.equal(app.calls.create.length, 1, '同一轮渲染双击不能创建两张配送单')
    pending.resolve(deliveryOrder); await app.flush()
    assert.equal(app.calls.wallet.length, 1)
    console.log('PASS 同帧重复点击只创建与支付一次')
  }
  {
    let attempts = 0
    const app = publishHarness({ create: async () => { if (++attempts === 1) throw new ApiError('网络超时', 0); return deliveryOrder } })
    await app.mount(); await app.submit()
    assert.match(text(app.tree), /创建结果待确认/)
    assert.ok(nodes(app.tree, (node) => node.type === 'Input').every((node) => node.props.disabled))
    await app.primary()
    assert.equal(app.calls.create.length, 2)
    assert.deepEqual(app.calls.create[0], app.calls.create[1], '重试保留完整快照和请求号')
    assert.equal(app.calls.wallet.length, 1)
    console.log('PASS 创建超时沿用原表单快照与幂等请求号')
  }
  {
    const pending = deferred()
    let attempts = 0
    const app = publishHarness({ create: async () => ++attempts === 1 ? pending.promise : deliveryOrder })
    await app.mount(); await app.submit(); await app.identity(false); await app.identity(true)
    assert.match(text(app.tree), /创建结果待确认/)
    await app.primary()
    assert.equal(app.calls.create.length, 2)
    assert.deepEqual(app.calls.create[0], app.calls.create[1], '同账户身份恢复必须沿用原请求号')
    pending.resolve(deliveryOrder); await app.flush()
    assert.equal(app.calls.wallet.length, 1)
    console.log('PASS 同账户身份恢复保留未知创建快照，不重复建单')
  }
  {
    const app = publishHarness({ navigate: async () => { throw new Error('暂时无法打开订单') } })
    await app.mount(); await app.submit(); await app.identity(false); await app.account(''); await app.account('MERCHANT-A'); await app.identity(true)
    assert.match(text(app.tree), /配送费已支付/)
    assert.match(text(app.tree), /查看配送单/)
    await app.primary()
    assert.equal(app.calls.create.length, 1)
    assert.equal(app.calls.wallet.length, 1)
    console.log('PASS 同账户重新核验及暂空资料保留已创建订单和付款结果')
  }
  {
    let attempts = 0
    const app = publishHarness({ create: async () => { if (++attempts === 1) throw new ApiError('网络超时', 0); throw new ApiError('创建参数冲突', 409) } })
    await app.mount(); await app.submit(); await app.primary()
    assert.match(text(app.tree), /创建结果待确认/)
    assert.doesNotMatch(text(app.tree), /配送单未创建/)
    assert.ok(nodes(app.tree, (node) => hasClass(node, 'merchant-publish-field__input')).every((node) => node.props.disabled))
    await app.primary()
    assert.equal(new Set(app.calls.create.map(([input]) => input.clientRequestId)).size, 1)
    const workbench = app.one('merchant-publish-feedback__workbench')
    assert.ok(workbench, '未知创建必须有既有工作台核对入口')
    workbench.props.onClick(); await app.flush()
    assert.equal(app.calls.navigate[0][0].url, '/pages/merchant-home/index')
    assert.equal(app.calls.create.length, 3)
    console.log('PASS 未知创建后409保留原请求与核对出口，不声称未创建')
  }
  {
    let walletCalls = 0
    const pendingWallet = deferred()
    const app = publishHarness({ getWallet: async () => ++walletCalls === 1 ? { availableBalanceCents: 10000 } : pendingWallet.promise })
    await app.mount(); await app.primary(); await app.account('MERCHANT-B')
    const fields = nodes(app.tree, (node) => hasClass(node, 'merchant-publish-field__input'))
    assert.ok(fields.every((node) => !node.props.value), '新账户不能保留上个账户的联系人或地址')
    assert.doesNotMatch(text(app.tree), /当前余额 ¥100.00/)
    assert.ok(nodes(app.tree, (node) => node.type === 'AppActionSheet').every((node) => !node.props.visible))
    pendingWallet.resolve({ availableBalanceCents: 0 }); await app.flush()
    console.log('PASS 真正切换账户清除表单、弹层与旧钱包余额')
  }
  {
    let attempts = 0
    const app = publishHarness({ create: async () => { if (++attempts === 1) throw new ApiError('取货电话格式不正确', 400); return deliveryOrder } })
    await app.mount(); await app.submit()
    assert.match(text(app.tree), /取货电话格式不正确/)
    assert.ok(nodes(app.tree, (node) => node.type === 'Input').every((node) => !node.props.disabled))
    await app.submit()
    assert.notEqual(app.calls.create[0][0].clientRequestId, app.calls.create[1][0].clientRequestId)
    console.log('PASS 明确业务拒绝后可以修改并重新提交')
  }
  {
    const app = publishHarness({ create: async () => ({ ...deliveryOrder, status: 'ready_for_claim', rowVersion: 3 }) })
    await app.mount(); await app.submit()
    assert.equal(app.calls.wallet.length, 0, '幂等创建返回已推进订单时不重新付款')
    assert.match(text(app.tree), /此单已有更新/)
    console.log('PASS 幂等创建返回已推进订单时仅查看，不重复支付')
  }
  {
    let navigationAttempts = 0
    const app = publishHarness({ navigate: async () => { if (++navigationAttempts === 1) throw new Error('页面暂时打不开') } })
    await app.mount(); await app.submit()
    assert.equal(app.calls.navigate.length, 1, '导航失败不在支付 catch 中再次跳转')
    assert.match(text(app.tree), /配送费已支付/)
    assert.match(text(app.tree), /页面暂时打不开/)
    assert.doesNotMatch(text(app.tree), /支付未完成|支付失败/)
    await app.primary()
    assert.equal(app.calls.create.length, 1)
    assert.equal(app.calls.wallet.length, 1)
    assert.equal(app.calls.navigate.length, 2)
    console.log('PASS 导航失败保留支付成功，仅重试查看原配送单')
  }
  {
    const app = publishHarness({ requestPayment: async () => { throw { errMsg: 'requestPayment:fail cancel' } } })
    await app.mount(); await app.method('wechat'); await app.submit()
    assert.match(text(app.tree), /已取消付款/)
    assert.match(text(app.tree), /查看配送单/)
    assert.equal(app.calls.poll.length, 0)
    await app.primary()
    assert.equal(app.calls.create.length, 1)
    assert.equal(app.calls.wechat.length, 1)
    console.log('PASS 微信取消保留已创建配送单，查看不会重新付款')
  }
  for (const status of ['failed', 'closed', 'prepay_issued']) {
    const app = publishHarness({ wechat: async () => ({ payment: { provider: 'mock', paymentPublicId: 'PAY-A' } }), poll: async () => ({ status }) })
    await app.mount(); await app.method('wechat'); await app.submit()
    assert.doesNotMatch(app.calls.toasts.map((item) => item.title).join(''), /配送单已发布/)
    assert.match(text(app.tree), status === 'failed' ? /支付失败/ : status === 'closed' ? /支付已关闭/ : /支付结果待确认/)
    assert.equal(app.calls.navigate.length, 0)
    console.log(`PASS ${status} 支付结果不会冒充发布成功`)
  }
  {
    const app = publishHarness({ wallet: async () => { throw new ApiError('连接中断', 0) } })
    await app.mount(); await app.submit()
    assert.match(text(app.tree), /支付结果待确认/)
    await app.primary()
    assert.equal(app.calls.wallet.length, 1, '付款结果未知时只查看已建配送单，不再次扣款')
    assert.equal(app.calls.create.length, 1)
    console.log('PASS 钱包支付结果未知后只查看，不重复扣款')
  }
  {
    const app = publishHarness({ wallet: async () => { throw new ApiError('钱包余额不足', 400) } })
    await app.mount(); await app.submit()
    assert.match(text(app.tree), /钱包余额不足/)
    assert.match(text(app.tree), /查看配送单/)
    assert.equal(app.calls.navigate.length, 0)
    console.log('PASS 已建配送单的明确付款拒绝保留真实原因')
  }
  for (const terminate of ['unmount', 'identity', 'account']) {
    const pending = deferred()
    const app = publishHarness({ create: () => pending.promise })
    await app.mount(); await app.submit()
    if (terminate === 'unmount') await app.unmount()
    if (terminate === 'identity') await app.identity(false)
    if (terminate === 'account') await app.account('MERCHANT-B')
    pending.resolve(deliveryOrder); await app.flush()
    assert.equal(app.calls.wallet.length, 0)
    assert.equal(app.calls.navigate.length, 0)
    console.log(`PASS ${terminate} 后创建响应不会继续支付或导航`)
  }
  {
    const paymentSetup = deferred()
    let nativePaymentCalls = 0
    const app = publishHarness({ wechat: () => paymentSetup.promise, requestPayment: async () => { nativePaymentCalls++ } })
    await app.mount(); await app.method('wechat'); await app.submit(); await app.unmount()
    paymentSetup.resolve({ payment: { provider: 'wechat', paymentPublicId: 'PAY-A', wechat: {} } }); await app.flush()
    assert.equal(nativePaymentCalls, 0)
    assert.equal(app.calls.poll.length, 0)
    assert.equal(app.calls.navigate.length, 0)
    console.log('PASS 卸载后的预支付响应不会唤起微信付款')
  }
  {
    const app = publishHarness({ flags: { deliveryPaymentEnabled: false, deliveryMockFeeEnabled: false } })
    await app.mount(); await app.primary(); await app.submit()
    assert.equal(app.calls.create.length, 0)
    const testApp = publishHarness({ flags: { deliveryPaymentEnabled: false, deliveryMockFeeEnabled: true } })
    await testApp.mount(); await testApp.primary()
    assert.equal(testApp.calls.mock.length, 1)
    assert.equal(testApp.calls.wallet.length, 0)
    console.log('PASS 真实支付与测试费用继续受原开关约束')
  }
  {
    const app = publishHarness({ toast: async () => { throw new Error('toast unavailable') } })
    await app.mount(); await app.submit()
    assert.equal(app.calls.create.length, 1)
    assert.equal(app.calls.wallet.length, 1)
    assert.equal(app.calls.navigate.length, 1)
    console.log('PASS 提示组件异常不覆盖支付成功结果')
  }
}

if (require.main === module) run().catch((error) => { console.error(error); process.exitCode = 1 })
module.exports = { publishHarness }
