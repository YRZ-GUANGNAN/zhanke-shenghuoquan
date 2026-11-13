const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = relative => fs.readFileSync(path.join(root, relative), 'utf8')
const failures = []
const main = read('backend/cmd/server/main.go')
const runtime = read('backend/cmd/server/user_notifications.go')
const im = read('backend/cmd/server/user_notification_im.go')
const templates = read('backend/cmd/server/user_notification_templates.go')
const notificationService = read('backend/internal/module/notification/service.go')
const wecomSender = read('backend/internal/module/notification/wecom_sender.go')
const officialService = read('backend/internal/module/notification/official_account.go')
const adminNotification = read('backend/internal/module/admin/notificationconsole/handler.go')
const adminNotificationService = read('apps/admin/src/services/delivery-admin.ts')
const adminNotificationConsole = read('apps/admin/src/NotificationConsole.tsx')
const callback = read('backend/internal/module/notification/im_handler.go')
const parser = read('backend/internal/module/notification/im_callback.go')
const delivery = fs.readdirSync(path.join(root, 'backend/internal/module/delivery'))
  .filter(name => name.endsWith('.go') && !name.endsWith('_test.go'))
  .map(name => read(`backend/internal/module/delivery/${name}`)).join('\n')

function requireText(source, pattern, message) {
  if (!pattern.test(source)) failures.push(message)
}

requireText(main, /notification\/tencent-im/, '未注册 Tencent IM 通知回调路由')
requireText(runtime, /financialNotificationRecipients/, '资金通知未接入通知展开器')
requireText(runtime, /privateNotificationRecipients/, '私信通知未接入通知展开器')
requireText(runtime, /wechatOfficial/, '微信服务号个人通知未接入工作线程')
requireText(runtime, /wecomSender/, '企业微信群通知未接入后台管理')
requireText(notificationService, /AllowOfficial/, '通知展开器未生成服务号渠道')
requireText(notificationService, /WithWecomGroupFormatter/, '通知展开器未生成企业微信群渠道')
requireText(templates, /orderOfficialAccountGuard/, '服务号发送前未复核订单资格')
requireText(wecomSender, /NewWeComGroupSender/, '企业微信群发送器未接入')
requireText(officialService, /OAuthURL/, '公众号 OAuth 绑定未接入')
requireText(adminNotification, /notification-channels/, '后台未注册通知通道只读接口')
requireText(adminNotification, /notification-channels\/wecom\/test/, '后台未注册企业微信群安全测试接口')
requireText(adminNotificationService, /notificationChannels:/, '后台前端未接入通知通道接口')
requireText(adminNotificationConsole, /testWecomGroup/, '后台通知运营页未接入企业微信群测试')
requireText(callback, /RecordCallbackAndEvent/, '私信回调未使用原子 receipt+event 写入')
requireText(parser, /discardCallbackValue/, '私信回调解析器未丢弃 MsgBody')
requireText(im, /KeyMessages/, '私信回调身份适配未复核消息功能开关')
requireText(im, /ReferenceType\s*!==?\s*['"]user['"]/, '私信回调未拒绝商品关系冒充用户关系')
requireText(im, /AllowWechat: false/, '私信通知在无离线信号时错误创建微信任务')
requireText(delivery, /appendFinancialNotificationTx/, 'delivery 资金事务未使用通知 appender')
if (/WECHAT_APP_SECRET|APPSECRET|API_KEY|PRIVATE_KEY|openid/i.test(read('apps/miniapp/src/services/notification.ts'))) {
  failures.push('小程序通知服务包含服务端敏感凭据字段')
}
if (/(?:appSecret|webhookUrl|openid)\s*[:=]/i.test(adminNotificationConsole)) {
  failures.push('后台通知运营页不得接收或展示服务端敏感凭据')
}

if (failures.length) {
  for (const failure of failures) console.error(`FAIL 通知合同：${failure}`)
  process.exitCode = 1
} else {
  console.log('PASS 通知合同：路由、原子回调、正文丢弃、资格复核、资金事务、敏感信息边界')
}
