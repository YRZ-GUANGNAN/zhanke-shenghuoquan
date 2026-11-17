const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const profilePage = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'pages', 'profile', 'index.tsx'),
  'utf8',
)
const profileStyle = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'pages', 'profile', 'index.scss'),
  'utf8',
)

assert.match(profilePage, /profile\.avatar/)
assert.match(profilePage, /mode='aspectFill'/)
assert.doesNotMatch(profilePage, /学校未设置/)
assert.doesNotMatch(profilePage, /student-profile-user__campus/)

for (const tab of ['favorites', 'posts', 'comments', 'items']) {
  assert.match(profilePage, new RegExp(`tab:\\s*'${tab}'`))
}

for (const route of [
  '/pages/edit-profile/index',
  '/pages/settings/index',
  '/pages/feedback/index',
  '/pages/about/index',
]) {
  assert.ok(profilePage.includes(route), `${route} must remain available`)
}
// 用户协议与隐私政策入口已收敛到「关于」页，不再要求在「我的」页重复出现。
const aboutPage = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'pages', 'about', 'index.tsx'),
  'utf8',
)
assert.ok(aboutPage.includes('/pages/legal/index'), 'about page must keep the legal entries')

for (const label of ['当前使用', '已通过', '审核中', '去申请', '需完善', '暂不可用']) {
  assert.ok(profilePage.includes(label), `${label} identity state is required`)
}

assert.match(profilePage, /身份与工作台/)
assert.match(profilePage, /账户与支持/)
assert.match(profilePage, /IdentitySwitchModal/)
assert.match(profilePage, /identity-application\/index\?role=/)

assert.doesNotMatch(profilePage, /student-profile-hero__orb/)
assert.doesNotMatch(profileStyle, /\$gradient-primary/)
assert.doesNotMatch(profileStyle, /student-profile-hero__orb/)
assert.doesNotMatch(profileStyle, /\.student-bottom-nav(?:--|__|\s*\{)/)

console.log('profile center ui: ok')
