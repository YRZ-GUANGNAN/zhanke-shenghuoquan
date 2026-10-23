const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path'), Module = require('node:module')
const ts = require('typescript'), React = require('react'), { JSDOM } = require('jsdom')
const root = path.resolve(__dirname, '..')
function compile(relative, dependencies = {}) {
  const file = path.join(root, relative), loaded = new Module(file, module)
  loaded.require = id => {
    if (id.endsWith('.css')) return {}
    if (id in dependencies) return dependencies[id]
    if (id === 'react' || id === 'react/jsx-runtime') return require(id)
    throw Error('Unexpected admin test dependency ' + id)
  }
  loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), { fileName: file, compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, file)
  return loaded.exports
}
async function run() {
  const dom = new JSDOM('<main></main>')
  const saved = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const { createRoot } = require('react-dom/client')
  const deferred = () => {
    let resolve, reject
    const promise = new Promise((yes, no) => { resolve = yes; reject = no })
    return { promise, resolve, reject }
  }
  let settings = { homeTitle: '湛科生活圈', secondhandTitle: '校园二手', servicesTitle: '校园服务', defaultImageUrl: 'https://zhanduiwang.top/existing.png', detailTitleSuffix: '湛科生活圈', preferContentImage: true, rowVersion: 7, updatedAt: '2026-10-17T00:00:00Z' }
  const writes = []
  let owner = 123, reads = 0, confirmations = 0, confirmAnswer = true
  const service = compile('apps/admin/src/services/share-settings.ts', { './http': { adminRequest: async () => {} } })
  let read = async () => ({ ...settings })
  const persist = async value => {
    settings = { ...value, rowVersion: settings.rowVersion + 1, updatedAt: settings.updatedAt }
    return { ...settings }
  }
  let write = persist
  service.shareSettingsAPI.read = () => { reads++; return read() }
  service.shareSettingsAPI.save = async (value, expected) => {
    writes.push({ value, expected })
    return write(value, expected)
  }
  const { AdminButton } = compile('apps/admin/src/components/AdminButton.tsx')
  const components = {
    ...compile('apps/admin/src/components/AdminPanel.tsx'),
    ...compile('apps/admin/src/components/AdminNotice.tsx'),
    ...compile('apps/admin/src/components/AdminState.tsx', { './AdminButton': { AdminButton } }),
    ...compile('apps/admin/src/components/AdminField.tsx'), AdminButton,
    useAdminDialog: () => ({ confirm: async () => { confirmations++; return confirmAnswer } })
  }
  class HTTPError extends Error { constructor(status, message) { super(message); this.status = status } }
  const { ShareSettingsConsole } = compile('apps/admin/src/ShareSettingsConsole.tsx', {
    './components': components, './services/share-settings': service,
    './services/http': { HTTPError },
    './store': { useAdminStore: select => select({ user: { id: owner } }) }
  })
  const app = createRoot(document.querySelector('main'))
  try {
    const render = (element = React.createElement(ShareSettingsConsole)) => React.act(async () => app.render(element))
    await render()
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), settings.defaultImageUrl)
    const button = label => [...document.querySelectorAll('button')].find(element => element.textContent === label)
    const select = mode => React.act(async () => document.querySelector(`input[name=share-cover-source][value=${mode}]`).click())
    const coverInput = () => document.getElementById('admin-field-通用封面地址')
    const input = value => React.act(async () => {
      Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value').set.call(coverInput(), value)
      coverInput().dispatchEvent(new dom.window.Event('input', { bubbles: true }))
    })
    const imageEvent = (type, element = document.querySelector('.share-card-preview img')) => React.act(async () => element.dispatchEvent(new dom.window.Event(type)))
    const replacement = 'https://zhanduiwang.top/new-cover.jpg'
    await input(replacement)
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), replacement)
    await select('builtin')
    assert.ok(coverInput().disabled)
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')
    await select('custom')
    assert.equal(coverInput().value, replacement, 'switching sources preserves the custom draft')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), replacement)
    const save = button('保存设置')
    assert.ok(save.disabled, 'wait for the selected custom image to load before saving')
    await imageEvent('load')
    assert.ok(save && !save.disabled)
    await React.act(async () => { save.click(); save.click() })
    assert.equal(writes.length, 1)
    assert.equal(writes[0].expected, 7, 'replacement keeps optimistic concurrency version')
    assert.equal(writes[0].value.defaultImageUrl, replacement, 'replacement reaches the actual save API')
    assert.equal(writes[0].value.preferContentImage, true)
    assert.equal(writes[0].value.homeTitle, settings.homeTitle)
    await input('')
    assert.ok(save.disabled, 'empty custom selection must not silently save as builtin')
    await React.act(async () => save.click())
    assert.equal(writes.length, 1)
    await input('https://example.com/invalid.jpg')
    assert.ok(save.disabled, 'switcher preserves the existing download-domain validation')
    await input('https://zhanduiwang.top/broken.jpg')
    await imageEvent('error')
    assert.ok(save.disabled, 'failed images cannot be saved with a misleading builtin preview')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')
    await React.act(async () => button('重试预览').click())
    assert.ok(save.disabled, 'retry remains pending until the image loads')
    await imageEvent('load')
    assert.ok(!save.disabled)
    await select('builtin')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')
    await React.act(async () => save.click())
    assert.equal(writes.length, 2)
    assert.equal(writes[1].value.defaultImageUrl, '', 'builtin uses existing backend empty-field contract')
    assert.equal(writes[1].expected, 8, 'subsequent save uses returned version')
    await select('custom')
    const beforeRemount = reads
    const unload = new dom.window.Event('beforeunload', { cancelable: true })
    assert.equal(window.dispatchEvent(unload), false, 'unfinished source selection warns on browser exit')
    await render(null); await render()
    assert.equal(document.querySelector('input[name=share-cover-source]:checked').value, 'custom', 'empty custom selection survives switching sections')
    assert.equal(reads, beforeRemount, 'unfinished drafts are not replaced by a new read')
    await input(replacement)
    await select('builtin')
    await render(null); await render()
    await select('custom')
    assert.equal(coverInput().value, replacement, 'inactive custom draft survives switching sections even when payload matches saved builtin')
    confirmAnswer = false
    const beforeConfirm = confirmations
    await React.act(async () => { button('重新读取').click(); button('重新读取').click() })
    assert.equal(confirmations, beforeConfirm + 1, 'same-task reload does not open two confirmations')
    assert.equal(coverInput().value, replacement, 'cancelled reload preserves all drafts')
    confirmAnswer = true
    await React.act(async () => button('重新读取').click())
    assert.equal(document.querySelector('input[name=share-cover-source]:checked').value, 'builtin', 'reload restores the persisted cover source')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')

    await select('custom')
    await input('https://zhanduiwang.top/slow-old.jpg')
    const oldImage = document.querySelector('.share-card-preview img')
    await input(replacement)
    await imageEvent('error', oldImage)
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), replacement, 'old image errors cannot replace the new preview')
    await imageEvent('load')
    const pendingSave = deferred()
    write = () => pendingSave.promise
    const writesBeforeSlowSave = writes.length
    await React.act(async () => { button('保存设置').click(); button('保存设置').click() })
    assert.equal(writes.length, writesBeforeSlowSave + 1, 'slow save is synchronously deduplicated')
    await render(null); await render()
    const newerDraft = 'https://zhanduiwang.top/newer-draft.png'
    await input(newerDraft)
    await React.act(async () => pendingSave.resolve(await persist(writes.at(-1).value)))
    assert.equal(coverInput().value, newerDraft, 'late response cannot replace edits after leaving and returning')
    await render(null); await render()
    assert.equal(coverInput().value, newerDraft, 'late response cannot overwrite retained draft either')
    write = async () => { throw new HTTPError(409, 'version conflict') }
    await imageEvent('load')
    await React.act(async () => button('保存设置').click())
    assert.equal(coverInput().value, newerDraft, 'version conflicts keep the local draft')
    assert.match(document.body.textContent, /其他管理员已更新配置/)
    write = persist
    await React.act(async () => button('重新读取').click())
    assert.equal(coverInput().value, replacement)

    const oldRead = deferred()
    owner = 456
    read = () => oldRead.promise
    await render()
    assert.equal(coverInput(), null, 'changing administrators must not expose the previous draft')
    owner = 789
    settings = { ...settings, homeTitle: '当前管理员配置', defaultImageUrl: '' }
    read = async () => ({ ...settings })
    await render()
    await React.act(async () => oldRead.resolve({ ...settings, homeTitle: '迟到的旧配置', defaultImageUrl: replacement }))
    assert.equal(document.getElementById('admin-field-首页分享标题').value, '当前管理员配置')
    assert.equal(coverInput().value, '')

    owner = 987
    const pendingReads = []
    read = () => { const pending = deferred(); pendingReads.push(pending); return pending.promise }
    await render(React.createElement(React.StrictMode, null, React.createElement(ShareSettingsConsole)))
    assert.equal(pendingReads.length, 2, 'exercise StrictMode effect cleanup and reinitialization')
    await React.act(async () => pendingReads[1].resolve({ ...settings, homeTitle: '新读取配置' }))
    await React.act(async () => pendingReads[0].resolve({ ...settings, homeTitle: '旧读取配置' }))
    assert.equal(document.getElementById('admin-field-首页分享标题').value, '新读取配置', 'late initial reads cannot overwrite newer reads')
    const { navigationItemError } = compile('apps/admin/src/services/navigation-validation.ts')
    const tool = { categoryId: 1, title: '测试入口', subtitle: '', iconText: '', appId: 'wx1234567890ABCDEF', path: 'pages/home/index', color: '', sort: 0, enabled: true }
    assert.equal(navigationItemError(tool), '')
    for (const path of ['https://example.com', '/pages/index', '../pages/index', 'pages/%zz', 'pages/%2E%2E/index']) assert.ok(navigationItemError({ ...tool, path }))
    assert.ok(navigationItemError({ ...tool, appId: 'wx0000000000000000' }))
    assert.ok(navigationItemError({ ...tool, sort: NaN }))
    console.log('admin share default: PASS (real fields/buttons, drafts, preview failure/retry, stale events, save deduplication, CAS, cancelled reload, account isolation, stale requests, StrictMode)')
  } finally {
    await React.act(async () => app.unmount())
    dom.window.close()
    saved.forEach(([key, descriptor]) => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key])
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
