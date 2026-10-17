const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path'), Module = require('node:module')
const ts = require('typescript'), React = require('react'), { createRoot } = require('react-dom/client'), { JSDOM } = require('jsdom')
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
  let settings = { homeTitle: '湛科生活圈', secondhandTitle: '校园二手', servicesTitle: '校园服务', defaultImageUrl: 'https://zhanduiwang.top/existing.png', detailTitleSuffix: '湛科生活圈', preferContentImage: true, rowVersion: 7, updatedAt: '2026-10-17T00:00:00Z' }
  const writes = []
  const service = compile('apps/admin/src/services/share-settings.ts', { './http': { adminRequest: async () => {} } })
  service.shareSettingsAPI.read = async () => ({ ...settings })
  service.shareSettingsAPI.save = async (value, expected) => {
    writes.push({ value, expected })
    settings = { ...value, rowVersion: settings.rowVersion + 1, updatedAt: settings.updatedAt }
    return { ...settings }
  }
  const box = p => React.createElement('div', null, p.children, p.actions)
  const components = {
    AdminPanel: box, AdminNotice: box, AdminState: p => React.createElement('div', null, p.title),
    AdminButton: p => React.createElement('button', { onClick: p.onClick, disabled: p.disabled || p.loading }, p.children),
    AdminField: p => React.createElement('input', { value: p.value, disabled: p.disabled, 'aria-label': p.label, onChange: p.onChange, onInput: p.onChange }),
    useAdminDialog: () => ({ confirm: async () => true })
  }
  const { ShareSettingsConsole } = compile('apps/admin/src/ShareSettingsConsole.tsx', {
    './components': components, './services/share-settings': service,
    './services/http': { HTTPError: class extends Error {} },
    './store': { useAdminStore: select => select({ user: { id: 123 } }) }
  })
  const app = createRoot(document.querySelector('main'))
  try {
    await React.act(async () => app.render(React.createElement(ShareSettingsConsole)))
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), settings.defaultImageUrl)
    const button = label => [...document.querySelectorAll('button')].find(element => element.textContent === label)
    const select = mode => React.act(async () => document.querySelector(`input[name=share-cover-source][value=${mode}]`).click())
    const coverInput = () => document.querySelector('[aria-label=通用封面地址]')
    const input = value => React.act(async () => {
      Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value').set.call(coverInput(), value)
      coverInput().dispatchEvent(new dom.window.Event('input', { bubbles: true }))
    })
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
    assert.ok(save && !save.disabled)
    await React.act(async () => save.click())
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
    await select('builtin')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')
    await React.act(async () => save.click())
    assert.equal(writes.length, 2)
    assert.equal(writes[1].value.defaultImageUrl, '', 'builtin uses existing backend empty-field contract')
    assert.equal(writes[1].expected, 8, 'subsequent save uses returned version')
    await select('custom')
    await input(replacement)
    await React.act(async () => button('重新读取').click())
    assert.equal(document.querySelector('input[name=share-cover-source]:checked').value, 'builtin', 'reload restores the persisted cover source')
    assert.equal(document.querySelector('.share-card-preview img').getAttribute('src'), '/images/default-share.png')
    const { navigationItemError } = compile('apps/admin/src/services/navigation-validation.ts')
    const tool = { categoryId: 1, title: '测试入口', subtitle: '', iconText: '', appId: 'wx1234567890ABCDEF', path: 'pages/home/index', color: '', sort: 0, enabled: true }
    assert.equal(navigationItemError(tool), '')
    for (const path of ['https://example.com', '/pages/index', '../pages/index', 'pages/%zz', 'pages/%2E%2E/index']) assert.ok(navigationItemError({ ...tool, path }))
    assert.ok(navigationItemError({ ...tool, appId: 'wx0000000000000000' }))
    assert.ok(navigationItemError({ ...tool, sort: NaN }))
    console.log('admin share default: PASS (custom edit, source switching, draft preservation, validation, preview, save payload, CAS, reload; tool target validation)')
  } finally {
    await React.act(async () => app.unmount())
    dom.window.close()
    saved.forEach(([key, descriptor]) => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key])
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
