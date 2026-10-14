# 社区评论互动实施计划

> **For agentic workers:** 本任务在当前隔离分支顺序执行，不派新代理。按下面检查点验证并记录；用户已要求全自动实施，常规实现选择不重复征求确认。

**Goal:** 完成真实楼内回复、评论点赞和可靠的帖子点赞状态，保留审核、权限和原主题。

**Architecture:** community 增加可选评论仓储接口与独立线程服务/handler；已有帖子、审核与 outbox 保持边界。前端将评论会话与页面媒体/帖子互动分离，通过服务端回执更新状态。

**Tech Stack:** Go/Gin/GORM/MySQL8、Taro/React/TypeScript/SCSS、Node回归与隔离数据库。

## 1. 后端增量

- [x] `backend/migrations/043_community_comment_interactions.sql` 与 `community/model.go`：`parent_comment_id`、`reply_to_comment_id`、`reply_to_user_id`、`liked_count` 默认0，comment_likes 主键 `(comment_id,user_id)`，分页索引 `(post_id,parent_comment_id,created_at,id)`。
- [x] `community/comment_threads_repository.go`：新接口负责 `FindThreadComment`、`ListCommentLevel`、`CommentMetadata`、`SetCommentLiked`，查询限制同帖/根楼/审核可见性。点赞事务只在关系实际变化时更新计数，回传确认状态。
- [x] `community/comment_threads.go`：根楼和回复分开分页；回复目标由服务器推导；幂等摘要包括 post/target/root/content；复用审核 outbox。未知目标返回404，空或超长内容400，键冲突409。
- [x] `community/comment_threads_handler.go`：注册 GET `/posts/:id/comments/threads`、GET/POST `/posts/:id/comments/:commentId/replies`、PUT/DELETE `/posts/:id/comments/:commentId/like`；认证及现有comment/likes守卫覆盖所有新路径。
- [x] `repository.go`/`handler.go`：旧列表仅根楼，帖子点赞成功返回权威 likedCount；旧客户端兼容。
- [x] 单测先覆盖不存在的方法行为，再实现使其通过。专用MySQL测试只读取 `COMMENT_INTERACTIONS_TEST_MYSQL_DSN`，使用临时引擎；不读取生产DSN。

## 2. 前端评论会话

- [x] `types.ts`/`b-community.ts`：新增线程字段与请求。请求路径含 post和comment，POST只送 content/clientRequestId，PUT/DELETE只送目标状态。
- [x] `pages/post-detail/post-comments.tsx`：根楼加载、楼内分页、错误/重试、独立点赞锁、回复目标、取消与幂等提交。原作者自己的待审核评论明确显示状态。
- [x] `pages/post-detail/index.tsx`：接入线程列表和回复输入；帖子点赞初始状态读取失败时保持不可误操作并提供重读；以服务端回执更新计数；页面变更后丢弃迟到响应。
- [x] SCSS：正文与作者使用现有语义级别，回复只缩进一层，点赞/回复/取消44px触控；长昵称/链接、大字号和键盘布局无覆盖。
- [x] 必要的前端行为测试验证请求去重、重试键、同帖目标归属、服务器计数和失败保留；不写只重复实现字符串的空测试。

## 3. 验证与回执

- [x] `npm.cmd run test:backend`：全部安全后端单测通过。
- [x] 通过扩展 `scripts/test-feature-policy-mysql.cjs --comment-interactions` 启动独立MySQL，验证迁移+并发点赞+审核可见性，退出清理仅本轮临时目录。
- [x] `npm.cmd run verify`、`npm.cmd run build:miniapp`、`git diff --check`。
- [x] 原二手回复合同、路由、分享返回、后台开关回归。
- [x] 本地静态布局提取只显示测试文本与实际图标，不写入生产；记录小屏/大字号的可读性与触控边界。
- [x] 中文提交明确文件，保留两份用户project配置；记录源码、已部署/未部署边界以及真机待测项。
