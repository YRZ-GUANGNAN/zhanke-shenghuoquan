const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const root = path.resolve(__dirname, '..')
const sourcePath = path.join(root, 'apps/miniapp/src/services/share-model.ts')
const compiled = ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } })
const loaded = new Module(sourcePath, module)
loaded._compile(compiled.outputText, sourcePath)
const { buildShareCard, DEFAULT_SHARE_SETTINGS: defaults, publicShareId, shareImage, parseShareSettings } = loaded.exports
for (const value of ['', '0', '01', '-1', '%', '%31', '1&token=secret', '1e3', 12]) assert.equal(publicShareId(value), '')
assert.equal(publicShareId('18446744073709551615'), '18446744073709551615')
for (const value of ['http://site.example/p.png', 'javascript:alert(1)', 'https://site.example/a.svg', 'https://user:secret@site.example/a.jpg', 'https://site.example/a.jpg#x']) assert.equal(shareImage(value), '')
const image = 'https://bucket.cos.ap-guangzhou.myqcloud.com/content.jpg'
const cover = 'https://zhanduiwang.top/cover.png'
const content = { id: '12', title: '真实内容', content: '真实正文', images: [image], reviewStatus: 'approved' }
const settings = { ...defaults, defaultImageUrl: cover }
for (const kind of ['post', 'product']) {
  assert.deepEqual(buildShareCard({ kind, id: '12', enabled: true, content }, settings), {
    title: '真实内容 · 湛科生活圈', path: `/pages/${kind === 'post' ? 'post-detail' : 'secondhand-detail'}/index?id=12`, imageUrl: image
  })
  for (const state of ['submitted', 'reviewing', 'rejected', 'taken_down', undefined]) {
    assert.equal(buildShareCard({ kind, id: '12', enabled: true, content: { ...content, reviewStatus: state } }, settings), null)
  }
  assert.equal(buildShareCard({ kind, id: '13', enabled: true, content }, settings), null)
  assert.equal(buildShareCard({ kind, id: '12', enabled: false, content }, settings), null)
  assert.equal(buildShareCard({ kind, id: '12', enabled: true, content }, { ...settings, preferContentImage: false }).imageUrl, cover)
  assert.equal(buildShareCard({ kind, id: '12', enabled: true, content: { ...content, images: [] } }, settings).imageUrl, cover)
}
assert.equal(buildShareCard({ kind: 'profile', enabled: true, id: '12', loadedId: '13', nickname: '同学' }, defaults), null)
assert.deepEqual(buildShareCard({ kind: 'home', enabled: true, token: 'secret', commentId: '5' }, defaults), { title: '湛科生活圈', path: '/pages/home/index' })
assert.equal(buildShareCard({ kind: 'home', enabled: false }, defaults), null)
assert.equal(parseShareSettings({ homeTitle: 'x' }), null)
assert.deepEqual(parseShareSettings(defaults), defaults)
const guide = { id: 'library-hours', title: '图书馆开放时间' }
assert.equal(buildShareCard({kind: 'guide', enabled: true, id: 'library-hours', content: guide}, defaults).path, '/pages/campus-guide/index?id=library-hours')
for (const id of ['', '../library-hours', 'library-hours?token=secret', '%6cibrary-hours', 'campus-addresses']) assert.equal(buildShareCard({kind: 'guide', enabled: true, id, content: guide}, defaults), null)
assert.equal(buildShareCard({kind: 'guide', enabled: true, id: guide.id, content: null}, defaults), null)
assert.equal(buildShareCard({kind: 'guide', enabled: false, id: guide.id, content: guide}, defaults), null)
for (const page of ['home', 'campus-guide', 'student-secondhand', 'student-takeout', 'post-detail', 'secondhand-detail', 'user-profile']) {
  const config = fs.readFileSync(path.join(root, `apps/miniapp/src/pages/${page}/index.config.ts`), 'utf8')
  assert.match(config, /enableShareAppMessage:\s*true/, `${page} native callback not enabled`)
}
console.log('分享模型检查通过：白名单路径、审核状态、真实媒体、封面回退、参数过滤及页面原生配置。')
