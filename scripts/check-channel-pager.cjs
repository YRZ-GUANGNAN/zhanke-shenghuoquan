const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { JSDOM } = require('jsdom')
const { createRoot } = require('react-dom/client')
const { act } = React
const root = path.resolve(__dirname, '..')

function deferred() {
  let resolve, reject
  const promise = new Promise((accept, decline) => { resolve = accept; reject = decline })
  return { promise, resolve, reject }
}

async function run() {
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const names = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = names.map((name) => Object.getOwnPropertyDescriptor(globalThis, name))
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const roots = []
  const component = (name) => (props) => React.createElement('div', {
    className: props.className,
    'data-component': name,
    'data-item-id': props.itemId,
    ref: (node) => { if (node) node.taroProps = props },
  }, props.children, props.title, props.description)
  const components = Object.fromEntries(['Image', 'ScrollView', 'Text', 'View', 'Swiper', 'SwiperItem'].map((name) => [name, component(name)]))
  const View = components.View
  const load = (filename, dependencies = {}) => {
    const loaded = new Module(filename, module)
    loaded.require = (id) => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return components
      if (id.endsWith('.scss')) return {}
      if (Object.prototype.hasOwnProperty.call(dependencies, id)) return dependencies[id]
      throw new Error(`Unexpected channel test dependency: ${id}`)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
      fileName: filename,
      compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    }).outputText, filename)
    return loaded.exports
  }
  const pagerModule = load(path.join(root, 'apps/miniapp/src/components/channel-pager.tsx'))
  const { default: ChannelPager, createChannelInteraction } = pagerModule
  const mount = async (renderElement) => {
    const container = document.createElement('main')
    document.body.appendChild(container)
    const rootNode = createRoot(container)
    roots.push(rootNode)
    const render = async () => { await act(async () => rootNode.render(renderElement())) }
    await render()
    return {
      container, render,
      event: async (callback) => { await act(async () => callback()) },
      props: (selector) => {
        const node = container.querySelector(selector)
        assert.ok(node, `Missing ${selector}`)
        return node.taroProps
      },
      all: (selector) => Array.from(container.querySelectorAll(selector)).map((node) => node.taroProps),
    }
  }
  try {
    let clock = 0
    const interaction = createChannelInteraction(() => clock)
    assert.equal(interaction.canActivate(), true)
    interaction.onTouchStart()
    interaction.onTouchEnd()
    assert.equal(interaction.canActivate(), true, 'ordinary taps stay clickable')
    interaction.onTouchStart()
    interaction.onTransition()
    clock = 1000
    assert.equal(interaction.canActivate(), false, 'holding a displaced swipe stays guarded')
    interaction.onTouchEnd()
    interaction.onAnimationFinish()
    assert.equal(interaction.canActivate(), false, 'release cannot open the underlying card')
    clock += 121
    assert.equal(interaction.canActivate(), true, 'settled content becomes clickable again')
    interaction.onTouchStart()
    interaction.onTransition()
    interaction.onTouchCancel()
    clock += 321
    assert.equal(interaction.canActivate(), true, 'missing animationfinish after cancel cannot lock content')

    let selected = 'discover'
    let keys = ['messages', 'discover', 'tools']
    const changes = []
    const mounts = new Map()
    const Page = ({ name }) => {
      React.useEffect(() => { mounts.set(name, (mounts.get(name) || 0) + 1) }, [])
      return React.createElement(View, { className: `test-${name}` }, name)
    }
    const pager = await mount(() => React.createElement(ChannelPager, {
      keys, activeKey: selected, onChange: (key) => changes.push(key), interaction,
      renderPage: (key) => React.createElement(Page, { name: key }),
    }))
    const swiper = () => pager.props('[data-component="Swiper"]')
    assert.equal(swiper().current, 1)
    assert.equal(swiper().circular, false)
    assert.equal(swiper().autoplay, false)
    const discoverNode = pager.container.querySelector('.test-discover')
    selected = 'tools'
    await pager.render()
    assert.equal(swiper().current, 2, 'tab click drives native current')
    await pager.event(() => swiper().onChange({ detail: { current: 1, source: '' } }))
    assert.equal(changes.length, 0, 'late programmatic echoes cannot revert a newer tab click')
    await pager.event(() => swiper().onTouchStart())
    await pager.event(() => swiper().onChange({ detail: { current: 1, source: 'touch', currentItemId: 'discover' } }))
    assert.deepEqual(changes, ['discover'], 'native swipe updates the tab selection')
    selected = changes.pop()
    await pager.render()
    assert.equal(pager.container.querySelector('.test-discover'), discoverNode)
    assert.deepEqual([...mounts.values()], [1, 1, 1], 'switching preserves every channel subtree')
    await pager.event(() => swiper().onChange({ detail: { current: 3, source: 'touch' } }))
    await pager.event(() => swiper().onChange({ detail: { current: -1, source: 'touch' } }))
    await pager.event(() => swiper().onChange({ detail: { current: 0.5, source: 'touch' } }))
    assert.equal(changes.length, 0, 'invalid native indices do not select a channel')
    keys = ['discover', 'tools']
    await pager.render()
    assert.equal(swiper().current, 0)
    await pager.event(() => swiper().onChange({ detail: { current: 1, source: 'touch' } }))
    assert.equal(changes.length, 0, 'late touch indices from removed channels are discarded')
    await pager.event(() => swiper().onTouchStart())
    await pager.event(() => swiper().onChange({ detail: { current: 1, source: 'touch', currentItemId: 'messages' } }))
    assert.equal(changes.length, 0, 'item identity must match the new visible index')
    await pager.event(() => swiper().onChange({ detail: { current: 1, source: 'touch', currentItemId: 'tools' } }))
    assert.equal(changes.pop(), 'tools', 'fresh native gesture works after visibility changes')
    keys = ['tools']
    await pager.render()
    assert.equal(changes.pop(), 'tools', 'hidden selected channel falls back to the remaining channel')
    assert.equal(swiper().current, 0)
    keys = []
    await pager.render()
    assert.equal(pager.container.querySelector('[data-component="Swiper"]'), null)
    assert.equal(changes.length, 0, 'empty visibility has no invented selection')

    let visibility = { available: true, presentationMode: 'standard', forum: true, secondhand: true, messages: true, secondhand_chat: true, campus_tools: true, search: true }
    let session = { token: 'test-token-a', profile: { id: 'test-user-a' } }
    let routeParams = {}
    const shareOptions = []
    const requests = { posts: [], products: [], messages: [], tools: [] }
    const navigate = []
    const shown = new Set()
    const queued = (kind, query) => { const task = deferred(); requests[kind].push({ ...task, query }); return task.promise }
    const homePath = path.join(root, 'apps/miniapp/src/pages/home/index.tsx')
    const Home = load(homePath, {
      '../../components/channel-pager': pagerModule,
      '../../components/feature-page-gate': { FeaturePageGate: View },
      '../../components/runtime-session-gate': { default: View },
      '../../components/student-bottom-nav': { default: View },
      '../student-secondhand/content': { default: View },
      '../../components/campus-info-home': { default: View },
      '../../components/capsule-page-header': { default: (props) => React.createElement(View, {}, props.center, props.right) },
      '../../components/app-icon': { default: View },
      '../../components/ui': { AppState: component('AppState') },
      '../../hooks/use-identity-guard': { useIdentityGuard: () => true },
      '../../hooks/use-feature-visibility': { useFeatureVisibility: () => visibility },
      '../../hooks/use-page-share': { usePageShare: (options) => shareOptions.push(options) },
      '../../stores/session': { useSessionStore: (selector) => selector(session) },
      '../../services/api': { isApiError: () => false },
      '../../services/b-community': { getCommunityPostsPage: (query) => queued('posts', query) },
      '../../services/c-trade': { getProductsPage: (query) => queued('products', query), getProduct: async () => null },
      '../../services/e-im': { listIMConversations: () => queued('messages') },
      '../../services/d-navigation': { listNavigationItems: () => queued('tools') },
      '../../services/session-recovery': { requestWithSingleSessionRecovery: (callback) => callback() },
      '../../services/taro': { default: { getCurrentInstance: () => ({ router: { params: routeParams } }), navigateTo: async (value) => navigate.push(value.url), showToast: async () => {} } },
      '../../utils/post-presentation': { postBodyWithoutTitle: (_, body) => body },
      '@tarojs/taro': { useDidShow: (callback) => {
        const ref = React.useRef(callback)
        ref.current = callback
        React.useEffect(() => { shown.add(ref); return () => shown.delete(ref) }, [])
      } },
    }).default
    const home = await mount(() => React.createElement(Home))
    const homeSwiper = () => home.props('[data-component="Swiper"]')
    const channel = (key) => home.props(`[data-item-id="${key}"] .student-home-content`)
    const clickTab = async (index) => home.event(() => home.all('.student-home-header__tab')[index].onClick())
    assert.deepEqual(Array.from(home.container.querySelectorAll('.student-home-header__tab'), node => node.textContent), ['发现', '二手', '工具'])
    assert.equal(homeSwiper().current, 0, '首页默认从发现开始')
    assert.equal(requests.posts.length, 1)
    assert.equal(requests.products.length, 1)
    assert.equal(requests.messages.length, 0)
    assert.equal(requests.tools.length, 0, 'rendering sibling pages never preloads their business data')
    await home.event(() => {
      requests.posts[0].resolve({ items: [{ id: 'post-1', title: '稳定内容', content: '', author: '测试', images: [], likedCount: 0 }], page: 1, total: 11 })
      requests.products[0].resolve({ items: [], page: 1, total: 0 })
    })
    const homeDiscoverNode = home.container.querySelector('[data-item-id="discover"] .student-home-content')
    await home.event(() => channel('discover').onScroll({ detail: { scrollTop: 640 } }))
    await clickTab(1)
    assert.equal(shareOptions.at(-1).kind, 'secondhand', '二手频道保留商品市场分享卡片和旧直达路由')
    await home.event(() => home.props('.student-home-header__search').onClick())
    assert.equal(navigate.at(-1), '/pages/search/index?scope=secondhand')
    assert.equal(requests.messages.length, 0, '二手频道使用独立商品内容，不预加载私信')
    await clickTab(2)
    assert.equal(channel('discover').scrollTop, 640)
    assert.equal(home.container.querySelector('[data-item-id="discover"] .student-home-content'), homeDiscoverNode)
    await clickTab(1)
    await home.event(() => channel('discover').onScrollToLower())
    assert.equal(requests.posts.length, 1, 'hidden channel cannot paginate')
    session = { token: 'test-token-b', profile: { id: 'test-user-b' } }
    await home.render()
    assert.equal(home.container.querySelector('.student-message-row'), null, '普通首页不挂载私信页面')
    assert.equal(requests.posts.length, 1, 'account invalidation does not preload an inactive discover page')
    await home.event(() => {
      homeSwiper().onTouchStart()
      homeSwiper().onTransition({ detail: { dx: 30, dy: 0 } })
      homeSwiper().onTouchEnd()
    })
    routeParams = { tab: 'messages' }
    const messagePage = await mount(() => React.createElement(Home))
    assert.equal(requests.messages.length, 1, '底栏私信入口加载会话')
    assert.equal(shareOptions.at(-1).enabled, false, '私信视图不提供公开分享')
    const messageRequest = requests.messages[0]
    const discoverRequestsBefore = requests.posts.length
    routeParams = { id: 'chat-child' }
    await messagePage.render()
    assert.deepEqual(messagePage.all('[data-component="SwiperItem"]').map(value => value.itemId), ['messages'], '子页面成为当前router后私信归属不变化')
    assert.equal(requests.posts.length, discoverRequestsBefore, '隐藏私信页面不误请求发现')
    session = { token: 'test-token-c', profile: { id: 'test-user-c' } }
    await messagePage.render()
    assert.equal(requests.messages.length, 2, '账号变化重新获取所属会话')
    await messagePage.event(() => messageRequest.resolve([{ relationId: 'stale', productId: 'old', buyerId: 'old' }]))
    assert.equal(messagePage.container.querySelector('.student-message-row'), null, '旧账号会话结果不能回写')
    await messagePage.event(() => requests.messages[1].resolve([]))
    await messagePage.event(() => { for (const ref of shown) ref.current() })
    assert.equal(requests.messages.length, 3, '从聊天页返回后刷新当前账号会话')
    await messagePage.event(() => requests.messages[2].resolve([{ relationId: 7, productId: 8, sellerId: 'test-user-c', buyerId: 'test-peer' }]))
    assert.ok(messagePage.container.querySelector('.student-message-row'))
    const messageSwiper = () => messagePage.props('[data-component="Swiper"]')
    const navigationCount = navigate.length
    await messagePage.event(() => {
      messageSwiper().onTouchStart()
      messageSwiper().onTransition({ detail: { dx: 20, dy: 0 } })
      messageSwiper().onTouchEnd()
      messagePage.props('.student-message-row').onClick()
    })
    assert.equal(navigate.length, navigationCount, '滑动松手不能误开聊天')
    await messagePage.event(() => { for (const ref of shown) ref.current() })
    assert.equal(requests.messages.length, 4)
    visibility = { ...visibility, messages: false }
    await messagePage.render()
    await messagePage.event(() => requests.messages[3].resolve([{ relationId: 9, productId: 9, sellerId: 'test-user-c', buyerId: 'test-peer' }]))
    assert.equal(messagePage.container.querySelector('[data-component="Swiper"]'), null, '私信开关关闭后不展示残留会话')
    assert.equal(requests.posts.length, discoverRequestsBefore)
    console.log('channel pager and home behavior: ok')
  } finally {
    for (const node of roots) await act(async () => node.unmount())
    dom.window.close()
    names.forEach((name, index) => {
      if (originals[index]) Object.defineProperty(globalThis, name, originals[index])
      else delete globalThis[name]
    })
  }
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
