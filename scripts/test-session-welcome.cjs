const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

const root = path.resolve(__dirname, '..', 'apps/miniapp/src')
let session, retries = 0
const emptyProfile = { id: '', nickname: '未登录', avatar: '' }
const retry = () => { retries += 1 }
const cache = new Map()
const component = tag => ({ children, className, onClick, onError, src, disabled, role, ...props }) => React.createElement(tag, {
  className, onClick, onError, src, disabled, role, 'aria-label': props['aria-label']
}, children)
function load(relative) {
  const file = path.resolve(root, relative)
  if (cache.has(file)) return cache.get(file).exports
  const loaded = new Module(file, module)
  cache.set(file, loaded)
  loaded.require = id => {
    if (id === 'react' || id === 'react/jsx-runtime') return require(id)
    if (id === '@tarojs/components') return {
      View: component('div'), Text: component('span'), Image: component('img'), Button: component('button')
    }
    if (id.endsWith('.scss')) return {}
    if (id.endsWith('.svg')) return { default: '/test-icon.svg' }
    if (id.endsWith('stores/session')) return { useSessionStore: selector => selector(session) }
    if (id.endsWith('services/api')) return { API_BASE: 'https://test.invalid/api' }
    if (id.endsWith('services/resource-url')) return { resolveApiResourceUrl: value => value }
    if (id === './ui') return load('components/ui/app-state.tsx')
    const candidate = path.resolve(path.dirname(file), id)
    for (const suffix of ['.tsx', '.ts']) if (fs.existsSync(candidate + suffix)) return load(candidate + suffix)
    throw new Error('Unexpected dependency ' + id)
  }
  loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    fileName: file, compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText, file)
  return loaded.exports
}

async function run() {
  const dom = new JSDOM('<main></main>')
  const keys = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT']
  const saved = keys.map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)])
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  const app = createRoot(document.querySelector('main'))
  const Gate = load('components/runtime-session-gate.tsx').default
  const render = (status, profile = emptyProfile, error = '', preserve = false) => {
    session = { status, profile, error, retry }
    return React.act(async () => app.render(React.createElement(Gate, { preserveAuthenticatedChildrenDuringRetry: preserve },
      React.createElement('input', { id: 'existing-content', defaultValue: '原页面' }))))
  }
  try {
    await render('idle')
    assert.equal(document.querySelector('.app-state__title').textContent, '你好')
    assert.ok(!document.body.textContent.includes('未登录'))
    assert.equal(document.querySelector('[role=status]').getAttribute('aria-label'), '正在登录')
    assert.equal(document.querySelector('#existing-content'), null)
    const known = { id: 'preview-user', nickname: '这是需要完整展示的长昵称同学', avatar: 'https://test.invalid/avatar.png' }
    await render('booting', known)
    assert.ok(document.querySelector('.app-state__title').textContent.includes(known.nickname))
    const avatar = document.querySelector('.chat-avatar__image')
    assert.equal(avatar.getAttribute('src'), known.avatar)
    await React.act(async () => avatar.dispatchEvent(new dom.window.Event('error')))
    assert.equal(document.querySelector('.chat-avatar__image'), null)
    assert.ok(document.querySelector('.chat-avatar__initial').textContent)
    await render('authenticated', known)
    assert.ok(document.querySelector('#existing-content'), 'successful login must not wait for the animation')
    assert.equal(document.querySelector('.app-state--welcome'), null)
    const existing = document.querySelector('#existing-content')
    existing.value = '未保存输入'
    await render('booting', known, '', true)
    assert.equal(document.querySelector('#existing-content'), existing, 'retry must preserve existing content when requested')
    assert.equal(existing.value, '未保存输入')
    await render('error', known, '网络连接失败')
    assert.ok(document.querySelector('.app-state--offline'))
    assert.equal(document.querySelector('.chat-avatar'), null, 'failed login must not pretend to remain welcomed')
    await React.act(async () => document.querySelector('button').click())
    assert.equal(retries, 1)
    await render('anonymous')
    assert.equal(document.querySelector('#existing-content'), null)
    await render('booting')
    assert.equal(document.querySelector('.app-state__title').textContent, '你好', 'cleared account must not retain the previous greeting')
    assert.equal(document.querySelector('.chat-avatar'), null)
    await render('authenticated')
    assert.ok(document.querySelector('#existing-content'))
    console.log('session welcome: PASS (cold start, long name, avatar fallback, immediate success, retry preservation, error retry, account reset)')
  } finally {
    await React.act(async () => app.unmount())
    dom.window.close()
    saved.forEach(([key, descriptor]) => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key])
  }
}
run().catch(error => { console.error(error); process.exitCode = 1 })
