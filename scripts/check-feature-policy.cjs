const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')
const root = path.resolve(__dirname, '..')
const source = fs.readFileSync(path.join(root, 'apps/miniapp/src/services/feature-policy-model.ts'), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2017 } }).outputText
const moduleExports = {}
new Function('exports', compiled)(moduleExports)
const { FEATURE_KEYS, CLOSED_VISIBILITY, parseFeatureVisibility, featurePageKeys, featureRequestKeys } = moduleExports
const flags = Object.fromEntries(FEATURE_KEYS.map(key => [key, true]))
const parse = flags => parseFeatureVisibility({ flags, enabled: true, rowVersion: 3, postingMode: 'admin_only' })
assert.equal(FEATURE_KEYS.length, 20)
assert.equal(parse(flags).available, true)
assert.equal(parse({ ...flags, forum: false }).post, false)
assert.equal(parse({ ...flags, forum: false }).secondhand_comments, true)
assert.equal(parse({ ...flags, publish: false }).secondhand_publish, false)
assert.equal(parse({ ...flags, messages: false }).secondhand_chat, false)
assert.equal(parse({ ...flags, takeout: false }).campus_tools, true)
assert.deepEqual(parseFeatureVisibility({ flags: { forum: true }, enabled: true, rowVersion: 1 }), CLOSED_VISIBILITY)
assert.deepEqual(parseFeatureVisibility(null), CLOSED_VISIBILITY)
for (const key of FEATURE_KEYS) {
  const value = parse({ ...flags, [key]: false })
  assert.equal(value[key], false, `${key} unexpectedly reopened`)
  const booleanMismatch = { ...flags, [key]: 1 }
  assert.equal(parse(booleanMismatch).available, false, `${key} accepted non-boolean`)
}
for (const [route, key] of [['post-detail', 'forum'], ['secondhand-detail', 'secondhand'], ['publish-post', 'post'], ['publish-secondhand', 'secondhand_publish'], ['secondhand-chat', 'secondhand_chat'], ['notifications', 'notifications'], ['merchant-publish', 'merchant']]) assert.ok(featurePageKeys(`pages/${route}/index`).includes(key))
assert.deepEqual(featurePageKeys('pages/my-content/index', { tab: 'favorites' }), [], 'initial closed tab must not block a different available content channel')
for (const [method, url, key] of [['GET','/community/posts/8/comments','comment'], ['PUT','/auth/identity-applications/rider','identity_application'], ['POST','/delivery/merchant/orders','delivery'], ['POST','/delivery/rider/orders/8/claim','rider'], ['PUT','/trade/products/8/contact','secondhand_chat'], ['GET','/trade/search/hot','search']]) assert.ok(featureRequestKeys(method,url).includes(key))
for (const [method, url] of [['GET','/auth/me'], ['GET','/delivery/im-credentials'], ['GET','/delivery/merchant/orders'], ['PATCH','/delivery/rider/orders/8/delivered'], ['POST','/delivery/merchant/orders/8/refunds'], ['GET','/delivery/merchant/wallet']]) assert.deepEqual(featureRequestKeys(method,url), [], `${url} must preserve necessary existing-order work`)
const backend = fs.readFileSync(path.join(root, 'backend/internal/module/featureflags/service.go'), 'utf8')
const admin = fs.readFileSync(path.join(root, 'apps/admin/src/types.ts'), 'utf8')
for (const key of FEATURE_KEYS) { assert.ok(backend.includes(`"${key}"`), `backend missing ${key}`); assert.ok(admin.includes(`'${key}'`), `Admin missing ${key}`) }
console.log('Feature policy: 20 modules, dependencies, invalid responses, routes and existing-order exceptions PASS')
