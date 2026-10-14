const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { JSDOM } = require('jsdom')
const { createRoot } = require('react-dom/client')

const deferred = () => {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

async function run() {
  // Exercise the real API mapping as well as the editor. No network is used.
  const apiFile = path.resolve(__dirname, '../apps/miniapp/src/services/api.ts')
  const apiModule = new Module(apiFile, module)
  apiModule.require = id => {
    if (id === './taro') return { default: {} }
    if (id === './feature-policy-model' || id === './feature-visibility') return {}
    throw new Error(`Unexpected API dependency: ${id}`)
  }
  apiModule._compile(ts.transpileModule(fs.readFileSync(apiFile, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, apiFile)
  const { ApiError } = apiModule.exports
  const apiCalls = []
  let apiValue = { id: 7, nickname: '原昵称', avatar: 'original-avatar', school: '', campus: '' }
  const clientFile = path.resolve(__dirname, '../apps/miniapp/src/services/a-user.ts')
  const client = new Module(clientFile, module)
  client.require = id => { assert.equal(id, './api'); return { request: async (url, options) => { apiCalls.push({ url, options }); return apiValue } } }
  client._compile(ts.transpileModule(fs.readFileSync(clientFile, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, clientFile)
  assert.equal((await client.exports.currentUser()).cover, '', '旧接口缺少封面字段时保持浅色背景')
  apiValue = { ...apiValue, cover: 'formal-cover-url' }
  assert.equal((await client.exports.updateProfileCover('approved-asset-id')).cover, 'formal-cover-url')
  assert.equal(apiCalls[1].url, '/auth/me/cover')
  assert.equal(apiCalls[1].options.method, 'PUT')
  assert.deepEqual(apiCalls[1].options.data, { assetId: 'approved-asset-id' })
  await client.exports.updateProfileCover('')
  assert.deepEqual(apiCalls[2].options.data, { assetId: '' }, '清空必须显式传空ID')
  await client.exports.updateProfile({ ...apiValue, id: '7' })
  assert.equal(Object.hasOwn(apiCalls[3].options.data, 'cover'), false, '原资料保存不覆盖封面')
  const dom = new JSDOM('<!doctype html><html><body><main></main></body></html>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  globalThis.window = dom.window; globalThis.document = dom.window.document; globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const requests = [], uploads = [], reviews = [], updates = [], modals = [], busyEvents = []
  let session = { token: 'unit-test-only', profile: { id: '7', nickname: '原昵称', avatar: 'existing-avatar', cover: 'existing-cover' }, updateProfile: patch => { updates.push(patch); session.profile = { ...session.profile, ...patch } } }
  const store = selector => selector(session)
  store.getState = () => session
  const component = name => props => React.createElement('div', { className: props.className, ref: node => { if (node) node.taroProps = props } }, props.children)
  const source = path.resolve(__dirname, '../apps/miniapp/src/components/profile-cover-editor.tsx')
  const mod = new Module(source, module)
  mod.require = id => {
    if (id === 'react' || id === 'react/jsx-runtime') return require(id)
    if (id === '@tarojs/components') return Object.fromEntries(['Button', 'View', 'Text', 'Image'].map(name => [name, component(name)]))
    if (id.endsWith('.scss')) return {}
    if (id === './app-icon') return { default: component('Icon') }
    if (id.endsWith('/stores/session')) return { useSessionStore: store }
    if (id.endsWith('/services/taro')) return { default: { showModal: () => { const value = deferred(); modals.push(value); return value.promise } } }
    if (id.endsWith('/services/a-user')) return { updateProfileCover: assetId => { const value = { ...deferred(), assetId }; requests.push(value); return value.promise } }
    if (id.endsWith('/services/api')) return apiModule.exports
    if (id.endsWith('/services/media')) return {
      chooseAndUploadImages: (purpose, count, progress, active) => { assert.equal(purpose, 'profile_cover'); assert.equal(count, 1); const value = { ...deferred(), progress, active }; uploads.push(value); return value.promise },
      waitForMediaApproval: (id, progress, timeout, active) => { const value = { ...deferred(), id, active }; reviews.push(value); return value.promise }
    }
    throw new Error(`Unexpected cover dependency: ${id}`)
  }
  mod._compile(ts.transpileModule(fs.readFileSync(source, 'utf8'), { compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, source)
  const Page = mod.exports.default
  const root = createRoot(document.querySelector('main'))
  const render = () => React.act(async () => root.render(React.createElement(Page, { key: `${session.token}:${session.profile.id}`, isProfileBusy: () => false, onBusyChange: value => busyEvents.push(value) })))
  const props = selector => { const el = document.querySelector(selector); assert.ok(el, `Missing ${selector}`); return el.taroProps }
  const event = action => React.act(async () => { action() })
  const choose = () => props('.profile-cover-editor__choose').onClick()
  const save = () => props('.profile-cover-editor__save').onClick()
  const approved = { state: 'approved', canUse: true }
  const uploaded = { assetId: '01J00000000000000000000001', previewPath: 'isolated-local-photo', ...approved }
  try {
    await render()
    await event(() => { choose(); choose() })
    assert.equal(uploads.length, 1, '同帧重复选择只上传一次')
    await event(() => uploads[0].reject({ errMsg: 'chooseMedia:fail cancel' }))
    assert.equal(document.querySelector('.profile-cover-editor__hint--error'), null, '取消选图不是错误')
    assert.equal(props('.profile-cover-editor__image').src, 'existing-cover')

    await event(choose)
    await event(() => uploads[1].resolve([uploaded]))
    assert.equal(props('.profile-cover-editor__image').src, 'existing-cover', '审核前不显示新封面')
    assert.equal(document.querySelector('.profile-cover-editor__save'), null)
    await event(() => reviews[0].reject(new Error('审核不通过')))
    assert.match(document.body.textContent, /审核不通过/)
    assert.equal(session.profile.cover, 'existing-cover')

    await event(choose)
    await event(() => uploads[2].resolve([uploaded]))
    await event(() => reviews[1].resolve(approved))
    assert.equal(props('.profile-cover-editor__image').src, 'isolated-local-photo')
    await event(() => { save(); save() })
    assert.equal(requests.length, 1, '同帧重复保存只发一次请求')
    await event(() => requests[0].reject(new Error('离线，稍后重试')))
    assert.equal(session.profile.cover, 'existing-cover')
    assert.ok(document.querySelector('.profile-cover-editor__save'), '失败保留通过审核的候选图')
    await event(save)
    await event(() => requests[1].resolve({ cover: 'formal-persisted-cover', nickname: '不能覆盖的响应昵称', avatar: '不能覆盖的响应头像' }))
    await render()
    assert.deepEqual(updates[0], { cover: 'formal-persisted-cover' })
    assert.equal(session.profile.nickname, '原昵称')
    assert.equal(session.profile.avatar, 'existing-avatar')
    assert.equal(props('.profile-cover-editor__image').src, 'formal-persisted-cover')

    await event(() => props('.profile-cover-editor__clear').onClick())
    await event(() => modals[0].resolve({ confirm: false }))
    assert.equal(requests.length, 2, '取消恢复默认不修改服务端')
    await event(() => props('.profile-cover-editor__clear').onClick())
    await event(() => modals[1].resolve({ confirm: true }))
    assert.equal(requests[2].assetId, '')
    await event(() => requests[2].resolve({ cover: '' }))
    await render()
    assert.equal(session.profile.cover, '')
    assert.equal(document.querySelector('.profile-cover-editor__image'), null, '清除后仅浅色背景')

    await event(choose)
    await event(() => uploads[3].resolve([uploaded]))
    await event(() => reviews[2].resolve(approved))
    await event(save)
    session = { ...session, token: 'other-unit-session', profile: { ...session.profile, id: '8', cover: '' } }
    await render()
    await event(() => requests[3].resolve({ cover: 'previous-account-cover' }))
    assert.equal(session.profile.cover, '', '旧账号请求不更新新账号')
    assert.equal(updates.length, 2)
    await event(choose)
    await event(() => uploads[4].reject(new ApiError('invalid media upload request', 400)))
    assert.match(document.body.textContent, /服务器未接受封面上传请求/)
    assert.equal(session.profile.cover, '', '服务器拒绝不写入未上传的照片')
    await event(choose)
    await event(() => uploads[5].reject(new ApiError('图片大小无效', 400)))
    assert.match(document.body.textContent, /图片大小无效/)
    assert.ok(!document.body.textContent.includes('服务器未接受封面上传请求'), '其他400不能误报为用途不支持')
    await event(choose)
    await event(() => uploads[6].reject(new ApiError('invalid media upload request', 503)))
    assert.match(document.body.textContent, /invalid media upload request/)
    assert.ok(!document.body.textContent.includes('服务器未接受封面上传请求'), '非400错误不使用旧上传协议提示')
    await event(choose)
    await React.act(async () => root.unmount())
    assert.equal(uploads[7].active(), false, '离开页面后停止上传后续步骤')
    await event(() => uploads[7].resolve([uploaded]))
    assert.equal(reviews.length, 3, '离开页面不再启动审核轮询')
    assert.equal(busyEvents.filter(Boolean).length, 13)
    console.log('profile cover: real component behavior, review gating, retries, clear, click locks and session isolation ok')
  } finally {
    await React.act(async () => root.unmount())
    previous.forEach(([key, value]) => { if (value) Object.defineProperty(globalThis, key, value); else delete globalThis[key] })
    dom.window.close()
  }
}

run().catch(error => { console.error(error); process.exitCode = 1 })
