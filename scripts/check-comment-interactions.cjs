const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const file = path.resolve(__dirname, '../apps/miniapp/src/services/comment-session.ts')
const loaded = new Module(file, module)
loaded._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2017 },
}).outputText, file)
const { CommentSession } = loaded.exports

const root = (id = '1', extra = {}) => ({ id, authorId: '2', author: '作者', avatar: '', content: `评论 ${id}`, createdAt: '', parentCommentId: '0', replyToCommentId: '0', replyToUserId: '0', replyToAuthor: '', reviewStatus: 'approved', liked: false, likedCount: 0, replyCount: 1, ...extra })
const reply = (id = '2', extra = {}) => root(id, { parentCommentId: '1', replyToCommentId: '1', replyToUserId: '2', replyToAuthor: '作者', replyCount: 0, ...extra })

async function run() {
  const calls = { roots: [], replies: [], submits: [], likes: [] }
  let submit = null
  let likeCount = 0
  const api = {
    roots: async (_post, page) => { calls.roots.push(page); return { items: page === 1 ? [root()] : [], page, pageSize: 20, total: 1, publicCount: 1 } },
    replies: async (_post, rootId, page) => { calls.replies.push([rootId, page]); return { items: page === 1 ? [reply()] : [], page, pageSize: 20, total: 1, publicCount: 1 } },
    submit: async (_post, content, key, target) => { calls.submits.push({ content, key, target }); if (submit) return submit(); return { ...reply('3', { content, reviewStatus: 'submitted' }), parentCommentId: target ? '1' : '0', replyToCommentId: target || '0' } },
    like: async (_post, id, liked) => { calls.likes.push({ id, liked }); likeCount += liked ? 1 : -1; return { liked, likedCount: likeCount } },
    key: () => '01J00000000000000000000001',
  }
  const session = new CommentSession('post-1', { id: '7', nickname: '本人', avatar: '' }, api)
  await session.load()
  assert.deepEqual(calls.roots, [1])
  assert.equal(session.snapshot().publicCount, 1)
  await session.expand('1')
  assert.deepEqual(calls.replies, [['1', 1]])
  await session.like('2')
  assert.deepEqual(calls.likes, [{ id: '2', liked: true }])
  await session.like('2')
  assert.deepEqual(calls.likes, [{ id: '2', liked: true }, { id: '2', liked: false }])

  session.replyTo(session.snapshot().roots.items[0])
  session.setDraft('保留的回复')
  let reject
  submit = () => new Promise((resolve, decline) => { reject = decline })
  const failed = session.submit()
  await Promise.resolve()
  reject(new Error('offline'))
  await failed
  assert.equal(session.snapshot().draft, '保留的回复')
  assert.equal(session.snapshot().target.id, '1')
  assert.equal(calls.submits.length, 1)
  submit = null
  await session.submit()
  assert.equal(calls.submits.length, 2)
  assert.equal(calls.submits[0].key, calls.submits[1].key)
  assert.equal(session.snapshot().draft, '')
  assert.equal(session.snapshot().target, null)
  session.dispose()
  console.log('comment interaction session: PASS (thread paging, independent like confirmation, draft/target/idempotency retry)')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
