const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const React = require('react')
const { createRoot } = require('react-dom/client')
const { JSDOM } = require('jsdom')

async function testPublishConfirmation() {
  const file = path.resolve(__dirname, '../apps/miniapp/src/services/media.ts')
  const mod = new Module(file)
  const calls = []
  let respond
  mod.require = id => {
    if (id === './taro') return { default: {} }
    if (id === './api') return {
      isApiError: () => false,
      request: async (url, options) => { calls.push({ url, method: options?.method ?? 'GET' }); return respond(url, options) },
    }
    throw new Error(`Unexpected media service dependency: ${id}`)
  }
  mod._compile('const setTimeout = fn => queueMicrotask(fn);\n' + ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText, file)
  const { confirmPostMotion, postMotionAvailable } = mod.exports
  const asset = id => ({ assetId: id, ticketId: `ticket-${id}`, purpose: 'post', state: 'approved', canUse: true,
    technicalStatus: 'final_ready', recovery: 'none' })
  const motion = { kind: 'video', poster: asset('cover'), video: asset('clip'), durationMs: 5000 }
  let videoReads = 0
  respond = url => {
    if (url.endsWith('/cover')) return motion.poster
    if (url.endsWith('/clip')) return ++videoReads === 1 ? { ...motion.video, state: 'review_pending', canUse: false } : motion.video
    throw new Error(`Unexpected request: ${url}`)
  }
  await confirmPostMotion(motion)
  assert.equal(videoReads, 2, 'publishing waits for video approval as well as the cover')
  assert.ok(calls.every(call => call.method === 'GET'), 'approved technical uploads are not uploaded or completed again')

  calls.length = 0
  respond = url => url.endsWith('/complete') ? motion.video : url.endsWith('/cover') ? motion.poster : motion.video
  await confirmPostMotion({ ...motion, video: { ...motion.video, technicalStatus: 'validating', recovery: 'complete_upload' } })
  assert.deepEqual(calls.filter(call => call.method === 'POST'), [{ url: '/miniapp/media/uploads/ticket-clip/complete', method: 'POST' }],
    'a timed-out completion retries the same immutable ticket without uploading the file again')

  respond = url => url.endsWith('/cover') ? motion.poster : { ...motion.video, state: 'rejected', canUse: false, message: '视频未通过审核' }
  await assert.rejects(confirmPostMotion(motion), /未通过审核/)
  const countBeforeCancellation = calls.length
  await assert.rejects(confirmPostMotion(motion, 1000, undefined, () => false), /取消/)
  await assert.rejects(confirmPostMotion(motion, 0), /仍在处理中/)
  assert.equal(calls.length, countBeforeCancellation, 'cancelled or expired attempts do not issue requests')
  respond = () => { throw new Error('capability service unavailable') }
  assert.equal(await postMotionAvailable(), false, 'an unavailable backend cannot expose video upload')
}

async function main() {
  await testPublishConfirmation()
  const dom = new JSDOM('<!doctype html><html><body></body></html>')
  const previous = new Map(['window', 'document', 'IS_REACT_ACT_ENVIRONMENT', '__postMotionTimers', 'matchMedia'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]))
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, IS_REACT_ACT_ENVIRONMENT: true })
  let seq = 0, networkType = 'wifi', reduced = false, delayedNetwork = null
  const timers = new Map(), hidden = new Set(), shown = new Set(), networks = new Set(), motionListeners = new Set()
  const observers = [], paused = [], cache = new Map(), roots = new Set()
  globalThis.__postMotionTimers = { set: (fn, ms) => { timers.set(++seq, { fn, ms }); return seq }, clear: id => timers.delete(id) }
  globalThis.matchMedia = () => ({ get matches() { return reduced }, addEventListener: (_, fn) => motionListeners.add(fn), removeEventListener: (_, fn) => motionListeners.delete(fn) })
  const native = {
    getNetworkType: () => delayedNetwork ?? Promise.resolve({ networkType }),
    onNetworkStatusChange: fn => networks.add(fn), offNetworkStatusChange: fn => networks.delete(fn),
    createVideoContext: id => ({ pause: () => paused.push(id) }),
    createIntersectionObserver: () => {
      const observer = { disconnected: false, relativeToViewport() { return this }, observe(selector, report) { this.selector = selector; this.report = report }, disconnect() { this.disconnected = true } }
      observers.push(observer); return observer
    },
  }
  const lifecycle = collection => fn => React.useEffect(() => { collection.add(fn); return () => collection.delete(fn) }, [fn])
  const component = name => ({ children, className, id, style, ...props }) => React.createElement('div', { className, id, style, 'data-kind': name, ref: element => { if (element) element.taroProps = props } }, children)
  function load(relative) {
    const file = path.resolve(__dirname, '../apps/miniapp/src', relative)
    if (cache.has(file)) return cache.get(file).exports
    const mod = new Module(file); cache.set(file, mod)
    mod.require = id => {
      if (id === 'react' || id === 'react/jsx-runtime') return require(id)
      if (id === '@tarojs/components') return Object.fromEntries(['View', 'Text', 'Image', 'Video'].map(name => [name, component(name)]))
      if (id === '@tarojs/taro') return { useDidHide: lifecycle(hidden), useDidShow: lifecycle(shown), nextTick: fn => queueMicrotask(fn) }
      if (id.endsWith('services/taro')) return { default: native }
      if (id.endsWith('capsule-safe')) return { getCapsuleLayout: () => ({ totalHeaderHeight: 88 }) }
      if (id.endsWith('.scss')) return {}
      const base = path.resolve(path.dirname(file), id)
      const target = ['.ts', '.tsx'].map(ext => base + ext).find(candidate => fs.existsSync(candidate))
      if (target) return load(path.relative(path.resolve(__dirname, '../apps/miniapp/src'), target))
      throw Error(`Unknown dependency: ${id}`)
    }
    const source = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX } }).outputText
    mod._compile('const setTimeout = globalThis.__postMotionTimers.set, clearTimeout = globalThis.__postMotionTimers.clear;\n' + source, file)
    return mod.exports
  }
  const { previewSeconds, createMotionPreviewCoordinator } = load('utils/motion-preview.ts')
  const Media = load('components/post-motion-media.tsx').default
  const { usePostMotionPreview } = load('hooks/use-post-motion-preview.ts')
  const motion = { kind: 'video', url: 'https://media.invalid/verified.mp4', posterUrl: 'https://media.invalid/cover.jpg', width: 720, height: 1280, durationMs: 10000 }
  const notify = async collection => React.act(async () => [...collection].forEach(fn => fn()))
  const visible = async (id, ratio) => React.act(async () => {
    const observer = [...observers].reverse().find(o => o.selector === `#post-video-preview-${id}-frame` && !o.disconnected)
    assert.ok(observer, `observer for ${id}`); observer.report({ intersectionRatio: ratio })
  })
  const fixture = () => {
    const container = document.createElement('main'); document.body.append(container)
    const root = createRoot(container); roots.add(root)
    return { container, render: element => React.act(async () => root.render(element)), unmount: async () => { await React.act(async () => root.unmount()); roots.delete(root); container.remove() } }
  }
  function Feed({ active = true, session = 'one' }) {
    const preview = usePostMotionPreview(active, session)
    return ['one', 'two'].map((id, order) => React.createElement(Media, { key: id, id, order, motion, mode: 'preview', ...preview }))
  }
  try {
    for (const duration of [500, 1000, 5000, 10000, 60000]) {
      const seconds = previewSeconds(duration)
      assert.ok(seconds > 0 && seconds <= 3 && seconds < duration / 1000 && duration - seconds * 1000 >= 300)
    }
    for (const invalid of [NaN, Infinity, 0, 499, 60001]) assert.equal(previewSeconds(invalid), 0)
    const changes = [], stops = []
    const coordinator = createMotionPreviewCoordinator(id => changes.push(id))
    coordinator.setEnabled(true); coordinator.visible('second', 1, () => stops.push('second')); coordinator.visible('first', 0, () => stops.push('first'))
    assert.deepEqual(changes, ['second'], 'newly visible cards cannot interrupt an active preview')
    coordinator.started('second')
    assert.equal(changes.at(-1), 'second', 'marking a started preview does not evict the current player')
    coordinator.finish('second'); assert.equal(changes.at(-1), 'first'); assert.deepEqual(stops, ['second'])
    coordinator.hidden('first'); assert.equal(changes.at(-1), '')
    coordinator.dispose()

    const feed = fixture()
    await feed.render(React.createElement(Feed))
    assert.equal(feed.container.querySelectorAll('[data-kind="Video"]').length, 0, 'cover first, no offscreen download')
    await visible('one', .8); await visible('two', .8)
    let players = feed.container.querySelectorAll('[data-kind="Video"]')
    assert.equal(players.length, 1)
    assert.equal(players[0].taroProps.muted, true); assert.equal(players[0].taroProps.controls, false); assert.equal(players[0].taroProps.loop, false)
    const firstProps = players[0].taroProps
    await React.act(async () => firstProps.onPlay())
    assert.ok([...timers.values()].some(t => t.ms === 3000), 'wall-time guard caps the preview')
    await React.act(async () => firstProps.onTimeUpdate({ detail: { currentTime: 3 } }))
    assert.equal(feed.container.querySelectorAll('[data-kind="Video"]').length, 1)
    assert.equal(feed.container.querySelector('[data-kind="Video"]').id, 'post-video-preview-two')
    await visible('two', .4)
    assert.ok(feed.container.querySelector('[data-kind="Video"]'), 'threshold hysteresis prevents boundary flicker')
    await visible('two', 0)
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null)
    await visible('one', .8)
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'finished clip does not replay on intersection callbacks')
    await visible('two', .8)
    await notify(hidden)
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'hide stops playback')
    assert.equal(timers.size, 0, 'hide clears playback and buffering timers')
    await notify(shown)
    await visible('two', .8)
    await feed.render(React.createElement(Feed, { active: false }))
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'tab change stops playback')
    networkType = 'none'
    await notify(shown); await feed.render(React.createElement(Feed, { session: 'two' }))
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'offline stays on cover')
    networkType = '4g'; await notify(shown)
    await visible('one', .8)
    await React.act(async () => { reduced = true; [...motionListeners].forEach(fn => fn()) })
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'reduced motion disables auto playback')
    await React.act(async () => { reduced = false; [...motionListeners].forEach(fn => fn()) })
    await visible('one', .8)
    const failing = feed.container.querySelector('[data-kind="Video"]').taroProps
    await React.act(async () => failing.onError())
    assert.equal(feed.container.querySelector('[data-kind="Video"]'), null, 'video errors keep the cover')
    assert.ok(feed.container.querySelector('[data-kind="Image"]'))
    const stale = [...observers]; await feed.unmount()
    await React.act(async () => stale.forEach(o => o.report?.({ intersectionRatio: 1 })))
    assert.equal(timers.size, 0)
    assert.ok(observers.every(o => o.disconnected), 'unmount disconnects native observers')

    const once = fixture()
    await once.render(React.createElement(Feed, { session: 'once-one' }))
    await visible('one', .8)
    const startedOnce = once.container.querySelector('[data-kind="Video"]').taroProps
    await React.act(async () => startedOnce.onPlay())
    await visible('one', 0)
    await visible('one', .8)
    assert.equal(once.container.querySelector('[data-kind="Video"]'), null, 'a preview that actually started only auto-plays once per session')
    await once.unmount()

    const staleEvents = fixture()
    await staleEvents.render(React.createElement(Feed, { session: 'stale-one' }))
    await visible('one', .8)
    const oldPlayer = staleEvents.container.querySelector('[data-kind="Video"]').taroProps
    await React.act(async () => oldPlayer.onPlay())
    await notify(hidden)
    await staleEvents.render(React.createElement(Feed, { session: 'stale-two' }))
    await notify(shown)
    await visible('one', .8)
    const newSessionPlayer = staleEvents.container.querySelector('[data-kind="Video"]').taroProps
    await React.act(async () => newSessionPlayer.onPlay())
    await React.act(async () => oldPlayer.onEnded())
    await React.act(async () => oldPlayer.onTimeUpdate({ detail: { currentTime: 3 } }))
    await React.act(async () => oldPlayer.onError())
    assert.ok(staleEvents.container.querySelector('[data-kind="Video"]'), 'late events from a hidden player and another session cannot poison the new session')
    await staleEvents.unmount()

    const failedSession = fixture()
    await failedSession.render(React.createElement(Feed, { session: 'failed-one' }))
    await visible('one', .8)
    const failedPlayer = failedSession.container.querySelector('[data-kind="Video"]').taroProps
    await React.act(async () => failedPlayer.onError())
    await failedSession.render(React.createElement(Feed, { session: 'failed-two' }))
    await visible('one', .8)
    assert.ok(failedSession.container.querySelector('[data-kind="Video"]'), 'a failure in one account does not suppress the same media in another account')
    await failedSession.unmount()

    let resolveNetwork
    delayedNetwork = new Promise(resolve => { resolveNetwork = resolve })
    const networkRace = fixture()
    const observerCount = observers.length
    await networkRace.render(React.createElement(Feed, { session: 'network-race' }))
    await React.act(async () => [...networks].forEach(fn => fn({ networkType: 'none' })))
    await React.act(async () => resolveNetwork({ networkType: 'wifi' }))
    assert.equal(observers.length, observerCount, 'a stale network query cannot override a newer offline event')
    delayedNetwork = null
    await networkRace.unmount()

    const detail = fixture()
    await detail.render(React.createElement(Media, { id: 'detail', motion, mode: 'detail' }))
    assert.equal(detail.container.querySelector('[data-kind="Video"]'), null)
    await React.act(async () => detail.container.querySelector('.post-motion__play').taroProps.onClick())
    const player = detail.container.querySelector('[data-kind="Video"]').taroProps
    assert.equal(player.controls, true); assert.equal(player.muted, false)
    await React.act(async () => player.onTimeUpdate({ detail: { currentTime: 8 } }))
    assert.ok(detail.container.querySelector('[data-kind="Video"]'), 'detail can play beyond the preview cutoff')
    await notify(hidden)
    assert.equal(detail.container.querySelector('[data-kind="Video"]'), null)
    await detail.unmount()
    assert.equal(networks.size, 0); assert.equal(hidden.size, 0); assert.equal(shown.size, 0)
    console.log('PASS: motion upload confirmation/retry/rejection and real components — one silent player, short cutoff, viewport/tab/background/error cleanup, network/accessibility fallback, manual detail playback.')
  } finally {
    for (const root of roots) await React.act(async () => root.unmount())
    for (const [key, value] of previous) { if (value) Object.defineProperty(globalThis, key, value); else delete globalThis[key] }
    dom.window.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
