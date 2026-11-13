const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (value) => fs.readFileSync(path.join(root, value), 'utf8')
const settings = read('apps/miniapp/src/pages/settings/index.tsx')
const blacklist = read('apps/miniapp/src/pages/blacklist/index.tsx')
const profile = read('apps/miniapp/src/pages/profile/index.tsx')
const appConfig = read('apps/miniapp/src/app.config.ts')
const userClient = read('apps/miniapp/src/services/a-user.ts')
const mediaClient = read('apps/miniapp/src/services/media.ts')
const notificationClient = read('apps/miniapp/src/services/notification.ts')
const userHandler = read('backend/internal/module/user/handler.go')

const requirements = [
  [settings, 'getUserSettings'],
  [settings, 'updateUserSettings'],
  [settings, 'bindWechatPhone'],
  [settings, "openType='getPhoneNumber'"],
  [settings, 'getUpdateManager'],
  [settings, 'closeAccount'],
  [settings, 'loadNotificationSubscriptionTemplates'],
  [settings, 'subscriptionRefreshSequence'],
  [settings, '可用 ${template.remainingQuota} 次'],
  [notificationClient, 'remainingQuota'],
  [notificationClient, 'reservedQuota'],
  [blacklist, 'listBlockedUsers'],
  [blacklist, 'removeBlockedUser'],
  [profile, '/pages/about/index'],
  [profile, '/pages/feedback/index'],
  [appConfig, 'pages/about/index'],
  [appConfig, 'pages/feedback/index'],
  [userClient, "request('/auth/settings'"],
  [userClient, "request('/auth/me', { url: '', method: 'DELETE'"],
  [mediaClient, "'feedback'"],
  [userHandler, 'rg.GET("/settings"'],
  [userHandler, 'rg.DELETE("/me"'],
]

for (const [source, fragment] of requirements) {
  if (!source.includes(fragment)) throw new Error(`settings contract missing: ${fragment}`)
}

// 手机号不是独立登录方式，界面只能说明绑定后的实际用途。
for (const fragment of ['wechatBound', '微信登录', '绑定后可用于', '不影响微信登录']) {
  if (!settings.includes(fragment) && !userClient.includes(fragment)) throw new Error(`account binding presentation missing: ${fragment}`)
}

for (const fragment of ["visible={closeStage === 'first'}", "visible={closeStage === 'final'}", "visible={closeStage === 'blocked'}"]) {
  if (!settings.includes(fragment)) throw new Error('account closure must keep two confirmations plus a blocked-state dialog')
}

for (const forbidden of ['159****8834', '12.6MB', 'v2.4.0', 'initialBlockedUsers']) {
  if (settings.includes(forbidden) || blacklist.includes(forbidden)) {
    throw new Error(`static settings placeholder is forbidden: ${forbidden}`)
  }
}

console.log('miniapp settings and account closure contract check PASS')
