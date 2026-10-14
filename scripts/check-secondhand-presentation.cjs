const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

// Execute actual page hooks with explicit test doubles; never connect to a server or persist data.
const rootDir = path.resolve(__dirname, '..')
const product = (id = '1', changes = {}) => ({ id, sellerId: 'seller', title: '测试商品 ' + id, description: '测试商品说明', price: 12.34, category: '数码', images: [], sellerName: '测试卖家', sellerAvatar: '', campus: '测试校区', condition: '二手', createdAt: '', status: 'selling', reviewStatus: 'approved', favoriteCount: 0, ...changes })
const comment = (id, changes = {}) => ({ id, productId: '1', parentCommentId: '', content: '测试留言 ' + id, reviewStatus: 'approved', author: { id: 'author', nickname: '测试同学', avatar: '' }, replies: [], createdAt: '', isSeller: false, ...changes })
const pageResult = (items, page = 1, total = items.length) => ({ items, page, total, pageSize: 20 })
function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

async function main() {
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const globals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const originals = globals.map((key) => Object.getOwnPropertyDescriptor(globalThis, key))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const roots = []
  const errors = []
  const originalError = console.error
  console.error = (...args) => errors.push(args)
  const component = (name) => (props) => props.visible === false ? null : React.createElement('div', {
    className: props.className, id: props.id, 'data-component': name,
    ref: (node) => { if (node) node.taroProps = props },
  }, props.children, props.title, props.description)
  const View = component('View')
  const defaultVisibility = { secondhand: true, secondhand_comments: true, secondhand_publish: true, secondhand_chat: true, search: true, forum: true, favorites: true }

  async function mount(file, options = {}, initialProps = {}) {
    options.visibility ??= { ...defaultVisibility }
    let showCallback, sequence = 0
    const cache = new Map()
    const taro = {
      getCurrentInstance: () => ({ router: { params: options.params ?? { id: '1' } } }),
      showToast: async () => {}, navigateTo: async () => {}, navigateBack: async () => {}, redirectTo: async () => {}, previewImage: async () => {},
      getImageInfo: async () => ({ width: 600, height: 800 }), ...options.taro,
    }
    function load(relative) {
      const fullPath = path.join(rootDir, 'apps/miniapp/src', relative)
      if (cache.has(fullPath)) return cache.get(fullPath)
      const loaded = new Module(fullPath, module)
      loaded.require = (id) => {
        if (id === 'react' || id === 'react/jsx-runtime') return require(id)
        if (id === '@tarojs/components') return Object.fromEntries(['Image', 'Input', 'ScrollView', 'Swiper', 'SwiperItem', 'Text', 'Textarea', 'View'].map((name) => [name, component(name)]))
        if (id === '@tarojs/taro') return { useDidShow: (handler) => { showCallback = handler; React.useEffect(() => handler(), []) }, useDidHide: (handler) => { options.hide = handler }, useResize: (handler) => { options.resize = handler } }
        if (id.endsWith('.scss')) return {}
        if (id.endsWith('/feature-page-gate')) return { withFeaturePage: (Page) => Page }
        if (id.endsWith('/use-feature-visibility')) return { useFeatureVisibility: () => options.visibility }
        if (id.endsWith('/use-page-share')) return { usePageShare: () => {} }
        if (id.endsWith('/capsule-page-header')) return { default: (props) => React.createElement(View, { className: 'test-header' }, props.left, props.center, props.right) }
        if (id.endsWith('/student-bottom-nav') || id.endsWith('/runtime-session-gate') || id.endsWith('/app-icon')) return { default: View }
        if (id.endsWith('/components/ui')) return Object.fromEntries(['AppIconButton', 'AppState', 'AppDialog', 'AppNotice', 'AppActionSheet'].map((name) => [name, component(name)]))
        if (id.endsWith('/services/taro')) return { default: taro }
        if (id.endsWith('/capsule-safe')) return { getCapsuleSafeStyle: () => ({}) }
        if (id.endsWith('/services/api')) return { isApiError: (error) => Boolean(error?.isApiError) }
        if (id.endsWith('/c-trade')) return { listProductComments: async () => pageResult([]), ...options.trade }
        if (id.endsWith('/services/media')) return options.media || {}
        if (id.endsWith('/client-request-id')) return { createClientRequestId: () => 'test-request-' + (++sequence) }
        if (id.endsWith('/stores/session')) return { useSessionStore: (selector) => selector({ profile: { id: options.userId || 'viewer' } }) }
        if (id.endsWith('/post-presentation')) return load('utils/post-presentation.ts')
        if (id.endsWith('/gallery-presentation')) return load('utils/gallery-presentation.ts')
        if (id.endsWith('/use-gallery-presentation')) return load('hooks/use-gallery-presentation.ts')
        if (id === './product-comments') return load('pages/secondhand-detail/product-comments.tsx')
        throw new Error('Unexpected test dependency: ' + id)
      }
      loaded._compile(ts.transpileModule(fs.readFileSync(fullPath, 'utf8'), {
        compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
      }).outputText, fullPath)
      cache.set(fullPath, loaded.exports)
      return loaded.exports
    }
    const exports = load(file)
    const container = document.createElement('main')
    document.body.appendChild(container)
    const root = createRoot(container)
    roots.push(root)
    let currentProps = initialProps
    const render = async (props = currentProps) => { currentProps = props; await React.act(async () => root.render(React.createElement(exports.default, props))) }
    await render()
    return {
      container, exports, options, render,
      props(selector) { const node = container.querySelector(selector); assert.ok(node, 'Missing test element: ' + selector); return node.taroProps },
      all(selector) { return Array.from(container.querySelectorAll(selector)).map((node) => node.taroProps) },
      event: async (callback) => { await React.act(async () => { callback() }) },
      show: async () => { await React.act(async () => showCallback()) },
    }
  }

  try {
    const requests = []
    const list = await mount('pages/student-secondhand/index.tsx', { trade: {
      getProductsPage: (query) => { const task = deferred(); requests.push({ query, ...task }); return task.promise },
    } })
    assert.equal(requests.length, 1)
    await list.event(() => list.all('.student-secondhand-category')[1].onClick())
    assert.equal(requests[1].query.category, '数码')
    const firstTwenty = Array.from({ length: 20 }, (_, index) => product(String(index + 1)))
    await list.event(() => requests[1].resolve(pageResult(firstTwenty, 1, 21)))
    await list.event(() => requests[0].resolve(pageResult([product('99', { title: '过期分类结果' })])))
    assert.ok(!list.container.textContent.includes('过期分类结果'), '分类旧响应不能覆盖当前分类')
    const listNode = list.container.querySelector('.student-secondhand-scroll')
    const itemNode = list.container.querySelector('.secondhand-card')
    await list.event(() => { list.props('.student-secondhand-scroll').onScrollToLower(); list.props('.student-secondhand-scroll').onScrollToLower() })
    assert.equal(requests.length, 3, '重复触底只请求一次')
    assert.equal(requests[2].query.page, 2)
    await list.event(() => requests[2].resolve(pageResult([product('21')], 2, 21)))
    await list.show()
    assert.equal(list.container.querySelector('.student-secondhand-scroll'), listNode, '返回保留滚动容器')
    assert.equal(list.container.querySelector('.secondhand-card'), itemNode, '刷新过程中保留商品节点')
    await list.event(() => requests[3].reject(new Error('test refresh offline')))
    assert.equal(list.container.querySelectorAll('.secondhand-card').length, 21, '刷新失败保留所有已加载商品')
    await list.event(() => list.props('.student-secondhand-footer__retry').onClick())
    await list.event(() => requests[4].resolve(pageResult(firstTwenty, 1, 21)))
    assert.equal(requests[5].query.page, 2, '返回后重新核对已加载分页而非丢弃下一页')
    await list.event(() => requests[5].resolve(pageResult([product('21')], 2, 21)))
    assert.equal(list.container.querySelector('.secondhand-card'), itemNode)
    list.options.visibility = { ...list.options.visibility, favorites: false }
    await list.render()
    assert.equal(requests.length, 6, '无关功能状态更新不能清空集市')
    await list.show()
    list.options.visibility = { ...list.options.visibility, secondhand: false }
    await list.render()
    await list.event(() => requests[6].resolve(pageResult(firstTwenty, 1, 21)))
    await list.show()
    assert.equal(requests.length, 7, '关闭二手使旧响应失效并停止返回刷新')
    assert.equal(list.container.querySelector('.secondhand-card'), null)

    const viewer = deferred(), favorite = deferred(), contact = deferred()
    let favorites = 0, contacts = 0, previews = []
    const detail = await mount('pages/secondhand-detail/index.tsx', {
      trade: {
        getProduct: async () => product('1', { images: ['test-a', 'test-b', 'test-c'] }),
        getTradeViewerState: () => viewer.promise,
        setProductFavorited: () => { favorites += 1; return favorite.promise },
        contactSeller: () => { contacts += 1; return contact.promise },
      }, taro: { previewImage: async (value) => { previews.push(value) } },
    })
    assert.ok(detail.container.textContent.includes('测试商品说明'), '个人收藏读取慢时仍能查看商品')
    await detail.event(() => detail.props('.secondhand-detail-favorite').onClick())
    assert.equal(favorites, 0, '收藏初始状态未读取时禁止误切换')
    await detail.event(() => viewer.resolve({ favorited: false }))
    await detail.event(() => { detail.props('.secondhand-detail-favorite').onClick(); detail.props('.secondhand-detail-favorite').onClick() })
    assert.equal(favorites, 1, '收藏同帧防重')
    await detail.event(() => favorite.resolve(true))
    assert.ok(detail.container.querySelector('.secondhand-detail-favorite--active'))
    const heroNode = detail.container.querySelector('.secondhand-detail-hero')
    await detail.event(() => detail.props('.secondhand-detail-gallery').onChange({ detail: { current: 2, source: 'touch' } }))
    await detail.event(() => detail.all('.secondhand-detail-hero__image')[0].onError())
    assert.equal(detail.props('.secondhand-detail-gallery').current, 1, '前图失败时仍查看原来第三张')
    await detail.event(() => detail.props('.secondhand-detail-gallery').onChange({ detail: { current: 0, source: '' } }))
    assert.equal(detail.props('.secondhand-detail-gallery').current, 1, '媒体删除后迟到的程序翻页事件不能覆盖当前图片身份')
    await detail.event(() => detail.all('.secondhand-detail-hero__image')[1].onClick())
    assert.deepEqual(previews[0], { urls: ['test-b', 'test-c'], current: 'test-c' }, '原图预览与当前有效图一致')
    await detail.event(() => { detail.all('.secondhand-detail-hero__image').forEach((image) => image.onError()) })
    assert.equal(detail.container.querySelector('.secondhand-detail-hero'), heroNode, '全失败保留媒体容器')
    assert.ok(detail.container.textContent.includes('图片暂不可用'))
    await detail.event(() => detail.props('.secondhand-detail-contact').onClick())
    await detail.event(() => { detail.props('.secondhand-contact-option').onClick(); detail.props('.secondhand-contact-option').onClick() })
    assert.equal(contacts, 1, '联系卖家同帧防重')
    await detail.event(() => contact.resolve({ relationId: 8 }))
    detail.options.visibility = { ...detail.options.visibility, favorites: false, secondhand_chat: false }
    await detail.render()
    assert.equal(detail.container.querySelector('.secondhand-detail-actionbar'), null, '没有可用操作时不保留空底栏')

    const commentRequests = [], submissions = []
    let loadMore
    const comments = await mount('pages/secondhand-detail/product-comments.tsx', { trade: {
      listProductComments: (id, page) => { const task = deferred(); commentRequests.push({ id, page, ...task }); return task.promise },
      createProductComment: (...args) => { const task = deferred(); submissions.push({ args, ...task }); return task.promise },
      replyProductComment: (...args) => { const task = deferred(); submissions.push({ args, ...task }); return task.promise },
    } }, { productId: '1', productStatus: 'selling', onLoadMoreReady: (handler) => { loadMore = handler } })
    const typeComment = (value) => comments.props('.product-comments__input').onInput({ detail: { value } })
    const sendComment = () => comments.props('.product-comments__send').onClick()
    await comments.event(() => typeComment('保留的草稿'))
    await comments.event(sendComment)
    assert.equal(submissions.length, 0, '留言加载与写入互斥')
    await comments.event(() => commentRequests[0].resolve(pageResult([comment('10')], 1, 21)))
    await comments.event(() => { loadMore(); loadMore() })
    assert.equal(commentRequests.length, 2, '留言分页同帧防重')
    await comments.event(sendComment)
    assert.equal(submissions.length, 0, '分页中不得写入后被旧分页覆盖')
    await comments.event(() => commentRequests[1].reject(new Error('test pagination offline')))
    assert.ok(comments.container.textContent.includes('测试留言 10'), '分页失败保留留言')
    await comments.event(() => comments.props('.product-comment__main').onClick())
    await comments.event(() => { sendComment(); sendComment() })
    assert.equal(submissions.length, 1)
    assert.equal(comments.props('.product-comments__input').disabled, true)
    await comments.event(() => { typeComment('不能改动'); comments.props('.product-comments__cancel').onClick() })
    assert.equal(comments.props('.product-comments__input').value, '保留的草稿')
    assert.ok(comments.container.querySelector('.product-comments__replying'), '发送中不能更改回复对象')
    await comments.event(() => submissions[0].reject(new Error('test submit offline')))
    assert.ok(comments.container.querySelector('.product-comments__error'))
    await comments.event(sendComment)
    assert.deepEqual(submissions[1].args, submissions[0].args, '留言失败重试使用相同内容、对象及幂等号')
    await comments.event(() => submissions[1].resolve(comment('11', { parentCommentId: '10', reviewStatus: 'submitted' })))
    assert.ok(comments.container.textContent.includes('审核中'), '使用实际审核状态而非假装立即公开')

    const locatedRequests = [], targets = []
    const located = await mount('pages/secondhand-detail/product-comments.tsx', { trade: {
      listProductComments: (id, page) => { const task = deferred(); locatedRequests.push({ id, page, ...task }); return task.promise },
    } }, { productId: '1', productStatus: 'selling', targetCommentId: '32', onTargetReady: (target) => targets.push(target) })
    await located.event(() => locatedRequests[0].resolve(pageResult([comment('20')], 1, 21)))
    await located.event(() => locatedRequests[1].reject(new Error('test target page offline')))
    assert.ok(located.container.textContent.includes('测试留言 20'), '目标留言后续页失败不清空已取得首屏')
    await located.event(() => located.props('.product-comments__state--retry').onClick())
    await located.event(() => locatedRequests[2].resolve(pageResult([comment('30', { replies: [comment('32', { parentCommentId: '30' })] })], 2, 21)))
    assert.deepEqual(targets, ['product-comment-32'], '加载到目标回复后只定位一次')
    await located.render({ productId: '2', productStatus: 'sold' })
    await located.render({ productId: '3', productStatus: 'sold' })
    await located.event(() => locatedRequests[3].resolve(pageResult([comment('99', { content: '旧商品留言' })])))
    await located.event(() => locatedRequests[4].resolve(pageResult([])))
    assert.ok(!located.container.textContent.includes('旧商品留言'), '商品切换后的旧留言响应作废')
    assert.ok(located.container.textContent.includes('商品已售出，不能继续留言'))
    assert.equal(located.container.querySelector('.product-comments__input'), null)

    const upload = deferred(), approvals = [], publishes = []
    let uploads = 0, redirects = 0
    const publish = await mount('pages/publish-secondhand/index.tsx', {
      trade: { createProduct: (payload) => { const task = deferred(); publishes.push({ payload, ...task }); return task.promise } },
      media: {
        chooseAndUploadImages: () => { uploads += 1; return upload.promise },
        waitForMediaApproval: (id) => { const task = deferred(); approvals.push({ id, ...task }); return task.promise },
      }, taro: { redirectTo: async () => { redirects += 1; if (redirects === 1) throw new Error('test navigation failure') } },
    })
    for (const [value, expected] of [['12.34', 1234], ['0.01', 1], ['0', null], ['1.234', null], ['-1', null], ['1e3', null]]) assert.equal(publish.exports.priceTextToCents(value), expected, '价格分值契约 ' + value)
    await publish.event(() => { publish.props('.publish-secondhand-upload').onClick(); publish.props('.publish-secondhand-upload').onClick() })
    assert.equal(uploads, 1, '图片选择上传同步防重')
    await publish.event(() => upload.resolve([{ assetId: 'asset-1', previewPath: 'test-local-1' }, { assetId: 'asset-2', previewPath: 'test-local-2' }]))
    assert.equal(publish.container.querySelectorAll('.publish-secondhand-media').length, 2, '展示全部已上传附件')
    const inputField = (selector, value) => publish.props(selector).onInput({ detail: { value } })
    await publish.event(() => {
      inputField('.publish-secondhand-field__input', '原商品标题')
      inputField('.publish-secondhand-price__input', '12.34')
      inputField('.publish-secondhand-description__textarea', '原商品描述')
      publish.all('.publish-secondhand-category')[0].onClick()
    })
    const submitProduct = () => publish.props('.publish-secondhand-footer__button').onClick()
    await publish.event(() => { submitProduct(); submitProduct() })
    assert.equal(approvals.length, 2, '双击发布每个附件只审核一次')
    await publish.event(() => {
      inputField('.publish-secondhand-field__input', '不能覆盖')
      inputField('.publish-secondhand-price__input', '888')
      inputField('.publish-secondhand-description__textarea', '不能覆盖')
      publish.all('.publish-secondhand-category')[1].onClick()
      publish.props('.publish-secondhand-upload-copy__remove').onClick()
    })
    assert.equal(publish.props('.publish-secondhand-field__input').disabled, true)
    assert.equal(publish.props('.publish-secondhand-field__input').value, '原商品标题')
    await publish.event(() => approvals.forEach((task) => task.resolve()))
    assert.equal(publishes.length, 1)
    assert.deepEqual(publishes[0].payload, { title: '原商品标题', description: '原商品描述', category: '数码', price: 1234, imageAssetIds: ['asset-1', 'asset-2'], clientRequestId: 'test-request-1' }, '提交快照不受发送中编辑影响')
    await publish.event(() => publishes[0].reject(new Error('test publish offline')))
    assert.ok(publish.container.querySelector('.publish-secondhand-error'))
    await publish.event(submitProduct)
    await publish.event(() => approvals.slice(2).forEach((task) => task.resolve()))
    assert.deepEqual(publishes[1].payload, publishes[0].payload, '失败重试保留原幂等号')
    await publish.event(() => publishes[1].resolve())
    assert.ok(publish.container.textContent.includes('已提交，返回集市'))
    await publish.event(() => { submitProduct(); submitProduct() })
    assert.equal(publishes.length, 2, '发布成功后跳转失败重试不得再创建商品')
    assert.equal(redirects, 2)
    assert.equal(errors.length, 0, 'React 页面没有运行时警告: ' + JSON.stringify(errors))
    console.log('second-hand presentation behavior: PASS (list lifecycle, media, viewer actions, comments, publishing)')
  } finally {
    for (const root of roots) await React.act(async () => root.unmount())
    dom.window.close()
    console.error = originalError
    globals.forEach((key, index) => originals[index] ? Object.defineProperty(globalThis, key, originals[index]) : delete globalThis[key])
  }
}
main().catch((error) => { console.error(error); process.exitCode = 1 })
