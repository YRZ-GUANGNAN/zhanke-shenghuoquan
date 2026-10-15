const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

async function main() {
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const names = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT', 'requestAnimationFrame', 'cancelAnimationFrame']
  const before = names.map(name => Object.getOwnPropertyDescriptor(globalThis, name))
  const frames = new Map()
  let frameId = 0
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true,
    requestAnimationFrame: callback => { frames.set(++frameId, callback); return frameId }, cancelAnimationFrame: id => frames.delete(id) })
  const cache = new Map(), roots = new Set(), resize = new Set()
  let geometry = { full: 900, line: 24 }
  let deferGeometry = false, deferViewTicks = false
  const queries = [], viewTicks = []
  const component = ({ children, className, id, style, ...props }) => React.createElement('div', {
    className, id, style, ref: node => { if (node) node.taroProps = props },
  }, children)
  const load = relative => {
    const file = path.resolve(__dirname, '../apps/miniapp/src', relative)
    if (cache.has(file)) return cache.get(file).exports
    const module = new Module(file)
    cache.set(file, module)
    module.require = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return { View: component, Text: component }
      if (id === '@tarojs/taro') return {
        nextTick: callback => deferViewTicks ? viewTicks.push(callback) : queueMicrotask(callback),
        useResize: callback => React.useEffect(() => { resize.add(callback); return () => resize.delete(callback) }, []),
        createSelectorQuery: () => ({ select() { return this }, boundingClientRect() { return this },
          exec(callback) { if (deferGeometry) queries.push(callback); else callback([{ height: geometry.full }, { height: geometry.line }]) } }),
      }
      if (id.endsWith('app-icon')) return { default: component }
      if (id.endsWith('.scss')) return {}
      const base = path.resolve(path.dirname(file), id)
      const selected = ['.ts', '.tsx'].map(ext => base + ext).find(file => fs.existsSync(file))
      if (selected) return load(path.relative(path.resolve(__dirname, '../apps/miniapp/src'), selected))
      throw new Error(`Unknown motion dependency: ${id}`)
    }
    module._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: {
      module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX,
    } }).outputText, file)
    return module.exports
  }
  const { ExpandableText, ReplyReveal, ConfirmedIcon } = load('components/content-motion.tsx')
  const { CommentSession } = load('services/comment-session.ts')
  const frame = async () => React.act(async () => { const callbacks = [...frames.values()]; frames.clear(); callbacks.forEach(callback => callback()) })
  const settle = async ms => React.act(async () => new Promise(resolve => setTimeout(resolve, ms)))
  const flushView = async () => React.act(async () => viewTicks.splice(0).forEach(callback => callback()))
  function fixture() {
    const container = document.createElement('main'); document.body.append(container)
    const root = createRoot(container); roots.add(root)
    return { container,
      render: element => React.act(async () => root.render(element)),
      click: selector => React.act(async () => container.querySelector(selector).taroProps.onClick({ stopPropagation() {} })),
      unmount: async () => { await React.act(async () => root.unmount()); roots.delete(root); container.remove() },
    }
  }
  try {
    const measured = fixture(), layoutEvents = []
    deferGeometry = true
    deferViewTicks = true
    const measuredProps = { text: '需要等待真实测量的长正文'.repeat(100), onLayoutReady: value => layoutEvents.push(value) }
    await measured.render(React.createElement(ExpandableText, measuredProps))
    assert.deepEqual(layoutEvents, [false])
    await flushView()
    await settle(260)
    assert.deepEqual(layoutEvents, [false], '慢测量超过旧定位延时仍不能宣称布局完成')
    assert.equal(measured.container.querySelector('.content-fold--ready'), null, '初次收缩不能播放高度动画')
    await React.act(async () => queries.shift()([{ height: 900 }, { height: 24 }]))
    assert.equal(measured.container.querySelector('.content-fold__viewport').style.height, '144px')
    assert.deepEqual(layoutEvents, [false], 'JS高度更新后仍等原生视图提交')
    await flushView()
    assert.deepEqual(layoutEvents, [false, true])
    assert.ok(measured.container.querySelector('.content-fold--ready'), '初次布局完成后才启用手动收展动画')
    await measured.render(React.createElement(ExpandableText, { ...measuredProps, text: '旧异步正文' }))
    await flushView()
    const staleGeometry = queries.shift()
    await measured.render(React.createElement(ExpandableText, { ...measuredProps, text: '新异步正文' }))
    await flushView()
    await React.act(async () => staleGeometry([{ height: 2000 }, { height: 24 }]))
    assert.equal(measured.container.querySelector('.content-fold__viewport').style.height, '', '旧正文测量不能写回新正文')
    await React.act(async () => queries.shift()([{ height: 48 }, { height: 24 }]))
    await flushView()
    assert.equal(measured.container.querySelector('.content-fold__viewport').style.height, '48px')
    await measured.render(React.createElement(ExpandableText, { ...measuredProps, text: '卸载后迟到' }))
    await flushView()
    const unmountedGeometry = queries.shift()
    await measured.unmount()
    const eventsBeforeUnmountedResult = layoutEvents.length
    await React.act(async () => unmountedGeometry([{ height: 2400 }, { height: 24 }]))
    await flushView()
    assert.equal(layoutEvents.length, eventsBeforeUnmountedResult, '卸载后测量回执不能通知父页定位')
    deferGeometry = false
    deferViewTicks = false

    const text = fixture()
    const body = '完整长评论'.repeat(300)
    await text.render(React.createElement(ExpandableText, { text: body, lines: 6 }))
    assert.equal(text.container.querySelector('.content-fold__viewport').style.height, '144px')
    assert.equal(text.container.querySelector('.content-fold__text').textContent, body, '折叠不截断数据')
    await text.click('.content-fold__toggle')
    assert.equal(text.container.querySelector('.content-fold__viewport').style.height, '900px', '长文展开完整实测高度，无任意上限')
    geometry = { full: 1800, line: 30 }
    await React.act(async () => resize.forEach(callback => callback()))
    assert.equal(text.container.querySelector('.content-fold__viewport').style.height, '1800px', '改宽后仍完整展开')
    await text.click('.content-fold__toggle')
    assert.equal(text.container.querySelector('.content-fold__viewport').style.height, '180px')
    await text.render(React.createElement(ExpandableText, { text: body, lines: 6, disabled: true }))
    await text.click('.content-fold__toggle')
    assert.equal(text.container.querySelector('.content-fold__viewport').style.height, '180px', '退出中的正文不可点击')
    await text.unmount()

    const panel = fixture()
    let actions = 0
    const renderPanel = expanded => panel.render(React.createElement(ReplyReveal, { expanded }, interactive =>
      React.createElement(component, { className: 'action', onClick: () => { if (interactive) actions += 1 } }, '回复')))
    await renderPanel(false)
    assert.equal(panel.container.querySelector('.action'), null)
    await renderPanel(true); await frame(); await frame()
    assert.ok(panel.container.querySelector('.reply-reveal--open'))
    await renderPanel(false)
    await panel.click('.action')
    assert.equal(actions, 0, '退出期间保留绘制但禁止子项操作')
    await renderPanel(true); await frame(); await frame(); await settle(220)
    assert.ok(panel.container.querySelector('.reply-reveal--open'), '快速反向取消旧卸载任务')
    await panel.click('.action'); assert.equal(actions, 1)
    await renderPanel(false); await settle(220)
    assert.equal(panel.container.querySelector('.action'), null, '退出结束不残留隐藏可点内容')
    await renderPanel(true); await panel.unmount(); await frame(); await settle(220)

    const instant = fixture()
    const renderInstant = (expanded, immediate) => instant.render(React.createElement(ReplyReveal, { expanded, immediate }, () => React.createElement(component, { className: 'target' }, '通知回复')))
    await renderInstant(true, true)
    assert.ok(instant.container.querySelector('.reply-reveal--open'), '通知自动展开不等待动画帧')
    assert.ok(instant.container.querySelector('.reply-reveal--immediate'))
    await renderInstant(true, false)
    assert.ok(instant.container.querySelector('.reply-reveal--open'), '定位完成后不得退回尚未完成的enter状态')
    await renderInstant(false, false)
    assert.equal(instant.container.querySelector('.reply-reveal--immediate'), null)
    await renderInstant(true, false); await frame(); await frame()
    assert.ok(instant.container.querySelector('.reply-reveal--open'))
    assert.equal(instant.container.querySelector('.reply-reveal--immediate'), null, '后续手动展开恢复动画')
    await instant.unmount()

    const icon = fixture()
    const renderIcon = (event, owner = 'a') => icon.render(React.createElement(ConfirmedIcon, { name: 'heart', active: true, event, owner }))
    await renderIcon(5)
    assert.equal(icon.container.querySelector('.confirmed-icon--pulse'), null, '已有点赞初次挂载不播放')
    await renderIcon(6)
    assert.ok(icon.container.querySelector('.confirmed-icon--pulse'))
    await renderIcon(6, 'b')
    assert.equal(icon.container.querySelector('.confirmed-icon--pulse'), null, '所有者改变立即清反馈')
    await renderIcon(7, 'b'); await icon.unmount(); await settle(290)

    let resolveLike, rejectLike
    const rootComment = { id: 'c', parentCommentId: '0', reviewStatus: 'approved', liked: false, likedCount: 0 }
    const session = new CommentSession('p', { id: 'u', nickname: '', avatar: '' }, {
      roots: async () => ({ items: [rootComment], page: 1, pageSize: 20, total: 1 }),
      like: () => new Promise((resolve, reject) => { resolveLike = resolve; rejectLike = reject }),
    })
    await session.load()
    assert.deepEqual(session.snapshot().likeFeedback, {})
    const first = session.like('c'), duplicate = session.like('c')
    rejectLike(new Error('offline')); await first; await duplicate
    assert.deepEqual(session.snapshot().likeFeedback, {}, '失败不产生成功反馈事件')
    const success = session.like('c'); resolveLike({ liked: true, likedCount: 1 }); await success
    assert.equal(session.snapshot().likeFeedback.c, 1)
    const stale = session.like('c'); session.dispose(); resolveLike({ liked: false, likedCount: 0 }); await stale
    assert.equal(session.snapshot().likeFeedback.c, 1, '卸载后回执不能触发反馈')

    const style = require('sass').compile(path.resolve(__dirname, '../apps/miniapp/src/components/content-motion.scss')).css
    assert.match(style, /prefers-reduced-motion: reduce/)
    assert.match(style, /animation: none/)
    assert.doesNotMatch(style, /max-height:/, '不以任意 max-height 裁掉长回复')
    console.log('content motion behavior: PASS (measured full text, independent disclosure, reversal, cleanup, confirmed feedback, reduced motion)')
  } finally {
    for (const root of roots) await React.act(async () => root.unmount())
    dom.window.close()
    names.forEach((name, index) => before[index] ? Object.defineProperty(globalThis, name, before[index]) : delete globalThis[name])
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
