const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const appearance = read('apps', 'miniapp', 'src', 'pages', 'appearance', 'index.tsx')
const home = read('apps', 'miniapp', 'src', 'pages', 'home', 'index.tsx')
const homeStyle = read('apps', 'miniapp', 'src', 'pages', 'home', 'index.scss')
const appearanceStyle = read('apps', 'miniapp', 'src', 'pages', 'appearance', 'index.scss')
const verify = read('apps', 'miniapp', 'src', 'pages', 'student-verify', 'index.tsx')
const verifyStyle = read('apps', 'miniapp', 'src', 'pages', 'student-verify', 'index.scss')
const editProfile = read('apps', 'miniapp', 'src', 'pages', 'edit-profile', 'index.tsx')
const appConfig = read('apps', 'miniapp', 'src', 'app.config.ts')

function includes(source, value, label) {
  if (!source.includes(value)) throw new Error(`${label}: missing ${value}`)
}

function excludes(source, value, label) {
  if (source.includes(value)) throw new Error(`${label}: forbidden ${value}`)
}

// 首页背景模块：本地偏好 + 可裁剪 + 效果预览；不包含学生验证内容。
for (const value of [
  'readHomeHeaderBackdrop', 'writeHomeHeaderBackdrop', 'saveHomeHeaderBackdropImage',
  'resetHomeHeaderBackdropImage', 'cropImageWithWechatEditor', '效果预览',
  'readHomeHeaderBackdropEffect', 'writeHomeHeaderBackdropEffect', '原图直显'
]) includes(appearance, value, 'appearance page')
excludes(appearance, 'readStudentVerification', 'appearance page')
excludes(appearance, '预留', 'appearance page')
for (const value of ['appearance-preview__blur', 'appearance-preview__veil', 'appearance-switch--on', 'appearance-preview__veil--plain', 'appearance-effect-option--active', 'appearance-effect-track']) {
  includes(appearanceStyle, value, 'appearance style')
}
// 首页必须按所选效果渲染：毛玻璃三层，或原图直显（仅底部淡出，顶部保留原图）。
includes(home, 'readHomeHeaderBackdropEffect', 'home header effect')
includes(home, 'student-home-header__veil--plain', 'home header effect')
includes(homeStyle, 'student-home-header__veil--plain', 'home header effect style')
if (homeStyle.includes('plain-glow')) throw new Error('home header effect: plain mode must not soften the top of the photo')

// 学生验证模块：本机保存、可裁剪、可清除；不触碰首页背景偏好。
for (const value of [
  'getStudentVerification', 'submitStudentVerification', 'requestStudentVerificationCancel',
  'uploadPickedImage', 'cropImageWithWechatEditor', 'requestCancel', '待审核', '已认证', '开始认证'
]) includes(verify, value, 'student verify page')
for (const value of ['readStudentVerification', 'writeStudentVerification', 'saveLocalFile', '保存到本机', '本机']) {
  excludes(verify, value, 'student verify page')
}
excludes(verify, 'readHomeHeaderBackdrop', 'student verify page')
excludes(verify, '预留', 'student verify page')
excludes(verify, 'next/v1', 'student verify page must stay local-only')
for (const value of ['student-verify-state--success', 'student-verify-thumb']) {
  includes(verifyStyle, value, 'student verify style')
}

// 编辑资料：两个独立入口，不残留模块内部逻辑。
includes(editProfile, '/pages/appearance/index', 'edit profile entries')
includes(editProfile, '/pages/student-verify/index', 'edit profile entries')
excludes(editProfile, 'readStudentVerification', 'edit profile')
excludes(editProfile, 'saveHomeHeaderBackdropImage', 'edit profile')
excludes(editProfile, '预留', 'edit profile')

// 路由注册在主包（避免 sub-common 分块在开发者工具下的加载问题）。
includes(appConfig, "'pages/appearance/index'", 'app config')
includes(appConfig, "'pages/student-verify/index'", 'app config')
excludes(appConfig, "root: 'pages/appearance'", 'app config')
excludes(appConfig, "root: 'pages/student-verify'", 'app config')

console.log('appearance & student-verify modules check PASS')
