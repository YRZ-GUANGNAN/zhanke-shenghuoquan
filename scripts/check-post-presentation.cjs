const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const sourcePath = path.resolve(__dirname, '../apps/miniapp/src/utils/post-presentation.ts')
const loaded = new Module(sourcePath, module)
loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText, sourcePath)
const { preparePostMedia, removeFailedPostMedia, postBodyWithoutTitle, formatPostDate } = loaded.exports

// Run real page hooks with isolated service doubles; no login, network or production writes.
function deferred() {
  let resolve, reject
  const promise = new Promise((accept, decline) => { resolve = accept; reject = decline })
  return { promise, resolve, reject }
}

async function pageBehavior() {
  const { JSDOM } = require('jsdom')
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { act } = React
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const original = globals.map((key) => Object.getOwnPropertyDescriptor(globalThis, key))
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const originalError = console.error
  const loggedErrors = []
  console.error = (...args) => loggedErrors.push(args)

  const component = (name) => (props) => React.createElement('div', {
    className: props.className,
    id: props.id,
    'data-component': name,
    ref: (node) => { if (node) node.taroProps = props },
  }, props.children, props.title, props.description)
  const View = component('View')
  const visibility = { forum: true, post: true, comment: true, canComment: true, likes: true, favorites: true, following: true }
  const roots = []
  const fakePost = { id: 'p1', authorId: 'u1', author: '测试作者', school: '测试学校', topic: '', tags: [], title: '测试标题', content: '保留的正文', images: [], reviewStatus: 'approved', commentCount: 1, likedCount: 0, favoriteCount: 0, createdAt: '' }

  async function mount(page, options) {
    options.visibility ||= { ...visibility }
    const pagePath = path.resolve(__dirname, `../apps/miniapp/src/pages/${page}/index.tsx`)
    const modulePage = new Module(pagePath, module)
    const taro = { getCurrentInstance: () => ({ router: { params: options.params || {} } }), showToast: async () => {}, navigateTo: async () => {}, getImageInfo: async () => ({ width: 600, height: 800 }), ...options.taro }
    modulePage.require = (id) => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/taro') return { nextTick: (callback) => queueMicrotask(callback), useDidShow: (callback) => React.useEffect(() => callback(), []), useDidHide: () => {}, useResize: () => {} }
      if (id === '@tarojs/components') return Object.fromEntries(['Image', 'Input', 'ScrollView', 'Swiper', 'SwiperItem', 'Text', 'Textarea', 'View'].map((name) => [name, component(name)]))
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('/feature-page-gate')) return { withFeaturePage: (pageComponent) => pageComponent, FeaturePageGate: View }
      if (id.endsWith('/use-feature-visibility')) return { useFeatureVisibility: () => options.visibility }
      if (id.endsWith('/use-identity-guard')) return { useIdentityGuard: () => true }
      if (id.endsWith('/stores/session')) return { useSessionStore: (selector) => selector({ token: options.token || 'test-token', profile: { id: 'test-user' } }) }
      if (id.endsWith('/session-recovery')) return { requestWithSingleSessionRecovery: (run) => run() }
      if (id.endsWith('/use-page-share')) return { usePageShare: () => {} }
      if (id.endsWith('/share-model')) return { publicShareId: (id) => id || '' }
      if (id.endsWith('/capsule-page-header')) return { default: (props) => React.createElement(View, { className: 'test-header' }, props.left, props.center, props.right) }
      if (id.endsWith('/runtime-session-gate') || id.endsWith('/app-icon')) return { default: View }
      if (id.endsWith('/student-bottom-nav')) return { default: View }
      if (id.endsWith('/campus-info-home')) return { default: () => React.createElement(View, { className: 'test-campus-info' }) }
      if (id.endsWith('/components/ui')) return { AppIconButton: View, AppState: View }
      if (id.endsWith('/services/taro')) return { default: taro }
      if (id.endsWith('/capsule-safe')) return { getCapsuleSafeStyle: () => ({}) }
      if (id.endsWith('/navigation')) return { navigateBackOrHome: async () => {} }
      if (id.endsWith('/post-presentation')) return loaded.exports
      if (id.endsWith('/use-gallery-presentation') || id.endsWith('/gallery-presentation')) {
        const helperPath = path.resolve(__dirname, '../apps/miniapp/src', id.endsWith('/use-gallery-presentation') ? 'hooks/use-gallery-presentation.ts' : 'utils/gallery-presentation.ts')
        const helper = new Module(helperPath, module)
        helper.require = modulePage.require
        helper._compile(ts.transpileModule(fs.readFileSync(helperPath, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, helperPath)
        return helper.exports
      }
      if (id.endsWith('/services/api')) return { isApiError: () => false }
      if (id.endsWith('/b-community')) return options.community || {}
      if (id.endsWith('/services/media')) return options.media || {}
      if (id.endsWith('/c-trade')) return options.trade || {}
      if (id.endsWith('/d-navigation')) return options.navigation || { listNavigationItems: async () => [] }
      if (id.endsWith('/e-im')) return { listIMConversations: async () => [] }
      throw new Error(`Unexpected page test dependency: ${id}`)
    }
    modulePage._compile(ts.transpileModule(fs.readFileSync(pagePath, 'utf8'), {
      compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    }).outputText, pagePath)
    const container = document.createElement('main')
    document.body.appendChild(container)
    const root = createRoot(container)
    roots.push(root)
    await act(async () => { root.render(React.createElement(modulePage.exports.default)) })
    return {
      container, options,
      render: async () => { await act(async () => root.render(React.createElement(modulePage.exports.default))) },
      all: (selector) => Array.from(container.querySelectorAll(selector)).map((node) => node.taroProps),
      props: (selector) => {
        const element = container.querySelector(selector)
        assert.ok(element, `page element must exist: ${selector}`)
        return element.taroProps
      },
      event: async (run) => { await act(async () => { run() }) },
    }
  }

  try {
    const homePostRequests = [], homeProductRequests = []
    const home = await mount('home', {
      visibility: { ...visibility, available: true, presentationMode: 'original', secondhand: true, messages: true, secondhand_chat: true, campus_tools: true, search: true },
      community: { getCommunityPostsPage: (query) => { const task = deferred(); homePostRequests.push({ query, ...task }); return task.promise } },
      trade: { getProductsPage: (query) => { const task = deferred(); homeProductRequests.push({ query, ...task }); return task.promise } },
    })
    assert.equal(homePostRequests.length, 1)
    assert.equal(homeProductRequests.length, 1)
    await home.event(() => {
      homePostRequests[0].resolve({ items: [fakePost], page: 1, total: 11 })
      homeProductRequests[0].resolve({ items: [], page: 1, total: 0 })
    })
    await home.event(() => home.props('.student-home-content').onScroll({ detail: { scrollTop: 640 } }))
    await home.event(() => home.all('.student-home-header__tab')[2].onClick())
    await home.event(() => home.props('.student-home-content').onScroll({ detail: { scrollTop: 96 } }))
    await home.event(() => home.all('.student-home-header__tab')[1].onClick())
    assert.equal(home.props('.student-home-content').scrollTop, 640, '切换栏目回来恢复发现滚动位置')
    assert.equal(homePostRequests.length, 1, '切换栏目不重载发现内容')
    const homeScroll = home.container.querySelector('.student-home-content')
    const homeCard = home.container.querySelector('.student-post-card')
    home.options.visibility = { ...home.options.visibility, favorites: false }
    await home.render()
    assert.equal(homePostRequests.length, 1, '无关功能更新不重载发现')
    assert.equal(home.container.querySelector('.student-home-content'), homeScroll)
    assert.equal(home.container.querySelector('.student-post-card'), homeCard)
    await home.event(() => {
      home.props('.student-home-content').onScrollToLower()
      home.props('.student-home-content').onScrollToLower()
    })
    assert.equal(homePostRequests.length, 2, '发现分页同帧防重')
    home.options.visibility = { ...home.options.visibility, forum: false }
    await home.render()
    assert.equal(homeProductRequests.length, 2, '数据来源改变立即重新读取，旧分页不能占住请求锁')
    await home.event(() => homeProductRequests[1].resolve({ items: [], page: 1, total: 0 }))
    await home.event(() => homePostRequests[1].resolve({ items: [{ ...fakePost, title: '不应回写的旧帖子' }], page: 2, total: 11 }))
    assert.ok(!home.container.textContent.includes('不应回写的旧帖子'), '来源关闭后旧响应不能恢复被关闭的内容')
    home.options.visibility = { ...home.options.visibility, forum: true }
    await home.render()
    await home.event(() => {
      homePostRequests[2].resolve({ items: [fakePost], page: 1, total: 1 })
      homeProductRequests[2].resolve({ items: [], page: 1, total: 0 })
    })
    assert.equal(home.props('.student-home-content').scrollTop, 0, '来源改变后新列表从顶部开始')
    home.options.visibility = { ...home.options.visibility, presentationMode: 'campus_info' }
    await home.render()
    assert.ok(home.container.querySelector('.test-campus-info'), '校园指南模式继续使用独立首页')
    assert.equal(home.container.querySelector('.student-home-content'), null)

    const searches = []
    const navigation = []
    const search = await mount('search', {
      params: { scope: 'community' },
      community: {
        listCommunitySearchHistory: async () => [], listCommunityHotSearches: async () => [],
        searchCommunityPosts: (keyword, page) => { const task = deferred(); searches.push({ keyword, page, ...task }); return task.promise },
      },
      taro: { navigateTo: () => { const task = deferred(); navigation.push(task); return task.promise } },
    })
    const input = (value) => search.props('.student-search-box__input').onInput({ detail: { value } })
    const submit = () => search.props('.student-search-submit').onClick()
    await search.event(() => input('旧词'))
    await search.event(() => { submit(); submit() })
    assert.equal(searches.length, 1, '重复搜索点击只发一次相同查询')
    await search.event(() => input('新词'))
    await search.event(submit)
    assert.equal(searches.length, 2, '未完成旧查询时仍可提交新词')
    await search.event(() => searches[1].resolve({ items: [fakePost], page: 1, total: 3 }))
    await search.event(() => searches[0].resolve({ items: [{ ...fakePost, id: 'old', title: '过期结果' }], page: 1, total: 1 }))
    assert.ok(!search.container.textContent.includes('过期结果'), '较晚到达的旧查询不能覆盖新结果')
    const resultNode = search.container.querySelector('.student-search-result')
    await search.event(() => { search.props('.student-search-result').onClick(); search.props('.student-search-result').onClick() })
    assert.equal(navigation.length, 1, '结果连续点击只打开一次详情')
    await search.event(() => navigation[0].resolve())
    assert.equal(search.container.querySelector('.student-search-result'), resultNode, '阅读标记不重建结果行或滚动容器')
    assert.ok(resultNode.classList.contains('student-search-result--read'), '成功打开后本会话记为已读')
    assert.equal(searches.length, 2, '详情返回更新阅读表现不重复搜索')
    await search.event(() => { search.props('.student-search-content').onScrollToLower(); search.props('.student-search-content').onScrollToLower() })
    assert.equal(searches.length, 3, '分页同一帧重复触底只发一次请求')
    assert.equal(searches[2].keyword, '新词')
    await search.event(() => searches[2].reject(new Error('test pagination offline')))
    assert.equal(search.container.querySelector('.student-search-result'), resultNode, '分页失败保留原结果节点')
    assert.ok(resultNode.compareDocumentPosition(search.container.querySelector('.student-search-state--error')) & dom.window.Node.DOCUMENT_POSITION_FOLLOWING, '分页错误位于内容之后')
    await search.event(() => input('待取消'))
    await search.event(submit)
    await search.event(() => search.props('.student-search-results__reset').onClick())
    await search.event(() => searches[3].resolve({ items: [fakePost], page: 1, total: 9 }))
    await search.event(() => search.props('.student-search-content').onScrollToLower())
    assert.equal(searches.length, 4, '退出结果态后旧响应和触底不能继续分页')
    assert.equal(search.container.querySelector('.student-search-result'), null)

    let postLoads = 0
    const commentRequests = []
    const detail = await mount('post-detail', {
      params: { id: fakePost.id, anchor: 'comments' },
      community: {
        getCommunityPost: async () => { postLoads += 1; return fakePost },
        getCommunityViewerState: async () => ({ liked: false, favorited: false, followingAuthor: false }),
        getPostCommentsPage: (id, args) => { const task = deferred(); commentRequests.push({ id, ...args, ...task }); return task.promise },
      },
    })
    assert.ok(detail.container.textContent.includes('保留的正文'), '评论请求未完成时正文仍可阅读')
    assert.equal(detail.props('.post-detail-scroll').scrollIntoView, '', '评论未完成布局时不提前定位到底部')
    await detail.event(() => commentRequests[0].reject(new Error('test comments offline')))
    const bodyNode = detail.container.querySelector('.post-detail-content')
    await detail.event(() => detail.props('.post-detail-input').onInput({ detail: { value: '保留输入' } }))
    await detail.event(() => detail.props('.post-detail-comments__bottom--retry').onClick())
    assert.equal(postLoads, 1, '评论失败重试不能重载整篇帖子')
    assert.equal(detail.container.querySelector('.post-detail-content'), bodyNode)
    assert.equal(detail.props('.post-detail-input').value, '保留输入')
    await detail.event(() => commentRequests[1].resolve({ items: [], page: 1, total: 0 }))
    assert.ok(detail.container.textContent.includes('共 0 条评论'), '真实零评论计数不能被详情旧计数覆盖')
    assert.equal(detail.props('.post-detail-scroll').scrollIntoView, 'post-comments', '评论读取结束后应用来源锚点')

    const approval = deferred()
    const publishCalls = []
    let approvalCalls = 0
    const publish = await mount('publish-post', {
      community: {
        listHotCommunityTags: async () => [{ tag: '初始标签' }, { tag: '另一个标签' }],
        createCommunityClientRequestId: () => 'test-operation',
        createCommunityPost: async (payload) => { publishCalls.push(payload); throw new Error('test publish offline') },
      },
      media: { waitForMediaApprovals: () => { approvalCalls += 1; return approval.promise } },
    })
    await publish.event(() => publish.props('.publish-post-textarea').onInput({ detail: { value: '原始正文' } }))
    await publish.event(() => publish.props('.publish-post-topic').onClick())
    await publish.event(() => { publish.props('.publish-post-nav__publish').onClick(); publish.props('.publish-post-nav__publish').onClick() })
    assert.equal(approvalCalls, 1, '连续点击发布只进入一个提交任务')
    await publish.event(() => {
      publish.props('.publish-post-topic').onClick()
      publish.props('.publish-post-textarea').onInput({ detail: { value: '提交时误触输入' } })
    })
    await publish.event(() => approval.resolve())
    assert.equal(publishCalls.length, 1)
    assert.equal(publishCalls[0].content, '原始正文')
    assert.deepEqual(publishCalls[0].tags, ['初始标签'], '提交中标签编辑不能改变发布快照')
    assert.equal(publish.props('.publish-post-textarea').value, '原始正文', '失败保留正文')
    assert.ok(publish.container.querySelector('.publish-post-error'), '失败有持续内联说明')
    await publish.event(() => publish.props('.publish-post-nav__publish').onClick())
    assert.equal(publishCalls[1].clientRequestId, publishCalls[0].clientRequestId, '结果未知时重试沿用幂等身份')

    let createdPosts = 0
    const publishNavigation = []
    const published = await mount('publish-post', {
      community: {
        listHotCommunityTags: async () => [], createCommunityClientRequestId: () => 'published-once',
        createCommunityPost: async () => { createdPosts += 1; return { id: '123' } },
      },
      media: { waitForMediaApprovals: async () => {} },
      taro: { redirectTo: (query) => { const task = deferred(); publishNavigation.push({ query, ...task }); return task.promise } },
    })
    await published.event(() => published.props('.publish-post-textarea').onInput({ detail: { value: '只发布一次的正文' } }))
    await published.event(() => published.props('.publish-post-nav__publish').onClick())
    await published.event(() => publishNavigation[0].reject(new Error('test navigation offline')))
    assert.equal(createdPosts, 1)
    assert.ok(published.container.textContent.includes('帖子已提交，页面打开失败'))
    assert.equal(published.props('.publish-post-textarea').disabled, true)
    await published.event(() => {
      published.props('.publish-post-nav__publish').onClick()
      published.props('.publish-post-nav__publish').onClick()
    })
    assert.equal(publishNavigation.length, 2, '成功后连续点击只重试一次导航')
    assert.equal(createdPosts, 1, '跳转失败后不能重新创建帖子')
    assert.equal(publishNavigation[1].query.url, '/pages/post-detail/index?id=123')
    await published.event(() => publishNavigation[1].resolve())
    assert.ok(loggedErrors.every(([message]) => typeof message === 'string' && !message.startsWith('Warning:')), '页面状态测试不能产生 React 更新警告')
    console.log('post page behavior: PASS (home retention, source races, search, comments, publishing snapshot and navigation retry)')
  } finally {
    for (const root of roots) await act(async () => { root.unmount() })
    dom.window.close()
    console.error = originalError
    globals.forEach((key, index) => original[index] ? Object.defineProperty(globalThis, key, original[index]) : delete globalThis[key])
  }
}

async function main() {
  let finishFirst
  const calls = []
  const result = preparePostMedia(['first', '', ' second ', 'broken', 'first', 'invalid'], (src) => {
    calls.push(src)
    if (src === 'first') return new Promise((resolve) => { finishFirst = resolve })
    if (src === 'second') return Promise.resolve({ width: 1200, height: 600 })
    if (src === 'broken') return Promise.reject(new Error('unavailable'))
    return Promise.resolve({ width: 0, height: 600 })
  })
  finishFirst({ width: 600, height: 800 })
  assert.deepEqual((await result).map((item) => item.src), ['first', 'second'], '异步图片请求完成顺序不能改变帖子附件顺序')
  assert.deepEqual(calls, ['first', 'second', 'broken', 'invalid'], '空地址不发请求，重复附件只校验一次')
  assert.deepEqual(await preparePostMedia([], () => assert.fail('no media request expected')), [])
  assert.deepEqual(await preparePostMedia(['offline'], async () => { throw new Error('offline') }), [])
  assert.deepEqual(await preparePostMedia(['bad'], async () => ({ width: Infinity, height: 50 })), [])
  const gallery = { items: ['a', 'b', 'c'].map((src) => ({ src, width: 600, height: 800 })), index: 1 }
  const earlierFailure = removeFailedPostMedia(gallery, 'a')
  assert.equal(earlierFailure.items[earlierFailure.index].src, 'b', '前面的图片失败不能把正在看的图片换掉')
  const laterFailure = removeFailedPostMedia(gallery, 'c')
  assert.equal(laterFailure.items[laterFailure.index].src, 'b', '后面的图片失败不改变当前图片')
  const activeFailure = removeFailedPostMedia(gallery, 'b')
  assert.equal(activeFailure.items[activeFailure.index].src, 'c', '当前图片失败时选择后一张有效附件')
  const lastFailure = removeFailedPostMedia({ ...gallery, index: 2 }, 'c')
  assert.equal(lastFailure.items[lastFailure.index].src, 'b', '最后一张失败时回到前一张')
  const emptyGallery = removeFailedPostMedia({ items: [gallery.items[0]], index: 0 }, 'a')
  assert.deepEqual(emptyGallery, { items: [], index: 0 }, '全部失败时不能出现负索引')
  assert.equal(removeFailedPostMedia(gallery, 'missing'), gallery, '迟到的重复失败不触发额外重排')
  assert.deepEqual(gallery.items.map((item) => item.src), ['a', 'b', 'c'], '错误处理不能修改此前的状态快照')
  assert.equal(postBodyWithoutTitle('第一行', '第一行\r\n\r\n正文\n第二段'), '正文\n第二段')
  assert.equal(postBodyWithoutTitle('一条内容', '一条内容'), '')
  assert.equal(postBodyWithoutTitle('前缀', '前缀后面还有内容'), '前缀后面还有内容', '不能把仅匹配前缀的正文截掉')
  assert.equal(postBodyWithoutTitle('', '正文'), '正文')
  assert.equal(formatPostDate(''), '')
  assert.equal(formatPostDate('刚刚'), '刚刚', '无法解析的既有文案原样保留')
  const date = new Date(2026, 9, 14, 9, 5)
  assert.equal(formatPostDate(date.toISOString()), '2026.10.14 09:05')
  console.log('post presentation behavior: PASS (media identity, failure recovery, text and dates)')
  await pageBehavior()
}

main().catch((error) => { console.error(error); process.exitCode = 1 })
