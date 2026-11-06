const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')
const service = read('apps', 'miniapp', 'src', 'services', 'delivery-rider.ts')
const hook = read('apps', 'miniapp', 'src', 'hooks', 'use-rider-availability.ts')
const page = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.tsx')
const adminService = read('apps', 'admin', 'src', 'services', 'delivery-admin.ts')
const adminPage = read('apps', 'admin', 'src', 'DeliveryConsole.tsx')

assert.match(service, /GET|BASE +?=/, '骑手服务应保留 API 基础路径')
assert.match(service, /availability/, '骑手服务应提供接单状态接口')
assert.match(service, /changeRiderAvailability/, '骑手服务应提供开始或停止接单操作')
assert.match(service, /rowVersion/, '接单操作必须携带服务端行版本')
assert.match(hook, /generation|owner|pending/, '接单 hook 应隔离切号、隐藏和迟到响应')
assert.match(hook, /不会自动重试|自动重试/, '未知 POST 结果不得自动重复提交')
assert.match(page, /useRiderAvailability/, '骑手工作台应接入服务端接单状态')
assert.match(page, /开始接单|停止接单/, '骑手工作台应提供主动开工操作')
assert.match(page, /dutyCanClaim/, '抢单按钮必须受服务端在岗和容量状态约束')
assert.match(adminService, /riderAvailabilityConfig|updateRiderAvailabilityConfig/, '后台应提供容量读取和更新服务')
assert.match(adminPage, /RiderAvailabilityConfigPanel/, '后台应展示骑手接单容量配置')
console.log('delivery rider availability contract: ok')

// Run the real parsers and hook with isolated ports. These fixtures never
// contact a backend or change any account, order or production configuration.
function loadSource(relative, imports) {
  const filename = path.join(root, relative)
  const compiled = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    fileName: filename,
  }).outputText
  const loaded = new Module(filename, module)
  loaded.filename = filename
  loaded.require = (id) => {
    assert.ok(Object.hasOwn(imports, id), `测试必须显式隔离依赖：${id}`)
    return imports[id]
  }
  loaded._compile(compiled, filename)
  return loaded.exports
}

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function snapshot(overrides = {}) {
  return { enabled: true, riderUserId: 101, state: 'off_duty', scope: 'campus',
    inFlight: 0, maxInFlight: 3, rowVersion: 0, ...overrides }
}

const rider = loadSource('apps/miniapp/src/services/delivery-rider.ts', {
  './api': { request: () => { throw new Error('未隔离的网络调用') } },
})

function hookHarness(ports = {}) {
  let cursor = 0, output, show, hide
  let account = 101, token = 'test-token-a', allowed = true, reads = 0
  const slots = [], effects = [], writes = []
  const sameDeps = (a, b) => Array.isArray(a) && Array.isArray(b)
    && a.length === b.length && a.every((item, i) => Object.is(item, b[i]))
  const React = {
    useState(initial) {
      const index = cursor++
      if (!slots[index]) slots[index] = { value: initial }
      return [slots[index].value, (value) => { slots[index].value = value }]
    },
    useRef(initial) {
      const index = cursor++
      if (!slots[index]) slots[index] = { current: initial }
      return slots[index]
    },
    useCallback(fn, deps) {
      const index = cursor++
      if (!slots[index] || !sameDeps(slots[index].deps, deps)) slots[index] = { fn, deps }
      return slots[index].fn
    },
    useEffect(fn, deps) {
      const index = cursor++
      if (!slots[index] || !sameDeps(slots[index].deps, deps)) {
        const previous = slots[index]
        const slot = { deps }
        slots[index] = slot
        effects.push(() => { previous?.cleanup?.(); slot.cleanup = fn() })
      }
    },
  }
  const { useRiderAvailability } = loadSource('apps/miniapp/src/hooks/use-rider-availability.ts', {
    react: React,
    '@tarojs/taro': { useDidShow: fn => { show = fn }, useDidHide: fn => { hide = fn } },
    '../services/delivery-rider': {
      riderAvailabilityCanClaim: rider.riderAvailabilityCanClaim,
      getRiderAvailability: async () => {
        reads++
        return rider.parseRiderAvailability(await (ports.read?.(reads) ?? snapshot({ riderUserId: account })))
      },
      changeRiderAvailability: async (action, version) => {
        writes.push({ action, version })
        return rider.parseRiderAvailability(await (ports.change?.(action, version)
          ?? snapshot({ riderUserId: account, state: action === 'start' ? 'on_duty' : 'off_duty', rowVersion: version + 1 })))
      },
    },
  })
  function render() { cursor = 0; output = useRiderAvailability(account, allowed, token); effects.splice(0).forEach(fn => fn()); return output }
  async function flush() { for (let i = 0; i < 4; i++) { await new Promise(setImmediate); render() } }
  render()
  return { render, flush, writes, get value() { return output }, get reads() { return reads },
    setAccount(id) { account = id; render() }, setToken(value) { token = value; render() },
    setAllowed(value) { allowed = value; render() },
    hide() { hide(); render() }, show() { show(); render() },
    unmount() { slots.forEach(slot => slot?.cleanup?.()) },
  }
}

async function runBehaviorChecks() {
  assert.deepEqual(rider.parseRiderAvailability(snapshot()), snapshot(), '首次未开工必须保留两个零值')
  assert.deepEqual(rider.parseRiderAvailability({ enabled: false }), { enabled: false })
  for (const key of ['enabled', 'riderUserId', 'state', 'scope', 'inFlight', 'maxInFlight', 'rowVersion']) {
    const invalid = snapshot(); delete invalid[key]
    assert.throws(() => rider.parseRiderAvailability(invalid), `缺少 ${key} 必须 fail-closed`)
  }
  for (const overrides of [{ state: 'active' }, { scope: 'unknown' }, { inFlight: -1 }, { inFlight: 0.5 },
    { maxInFlight: 0 }, { maxInFlight: 101 }, { rowVersion: Number.MAX_SAFE_INTEGER + 1 },
    { riderUserId: '101' }, { state: 'on_duty', rowVersion: 0 }]) {
    assert.throws(() => rider.parseRiderAvailability(snapshot(overrides)))
  }
  for (const value of [null, { enabled: false }, snapshot(), snapshot({ state: 'on_duty', rowVersion: 1, inFlight: 3 })]) {
    assert.equal(rider.riderAvailabilityCanClaim(value), false)
  }
  assert.equal(rider.riderAvailabilityCanClaim(snapshot({ state: 'on_duty', rowVersion: 1, inFlight: 2 })), true)

  const requests = []
  const api = loadSource('apps/miniapp/src/services/delivery-rider.ts', {
    './api': { request: async (url, options) => { requests.push({ url, options }); return snapshot() } },
  })
  await api.getRiderAvailability()
  await api.changeRiderAvailability('start', 0)
  await api.changeRiderAvailability('stop', 7)
  assert.deepEqual(requests.map(value => [value.url, value.options.method, value.options.data]), [
    ['/delivery/rider/availability', undefined, undefined],
    ['/delivery/rider/availability/start', 'POST', { expectedRowVersion: 0 }],
    ['/delivery/rider/availability/stop', 'POST', { expectedRowVersion: 7 }],
  ])
  await assert.rejects(() => api.changeRiderAvailability('invalid', 1))
  await assert.rejects(() => api.changeRiderAvailability('start', -1))
  assert.equal(requests.length, 3, '无效参数不得发出请求')

  const adminRequests = []
  const config = { enabled: true, maxInFlightPerRider: 3, rowVersion: 1, updatedAt: '2026-11-06T00:00:00Z' }
  const admin = loadSource('apps/admin/src/services/delivery-admin.ts', {
    './http': { adminRequest: async (url, options) => { adminRequests.push({ url, options }); return config } },
  })
  for (const limit of [1, 100]) assert.equal(admin.parseRiderAvailabilityConfig({ ...config, maxInFlightPerRider: limit }).maxInFlightPerRider, limit)
  for (const limit of [0, 101, 1.5, '3', NaN]) {
    assert.throws(() => admin.parseRiderAvailabilityConfig({ ...config, maxInFlightPerRider: limit }))
    await assert.rejects(() => admin.deliveryAdminAPI.updateRiderAvailabilityConfig(limit, 1))
  }
  await admin.deliveryAdminAPI.updateRiderAvailabilityConfig(4, 2)
  assert.equal(adminRequests.length, 1)
  assert.equal(adminRequests[0].url, '/admin/delivery/rider-config')
  assert.equal(adminRequests[0].options.method, 'PATCH')
  assert.deepEqual(JSON.parse(adminRequests[0].options.body), { maxInFlightPerRider: 4, expectedRowVersion: 2 })

  const start = deferred()
  const normal = hookHarness({ change: () => start.promise })
  await normal.flush()
  assert.equal(normal.value.canClaim, false)
  const changing = normal.value.change()
  normal.render()
  assert.equal(normal.value.busy, true)
  assert.equal(await normal.value.change(), false, '连续点击不能重复发 POST')
  assert.equal(await normal.value.refresh(), false, 'POST 进行中不能用 GET 覆盖当前操作')
  start.resolve(snapshot({ state: 'on_duty', rowVersion: 1 }))
  assert.equal(await changing, true)
  await normal.flush()
  assert.equal(normal.value.canClaim, true)
  assert.deepEqual(normal.writes, [{ action: 'start', version: 0 }])
  normal.unmount()

  const stop = hookHarness({ read: () => snapshot({ state: 'on_duty', rowVersion: 5 }) })
  await stop.flush()
  assert.equal(await stop.value.change(), true)
  await stop.flush()
  assert.equal(stop.value.canClaim, false)
  assert.deepEqual(stop.writes, [{ action: 'stop', version: 5 }])
  stop.unmount()

  const unknown = hookHarness({
    read: count => snapshot(count > 1 ? { state: 'on_duty', rowVersion: 1 } : {}),
    change: async () => { throw new Error('response lost') },
  })
  await unknown.flush()
  assert.equal(await unknown.value.change(), true)
  await unknown.flush()
  assert.equal(unknown.reads, 2)
  assert.equal(unknown.writes.length, 1, '未知 POST 结果只补 GET，不能自动重试或反向操作')
  assert.equal(unknown.value.canClaim, true)
  unknown.unmount()

  const failed = hookHarness({
    read: count => { if (count > 1) throw new Error('offline'); return snapshot() },
    change: async () => { throw new Error('offline') },
  })
  await failed.flush()
  assert.equal(await failed.value.change(), false)
  await failed.flush()
  assert.equal(failed.value.value, null)
  assert.equal(failed.value.canClaim, false)
  assert.match(failed.value.message, /不会自动重试/)
  assert.equal(failed.writes.length, 1)
  failed.unmount()

  for (const boundary of ['account', 'token', 'hide', 'permission', 'unmount']) {
    const delayed = deferred()
    const h = hookHarness({ read: count => count === 1 ? delayed.promise : snapshot({ riderUserId: boundary === 'account' ? 202 : 101 }) })
    if (boundary === 'account') h.setAccount(202)
    if (boundary === 'token') h.setToken('test-token-b')
    if (boundary === 'hide') h.hide()
    if (boundary === 'permission') h.setAllowed(false)
    if (boundary === 'unmount') h.unmount()
    delayed.resolve(snapshot({ state: 'on_duty', rowVersion: 1 }))
    await h.flush()
    assert.equal(h.value.canClaim, false, `${boundary} 后的迟到响应不能恢复抢单资格`)
    assert.equal(await h.value.change(), boundary === 'account' || boundary === 'token')
    assert.equal(h.writes.length, boundary === 'account' || boundary === 'token' ? 1 : 0)
    h.unmount()
  }
  console.log('delivery rider availability behavior: PASS (parsers, API, zero-version, concurrency, recovery, lifecycle)')
}

runBehaviorChecks().catch(error => { console.error(error); process.exitCode = 1 })
