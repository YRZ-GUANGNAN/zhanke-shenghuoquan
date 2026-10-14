const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const sass = require('sass')
const postcss = require('postcss')
const pxtransform = require('postcss-pxtransform')

function pixels(value, viewport) {
  const unit = value.trim().match(/^([\d.]+)(rpx|px)$/i)
  if (unit) return Number(unit[1]) * (unit[2].toLowerCase() === 'rpx' ? viewport / 750 : 1)
  const maximum = value.trim().match(/^max\((.+)\)$/)
  if (maximum) return Math.max(...maximum[1].split(',').map(part => pixels(part, viewport)))
  throw new Error(`无法验证导航尺寸: ${value}`)
}

function deferred() {
  let resolve
  let reject
  const promise = new Promise((accept, decline) => { resolve = accept; reject = decline })
  return { promise, resolve, reject }
}

async function run() {
  const { JSDOM } = require('jsdom')
  const React = require('react')
  const { createRoot } = require('react-dom/client')
  const { act } = React
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const previous = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'].map((name) => Object.getOwnPropertyDescriptor(globalThis, name))
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.IS_REACT_ACT_ENVIRONMENT = true
  const roots = []
  const component = (name) => (props) => React.createElement('div', {
    className: props.className,
    'data-component': name,
    ref: (node) => { if (node) node.taroProps = props },
  }, props.children)
  const View = component('View')
  const Text = component('Text')
  const visibility = {
    takeout: true,
    campus_tools: true,
    secondhand: true,
    publish: true,
    post: true,
    secondhand_publish: true,
  }
  const shows = new Set()
  const navigations = []
  const options = { visibility: { ...visibility } }
  const taro = {
    redirectTo: ({ url }) => {
      const task = deferred()
      navigations.push({ ...task, kind: 'redirect', url })
      return task.promise
    },
    navigateTo: ({ url }) => {
      const task = deferred()
      navigations.push({ ...task, kind: 'navigate', url })
      return task.promise
    },
  }
  const loadSource = (sourcePath) => {
    const loaded = new Module(sourcePath, module)
    loaded.require = (id) => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return { View, Text }
      if (id === '@tarojs/taro') return {
        useDidShow: (callback) => {
          const ref = React.useRef(callback)
          ref.current = callback
          React.useEffect(() => { shows.add(ref); return () => shows.delete(ref) }, [])
        },
      }
      if (id.endsWith('.scss')) return {}
      if (id.endsWith('/use-feature-visibility')) return { useFeatureVisibility: () => options.visibility }
      if (id.endsWith('/app-icon')) return { default: View }
      if (id.endsWith('/services/taro')) return { default: taro }
      throw new Error(`Unexpected navigation test dependency: ${id}`)
    }
    loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
      compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    }).outputText, sourcePath)
    return loaded.exports.default
  }

  try {
    const StudentBottomNav = loadSource(path.resolve(__dirname, '../apps/miniapp/src/components/student-bottom-nav.tsx'))
    const container = document.createElement('main')
    document.body.appendChild(container)
    const root = createRoot(container)
    roots.push(root)
    const render = async () => {
      await act(async () => root.render(React.createElement(StudentBottomNav, { active: 'home' })))
    }
    const event = async (callback) => { await act(async () => callback()) }
    await render()

    const items = () => Array.from(container.querySelectorAll('.student-bottom-nav__item')).map((node) => node.taroProps)
    const publish = () => container.querySelector('.student-bottom-nav__publish').taroProps
    assert.equal(items().length, 4, '默认保留四个导航入口')
    assert.equal(publish().className, 'student-bottom-nav__publish', '发布入口使用独立触控容器')

    await event(() => { items()[0].onClick(); items()[0].onClick() })
    assert.equal(navigations.length, 0, '当前 active 入口不应重复导航')

    await event(() => { items()[2].onClick(); items()[2].onClick() })
    assert.equal(navigations.length, 1, '同帧点击同一入口只导航一次')
    assert.equal(navigations[0].url, '/pages/student-secondhand/index')
    await event(() => navigations[0].reject(new Error('isolated navigation failure')))
    await event(() => items()[2].onClick())
    assert.equal(navigations.length, 2, '导航失败后可以立即重试')
    await event(() => navigations[1].resolve())
    await event(() => { for (const ref of shows) ref.current() })
    await event(() => items()[3].onClick())
    assert.equal(navigations.length, 3, '返回后 useDidShow 解锁导航')
    assert.equal(navigations[2].url, '/pages/profile/index')
    await event(() => navigations[2].resolve())
    await event(() => { for (const ref of shows) ref.current() })

    await event(() => { publish().onClick(); publish().onClick() })
    assert.equal(navigations.length, 4, '同帧点击发布入口只打开一次')
    assert.equal(navigations[3].url, '/pages/publish-entry/index')
    await event(() => navigations[3].resolve())
    await event(() => { for (const ref of shows) ref.current() })
    await event(() => publish().onClick())
    assert.equal(navigations.length, 5, '发布失败或返回后仍可再次打开')

    // A late failure from a previous page lifetime must not release the lock
    // for a newer request to the same destination.
    await event(() => { for (const ref of shows) ref.current() })
    await event(() => publish().onClick())
    assert.equal(navigations.length, 6)
    await event(() => navigations[4].reject(new Error('stale publish failure')))
    await event(() => publish().onClick())
    assert.equal(navigations.length, 6, '旧请求失败不能解锁返回前台后的同目标新请求')
    await event(() => navigations[5].reject(new Error('current publish failure')))
    await event(() => publish().onClick())
    assert.equal(navigations.length, 7, '当前请求失败后仍允许重试')
    await event(() => navigations[6].resolve())

    options.visibility = { ...visibility, publish: false, post: false, secondhand_publish: false }
    await render()
    assert.equal(container.querySelector('.student-bottom-nav__publish'), null, '关闭发布策略时不渲染发布入口')
    options.visibility = { ...visibility, secondhand: false }
    await render()
    assert.equal(container.querySelectorAll('.student-bottom-nav__item').length, 3, '关闭二手只移除二手入口')

    const stylePath = path.resolve(__dirname, '../apps/miniapp/src/components/student-bottom-nav.scss')
    const scss = fs.readFileSync(stylePath, 'utf8')
    const compiled = sass.compile(stylePath, { silenceDeprecations: ['legacy-js-api', 'global-builtin', 'color-functions', 'import'] }).css
    const transformed = postcss([pxtransform({ platform: 'weapp', designWidth: 750 })]).process(compiled, { from: stylePath }).root
    const declarationsFor = (selector) => {
      const declarations = {}
      transformed.walkRules(selector, rule => rule.walkDecls(decl => { declarations[decl.prop] = decl.value }))
      return declarations
    }
    for (const selector of ['.student-bottom-nav__item', '.student-bottom-nav__publish']) {
      const declarations = declarationsFor(selector)
      for (const width of [320, 375, 390, 430, 768]) {
        for (const property of ['min-width', 'min-height']) {
          assert.ok(declarations[property] && pixels(declarations[property], width) >= 44,
            `${selector} 的 ${property} 经 Sass + Taro PostCSS 后在 ${width}px 屏仍须至少 44px，实际为 ${declarations[property]}`)
        }
      }
    }
    const publishVisual = declarationsFor('.student-bottom-nav__publish-visual')
    assert.equal(publishVisual.width, '72rpx', '发布按钮视觉宽度保持原值，仅扩大外层触控区')
    assert.equal(publishVisual.height, '64rpx', '发布按钮视觉高度保持原值，仅扩大外层触控区')
    const icon = declarationsFor('.student-bottom-nav__icon-asset')
    assert.equal(icon.width, '44rpx', '导航图标保留现有视觉宽度')
    assert.equal(icon.height, '44rpx', '导航图标保留现有视觉高度')
    assert.match(scss, /__item:active\s+\.student-bottom-nav__item-visual/, '按压反馈仅作用于导航内部视觉层')
    assert.match(scss, /@media\s*\(prefers-reduced-motion:\s*reduce\)/, '提供减少动效降级')
    console.log('tactile navigation: isolated behavior and Sass + Taro PostCSS geometry checks passed (320–768px)')
  } finally {
    await act(async () => { for (const root of roots) root.unmount() })
    previous.forEach((descriptor, index) => {
      const name = ['window', 'document', 'IS_REACT_ACT_ENVIRONMENT'][index]
      if (descriptor) Object.defineProperty(globalThis, name, descriptor)
      else delete globalThis[name]
    })
    dom.window.close()
  }
}

run().catch((error) => { console.error(error); process.exitCode = 1 })
