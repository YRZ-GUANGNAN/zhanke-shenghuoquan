// 导航条目自定义图标：上传/托管/三端渲染闭环契约。
const fs = require('node:fs')
const path = require('node:path')

const root = path.join(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

function includes(source, value, label) {
  if (!source.includes(value)) throw new Error(`${label}: missing ${value}`)
}

const iconGo = read('backend', 'internal', 'module', 'navigation', 'icon.go')
const service = read('backend', 'internal', 'module', 'navigation', 'service.go')
const handler = read('backend', 'internal', 'module', 'navigation', 'handler.go')
const mainGo = read('backend', 'cmd', 'server', 'main.go')
const config = read('backend', 'internal', 'config', 'config.go')
const dNavigation = read('apps', 'miniapp', 'src', 'services', 'd-navigation.ts')
const toolDirectory = read('apps', 'miniapp', 'src', 'components', 'tool-directory.tsx')
const takeout = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.tsx')
const adminClient = read('apps', 'admin', 'src', 'services', 'f-admin.ts')
const adminSettings = read('apps', 'admin', 'src', 'preview', 'views', 'SettingsView.tsx')

// 后端：上传（管理员）、内容寻址存储、公开托管、条目绑定、三频道共用。
for (const fragment of ['RegisterAdminIcons', 'uploadItemIcon', 'deleteItemIcon', 'serveIcon']) {
  includes(iconGo, fragment, 'navigation icon handler')
}
includes(iconGo, 'sha256.Sum256', 'navigation icon handler')
includes(service, 'SetItemIconPath', 'navigation service')
includes(service, 'itemCategoryLocked', 'navigation service')
includes(handler, '/navigation-icons/:name', 'navigation handler')
includes(handler, 'decorateCategories', 'navigation handler')
includes(mainGo, 'WithIconStorage(cfg.NavigationIconDir, cfg.MediaPublicBaseURL)', 'main.go')
includes(config, 'NAVIGATION_ICON_DIR', 'config')

// 小程序：工具、众创、服务三类入口都渲染自定义图标并带失败回退。
includes(dNavigation, 'iconUrl:item.iconUrl', 'miniapp navigation client')
includes(dNavigation, 'iconUrl?:string', 'miniapp navigation client')
includes(toolDirectory, 'tool-entry__mark-image', 'tool directory')
includes(toolDirectory, 'failedIcons', 'tool directory')
includes(takeout, 'student-takeout-merchant__avatar-image', 'takeout page')
includes(takeout, 'failedMerchantIcons', 'takeout page')

// 后台：编辑弹窗内的图标上传与移除。
includes(adminClient, 'uploadNavigationItemIcon', 'admin client')
includes(adminClient, 'removeNavigationItemIcon', 'admin client')
for (const fragment of ['图标图片', 'entryIconField', 'uploadEntryIcon', 'removeEntryIcon']) {
  includes(adminSettings, fragment, 'admin settings view')
}

console.log('navigation icons contract: ok (upload, hosting, three surfaces)')
