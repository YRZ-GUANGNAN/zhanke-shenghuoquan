// 验证骑手 T+1 资金字段的前端规范化与页面展示。测试不访问网络、不触发支付或提现写入。
const assert = require('node:assert/strict')
const { loadSource, loadPage, hookRenderer, nodes, text, hasClass } = require('./test-delivery-stage-workbench.cjs')

const requestHarness = (responses) => {
  const calls = []
  const service = loadSource('services/delivery-rider-wallet.ts', {
    './api': {
      request: async (url, options) => {
        calls.push({ url, options })
        const value = responses[url]
        if (typeof value === 'function') return value(url, options)
        if (value instanceof Error) throw value
        return value
      },
    },
    './delivery-merchant': { createDeliveryClientRequestId: () => '01ARZ3NDEKTSV4RRFFQ69G5FAV' },
  }, hookRenderer())
  return { service, calls }
}

const wallet = {
  walletState: 'ready', walletPublicId: '01ARZ3NDEKTSV4RRFFQ69G5FAA',
  availableBalanceCents: 1200, reservedCents: 200, unverifiedCents: 300,
  pendingSettlementCents: 500, nextSettlementAt: '2026-11-03T16:00:00.000Z', updatedAt: '2026-11-02T16:00:00.000Z',
}
const earning = (value) => ({ earningPublicId: value.id, deliveryOrderPublicId: `ORDER-${value.id}`, amountCents: value.amountCents || 100, settlementStatus: value.status, settleAt: value.settleAt, settledAt: value.settledAt, paidOfflineAt: value.paidOfflineAt, createdAt: value.createdAt || '2026-11-02T08:00:00.000Z' })

async function expectReject(promise, message = '资金记录格式异常') {
  await assert.rejects(promise, error => error instanceof Error && error.message.includes(message))
}

async function runServiceAssertions() {
  {
    const { service } = requestHarness({ '/delivery/rider/wallet': wallet })
    const value = await service.getRiderWallet()
    assert.equal(value.pendingSettlementCents, 500)
    assert.equal(value.nextSettlementAt, wallet.nextSettlementAt)
    assert.equal(value.updatedAt, wallet.updatedAt)
    console.log('PASS 钱包规范化保留预计结算金额与带时区时间')
  }
  {
    const legacy = { walletState: 'ready', walletPublicId: wallet.walletPublicId, availableBalanceCents: 1200, reservedCents: 200, unverifiedCents: 300 }
    const { service } = requestHarness({ '/delivery/rider/wallet': legacy })
    const value = await service.getRiderWallet()
    assert.equal(value.pendingSettlementCents, undefined)
    assert.equal(value.nextSettlementAt, undefined)
    assert.equal(value.unverifiedCents, 300)
    assert.equal(value.withdrawalEnabled, false, '旧服务端缺失提现能力必须关闭')
    console.log('PASS 旧服务端缺失结算字段时保持 undefined，不以待核实收入补值')
  }
  for (const withdrawalEnabled of [false, true, undefined, null, 'true', 1]) {
    const { service } = requestHarness({ '/delivery/rider/wallet': { ...wallet, withdrawalEnabled } })
    assert.equal((await service.getRiderWallet()).withdrawalEnabled, withdrawalEnabled === true)
  }
  console.log('PASS 提现能力只接受明确 true，缺失或非布尔值均关闭')
  for (const invalid of [
    { ...wallet, availableBalanceCents: -1 },
    { ...wallet, availableBalanceCents: 1.2 },
    { ...wallet, reservedCents: 1300 },
    { ...wallet, pendingSettlementCents: -1 },
  ]) {
    const { service } = requestHarness({ '/delivery/rider/wallet': invalid })
    await expectReject(service.getRiderWallet())
  }
  console.log('PASS 钱包负数、小数、占用超余额与负待结算金额全部拒绝')
  for (const nextSettlementAt of ['2026-11-03 00:00:00', '2026-11-03T00:00:00', 'not-a-time']) {
    const { service } = requestHarness({ '/delivery/rider/wallet': { ...wallet, nextSettlementAt } })
    await expectReject(service.getRiderWallet())
  }
  console.log('PASS 钱包结算时间必须是带时区的合法 ISO 时间')
  {
    const items = [
      earning({ id: 'E-PENDING', status: 'pending', settleAt: '2026-11-03T16:00:00.000Z' }),
      earning({ id: 'E-SETTLED', status: 'settled', settleAt: '2026-11-02T16:00:00.000Z', settledAt: '2026-11-02T16:01:00.000Z' }),
      earning({ id: 'E-OFFLINE', status: 'paid_offline', settleAt: '2026-11-01T16:00:00.000Z', settledAt: '2026-11-01T16:01:00.000Z', paidOfflineAt: '2026-11-02T10:00:00.000Z' }),
    ]
    const { service } = requestHarness({ '/delivery/rider/wallet/earnings?page=1&pageSize=20': { items, total: 3, page: 1, pageSize: 20 } })
    const value = await service.listRiderEarnings()
    assert.deepEqual(value.items.map(item => item.settlementStatus), ['pending', 'settled', 'paid_offline'])
    assert.equal(value.items[0].settleAt, items[0].settleAt)
    assert.equal(value.items[1].settledAt, items[1].settledAt)
    assert.equal(value.items[2].paidOfflineAt, items[2].paidOfflineAt)
    console.log('PASS 收入记录保留 pending、settled、paid_offline 三种结算状态与时间')
  }
  for (const invalid of [
    { ...earning({ id: 'BAD', status: 'unknown' }) },
    { ...earning({ id: 'BAD', status: 'pending', amountCents: -1 }) },
    { ...earning({ id: 'BAD', status: 'pending', createdAt: '2026-11-02 08:00:00' }) },
  ]) {
    const { service } = requestHarness({ '/delivery/rider/wallet/earnings?page=1&pageSize=20': { items: [invalid], total: 1, page: 1, pageSize: 20 } })
    await expectReject(service.listRiderEarnings())
  }
  console.log('PASS 收入状态、金额与创建时间格式异常全部拒绝')
}

function riderWalletHarness() {
  const renderer = hookRenderer()
  let show
  const session = () => ({ profile: { id: 'RIDER-A' }, token: 'TOKEN-RIDER-A' })
  const store = select => select(session()); store.getState = session
  const services = {
    getRiderWallet: async () => wallet,
    listRiderEarnings: async () => ({
      items: [
        earning({ id: 'E-PENDING', status: 'pending', settleAt: '2026-11-03T16:00:00.000Z' }),
        earning({ id: 'E-SETTLED', status: 'settled', settleAt: '2026-11-02T16:00:00.000Z', settledAt: '2026-11-02T16:01:00.000Z' }),
        earning({ id: 'E-OFFLINE', status: 'paid_offline', settleAt: '2026-11-01T16:00:00.000Z', settledAt: '2026-11-01T16:01:00.000Z', paidOfflineAt: '2026-11-02T10:00:00.000Z', createdAt: '2026-10-31T08:00:00.000Z' }),
      ], total: 3, page: 1, pageSize: 20,
    }),
    listRiderLedger: async () => ({ items: [], total: 0, page: 1, pageSize: 20 }),
    listRiderPayoutQueue: async () => ({ items: [], total: 0, page: 1, pageSize: 20 }),
    listRiderWithdrawals: async () => ({ items: [], total: 0, page: 1, pageSize: 20 }),
  }
  loadPage('rider-wallet', {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView', Input: 'Input' },
    '@tarojs/taro': { useDidShow: fn => { show = fn }, useDidHide: () => {} },
    '../../components/capsule-page-header': { CapsuleSystemRow: 'CapsuleSystemRow' },
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../components/ui': { AppButton: 'AppButton', AppIconButton: 'AppIconButton', AppNotice: 'AppNotice', AppActionSheet: 'AppActionSheet' },
    '../../hooks/use-identity-guard': { useIdentityGuard: () => true },
    '../../services/delivery-rider': { getDeliveryRiderTodayStats: async () => ({ todayOrders: 3, todayIncomeCents: 360 }) },
    '../../services/delivery-rider-wallet': services,
    '../../services/delivery-merchant': { createDeliveryClientRequestId: () => '01ARZ3NDEKTSV4RRFFQ69G5FAV' },
    '../../services/api': { isApiError: () => false },
    './withdrawal-state': { clearPendingRiderWithdrawal: () => {}, parseWithdrawalCents: () => null, readPendingRiderWithdrawal: () => null, savePendingRiderWithdrawal: () => {} },
    '../../services/taro': { navigateBack: async () => {}, showToast: async () => {} },
    '../../stores/session': { useSessionStore: store },
    './index.scss': {},
  }, renderer)
  return { renderer, async mount() { renderer.render(); show?.(); await renderer.flush() } }
}

async function runPageAssertions() {
  const app = riderWalletHarness()
  await app.mount()
  const rendered = text(app.renderer.tree)
  assert.match(rendered, /预计待结算收入/)
  assert.match(rendered, /¥5\.00/)
  assert.match(rendered, /预计 11月4日 00:00（北京时间）结算/)
  assert.match(rendered, /待结算/)
  assert.match(rendered, /已结算/)
  assert.match(rendered, /后台已登记/)
  assert.match(rendered, /已计入平台余额，不代表外部账户到账/)
  assert.match(rendered, /后台已登记打款，以收款记录为准/)
  const earningRows = nodes(app.renderer.tree, node => hasClass(node, 'rider-wallet-list-item'))
  assert.equal(earningRows.filter(node => text(node).includes('E-SETTLED') && text(node).includes('待结算')).length, 0, '已结算不得显示为待结算')
  assert.doesNotMatch(rendered, /上滑/)
  console.log('PASS 骑手钱包展示预计结算、三种状态说明，并隔离无关沉浸式文案')
}

async function run() {
  await runServiceAssertions()
  await runPageAssertions()
}

if (require.main === module) run().catch(error => { console.error(error); process.exitCode = 1 })
