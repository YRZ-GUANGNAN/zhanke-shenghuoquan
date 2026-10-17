const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

// Real page/hook code with in-memory test dependencies only. No login or network.
function deferred() {
  let resolve, reject
  const promise = new Promise((accept, decline) => { resolve = accept; reject = decline })
  return { promise, resolve, reject }
}

async function run() {
  const { JSDOM } = require('jsdom')
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { act } = React
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const names = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const descriptors = names.map((name) => Object.getOwnPropertyDescriptor(globalThis, name))
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const roots = []
  const component = (name) => (props) => React.createElement('div', {
    className: props.className,
    'data-component': name,
    'data-item-id': props.itemId,
    ref: (node) => { if (node) node.taroProps = props },
  }, props.children)
  const View = component('View')
  const visibility = { available: true, presentationMode: 'standard', forum: true, favorites: true, comment: true, secondhand: true, delivery: true, rider: true, merchant: true, identity_application: true }
  const session = () => ({
    token: 'isolated-test-session', status: 'authenticated',
    profile: { id: 'fixture-account-a', nickname: '测试账号', avatar: '', school: '', campus: '' },
    identity: { currentIdentity: 'student', identities: [{ identity: 'student', available: true }, { identity: 'rider', available: true }, { identity: 'merchant', available: false, status: 'pending' }] },
    refreshIdentity: async () => {}, switchIdentity: async () => {},
  })
  const post = (id) => ({ id, author: '测试作者', title: `测试内容 ${id}`, content: '', images: [], createdAt: '', likedCount: 0, commentCount: 0, reviewStatus: 'approved' })

  async function mount(page, options = {}) {
    options.visibility ||= { ...visibility }
    options.session ||= session()
    const shows = new Set()
    const taro = { getCurrentInstance: () => ({ router: { params: options.params || {} } }), navigateTo: async () => {}, navigateBack: async () => {}, reLaunch: async () => {}, showToast: async () => {}, ...options.taro }
    const loadSource = (sourcePath) => {
      const loaded = new Module(sourcePath, module)
      loaded.require = (id) => {
        if (id === 'react' || id === 'react/jsx-runtime') return require(id)
        if (id === '@tarojs/components') return Object.fromEntries(['Image', 'ScrollView', 'Swiper', 'SwiperItem', 'Text', 'View', 'Button'].map((name) => [name, component(name)]))
        if (id === '@tarojs/taro') return { useDidShow: (callback) => {
          const ref = React.useRef(callback)
          ref.current = callback
          React.useEffect(() => { shows.add(ref); return () => shows.delete(ref) }, [])
        } }
        if (id.endsWith('.scss')) return {}
        if (id.endsWith('/use-feature-visibility')) return { useFeatureVisibility: () => options.visibility }
        if (id.endsWith('/stores/session')) return { useSessionStore: (selector) => selector(options.session) }
        if (id.endsWith('/use-identity-guard')) return { useIdentityGuard: () => true, identityRoute: (role) => `/pages/${role}/index` }
        if (id.endsWith('/runtime-session-gate')) return { default: View }
        if (id.endsWith('/feature-page-gate')) return { withFeaturePage: (pageComponent) => pageComponent }
        if (id.endsWith('/student-bottom-nav') || id.endsWith('/app-icon')) return { default: View }
        if (id.endsWith('/IdentitySwitchModal')) return { default: (props) => React.createElement(View, { ...props, className: 'test-identity-modal' }) }
        if (id.endsWith('/capsule-page-header')) return { CapsuleSystemRow: View, default: (props) => React.createElement(View, { className: 'test-header' }, props.left, props.right) }
        if (id.endsWith('/use-viewport-refresh')) return { useViewportRefresh: () => ({}) }
        if (id.endsWith('/capsule-safe')) return { getCapsuleSafeStyle: () => ({}) }
        if (id.endsWith('/components/ui')) return { AppIconButton: View, AppActionSheet: (props) => props.visible ? React.createElement(View, props) : null, AppDialog: (props) => props.visible ? React.createElement(View, { ...props, className: 'test-dialog' }) : null }
        if (id.endsWith('/use-page-share')) return { usePageShare: () => true }
        if (id.endsWith('/services/a-user')) return options.user || {}
        if (id.endsWith('/services/taro')) return { default: taro }
        if (id.endsWith('/services/b-community')) return options.community || {}
        if (id.endsWith('/services/c-trade')) return options.trade || {}
        if (id.endsWith('/post-presentation')) return loadSource(path.resolve(__dirname, '../apps/miniapp/src/utils/post-presentation.ts'))
        if (id.endsWith('/product-price')) return loadSource(path.resolve(__dirname, '../apps/miniapp/src/components/product-price.tsx'))
        if (id.endsWith('/product-presentation')) return loadSource(path.resolve(__dirname, '../apps/miniapp/src/utils/product-presentation.ts'))
        if (id.endsWith('/channel-pager')) return loadSource(path.resolve(__dirname, '../apps/miniapp/src/components/channel-pager.tsx'))
        if (id.endsWith('/utils/navigation')) return loadSource(path.resolve(__dirname, '../apps/miniapp/src/utils/navigation.ts'))
        if (id === './use-content-tabs') return loadSource(path.resolve(path.dirname(sourcePath), 'use-content-tabs.ts'))
        throw new Error(`Unexpected profile test dependency: ${id}`)
      }
      loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
        compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
      }).outputText, sourcePath)
      return loaded.exports
    }
    const Page = loadSource(path.resolve(__dirname, `../apps/miniapp/src/pages/${page}/index.tsx`)).default
    const container = document.createElement('main')
    document.body.appendChild(container)
    const root = createRoot(container)
    roots.push(root)
    const render = async () => { await act(async () => root.render(React.createElement(Page))) }
    await render()
    return {
      options, container, render,
      show: async () => { await act(async () => { for (const ref of shows) ref.current() }) },
      event: async (callback) => { await act(async () => { callback() }) },
      all: (selector) => Array.from(container.querySelectorAll(selector)).map((node) => node.taroProps),
      props: (selector) => { const node = container.querySelector(selector); assert.ok(node, `Missing ${selector}`); return node.taroProps },
    }
  }

  try {
    const communityCalls = [], productCalls = [], navigations = [], switches = []
    const profile = await mount('profile', {
      community: { getMyCommunitySummary: () => { const task = deferred(); communityCalls.push(task); return task.promise } },
      trade: { getMyProductsPage: () => { const task = deferred(); productCalls.push(task); return task.promise } },
      taro: { navigateTo: (args) => { const task = deferred(); navigations.push({ ...task, ...args }); return task.promise } },
    })
    await profile.show()
    assert.equal(communityCalls.length, 1, '初始 effect 和 show 不重复请求社区计数')
    assert.equal(productCalls.length, 1, '初始 effect 和 show 不重复请求商品计数')
    await profile.event(() => { communityCalls[0].reject(new Error('isolated offline')); productCalls[0].resolve({ total: 7 }) })
    const stats = () => Array.from(profile.container.querySelectorAll('.student-profile-stats__item')).map((node) => ({
      label: node.querySelector('.student-profile-stats__label').textContent,
      value: node.querySelector('.student-profile-stats__value').textContent,
    }))
    assert.equal(stats().find((item) => item.label === '宝贝').value, '7', '社区失败不吞掉二手计数')
    assert.equal(stats().find((item) => item.label === '帖子').value, '—', '读取失败不能伪装成零内容')
    await profile.event(() => profile.props('.student-profile-count-retry').onClick())
    assert.equal(productCalls.length, 1, '只重试失败的计数来源')
    await profile.event(() => communityCalls[1].resolve({ favorites: 0, posts: 2, comments: 3 }))
    assert.equal(stats()[0].value, '0', '真实零值仍准确显示')
    assert.equal(stats()[1].value, '2')
    await profile.event(() => { profile.all('.student-profile-stats__item')[1].onClick(); profile.all('.student-profile-stats__item')[1].onClick() })
    assert.equal(navigations.length, 1, '同帧连续点击只导航一次')
    assert.equal(navigations[0].url, '/pages/my-content/index?tab=posts')
    await profile.event(() => navigations[0].reject(new Error('isolated navigation failure')))
    await profile.event(() => profile.all('.student-profile-stats__item')[1].onClick())
    assert.equal(navigations.length, 2, '导航失败可以重试')
    await profile.event(() => navigations[1].resolve())
    await profile.show()
    assert.equal(profile.props('.student-profile-hero').onClick, undefined, '系统留白不触发编辑')
    assert.equal(profile.container.querySelector('.student-profile-cover__edit'), null, '主页不再单独提供封面入口，统一从编辑资料设置')
    assert.ok(profile.container.querySelector('.student-profile-hero > .student-profile-user'), '个人资料处于同一个背景头部')
    assert.equal(profile.container.querySelector('.student-profile-hero--with-cover'), null, '无图状态不冒充已设置照片')
    assert.equal(profile.props('.student-profile-user').onClick, undefined, '资料留白不是大面积隐形按钮')
    assert.equal(typeof profile.props('.student-profile-user__edit').onClick, 'function', '编辑资料具有独立触控入口')
    await profile.event(() => { profile.props('.student-profile-user__avatar').onClick(); profile.props('.student-profile-user__main').onClick(); profile.props('.student-profile-user__edit').onClick() })
    assert.equal(navigations.length, 3, '头像、资料和编辑按钮共用防重导航')
    assert.equal(navigations[2].url, '/pages/edit-profile/index')
    await profile.event(() => navigations[2].resolve())
    profile.options.visibility = { ...visibility, forum: false }
    await profile.render()
    assert.equal(stats().length, 1)
    assert.match(profile.props('.student-profile-stats').style.gridTemplateColumns, /repeat\(1,/)
    await profile.event(() => communityCalls[2].resolve({ favorites: 99, posts: 99, comments: 99 }))
    assert.ok(!profile.container.textContent.includes('99'), '关闭社区后迟到响应不能恢复社区统计')
    await profile.event(() => productCalls[1].reject(new Error('isolated refresh failure')))
    assert.equal(stats()[0].value, '7', '刷新失败保留已知的数量')
    profile.options.session = { ...session(), token: 'isolated-next-session', profile: { ...session().profile, id: 'fixture-account-b' } }
    await profile.render()
    assert.equal(stats()[0].value, '—', '账号变化不继承前一账号数量')
    profile.options.session.profile = { ...profile.options.session.profile, avatar: 'isolated-avatar-a' }
    await profile.render()
    await profile.event(() => profile.props('.student-profile-user__avatar-image').onError())
    assert.equal(profile.container.querySelector('.student-profile-user__avatar-image'), null, '失效真实头像回退首字')
    profile.options.session.profile = { ...profile.options.session.profile, nickname: '已更新昵称', avatar: 'isolated-avatar-b' }
    await profile.render()
    assert.equal(profile.props('.student-profile-user__avatar-image').src, 'isolated-avatar-b', '编辑后的新头像重新显示')
    assert.ok(profile.container.textContent.includes('已更新昵称'), '资料读取同一个会话源')
    profile.options.session.profile = { ...profile.options.session.profile, cover: 'isolated-cover-a' }
    await profile.render()
    assert.equal(profile.props('.student-profile-cover__image').src, 'isolated-cover-a')
    assert.ok(profile.container.querySelector('.student-profile-hero--with-cover'), '真实封面启用融合背景状态')
    await profile.event(() => profile.props('.student-profile-cover__image').onError())
    assert.equal(profile.container.querySelector('.student-profile-cover__image'), null, '封面失败不破坏资料区')
    assert.equal(profile.container.querySelector('.student-profile-hero--with-cover'), null, '图片失败同时恢复浅色背景状态')
    profile.options.session.profile = { ...profile.options.session.profile, cover: 'isolated-cover-b' }
    await profile.render()
    assert.equal(profile.props('.student-profile-cover__image').src, 'isolated-cover-b', '新封面不沿用旧图失败状态')
    assert.ok(profile.container.querySelector('.student-profile-hero--with-cover'))
    profile.options.session.profile = { ...profile.options.session.profile, cover: '' }
    await profile.render()
    assert.equal(profile.container.querySelector('.student-profile-hero--with-cover'), null, '恢复默认后移除照片状态')
    await profile.show()
    await profile.event(() => { profile.props('.student-profile-user__edit').onClick(); profile.props('.student-profile-user__edit').onClick() })
    assert.equal(navigations.length, 4, '恢复默认封面后，编辑资料入口仍可进入并防止重复导航')
    assert.equal(navigations[3].url, '/pages/edit-profile/index')
    await profile.event(() => navigations[3].reject(new Error('isolated editor navigation failure')))
    profile.options.session.switchIdentity = () => { const task = deferred(); switches.push(task); return task.promise }
    await profile.render()
    await profile.event(() => profile.all('.student-profile-role')[1].onClick())
    await profile.event(() => { profile.props('.test-identity-modal').onConfirm(); profile.props('.test-identity-modal').onConfirm() })
    assert.equal(switches.length, 1, '身份确认同步防重，不重复切换')
    await profile.event(() => switches[0].reject(new Error('isolated switch failure')))

    const requests = { favorites: [], posts: [], comments: [], items: [] }
    const fetchPage = (tab) => (page) => { const task = deferred(); requests[tab].push({ ...task, page }); return task.promise }
    const content = await mount('my-content', {
      params: { tab: 'posts' },
      community: { getMyPosts: fetchPage('posts'), getMyFavoritePosts: fetchPage('favorites'), getMyComments: fetchPage('comments') },
      trade: { getMyProductsPage: fetchPage('items') },
    })
    await content.show()
    assert.equal(requests.posts.length, 1, '我的内容首次展示只读取一次')
    await content.event(() => requests.posts[0].resolve({ items: [post('p1')], total: 3, page: 1 }))
    const retainedPostsScroll = content.container.querySelector('.my-content-body--active')
    await content.event(() => {
      content.props('.my-content-body--active').onScroll({ detail: { scrollTop: 640 } })
      content.props('.my-content-body--active').onScrollToLower()
      content.props('.my-content-body--active').onScrollToLower()
    })
    assert.equal(requests.posts.length, 2, '同帧重复触底只请求一次下一页')
    await content.event(() => requests.posts[1].resolve({ items: [post('p1'), post('p2')], total: 3, page: 2 }))
    assert.equal(content.all('.my-content-body--active .my-content-row').length, 2, '分页按 ID 去重')
    await content.event(() => content.all('.my-content-tab')[0].onClick())
    await content.event(() => requests.favorites[0].resolve({ items: [], total: 0, page: 1 }))
    await content.event(() => content.props('.my-content-body--active').onScroll({ detail: { scrollTop: 48 } }))
    await content.event(() => content.all('.my-content-tab')[1].onClick())
    assert.equal(content.container.querySelector('.my-content-body--active'), retainedPostsScroll, '切栏不重建已访问的滚动容器')
    assert.equal(content.props('.my-content-body--active').scrollTop, 640, '切栏恢复各自滚动位置')
    assert.equal(requests.posts.length, 2, '切栏不重复读取已加载页面')
    await content.event(() => content.props('.my-content-body--active').onScrollToLower())
    assert.equal(requests.posts[2].page, 3, '切回来延续原栏目的分页进度')
    await content.event(() => requests.posts[2].reject(new Error('isolated next page failure')))
    assert.equal(content.all('.my-content-body--active .my-content-row').length, 2, '分页失败保留原列表')
    await content.show()
    assert.deepEqual(requests.posts.slice(3).map((item) => item.page), [1, 2], '详情返回刷新已加载范围')
    assert.equal(content.all('.my-content-body--active .my-content-row').length, 2, '返回刷新不中断已显示内容')
    await content.event(() => { requests.posts[3].resolve({ items: [post('p1')], total: 2, page: 1 }); requests.posts[4].reject(new Error('isolated refresh failure')) })
    assert.equal(content.all('.my-content-body--active .my-content-row').length, 2, '刷新部分失败保留完整旧列表')
    assert.ok(content.container.querySelector('.my-content-refresh--retry'))
    const retainedScroll = content.container.querySelector('.my-content-body--active')
    await content.event(() => content.props('.my-content-refresh--retry').onClick())
    await content.event(() => { requests.posts[5].resolve({ items: [post('p1')], total: 2, page: 1 }); requests.posts[6].resolve({ items: [post('p2')], total: 2, page: 2 }) })
    assert.equal(content.container.querySelector('.my-content-body--active'), retainedScroll, '后台刷新不重建正在阅读的滚动容器')
    assert.equal(content.props('.my-content-body--active').scrollTop, 640, '返回更新后保留当前滚动位置')
    assert.equal(content.container.querySelector('.my-content-refresh--retry'), null)
    await content.event(() => content.all('.my-content-tab')[2].onClick())
    content.options.visibility = { ...visibility, comment: false }
    await content.render()
    await content.event(() => requests.comments[0].resolve({ items: [{ id: 'c1', postId: 'p1', content: '不应显示的旧评论' }], total: 1, page: 1 }))
    assert.ok(!content.container.textContent.includes('不应显示的旧评论'), '关闭栏目立即隔离未完成请求')
    content.options.visibility = { ...visibility }
    await content.render()
    assert.ok(content.container.querySelector('.my-content-tab--active').textContent.includes('收藏'), '重开栏目不抢走用户当前选择')
    assert.equal(requests.comments.length, 1, '重新开放但尚未选中的栏目不自动读取')
    content.options.session = { ...session(), token: 'isolated-other-session', profile: { ...session().profile, id: 'fixture-account-c' } }
    await content.render()
    assert.equal(content.all('.my-content-row').length, 0, '更换账号后不显示上个账号的内容')
    const publicValue = { profile: { id: 7, nickname: '封面用户', avatar: 'test-avatar', cover: 'test-cover', school: '不展示的学校', campus: '' }, followingCount: 12, followerCount: 8, likedCount: 26, isFollowing: false }
    const publicCalls = [], productReads = [], profileRoutes = [], follows = []
    let returned = 0
    const publicCommunity = {
      getPublicCommunityProfile: () => { const task = deferred(); publicCalls.push(task); return task.promise },
      getPublicUserPosts: async () => ({ items: [post('11')], total: 1, page: 1 }),
      setUserFollowing: (id, value) => { const task = deferred(); follows.push({ ...task, id, value }); return task.promise },
    }
    const publicPage = await mount('user-profile', {
      params: { userId: '7', tab: 'secondhand' }, visibility: { ...visibility, likes: true, following: true },
      community: publicCommunity,
      trade: { getPublicUserProducts: async (...args) => { productReads.push(args); return { items: [], total: 0, page: 1 } } },
      taro: { navigateBack: async () => { returned++ }, navigateTo: async ({ url }) => profileRoutes.push(url) },
    })
    await publicPage.event(() => publicPage.props('.public-profile-nav__back').onClick())
    assert.equal(returned, 1, '加载时也可以返回')
    await publicPage.event(() => publicCalls[0].resolve(publicValue))
    assert.equal(publicPage.props('[data-component="Swiper"]').current, 1, '卖家入口默认打开二手')
    assert.equal(publicPage.props('.public-profile-cover__image').src, 'test-cover', '公开主页展示服务端真实封面')
    assert.ok(!publicPage.container.textContent.includes('不展示的学校'))
    await publicPage.event(() => { publicPage.props('.public-profile-action--primary').onClick(); publicPage.props('.public-profile-action--primary').onClick() })
    assert.equal(follows.length, 1, '同帧连点关注只写一次')
    await publicPage.event(() => follows[0].resolve())
    assert.ok(publicPage.container.textContent.includes('已关注'))
    await publicPage.event(() => publicPage.props('[data-item-id="secondhand"] .public-profile-scroll').onScroll({ detail: { scrollTop: 655 } }))
    await publicPage.event(() => publicPage.props('[data-item-id="secondhand"] .public-profile-tab').onClick())
    assert.equal(publicPage.props('[data-component="Swiper"]').current, 0)
    assert.equal(publicPage.props('[data-item-id="secondhand"] .public-profile-scroll').scrollTop, 655, '不同内容标签分别保留滚动位置')
    await publicPage.event(() => publicPage.props('.public-profile-cover__image').onError())
    await publicPage.event(() => publicPage.props('.public-profile-user__avatar-image').onError())
    assert.equal(publicPage.all('.public-profile-cover__image').length, 0, '封面失效回退浅色底，保留头像资料')
    assert.equal(publicPage.all('.public-profile-user__avatar-image').length, 0, '头像失效回退首字')
    await publicPage.show()
    await publicPage.show()
    await publicPage.event(() => publicCalls[1].resolve({ ...publicValue, profile: { ...publicValue.profile, cover: 'returned-cover', avatar: 'returned-avatar' } }))
    assert.equal(publicPage.props('.public-profile-cover__image').src, 'returned-cover', '从编辑页返回后同步封面，不重置内容滚动')
    assert.equal(publicPage.props('[data-item-id="secondhand"] .public-profile-scroll').scrollTop, 655)
    publicPage.options.params = { userId: '999', tab: 'posts' }
    publicPage.options.visibility.secondhand = false
    await publicPage.render()
    await publicPage.event(() => publicCalls[2].resolve({ ...publicValue, profile: { ...publicValue.profile, cover: 'test-cover-new', avatar: 'test-avatar-new' } }))
    assert.equal(publicPage.props('.public-profile-cover__image').src, 'test-cover-new', '新封面不继承旧图失败状态')
    assert.equal(publicPage.props('.public-profile-user__avatar-image').src, 'test-avatar-new')
    assert.equal(publicPage.all('[data-component="SwiperItem"]').length, 1)
    assert.equal(productReads.length, 1, '关闭二手不请求或显示商品')
    assert.ok(publicPage.container.textContent.includes('用户号 7'), '子页成为当前 router 不改变原主页用户')

    const plainProfile = await mount('user-profile', {
      params: { userId: '7' }, session: { ...session(), profile: { ...session().profile, id: '7' } },
      community: { ...publicCommunity, getPublicCommunityProfile: async () => ({ ...publicValue, profile: { id: 7, nickname: '本人', avatar: '', school: '', campus: '' } }) },
      trade: { getPublicUserProducts: async () => ({ items: [], total: 0, page: 1 }) },
      taro: { navigateTo: async ({ url }) => profileRoutes.push(url) },
    })
    assert.equal(plainProfile.props('[data-component="Swiper"]').current, 0, '帖子入口默认打开帖子')
    assert.equal(plainProfile.all('.public-profile-cover__image').length, 0, '旧接口无 cover 可正常查看')
    await plainProfile.event(() => plainProfile.props('.public-profile-action--primary').onClick())
    assert.equal(profileRoutes.at(-1), '/pages/edit-profile/index', '自己的公开主页仍可编辑资料')
    assert.equal(plainProfile.props('.public-profile-action--secondary').openType, 'share')
    const invalid = await mount('user-profile', { params: { userId: '../7' }, community: { getPublicCommunityProfile: () => assert.fail('无效 ID 不请求') } })
    assert.ok(invalid.container.textContent.includes('用户主页入口无效'))
    assert.ok(invalid.props('.public-profile-nav__back').onClick)
    assert.ok(invalid.props('.public-profile-retry').onClick)
    console.log('profile presentation: isolated page behavior ok')
  } finally {
    await act(async () => { for (const root of roots) root.unmount() })
    names.forEach((name, index) => { if (descriptors[index]) Object.defineProperty(globalThis, name, descriptors[index]); else delete globalThis[name] })
    dom.window.close()
  }
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
