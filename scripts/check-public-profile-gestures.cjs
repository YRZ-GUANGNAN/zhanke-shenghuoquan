const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

// Execute the real page and pager with isolated services; never call a live API.
async function run() {
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { JSDOM } = require('jsdom')
  const dom = new JSDOM('<main></main>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const previous = globals.map(key => Object.getOwnPropertyDescriptor(globalThis, key))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const originalNow = Date.now
  let now = 1000
  Date.now = () => now
  const container = document.querySelector('main')
  const root = createRoot(container)
  const requests = { profile: [], posts: [], secondhand: [] }
  const navigation = []
  const visibility = { forum: true, secondhand: true, following: true, likes: true }
  const deferred = (kind, page) => {
    let resolve, reject
    const promise = new Promise((yes, no) => { resolve = yes; reject = no })
    requests[kind].push({ page, resolve, reject })
    return promise
  }
  const component = name => props => React.createElement('div', {
    className: props.className, 'data-item': props.itemId, 'data-component': name,
    ref: node => { if (node) node.taroProps = props },
  }, props.children)
  const components = Object.fromEntries(['View', 'Text', 'Image', 'Button', 'ScrollView', 'Swiper', 'SwiperItem'].map(name => [name, component(name)]))
  const cache = new Map()
  function load(relative) {
    const file = path.resolve(__dirname, '../apps/miniapp/src', relative)
    if (cache.has(file)) return cache.get(file)
    const loaded = new Module(file, module)
    loaded.require = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return components
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('/feature-page-gate')) return { withFeaturePage: Page => Page }
      if (id.endsWith('/use-feature-visibility')) return { useFeatureVisibility: () => visibility }
      if (id.endsWith('/use-page-share')) return { usePageShare: () => true }
      if (id.endsWith('/runtime-session-gate')) return { default: components.View }
      if (id.endsWith('/capsule-page-header')) return { CapsuleSystemRow: components.View }
      if (id.endsWith('/use-viewport-refresh')) return { useViewportRefresh: () => ({}) }
      if (id.endsWith('/stores/session')) return { useSessionStore: select => select({ profile: { id: '9' } }) }
      if (id.endsWith('/components/ui')) return { AppActionSheet: () => null, AppDialog: () => null }
      if (id.endsWith('/channel-pager')) return load('components/channel-pager.tsx')
      if (id.endsWith('/navigation')) return { navigateBackOrHome: async () => navigation.push('back') }
      if (id.endsWith('/services/a-user')) return { blockUser: async () => {} }
      if (id.endsWith('/services/b-community')) return {
        getPublicCommunityProfile: () => deferred('profile'),
        getPublicUserPosts: (_, page) => deferred('posts', page),
        setUserFollowing: async () => {},
      }
      if (id.endsWith('/services/c-trade')) return { getPublicUserProducts: (_, page) => deferred('secondhand', page) }
      if (id.endsWith('/services/taro')) return { default: {
        getCurrentInstance: () => ({ router: { params: { userId: '1' } } }),
        navigateTo: async ({ url }) => navigation.push(url), showToast: async () => {},
      } }
      throw new Error('Unexpected public-profile dependency: ' + id)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {
      compilerOptions: { jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
    }).outputText, file)
    cache.set(file, loaded.exports)
    return loaded.exports
  }
  const Page = load('pages/user-profile/index.tsx').default
  const event = fn => React.act(async () => { fn() })
  const render = () => event(() => root.render(React.createElement(Page)))
  const node = selector => { const value = container.querySelector(selector); assert.ok(value, selector); return value }
  const props = selector => node(selector).taroProps
  const pane = tab => `[data-item="${tab}"]`
  const scroll = tab => `${pane(tab)} .public-profile-scroll`
  const swipe = (index, key) => event(() => props('.channel-pager').onChange({ detail: { current: index, currentItemId: key, source: 'touch' } }))
  const post = { id: 'p1', title: '测试帖子', content: '', images: [], likedCount: 2 }
  const product = { id: 's1', title: '测试商品', images: [], price: 10, status: 'selling' }
  const profile = { profile: { id: '1', nickname: '测试用户', avatar: '' }, followingCount: 0, followerCount: 0, likedCount: 2, isFollowing: false }
  try {
    await render()
    await event(() => {
      requests.profile[0].resolve(profile)
      requests.posts[0].resolve({ items: [post], total: 3, page: 1 })
      requests.secondhand[0].resolve({ items: [product], total: 3, page: 1 })
    })
    const postScroll = node(scroll('posts')), productScroll = node(scroll('secondhand'))
    await event(() => props(scroll('posts')).onScroll({ detail: { scrollTop: 640 } }))
    await swipe(1, 'secondhand')
    assert.equal(props('.channel-pager').current, 1)
    assert.ok(node('.public-profile-tab--active').textContent.includes('二手'))
    await event(() => props(scroll('secondhand')).onScroll({ detail: { scrollTop: 96 } }))
    await swipe(0, 'posts')
    assert.equal(node(scroll('posts')), postScroll, '来回切换保留帖子滚动容器')
    assert.equal(node(scroll('secondhand')), productScroll, '来回切换保留商品滚动容器')
    assert.equal(props(scroll('posts')).scrollTop, 640)
    assert.equal(props(scroll('secondhand')).scrollTop, 96)
    assert.equal(requests.posts.length, 1, '切换不重复初始加载')
    await event(() => props(scroll('secondhand')).onScrollToLower())
    assert.equal(requests.secondhand.length, 1, '隐藏页触底不能发起分页')
    await event(() => { props(scroll('posts')).onScrollToLower(); props(scroll('posts')).onScrollToLower() })
    assert.equal(requests.posts.length, 2, '同帧触底去重')
    await swipe(1, 'secondhand')
    await event(() => requests.posts[1].reject(new Error('仅帖子分页失败')))
    assert.ok(node(pane('posts')).textContent.includes('仅帖子分页失败'))
    assert.ok(!node(pane('secondhand')).textContent.includes('仅帖子分页失败'), '旧频道错误不串到商品')
    await event(() => props(`${pane('secondhand')} .public-profile-card`).onClick())
    assert.equal(navigation.length, 0, '滑动结束后的派生点击不得打开商品')
    now += 1000
    await event(() => props(`${pane('secondhand')} .public-profile-card`).onClick())
    assert.equal(navigation[0], '/pages/secondhand-detail/index?id=s1', '恢复后正常点击保留路由')
    await event(() => props(scroll('secondhand')).onScrollToLower())
    assert.equal(requests.secondhand.length, 2)
    visibility.secondhand = false
    await render()
    await event(() => {
      requests.secondhand[1].resolve({ items: [{ ...product, title: '不应显示的旧响应' }], total: 3, page: 2 })
      requests.profile[1].resolve(profile)
      requests.posts[2].resolve({ items: [post], total: 1, page: 1 })
    })
    assert.equal(container.querySelector(pane('secondhand')), null)
    assert.ok(!container.textContent.includes('不应显示的旧响应'), '权限变化隔离旧请求')
    assert.equal(props('.channel-pager').current, 0)
    await event(() => props('.public-profile-nav__back').onClick())
    assert.equal(navigation[1], 'back')
    console.log('公开主页横滑检查通过：独立位置、稳定容器、分页归属/防重、滑后防误触、关闭栏目与旧请求隔离。')
  } finally {
    await React.act(async () => root.unmount())
    Date.now = originalNow
    globals.forEach((key, index) => previous[index] ? Object.defineProperty(globalThis, key, previous[index]) : delete globalThis[key])
    dom.window.close()
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
