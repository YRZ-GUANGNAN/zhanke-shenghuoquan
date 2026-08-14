const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (value) => fs.readFileSync(path.join(root, value), 'utf8')

const chat = read('apps/miniapp/src/pages/secondhand-chat/index.tsx')
const chatStyle = read('apps/miniapp/src/pages/secondhand-chat/index.scss')
const market = read('apps/miniapp/src/pages/student-secondhand/index.tsx')
const marketStyle = read('apps/miniapp/src/pages/student-secondhand/index.scss')
const profile = read('apps/miniapp/src/pages/user-profile/index.tsx')
const profileStyle = read('apps/miniapp/src/pages/user-profile/index.scss')
const settings = read('apps/miniapp/src/pages/settings/index.tsx')
const settingsStyle = read('apps/miniapp/src/pages/settings/index.scss')

for (const fragment of [
  'contactSeller',
  'getIMConversation',
  'loadTencentMessages',
  'sendTencentText',
  'subscribeTencentMessages',
  'requestWithSingleSessionRecovery',
  'secondhand-chat-product__status',
]) assert.ok(chat.includes(fragment), `chat contract missing: ${fragment}`)

assert.doesNotMatch(
  chat,
  /图片消息暂未开放|secondhand-chat-composer__media/,
  'secondhand chat must only expose working composer actions',
)
assert.doesNotMatch(
  chatStyle,
  /secondhand-chat-composer__media|secondhand-chat-image-message/,
  'secondhand chat must not retain dead image-composer styles',
)
assert.match(
  chatStyle,
  /grid-template-columns:\s*1fr\s+92rpx;/,
  'secondhand composer must reserve two columns for text and send',
)
assert.match(
  chatStyle,
  /\.secondhand-chat-scroll__inner\s*\{[^}]*padding:\s*22rpx\s+30rpx\s+calc\(184rpx\s*\+\s*env\(safe-area-inset-bottom\)\);/s,
  'secondhand chat history must clear the fixed composer and iPhone safe area',
)
assert.ok(chatStyle.includes('.secondhand-chat-send--active'), 'active send state is required')

for (const fragment of [
  'getProductsPage',
  "status: 'selling'",
  '/pages/search/index?scope=secondhand',
  'onScrollToLower={loadMoreProducts}',
  'secondhand-card__category',
]) assert.ok(market.includes(fragment), `market contract missing: ${fragment}`)

assert.doesNotMatch(market, /ProductHeight/)
assert.doesNotMatch(marketStyle, /secondhand-card__media--(?:short|medium|tall)/)

for (const fragment of [
  'getPublicCommunityProfile',
  'getPublicUserProducts',
  'setUserFollowing',
  '/pages/secondhand-detail/index',
  'public-profile-identity',
]) assert.ok(profile.includes(fragment), `public profile contract missing: ${fragment}`)

assert.ok(profileStyle.includes('.public-profile-identity'), 'public profile identity style is required')
assert.doesNotMatch(profileStyle, /background:\s*\$brand-primary;/)

for (const fragment of [
  'getUserSettings',
  'updateUserSettings',
  'bindWechatPhone',
  "openType='getPhoneNumber'",
  '/pages/blacklist/index',
  '/pages/feedback/index',
  '/pages/about/index',
  'closeAccount',
  'student-settings-account',
]) assert.ok(settings.includes(fragment), `settings contract missing: ${fragment}`)

assert.ok(settingsStyle.includes('.student-settings-account'), 'settings account summary style is required')

console.log('secondhand and private messaging ui contract check PASS')
