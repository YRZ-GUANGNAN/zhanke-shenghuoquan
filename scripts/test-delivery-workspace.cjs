// Execute real workspace components against isolated hooks and service replies.
// No backend, logged-in account, payment, conversation or network write is used.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const { deferred, nodes, text, hasClass, hookRenderer, loadPage } = require('./test-delivery-stage-workbench.cjs')

let assertions = 0
let scenarios = 0
const check = new Proxy(assert, { get(target, property) {
  return typeof target[property] === 'function' ? (...args) => { assertions++; return target[property](...args) } : target[property]
} })
const passed = (label) => { scenarios++; console.log(`PASS ${label}`) }
const root = path.resolve(__dirname, '..')
const header = { __esModule: true, CapsuleSystemRow: 'CapsuleSystemRow', default: ({ left, right }) => ({ type: 'Header', props: { children: [left, right] } }) }

function loadModule(relative, imports, renderer) {
  const filename = path.join(root, 'apps/miniapp/src', relative)
  const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.React, target: ts.ScriptTarget.ES2020, esModuleInterop: true },
    fileName: filename,
  }).outputText
  const loaded = new Module(filename, module)
  loaded.filename = filename
  loaded.require = (id) => {
    assert.ok(id === 'react' || id in imports, `必须显式隔离工作区依赖：${id}`)
    return id === 'react' ? renderer.React : imports[id]
  }
  loaded._compile(compiled, filename)
  return loaded.exports
}

function navHarness(options = {}) {
  const renderer = hookRenderer()
  let show
  const calls = { navigate: [], redirect: [], toast: [], select: [] }
  const visibility = { delivery: true, merchant: true, rider: true, ...options.visibility }
  const invoke = (name) => (...args) => {
    calls[name].push(args)
    return options[name]?.(...args) || Promise.resolve()
  }
  const exports = loadModule('components/delivery-workspace-nav.tsx', {
    '@tarojs/components': { View: 'View', Text: 'Text' },
    '@tarojs/taro': { useDidShow: (fn) => { show = fn }, useDidHide: () => {} },
    '../hooks/use-horizontal-navigation': { useHorizontalNavigation: () => ({ handlers: {}, canActivate: () => true }) },
    './app-icon': 'AppIcon',
    '../services/taro': { navigateTo: invoke('navigate'), redirectTo: invoke('redirect'), showToast: invoke('toast') },
    '../hooks/use-feature-visibility': { useFeatureVisibility: () => visibility },
    '../utils/bottom-nav-motion': { useBottomNavVisibility: () => false },
    './delivery-workspace-nav.scss': {},
  }, renderer)
  const props = { role: options.role || 'merchant', active: options.active || 'orders', onSelect: options.localSelect ? (key) => calls.select.push(key) : undefined, gestureGuard: options.gestureGuard }
  renderer.setComponent(() => exports.default(props))
  const item = (label) => nodes(renderer.tree, (node) => hasClass(node, 'delivery-workspace-nav__item') && text(node) === label)[0]
  return {
    exports, calls, item, flush: renderer.flush,
    get tree() { return renderer.tree },
    async mount() { renderer.render(); show(); await renderer.flush() },
    async click(label) { assert.ok(item(label), `导航应有${label}`); item(label).props.onClick(); await renderer.flush() },
    async show() { show(); await renderer.flush() },
  }
}

function sessionState(role) {
  let state = { profile: { id: 'ACCOUNT-A', nickname: '当前用户' }, token: 'TOKEN-A', status: 'authenticated', identity: { currentIdentity: role } }
  const useSessionStore = (select) => select(state)
  useSessionStore.getState = () => state
  return { useSessionStore, get state() { return state }, update(patch) { state = { ...state, ...patch } } }
}

function profileHarness(options = {}) {
  const renderer = hookRenderer()
  const session = sessionState(options.role || 'rider')
  let allowed = true, show, hide
  const calls = { applications: [], switches: [], reLaunch: [], navigate: [], guards: [], workspaceNavigation: [] }
  const swipe = { handlers: options.swipeHandlers || {}, canActivate: options.gestureCanActivate || (() => true) }
  session.update({ switchIdentity: async (role) => {
    calls.switches.push(role)
    const identity = await (options.switchIdentity?.(role) || Promise.resolve({ currentIdentity: role }))
    if (options.persistIdentity !== false) session.update({ identity })
    return identity
  } })
  const imports = {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView', Image: 'Image' },
    '@tarojs/taro': { useDidShow: (fn) => { show = fn }, useDidHide: (fn) => { hide = fn } },
    '../../components/capsule-page-header': header,
    '../../components/delivery-workspace-nav': { __esModule: true, default: 'DeliveryWorkspaceNav', useDeliveryWorkspaceNavigation: (...args) => { calls.workspaceNavigation.push(args); return { swipe } } },
    '../../components/chat-avatar': 'ChatAvatar',
    '../../hooks/use-delivery-payment': { useDeliveryPayment: () => ({ enabled: options.paymentEnabled ?? true }) },
    '../../components/app-icon': 'AppIcon',
    '../../components/ui': { AppButton: 'AppButton', AppDialog: 'AppDialog' },
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../hooks/use-identity-guard': { identityRoute: (role) => `/existing/${role}`, useIdentityGuard: (role) => { calls.guards.push(role); return allowed } },
    '../../services/a-user': { getIdentityApplication: async (role) => {
      calls.applications.push(role)
      return options.application?.(role) || { status: 'approved', realName: '登记姓名', merchantName: '登记商家', phone: '13800000000', merchantAddress: '登记地址' }
    } },
    '../../feature-flags': { deliveryPaymentEnabled: options.paymentEnabled ?? true },
    '../../services/taro': {
      getCurrentInstance: () => ({ router: { params: { role: options.role } } }),
      reLaunch: async (value) => { calls.reLaunch.push(value); await options.reLaunch?.(value) },
      navigateTo: async (value) => { calls.navigate.push(value) }, showToast: async () => {},
    },
    '../../stores/session': { useSessionStore: session.useSessionStore },
    '../../utils/bottom-nav-motion': { useBottomNavScroll: () => () => {} },
    './index.scss': {},
  }
  loadPage('delivery-profile', imports, renderer)
  const one = (className) => nodes(renderer.tree, (node) => hasClass(node, className))[0]
  const dialog = () => nodes(renderer.tree, (node) => node.type === 'AppDialog')[0]
  return {
    calls, session, swipe, one, dialog, flush: renderer.flush,
    get tree() { return renderer.tree },
    async mount() { renderer.render(); await renderer.flush() },
    async switch() {
      nodes(renderer.tree, (node) => node.type === 'AppButton' && text(node) === '切换到学生端')[0].props.onClick(); await renderer.flush()
      dialog().props.onConfirm(); await renderer.flush()
    },
    async state(patch, render = true) { session.update(patch); if (render) renderer.render(); await renderer.flush() },
    async allowed(value) { allowed = value; renderer.render(); await renderer.flush() },
    async hide() { hide(); await renderer.flush() },
    async show() { show(); await renderer.flush() },
    async unmount() { renderer.unmount(); await renderer.flush() },
  }
}

function conversation(id, orderId = id) {
  return { conversationPublicId: id, deliveryOrderPublicId: orderId, tencentConversationId: `IM-${id}`, status: 'active', canSend: true, updatedAt: '2026-10-16T12:34:56+08:00' }
}
function messagesHarness(options = {}) {
  const renderer = hookRenderer()
  const session = sessionState(options.role || 'merchant')
  let allowed = true, show, hide
  const calls = { lists: [], unread: [], navigate: [], redirect: [], guards: [] }
  const imports = {
    '@tarojs/components': { View: 'View', Text: 'Text', ScrollView: 'ScrollView' },
    '@tarojs/taro': { useDidShow: (fn) => { show = fn }, useDidHide: (fn) => { hide = fn } },
    '../../components/capsule-page-header': header,
    '../../components/ui': { AppIconButton: 'AppIconButton' },
    '../../components/runtime-session-gate': 'RuntimeSessionGate',
    '../../components/delivery-workspace-nav': { __esModule: true, default: 'DeliveryWorkspaceNav', useDeliveryWorkspaceNavigation: () => ({ swipe: { handlers: {}, canActivate: () => true } }) },
    '../../components/chat-avatar': 'ChatAvatar',
    '../../hooks/use-delivery-payment': { useDeliveryPayment: () => ({ enabled: options.paymentEnabled ?? true }) },
    '../../hooks/use-identity-guard': { useIdentityGuard: (role) => { calls.guards.push(role); return allowed } },
    '../../stores/session': { useSessionStore: session.useSessionStore },
    '../../utils/bottom-nav-motion': { useBottomNavScroll: () => () => {} },
    '../../services/delivery-im': {
      listDeliveryConversations: async (...args) => { calls.lists.push(args); return options.list?.(...args) || { items: [conversation('A')], total: 1 } },
      getDeliveryConversationUnread: async (id) => { calls.unread.push(id); return options.unread ? options.unread(id) : 0 },
    },
    '../../services/taro': {
      navigateTo: async (value) => { calls.navigate.push(value); await options.navigate?.(value) },
      redirectTo: async (value) => { calls.redirect.push(value) }, showToast: async () => {},
    },
    './index.scss': {},
  }
  const content = loadModule('pages/merchant-messages/content.tsx', imports, renderer)
  if (options.role === 'rider') loadPage('rider-messages', { ...imports, '../merchant-messages/content': content }, renderer)
  else loadPage('merchant-messages', { ...imports, './content': content }, renderer)
  const one = (className) => nodes(renderer.tree, (node) => hasClass(node, className))[0]
  const items = () => nodes(renderer.tree, (node) => hasClass(node, 'merchant-message-item'))
  return {
    calls, session, one, items, flush: renderer.flush,
    get tree() { return renderer.tree },
    async mount() { renderer.render(); await renderer.flush() },
    async refresh() { one('merchant-messages-nav__read').props.onClick(); await renderer.flush() },
    async more() { one('merchant-messages-scroll').props.onScrollToLower(); await renderer.flush() },
    async state(patch) { session.update(patch); renderer.render(); await renderer.flush() },
    async allowed(value) { allowed = value; renderer.render(); await renderer.flush() },
    async hide() { hide(); await renderer.flush() },
    async show() { show(); await renderer.flush() },
    async unmount() { renderer.unmount(); await renderer.flush() },
  }
}

async function profilePresentationScenarios() {
  const failures = []
  const scenario = async (label, verify) => {
    try { await verify(); passed(label) }
    catch (error) { failures.push(error); console.error(`FAIL ${label}: ${error.message}`) }
  }
  for (const role of ['merchant', 'rider']) {
    const roleName = role === 'merchant' ? '商家' : '骑手'
    await scenario(`${role} 真实封面随内容滚动，失败收起且新封面可恢复`, async () => {
      const app = profileHarness({ role })
      await app.mount()
      check.equal(app.one('delivery-profile__hero-image'), undefined, '无封面时不能生成占位图片')
      check.equal(hasClass(app.one('delivery-profile__hero'), 'delivery-profile__hero--cover'), false)
      const cover = '/test-media/profile-cover-a.png'
      await app.state({ profile: { ...app.session.state.profile, cover } })
      const image = app.one('delivery-profile__hero-image')
      check.equal(image?.type, 'Image', '封面必须使用真实 Image 组件')
      check.equal(image.props.src, cover)
      check.equal(image.props.mode, 'aspectFill')
      check.equal(hasClass(app.one('delivery-profile__hero'), 'delivery-profile__hero--cover'), true)
      check.equal(hasClass(app.one('delivery-profile__identity'), 'delivery-profile__identity--cover'), true)
      image.props.onError(); await app.flush()
      check.equal(app.one('delivery-profile__hero-image'), undefined)
      check.equal(hasClass(app.one('delivery-profile__hero'), 'delivery-profile__hero--cover'), false)
      check.equal(hasClass(app.one('delivery-profile__identity'), 'delivery-profile__identity--cover'), false)
      const nextCover = '/test-media/profile-cover-b.png'
      await app.state({ profile: { ...app.session.state.profile, cover: nextCover } })
      check.equal(app.one('delivery-profile__hero-image')?.props.src, nextCover, '旧封面失败不能阻止展示新封面')
      check.equal(hasClass(app.one('delivery-profile__hero'), 'delivery-profile__hero--cover'), true)
      const scroll = app.one('delivery-profile__scroll')
      check.equal(scroll?.type, 'ScrollView')
      check.equal(scroll.props.scrollY, true)
      check.equal(nodes(scroll, (node) => hasClass(node, 'delivery-profile__hero')).length, 1, 'hero 必须处于 ScrollView 内')
      check.equal(nodes(scroll, (node) => node.type === 'Image' && node.props.src === nextCover).length, 1)
    })
    await scenario(`${role} 身份区仅有一个编辑按钮且保留原编辑路由`, async () => {
      const app = profileHarness({ role })
      await app.mount()
      const edits = nodes(app.tree, (node) => hasClass(node, 'delivery-profile__edit'))
      check.equal(edits.length, 1, '页面不能重复编辑入口')
      check.equal(edits[0].props.ariaRole, 'button')
      check.ok(edits[0].props.ariaLabel, '编辑入口保留可访问名称')
      edits[0].props.onClick(); await app.flush()
      check.deepEqual(app.calls.navigate, [{ url: '/pages/edit-profile/index' }])
      check.equal(nodes(app.one('delivery-profile__identity'), (node) => node === app.one('delivery-profile__edit')).length, 1, '编辑按钮必须在身份区内')
    })
    for (const [status, label] of [['approved', '已通过'], ['pending', '审核中'], ['rejected', '未通过']]) {
      await scenario(`${role} ${status} 审核状态由真实返回映射至资料标题且不重复`, async () => {
        const app = profileHarness({ role, application: async () => ({ status, realName: '测试联系人', merchantName: '测试商家' }) })
        await app.mount()
        check.equal(text(app.tree).split(label).length - 1, 1, '审核结果只展示一次')
        check.deepEqual(app.calls.applications, [role])
        const heading = app.one('delivery-profile__heading-row')
        check.ok(heading, '资料区必须有 heading-row')
        check.equal(text(nodes(heading, (node) => hasClass(node, 'delivery-profile__heading'))), `${roleName}资料`)
        const statuses = nodes(heading, (node) => hasClass(node, 'delivery-profile__status'))
        check.equal(statuses.length, 1, '审核状态必须置于资料标题行')
        check.equal(text(statuses[0]), label)
        check.equal(nodes(app.tree, (node) => hasClass(node, 'delivery-profile__status')).length, 1)
        check.ok(nodes(app.tree, (node) => hasClass(node, 'delivery-profile__fact')).every((node) => !text(node).includes('审核状态')), '事实列表不能重复审核状态')
      })
    }
    await scenario(`${role} 资料加载和错误不冒充审核通过，重试使用真实结果`, async () => {
      const pending = deferred()
      let attempts = 0
      const app = profileHarness({ role, application: async () => ++attempts === 1 ? pending.promise : { status: 'approved', realName: '重试返回联系人', merchantName: '重试返回商家' } })
      await app.mount()
      check.match(text(app.tree), /正在读取已提交资料/)
      check.doesNotMatch(text(app.tree), /已通过|审核中|未通过/)
      check.equal(text(app.one('delivery-profile__status')), '资料读取中', '加载时应显示资料读取状态')
      check.doesNotMatch(text(app.one('delivery-profile__status')), /已通过|审核中|未通过/, '加载时不能冒充审核结论')
      pending.reject(new Error('测试资料读取失败')); await app.flush()
      check.match(text(app.tree), /测试资料读取失败.*点击重试/)
      check.doesNotMatch(text(app.tree), /已通过|审核中|未通过/)
      check.equal(text(app.one('delivery-profile__status')), '资料待重试', '错误时应显示可重试状态')
      check.doesNotMatch(text(app.one('delivery-profile__status')), /已通过|审核中|未通过/, '错误时不能冒充审核结论')
      app.one('delivery-profile__retry').props.onClick(); await app.flush()
      check.deepEqual(app.calls.applications, [role, role])
      check.match(text(app.tree), /已通过/)
      check.doesNotMatch(text(app.tree), /测试资料读取失败|正在读取已提交资料/)
      check.equal(text(app.one('delivery-profile__status')), '已通过')
    })
    await scenario(`${role} 底栏 mine、页面横滑处理器和 gestureGuard 保持同一来源`, async () => {
      const events = []
      const swipeHandlers = Object.fromEntries(['onTouchStart', 'onTouchMove', 'onTouchEnd', 'onTouchCancel'].map((name) => [name, (event) => events.push([name, event])]))
      const gestureCanActivate = () => true
      const app = profileHarness({ role, swipeHandlers, gestureCanActivate })
      await app.mount()
      const nav = nodes(app.tree, (node) => node.type === 'DeliveryWorkspaceNav')
      check.equal(nav.length, 1)
      check.equal(nav[0].props.role, role)
      check.equal(nav[0].props.active, 'mine')
      check.equal(nav[0].props.gestureGuard, gestureCanActivate)
      check.deepEqual(app.calls.workspaceNavigation.at(-1), [{ role, active: 'mine' }, true])
      const page = app.one('delivery-profile')
      const event = { touches: [{ clientX: 240, clientY: 180 }] }
      for (const [name, handler] of Object.entries(swipeHandlers)) {
        check.equal(page.props[name], handler, `${name} 必须继续由工作区 hook 提供`)
        page.props[name](event)
      }
      check.deepEqual(events, Object.keys(swipeHandlers).map((name) => [name, event]))
      const switchButton = nodes(app.tree, (node) => node.type === 'AppButton' && text(node) === '切换到学生端')[0]
      switchButton.props.onClick(); await app.flush()
      check.deepEqual(app.calls.workspaceNavigation.at(-1), [{ role, active: 'mine' }, false], '身份确认时暂停页面横滑')
      app.dialog().props.onCancel(); await app.flush()
      check.deepEqual(app.calls.workspaceNavigation.at(-1), [{ role, active: 'mine' }, true])
    })
    await scenario(`${role} 支付关闭保留真实说明，骑手只读收入入口可用`, async () => {
      const app = profileHarness({ role, paymentEnabled: false })
      await app.mount()
      const funds = app.one('delivery-profile__section--funds')
      check.ok(funds)
      check.doesNotMatch(text(funds), /查看真实余额|充值记录|￥|¥/)
      if (role === 'merchant') {
        check.match(text(funds), /支付功能未开通，查看资金说明/)
        const links = nodes(funds, (node) => hasClass(node, 'delivery-profile__link'))
        check.equal(links.length, 1)
        links[0].props.onClick(); await app.flush()
        check.deepEqual(app.calls.navigate, [{ url: '/pages/merchant-wallet/index' }])
      } else {
        check.match(text(funds), /配送收入与结算|账面未结款|排队占用/)
        check.doesNotMatch(text(app.tree), /钱包与账单/)
        const links = nodes(funds, (node) => hasClass(node, 'delivery-profile__link'))
        check.equal(links.length, 1)
        links[0].props.onClick(); await app.flush()
        check.deepEqual(app.calls.navigate, [{ url: '/pages/rider-wallet/index' }])
      }
    })
    await scenario(`${role} 横滑保护拒绝编辑和身份切换，恢复后仍需确认`, async () => {
      let canActivate = false
      const app = profileHarness({ role, gestureCanActivate: () => canActivate })
      await app.mount()
      const clickSwitch = () => nodes(app.tree, (node) => node.type === 'AppButton' && text(node) === '切换到学生端')[0].props.onClick()
      app.one('delivery-profile__edit').props.onClick(); clickSwitch(); await app.flush()
      check.deepEqual(app.calls.navigate, [])
      check.deepEqual(app.calls.switches, [])
      check.deepEqual(app.calls.reLaunch, [])
      check.equal(app.dialog().props.visible, false, '拒绝手势时不打开身份确认弹窗')
      check.equal(nodes(app.tree, (node) => node.type === 'DeliveryWorkspaceNav')[0].props.gestureGuard(), false)
      canActivate = true
      app.one('delivery-profile__edit').props.onClick(); clickSwitch(); await app.flush()
      check.deepEqual(app.calls.navigate, [{ url: '/pages/edit-profile/index' }])
      check.equal(app.dialog().props.visible, true)
      check.deepEqual(app.calls.switches, [], '身份切换必须等待明确确认')
      app.dialog().props.onConfirm(); await app.flush()
      check.deepEqual(app.calls.switches, ['student'])
      check.deepEqual(app.calls.reLaunch, [{ url: '/existing/student' }])
    })
    await scenario(`${role} 长名称和联系资料保留完整服务字段，缺少名称才回退昵称`, async () => {
      const application = { status: 'approved', realName: '测试服务返回的超长联系人姓名'.repeat(3), merchantName: '测试服务返回的超长商家名称'.repeat(3), phone: '+86 13800000000 转 1234567890', merchantAddress: '测试服务返回的校区取货地址和详细楼层说明'.repeat(4) }
      const app = profileHarness({ role, application: async () => application })
      await app.mount()
      const avatar = '/test-media/profile-avatar.png'
      const nickname = '测试会话中的超长昵称'.repeat(3)
      await app.state({ profile: { ...app.session.state.profile, nickname, avatar } })
      const name = role === 'merchant' ? application.merchantName : application.realName
      const avatarNode = nodes(app.tree, (node) => node.type === 'ChatAvatar')[0]
      check.equal(avatarNode.props.src, avatar)
      check.equal(avatarNode.props.name, name)
      check.ok(text(app.one('delivery-profile__name')).includes(name))
      const facts = nodes(app.tree, (node) => hasClass(node, 'delivery-profile__fact')).map(text)
      check.ok(facts.includes(`联系人${application.realName}`))
      check.ok(facts.includes(`联系电话${application.phone}`))
      if (role === 'merchant') check.ok(facts.includes(`取货地址${application.merchantAddress}`))
      else check.ok(facts.every((value) => !value.startsWith('取货地址')))
      application.realName = ''; application.merchantName = ''
      await app.show()
      check.equal(nodes(app.tree, (node) => node.type === 'ChatAvatar')[0].props.name, nickname)
      check.ok(text(app.one('delivery-profile__name')).includes(nickname))
      check.ok(nodes(app.tree, (node) => hasClass(node, 'delivery-profile__fact')).map(text).includes('联系人未填写'))
    })
  }
  if (failures.length) throw new AggregateError(failures, `资料页 B 方向 ${failures.length} 组回归场景失败`)
}

async function run() {
  {
    const app = navHarness()
    await app.mount()
    check.deepEqual(app.exports.deliveryWorkspaceItems('merchant').map(({ key, url }) => [key, url]), [
      ['orders', '/pages/merchant-home/index'], ['publish', '/pages/merchant-publish/index'],
      ['messages', '/pages/merchant-messages/index'], ['mine', '/pages/delivery-profile/index?role=merchant'],
    ])
    check.deepEqual(app.exports.deliveryWorkspaceItems('rider').map(({ key, url }) => [key, url]), [
      ['grab', '/pages/delivery-rider/index'], ['tasks', '/pages/delivery-rider/index?tab=tasks'],
      ['messages', '/pages/rider-messages/index'], ['mine', '/pages/delivery-profile/index?role=rider'],
    ])
    await app.click('订单')
    check.equal(app.calls.redirect.length + app.calls.navigate.length, 0)
    passed('两种身份导航默认路由明确，当前页面点击不重复跳转')
  }
  {
    const pending = deferred()
    const app = navHarness({ navigate: () => pending.promise })
    await app.mount()
    const click = app.item('发单').props.onClick
    click(); click(); await app.flush()
    check.equal(app.calls.navigate.length, 1)
    check.equal(app.calls.redirect.length, 0, '发单保留原工作台返回栈')
    pending.resolve(); await app.flush(); await app.show(); await app.click('消息')
    check.equal(app.calls.redirect.length, 1, '返回原工作台后导航锁应重置')
    passed('导航同帧防重，发单保留返回栈，页面返回后可以继续导航')
  }
  {
    let attempts = 0
    const app = navHarness({ redirect: async () => { if (++attempts === 1) throw new Error('导航失败') } })
    await app.mount(); await app.click('消息'); await app.click('消息')
    check.equal(app.calls.redirect.length, 2)
    check.equal(app.calls.toast.length, 1)
    passed('导航失败提示原因并释放锁，允许重试')
  }
  {
    const merchant = navHarness({ visibility: { delivery: false, merchant: false } })
    await merchant.mount()
    check.equal(merchant.item('发单'), undefined)
    check.ok(merchant.item('订单') && merchant.item('消息') && merchant.item('我的'))
    const rider = navHarness({ role: 'rider', active: 'tasks', visibility: { delivery: false, rider: false } })
    await rider.mount()
    check.equal(rider.item('接单'), undefined)
    check.ok(rider.item('配送') && rider.item('消息') && rider.item('我的'))
    passed('功能关闭隐藏新发单接单，历史订单与会话入口仍保留')
  }
  {
    const app = navHarness({ role: 'rider', active: 'grab', localSelect: true })
    await app.mount(); await app.click('配送')
    check.deepEqual(app.calls.select, ['tasks'])
    check.equal(app.calls.redirect.length + app.calls.navigate.length, 0)
    passed('骑手接单与配送在同工作台切换，不堆叠页面')
  }
  {
    let allowed = false
    const app = navHarness({ gestureGuard: () => allowed })
    await app.mount(); await app.click('消息')
    check.equal(app.calls.redirect.length + app.calls.navigate.length, 0, '页面横滑保护期内底部导航不得误触发')
    allowed = true; await app.click('消息')
    check.equal(app.calls.redirect.length, 1)
    passed('底部导航复用页面手势保护，横滑结束后不会误跳页')
  }
  {
    const app = profileHarness()
    await app.mount()
    check.ok(app.calls.guards.every((role) => role === 'rider'))
    check.match(text(app.tree), /骑手工作区/)
    check.doesNotMatch(text(app.tree), /钱包与账单/)
    const merchant = profileHarness({ role: 'merchant', paymentEnabled: false })
    await merchant.mount()
    check.match(text(merchant.tree), /商家工作区/)
    check.match(text(merchant.tree), /支付功能未开通/)
    merchant.one('delivery-profile__link').props.onClick(); await merchant.flush()
    check.equal(merchant.calls.navigate[0].url, '/pages/merchant-wallet/index')
    passed('资料页默认骑手，商家沿用已有钱包入口与支付开关说明')
  }
  {
    const pending = deferred()
    const app = profileHarness({ switchIdentity: () => pending.promise })
    await app.mount(); await app.switch()
    app.dialog().props.onConfirm(); await app.flush()
    check.equal(app.calls.switches.length, 1)
    check.equal(app.calls.reLaunch.length, 0)
    pending.resolve({ currentIdentity: 'student' }); await app.flush()
    check.equal(app.calls.reLaunch.length, 1)
    check.equal(app.calls.reLaunch[0].url, '/existing/student')
    passed('身份切换同帧防重，仅真实成功后进入学生端')
  }
  for (const interrupt of ['account', 'token', 'unauthenticated', 'hide', 'unmount']) {
    const pending = deferred()
    const app = profileHarness({ switchIdentity: () => pending.promise })
    await app.mount(); await app.switch()
    if (interrupt === 'account') await app.state({ profile: { id: 'ACCOUNT-B', nickname: '新用户' } })
    else if (interrupt === 'token') await app.state({ token: 'TOKEN-B' })
    else if (interrupt === 'unauthenticated') await app.state({ status: 'anonymous' })
    else await app[interrupt]()
    pending.resolve({ currentIdentity: 'student' }); await app.flush()
    check.equal(app.calls.reLaunch.length, 0)
    passed(`${interrupt} 后旧身份切换响应不得跳页`)
  }
  {
    let attempts = 0
    const app = profileHarness({ switchIdentity: async () => { if (++attempts === 1) throw new Error('身份更新失败'); return { currentIdentity: 'student' } } })
    await app.mount(); await app.switch()
    check.equal(app.dialog().props.error, '身份更新失败')
    check.equal(app.calls.reLaunch.length, 0)
    app.dialog().props.onConfirm(); await app.flush()
    check.equal(app.calls.switches.length, 2)
    check.equal(app.calls.reLaunch.length, 1)
    passed('身份切换失败保留确认框真实错误，可以再次确认')
  }
  {
    for (const [options, label] of [[{ switchIdentity: async () => ({ currentIdentity: 'merchant' }) }, '服务未切换'], [{ persistIdentity: false }, '会话未应用']]) {
      const app = profileHarness(options)
      await app.mount(); await app.switch()
      check.equal(app.calls.reLaunch.length, 0, label)
    }
    passed('身份返回值或当前会话未应用学生身份时不擅自跳页')
  }
  {
    const pending = deferred()
    let requests = 0
    const app = profileHarness({ role: 'merchant', application: async () => ++requests === 1 ? pending.promise : { status: 'approved', merchantName: '新商家', realName: '新联系人' } })
    await app.mount(); await app.state({ profile: { id: 'ACCOUNT-B', nickname: '新用户' } })
    pending.resolve({ status: 'approved', merchantName: '旧商家', realName: '旧联系人' }); await app.flush()
    check.match(text(app.tree), /新商家/)
    check.doesNotMatch(text(app.tree), /旧商家|旧联系人/)
    passed('换账号后迟到的商家资料不覆盖新账号')
  }
  {
    const pending = deferred()
    let requests = 0
    const app = profileHarness({ application: async () => ++requests === 1 ? pending.promise : { status: 'approved', realName: '重新读取资料' } })
    await app.mount(); await app.hide()
    pending.resolve({ status: 'approved', realName: '隐藏后旧资料' }); await app.flush()
    check.doesNotMatch(text(app.tree), /隐藏后旧资料/)
    await app.show()
    check.match(text(app.tree), /重新读取资料/)
    passed('资料页隐藏后丢弃旧响应，重新显示时读取当前资料')
  }
  {
    const pending = deferred()
    const app = messagesHarness({ unread: () => pending.promise })
    await app.mount()
    check.match(text(app.tree), /未读状态同步中/)
    check.doesNotMatch(text(app.tree), /可发送消息|0 条未读/)
    pending.reject(new Error('IM断开')); await app.flush()
    check.match(text(app.tree), /未读状态暂未同步/)
    check.doesNotMatch(text(app.tree), /可发送消息|0 条未读/)
    check.equal(app.items().length, 1)
    passed('未读等待与读取失败显示未知，保留会话且不冒充零未读')
  }
  {
    const app = messagesHarness({ list: async () => ({ items: [conversation('A'), conversation('B')], total: 2 }), unread: async (id) => id === 'IM-A' ? 3 : 0 })
    await app.mount()
    check.match(text(app.items()[0]), /3 条未读消息/)
    check.equal(nodes(app.items()[0], (node) => hasClass(node, 'merchant-message-item__dot')).length, 1)
    check.match(text(app.items()[1]), /可发送消息/)
    check.equal(nodes(app.items()[1], (node) => hasClass(node, 'merchant-message-item__dot')).length, 0)
    passed('真实未读数与零未读使用不同提示和提醒点')
  }
  {
    let refresh = false
    const pending = deferred()
    const app = messagesHarness({ unread: () => refresh ? pending.promise : 2 })
    await app.mount()
    check.match(text(app.tree), /2 条未读消息/)
    refresh = true; await app.refresh()
    check.match(text(app.tree), /未读状态同步中/)
    check.doesNotMatch(text(app.tree), /2 条未读消息/)
    pending.resolve(0); await app.flush()
    check.match(text(app.tree), /可发送消息/)
    passed('刷新新会话快照时重新核对未读数，不沿用旧数字')
  }
  {
    let failPageTwo = true, failRefresh = false
    const app = messagesHarness({ list: async (pageNumber) => {
      if (pageNumber === 2 && failPageTwo) throw new Error('下一页读取失败')
      if (pageNumber === 1 && failRefresh) throw new Error('刷新读取失败')
      return { items: pageNumber === 1 ? [conversation('A')] : [conversation('A'), conversation('B')], total: 2 }
    } })
    await app.mount(); await app.more()
    check.equal(app.items().length, 1)
    check.match(text(app.tree), /下一页读取失败/)
    failPageTwo = false; await app.more()
    check.deepEqual(app.calls.lists.map(([pageNumber]) => pageNumber), [1, 2, 2])
    check.deepEqual(app.items().map((item) => item.props.key), ['A', 'B'])
    await app.more()
    check.equal(app.calls.lists.length, 3, '达到total后不再发分页请求')
    failRefresh = true; await app.refresh()
    check.deepEqual(app.items().map((item) => item.props.key), ['A', 'B'])
    check.match(text(app.tree), /刷新读取失败/)
    check.doesNotMatch(text(app.tree), /当前没有配送会话/)
    passed('分页失败不跳页、重复会话去重、刷新失败保留已加载数据')
  }
  {
    const pending = deferred()
    const app = messagesHarness({ list: () => pending.promise })
    await app.mount(); await app.refresh(); await app.more()
    check.equal(app.calls.lists.length, 1)
    pending.resolve({ items: [], total: 0 }); await app.flush()
    check.match(text(app.tree), /当前没有配送会话/)
    passed('消息列表加载防重，只有成功空响应才能显示空态')
  }
  for (const interruption of ['account', 'token', 'identity', 'hide', 'unmount']) {
    const pending = deferred()
    let requests = 0
    const app = messagesHarness({ list: async () => ++requests === 1 ? pending.promise : { items: [conversation('NEW')], total: 1 } })
    await app.mount()
    if (interruption === 'account') await app.state({ profile: { id: 'ACCOUNT-B', nickname: '新用户' } })
    else if (interruption === 'token') await app.state({ token: 'TOKEN-B' })
    else if (interruption === 'identity') await app.allowed(false)
    else await app[interruption]()
    pending.resolve({ items: [conversation('OLD')], total: 1 }); await app.flush()
    check.doesNotMatch(text(app.tree), /#OLD/)
    check.ok(!app.calls.unread.includes('IM-OLD'), '旧列表响应不继续读取IM未读')
    if (interruption === 'account' || interruption === 'token') check.match(text(app.tree), /#NEW/)
    passed(`${interruption} 后旧消息列表不回流或继续读取未读`)
  }
  {
    const oldUnread = deferred()
    let requests = 0
    const app = messagesHarness({ list: async () => ({ items: [conversation(++requests === 1 ? 'OLD' : 'NEW')], total: 1 }), unread: async (id) => id === 'IM-OLD' ? oldUnread.promise : 1 })
    await app.mount(); await app.state({ profile: { id: 'ACCOUNT-B', nickname: '新用户' } })
    oldUnread.resolve(99); await app.flush()
    check.match(text(app.tree), /#NEW/)
    check.match(text(app.tree), /1 条未读消息/)
    check.doesNotMatch(text(app.tree), /#OLD|99 条未读/)
    passed('换账号后迟到的未读响应不会污染新会话')
  }
  {
    let requests = 0
    const pending = deferred()
    const app = messagesHarness({ list: async () => ++requests === 1 ? pending.promise : { items: [conversation('AFTER-SHOW')], total: 1 } })
    await app.mount(); await app.hide()
    pending.resolve({ items: [conversation('HIDDEN')], total: 1 }); await app.flush(); await app.show()
    check.equal(app.calls.lists.length, 2)
    check.match(text(app.tree), /#AFTER-SHOW/)
    check.doesNotMatch(text(app.tree), /#HIDDEN|正在加载真实配送会话/)
    passed('消息页面隐藏后取消旧链，返回时自动刷新且不会永久加载')
  }
  for (const role of ['merchant', 'rider']) {
    const app = messagesHarness({ role, list: async () => ({ items: [conversation('A', 'ORDER /?&中文')], total: 1 }) })
    await app.mount()
    check.ok(app.calls.guards.every((value) => value === role))
    app.items()[0].props.onClick(); await app.flush()
    check.equal(app.calls.navigate[0].url, `/pages/delivery-conversation/index?orderId=${encodeURIComponent('ORDER /?&中文')}&role=${role}`)
    app.one('merchant-messages-nav__back').props.onClick(); await app.flush()
    check.equal(app.calls.redirect[0].url, role === 'merchant' ? '/pages/merchant-home/index' : '/pages/delivery-rider/index')
    check.equal(nodes(app.tree, (node) => node.type === 'DeliveryWorkspaceNav')[0].props.role, role)
    passed(`${role} 消息守卫、完整订单号路由、返回入口与导航身份一致`)
  }
  await profilePresentationScenarios()
  console.log(`PASS 配送多页工作区 ${scenarios} 组场景，${assertions} 项行为断言`)
}

if (require.main === module) run().catch((error) => { console.error(error); process.exitCode = 1 })
module.exports = { navHarness, profileHarness, messagesHarness }
