const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...segments) => fs.readFileSync(path.join(root, ...segments), 'utf8')

const publishPost = read('apps', 'miniapp', 'src', 'pages', 'publish-post', 'index.tsx')
const secondhandChat = read('apps', 'miniapp', 'src', 'pages', 'secondhand-chat', 'index.tsx')
const secondhandDetail = read('apps', 'miniapp', 'src', 'pages', 'secondhand-detail', 'index.tsx')
const settings = read('apps', 'miniapp', 'src', 'pages', 'settings', 'index.tsx')
const deliveryStaticDataPath = path.join(root, 'apps', 'miniapp', 'src', 'pages', 'delivery-static-data.ts')

assert.doesNotMatch(publishPost, /FALLBACK_TAGS/, 'publish page must not fall back to fabricated hot tags')
assert.match(publishPost, /useState<string\[\]>\(\[\]\)/, 'hot tags must start empty until the real aggregation endpoint responds')
assert.match(publishPost, /listHotCommunityTags\(/, 'publish page must load hot tags from the real aggregation endpoint')
assert.doesNotMatch(secondhandChat, /二手私信图片发送尚未开放|secondhand-chat-image-button/, 'secondhand chat must not expose a non-functional image action')
assert.doesNotMatch(secondhandDetail, /卖家未提供微信联系方式|加微信联系/, 'product contact sheet must not expose a contact method without real data')
assert.doesNotMatch(settings, /当前没有可安全清理的缓存|>清理缓存</, 'settings must not expose a cache action that performs no work')
assert.equal(fs.existsSync(deliveryStaticDataPath), false, 'unreferenced static delivery fixtures must be removed')

console.log('miniapp no-fake-pages contract: ok')
