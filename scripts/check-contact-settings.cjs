// 联系设置（众创申请弹窗内容）：后台可改、小程序读取、后端公开读与管理员写。
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

function includes(source, value, label) {
  if (!source.includes(value)) throw new Error(`${label}: missing ${value}`)
}

const mainGo = read('backend', 'cmd', 'server', 'main.go')
const handler = read('backend', 'internal', 'module', 'contactsettings', 'handler.go')
const client = read('apps', 'miniapp', 'src', 'services', 'contact-settings.ts')
const profile = read('apps', 'miniapp', 'src', 'pages', 'profile', 'index.tsx')
const adminSettings = read('apps', 'admin', 'src', 'preview', 'views', 'SettingsView.tsx')
const adminClient = read('apps', 'admin', 'src', 'services', 'f-admin.ts')

// 后端：公开读 + 管理员写，模块在入口接线。
for (const fragment of ['contactsettings.NewService', 'contactsettings.NewHandler']) {
  includes(mainGo, fragment, 'main.go')
}
for (const fragment of ['api.GET("/contact-settings"', '"/admin/contact-settings"']) {
  includes(handler, fragment, 'contact settings handler')
}

// 小程序：弹窗内容来自接口，不得再硬编码微信号。
for (const fragment of ["'/contact-settings'", 'getContactSettings']) {
  includes(client, fragment, 'miniapp contact settings client')
}
for (const fragment of ['getContactSettings', 'maker-contact__wechat', 'copyMakerContact', 'makerIntro']) {
  includes(profile, fragment, 'profile maker dialog')
}
for (const forbidden of ['MAKER_CONTACT_WECHAT', 'zhanke-zhongchuang']) {
  if (profile.includes(forbidden)) throw new Error(`profile must not hardcode contact info: ${forbidden}`)
}

// 后台：接口与编辑面板。
for (const fragment of ['contactSettings:', 'updateContactSettings:']) {
  includes(adminClient, fragment, 'admin contact settings client')
}
for (const fragment of ['MAKER CONTACT', '众创申请弹窗', '弹窗文案', '联系微信号']) {
  includes(adminSettings, fragment, 'admin contact settings panel')
}

console.log('contact settings contract: ok (admin editable maker dialog)')
