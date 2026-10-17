const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), 'utf8')
}

function requireMatch(source, pattern, message) {
  if (!pattern.test(source)) throw new Error(message)
}

function forbidMatch(source, pattern, message) {
  if (pattern.test(source)) throw new Error(message)
}

const home = read('apps/miniapp/src/pages/home/index.tsx')
requireMatch(home, /import\s*\{[^}]*ScrollView[^}]*\}\s*from\s*'@tarojs\/components'/s, '首页必须使用原生 ScrollView')
requireMatch(home, /<ScrollView[\s\S]*scrollY[\s\S]*onScrollToLower=/, '首页必须通过 onScrollToLower 触发翻页')
forbidMatch(home, /createIntersectionObserver|home-discover-sentinel/, '首页不得继续使用不稳定的 IntersectionObserver sentinel')
requireMatch(home, /conversationOwnerRef\.current\s*!==\s*ownerKey/, '私信列表必须拒绝旧账号请求写回')
requireMatch(home, /key=\{channel\}/, '每个首页频道保留独立 ScrollView')
requireMatch(home, /scrollTop=\{tabScrollPositions\.current\[channel\]\}/, '首页标签切换时必须独立保存滚动位置')

const chatPage = read('apps/miniapp/src/pages/secondhand-chat/index.tsx')
requireMatch(chatPage, /onClick=\{openPeerProfile\}[\s\S]{0,100}<Text>主页<\/Text>/, '聊天页必须保留明确的对方主页入口')
requireMatch(chatPage, /`campus_\$\{currentUserId\}`/, '聊天页必须绑定当前后端用户对应的 Tencent IM 账号')
requireMatch(chatPage, /activeChatKeyRef\.current\s*!==\s*operationKey/, '聊天发送完成后必须拒绝旧账号结果写回')

forbidMatch(
  chatPage,
  /className='secondhand-chat-scroll'[\s\S]{0,180}marginBottom:\s*`\$\{keyboardHeight\}px`/,
  '聊天滚动区不得再次按键盘高度缩小，否则真机会发生重复补偿'
)
requireMatch(
  chatPage,
  /className='secondhand-chat-page'[\s\S]{0,100}paddingBottom:\s*`\$\{keyboardHeight\}px`/,
  '聊天页面应由 flex 布局统一避让键盘，输入栏与消息滚动区共享剩余空间'
)
const chatStyle = read('apps/miniapp/src/pages/secondhand-chat/index.scss')
requireMatch(chatStyle, /\.secondhand-chat-scroll\s*\{[^}]*flex:\s*1 1 0;[^}]*height:\s*0;/, '聊天滚动区必须收缩以让出输入区与键盘空间')
forbidMatch(chatStyle, /\.secondhand-chat-inputbar\s*\{[^}]*(position:\s*fixed|bottom:)/, '聊天输入区必须参与布局，不能重复固定定位')
requireMatch(chatPage, /adjustPosition=\{false\}/, '聊天输入框必须关闭系统自动顶起，避免与手动键盘适配冲突')

const sessionStore = read('apps/miniapp/src/stores/session.ts')
const userService = read('apps/miniapp/src/services/a-user.ts')
const apiServiceReliability = read('apps/miniapp/src/services/api.ts')
requireMatch(sessionStore, /let sessionEpoch = 0/, '登录状态必须维护会话代次')
requireMatch(sessionStore, /const isCurrent = \(\) => sessionEpoch === epoch/, 'bootstrap 异步结果必须校验会话代次')
requireMatch(sessionStore, /setToken:[\s\S]*profile: emptyProfile[\s\S]*identity: null/, '更换 token 时不得保留旧用户资料')

requireMatch(userService, /wechatLogin[\s\S]{0,480}timeout:\s*12000/, '微信登录请求必须使用 12 秒超时')
requireMatch(userService, /currentUser[\s\S]{0,240}timeout:\s*12000/, '会话恢复请求必须使用 12 秒超时')
requireMatch(userService, /currentIdentity[\s\S]{0,240}timeout:\s*12000/, '身份恢复请求必须使用 12 秒超时')
requireMatch(apiServiceReliability, /export function isRetryableNetworkError\(/, 'API 层必须统一识别可重试网络错误')
requireMatch(sessionStore, /async function retryNetworkOnce</, '会话恢复必须提供一次性网络重试')
requireMatch(sessionStore, /if \(!isRetryableNetworkError\(error\)\) throw error[\s\S]{0,220}return action\(\)/, '会话恢复只能对网络错误额外重试一次')
for (const action of ['currentUser', 'currentIdentity']) {
  requireMatch(sessionStore, new RegExp(`retryNetworkOnce\\([^\\n]*${action}`), `会话流程必须为 ${action} 提供网络重试`)
}
requireMatch(sessionStore, /const session = await wechatLogin\(loginResult\.code\)/, '微信 code 交换失败必须返回上层重新获取 code')
forbidMatch(sessionStore, /retryNetworkOnce\(\(\) => wechatLogin\(loginResult\.code\)\)/, '同一个微信 code 不得重复交换')

const imService = read('apps/miniapp/src/services/e-im.ts')
requireMatch(imService, /function\s+withTimeout\s*</, 'Tencent IM 必须提供统一的 Promise 超时边界')
requireMatch(
  imService,
  /if\s*\(loginTask\)[\s\S]*loginTask\.generation\s*===\s*generation[\s\S]*loginTask\.userId\s*===\s*credentials\.userId[\s\S]*return loginTask\.promise/,
  'Tencent IM 登录必须只复用同账号、同会话代次的进行中任务'
)
requireMatch(
  imService,
  /loginTask\s*=\s*\{\s*userId:\s*credentials\.userId,\s*generation,\s*promise\s*\}/,
  'Tencent IM 登录任务必须记录账号与会话代次'
)
requireMatch(imService, /await resetTask[\s\S]*const generation = sessionGeneration/, 'Tencent IM 新登录必须等待旧连接释放')
requireMatch(
  imService,
  /nextReqMessageID[\s\S]*TENCENT_IM_HISTORY_PAGE_LIMIT/,
  'Tencent IM 历史消息必须支持有上限的分页读取'
)
for (const call of ['getConversationProfile', 'getMessageList', 'setMessageRead']) {
  const callIndex = imService.indexOf(call)
  const wrapperIndex = imService.lastIndexOf('withTimeout(', callIndex)
  if (callIndex < 0 || wrapperIndex < 0 || callIndex - wrapperIndex > 180) {
    throw new Error(`${call} 必须受超时保护`)
  }
}

const postDetail = read('apps/miniapp/src/pages/post-detail/index.tsx')
const postComments = read('apps/miniapp/src/pages/post-detail/post-comments.tsx')
const commentSession = read('apps/miniapp/src/services/comment-session.ts')
const myContent = read('apps/miniapp/src/pages/my-content/index.tsx')
const myContentStyle = read('apps/miniapp/src/pages/my-content/index.scss')
const postDetailStyle = read('apps/miniapp/src/pages/post-detail/index.scss')
const devConfig = read('apps/miniapp/config/dev.ts')
const prodConfig = read('apps/miniapp/config/prod.ts')
const apiService = read('apps/miniapp/src/services/api.ts')
for (const fakeText of ['奶茶脑袋', '三食堂新出的芋泥波波', '预览模式', '本地预览内容', 'DEMO_POST', 'DEMO_COMMENTS']) {
  forbidMatch(postDetail, new RegExp(fakeText), `帖子详情不得包含旧假数据：${fakeText}`)
}
requireMatch(postDetail, /getMyCommunityPost\(postId\)/, '待审核帖子必须从本人真实帖子接口读取')
for (const [source, label] of [
  [devConfig, '开发配置'],
  [prodConfig, '生产配置'],
  [apiService, 'API 默认配置']
]) {
  requireMatch(source, /https:\/\/zhanduiwang\.top\/api\/v1/, `${label}必须默认使用真机可访问的 HTTPS 接口`)
  forbidMatch(source, /127\.0\.0\.1:8080/, `${label}不得默认指向真机自身的 localhost`)
}
requireMatch(postDetail, /帖子读取失败[\s\S]*重新读取/, '帖子接口失败时必须显示可重试错误态')

for (const className of [
  'my-content-row',
  'my-content-row__main',
  'my-content-row__title',
  'my-content-row__meta',
  'my-content-row__status'
]) {
  requireMatch(myContent, new RegExp(`className=['"\`][^'"\`]*${className}`), `我的内容 JSX 必须使用 ${className}`)
  requireMatch(myContentStyle, new RegExp(`\\.${className}(?:[\\s,{.:]|$)`), `我的内容样式必须覆盖 ${className}`)
}
requireMatch(myContent, /my-content-thumb--empty/, '无图片内容必须使用紧凑无图样式')
requireMatch(myContent, /label\s*=\s*'无图'/, '无图片内容必须显示真实无图文案')
requireMatch(postDetail, /onError=\{\(\) => setGallery\([\s\S]*removeFailedPostMedia\([\s\S]*image\.src/, '附件渲染失败时必须按附件身份移除失效图片并保留其他附件')
requireMatch(postDetail, /removeFailedPostMedia/, '帖子详情必须使用稳定的图集失败状态处理')
requireMatch(postDetail, /preparePostMedia\(images,\s*\(src\) => Taro\.getImageInfo\(\{ src \}\)\)/, '帖子全部附件展示前必须校验媒体资源可用')
requireMatch(postDetail, /showHero\s*\?[\s\S]*post-detail-hero/, '帖子详情必须只在存在有效附件时展示图集')
forbidMatch(postDetailStyle, /post-detail-hero[\s\S]{0,240}background:\s*\$like-active/, '帖子媒体区不得恢复粉色占位背景')
requireMatch(postDetail, /post-detail-follow-hitbox/, '帖子关注按钮必须使用独立点击热区')
requireMatch(postComments, /post-detail-comments__empty[\s\S]*还没有评论[\s\S]*说说你的看法吧/, '无评论状态必须使用紧凑语义空态')
requireMatch(postDetailStyle, /\.post-detail-follow-hitbox[\s\S]*width:\s*116rpx[\s\S]*height:\s*88rpx/, '帖子关注点击热区必须保持 58×44 逻辑像素')
requireMatch(postDetailStyle, /\.post-detail-follow\s*\{[\s\S]*width:\s*116rpx[\s\S]*height:\s*64rpx/, '帖子关注按钮视觉尺寸必须收敛为 58×32 逻辑像素')
requireMatch(postComments, /post-detail-comments__empty/, '评论组件必须提供结构化空态')
forbidMatch(postDetailStyle, /\.post-detail-follow\s*\{[\s\S]{0,160}width:\s*72px[\s\S]{0,160}height:\s*44px/, '帖子关注按钮不得恢复为会被 Taro 二次缩小的 px 写法')
requireMatch(postDetailStyle, /\.post-detail-action\s*\{[\s\S]*flex-direction:\s*column/, '帖子底部统计动作必须纵向排列')

const communityService = read('apps/miniapp/src/services/b-community.ts')
requireMatch(communityService, /submitThreadComment\(/, '必须提供楼内回复请求方法')
requireMatch(communityService, /data:\s*\{\s*content,\s*clientRequestId\s*\}[\s\S]{0,120}Idempotency-Key/, '楼内回复必须同时发送 clientRequestId 与 Idempotency-Key')
requireMatch(postDetail, /usePostComments\(/, '帖子详情必须接入独立评论会话')
requireMatch(commentSession, /same idempotency key|Keep text|submission/, '评论会话必须保留失败重试所需的幂等状态')
requireMatch(commentSession, /like = async|api\.like/, '评论点赞必须走独立请求与确认状态')
requireMatch(postComments, /查看 \{comment\.replyCount\} 条回复/, '评论组件必须提供楼内回复入口')
requireMatch(postComments, /onClick=\{\(\) => \{ if \(interactive\) props\.onLike\(comment\.id\) \}\}/, '评论组件必须提供单条点赞入口并拦截退出中的点击')
requireMatch(communityService, /\/community\/me\/posts\/\$\{encodeURIComponent\(postId\)\}/, '前端必须接入本人帖子详情接口')

const communityHandler = read('backend/internal/module/community/handler.go')
requireMatch(communityHandler, /auth\.GET\("\/me\/posts\/:id", h\.myPost\)/, '后端必须提供受认证保护的本人帖子详情接口')

console.log('miniapp regression contract: PASS')
