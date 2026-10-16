// Exercise the real page handlers with an isolated hook renderer and deferred
// service responses. No backend, account, media upload or production write.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const root = path.resolve(__dirname, '..')
function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
function page(items) { return { items, total: items.length, page: 1, pageSize: 50 } }
function order(id, status = 'claimed') {
  return {
    deliveryOrderPublicId: id, status, rowVersion: 1,
    pickupContactName: '测试取货人', pickupContactPhone: '', pickupAddress: '测试取货地址',
    recipientName: '测试收货人', recipientPhone: '', deliveryAddress: '测试送达地址',
    remark: '', routeSummary: '测试取货地址 → 测试送达地址', deliveryFeeCents: 150,
    createdAt: '2026-10-16T10:00:00+08:00', updatedAt: '2026-10-16T10:00:00+08:00',
  }
}
function nodes(tree, predicate) {
  if (Array.isArray(tree)) return tree.flatMap((child) => nodes(child, predicate))
  if (!tree || typeof tree !== 'object') return []
  return [...(predicate(tree) ? [tree] : []), ...nodes(tree.props?.children, predicate)]
}
function text(tree) {
  if (Array.isArray(tree)) return tree.map(text).join('')
  if (tree == null || typeof tree === 'boolean') return ''
  return typeof tree === 'object' ? text(tree.props?.children) : String(tree)
}
const hasClass = (node, name) => String(node.props?.className || '').split(/\s+/).includes(name)

function hookRenderer() {
  let cursor = 0, tree, component
  let mounted = true
  const slots = []
  const effects = []
  const sameDeps = (a, b) => Array.isArray(a) && Array.isArray(b)
    && a.length === b.length && a.every((value, index) => Object.is(value, b[index]))
  const memo = (factory, deps) => {
    const index = cursor++
    if (!slots[index] || !sameDeps(slots[index].deps, deps)) {
      slots[index] = { value: factory(), deps }
    }
    return slots[index].value
  }
  const React = {
    createElement(type, props, ...children) {
      const resolvedProps = { ...props, children }
      // These pages have a single stateful content component; the other local
      // components are pure. Render them too so assertions inspect visible UI.
      return typeof type === 'function' ? type(resolvedProps) : { type, props: resolvedProps }
    },
    Fragment: 'Fragment',
    useState(initial) {
      const index = cursor++
      if (!(index in slots)) {
        const state = { value: typeof initial === 'function' ? initial() : initial }
        state.set = (value) => { state.value = typeof value === 'function' ? value(state.value) : value }
        slots[index] = state
      }
      return [slots[index].value, slots[index].set]
    },
    useRef(initial) {
      const index = cursor++
      if (!(index in slots)) slots[index] = { current: initial }
      return slots[index]
    },
    useCallback(fn, deps) { return memo(() => fn, deps) },
    useMemo(factory, deps) { return memo(factory, deps) },
    useEffect(fn, deps) {
      const index = cursor++
      if (!slots[index] || !sameDeps(slots[index].deps, deps)) {
        const previous = slots[index]
        const current = { deps }
        slots[index] = current
        effects.push(() => { previous?.cleanup?.(); current.cleanup = fn() })
      }
    },
  }
  function render() {
    if (!mounted) return tree
    cursor = 0
    tree = component()
    effects.splice(0).forEach((effect) => effect())
    return tree
  }
  async function flush() {
    // Drain service promises and render state updates, without wall-clock waits.
    for (let i = 0; i < 6; i++) { await new Promise(setImmediate); render() }
  }
  function unmount() {
    mounted = false
    slots.forEach((slot) => slot?.cleanup?.())
  }
  return { React, render, flush, unmount, setComponent(value) { component = value }, get tree() { return tree } }
}

function loadPage(relativePath, imports, renderer) {
  const filename = path.join(root, 'apps/miniapp/src/pages', relativePath, 'index.tsx')
  const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.React, target: ts.ScriptTarget.ES2020, esModuleInterop: true },
    fileName: filename,
  }).outputText
  const loaded = new Module(filename, module)
  loaded.filename = filename
  loaded.require = (id) => {
    assert.ok(id === 'react' || id in imports, `测试必须显式隔离依赖：${id}`)
    return id === 'react' ? renderer.React : imports[id]
  }
  loaded._compile(compiled, filename)
  renderer.setComponent(loaded.exports.default)
  return loaded.exports
}

function riderHarness(overrides = {}) {
  let show, hide, identityAllowed = true, accountId = 101
  let identity = { currentIdentity: 'rider' }, switchCalls = 0, relaunches = 0
  const switchIdentity = async () => {
    switchCalls++
    const result = overrides.switchIdentity ? await overrides.switchIdentity() : { currentIdentity: 'student' }
    identity = result
    return result
  }
  const session = () => ({ profile: { id: accountId }, token: `TOKEN-${accountId}`, status: 'authenticated', identity, switchIdentity })
  const sessionStore = (select) => select(session())
  sessionStore.getState = session
  const renderer = hookRenderer()
  const toasts = []
  const phoneCalls = [], navigations = []
  let meCalls = 0
  const services = {
    getDeliveryRiderMe: async () => {
      meCalls++
      return { registered: true, rider: { status: 'active', realName: '测试骑手' } }
    },
    listClaimableDeliveryOrders: async () => page([]),
    listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [order('ORDER-A'), order('ORDER-B')] : []),
    startToPickup: async (value) => ({ ...value, rowVersion: 2, status: 'to_pickup' }),
    startDelivery: async (value) => ({ ...value, rowVersion: 2, status: 'delivering' }),
    claimDeliveryOrder: async (value) => ({ ...value, rowVersion: 2, status: 'claimed' }),
    markPickedUp: async (value) => ({ ...value, rowVersion: 2, status: 'picked_up' }),
    markDelivered: async (value) => ({ ...value, rowVersion: 2, status: 'delivered' }),
    releaseDeliveryOrder: async (value) => ({ ...value, rowVersion: 2, status: 'ready_for_claim' }),
    getAssignedDeliveryOrder: async (id) => {
      for (const group of ['pickup', 'delivery', 'completed']) {
        const result = await services.listAssignedDeliveryOrders(1, 50, group)
        const found = result.items.find((item) => item.deliveryOrderPublicId === id)
        if (found) return found
      }
      throw new Error('配送单不存在')
    },
    ...overrides.services,
  }
  const media = {
    chooseAndUploadImages: async (_, __, progress) => {
      progress?.({ message: '上传中', percent: 70 })
      return [{ assetId: 'TEST-ASSET' }]
    },
    waitForMediaApproval: async () => ({ canUse: true, state: 'approved' }),
    ...overrides.media,
  }
  const taro = {
    showToast: async (value) => { toasts.push(value) },
    showLoading: () => { throw new Error('不得使用全屏 loading 阻塞其他订单') },
    hideLoading: () => {},
    navigateTo: async (value) => { navigations.push(value); await overrides.navigateTo?.(value) },
    makePhoneCall: async (value) => { phoneCalls.push(value); await overrides.makePhoneCall?.(value) },
    reLaunch: async () => { relaunches++ },
  }
  const imports = {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView' },
    '@tarojs/taro': {
      getCurrentInstance: () => ({ router: { params: {} } }),
      useDidShow: (fn) => { show = fn },
      useDidHide: (fn) => { hide = fn },
    },
    '../../components/capsule-page-header': { CapsuleSystemRow: 'CapsuleSystemRow' },
    '../../components/delivery-workspace-nav': 'DeliveryWorkspaceNav',
    '../../components/ui': { AppDialog: 'AppDialog' },
    '../../hooks/use-identity-guard': { useIdentityGuard: () => identityAllowed, identityRoute: () => '' },
    '../../hooks/use-feature-visibility': { useFeatureVisibility: () => ({ delivery: true, rider: true }) },
    '../../services/api': {
      isApiError: (value) => value instanceof Error && typeof value.status === 'number',
      isRetryableNetworkError: (value) => value?.status === 0,
    },
    '../../services/delivery-rider': services,
    '../../services/delivery-merchant': { centsToYuan: (value) => String(value / 100) },
    '../../services/media': media,
    '../../services/taro': taro,
    '../../stores/session': { useSessionStore: sessionStore },
    '../../utils/delivery-order-source': { deliverySourceLabel: () => '校内配送', deliveryPickupLabel: (value) => value.pickupCode ? `${value.pickupCode}号` : '配送单' },
    './rider-workbench-icon': 'Icon',
    './index.scss': {},
  }
  loadPage('delivery-rider', imports, renderer)
  const { render, flush } = renderer
  function card(id) { return nodes(renderer.tree, (node) => hasClass(node, 'rider-order-card') && node.props.key === id)[0] }
  function primary(id) {
    const found = nodes(card(id), (node) => hasClass(node, 'rider-order-card__grab'))[0]
    assert.ok(found, `${id} 应有当前阶段主操作`)
    return found
  }
  return {
    services, toasts, phoneCalls, navigations, render, flush, card, primary,
    get tree() { return renderer.tree }, get meCalls() { return meCalls },
    get switchCalls() { return switchCalls }, get relaunches() { return relaunches },
    async mount() { render(); show(); await flush() },
    async setIdentityAllowed(value) { identityAllowed = value; render(); await flush() },
    async setAccount(value) { accountId = value; render(); await flush() },
    async unmount() { renderer.unmount(); await flush() },
    async hide() { hide?.(); await flush() },
    async tab(label) {
      const tab = nodes(renderer.tree, (node) => hasClass(node, 'rider-grab-tabs__item') && text(node).startsWith(label))[0]
      assert.ok(tab, `${label} 标签应存在`)
      tab.props.onClick(); await flush()
    },
  }
}

function merchantHarness(kind, overrides = {}) {
  let show, hide, orderCalls = 0, identityAllowed = true, accountId = 101
  let sessionToken = 'TOKEN-101', routeOrderId = 'ORDER-A'
  let identity = { currentIdentity: 'merchant' }, switchCalls = 0, relaunches = 0
  const switchIdentity = async () => {
    switchCalls++
    const result = overrides.switchIdentity ? await overrides.switchIdentity() : { currentIdentity: 'student' }
    identity = result
    return result
  }
  const session = () => ({ profile: { id: accountId }, token: sessionToken, status: 'authenticated', identity, switchIdentity })
  const sessionStore = (select) => select(session())
  sessionStore.getState = session
  const renderer = hookRenderer()
  const previews = [], proofRequests = [], navigations = [], toasts = []
  const services = {
    listMerchantDeliveryOrders: async () => page([]),
    getMerchantDeliveryOrder: async () => order('ORDER-A'),
    getDeliveryOrderFunds: async () => ({ deliveryOrderPublicId: 'ORDER-A', payment: null, settlement: null, refund: null }),
    loadPrivateDeliveryProofs: async () => [],
    ...overrides,
  }
  const imports = {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView', Image: 'Image' },
    '@tarojs/taro': { useDidShow: (fn) => { show = fn }, useDidHide: (fn) => { hide = fn } },
    '../../components/capsule-page-header': {
      __esModule: true,
      default: ({ left, right }) => ({ type: 'Header', props: { children: [left, right] } }),
      CapsuleSystemRow: 'CapsuleSystemRow',
    },
    '../../components/ui': { AppIconButton: 'AppIconButton' },
    '../../components/IdentitySwitchModal': 'IdentitySwitchModal',
    '../../components/delivery-workspace-nav': 'DeliveryWorkspaceNav',
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../feature-flags': { deliveryPaymentEnabled: true },
    '../../hooks/use-identity-guard': { useIdentityGuard: () => identityAllowed, identityRoute: () => '' },
    '../../hooks/use-feature-visibility': { useFeatureVisibility: () => ({ delivery: true, merchant: true }) },
    '../../hooks/use-viewport-refresh': { useViewportRefresh: () => ({}) },
    '../../services/a-user': { getIdentityApplication: async () => ({ status: 'approved', merchantName: '测试商家', merchantAddress: '测试取货点' }) },
    '../../services/delivery-merchant': {
      centsToYuan: (value) => String(value / 100),
      listMerchantDeliveryOrders: (...args) => services.listMerchantDeliveryOrders(...args),
      getMerchantDeliveryOrder: (...args) => { orderCalls++; return services.getMerchantDeliveryOrder(...args) },
    },
    '../../services/delivery-payment': {
      getDeliveryWallet: async () => ({ availableBalanceCents: 1000 }),
      getDeliveryOrderFunds: (...args) => services.getDeliveryOrderFunds(...args),
    },
    '../../services/delivery-proof': { loadPrivateDeliveryProofs: (...args) => { proofRequests.push(args); return services.loadPrivateDeliveryProofs(...args) } },
    '../../services/taro': {
      getCurrentInstance: () => ({ router: { params: { orderId: routeOrderId } } }),
      previewImage: async (value) => { previews.push(value); await overrides.previewImage?.(value) },
      showToast: async (value) => { toasts.push(value) },
      navigateTo: async (value) => { navigations.push(value); await overrides.navigateTo?.(value) },
      navigateBack: async () => {}, reLaunch: async () => { relaunches++ },
    },
    '../../stores/session': { useSessionStore: sessionStore },
    '../../utils/delivery-order-source': { deliverySourceLabel: () => '校内配送', deliveryPickupLabel: (value) => value.pickupCode ? `${value.pickupCode}号` : '配送单' },
    './merchant-home-icon': 'Icon',
    '../merchant-home/merchant-home-icon': 'Icon',
    './index.scss': {},
  }
  loadPage(kind, imports, renderer)
  const one = (className) => {
    const found = nodes(renderer.tree, (node) => hasClass(node, className))[0]
    assert.ok(found, `应渲染 ${className}`)
    return found
  }
  return {
    services, one, previews, proofRequests, navigations, toasts, flush: renderer.flush,
    get tree() { return renderer.tree }, get orderCalls() { return orderCalls },
    get switchCalls() { return switchCalls }, get relaunches() { return relaunches },
    async mount() { renderer.render(); show?.(); await renderer.flush() },
    async setIdentityAllowed(value) { identityAllowed = value; renderer.render(); await renderer.flush() },
    async setAccount(value) { accountId = value; sessionToken = `TOKEN-${value}`; renderer.render(); await renderer.flush() },
    async setToken(value) { sessionToken = value; renderer.render(); await renderer.flush() },
    async setOrderId(value) { routeOrderId = value; renderer.render(); await renderer.flush() },
    async unmount() { renderer.unmount(); await renderer.flush() },
    async hide() { hide?.(); await renderer.flush() },
    async show() { show?.(); await renderer.flush() },
    async toggleProgress() { one('merchant-detail-section-toggle').props.onClick(); await renderer.flush() },
    async preview(proofId) {
      const selected = proofId ? nodes(renderer.tree, (node) => hasClass(node, 'merchant-detail-photo-item') && node.props.key === proofId)[0] : renderer.tree
      const button = proofId ? nodes(selected, (node) => hasClass(node, 'merchant-detail-photo-item__thumb'))[0] : one('merchant-detail-photo-card__view')
      assert.equal(typeof button?.props.onClick, 'function', '应可查看已激活配送照片')
      button.props.onClick(); await renderer.flush()
    },
    async refresh() {
      if (kind === 'merchant-home') show()
      else one('merchant-detail-nav__refresh').props.onClick()
      await renderer.flush()
    },
    async more() { one('merchant-home-orders').props.onScrollToLower(); await renderer.flush() },
    async tab(key) {
      const found = nodes(renderer.tree, (node) => hasClass(node, 'merchant-home-tab') && node.props.key === key)[0]
      assert.ok(found, `应渲染 ${key} 分组`)
      found.props.onClick(); await renderer.flush()
    },
    count(key) {
      const found = nodes(renderer.tree, (node) => hasClass(node, 'merchant-home-tab') && node.props.key === key)[0]
      return text(nodes(found, (node) => hasClass(node, 'merchant-home-tab__count')))
    },
    serials() { return nodes(renderer.tree, (node) => hasClass(node, 'merchant-order-card')).map((node) => node.props.key) },
    timeline() {
      return nodes(renderer.tree, (node) => hasClass(node, 'merchant-detail-timeline__row')).map((row) => ({
        key: row.props.key,
        title: text(nodes(row, (node) => hasClass(node, 'merchant-detail-timeline__title'))),
        time: text(nodes(row, (node) => hasClass(node, 'merchant-detail-timeline__time'))),
      }))
    },
  }
}

async function run() {
  {
    let writes = 0, failReads = false
    const app = riderHarness({ services: {
      startToPickup: async () => { writes++; failReads = true; throw Object.assign(new Error('网络请求超时'), { status: 0 }) },
      listAssignedDeliveryOrders: async (_, __, group) => {
        if (failReads) throw new Error('状态读取失败')
        return page(group === 'pickup' ? [order('ORDER-A')] : [])
      },
    } })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.match(text(app.card('ORDER-A')), /结果待确认/)
    assert.equal(text(app.primary('ORDER-A')), '刷新状态')
    app.primary('ORDER-A').props.onClick(); app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.equal(writes, 1, '未知写结果在读取失败时不能再次提交')
    failReads = false
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.equal(writes, 1, '同步成功也不应自动重试写操作')
    assert.equal(text(app.primary('ORDER-A')), '前往取货')
    console.log('PASS 写请求结果未知必须先同步，读取失败不能重复提交')
  }
  {
    let writes = 0
    const app = riderHarness({ services: {
      releaseDeliveryOrder: async () => { writes++; throw Object.assign(new Error('该订单暂不可释放'), { status: 409 }) },
    } })
    await app.mount(); await app.tab('待取货')
    nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
    await app.flush()
    nodes(app.tree, (node) => node.type === 'AppDialog')[0].props.onConfirm(); await app.flush()
    const dialog = nodes(app.tree, (node) => node.type === 'AppDialog')[0]
    assert.equal(writes, 1)
    assert.equal(dialog.props.visible, true, '释放明确失败必须保留确认框')
    assert.match(dialog.props.error, /暂不可释放/, '失败原因应留在确认操作附近')
    console.log('PASS 释放业务拒绝保留确认框与真实原因')
  }
  {
    for (const mismatch of ['status', 'version', 'id', 'read-failure']) {
      const approval = deferred()
      let writes = 0, preflightCalls = 0
      const initial = order('ORDER-A', 'to_pickup')
      const app = riderHarness({ services: {
        listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [initial] : []),
        getAssignedDeliveryOrder: async () => {
          preflightCalls++
          if (mismatch === 'read-failure') throw new Error('订单校验读取失败')
          return { ...initial, ...(mismatch === 'status' ? { status: 'picked_up' } : mismatch === 'version' ? { rowVersion: 2 } : { deliveryOrderPublicId: 'ORDER-B' }) }
        },
        markPickedUp: async () => { writes++ },
      }, media: { waitForMediaApproval: () => approval.promise } })
      await app.mount(); await app.tab('待取货')
      app.primary('ORDER-A').props.onClick(); await app.flush()
      assert.equal(preflightCalls, 0, '凭证未审核完成前不应发履约前校验')
      approval.resolve({ canUse: true, state: 'approved' }); await app.flush()
      assert.equal(preflightCalls, 1)
      assert.equal(writes, 0, `${mismatch} 时不得提交旧凭证动作`)
      assert.match(text(app.card('ORDER-A')), /订单|校验/)
    }
    console.log('PASS 凭证审核后的订单阶段、版本、身份及读取失败均停止旧提交')
  }
  {
    const a = deferred(), b = deferred(), calls = []
    const app = riderHarness({ services: { startToPickup: (value) => {
      calls.push(value.deliveryOrderPublicId)
      return value.deliveryOrderPublicId === 'ORDER-A' ? a.promise : b.promise
    } } })
    await app.mount(); await app.tab('待取货')
    const action = app.primary('ORDER-A').props.onClick
    action(); action()
    await app.flush()
    assert.deepEqual(calls, ['ORDER-A'], '同一渲染帧双击只提交一次')
    assert.match(app.primary('ORDER-A').props.className, /disabled/)
    assert.doesNotMatch(app.primary('ORDER-B').props.className, /disabled/)
    app.primary('ORDER-B').props.onClick()
    assert.deepEqual(calls, ['ORDER-A', 'ORDER-B'], 'A 正在提交时 B 仍可操作')
    a.resolve(order('ORDER-A', 'to_pickup')); b.resolve(order('ORDER-B', 'to_pickup'))
    await app.flush()
    assert.ok(app.meCalls > 1, '动作后应重新同步骑手状态')
    console.log('PASS 同单防重复与不同订单并发')
  }
  {
    let writes = 0, selections = 0, failReads = false
    const app = riderHarness({ services: {
      listAssignedDeliveryOrders: async (_, __, group) => {
        if (failReads) throw new Error('订单同步失败')
        return page(group === 'pickup' ? [order('ORDER-A', 'to_pickup')] : [])
      },
      markPickedUp: async () => { writes++; failReads = true; throw Object.assign(new Error('响应中断'), { status: 502 }) },
    }, media: { chooseAndUploadImages: async () => { selections++; return [{ assetId: 'TEST-ASSET' }] } } })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.match(text(app.card('ORDER-A')), /结果待确认/)
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.equal(writes, 1)
    assert.equal(selections, 1, '凭证提交结果未知的同步不能再次选择和上传照片')
    console.log('PASS 凭证提交结果未知时只同步，不重新选图或重复履约')
  }
  {
    for (const mismatch of ['status', 'version', 'id', 'read-failure']) {
      let writes = 0
      const app = riderHarness({ services: {
        getAssignedDeliveryOrder: async () => {
          if (mismatch === 'read-failure') throw new Error('订单预检失败')
          return { ...order('ORDER-A'), ...(mismatch === 'status' ? { status: 'picked_up' } : mismatch === 'version' ? { rowVersion: 2 } : { deliveryOrderPublicId: 'ORDER-B' }) }
        },
        releaseDeliveryOrder: async () => { writes++ },
      } })
      await app.mount(); await app.tab('待取货')
      nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
      await app.flush()
      nodes(app.tree, (node) => node.type === 'AppDialog')[0].props.onConfirm(); await app.flush()
      const dialog = nodes(app.tree, (node) => node.type === 'AppDialog')[0]
      assert.equal(writes, 0, `${mismatch} 时不得释放旧订单快照`)
      assert.equal(dialog.props.visible, true)
      assert.equal(dialog.props.confirmText, '刷新状态')
      assert.match(dialog.props.error, /未提交/)
    }
    console.log('PASS 释放前校验阶段、版本、订单标识及读取失败均阻止旧确认')
  }
  {
    let current = order('ORDER-A'), released = false
    const writes = []
    const app = riderHarness({ services: {
      listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' && !released ? [current] : []),
      getAssignedDeliveryOrder: async () => current,
      releaseDeliveryOrder: async (value) => { writes.push(value.rowVersion); released = true; return { ...value, status: 'ready_for_claim' } },
    } })
    await app.mount(); await app.tab('待取货')
    nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
    await app.flush()
    current = { ...current, rowVersion: 2 }
    const dialog = () => nodes(app.tree, (node) => node.type === 'AppDialog')[0]
    dialog().props.onConfirm(); await app.flush()
    assert.equal(dialog().props.confirmText, '刷新状态')
    dialog().props.onConfirm(); await app.flush()
    assert.deepEqual(writes, [], '同步新版本不自动释放')
    assert.equal(dialog().props.visible, true)
    assert.equal(dialog().props.confirmText, '确认释放')
    assert.match(dialog().props.error, /再次确认/)
    dialog().props.onConfirm(); await app.flush()
    assert.deepEqual(writes, [2], '显式再确认使用同步后的新版本')
    assert.equal(dialog().props.visible, false, '释放确认成功才自动关闭')
    console.log('PASS 释放同步新版本后需再次确认，成功才关闭弹窗')
  }
  {
    let writes = 0, released = false
    const app = riderHarness({ services: {
      listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' && !released ? [order('ORDER-A')] : []),
      releaseDeliveryOrder: async () => { writes++; released = true; throw Object.assign(new Error('响应丢失'), { status: 0 }) },
    } })
    await app.mount(); await app.tab('待取货')
    nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
    await app.flush()
    const dialog = () => nodes(app.tree, (node) => node.type === 'AppDialog')[0]
    dialog().props.onConfirm(); await app.flush()
    assert.equal(dialog().props.visible, true, '只读同步不应被当成释放成功')
    assert.equal(dialog().props.confirmText, '返回工作台')
    assert.ok(!app.toasts.some((toast) => toast.icon === 'success'))
    dialog().props.onConfirm(); await app.flush()
    assert.equal(writes, 1)
    assert.equal(dialog().props.visible, false, '用户可以手动退出已失效的旧确认')
    console.log('PASS 释放响应未知且订单已变更时只返回工作台，不伪造成功或再释放')
  }
  {
    for (const interruption of ['identity', 'unmount']) {
      for (const action of ['release', 'proof']) {
        const preflight = deferred()
        let writes = 0
        const current = order('ORDER-A', action === 'proof' ? 'to_pickup' : 'claimed')
        const app = riderHarness({ services: {
          listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [current] : []),
          getAssignedDeliveryOrder: () => preflight.promise,
          releaseDeliveryOrder: async () => { writes++ },
          markPickedUp: async () => { writes++ },
        } })
        await app.mount(); await app.tab('待取货')
        if (action === 'release') {
          nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary--danger'))[0].props.onClick()
          await app.flush()
          nodes(app.tree, (node) => node.type === 'AppDialog')[0].props.onConfirm()
        } else app.primary('ORDER-A').props.onClick()
        await app.flush()
        if (interruption === 'identity') await app.setIdentityAllowed(false)
        else await app.unmount()
        preflight.resolve(current); await app.flush()
        assert.equal(writes, 0, `${action} 的订单预检等待中 ${interruption} 后不得写入`)
        assert.equal(app.toasts.length, 0)
      }
    }
    console.log('PASS 订单预检等待中切换身份或卸载不会继续释放或提交凭证')
  }
  {
    let commands = 0, failLists = false
    const app = riderHarness({ services: {
      listAssignedDeliveryOrders: async (_, __, group) => {
        if (failLists) throw new Error('测试网络失败')
        return page(group === 'pickup' ? [order('ORDER-A')] : [])
      },
      startToPickup: async (value) => { commands++; failLists = true; return { ...value, status: 'to_pickup' } },
    } })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.ok(app.toasts.some((item) => item.icon === 'success'))
    assert.ok(!app.toasts.some((item) => item.title.includes('操作失败')), '刷新失败不能改写命令成功')
    assert.match(text(app.tree), /操作已提交/)
    assert.equal(text(app.primary('ORDER-A')), '刷新状态')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.equal(commands, 1, '同步失败后重试只读状态，不重复履约提交')
    console.log('PASS 提交成功与刷新失败分离')
  }
  {
    const approval = deferred()
    let submitted = 0
    const app = riderHarness({
      services: {
        listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [order('ORDER-A', 'to_pickup'), order('ORDER-B')] : []),
        markPickedUp: async (value) => { submitted++; return { ...value, status: 'picked_up' } },
      },
      media: { waitForMediaApproval: async (_, progress) => { progress({ message: '等待审核' }); return approval.promise } },
    })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.match(text(app.card('ORDER-A')), /等待审核/)
    assert.equal(submitted, 0, '审核未通过不得提交取货')
    assert.doesNotMatch(app.primary('ORDER-B').props.className, /disabled/)
    approval.reject(new Error('照片未通过内容安全审核'))
    await app.flush()
    assert.equal(submitted, 0)
    assert.match(text(app.card('ORDER-A')), /审核/)
    assert.doesNotMatch(app.primary('ORDER-A').props.className, /disabled/, '审核失败释放当前订单锁')
    console.log('PASS 凭证审核等待与失败不会提前履约')
  }
  {
    let submitted = 0
    const app = riderHarness({
      services: {
        listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [order('ORDER-A', 'to_pickup')] : []),
        markPickedUp: async () => { submitted++ },
      },
      media: { chooseAndUploadImages: async () => { throw { errMsg: 'chooseImage:fail cancel' } } },
    })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    assert.equal(submitted, 0)
    assert.equal(app.toasts.length, 0, '取消照片不报提交失败')
    assert.doesNotMatch(app.primary('ORDER-A').props.className, /disabled/)
    console.log('PASS 用户取消照片后可继续操作')
  }
  {
    const app = riderHarness({ services: { getDeliveryRiderMe: async () => { throw new Error('身份服务暂不可用') } } })
    await app.mount()
    assert.match(text(app.tree), /加载失败/)
    assert.match(text(app.tree), /重新加载/)
    assert.doesNotMatch(text(app.tree), /正在同步身份与配送订单/)
    console.log('PASS 首次加载失败显示重试而非永久加载')
  }
  {
    for (const interruption of ['identity', 'unmount']) {
      for (const outcome of ['approved', 'rejected']) {
        const approval = deferred()
        let submitted = 0, shouldContinue
        const app = riderHarness({
          services: {
            listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [order('ORDER-A', 'to_pickup')] : []),
            markPickedUp: async (value) => { submitted++; return { ...value, status: 'picked_up' } },
          },
          media: { waitForMediaApproval: (_, __, ___, predicate) => { shouldContinue = predicate; return approval.promise } },
        })
        await app.mount(); await app.tab('待取货')
        app.primary('ORDER-A').props.onClick(); await app.flush()
        assert.equal(typeof shouldContinue, 'function', '凭证审核轮询应接收生命周期取消条件')
        assert.equal(shouldContinue(), true)
        if (interruption === 'identity') await app.setIdentityAllowed(false)
        else await app[interruption]()
        assert.equal(shouldContinue(), false, `${interruption} 应立即停止后续凭证轮询`)
        if (outcome === 'approved') approval.resolve({ canUse: true, state: 'approved' })
        else approval.reject(new Error('旧凭证审核失败'))
        await app.flush()
        assert.equal(submitted, 0, `${interruption}/${outcome} 后不得继续履约`)
        assert.equal(app.toasts.length, 0, `${interruption}/${outcome} 后不得弹出旧任务结果`)
      }
    }
    console.log('PASS 身份变化与页面卸载终止旧凭证流程且不弹旧提示')
  }
  {
    const selected = deferred()
    let submitted = 0
    const app = riderHarness({
      services: {
        listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [order('ORDER-A', 'to_pickup')] : []),
        markPickedUp: async (value) => { submitted++; return { ...value, status: 'picked_up' } },
      },
      media: { chooseAndUploadImages: () => selected.promise },
    })
    await app.mount(); await app.tab('待取货')
    app.primary('ORDER-A').props.onClick(); await app.flush()
    await app.hide()
    selected.resolve([{ assetId: 'TEST-ASSET' }]); await app.flush()
    assert.equal(submitted, 1, '打开系统相册产生的 onHide 不应取消正常凭证提交')
    assert.equal(app.toasts.filter((toast) => toast.icon === 'success').length, 1)
    console.log('PASS 系统相册暂时隐藏页面仍可完成正常凭证提交')
  }
  {
    for (const role of ['merchant', 'rider']) {
      for (const accountChanged of [false, true]) {
        const pending = deferred()
        const options = { switchIdentity: () => pending.promise }
        const app = role === 'merchant' ? merchantHarness('merchant-home', options) : riderHarness(options)
        await app.mount()
        const action = role === 'merchant'
          ? nodes(app.tree, node => node.type === 'IdentitySwitchModal')[0].props.onConfirm
          : nodes(app.tree, node => hasClass(node, 'rider-grab-user__switch'))[0].props.onClick
        action(); action(); await app.flush()
        assert.equal(app.switchCalls, 1, `${role} 身份切换同帧防重`)
        if (accountChanged) await app.setAccount(202)
        pending.resolve({ currentIdentity: 'student' }); await app.flush()
        assert.equal(app.relaunches, accountChanged ? 0 : 1, `${role} 仅当前账号成功切换后跳页`)
      }
    }
    console.log('PASS 两端工作台身份切换防重复，旧账号迟到响应不会导航')
  }
  {
    const late = deferred()
    let account = 'A', listCalls = 0
    const app = merchantHarness('merchant-home', { listMerchantDeliveryOrders: async (_, __, group) => {
      listCalls++
      if (account === 'A') return late.promise
      return page(group === 'pending' ? [{ ...order('ACCOUNT-B'), pickupAddress: '账户B取货地址' }] : [])
    } })
    await app.mount()
    assert.equal(listCalls, 3, '挂载与 didShow 同时触发不能重复发起三组读取')
    await app.setIdentityAllowed(false)
    account = 'B'
    await app.setAccount(202)
    await app.setIdentityAllowed(true)
    assert.match(text(app.tree), /账户B取货地址/, '身份恢复不依赖额外 didShow 就应读取当前账号')
    late.resolve(page([{ ...order('ACCOUNT-A-LATE'), pickupAddress: '账户A迟到地址' }]))
    await app.flush()
    assert.match(text(app.tree), /账户B取货地址/, '旧账号迟到响应不能覆盖当前账号')
    assert.doesNotMatch(text(app.tree), /账户A迟到地址/)
    console.log('PASS 商家账号切换作废迟到响应并主动重新加载')
  }
  {
    let rejectReads = false
    const app = merchantHarness('merchant-home', { listMerchantDeliveryOrders: async (_, __, group) => {
      if (rejectReads) throw new Error('账号恢复读取失败')
      return page(group === 'pending' ? [{ ...order('KNOWN-ACCOUNT-A'), pickupAddress: '账户A已知地址' }] : [])
    } })
    await app.mount()
    rejectReads = true
    await app.setIdentityAllowed(false)
    await app.setIdentityAllowed(true)
    assert.match(text(app.tree), /账户A已知地址/, '同账号临时核验失败保留原有数据')
    await app.setAccount(202)
    assert.deepEqual(app.serials(), [], '不同账号读取失败时不能继承旧账号数据')
    assert.equal(app.count('pending'), '—')
    console.log('PASS 商家同账号恢复保留已知订单，新账号失败不继承旧数据')
  }
  {
    const late = deferred()
    let account = 'A'
    const app = riderHarness({ services: {
      getDeliveryRiderMe: async () => ({ registered: true, rider: { status: 'active', realName: `骑手${account}` } }),
      listAssignedDeliveryOrders: async (_, __, group) => {
        if (account === 'A') return late.promise
        return page(group === 'pickup' ? [order('RIDER-B')] : [])
      },
    } })
    await app.mount()
    await app.setIdentityAllowed(false)
    account = 'B'
    await app.setAccount(202)
    await app.setIdentityAllowed(true)
    await app.tab('待取货')
    assert.ok(app.card('RIDER-B'), '骑手身份恢复应主动读取当前账号任务')
    late.resolve(page([order('RIDER-A-LATE')]))
    await app.flush()
    assert.ok(app.card('RIDER-B'))
    assert.equal(app.card('RIDER-A-LATE'), undefined)
    console.log('PASS 骑手账号切换隔离迟到任务并主动恢复加载')
  }
  {
    let rejectReads = false
    const app = riderHarness({ services: {
      listAssignedDeliveryOrders: async (_, __, group) => {
        if (rejectReads) throw new Error('骑手读取失败')
        return page(group === 'pickup' ? [order('RIDER-KNOWN')] : [])
      },
    } })
    await app.mount(); await app.tab('待取货')
    rejectReads = true
    await app.setIdentityAllowed(false); await app.setIdentityAllowed(true)
    assert.ok(app.card('RIDER-KNOWN'), '同账号身份重试失败保留任务')
    await app.setAccount(202)
    assert.equal(app.card('RIDER-KNOWN'), undefined, '新账号不得显示原骑手任务')
    assert.match(text(app.tree), /订单加载失败/)
    console.log('PASS 骑手同账号失败保留任务，新账号清除旧任务')
  }
  {
    const response = deferred()
    const app = merchantHarness('merchant-home', { listMerchantDeliveryOrders: (_, __, group) => response.promise.then(() => ({
      items: group === 'pending' ? [order('ORDER-A')] : [],
      total: { pending: 62, delivering: 8, completed: 120 }[group], page: 1,
    })) })
    await app.mount()
    assert.deepEqual(['pending', 'delivering', 'completed'].map((key) => app.count(key)), ['—', '—', '—'])
    assert.match(text(app.tree), /正在加载真实配送单/)
    response.resolve(); await app.flush()
    assert.deepEqual(['pending', 'delivering', 'completed'].map((key) => app.count(key)), ['62', '8', '120'], '数量使用服务端 total，不用当前页条数')
    const total = nodes(app.tree, (node) => hasClass(node, 'merchant-home-metric') && text(node).startsWith('累计单量'))[0]
    assert.equal(text(total), '累计单量190')
    console.log('PASS 商家加载数量占位与真实服务端总量')
  }
  {
    let failPending = false
    const requested = []
    const app = merchantHarness('merchant-home', { listMerchantDeliveryOrders: async (pageNumber, _, group) => {
      requested.push({ page: pageNumber, group })
      if (failPending && group === 'pending') throw new Error('测试待接单网络失败')
      return { items: group === 'pending' ? [order(`ORDER-${pageNumber}`)] : [], total: group === 'pending' ? 10 : 0, page: pageNumber }
    } })
    await app.mount(); await app.more()
    assert.equal(app.serials().length, 2)
    failPending = true
    await app.refresh()
    assert.equal(app.serials().length, 2, '一个分组刷新失败保留已加载的多页订单')
    assert.equal(app.count('pending'), '10', '刷新失败保留已知 total')
    assert.match(text(app.tree), /配送单刷新失败/)
    assert.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-home-empty')).length, 0)
    await app.tab('delivering')
    assert.doesNotMatch(text(app.tree), /测试待接单网络失败/, '单组错误不得污染其他成功分组')
    await app.tab('pending')
    await app.more()
    assert.equal(requested.at(-1).page, 3, '刷新失败后分页游标保留在第 2 页')
    assert.equal(app.serials().length, 2, '后续页失败同样保留订单')
    failPending = false
    await app.more()
    assert.equal(requested.at(-1).page, 3, '分页失败重试当前页，不跳页')
    assert.equal(app.serials().length, 3)
    console.log('PASS 商家分组刷新与翻页失败保留数据和游标')
  }
  {
    let cancelledAt = '2026-10-16T11:30:00+08:00'
    const app = merchantHarness('merchant-order-detail', { getMerchantDeliveryOrder: async () => ({
      ...order('ORDER-A', 'cancelled'), readyForClaimAt: '2026-10-16T10:10:00+08:00',
      claimedAt: '2026-10-16T10:20:00+08:00', cancelledAt,
    }) })
    await app.mount()
    assert.deepEqual(app.timeline().map((step) => step.key), ['cancelledAt'], '取消单默认摘要显示真实取消节点')
    await app.toggleProgress()
    assert.deepEqual(app.timeline().map((step) => step.key), ['createdAt', 'readyForClaimAt', 'claimedAt', 'cancelledAt'])
    assert.equal(app.timeline().at(-1).time, '2026-10-16 11:30')
    cancelledAt = undefined
    await app.refresh()
    assert.deepEqual(app.timeline().map((step) => step.key), ['createdAt', 'readyForClaimAt', 'claimedAt'], '取消时间缺失时保留实际节点，不使用更新时间伪造取消节点')
    assert.equal(text(app.one('merchant-detail-status-card__state')), '已取消', '没有取消时间仍准确显示订单已取消')
    await app.toggleProgress()
    assert.deepEqual(app.timeline().map((step) => step.key), ['claimedAt'], '取消时间缺失时摘要仅展示最近真实节点')
    assert.equal(app.orderCalls, 2, '稳定 Hook 依赖不会在每轮渲染重新请求详情')
    console.log('PASS 取消时间线保留实际节点且不伪造缺失时间')
  }
  {
    let funds
    const app = merchantHarness('merchant-order-detail', { getDeliveryOrderFunds: async () => funds })
    const cases = [
      [{ payment: { status: 'failed' } }, '配送费支付失败'],
      [{ payment: { status: 'created' } }, '等待支付配送费'],
      [{ payment: { status: 'paid' } }, '支付结果待确认'],
      [{ settlement: { status: 'paid' } }, '配送费已支付'],
      [{ settlement: { status: 'paid' }, refund: { status: 'pending' } }, '退款处理中'],
      [{ settlement: { status: 'refund_pending' }, refund: { status: 'failed' } }, '退款失败，请查看账单'],
      [{ settlement: { status: 'paid' }, refund: { status: 'succeeded' } }, '配送费已退还'],
    ]
    for (let index = 0; index < cases.length; index++) {
      funds = { deliveryOrderPublicId: 'ORDER-A', payment: null, settlement: null, refund: null, ...cases[index][0] }
      if (index === 0) await app.mount()
      else await app.refresh()
      assert.equal(text(app.one('merchant-detail-status-card__pay')), cases[index][1])
    }
    assert.equal(app.orderCalls, cases.length)
    console.log('PASS 真实详情区分支付失败、待确认、退款处理中与退款失败')
  }
  {
    const funding = deferred(), photos = deferred()
    let failOrder = false
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: async () => { if (failOrder) throw new Error('订单重试网络失败'); return order('ORDER-A') },
      getDeliveryOrderFunds: () => funding.promise,
      loadPrivateDeliveryProofs: () => photos.promise,
    })
    await app.mount()
    assert.equal(text(app.one('merchant-detail-status-card__pay')), '正在加载支付状态…')
    assert.match(text(app.one('merchant-detail-photo-card')), /正在更新配送照片/)
    assert.doesNotMatch(text(app.tree), /暂无支付记录|尚未提交取货或送达凭证/)
    funding.reject(new Error('资金服务暂不可用')); photos.reject(new Error('凭证服务暂不可用'))
    await app.flush()
    assert.equal(text(app.one('merchant-detail-status-card__pay')), '支付状态暂未更新')
    assert.match(text(app.one('merchant-detail-status-card__funds-error')), /资金服务暂不可用/)
    assert.match(text(app.one('merchant-detail-photo-card__error')), /凭证服务暂不可用/)
    assert.doesNotMatch(text(app.tree), /暂无支付记录|尚未提交取货或送达凭证/)
    failOrder = true
    await app.refresh()
    assert.match(text(app.tree), /订单重试网络失败/)
    assert.equal(text(app.one('merchant-detail-status-card__pay')), '支付状态暂未更新', '主订单重试失败不能将此前资金错误变成永久加载')
    assert.match(text(app.one('merchant-detail-photo-card__error')), /凭证服务暂不可用/, '主请求失败不能清掉尚未恢复的凭证错误')
    assert.doesNotMatch(text(app.tree), /正在加载支付状态|正在更新配送照片|暂无支付记录|尚未提交取货或送达凭证/)
    console.log('PASS 资金和凭证部分失败及后续主请求失败不会冒充空态或永久加载')
  }
  {
    let failSections = false
    const app = merchantHarness('merchant-order-detail', {
      getDeliveryOrderFunds: async () => {
        if (failSections) throw new Error('资金更新失败')
        return { payment: null, refund: null, settlement: { status: 'paid' } }
      },
      loadPrivateDeliveryProofs: async () => {
        if (failSections) throw new Error('照片更新失败')
        return [{ proofPublicId: 'PROOF-A', proofType: 'pickup', status: 'active', url: 'https://fixture.invalid/private-proof.jpg' }]
      },
    })
    await app.mount()
    failSections = true
    await app.refresh()
    assert.match(text(app.one('merchant-detail-status-card__funds-error')), /上次状态：配送费已支付/)
    assert.match(text(app.one('merchant-detail-photo-card__error')), /保留上次读取的照片/)
    assert.equal(app.one('merchant-detail-photo-item__image').props.src, 'https://fixture.invalid/private-proof.jpg')
    assert.doesNotMatch(text(app.tree), /暂无支付记录|尚未提交取货或送达凭证/)
    console.log('PASS 详情分区刷新失败保留已知资金状态和私密凭证')
  }
  let detailAssertions = 0
  const check = new Proxy(assert, { get(target, property) {
    return typeof target[property] === 'function' ? (...args) => { detailAssertions++; return target[property](...args) } : target[property]
  } })
  const proof = (id, version = 'old', extra = {}) => ({
    proofPublicId: id, proofType: id === 'PROOF-A' ? 'pickup' : 'delivery', slot: 0,
    status: 'active', url: `https://fixture.invalid/${id}/${version}.jpg`, expiresAt: Date.now() + 60000, ...extra,
  })
  {
    const app = merchantHarness('merchant-order-detail', { getMerchantDeliveryOrder: async () => ({
      ...order('ORDER-A', 'to_pickup'), readyForClaimAt: '2026-10-16T10:05:00+08:00',
      claimedAt: '2026-10-16T10:10:00+08:00', toPickupAt: '2026-10-16T10:15:00+08:00',
    }) })
    await app.mount()
    check.deepEqual(app.timeline().map((step) => step.key), ['toPickupAt'])
    check.equal(app.timeline()[0].time, '2026-10-16 10:15')
    check.equal(app.one('merchant-detail-section-toggle').props.ariaLabel, '展开全部配送进度')
    await app.toggleProgress()
    check.deepEqual(app.timeline().map((step) => step.key), ['createdAt', 'readyForClaimAt', 'claimedAt', 'toPickupAt', 'pickedUpAt', 'deliveringAt', 'deliveredAt'])
    check.equal(app.timeline().find((step) => step.key === 'claimedAt').time, '2026-10-16 10:10')
    check.equal(app.one('merchant-detail-section-toggle').props.ariaLabel, '收起配送进度')
    await app.toggleProgress()
    check.deepEqual(app.timeline().map((step) => step.key), ['toPickupAt'])
    check.equal(app.orderCalls, 1, '折叠展开仅改变呈现，不重取或修改订单')
    console.log('PASS 进度默认当前节点，展开保留全部记录且收起不触发业务请求')
  }
  {
    for (const [status, key, label] of [['to_pickup', 'toPickupAt', '前往取货'], ['delivered', 'deliveredAt', '已完成']]) {
      const app = merchantHarness('merchant-order-detail', { getMerchantDeliveryOrder: async () => ({ ...order('ORDER-A', status), updatedAt: '2026-10-16T12:59:00+08:00' }) })
      await app.mount()
      check.deepEqual(app.timeline().map((step) => step.key), [key])
      check.equal(app.timeline()[0].time, '', '缺失阶段时间不能用更新时间伪造')
      check.equal(text(app.one('merchant-detail-status-card__state')), label)
      check.doesNotMatch(text(app.one('merchant-detail-progress-card')), /12:59/)
      await app.toggleProgress()
      check.equal(app.timeline().find((step) => step.key === key).time, '')
    }
    console.log('PASS 当前及完成节点缺少时间仍表达真实状态，不补造履约时间')
  }
  {
    let loads = 0
    const app = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++loads === 1
      ? [proof('PROOF-A'), proof('PROOF-B')]
      : [proof('PROOF-B', 'fresh'), proof('PROOF-A', 'fresh')],
    })
    await app.mount(); await app.preview('PROOF-B')
    check.deepEqual(app.proofRequests, [['ORDER-A'], ['ORDER-A']])
    check.equal(app.previews.length, 1)
    check.deepEqual(app.previews[0], { urls: [proof('PROOF-B', 'fresh').url, proof('PROOF-A', 'fresh').url], current: proof('PROOF-B', 'fresh').url })
    check.ok(app.previews[0].urls.every((url) => !url.includes('/old.')))
    await app.preview()
    check.equal(app.previews.length, 2)
    check.equal(app.previews[1].current, proof('PROOF-B', 'fresh').url, '查看全部从最新可用列表开始')
    check.equal(loads, 3, '每次查看都重新取得私密签名')
    console.log('PASS 照片预览逐次刷新签名，以凭证ID定位而非旧数组下标')
  }
  {
    for (const changed of ['revoked', 'missing', 'no-url', 'expired']) {
      let loads = 0
      const changedProof = changed === 'revoked' ? proof('PROOF-A', 'revoked', { status: 'revoked' })
        : changed === 'no-url' ? proof('PROOF-A', 'no-url', { url: undefined })
          : proof('PROOF-A', 'expired', { expiresAt: Date.now() - 1000 })
      const app = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++loads === 1
        ? [proof('PROOF-A'), proof('PROOF-B')] : [...(changed === 'missing' ? [] : [changedProof]), proof('PROOF-B', 'fresh')],
      })
      await app.mount(); await app.preview('PROOF-A')
      check.equal(app.previews.length, 0, `${changed} 不能误打开另一张或过期缓存照片`)
      check.match(text(app.one('merchant-detail-photo-card__preview-error')), /暂不可查看/)
      if (changed === 'revoked') {
        const target = nodes(app.tree, (node) => hasClass(node, 'merchant-detail-photo-item') && node.props.key === 'PROOF-A')[0]
        check.equal(nodes(target, (node) => hasClass(node, 'merchant-detail-photo-item__image')).length, 0)
        check.equal(nodes(target, (node) => hasClass(node, 'merchant-detail-photo-item__thumb'))[0].props.onClick, undefined)
      }
    }
    console.log('PASS 选中凭证撤回缺失无链接或过期均不误开缓存和其他照片')
  }
  {
    const pending = deferred()
    let loads = 0
    const app = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++loads === 1
      ? [proof('PROOF-A')] : loads === 2 ? pending.promise : [proof('PROOF-A', 'retry')],
    })
    await app.mount()
    const click = app.one('merchant-detail-photo-item__thumb').props.onClick
    click(); click(); await app.flush()
    check.equal(loads, 2, '同帧点击只追加一个签名请求')
    check.equal(app.previews.length, 0)
    pending.reject(new Error('签名服务暂不可用')); await app.flush()
    check.equal(app.one('merchant-detail-photo-item__image').props.src, proof('PROOF-A').url)
    check.match(text(app.one('merchant-detail-photo-card__preview-error')), /签名服务暂不可用/)
    check.doesNotMatch(text(app.tree), /骑手尚未提交/)
    await app.preview('PROOF-A')
    check.equal(app.previews.length, 1)
    check.equal(app.previews[0].current, proof('PROOF-A', 'retry').url)
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-photo-card__preview-error')).length, 0)
    console.log('PASS 签名读取防重，失败保留已知照片并释放锁允许重试')
  }
  {
    for (const interrupt of ['account', 'token', 'identity', 'order', 'unmount', 'hide']) {
      const pending = deferred()
      let loads = 0
      const app = merchantHarness('merchant-order-detail', {
        getMerchantDeliveryOrder: async (id) => order(id),
        loadPrivateDeliveryProofs: async () => ++loads === 1 ? [proof('PROOF-A')] : loads === 2 ? pending.promise : [proof('PROOF-B', 'new-context')],
      })
      await app.mount(); await app.preview('PROOF-A')
      if (interrupt === 'account') await app.setAccount(202)
      else if (interrupt === 'token') await app.setToken('TOKEN-RENEWED')
      else if (interrupt === 'identity') await app.setIdentityAllowed(false)
      else if (interrupt === 'order') await app.setOrderId('ORDER-B')
      else await app[interrupt]()
      pending.resolve([proof('PROOF-A', 'late-old-context')]); await app.flush()
      check.equal(app.previews.length, 0, `${interrupt} 后旧签名不能打开原生预览`)
      check.ok(nodes(app.tree, (node) => node.type === 'Image').every((node) => !String(node.props.src).includes('late-old-context')))
      if (['account', 'token', 'order'].includes(interrupt)) {
        check.equal(app.one('merchant-detail-photo-item__image').props.src, proof('PROOF-B', 'new-context').url)
      }
    }
    console.log('PASS 账号token身份订单变化及隐藏卸载均取消迟到照片预览')
  }
  {
    for (const interrupt of ['account', 'token']) {
      const pendingOrder = deferred()
      let orderLoads = 0
      const app = merchantHarness('merchant-order-detail', {
        getMerchantDeliveryOrder: async () => ++orderLoads === 1 ? { ...order('OLD-ORDER'), pickupAddress: '旧账号取货地址' } : pendingOrder.promise,
        loadPrivateDeliveryProofs: async () => [proof('PROOF-A')],
      })
      await app.mount()
      if (interrupt === 'account') await app.setAccount(202)
      else await app.setToken('TOKEN-RENEWED')
      check.doesNotMatch(text(app.tree), /旧账号取货地址|OLD-ORDER/)
      check.equal(nodes(app.tree, (node) => node.type === 'Image').length, 0, `${interrupt} 变更应先清空旧账号照片`)
      pendingOrder.reject(new Error('新上下文暂无法读单')); await app.flush()
      check.match(text(app.tree), /新上下文暂无法读单/)
      check.doesNotMatch(text(app.tree), /旧账号取货地址|OLD-ORDER/)
    }
    for (const interrupt of ['account', 'token']) {
      const oldProof = deferred()
      let loads = 0
      const app = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++loads === 1 ? oldProof.promise : [proof('PROOF-B', 'new-context')] })
      await app.mount()
      if (interrupt === 'account') await app.setAccount(202)
      else await app.setToken('TOKEN-RENEWED')
      oldProof.resolve([proof('PROOF-A', 'late-old-context')]); await app.flush()
      check.equal(app.one('merchant-detail-photo-item__image').props.src, proof('PROOF-B', 'new-context').url)
      check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-photo-item')).length, 1)
    }
    console.log('PASS 详情账号token变更清旧数据，新上下文失败及旧分区迟到不恢复私密照片')
  }
  {
    const pending = deferred()
    let loads = 0
    const app = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++loads === 1
      ? [proof('PROOF-A')] : loads === 2 ? pending.promise : [proof('PROOF-A', 'revoked', { status: 'revoked' })],
    })
    await app.mount(); await app.preview('PROOF-A'); await app.refresh()
    check.equal(app.orderCalls, 1, '签名预览请求中不能并发覆盖整页照片快照')
    pending.resolve([proof('PROOF-A', 'fresh')]); await app.flush(); await app.refresh()
    check.equal(app.orderCalls, 2)
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-photo-item__image')).length, 0)
    check.match(text(app.tree), /照片已撤回/)
    const pendingDetail = deferred()
    let detailLoads = 0
    const detail = merchantHarness('merchant-order-detail', { loadPrivateDeliveryProofs: async () => ++detailLoads === 1 ? [proof('PROOF-A')] : pendingDetail.promise })
    await detail.mount(); await detail.refresh(); await detail.preview('PROOF-A')
    check.equal(detailLoads, 2, '整页刷新中不再追加签名预览')
    check.equal(detail.previews.length, 0)
    pendingDetail.resolve([proof('PROOF-A', 'revoked', { status: 'revoked' })]); await detail.flush()
    check.equal(nodes(detail.tree, (node) => hasClass(node, 'merchant-detail-photo-item__image')).length, 0)
    console.log('PASS 整页刷新与签名预览互斥，最新撤回状态不会被旧预览覆盖')
  }
  {
    const native = deferred()
    let opens = 0
    const app = merchantHarness('merchant-order-detail', {
      loadPrivateDeliveryProofs: async () => [proof('PROOF-A', 'fresh')],
      previewImage: async () => ++opens === 1 ? native.promise : undefined,
    })
    await app.mount(); await app.preview(); await app.hide()
    native.reject(new Error('已离开的原生预览错误')); await app.flush(); await app.show(); await app.preview()
    check.equal(app.previews.length, 2)
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-photo-card__preview-error')).length, 0)
    check.doesNotMatch(text(app.tree), /正在打开|已离开的原生预览错误/)
    let retries = 0
    const failed = merchantHarness('merchant-order-detail', {
      loadPrivateDeliveryProofs: async () => [proof('PROOF-A', 'fresh')],
      previewImage: async () => { if (++retries === 1) throw new Error('原生预览暂不可用') },
    })
    await failed.mount(); await failed.preview()
    check.match(text(failed.one('merchant-detail-photo-card__preview-error')), /原生预览暂不可用/)
    check.equal(failed.one('merchant-detail-photo-item__image').props.src, proof('PROOF-A', 'fresh').url)
    await failed.preview()
    check.equal(failed.previews.length, 2)
    console.log('PASS 原生照片预览失败或隐藏返回不会锁死，旧错误不覆盖当前页面')
  }
  {
    let riderUserId, navigationCalls = 0
    const app = merchantHarness('merchant-order-detail', {
      getMerchantDeliveryOrder: async () => ({ ...order('ORDER /?&中文'), riderUserId }),
      navigateTo: async () => { if (++navigationCalls === 1) throw new Error('会话暂不可用') },
    })
    await app.mount()
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-action-bar__contact')).length, 0)
    riderUserId = 202; await app.refresh()
    check.equal(nodes(app.one('merchant-detail-scroll'), (node) => hasClass(node, 'merchant-detail-action-bar')).length, 0, '联系操作应在滚动区外')
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-action-bar__contact')).length, 1)
    check.match(text(app.one('merchant-detail-action-bar')), /骑手 #202/)
    app.one('merchant-detail-action-bar__contact').props.onClick(); await app.flush()
    check.equal(app.navigations[0].url, `/pages/delivery-conversation/index?orderId=${encodeURIComponent('ORDER /?&中文')}&role=merchant`)
    check.ok(app.toasts.some((toast) => toast.title.includes('会话打开失败')))
    app.one('merchant-detail-action-bar__contact').props.onClick(); await app.flush()
    check.equal(app.navigations.length, 2, '会话导航失败后可以重试')
    riderUserId = undefined; await app.refresh()
    check.equal(nodes(app.tree, (node) => hasClass(node, 'merchant-detail-action-bar')).length, 0)
    console.log('PASS 底部联系骑手保留角色完整订单路由与权限，且位于滚动区外')
  }
  {
    for (const status of ['claimed', 'to_pickup', 'picked_up', 'delivering']) {
      const deliveryStage = status === 'picked_up' || status === 'delivering'
      const assigned = { ...order('ORDER-A', status), pickupContactPhone: '13800000001', recipientPhone: '13900000002' }
      const app = riderHarness({ services: { listAssignedDeliveryOrders: async (_, __, group) => page(group === (deliveryStage ? 'delivery' : 'pickup') ? [assigned] : []) } })
      await app.mount(); await app.tab(deliveryStage ? '待送达' : '待取货')
      const currentRows = nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-route__row--current'))
      check.equal(currentRows.length, 1)
      check.match(text(currentRows[0]), deliveryStage ? /当前前往 · 送达点/ : /当前前往 · 取货点/)
      check.match(text(currentRows[0]), deliveryStage ? /测试送达地址/ : /测试取货地址/)
      const label = deliveryStage ? '联系收货人' : '联系取货人'
      const phone = nodes(app.card('ORDER-A'), (node) => node.props?.ariaLabel === label)[0]
      check.ok(phone, `${status} 应说明实际拨号对象`)
      check.equal(text(phone), label)
      phone.props.onClick(); await app.flush()
      check.deepEqual(app.phoneCalls, [{ phoneNumber: deliveryStage ? assigned.recipientPhone : assigned.pickupContactPhone }])
      check.equal(text(app.primary('ORDER-A')), status === 'claimed' ? '前往取货' : status === 'to_pickup' ? '确认取货' : status === 'picked_up' ? '开始配送' : '确认送达')
    }
    console.log('PASS 骑手四阶段目的地强调、电话标签与拨号对象一致，履约主操作不变')
  }
  {
    for (const status of ['claimed', 'delivering']) {
      for (const empty of ['', '   ']) {
        const deliveryStage = status === 'delivering'
        const assigned = { ...order('ORDER-A', status), pickupContactPhone: deliveryStage ? '13800000001' : empty, recipientPhone: deliveryStage ? empty : '13900000002' }
        const app = riderHarness({ services: { listAssignedDeliveryOrders: async (_, __, group) => page(group === (deliveryStage ? 'delivery' : 'pickup') ? [assigned] : []) } })
        await app.mount(); await app.tab(deliveryStage ? '待送达' : '待取货')
        const phone = nodes(app.card('ORDER-A'), (node) => hasClass(node, 'rider-order-card__secondary') && String(node.props.ariaLabel).includes('暂无号码'))[0]
        check.ok(phone)
        check.equal(text(phone), '暂无号码')
        phone.props.onClick(); await app.flush()
        check.equal(app.phoneCalls.length, 0, '当前对象缺少号码时不得拨空号或退回另一方')
      }
    }
    for (const outcome of ['cancel', 'fail']) {
      let writes = 0
      const app = riderHarness({
        services: {
          listAssignedDeliveryOrders: async (_, __, group) => page(group === 'pickup' ? [{ ...order('ORDER-A'), pickupContactPhone: '13800000001' }] : []),
          startToPickup: async () => { writes++ },
        },
        makePhoneCall: async () => { throw { errMsg: `makePhoneCall:fail ${outcome}` } },
      })
      await app.mount(); await app.tab('待取货')
      nodes(app.card('ORDER-A'), (node) => node.props?.ariaLabel === '联系取货人')[0].props.onClick(); await app.flush()
      check.equal(app.phoneCalls.length, 1)
      check.equal(writes, 0)
      check.equal(text(app.primary('ORDER-A')), '前往取货')
      check.ok(app.toasts.every((toast) => toast.icon !== 'success'))
    }
    console.log('PASS 当前号码为空不拨另一方，拨号取消失败不改变履约结果')
  }
  console.log(`PASS 详情紧凑布局与照片联系新增12组，${detailAssertions}项行为断言`)
}
module.exports = { deferred, page, order, nodes, text, hasClass, hookRenderer, loadPage }
if (require.main === module) {
  run().catch((error) => { console.error(error); process.exitCode = 1 })
}
