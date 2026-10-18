// Real component handlers with isolated services; no account or media writes.
const assert = require('node:assert/strict')
const fs = require('node:fs'), path = require('node:path'), Module = require('node:module'), ts = require('typescript')
const { hookRenderer, nodes, text, hasClass, deferred } = require('./test-delivery-stage-workbench.cjs')
const root = path.resolve(__dirname, '../apps/miniapp/src')
function apiError(status) { return Object.assign(new Error(`isolated failure ${status}`), { status }) }
function application(role = 'merchant', status = 'pending', extra = {}) {
  return { applicationId: '01TESTAPPLICATION0000000001', userId: '1', role, status, rowVersion: 3,
    realName: '林同学', phone: '13800138000', merchantName: '校园小食堂', merchantAddress: '第一食堂东侧 03 号窗口',
    assetIds: ['PRIVATE-PROOF-1'], reviewReason: status === 'rejected' ? '请补充清晰完整的证明图片。' : '',
    submittedAt: '2026-10-18T00:22:00Z', reviewedAt: status === 'pending' ? undefined : '2026-10-18T01:15:00Z', ...extra }
}
function harness(kind = 'page', options = {}) {
  const renderer = hookRenderer(), cache = new Map(), role = options.role || 'merchant'
  let session = { token: 'TOKEN-1', profile: { id: '1' }, status: 'authenticated' }, hidden
  let reads = 0, uploads = 0, writes = 0, closes = 0, navigations = 0, backs = 0, relaunches = 0
  const submitted = [], toasts = [], styles = new Set()
  const sessionStore = Object.assign(select => select(session), { getState: () => session })
  const service = {
    getIdentityApplication: async value => { reads++; return options.read ? options.read(value, reads) : application(role, options.status || 'pending') },
    submitIdentityApplication: async (value, input) => { writes++; submitted.push({ role: value, ...input }); return options.submit ? options.submit(value, input) : application(role, 'pending', input) },
  }
  const media = {
    chooseAndUploadImagesSettled: async (...args) => { uploads++; return options.upload ? options.upload(...args) : [{ status: 'fulfilled', index: 0, value: { assetId: 'NEW-PROOF', previewPath: 'local-preview', purpose: 'identity_application', state: 'ready', canUse: true } }] },
    getMediaStatus: async id => options.mediaStatus ? options.mediaStatus(id) : ({ purpose: 'identity_application', state: 'ready', canUse: true }),
    isMediaUsable: value => value.canUse === true && (value.state === 'ready' || value.state === 'approved'),
  }
  const taro = {
    getCurrentInstance: () => ({ router: { params: { role } } }),
    showToast: async value => { toasts.push(value); return options.toast?.(value) },
    navigateBack: async () => { backs++; return options.back?.() },
    reLaunch: async () => { relaunches++; return options.relaunch?.() },
    previewImage: async () => { throw new Error('No private URL may be resolved in this harness') },
  }
  function load(filename) {
    filename = [filename, `${filename}.tsx`, `${filename}.ts`].find(file => fs.existsSync(file))
    assert.ok(filename, 'Module must exist')
    if (filename.endsWith('.scss')) { styles.add(filename); return {} }
    if (filename.endsWith('.svg')) return `data:image/svg+xml;base64,${fs.readFileSync(filename).toString('base64')}`
    if (cache.has(filename)) return cache.get(filename)
    const loaded = new Module(filename, module)
    loaded.filename = filename
    loaded.require = id => {
      if (id === 'react') return renderer.React
      if (id === 'react/jsx-runtime') return { jsx: (type, props) => renderer.React.createElement(type, props, props.children), jsxs: (type, props) => renderer.React.createElement(type, props, props.children), Fragment: 'Fragment' }
      if (id === '@tarojs/components') return Object.fromEntries(['View', 'Text', 'Button', 'ScrollView', 'Image', 'Input', 'Textarea'].map(name => [name, name]))
      if (id === '@tarojs/taro') return { useDidHide: fn => { hidden = fn } }
      if (id.endsWith('/services/a-user')) return service
      if (id.endsWith('/services/api')) return { isApiError: value => typeof value?.status === 'number' }
      if (id.endsWith('/services/media')) return media
      if (id.endsWith('/services/taro')) return taro
      if (id.endsWith('/stores/session')) return { useSessionStore: sessionStore }
      if (id.endsWith('/components/runtime-session-gate')) return ({ children }) => children
      if (id.endsWith('/components/feature-page-gate')) return { withFeaturePage: page => page }
      if (id.endsWith('/components/capsule-page-header')) {
        if (options.visual) return props => renderer.React.createElement('View', { className: 'capsule-page-header' }, renderer.React.createElement('View', { className: 'preview-system' }, '9:41'), renderer.React.createElement('View', { className: 'preview-header' }, props.left, renderer.React.createElement('Text', {}, props.title)))
        return 'CapsulePageHeader'
      }
      if (id.endsWith('/hooks/use-layer-presence')) return { useLayerPresence: visible => ({ mounted: visible, phase: 'open' }) }
      if (id === './ui' || id.endsWith('/components/ui')) {
        if (!options.visual) return Object.fromEntries(['AppButton','AppField','AppState','AppNotice','AppSkeleton','AppIconButton','AppDialog'].map(name => [name, name]))
        return Object.assign({}, ...['app-button','app-field','app-state','app-notice','app-skeleton','app-icon-button','app-dialog'].map(name => load(path.join(root, 'components/ui', name))))
      }
      if (id.startsWith('.')) return load(path.resolve(path.dirname(filename), id))
      throw new Error(`Unisolated dependency: ${id}`)
    }
    const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), { fileName: filename,
      compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020, esModuleInterop: true } }).outputText
    loaded._compile(compiled, filename); cache.set(filename, loaded.exports); return loaded.exports
  }
  const component = load(path.join(root, kind === 'entry' ? 'components/IdentityApplicationEntryModal.tsx' : 'pages/identity-application/index.tsx')).default
  renderer.setComponent(() => kind === 'entry' ? component({ role, onCancel: () => { closes++ }, onContinue: async value => { navigations++; await options.navigate?.(value) } }) : component())
  const one = (type, className) => nodes(renderer.tree, node => node.type === type && (!className || hasClass(node, className)))[0]
  return {
    renderer, styles, one, submitted, toasts,
    get tree() { return renderer.tree }, get counters() { return { reads, uploads, writes, closes, navigations, backs, relaunches } },
    async mount() { renderer.render(); await renderer.flush() }, async flush() { await renderer.flush() },
    async hide() { hidden?.(); await renderer.flush() }, async unmount() { renderer.unmount(); await renderer.flush() },
    setAccount(id) { session = { ...session, token: `TOKEN-${id}`, profile: { id } }; renderer.render() },
    async field(label, value) { const field = nodes(renderer.tree, node => node.type === 'AppField' && node.props.label === label)[0]; assert.ok(field, label); field.props.onInput(value); await renderer.flush() },
    button(name) { return one('AppButton', name) }, dialog() { return one('AppDialog')?.props },
  }
}
async function run() {
  for (const role of ['merchant', 'rider']) {
    for (const status of ['missing','pending','approved','rejected']) {
      const app = harness('entry', { role, read: async () => { if (status === 'missing') throw apiError(404); return application(role, status) } })
      await app.mount()
      assert.equal(app.dialog().confirmText, status === 'missing' ? '开始申请' : status === 'rejected' ? '修改申请资料' : '查看申请详情')
      assert.equal(app.dialog().loading, false)
      app.dialog().onConfirm(); app.dialog().onConfirm(); await app.flush()
      assert.equal(app.counters.navigations, 1); assert.equal(app.counters.writes, 0); assert.equal(app.counters.closes, 1)
    }
  }
  console.log('PASS 两角色四状态入口、单次导航、只读查询')
  {
    const app = harness('entry', { read: (_, count) => count === 1 ? Promise.reject(apiError(0)) : Promise.resolve(application()) })
    await app.mount(); assert.ok(app.dialog().error)
    app.dialog().onConfirm(); app.dialog().onConfirm(); await app.flush()
    assert.equal(app.counters.reads, 2); assert.equal(app.dialog().error, '')
    const nav = harness('entry', { navigate: async () => { throw new Error('navigation failed') } })
    await nav.mount(); nav.dialog().onConfirm(); await nav.flush()
    assert.match(nav.dialog().error, /页面打开失败/); assert.equal(nav.dialog().loading, false)
    nav.dialog().onConfirm(); await nav.flush(); assert.equal(nav.counters.navigations, 2)
  }
  console.log('PASS 入口读取失败与导航失败可重试')
  for (const action of ['hide','unmount','account']) {
    const pending = deferred(), app = harness('entry', { read: () => pending.promise })
    await app.mount()
    if (action === 'account') app.setAccount('2'); else await app[action]()
    pending.resolve(application('merchant','rejected')); await app.flush()
    assert.ok(!text(app.tree).includes('请补充清晰完整')); assert.equal(app.counters.navigations, 0)
  }
  console.log('PASS 隐藏、卸载、账号变化后的旧摘要失效')
  for (const role of ['merchant','rider']) {
    for (const status of ['pending','approved']) {
      const app = harness('page', { role, status }); await app.mount()
      assert.equal(nodes(app.tree, node => node.type === 'AppField').length, 0)
      assert.equal(app.button('identity-application-submit'), undefined)
      assert.ok(text(app.tree).includes('私密证明 1'))
      assert.ok(!nodes(app.tree, node => node.type === 'Image').some(node => node.props.src === 'PRIVATE-PROOF-1'))
      assert.ok(text(app.tree).includes('2026-10-18 08:22'))
    }
  }
  console.log('PASS 已提交申请只读、历史证明不暴露 URL、北京时间')
  for (const role of ['merchant','rider']) {
    const pending = deferred(), app = harness('page', { role, status: 'rejected', submit: () => pending.promise })
    await app.mount()
    const submit = app.button('identity-application-submit').props.onClick
    submit(); submit(); await app.flush()
    assert.equal(app.counters.writes, 1); assert.equal(app.submitted[0].expectedRowVersion, 3)
    assert.deepEqual(app.submitted[0].assetIds, ['PRIVATE-PROOF-1'])
    assert.ok(nodes(app.tree, node => node.type === 'AppField').every(node => node.props.disabled))
    if (role === 'rider') assert.equal(app.submitted[0].merchantName, '')
    pending.resolve(application(role,'pending')); await app.flush()
    assert.equal(app.button('identity-application-submit'), undefined)
  }
  console.log('PASS 拒绝重提版本、防重复 PUT、提交时锁定表单、人工核验材料合同')
  {
    const app = harness('page', { read: async () => { throw apiError(404) } }); await app.mount()
    app.button('identity-application-submit').props.onClick(); await app.flush()
    assert.equal(app.counters.writes, 0); assert.ok(nodes(app.tree, node => node.type === 'AppField').some(node => node.props.error))
    await app.field('商家名称','新商家'); await app.field('负责人姓名','林同学'); await app.field('联系电话','13800138000'); await app.field('经营地址','第一食堂')
    app.one('Button','identity-application-add').props.onClick(); await app.flush()
    app.button('identity-application-submit').props.onClick(); await app.flush()
    assert.equal(app.counters.writes, 1); assert.equal(app.submitted[0].expectedRowVersion, 0)
  }
  console.log('PASS 首次申请字段校验、上传与提交闭环')
  {
    const app = harness('page', { status:'rejected', submit: async () => { throw apiError(0) } }); await app.mount()
    await app.field('商家名称','保留修改的商家名称')
    app.button('identity-application-submit').props.onClick(); await app.flush()
    assert.ok(nodes(app.tree, node => node.type === 'AppField' && node.props.value === '保留修改的商家名称').length)
    assert.equal(app.button('identity-application-submit').props.disabled, false)
    const conflict = harness('page', { read: (_, n) => Promise.resolve(application('merchant',n === 1 ? 'rejected' : 'pending')), submit: async () => { throw apiError(409) } })
    await conflict.mount(); conflict.button('identity-application-submit').props.onClick(); await conflict.flush()
    assert.equal(conflict.counters.reads, 2); assert.equal(conflict.button('identity-application-submit'),undefined)
    const stale = harness('page', { read: (_, n) => n === 1 ? Promise.resolve(application()) : Promise.reject(apiError(0)) })
    await stale.mount(); stale.button('identity-application-refresh').props.onClick(); await stale.flush()
    assert.ok(text(stale.tree).includes('校园小食堂')); assert.ok(nodes(stale.tree, n => n.type === 'AppNotice' && n.props.title === '暂时无法更新申请状态').length)
  }
  console.log('PASS 网络失败保留输入、409 同步真实状态、刷新失败保留详情')
  for (const action of ['back', 'account', 'unmount']) {
    const media = deferred(), app = harness('page', { status:'rejected', mediaStatus: () => media.promise })
    await app.mount(); app.button('identity-application-submit').props.onClick(); await app.flush()
    if (action === 'back') { app.button('identity-application-secondary').props.onClick(); await app.flush() }
    if (action === 'account') app.setAccount('2')
    if (action === 'unmount') await app.unmount()
    media.resolve({ purpose:'identity_application',state:'ready',canUse:true }); await app.flush()
    assert.equal(app.counters.writes,0)
  }
  console.log('PASS 媒体等待期间取消、换号、卸载阻止迟到提交')
  {
    const app = harness('page', { status:'rejected', back:async()=>{throw new Error('back failed')}, relaunch:async()=>{throw new Error('relaunch failed')} })
    await app.mount(); app.button('identity-application-secondary').props.onClick(); await app.flush()
    assert.equal(app.counters.backs,1); assert.equal(app.counters.relaunches,1); assert.equal(app.counters.reads,2)
    assert.equal(app.button('identity-application-submit').props.disabled,false)
    const wrong = harness('entry', { read:async()=>application('rider') }); await wrong.mount()
    assert.ok(wrong.dialog().error); assert.ok(!text(wrong.tree).includes('林同学'))
  }
  console.log('PASS 返回失败恢复交互、错误角色资料拒绝展示')
}
module.exports = { harness, application, apiError, nodes, text, hasClass, deferred }
if (require.main === module) run().catch(error => { console.error(error); process.exitCode = 1 })
