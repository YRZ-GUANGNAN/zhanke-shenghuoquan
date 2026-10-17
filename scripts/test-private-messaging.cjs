// Exercise the actual identity mapping, avatar and chat handlers. All services
// are isolated; this does not log into IM or send a network message.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const { deferred, nodes, text, hasClass, hookRenderer, loadPage } = require('./test-delivery-stage-workbench.cjs')
const root = path.resolve(__dirname, '..')

function loadSource(relative, imports = {}) {
  const filename = path.join(root, relative)
  const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    fileName: filename,
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true },
  }).outputText
  const loaded = new Module(filename, module)
  loaded.filename = filename
  loaded.require = id => { assert.ok(id in imports, `unisolated dependency: ${id}`); return imports[id] }
  loaded._compile(compiled, filename)
  return loaded.exports
}

const presentation = loadSource('apps/miniapp/src/utils/conversation-presentation.ts')
const resource = loadSource('apps/miniapp/src/services/resource-url.ts')
const context = viewer => ({ relationId: 7, productId: 9, buyerId: 2, sellerId: 3, peerUserId: viewer === '3' ? 'campus_2' : 'campus_3', conversationId: viewer === '3' ? 'C2Ccampus_2' : 'C2Ccampus_3', peer: { id: viewer === '3' ? 2 : 3, nickname: viewer === '3' ? '买家昵称' : '卖家昵称', avatar: viewer === '3' ? '/buyer.jpg' : '/seller.jpg' } })
const product = { id: '9', sellerId: '3', sellerName: '旧卖家资料', sellerAvatar: '/old.jpg', title: '测试商品', images: [], price: 35 }
const msg = (ID, from, content) => ({ ID, from, time: 1792200000, type: 'TIMTextElem', payload: { text: content } })

function chatHarness(options = {}) {
  const renderer = hookRenderer()
  let viewer = options.viewer || '2', subscribed, unsubscribeCount = 0
  const params = options.params || { relationId: '7' }
  const toasts = [], writes = [], navigations = []
  const state = () => ({ status: 'authenticated', profile: { id: viewer, nickname: '自己', avatar: '/self.jpg' }, retry: async () => {} })
  const useSessionStore = selector => selector(state())
  useSessionStore.getState = state
  const reset = () => {}, focus = () => {}, height = () => {}
  let detailCalls = 0
  const services = {
    getIMConversation: async () => { detailCalls++; return context(viewer) },
    getTencentIMLoginUser: async id => id,
    subscribeTencentMessages: async (_, callback) => { subscribed = callback; return () => { unsubscribeCount++ } },
    loadTencentMessages: async () => [],
    sendTencentText: async (_, value) => { writes.push(value); return options.send ? options.send(value) : msg('sent', `campus_${viewer}`, value) },
    ...options.services,
  }
  const imports = {
    '../../components/product-price': 'ProductPrice',
    '../../components/feature-page-gate': { withFeaturePage: Page => Page },
    '@tarojs/components': { View: 'View', Text: 'Text', Image: 'Image', ScrollView: 'ScrollView', Textarea: 'Textarea' },
    '@tarojs/taro': { hideKeyboard: async () => {} },
    '../../components/capsule-page-header': ({ left, center, right }) => renderer.React.createElement('Header', {}, left, center, right),
    '../../components/app-icon': 'AppIcon',
    '../../components/chat-avatar': 'ChatAvatar',
    '../../services/api': { isApiError: () => false },
    '../../services/c-trade': { getProduct: async () => { if (options.missingProduct) throw Error('unavailable'); return product }, contactSeller: async () => ({ ...context(viewer), peer: undefined }) },
    '../../services/e-im': services,
    '../../services/session-recovery': { requestWithSingleSessionRecovery: fn => fn() },
    '../../services/taro': { getCurrentInstance: () => ({ router: { params } }), showToast: async ({ title }) => { toasts.push(title) }, navigateTo: async ({ url }) => { navigations.push(url) }, navigateBack: async () => {}, redirectTo: async () => {} },
    '../../stores/session': { useSessionStore },
    '../../hooks/use-viewport-refresh': { useViewportRefresh: () => ({}) },
    '../../hooks/use-keyboard-inset': { useKeyboardInset: () => ({ inset: 0, onFocus: focus, onKeyboardHeightChange: height, reset }) },
    '../../utils/conversation-presentation': presentation,
    './index.scss': {},
  }
  loadPage('secondhand-chat', imports, renderer)
  const byClass = name => nodes(renderer.tree, node => hasClass(node, name))[0]
  const input = () => nodes(renderer.tree, node => node.type === 'Textarea')[0]
  return {
    ...renderer, toasts, writes, navigations, services,
    get tree() { return renderer.tree }, get detailCalls() { return detailCalls }, get unsubscribeCount() { return unsubscribeCount },
    async mount() { renderer.render(); await renderer.flush() },
    input, byClass,
    type(value) { input().props.onInput({ detail: { value } }); renderer.render() },
    send() { return byClass('secondhand-chat-send').props.onClick() },
    changeAccount(id) { viewer = id; renderer.render() },
    incoming(values) { subscribed(values); renderer.render() },
  }
}

async function run() {
  assert.equal(presentation.conversationPeer(context('2'), product, '2').avatar, '/seller.jpg')
  assert.equal(presentation.conversationPeer(context('3'), product, '3').avatar, '/buyer.jpg')
  assert.deepEqual(presentation.conversationPeer(context('2'), product, '99'), { id: '', name: '交易对方', avatar: '' })
  assert.equal(presentation.conversationPeer({ ...context('3'), peer: undefined }, product, '3').avatar, '')
  assert.equal(presentation.conversationPeer({ ...context('2'), peer: undefined }, product, '2').avatar, '/old.jpg')
  assert.equal(presentation.conversationPeer({ ...context('2'), peer: { id: 99, nickname: '错误账号', avatar: '/wrong.jpg' } }, { ...product, sellerId: '999' }, '2').avatar, '')
  assert.equal(presentation.conversationPeer({ ...context('2'), peer: { id: 3, nickname: '账号已注销', avatar: '' } }, product, '2').avatar, '')
  console.log('PASS 双向身份、旧接口、注销与错误资料回退')

  const avatarRenderer = hookRenderer()
  let avatarProps = { src: '/avatar.jpg', name: '头像昵称' }
  const Avatar = loadSource('apps/miniapp/src/components/chat-avatar.tsx', {
    react: avatarRenderer.React,
    'react/jsx-runtime': { jsx: (type, props) => avatarRenderer.React.createElement(type, props, props.children), jsxs: (type, props) => avatarRenderer.React.createElement(type, props, props.children) },
    '@tarojs/components': { View: 'View', Image: 'Image', Text: 'Text' },
    '../services/api': { API_BASE: 'https://example.test/api/v1' },
    '../services/resource-url': resource,
    './chat-avatar.scss': {},
  }).default
  avatarRenderer.setComponent(() => Avatar(avatarProps))
  avatarRenderer.render()
  let image = nodes(avatarRenderer.tree, node => node.type === 'Image')[0]
  assert.equal(image.props.src, 'https://example.test/avatar.jpg')
  image.props.onError(); await avatarRenderer.flush()
  assert.equal(text(avatarRenderer.tree), '头')
  avatarProps = { src: '/new.jpg', name: '新用户' }
  await avatarRenderer.flush()
  assert.equal(nodes(avatarRenderer.tree, node => node.type === 'Image')[0].props.src, 'https://example.test/new.jpg')
  avatarProps = { src: '', name: '无图用户' }; await avatarRenderer.flush()
  assert.equal(text(avatarRenderer.tree), '无')
  console.log('PASS 头像资源地址、坏图首字与账号资料变化')

  for (const viewer of ['2', '3']) {
    const app = chatHarness({ viewer, missingProduct: true, services: { loadTencentMessages: async () => [msg('1', 'campus_2', '多行\n消息'), msg('2', 'campus_3', '卖家回复')] } })
    await app.mount()
    const avatars = nodes(app.tree, node => node.type === 'ChatAvatar')
    const peerAvatar = viewer === '2' ? '/seller.jpg' : '/buyer.jpg'
    assert.equal(avatars.filter(node => node.props.src === peerAvatar).length, 2)
    assert.equal(avatars.filter(node => node.props.src === '/self.jpg').length, 1)
    assert.equal(app.input().props.disabled, false)
    assert.ok(text(app.tree).includes('多行\n消息'))
    app.byClass('secondhand-chat-header__home').props.onClick()
    assert.ok(app.navigations[0].includes(`userId=${viewer === '2' ? '3' : '2'}&tab=secondhand`))
    app.unmount(); assert.equal(app.unsubscribeCount, 1)
  }
  const productEntry = chatHarness({ params: { productId: '9' } }); await productEntry.mount()
  assert.equal(productEntry.detailCalls, 1)
  assert.ok(text(productEntry.tree).includes('卖家昵称')); productEntry.unmount()
  console.log('PASS 列表进入与商品进入均取得对方头像，商品缺失仍可聊天')

  const pending = deferred()
  const app = chatHarness({ send: () => pending.promise }); await app.mount(); app.type('第一条')
  app.send(); app.send()
  assert.equal(app.writes.length, 1)
  app.type('继续输入第二条')
  pending.resolve(msg('one', 'campus_2', '第一条')); await app.flush()
  assert.equal(app.input().props.value, '继续输入第二条')
  assert.equal(nodes(app.tree, node => hasClass(node, 'secondhand-chat-row')).length, 1)
  app.incoming([msg('one', 'campus_2', '第一条')])
  assert.equal(nodes(app.tree, node => hasClass(node, 'secondhand-chat-row')).length, 1)
  app.unmount()
  const failed = chatHarness({ send: async () => { throw Error('测试网络故障') } }); await failed.mount(); failed.type('不能丢的草稿'); failed.send(); await failed.flush()
  assert.equal(failed.input().props.value, '不能丢的草稿')
  assert.ok(failed.toasts.includes('测试网络故障'))
  failed.unmount()
  console.log('PASS 连点只发送一次，成功去重不清新草稿，失败保留原文')

  const old = deferred()
  const switched = chatHarness({ send: () => old.promise }); await switched.mount(); switched.type('旧账号消息'); switched.send()
  switched.changeAccount('3'); await switched.flush()
  old.resolve(msg('stale', 'campus_2', '旧账号消息')); await switched.flush()
  assert.ok(!text(switched.tree).includes('旧账号消息'))
  assert.ok(text(switched.tree).includes('买家昵称'))
  switched.unmount()
  const gone = deferred()
  const unmounted = chatHarness({ send: () => gone.promise }); await unmounted.mount(); unmounted.type('离开页面'); unmounted.send(); unmounted.unmount()
  gone.reject(Error('离开后失败')); await new Promise(setImmediate)
  assert.equal(unmounted.toasts.length, 0)
  console.log('PASS 切换账号与页面卸载隔离旧收发结果')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
