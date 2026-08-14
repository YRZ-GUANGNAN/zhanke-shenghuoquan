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
const appIconSource = fs.readFileSync(
  path.join(root, 'apps', 'miniapp', 'src', 'components', 'app-icon.tsx'),
  'utf8',
)

const supportIcons = {
  settings: 'actionSettings',
  document: 'actionDocument',
  feedback: 'actionFeedback',
  info: 'actionInfo',
}

assert.match(profilePage, /<AppIcon name=\{item\.icon\} className='student-profile-setting__icon-image' \/>/)

for (const [name, importName] of Object.entries(supportIcons)) {
  const assetName = `action-${name}.svg`
  const svg = fs.readFileSync(
    path.join(root, 'apps', 'miniapp', 'src', 'assets', 'icons', assetName),
    'utf8',
  )

  assert.match(profilePage, new RegExp(`icon:\\s*'${name}'`), `${name} must remain mapped in profile settings`)
  assert.match(appIconSource, new RegExp(`import ${importName} from '../assets/icons/${assetName}'`))
  assert.match(appIconSource, new RegExp(`${name}: \\{ normal: ${importName}, active: ${importName} \\}`))
  assert.match(svg, /viewBox="0 0 48 48"/, `${assetName} must use the shared action icon grid`)
  assert.match(svg, /stroke-width="3\.4"/, `${assetName} must use the shared support icon stroke`)
  assert.match(svg, /stroke-linecap="round"/, `${assetName} must use rounded stroke caps`)
  assert.match(svg, /stroke-linejoin="round"/, `${assetName} must use rounded stroke joins`)
  assert.doesNotMatch(svg, /<(?:text|image|use)\b/i, `${assetName} must remain a self-contained vector icon`)
}

assert.match(profilePage, /profile\.avatar/)
assert.match(profilePage, /mode='aspectFill'/)
assert.match(profilePage, /学校未设置/)
assert.match(profilePage, /student-profile-user__campus-row/)
assert.match(profilePage, /\{campusLabel\}/)
assert.doesNotMatch(profilePage, /student-profile-user__campus-dot/)
assert.doesNotMatch(profileStyle, /student-profile-user__campus-dot/)

for (const tab of ['favorites', 'posts', 'comments', 'items']) {
  assert.match(profilePage, new RegExp(`tab:\\s*'${tab}'`))
}

for (const route of [
  '/pages/edit-profile/index',
  '/pages/settings/index',
  '/pages/legal/index',
  '/pages/feedback/index',
  '/pages/about/index',
]) {
  assert.ok(profilePage.includes(route), `${route} must remain available`)
}

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
