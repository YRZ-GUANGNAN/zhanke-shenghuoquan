const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const takeout = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.tsx')
const takeoutStyle = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.scss')
const profile = read('apps', 'miniapp', 'src', 'pages', 'profile', 'index.tsx')
const profileStyle = read('apps', 'miniapp', 'src', 'pages', 'profile', 'index.scss')
const editProfile = read('apps', 'miniapp', 'src', 'pages', 'edit-profile', 'index.tsx')
const userClient = read('apps', 'miniapp', 'src', 'services', 'a-user.ts')
const mediaClient = read('apps', 'miniapp', 'src', 'services', 'media.ts')
const userHandler = read('backend', 'internal', 'module', 'user', 'handler.go')

for (const required of [
  'listExternalTakeoutMerchants',
  '/pages/search/index?scope=takeout',
  'student-takeout-directory',
  'student-takeout-state__visual',
]) {
  assert.ok(takeout.includes(required), `student takeout entry is missing ${required}`)
}

for (const forbidden of ['蜜雪冰城', '华莱士', '塔斯汀', 'const merchants']) {
  assert.ok(!takeout.includes(forbidden), `student takeout must not contain fabricated merchant ${forbidden}`)
}

assert.match(takeoutStyle, /\.student-takeout-directory/)
assert.match(takeoutStyle, /\.student-takeout-state__visual/)

for (const required of [
  "chooseAndUploadImages('avatar'",
  'waitForMediaApproval',
  'updateProfile',
  'currentUser',
  'avatarAssetId',
]) {
  assert.ok(
    editProfile.includes(required) || userClient.includes(required) || userHandler.includes(required),
    `avatar pipeline is missing ${required}`,
  )
}

assert.match(editProfile, /const refreshed = await currentUser\(\)/)
assert.match(editProfile, /setChoosingAvatar\(true\)/)
assert.match(mediaClient, /purpose:\s*MediaPurpose/)
assert.match(userHandler, /media\.PurposeAvatar/)
assert.match(userHandler, /h\.media\.Commit/)

assert.match(profile, /profile\.avatar/)
assert.match(profile, /student-profile-user__identity/)
assert.match(profile, /student-profile-user__details/)
assert.match(profileStyle, /border-radius:\s*50%/)

console.log('student entry and avatar contract: ok')
