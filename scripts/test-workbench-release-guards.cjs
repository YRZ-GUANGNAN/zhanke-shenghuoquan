const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const sass = require('sass')
const { deferred, riderHarness, merchantHarness, order, page, nodes, text, hasClass } = require('./test-delivery-stage-workbench.cjs')
const stage = app => nodes(app.tree, node => node.type === 'DeliveryStagePager')[0]
const nav = app => nodes(app.tree, node => node.type === 'DeliveryWorkspaceNav')[0]

async function run() {
  {
    const pending = deferred()
    const app = riderHarness({ services: { startToPickup: () => pending.promise } })
    try {
      await app.mount(); await app.tab('待取货')
      app.primary('ORDER-A').props.onClick(); await app.flush()
      assert.equal(stage(app).props.enabled, false, '订单提交中必须禁用原生阶段切换')
      assert.equal(nav(app).props.gestureGuard(), false, '订单提交中不得跨工作区')
      stage(app).props.onChange('delivery'); nav(app).props.onSelect('grab'); await app.flush()
      await app.tab('待送达')
      assert.equal(stage(app).props.activeKey, 'pickup', '处理中阶段、底栏及页签点击均保持当前任务')
      pending.resolve({ ...order('ORDER-A', 'to_pickup'), rowVersion: 2 }); await app.flush()
      assert.equal(stage(app).props.enabled, true)
      assert.equal(nav(app).props.gestureGuard(), true)
      await app.tab('待送达')
      assert.equal(stage(app).props.activeKey, 'delivery')
    } finally { await app.unmount() }
    console.log('PASS 提交中禁止阶段和工作区切换，处理完成后恢复')
  }
  {
    const app = riderHarness()
    try {
      await app.mount(); await app.tab('待取货')
      nodes(app.card('ORDER-A'), node => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
      await app.flush()
      assert.equal(stage(app).props.enabled, false)
      assert.equal(nav(app).props.gestureGuard(), false, '释放确认弹窗中不能通过底栏离开')
      await app.tab('待送达')
      assert.equal(stage(app).props.activeKey, 'pickup')
      nodes(app.tree, node => node.type === 'AppDialog')[0].props.onCancel(); await app.flush()
      assert.equal(stage(app).props.enabled, true)
      assert.equal(nav(app).props.gestureGuard(), true)
    } finally { await app.unmount() }
    console.log('PASS 释放确认弹窗期间所有工作区入口共用防误触边界')
  }
  for (const role of ['rider', 'merchant']) {
    let canActivate = true
    const app = role === 'rider' ? riderHarness({ canActivate: () => canActivate }) : merchantHarness('merchant-home', { canActivate: () => canActivate })
    try {
      await app.mount()
      if (role === 'rider') await app.tab('待取货')
      const initial = stage(app).props.activeKey
      canActivate = false
      await app.tab(role === 'rider' ? '待送达' : 'completed')
      assert.equal(stage(app).props.activeKey, initial, '滑动后的残留点击不得切换页签')
      canActivate = true
      await app.tab(role === 'rider' ? '待送达' : 'completed')
      assert.notEqual(stage(app).props.activeKey, initial)
    } finally { await app.unmount() }
    console.log(`PASS ${role} 页签沿用已存在的滑动后点击隔离`)
  }
  const samples = [
    { ...order('SPACE'), pickupCode: '   ' },
    { ...order('LONG'), pickupCode: '  AB123456789012  ' },
    { ...order('SHORT'), pickupCode: '22' },
  ]
  {
    const app = riderHarness({ services: { listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? samples : []) } })
    try {
      await app.mount(); await app.tab('待取货')
      const code = id => nodes(app.card(id), node => hasClass(node, 'rider-order-card__pickup-code'))[0]
      assert.ok(hasClass(code('SPACE'), 'rider-order-card__pickup-code--fallback'))
      assert.ok(hasClass(code('LONG'), 'rider-order-card__pickup-code--long'))
      assert.equal(hasClass(code('SHORT'), 'rider-order-card__pickup-code--long'), false)
    } finally { await app.unmount() }
    console.log('PASS 骑手空白取货号回退、长号保护，正常取货号不变')
  }
  {
    const app = merchantHarness('merchant-home', { listMerchantDeliveryOrders: async () => page([...samples, order('FAILED', 'payment_failed')]) })
    try {
      await app.mount()
      const cards = nodes(app.tree, node => hasClass(node, 'merchant-order-card'))
      assert.ok(nodes(cards[0], node => hasClass(node, 'merchant-order-card__pickup-code--fallback')).length)
      assert.ok(nodes(cards[1], node => hasClass(node, 'merchant-order-card__pickup-code--long')).length)
      const failure = cards.find(card => text(card).includes('支付失败'))
      assert.ok(nodes(failure, node => hasClass(node, 'merchant-order-card__status--danger')).length, '支付失败使用既有危险状态chip')
    } finally { await app.unmount() }
    console.log('PASS 商家空白取货号回退及支付失败危险语义')
  }
  const src = path.resolve(__dirname, '../apps/miniapp/src')
  const riderStyle = fs.readFileSync(path.join(src, 'pages/delivery-rider/index.scss'), 'utf8')
  const merchantStyle = fs.readFileSync(path.join(src, 'pages/merchant-home/index.scss'), 'utf8')
  assert.match(riderStyle, /\.rider-order-card__pickup-code--long\s*\{[^}]*font-size:\s*40rpx/)
  assert.match(riderStyle, /\.rider-order-card__pickup-code\s*\{[^}]*font-size:\s*64rpx/, '普通取货号保留v10主指标')
  assert.match(merchantStyle, /\.merchant-home-order-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/)
  assert.match(merchantStyle, /\.merchant-order-card\s*\{[^}]*padding:\s*22rpx 20rpx 18rpx/, '商家原卡片间距不变')
  assert.match(merchantStyle, /\.merchant-home-page \.merchant-order-card__status--danger\s*\{[^}]*background:\s*\$danger-tint/)
  for (const file of ['pages/delivery-rider/index.scss', 'pages/merchant-home/index.scss']) {
    sass.compile(path.join(src, file), { logger: sass.Logger.silent })
  }
  console.log('PASS 保留v10取货号、商家双列及原间距，新增样式可编译')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
