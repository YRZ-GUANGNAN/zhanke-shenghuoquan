const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const root = path.resolve(__dirname, '../apps/miniapp/src')

// Isolated request and DOM fixtures; never sign in or contact a server.
function load(relative, dependencies = {}) {
  const filename = path.join(root, relative)
  const mod = new Module(filename, module)
  mod.require = (id) => {
    if (id in dependencies) return dependencies[id]
    if (id === 'react' || id === 'react/jsx-runtime') return require(id)
    if (id.endsWith('.scss')) return {}
    if (id.startsWith('.')) {
      const resolved = path.resolve(path.dirname(filename), id)
      for (const ext of ['.ts', '.tsx']) if (fs.existsSync(resolved + ext)) return load(path.relative(root, resolved + ext), dependencies)
    }
    throw new Error(`Unexpected search test dependency: ${id}`)
  }
  mod._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    fileName: filename,
    compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, filename)
  return mod.exports
}

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

async function run() {
  const calls = []
  let version = 1
  const request = async (url, options) => { calls.push({ url, ...options }); return { items: [], page: 1, pageSize: 20, total: 0, filterVersion: version } }
  const deps = { './api': { request, API_BASE: '' }, './client-request-id': { createClientRequestId: () => 'isolated' }, './resource-url': { resolveApiResourceUrl: (url) => url } }
  const community = load('services/b-community.ts', deps)
  const trade = load('services/c-trade.ts', deps)
  await community.searchCommunityPosts('校园', 1, 20, { sort: 'likes', contentType: 'image', withinDays: 7 })
  assert.deepEqual(calls.at(-1).data.filters, { sort: 'likes', contentType: 'image', withinDays: 7 })
  await trade.searchTradeProducts('教材', 2, 20, { category: '书籍资料', minPrice: 0, maxPrice: 1001 })
  assert.equal(calls.at(-1).data.filters.minPrice, 0)
  assert.equal(calls.at(-1).data.filters.maxPrice, 1001)
  version = undefined
  await assert.rejects(community.searchCommunityPosts('校园', 1, 20, { sort: 'recent' }), /暂不支持筛选/)
  await assert.rejects(trade.searchTradeProducts('教材', 1, 20, { minPrice: 0 }), /暂不支持筛选/)
  await community.searchCommunityPosts('校园')
  assert.deepEqual(calls.at(-1).data, { keyword: '校园', page: 1, pageSize: 20 }, '旧调用保留 payload')
  await trade.searchTradeProducts('教材', 1, 20, {})
  assert.deepEqual(calls.at(-1).data, { keyword: '教材', page: 1, pageSize: 20 })

  const model = load('pages/search/search-filters.ts')
  const defaults = model.defaultSearchFilters()
  assert.deepEqual(model.tradeSearchFilters({ ...defaults, minPrice: '0', maxPrice: '10.01' }), { minPrice: 0, maxPrice: 1001 })
  for (const price of ['-1', '1e3', '0.001', 'Infinity', '999999999999']) {
    assert.throws(() => model.tradeSearchFilters({ ...defaults, minPrice: price }), /价格/)
  }
  assert.throws(() => model.tradeSearchFilters({ ...defaults, minPrice: '10', maxPrice: '0' }), /最低价格/)

  const { JSDOM } = require('jsdom')
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { act } = React
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const originals = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  globalThis.window = dom.window; globalThis.document = dom.window.document; globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const roots = []
  const oldError = console.error
  console.error = () => {}
  const component = (name) => (props) => React.createElement('div', { className: props.className, id: props.id, 'data-component': name, ref: (node) => { if (node) node.taroProps = props } }, props.children)
  const View = component('View')
  const post = { id: 'fixture-post', title: '隔离测试帖子', content: '', author: '测试作者', images: [], tags: [], createdAt: '' }

  async function mount(scope) {
    const requests = [], jumps = [], toasts = []
    const state = { profile: { id: 'fixture-user' }, token: 'isolated-only' }
    const search = (...args) => { const job = deferred(); requests.push({ args, ...job }); return job.promise }
    const taro = { getCurrentInstance: () => ({ router: { params: { scope } } }), showToast: async (value) => toasts.push(value), navigateTo: async (value) => jumps.push(value), navigateBack: async () => {}, navigateToMiniProgram: async () => {} }
    const Page = load('pages/search/index.tsx', {
      '@tarojs/components': Object.fromEntries(['View', 'Text', 'Image', 'Input', 'ScrollView', 'Button'].map((name) => [name, component(name)])),
      '@tarojs/taro': { nextTick: (cb) => queueMicrotask(cb), hideKeyboard: async () => {} },
      '../../components/capsule-page-header': { default: (props) => React.createElement(View, {}, props.left, props.center, props.right) },
      '../../components/feature-page-gate': { withFeaturePage: (page) => page },
      '../../hooks/use-viewport-refresh': { useViewportRefresh: () => ({}) },
      '../../components/app-icon': { default: View },
      '../../utils/capsule-safe': { getCapsuleSafeStyle: () => ({}) },
      '../../stores/session': { useSessionStore: (selector) => selector(state) },
      '../../services/taro': { default: taro },
      '../../services/b-community': { searchCommunityPosts: search, listCommunityHotSearches: async () => [], listCommunitySearchHistory: async () => [], clearCommunitySearchHistory: async () => {} },
      '../../services/c-trade': { searchTradeProducts: search, listTradeHotSearches: async () => [], listTradeSearchHistory: async () => [], clearTradeSearchHistory: async () => {} },
      '../../services/d-navigation': { searchExternalTakeoutMerchants: search, listExternalTakeoutHotSearches: async () => [], listExternalTakeoutSearchHistory: async () => [] },
    }).default
    const container = document.createElement('main'); document.body.appendChild(container)
    const rendered = createRoot(container); roots.push(rendered)
    const render = async () => { await act(async () => rendered.render(React.createElement(Page))) }
    await render()
    const props = (selector) => { const node = container.querySelector(selector); assert.ok(node, selector); return node.taroProps }
    const event = async (fn) => { await act(async () => { fn() }) }
    const click = async (selector) => event(() => props(selector).onClick())
    const input = async (selector, value) => event(() => props(selector).onInput({ detail: { value } }))
    return { requests, jumps, state, render, container, props, event, click, input, toasts }
  }

  try {
    const page = await mount('community')
    await page.input('.student-search-box__input', '校园')
    await page.click('.student-search-submit')
    await page.event(() => page.requests[0].resolve({ items: [post], page: 1, total: 5 }))
    const scroll = page.container.querySelector('.student-search-content')
    await page.click('.student-search-filter-toggle')
    await page.click('#search-filter-content-image')
    assert.equal(page.requests.length, 1, '草稿选择不查询')
    await page.click('.student-search-filter-cancel')
    assert.equal(page.requests.length, 1, '取消不查询')
    assert.equal(page.container.querySelector('.student-search-content'), scroll)
    await page.input('.student-search-box__input', '尚未提交')
    await page.click('.student-search-filter-toggle')
    assert.ok(!page.props('#search-filter-content-image').className.includes('--selected'), '取消丢弃草稿')
    await page.click('#search-filter-content-image')
    await page.click('#search-filter-days-7')
    await page.click('.student-search-filter-apply')
    assert.equal(page.requests[1].args[0], '校园', '筛选使用已提交的词')
    assert.deepEqual(page.requests[1].args[3], { contentType: 'image', withinDays: 7 })
    assert.equal(page.requests[1].args[1], 1)
    await page.click('#search-sort-recent')
    assert.equal(page.requests[2].args[3].sort, 'recent')
    await page.event(() => page.requests[2].resolve({ items: [post], page: 1, total: 4 }))
    await page.event(() => page.requests[1].resolve({ items: [{ ...post, title: '迟到旧条件' }], page: 1, total: 1 }))
    assert.ok(!page.container.textContent.includes('迟到旧条件'))
    await page.event(() => { page.props('.student-search-content').onScrollToLower(); page.props('.student-search-content').onScrollToLower() })
    assert.equal(page.requests.length, 4)
    assert.deepEqual(page.requests[3].args.slice(1), [2, 20, { sort: 'recent', contentType: 'image', withinDays: 7 }])
    await page.event(() => page.requests[3].reject(new Error('isolated offline')))
    assert.ok(page.container.textContent.includes(post.title))
    await page.click('.student-search-state--error')
    assert.deepEqual(page.requests[4].args, page.requests[3].args)
    await page.click('.student-search-filter-toggle')
    await page.click('.student-search-filter-reset')
    assert.equal(page.requests.length, 5)
    await page.click('.student-search-filter-apply')
    assert.deepEqual(page.requests[5].args[3], {})
    await page.event(() => page.requests[5].resolve({ items: [], page: 1, total: 0 }))
    await page.event(() => page.requests[4].resolve({ items: [{ ...post, title: '迟到下一页' }], page: 2, total: 5 }))
    assert.ok(!page.container.textContent.includes('迟到下一页'))
    await page.click('.student-search-filter-toggle')
    await page.click('.student-search-back')
    assert.equal(page.container.querySelector('.student-search-filter-layer'), null, '页内返回先关面板')
    await page.input('.student-search-box__input', '新账号前的查询')
    await page.click('.student-search-submit')
    page.state.token = 'different-isolated-session'; await page.render()
    await page.event(() => page.requests[6].resolve({ items: [post], page: 1, total: 1 }))
    assert.equal(page.container.querySelector('.student-search-result'), null, '换账号不回写旧结果')

    const market = await mount('secondhand')
    await market.click('.student-search-filter-toggle')
    await market.input('.student-search-price-min', '12')
    await market.input('.student-search-price-max', '0')
    await market.click('.student-search-filter-apply')
    assert.equal(market.requests.length, 0)
    assert.ok(market.container.textContent.includes('最低价格不能高于最高价格'))
    await market.input('.student-search-price-min', '0')
    await market.input('.student-search-price-max', '10.01')
    await market.click('#search-filter-category-2')
    await market.click('#search-filter-sort-price_asc')
    await market.click('.student-search-filter-apply')
    await market.input('.student-search-box__input', '教材')
    await market.click('.student-search-submit')
    assert.deepEqual(market.requests[0].args[3], { sort: 'price_asc', category: '书籍资料', minPrice: 0, maxPrice: 1001 })
    await market.event(() => market.requests[0].resolve({ items: [], page: 1, total: 0 }))
    assert.ok(market.container.textContent.includes('清除筛选'))
    const takeout = await mount('takeout')
    assert.equal(takeout.container.querySelector('.student-search-filter-toggle'), null)
    await takeout.input('.student-search-box__input', '商家')
    await takeout.click('.student-search-submit')
    assert.deepEqual(takeout.requests[0].args, ['商家', 1, 20], '外卖调用保持不变')
    console.log('search filters: service compatibility, real-page draft/apply, stale results, paging, prices and account isolation PASS')
  } finally {
    for (const rendered of roots) await act(async () => rendered.unmount())
    console.error = oldError
    dom.window.close()
    for (const [key, original] of originals) { if (original) Object.defineProperty(globalThis, key, original); else delete globalThis[key] }
  }
}
run().catch((error) => { console.error(error); process.exitCode = 1 })
