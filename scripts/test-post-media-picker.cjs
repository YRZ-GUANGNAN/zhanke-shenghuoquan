const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const { execFileSync } = require('node:child_process')
const ts = require('typescript')

// Compile the real service in isolation. --baseline verifies the original RED
// state without overwriting a concurrently edited service or touching the network.
const root = path.resolve(__dirname, '..')
const relative = 'apps/miniapp/src/services/media.ts'
const serviceFile = path.join(root, relative)
const baseline = process.argv.includes('--baseline')
const source = baseline
  ? execFileSync('git', ['show', `HEAD:${relative}`], { cwd: root, encoding: 'utf8' })
  : fs.readFileSync(serviceFile, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const LIMIT = 30 * 1024 * 1024
const digest = '0123456789abcdef0123456789abcdef'
const image = (tempFilePath = '/photo.jpg', extra = {}) => ({ tempFilePath, fileType: 'image', size: 24, ...extra })
const video = (extra = {}) => ({ tempFilePath: '/clip.mp4', fileType: 'video', size: 24, duration: 2.5, thumbTempFilePath: '/thumb.jpg', ...extra })
const mp4 = (length = 24) => {
  const bytes = new Uint8Array(length)
  if (length >= 12) bytes.set([0, 0, 0, 24, 102, 116, 121, 112, 105, 115, 111, 109])
  return bytes.buffer
}
const asset = (id = 'existing-poster') => ({
  assetId: id, ticketId: `ticket-${id}`, purpose: 'post', state: 'approved',
  technicalStatus: 'final_ready', reviewStatus: 'approved', ticketStatus: 'completed',
  message: '已完成', recovery: 'none', canUse: true, updatedAt: '2026-10-25T00:00:00.000Z',
  previewPath: '/existing-poster.webp',
})

function fixture(selections = [], config = {}) {
  const calls = { choose: [], imageInfo: [], compress: [], fileInfo: [], reads: [], api: [], puts: [], progress: [] }
  const queue = [...selections]
  const files = new Map(Object.entries(config.files || {}))
  const sessions = new Map()
  const state = { active: true }
  const file = filePath => files.get(filePath) || { bytes: mp4(), size: 24 }
  const taro = {
    chooseMedia: async options => {
      calls.choose.push(options)
      assert.ok(queue.length, '不能重复打开没有预期的相册')
      const selected = queue.shift()
      await config.onChoose?.(options, calls, state)
      if (selected instanceof Error || selected?.errMsg) throw selected
      return { tempFiles: selected }
    },
    getImageInfo: async options => {
      calls.imageInfo.push(options)
      await config.onImageInfo?.(options, calls, state)
      return { width: 2400, height: 1600, type: file(options.src).type || (options.src.endsWith('.webp') ? 'webp' : 'jpeg') }
    },
    compressImage: async options => {
      calls.compress.push(options)
      await config.onCompress?.(options, calls, state)
      return { tempFilePath: `${options.src}.compressed.webp` }
    },
    getFileInfo: async options => {
      calls.fileInfo.push(options)
      await config.onFileInfo?.(options, calls, state)
      return { size: file(options.filePath).size, digest }
    },
    getFileSystemManager: () => ({
      readFileSync: filePath => {
        calls.reads.push(filePath)
        return file(filePath).bytes
      },
    }),
    request: async options => {
      calls.puts.push(options)
      assert.equal(options.method, 'PUT')
      assert.match(options.url, /^https:\/\/mock-upload\.invalid\//)
      assert.ok(options.data instanceof ArrayBuffer, '必须上传经过文件读取校验的二进制')
      assert.equal(Number(options.header['Content-Length']), options.data.byteLength)
      assert.equal(options.header['x-cos-forbid-overwrite'], 'true')
      await config.onPut?.(options, calls, state)
      return { statusCode: 200 }
    },
  }
  const api = {
    isApiError: error => error?.isApiError === true,
    request: async (url, options) => {
      const call = { url, options }
      calls.api.push(call)
      await config.onApi?.(call, calls, state)
      if (url === '/miniapp/media/uploads') {
        assert.equal(options.method, 'POST')
        assert.equal(options.data.purpose, 'post')
        assert.equal(options.data.contentMD5, Buffer.from(digest, 'hex').toString('base64'))
        assert.ok(options.data.idempotencyKey)
        const ticketId = `ticket-${sessions.size + 1}`
        const session = {
          ticketId, assetId: `asset-${sessions.size + 1}`, uploadUrl: `https://mock-upload.invalid/${ticketId}`,
          requiredHeaders: { contentType: options.data.contentType, contentMD5: options.data.contentMD5, contentLength: options.data.sizeBytes, forbidOverwrite: 'true' },
          expiresAt: new Date(Date.now() + 60000).toISOString(), state: 'waiting_upload',
        }
        sessions.set(ticketId, session)
        return session
      }
      const match = /^\/miniapp\/media\/uploads\/([^/]+)\/complete$/.exec(url)
      if (match) {
        assert.equal(options.method, 'POST')
        const session = sessions.get(match[1])
        assert.ok(session, '只确认本次授权生成的不可变 ticket')
        return { ...asset(session.assetId), ticketId: session.ticketId }
      }
      throw new Error(`未预期 API 请求（不触网）：${url}`)
    },
  }
  const mod = new Module(serviceFile)
  mod.require = id => {
    if (id === './taro') return { default: taro }
    if (id === './api') return api
    throw new Error(`未预期媒体服务依赖：${id}`)
  }
  mod._compile(compiled, serviceFile)
  const progress = message => {
    calls.progress.push(message)
    config.onProgress?.(message, calls, state)
  }
  return {
    service: mod.exports, calls, state,
    pick: (count = 9, allowMotion = true) => mod.exports.chooseAndUploadPostMedia(count, allowMotion, progress, () => state.active),
    attach: poster => mod.exports.chooseAndAttachPostLiveClip(poster, progress, () => state.active),
    legacy: kind => mod.exports.chooseAndUploadPostMotion(kind, progress, () => state.active),
    uploads: () => calls.api.filter(call => call.url === '/miniapp/media/uploads'),
  }
}

function assertNoUploads(f, message = '拒绝选择时不得申请上传票据或 PUT') {
  assert.equal(f.calls.api.length, 0, message)
  assert.equal(f.calls.puts.length, 0, message)
}

const cases = []
const test = (name, run) => cases.push({ name, run })

test('导出两个新入口并保留旧动态入口', async () => {
  const { service } = fixture()
  for (const name of ['chooseAndUploadPostMedia', 'chooseAndAttachPostLiveClip', 'chooseAndUploadPostMotion']) {
    assert.equal(typeof service[name], 'function', `缺少媒体服务接口 ${name}`)
  }
})

test('空选择返回空图片结果且不上传', async () => {
  const f = fixture([[]])
  assert.deepEqual(await f.pick(), { kind: 'images', images: [] })
  assertNoUploads(f)
})

test('图片经过真实压缩、授权、PUT、complete 链路并保留顺序', async () => {
  const f = fixture([[image('/one.jpg'), image('/two.png')]])
  const result = await f.pick(2)
  assert.equal(result.kind, 'images')
  assert.deepEqual(result.images.map(item => item.previewPath), ['/one.jpg.compressed.webp', '/two.png.compressed.webp'])
  assert.ok(result.images.every(item => item.canUse && item.ticketId && item.assetId))
  assert.deepEqual(f.calls.choose[0].mediaType, ['image', 'video'])
  assert.equal(f.calls.choose[0].count, 2)
  assert.equal(f.calls.choose.length, 1)
  assert.equal(f.uploads().length, 2)
  assert.equal(f.calls.puts.length, 2)
  assert.equal(f.calls.api.length, 4)
  assert.ok(f.uploads().every(call => call.options.data.contentType === 'image/webp'))
  assert.ok(f.calls.compress.every(call => call.compressedWidth <= 1920 && call.compressedHeight <= 1920))
  assert.ok(f.calls.progress.length > 0 && f.calls.progress.every(message => typeof message === 'string'))
})

test('能力关闭时相册参数仅允许 image，并兼容缺省 fileType', async () => {
  const f = fixture([[image('/old-client.jpg', { fileType: undefined })]])
  const result = await f.pick(3, false)
  assert.equal(result.kind, 'images')
  assert.equal(result.images.length, 1)
  assert.deepEqual(f.calls.choose[0].mediaType, ['image'])
  assert.equal(f.uploads().length, 1)
})

for (const [name, files, count, allowMotion] of [
  ['混选图片与视频', [image(), video()], 9, true],
  ['两个视频', [video(), video({ tempFilePath: '/second.mp4' })], 9, true],
  ['未知 fileType', [image(), image('/unknown.bin', { fileType: 'audio' })], 9, true],
  ['混合入口缺省 fileType', [image('/unknown.jpg', { fileType: undefined })], 9, true],
  ['纯图片入口未知 fileType', [image('/unknown.bin', { fileType: 'audio' })], 9, false],
  ['能力关闭却返回视频', [video()], 9, false],
  ['返回数量超过剩余 count', [image('/one.jpg'), image('/two.jpg')], 1, false],
  ['缺少文件路径', [image('')], 9, true],
]) {
  test(`${name}在上传前拒绝整个选择集合`, async () => {
    const f = fixture([files])
    await assert.rejects(f.pick(count, allowMotion))
    assertNoUploads(f)
    assert.equal(f.calls.compress.length, 0, '不应先处理集合中的合法图片')
  })
}

for (const count of [0, -1, 10, 1.5, NaN, Infinity]) {
  test(`非法 count ${count} 不打开相册`, async () => {
    const f = fixture()
    await assert.rejects(f.pick(count))
    assert.equal(f.calls.choose.length, 0)
    assertNoUploads(f)
  })
}

test('选中视频直接上传既有 clip，缩略图存在时不再打开相册', async () => {
  const f = fixture([[video()]])
  const result = await f.pick()
  assert.equal(result.kind, 'motion')
  assert.equal(result.motion.kind, 'video')
  assert.equal(result.motion.durationMs, 2500)
  assert.equal(result.motion.video.previewPath, '/clip.mp4')
  assert.equal(result.motion.poster.previewPath, '/thumb.jpg.compressed.webp')
  assert.equal(f.calls.choose.length, 1)
  assert.deepEqual(f.uploads().map(call => call.options.data.contentType), ['image/webp', 'video/mp4'])
  assert.equal(f.calls.puts.length, 2)
  assert.ok(f.calls.reads.includes('/clip.mp4'))
})

test('视频没有 thumb 时只补选一次图片封面，不重新选视频', async () => {
  const f = fixture([[video({ thumbTempFilePath: undefined })], [image('/cover.jpg')]])
  const result = await f.pick()
  assert.equal(result.motion.poster.previewPath, '/cover.jpg.compressed.webp')
  assert.equal(result.motion.video.previewPath, '/clip.mp4')
  assert.equal(f.calls.choose.length, 2)
  assert.deepEqual(f.calls.choose[1].mediaType, ['image'])
  assert.equal(f.calls.choose[1].count, 1)
})

for (const duration of [.5, 60]) {
  test(`视频允许时长边界 ${duration} 秒`, async () => {
    const f = fixture([[video({ duration })]])
    assert.equal((await f.pick()).motion.durationMs, duration * 1000)
  })
}
for (const duration of [0, .499, 60.001, NaN, Infinity, undefined]) {
  test(`视频拒绝非法时长 ${duration} 且无上传`, async () => {
    const f = fixture([[video({ duration })]])
    await assert.rejects(f.pick(), /0\.5.*60/)
    assertNoUploads(f)
  })
}

test('30 MiB 视频边界允许上传', async () => {
  const f = fixture([[video({ size: LIMIT })]], { files: { '/clip.mp4': { size: LIMIT, bytes: mp4(LIMIT) } } })
  assert.equal((await f.pick()).kind, 'motion')
  assert.equal(f.uploads().at(-1).options.data.sizeBytes, LIMIT)
})

for (const [name, clip, info, pattern] of [
  ['相册声明超 30 MiB', video({ size: LIMIT + 1 }), { size: 24, bytes: mp4() }, /30/],
  ['实际文件超 30 MiB', video(), { size: LIMIT + 1, bytes: mp4() }, /30/],
  ['实际文件零字节', video(), { size: 0, bytes: new ArrayBuffer(0) }, /30|读取/],
  ['读取长度不一致', video(), { size: 25, bytes: mp4() }, /读取/],
  ['伪装 mp4 扩展名但无 ftyp', video(), { size: 24, bytes: new ArrayBuffer(24) }, /MP4/],
  ['文件头不足 12 字节', video(), { size: 11, bytes: mp4(11) }, /MP4/],
]) {
  test(`${name}在封面和视频授权前拒绝`, async () => {
    const f = fixture([[clip]], { files: { '/clip.mp4': info } })
    await assert.rejects(f.pick(), pattern)
    assertNoUploads(f)
    assert.equal(f.calls.compress.length, 0)
  })
}

for (const duration of [.5, 5]) {
  test(`添加 ${duration} 秒动态片段复用原 poster，仅上传 video`, async () => {
    const poster = Object.freeze(asset())
    const before = { ...poster }
    const f = fixture([[video({ duration, thumbTempFilePath: undefined })]])
    const motion = await f.attach(poster)
    assert.equal(motion.kind, 'live_photo')
    assert.equal(motion.poster, poster, '保持原 poster 对象及不可变资产身份')
    assert.deepEqual(poster, before)
    assert.equal(motion.durationMs, duration * 1000)
    assert.equal(f.calls.choose.length, 1)
    assert.deepEqual(f.calls.choose[0].mediaType, ['video'])
    assert.equal(f.calls.choose[0].count, 1)
    assert.equal(f.calls.choose[0].maxDuration, 5)
    assert.deepEqual(f.uploads().map(call => call.options.data.contentType), ['video/mp4'])
    assert.equal(f.calls.puts.length, 1)
    assert.equal(f.calls.compress.length, 0)
    assert.equal(f.calls.imageInfo.length, 0)
  })
}
for (const duration of [.499, 5.001, NaN, Infinity]) {
  test(`动态片段拒绝 ${duration} 秒且保留原图`, async () => {
    const poster = Object.freeze(asset())
    const before = { ...poster }
    const f = fixture([[video({ duration })]])
    await assert.rejects(f.attach(poster), /0\.5.*5/)
    assertNoUploads(f)
    assert.deepEqual(poster, before)
  })
}

test('动态片段取消不动原 poster、不上传', async () => {
  const cancel = { errMsg: 'chooseMedia:fail cancel' }
  const poster = Object.freeze(asset())
  const before = { ...poster }
  const f = fixture([cancel])
  await assert.rejects(f.attach(poster), error => error === cancel)
  assert.deepEqual(poster, before)
  assertNoUploads(f)
})

for (const files of [[], [video(), video()], [image()]]) {
  test(`动态片段异常选择 ${files.length} 个文件不上传`, async () => {
    const f = fixture([files])
    await assert.rejects(f.attach(asset()))
    assertNoUploads(f)
  })
}

test('统一相册取消透传原取消结果，且无上传', async () => {
  const cancel = { errMsg: 'chooseMedia:fail cancel' }
  const f = fixture([cancel])
  await assert.rejects(f.pick(), error => error === cancel)
  assertNoUploads(f)
})

test('补封面取消或空选择时不上传视频', async () => {
  for (const selection of [{ errMsg: 'chooseMedia:fail cancel' }, []]) {
    const f = fixture([[video({ thumbTempFilePath: undefined })], selection])
    await assert.rejects(f.pick())
    assertNoUploads(f)
  }
})

for (const method of ['pick', 'attach']) {
  test(`${method} 开始前 shouldContinue=false 不打开相册`, async () => {
    const f = fixture()
    f.state.active = false
    await assert.rejects(method === 'pick' ? f.pick() : f.attach(asset()), /取消/)
    assert.equal(f.calls.choose.length, 0)
    assertNoUploads(f)
  })
}

test('相册返回后失效不处理或上传图片', async () => {
  const f = fixture([[image()]], { onChoose: (_options, _calls, state) => { state.active = false } })
  await assert.rejects(f.pick(), /取消/)
  assertNoUploads(f)
  assert.equal(f.calls.compress.length, 0)
})

test('视频文件信息返回后失效不继续补封面或上传', async () => {
  const f = fixture([[video({ thumbTempFilePath: undefined })]], { onFileInfo: (_options, _calls, state) => { state.active = false } })
  await assert.rejects(f.pick(), /取消/)
  assertNoUploads(f)
  assert.equal(f.calls.choose.length, 1)
})

for (const point of ['image-file', 'authorized', 'put', 'completed']) {
  test(`图片在 ${point} 失效后不执行后续上传阶段`, async () => {
    const config = {}
    if (point === 'image-file') config.onFileInfo = (_options, _calls, state) => { state.active = false }
    if (point === 'authorized') config.onApi = (call, _calls, state) => { if (call.url === '/miniapp/media/uploads') state.active = false }
    if (point === 'put') config.onPut = (_options, _calls, state) => { state.active = false }
    if (point === 'completed') config.onApi = (call, _calls, state) => { if (call.url.endsWith('/complete')) state.active = false }
    const f = fixture([[image()]], config)
    await assert.rejects(f.pick(), /取消/)
    assert.equal(f.uploads().length, point === 'image-file' ? 0 : 1)
    assert.equal(f.calls.puts.length, ['image-file', 'authorized'].includes(point) ? 0 : 1)
    assert.equal(f.calls.api.filter(call => call.url.endsWith('/complete')).length, point === 'completed' ? 1 : 0)
  })
}

test('封面完成后失效不继续授权视频', async () => {
  const f = fixture([[video()]], { onApi: (call, _calls, state) => { if (call.url.endsWith('/complete')) state.active = false } })
  await assert.rejects(f.pick(), /取消/)
  assert.deepEqual(f.uploads().map(call => call.options.data.contentType), ['image/webp'])
  assert.equal(f.calls.puts.length, 1)
})

for (const point of ['authorized', 'put', 'completed']) {
  test(`复用封面的片段在 ${point} 失效后不执行后续阶段`, async () => {
    const config = {}
    if (point === 'authorized') config.onApi = (call, _calls, state) => { if (call.url === '/miniapp/media/uploads') state.active = false }
    if (point === 'put') config.onPut = (_options, _calls, state) => { state.active = false }
    if (point === 'completed') config.onApi = (call, _calls, state) => { if (call.url.endsWith('/complete')) state.active = false }
    const f = fixture([[video()]], config)
    await assert.rejects(f.attach(asset()), /取消/)
    assert.equal(f.uploads().length, 1)
    assert.equal(f.calls.puts.length, point === 'authorized' ? 0 : 1)
    assert.equal(f.calls.api.filter(call => call.url.endsWith('/complete')).length, point === 'completed' ? 1 : 0)
  })
}

test('旧 video / live_photo 入口仍可通过原上传链路', async () => {
  const single = fixture([[video()]])
  assert.equal((await single.legacy('video')).kind, 'video')
  assert.equal(single.calls.choose.length, 1)
  const live = fixture([[image('/original.jpg')], [video({ duration: 3 })]])
  const motion = await live.legacy('live_photo')
  assert.equal(motion.kind, 'live_photo')
  assert.equal(motion.poster.previewPath, '/original.jpg.compressed.webp')
  assert.equal(live.calls.choose.length, 2)
})

async function main() {
  let passed = 0
  for (const entry of cases) {
    try { await entry.run(); passed += 1 }
    catch (error) { console.error(`FAIL: ${entry.name}`); throw error }
  }
  console.log(`PASS: ${passed} 项帖子媒体选择测试（真实 media.ts；Taro/API 全桩化，无网络请求${baseline ? '；Git HEAD 基线' : ''}）。`)
}
main().catch(error => { console.error(error); process.exitCode = 1 })
