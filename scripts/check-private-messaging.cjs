const fs = require('fs')
const path = require('path')

const root = path.resolve(__dirname, '..')
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8')
const home = read('apps/miniapp/src/pages/home/index.tsx')
const chat = read('apps/miniapp/src/pages/secondhand-chat/index.tsx')
const productDetail = read('apps/miniapp/src/pages/product-detail/index.tsx')
const conversations = read('apps/miniapp/src/pages/conversations/index.tsx')

const checks = [
  ['首页会话入口使用现行单聊页', home.includes('/pages/secondhand-chat/index?relationId=')],
  ['商品详情统一使用现行单聊页', productDetail.includes('/pages/secondhand-chat/index?relationId=') && !productDetail.includes('/pages/chat/index')],
  ['旧会话列表统一使用现行单聊页', conversations.includes('/pages/secondhand-chat/index?relationId=') && !conversations.includes('/pages/chat/index')],
  ['单聊使用多行输入控件', chat.includes("import { Image, ScrollView, Text, Textarea, View }") && chat.includes('<Textarea')],
  ['返回先处理键盘输入层', chat.includes('hideKeyboard') && chat.includes('inputFocused')],
  ['未知消息安全降级为可见占位', chat.includes('messageDisplayText') && chat.includes('暂不支持的消息类型')],
  ['消息按稳定 ID 渲染', chat.includes('key={message.ID}')],
  ['消息按日期分组', chat.includes('secondhand-chat-day-divider') && chat.includes('dayKey(')],
  ['会话列表不暴露内部标识', home.includes('二手交易') && !home.includes('交易会话 #') && !home.includes('对方买家 #') && !home.includes('对方卖家 #')]
]

const failed = checks.filter(([, passed]) => !passed)
if (failed.length) {
  for (const [label] of failed) console.error(`FAIL ${label}`)
  process.exit(1)
}
for (const [label] of checks) console.log(`PASS ${label}`)
