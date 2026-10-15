const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')

async function run() {
  const filename = path.resolve(__dirname, '../apps/miniapp/src/services/media.ts')
  const mod = new Module(filename, module)
  let requests = 0, response
  mod.require = id => {
    if (id === './taro') return { default: {} }
    if (id === './api') return { request: async () => { requests++; return response } }
    throw new Error(`Unexpected dependency: ${id}`)
  }
  mod._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, filename)
  const { isMediaUsable, confirmDirectUpload, waitForMediaApproval } = mod.exports
  const ready = { assetId: 'asset-one', purpose: 'avatar', canUse: true, state: 'ready', reviewStatus: 'not_required' }
  for (const purpose of ['avatar', 'profile_cover', 'identity_application', 'dish', 'merchant_logo']) {
    const image = { ...ready, purpose }
    assert.equal(isMediaUsable(image), true)
    assert.deepEqual(await confirmDirectUpload(image), image)
  }
  assert.equal(requests, 0, 'ready profile uploads do not poll review')
  for (const purpose of ['post', 'product', 'delivery_im']) {
    const image = { ...ready, purpose }
    assert.equal(isMediaUsable(image), false)
    await assert.rejects(confirmDirectUpload(image), /仍需内容审核/)
  }
  assert.equal(isMediaUsable({ ...ready, canUse: false }), false)
  assert.equal(isMediaUsable({ ...ready, reviewStatus: 'pending' }), false)
  response = { ...ready, canUse: false, state: 'processing', message: '上传未完成' }
  await assert.rejects(confirmDirectUpload(response), /上传未完成/)
  response = { ...ready, purpose: 'identity_application' }
  await assert.rejects(confirmDirectUpload({ ...ready, canUse: false }), Error, 'cannot substitute a different purpose')
  response = { ...ready, purpose: 'post' }
  await assert.rejects(waitForMediaApproval('asset-one', undefined, 0), /仍在审核/)
  response = { ...ready, purpose: 'post', state: 'approved', reviewStatus: 'approved' }
  assert.deepEqual(await waitForMediaApproval('asset-one', undefined, 0), response)
  await assert.rejects(confirmDirectUpload(ready, () => false), /取消/)
  console.log('media purpose policy: no profile polling, content review retained, invalid/mismatched states rejected and cancellation respected')
}

run().catch(error => { console.error(error); process.exitCode = 1 })
