const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

const sourcePath = path.resolve(__dirname, '../apps/miniapp/src/utils/post-presentation.ts')
const loaded = new Module(sourcePath, module)
loaded._compile(ts.transpileModule(fs.readFileSync(sourcePath, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText, sourcePath)
const { preparePostMedia, postMediaHeight, postBodyWithoutTitle, formatPostDate } = loaded.exports

async function main() {
  let finishFirst
  const calls = []
  const result = preparePostMedia(['first', '', ' second ', 'broken', 'first', 'invalid'], (src) => {
    calls.push(src)
    if (src === 'first') return new Promise((resolve) => { finishFirst = resolve })
    if (src === 'second') return Promise.resolve({ width: 1200, height: 600 })
    if (src === 'broken') return Promise.reject(new Error('unavailable'))
    return Promise.resolve({ width: 0, height: 600 })
  })
  finishFirst({ width: 600, height: 800 })
  assert.deepEqual((await result).map((item) => item.src), ['first', 'second'], '异步图片请求完成顺序不能改变帖子附件顺序')
  assert.deepEqual(calls, ['first', 'second', 'broken', 'invalid'], '空地址不发请求，重复附件只校验一次')
  assert.deepEqual(await preparePostMedia([], () => assert.fail('no media request expected')), [])
  assert.deepEqual(await preparePostMedia(['offline'], async () => { throw new Error('offline') }), [])
  assert.deepEqual(await preparePostMedia(['bad'], async () => ({ width: Infinity, height: 50 })), [])
  assert.equal(postMediaHeight({ width: 600, height: 600 }), 750)
  assert.equal(postMediaHeight({ width: 100, height: 5000 }), 820, '长截图不撑满正文滚动区')
  assert.equal(postMediaHeight({ width: 5000, height: 100 }), 360, '横图仍有受控查看区域')
  assert.equal(postMediaHeight({ width: 0, height: 10 }), 560)
  assert.equal(postMediaHeight(), 560)
  assert.equal(postBodyWithoutTitle('第一行', '第一行\r\n\r\n正文\n第二段'), '正文\n第二段')
  assert.equal(postBodyWithoutTitle('一条内容', '一条内容'), '')
  assert.equal(postBodyWithoutTitle('前缀', '前缀后面还有内容'), '前缀后面还有内容', '不能把仅匹配前缀的正文截掉')
  assert.equal(postBodyWithoutTitle('', '正文'), '正文')
  assert.equal(formatPostDate(''), '')
  assert.equal(formatPostDate('刚刚'), '刚刚', '无法解析的既有文案原样保留')
  const date = new Date(2026, 9, 14, 9, 5)
  assert.equal(formatPostDate(date.toISOString()), '2026.10.14 09:05')
  console.log('post presentation behavior: PASS (17 assertions)')
}

main().catch((error) => { console.error(error); process.exitCode = 1 })
