# 二手商品留言与混合自动审核 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在保留现有二手发布、搜索、收藏、腾讯云 IM 和人工审核契约的前提下，完成卖家商品状态管理、公开留言与一级回复、帖子/商品/留言的混合自动审核、站内通知定位，以及可信的星形收藏反馈。

**Architecture:** 交易域新增独立 `product_comments` 聚合和复用 `trade_write_outbox` 的幂等审核投递；统一审核域只根据持久化的安全结果做自动批准、自动拒绝或保留人工处理，所有终态仍通过 `review.Target` 写回业务表。商品留言获批时，由交易仓储在同一事务中写入唯一站内通知；MiniApp 只消费服务端返回的真实身份、状态和留言树，不推断卖家标签或审核结果。

**Tech Stack:** Go 1.22、Gin、GORM、MySQL 8、React 18、Taro 4、TypeScript 5、SCSS、Node.js 20、微信文本内容安全接口。

---

## 文件与职责映射

### 数据库

- `backend/migrations/036_trade_comments_auto_review.sql`
  - 创建 `product_comments`。
  - 扩展 `notifications` 的商品定位字段与唯一事件索引。
  - 给 `review_tasks` 增加 `decision_source`。
- `backend/migrations/rollback/036_trade_comments_auto_review.sql`
  - 只回滚 036 引入的表、字段和索引。
- `backend/migrations/migration_036_test.go`
  - 静态验证迁移的约束、重复执行保护和回滚边界。

### 统一审核

- `backend/internal/module/admin/review/model.go`
  - 定义 `DecisionSource`，把自动决策来源暴露给后台。
- `backend/internal/module/admin/review/automatic.go`
  - 将 `wechat pass`、`wechat risky`、`wechat review`、`manual:*` 映射为自动批准、自动拒绝或人工兜底。
- `backend/internal/module/admin/review/automatic_test.go`
  - 覆盖对象白名单和所有安全结果。
- `backend/internal/module/admin/review/safety.go`
  - 对可重试错误进行一次有界重试，仍失败则返回 `manual:*`，不泄露 OpenID 或 access token。
- `backend/internal/module/admin/review/safety_test.go`
  - 覆盖通过、明确违规、可疑、超时重试和未配置。
- `backend/internal/module/admin/review/repository.go`
  - 保存 `decision_source`，为自动决策和人工改判提供幂等状态写入。
- `backend/internal/module/admin/review/service.go`
  - `Submit`/`EnsureSubmitted` 自动处理明确结果；终态自动任务允许管理员改判。
- `backend/internal/module/admin/review/service_test.go`
  - 覆盖自动决策、重放、人工改判和身份申请不自动处理。
- `backend/internal/module/admin/handler.go`
  - 增加显式改判接口，不改变现有开始/批准/拒绝接口。

### 交易域

- `backend/internal/module/trade/comment_model.go`
  - 商品留言持久化模型、返回视图和输入类型。
- `backend/internal/module/trade/comment_repository.go`
  - 幂等创建、公开读取、本人审核状态读取、频控、一级回复约束和审核副作用。
- `backend/internal/module/trade/comment_repository_test.go`
  - 使用仓储假实现或 MySQL 测试验证查询边界与通知去重。
- `backend/internal/module/trade/comment_service.go`
  - 校验商品可留言状态、生成规范哈希、推导回复对象、组装卖家标签。
- `backend/internal/module/trade/comment_service_test.go`
  - 覆盖卖家/买家/第三方、一级回复、跨商品拒绝、幂等与频控。
- `backend/internal/module/trade/comment_handler.go`
  - 注册商品留言列表、创建和回复接口。
- `backend/internal/module/trade/comment_handler_test.go`
  - 覆盖 HTTP 状态码和拒绝客户端伪造坐标。
- `backend/internal/module/trade/write_outbox_processor.go`
  - 支持 `trade_product_comment_created` 事件，仍通过 `EnsureSubmitted` 进入统一审核。
- `backend/internal/module/trade/write_outbox_processor_test.go`
  - 验证新事件坐标、重试和死信规则。
- `backend/internal/module/trade/service.go`
  - 补齐 `sold -> selling`，并保持售出商品禁止新增收藏/联系。
- `backend/internal/module/trade/service_test.go`
  - 覆盖完整状态机与服务端所有权校验。
- `backend/cmd/server/main.go`
  - 构造商品留言服务、注册 `product_comment` 审核目标和路由。

### 站内通知

- `backend/internal/module/community/model.go`
  - 新增 `product_commented`、`product_comment_replied` 和商品坐标字段。
- `backend/internal/module/community/service.go`
  - 通知视图继续补齐行为人资料，不改变现有列表/已读接口。
- `backend/internal/module/community/service_test.go`
  - 验证商品通知的行为人、坐标和本人不通知规则。

### MiniApp

- `apps/miniapp/src/types.ts`
  - 增加商品留言、回复和商品通知联合类型。
- `apps/miniapp/src/services/c-trade.ts`
  - 增加商品留言列表、创建、回复和状态变更返回值。
- `apps/miniapp/src/services/b-community.ts`
  - 正确归一化商品通知坐标。
- `apps/miniapp/src/pages/secondhand-detail/product-comments.tsx`
  - 独立留言区组件，处理审核中、人工审核、拒绝、回复和加载状态。
- `apps/miniapp/src/pages/secondhand-detail/product-comments.scss`
  - 一级回复、卖家标签、输入区和安全区布局。
- `apps/miniapp/src/pages/secondhand-detail/index.tsx`
  - 按真实 session 判断卖家视角，接入状态按钮、留言组件和目标评论定位。
- `apps/miniapp/src/pages/secondhand-detail/index.scss`
  - 将伪方块收藏图标改成空心/实心星形，并兼容 iOS/Android 安全区。
- `apps/miniapp/src/pages/notifications/index.tsx`
  - 商品通知跳转到商品详情并携带 `commentId`。
- `apps/miniapp/src/pages/notifications/index.scss`
  - 仅补充商品通知类型所需的最小状态样式。

### 管理后台与聚焦检查

- `apps/admin/src/App.tsx`
  - 展示自动决策来源，并允许对自动终态执行显式改判。
- `apps/admin/src/styles.css`
  - 只增加来源徽标和改判按钮状态，不做页面重设计。
- `scripts/check-secondhand-comments-contract.cjs`
  - 检查接口、卖家操作、星形语义、通知坐标和禁止客户端伪造字段。
- `package.json`
  - 增加 `check:secondhand-comments-contract`，并纳入 `verify`。

## 工作区保护规则

当前工作树已有大量用户未提交修改，实施时必须遵守：

1. 禁止使用 `git add .`、`git add -A`、`git reset --hard` 或覆盖式恢复命令。
2. 每个任务开始前运行：

   ```powershell
   git status --short
   git diff -- <本任务涉及的已有文件>
   ```

3. 新文件可用精确路径暂存；任务开始前已经是脏状态的文件只能使用：

   ```powershell
   git add -p -- <已有脏文件>
   ```

4. 每次提交前必须运行：

   ```powershell
   git diff --cached --check
   git diff --cached --name-only
   ```

   预期：只列出本任务文件，不包含部署、外卖、支付、IM、身份申请或用户先前改动。

## Task 1：建立 036 数据库契约

**Files:**

- Create: `backend/migrations/036_trade_comments_auto_review.sql`
- Create: `backend/migrations/rollback/036_trade_comments_auto_review.sql`
- Create: `backend/migrations/migration_036_test.go`

- [ ] **Step 1：先写失败的迁移静态测试**

  在 `migration_036_test.go` 中固定必须出现的契约：

  ```go
  func TestMigration036DefinesTradeCommentsAndReviewAudit(t *testing.T) {
      source := readMigration036(t, "036_trade_comments_auto_review.sql")
      for _, token := range []string{
          "CREATE TABLE product_comments",
          "parent_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
          "reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
          "client_request_id CHAR(26) NOT NULL",
          "request_hash BINARY(32) NOT NULL",
          "UNIQUE INDEX uq_product_comments_user_request",
          "idx_product_comments_public",
          "idx_product_comment_replies",
          "ADD COLUMN product_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
          "ADD COLUMN product_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
          "ADD COLUMN decision_source VARCHAR(24) NOT NULL DEFAULT 'manual'",
      } {
          if !strings.Contains(source, token) {
              t.Fatalf("migration 036 missing %q", token)
          }
      }
  }
  ```

- [ ] **Step 2：运行测试并确认失败**

  ```powershell
  cd backend
  go test ./migrations -run TestMigration036 -count=1
  ```

  预期：FAIL，原因是 036 文件尚不存在。

- [ ] **Step 3：编写可重复执行的前向迁移**

  使用与 035 相同的存储过程、`GET_LOCK` 和 `information_schema` 守卫。核心表结构固定为：

  ```sql
  CREATE TABLE product_comments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    product_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    parent_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    content VARCHAR(500) NOT NULL,
    review_status VARCHAR(24) NOT NULL DEFAULT 'submitted',
    client_request_id CHAR(26) NOT NULL,
    request_hash BINARY(32) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    deleted_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE INDEX uq_product_comments_user_request (user_id, client_request_id),
    INDEX idx_product_comments_public (product_id, parent_comment_id, review_status, created_at, id),
    INDEX idx_product_comment_replies (parent_comment_id, review_status, created_at, id),
    INDEX idx_product_comments_user_recent (user_id, created_at, id)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
  ```

  扩展 `notifications` 时先守卫并重建 `idx_notification_event` 为：

  ```sql
  UNIQUE INDEX idx_notification_event (
    recipient_id, actor_id, type, post_id, comment_id, product_id, product_comment_id
  )
  ```

  `review_tasks.decision_source` 允许 `manual`、`automatic`、`manual_override`，默认 `manual`。

- [ ] **Step 4：编写对称回滚**

  回滚顺序必须先恢复通知唯一索引，再移除商品坐标，最后删除 `product_comments` 和 `decision_source`。回滚不触碰现有帖子、商品、身份、IM 或配送数据。

- [ ] **Step 5：运行迁移测试**

  ```powershell
  cd backend
  go test ./migrations -run TestMigration036 -count=1
  ```

  预期：PASS，输出包含 `ok campus-circle/backend/migrations`。

- [ ] **Step 6：提交迁移**

  ```powershell
  git add -- backend/migrations/036_trade_comments_auto_review.sql backend/migrations/rollback/036_trade_comments_auto_review.sql backend/migrations/migration_036_test.go
  git diff --cached --check
  git commit -m "数据库：新增商品留言与自动审核审计字段"
  ```

## Task 2：实现安全结果分类与有界重试

**Files:**

- Create: `backend/internal/module/admin/review/automatic.go`
- Create: `backend/internal/module/admin/review/automatic_test.go`
- Modify: `backend/internal/module/admin/review/safety.go`
- Create or modify: `backend/internal/module/admin/review/safety_test.go`

- [ ] **Step 1：写自动分类失败测试**

  使用表驱动测试固定以下结果：

  | objectType | risk | automatic | status |
  | --- | --- | --- | --- |
  | `post` | `wechat pass` | true | `approved` |
  | `comment` | `wechat risky label=100` | true | `rejected` |
  | `product` | `wechat review label=20001` | false | 空 |
  | `product_comment` | `manual:wechat_request_failed` | false | 空 |
  | `identity_application` | `wechat pass` | false | 空 |

  核心 API：

  ```go
  type AutomaticDecision struct {
      Status Status
      Reason string
      Apply  bool
  }

  func ClassifyAutomaticDecision(objectType, risk string) AutomaticDecision
  ```

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/admin/review -run TestClassifyAutomaticDecision -count=1
  ```

  预期：FAIL，`ClassifyAutomaticDecision` 尚未定义。

- [ ] **Step 3：实现最小白名单分类器**

  白名单只能包含：

  ```go
  var automaticObjectTypes = map[string]struct{}{
      "post": {},
      "comment": {},
      "product": {},
      "product_comment": {},
  }
  ```

  规则必须为：

  ```go
  switch {
  case risk == "wechat pass":
      return AutomaticDecision{Status: StatusApproved, Reason: "wechat text safety pass", Apply: true}
  case strings.HasPrefix(risk, "wechat risky"):
      return AutomaticDecision{Status: StatusRejected, Reason: "wechat text safety risky", Apply: true}
  default:
      return AutomaticDecision{}
  }
  ```

  `wechat review` 和所有 `manual:*` 必须失败关闭并保留人工任务。

- [ ] **Step 4：写安全客户端重试失败测试**

  使用可注入的 `http.Client`/`RoundTripper`，验证：

  - 第一次网络失败、第二次通过，最终返回 `wechat pass`。
  - 两次超时，最终返回 `manual:wechat_request_failed`。
  - `risky` 返回 `wechat risky label=<值>`，不被改写成 `review`。
  - 未配置 AppID/Secret 不发请求，直接人工兜底。

- [ ] **Step 5：重构为两次有界尝试**

  把单次调用提取为：

  ```go
  func (c *SafetyClient) checkTextOnce(ctx context.Context, openID, content string) (string, bool)
  ```

  第二个返回值表示是否可重试。总尝试次数固定为 2；两次之间不 `Sleep`，由 HTTP 超时控制总时长。任何日志和返回值都不得包含 OpenID、access token 或微信原始响应正文。

- [ ] **Step 6：运行审核包测试**

  ```powershell
  cd backend
  go test ./internal/module/admin/review -run 'TestClassifyAutomaticDecision|TestSafetyClient' -count=1
  ```

  预期：PASS。

- [ ] **Step 7：提交**

  ```powershell
  git add -- backend/internal/module/admin/review/automatic.go backend/internal/module/admin/review/automatic_test.go backend/internal/module/admin/review/safety_test.go
  git add -p -- backend/internal/module/admin/review/safety.go
  git diff --cached --check
  git commit -m "审核：分类微信安全结果并有界重试"
  ```

## Task 3：让统一审核自动决策且支持人工改判

**Files:**

- Modify: `backend/internal/module/admin/review/model.go`
- Modify: `backend/internal/module/admin/review/repository.go`
- Modify: `backend/internal/module/admin/review/service.go`
- Modify: `backend/internal/module/admin/review/service_test.go`
- Modify: `backend/internal/module/admin/handler.go`
- Modify or create: `backend/internal/module/admin/handler_test.go`

- [ ] **Step 1：先写服务失败测试**

  覆盖四个行为：

  1. `EnsureSubmitted(product, wechat pass)` 调用目标一次并返回 `approved/automatic`。
  2. 同一对象重放不重复调用已完成副作用。
  3. `wechat review` 保持 `submitted/manual`。
  4. 管理员把自动拒绝改判为批准，最终来源为 `manual_override`。

  模型新增：

  ```go
  type DecisionSource string

  const (
      DecisionSourceManual         DecisionSource = "manual"
      DecisionSourceAutomatic      DecisionSource = "automatic"
      DecisionSourceManualOverride DecisionSource = "manual_override"
  )
  ```

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/admin/review -run 'TestEnsureSubmittedAutomatically|TestOverrideAutomaticDecision' -count=1
  ```

  预期：FAIL，自动决策和改判入口尚不存在。

- [ ] **Step 3：实现自动决策入口**

  在 `Submit` 和 `EnsureSubmitted` 持久化任务后调用同一个私有方法：

  ```go
  func (s *Service) applyAutomaticDecision(ctx context.Context, task Task) (Task, error)
  ```

  约束：

  - 只有 `task.Status == submitted` 才分类。
  - `Apply == false` 原样返回。
  - 自动终态必须先通过 `setTarget` 写业务副作用，再保存任务终态和 `DecisionSourceAutomatic`。
  - 若保存任务失败，返回错误；下一次 `EnsureSubmitted` 会对仍为 `submitted` 的任务幂等重放目标副作用。
  - 已为相同自动终态时直接返回，不重复副作用。

- [ ] **Step 4：实现人工改判**

  增加：

  ```go
  func (s *Service) Override(
      ctx context.Context,
      id, operatorID uint,
      status Status,
      reason string,
  ) (Task, error)
  ```

  只允许管理员对 `approved` 或 `rejected` 终态改判；必须提供非零 `operatorID` 和非空原因；通过 `setTarget` 重放业务副作用并保存 `manual_override`。

- [ ] **Step 5：增加后台接口测试和路由**

  注册：

  ```go
  secured.POST("/reviews/:id/override", h.overrideReview)
  ```

  请求体：

  ```json
  {
    "status": "approved",
    "reason": "人工复核确认无违规"
  }
  ```

  无效状态/空原因返回 400；不存在返回 404；成功返回更新后的任务。

- [ ] **Step 6：运行测试**

  ```powershell
  cd backend
  go test ./internal/module/admin/review ./internal/module/admin -run 'Test.*Automatic|Test.*Override' -count=1
  ```

  预期：PASS。

- [ ] **Step 7：提交**

  ```powershell
  git add -p -- backend/internal/module/admin/review/model.go backend/internal/module/admin/review/repository.go backend/internal/module/admin/review/service.go backend/internal/module/admin/review/service_test.go backend/internal/module/admin/handler.go
  git add -- backend/internal/module/admin/handler_test.go
  git diff --cached --check
  git commit -m "审核：自动完成明确结果并支持人工改判"
  ```

## Task 4：建立商品留言模型与幂等仓储

**Files:**

- Create: `backend/internal/module/trade/comment_model.go`
- Create: `backend/internal/module/trade/comment_repository.go`
- Create: `backend/internal/module/trade/comment_repository_test.go`
- Modify: `backend/internal/module/trade/write_contract_repository.go`
- Modify: `backend/internal/module/trade/write_contract_repository_test.go`

- [ ] **Step 1：写模型和仓储失败测试**

  固定模型：

  ```go
  type ProductComment struct {
      ID              uint           `json:"id" gorm:"primaryKey"`
      ProductID       uint           `json:"productId" gorm:"index"`
      UserID          uint           `json:"userId" gorm:"index"`
      ParentCommentID uint           `json:"parentCommentId" gorm:"index"`
      ReplyToUserID   uint           `json:"replyToUserId" gorm:"index"`
      Content         string         `json:"content" gorm:"size:500"`
      ReviewStatus    review.Status  `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
      ClientRequestID string         `json:"-" gorm:"type:char(26)"`
      RequestHash     []byte         `json:"-" gorm:"type:binary(32)"`
      CreatedAt       time.Time      `json:"createdAt"`
      UpdatedAt       time.Time      `json:"updatedAt"`
      DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
  }
  ```

  仓储接口至少包含：

  ```go
  type ProductCommentRepository interface {
      FindProductComment(context.Context, uint) (ProductComment, error)
      FindProductCommentByRequest(context.Context, uint, string) (ProductComment, bool, error)
      CreateProductCommentOrReplay(context.Context, *ProductComment, *WriteOutbox) (bool, error)
      ListProductCommentThreads(context.Context, uint, uint, int, int) ([]ProductComment, int64, error)
      ListProductCommentReplies(context.Context, []uint, uint) ([]ProductComment, error)
      CountRecentProductComments(context.Context, uint, time.Time) (int64, error)
      HasRecentDuplicateProductComment(context.Context, uint, uint, []byte, time.Time) (bool, error)
      SetProductCommentReviewStatus(context.Context, uint, review.Status) error
  }
  ```

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentRepository|TestProductCommentCreateReplay' -count=1
  ```

  预期：FAIL，类型和方法尚不存在。

- [ ] **Step 3：实现事务性幂等创建**

  `CreateProductCommentOrReplay` 必须：

  1. 同一 `(user_id, client_request_id)` 且哈希一致时返回原留言和 `replayed=true`。
  2. 同一键但哈希不同返回现有 `ErrIdempotencyConflict`。
  3. 新留言和 `trade_write_outbox` 在同一个 GORM 事务内写入。
  4. 事件坐标固定为：

     ```go
     event.EventType = "trade_product_comment_created"
     event.AggregateType = "product_comment"
     event.AggregateID = uint64(comment.ID)
     event.OwnerUserID = uint64(comment.UserID)
     ```

- [ ] **Step 4：实现可见性查询**

  顶层和回复查询都采用：

  ```sql
  review_status = 'approved' OR user_id = ?
  ```

  其中 `?` 是当前登录用户。别人处于 `submitted/reviewing/rejected` 的内容绝不返回。顶层按最近更新时间倒序，回复按 `created_at ASC, id ASC`。

- [ ] **Step 5：实现审核副作用事务**

  `SetProductCommentReviewStatus` 必须在一个事务中：

  - 锁定留言和商品。
  - 幂等更新审核状态。
  - 仅在首次进入 `approved` 时创建通知。
  - 顶层留言通知商品卖家；回复通知 `reply_to_user_id`。
  - 行为人与接收人相同时跳过。
  - 使用扩展后的 `idx_notification_event` 防重复。

- [ ] **Step 6：运行仓储测试**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentRepository|TestProductCommentCreateReplay|TestProductCommentReviewNotification' -count=1
  ```

  预期：PASS。

- [ ] **Step 7：提交**

  ```powershell
  git add -- backend/internal/module/trade/comment_model.go backend/internal/module/trade/comment_repository.go backend/internal/module/trade/comment_repository_test.go
  git add -p -- backend/internal/module/trade/write_contract_repository.go backend/internal/module/trade/write_contract_repository_test.go
  git diff --cached --check
  git commit -m "交易：新增幂等商品留言仓储"
  ```

## Task 5：实现商品留言与一级回复业务规则

**Files:**

- Create: `backend/internal/module/trade/comment_service.go`
- Create: `backend/internal/module/trade/comment_service_test.go`

- [ ] **Step 1：写业务失败测试**

  必须覆盖：

  - 登录用户可在 `selling`/`reserved` 且已审核商品下留言。
  - `sold`、下架、删除和未审核商品返回冲突或 404。
  - 回复顶层留言成功。
  - 回复一条回复时仍归入原顶层，并把 `reply_to_user_id` 设为目标回复作者。
  - 目标留言属于另一个商品时返回 404。
  - 60 秒内最多 5 条；60 秒内同商品、同坐标、同正文拒绝。
  - 内容去首尾空格后必须为 1–500 个 Unicode 字符。
  - 幂等重放返回同一留言；不同正文冲突。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentService' -count=1
  ```

  预期：FAIL，`ProductCommentService` 尚不存在。

- [ ] **Step 3：实现规范请求哈希**

  固定版本和坐标，禁止把客户端伪造字段写入哈希：

  ```go
  type canonicalProductCommentRequest struct {
      Version           string `json:"version"`
      ProductID         uint   `json:"productId"`
      ParentCommentID   uint   `json:"parentCommentId"`
      ReplyToUserID     uint   `json:"replyToUserId"`
      Content           string `json:"content"`
  }
  ```

  `Version` 固定为 `trade-product-comment:v1`，序列化后取 SHA-256。

- [ ] **Step 4：实现回复坐标推导**

  服务端读取目标留言：

  ```go
  if target.ParentCommentID == 0 {
      parentID = target.ID
  } else {
      parentID = target.ParentCommentID
  }
  replyToUserID = target.UserID
  ```

  然后重新读取 `parentID`，确认其 `ParentCommentID == 0` 且与 URL 商品一致。请求体只接受 `content` 和 `clientRequestId`。

- [ ] **Step 5：实现返回视图**

  `ProductCommentView` 返回：

  ```go
  type ProductCommentView struct {
      ID              uint                 `json:"id"`
      ProductID       uint                 `json:"productId"`
      ParentCommentID uint                 `json:"parentCommentId"`
      ReplyToUserID   uint                 `json:"replyToUserId"`
      Content         string               `json:"content"`
      ReviewStatus    review.Status        `json:"reviewStatus"`
      Author          user.Profile         `json:"author"`
      ReplyTo         *user.Profile        `json:"replyTo,omitempty"`
      IsSeller        bool                 `json:"isSeller"`
      Replies         []ProductCommentView `json:"replies"`
      CreatedAt       time.Time            `json:"createdAt"`
  }
  ```

  `IsSeller` 必须由 `comment.UserID == product.SellerID` 计算。

- [ ] **Step 6：运行测试**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentService' -count=1
  ```

  预期：PASS。

- [ ] **Step 7：提交**

  ```powershell
  git add -- backend/internal/module/trade/comment_service.go backend/internal/module/trade/comment_service_test.go
  git diff --cached --check
  git commit -m "交易：实现商品留言与一级回复规则"
  ```

## Task 6：接入商品留言 HTTP 与交易 Outbox

**Files:**

- Create: `backend/internal/module/trade/comment_handler.go`
- Create: `backend/internal/module/trade/comment_handler_test.go`
- Modify: `backend/internal/module/trade/write_outbox_processor.go`
- Modify: `backend/internal/module/trade/write_outbox_processor_test.go`

- [ ] **Step 1：写路由失败测试**

  固定三个鉴权接口：

  ```text
  GET  /api/v1/trade/products/:id/comments?page=1&pageSize=20
  POST /api/v1/trade/products/:id/comments
  POST /api/v1/trade/products/:id/comments/:commentId/replies
  ```

  提交只接受：

  ```go
  type productCommentRequest struct {
      Content         string `json:"content"`
      ClientRequestID string `json:"clientRequestId"`
  }
  ```

  测试额外提交 `productId`、`parentCommentId`、`replyToUserId`、`isSeller`，确认服务端忽略这些字段并使用 URL/数据库坐标。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentHandler' -count=1
  ```

  预期：FAIL，handler 尚不存在。

- [ ] **Step 3：实现错误映射**

  - 400：空内容、超长、非法 ULID。
  - 401：未登录。
  - 404：商品/留言不存在或不可安全枚举。
  - 409：商品不可留言、幂等冲突、重复正文。
  - 429：频率限制。
  - 201：首次创建。
  - 200 + `Idempotency-Replayed: true`：幂等重放。

- [ ] **Step 4：扩展 Outbox 验证**

  `validateTradeWriteOutbox` 改为按事件类型分派：

  ```go
  switch event.EventType {
  case "trade_product_created":
      return validateProductCreatedEvent(event)
  case "trade_product_comment_created":
      return validateProductCommentCreatedEvent(event)
  default:
      return payload, permanentTradeOutboxError("unsupported trade write event")
  }
  ```

  商品留言事件必须没有 media reservation，审核对象必须是 `product_comment`。

- [ ] **Step 5：运行测试**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductCommentHandler|TestTradeWriteOutbox.*Comment' -count=1
  ```

  预期：PASS。

- [ ] **Step 6：提交**

  ```powershell
  git add -- backend/internal/module/trade/comment_handler.go backend/internal/module/trade/comment_handler_test.go
  git add -p -- backend/internal/module/trade/write_outbox_processor.go backend/internal/module/trade/write_outbox_processor_test.go
  git diff --cached --check
  git commit -m "交易：开放商品留言接口并接入审核投递"
  ```

## Task 7：扩展站内通知商品定位

**Files:**

- Modify: `backend/internal/module/community/model.go`
- Modify: `backend/internal/module/community/service.go`
- Modify: `backend/internal/module/community/service_test.go`
- Modify: `backend/internal/module/community/repository.go`

- [ ] **Step 1：写通知失败测试**

  新类型：

  ```go
  const (
      NotificationProductComment NotificationType = "product_commented"
      NotificationProductReply   NotificationType = "product_comment_replied"
  )
  ```

  `Notification` 新增：

  ```go
  ProductID        uint `json:"productId" gorm:"index;uniqueIndex:idx_notification_event"`
  ProductCommentID uint `json:"productCommentId" gorm:"uniqueIndex:idx_notification_event"`
  ```

  测试 `Notifications` 返回行为人资料、商品 ID、目标留言 ID 和准确未读数。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/community -run TestProductCommentNotification -count=1
  ```

  预期：FAIL，字段和类型尚不存在。

- [ ] **Step 3：实现模型和读取**

  保持现有通知列表排序、分页和已读接口不变；只让 GORM 扫描新增字段。帖子通知继续返回 `productId=0`，商品通知继续返回 `postId=0`。

- [ ] **Step 4：运行测试**

  ```powershell
  cd backend
  go test ./internal/module/community -run 'TestProductCommentNotification|TestNotifications' -count=1
  ```

  预期：PASS。

- [ ] **Step 5：提交**

  ```powershell
  git add -p -- backend/internal/module/community/model.go backend/internal/module/community/service.go backend/internal/module/community/service_test.go backend/internal/module/community/repository.go
  git diff --cached --check
  git commit -m "通知：支持商品留言与回复定位"
  ```

## Task 8：补齐卖家状态机和售出操作保护

**Files:**

- Modify: `backend/internal/module/trade/service.go`
- Modify: `backend/internal/module/trade/service_test.go`
- Modify: `backend/internal/module/trade/handler_test.go`

- [ ] **Step 1：写状态机失败测试**

  用表驱动测试固定：

  ```go
  tests := []struct {
      from ProductStatus
      to   ProductStatus
      ok   bool
  }{
      {StatusSelling, StatusReserved, true},
      {StatusSelling, StatusSold, true},
      {StatusReserved, StatusSelling, true},
      {StatusReserved, StatusSold, true},
      {StatusSold, StatusSelling, true},
      {StatusSold, StatusReserved, false},
  }
  ```

  同时验证非卖家始终 `ErrForbidden`，未审核商品 `ErrConflict`。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductStatusTransitions|TestSoldProductGuards' -count=1
  ```

  预期：至少 `sold -> selling` 失败。

- [ ] **Step 3：实现最小状态机修改**

  将 `StatusSold` 分支改为只允许 `StatusSelling`。不改变 `SetStatus` 的旧状态 CAS，服务端仍重新读取商品验证卖家和审核状态。

- [ ] **Step 4：确认售出保护**

  测试确认：

  - 已有收藏可取消。
  - 售出后不能新增收藏。
  - 售出后不能新建联系关系。
  - 售出后历史留言仍可读取但不能新增留言/回复。

- [ ] **Step 5：运行测试**

  ```powershell
  cd backend
  go test ./internal/module/trade -run 'TestProductStatusTransitions|TestSoldProductGuards' -count=1
  ```

  预期：PASS。

- [ ] **Step 6：提交**

  ```powershell
  git add -p -- backend/internal/module/trade/service.go backend/internal/module/trade/service_test.go backend/internal/module/trade/handler_test.go
  git diff --cached --check
  git commit -m "交易：补齐卖家重新上架和售出保护"
  ```

## Task 9：装配商品留言与审核目标

**Files:**

- Modify: `backend/cmd/server/main.go`
- Create or modify: `backend/cmd/server/main_test.go`

- [ ] **Step 1：写装配契约失败测试**

  静态或构造测试必须确认：

  - 使用现有 `tradeRepository` 构造 `ProductCommentService`。
  - 注册 `product_comment` 审核目标。
  - 在 `/api/v1/trade` 注册商品留言 handler。
  - 没有新增第二套审核服务或通知服务。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  cd backend
  go test ./cmd/server -run TestProductCommentWiring -count=1
  ```

  预期：FAIL，装配尚不存在。

- [ ] **Step 3：实现最小装配**

  装配顺序固定为：

  ```go
  tradeCommentService := trade.NewProductCommentService(
      tradeRepository,
      userService,
      reviewService,
  )
  reviewService.RegisterTarget(tradeCommentService)
  trade.NewProductCommentHandler(tradeCommentService, userService, reviewSafety).Register(tradeRoutes)
  ```

  若现有路由变量名称不同，使用当前 `/trade` 路由组，不创建重复前缀。

- [ ] **Step 4：运行聚焦测试**

  ```powershell
  cd backend
  go test ./cmd/server ./internal/module/trade ./internal/module/admin/review ./internal/module/community -count=1
  ```

  预期：PASS。

- [ ] **Step 5：提交**

  ```powershell
  git add -p -- backend/cmd/server/main.go backend/cmd/server/main_test.go
  git diff --cached --check
  git commit -m "服务端：装配商品留言和自动审核目标"
  ```

## Task 10：增加 MiniApp 商品留言与通知类型客户端

**Files:**

- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`
- Modify: `apps/miniapp/src/services/b-community.ts`
- Create: `scripts/check-secondhand-comments-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1：先写失败的契约脚本**

  脚本检查：

  - `ProductComment` 包含 `reviewStatus`、`isSeller`、`replies`。
  - `c-trade.ts` 调用三条留言接口。
  - 创建和回复都使用 `createClientRequestId()`。
  - 请求体不含 `productId`、`parentCommentId`、`replyToUserId`、`isSeller`。
  - 通知类型包含 `product_commented` 和 `product_comment_replied`。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  ```

  预期：FAIL，缺少类型或接口。

- [ ] **Step 3：增加 MiniApp 类型**

  ```ts
  export type ProductCommentReviewStatus =
    | 'submitted'
    | 'reviewing'
    | 'approved'
    | 'rejected'

  export interface ProductComment {
    id: string
    productId: string
    parentCommentId: string
    replyToUserId: string
    content: string
    reviewStatus: ProductCommentReviewStatus
    author: UserProfile
    replyTo?: UserProfile
    isSeller: boolean
    replies: ProductComment[]
    createdAt: string
  }
  ```

  `CommunityNotification` 改为判别联合所需字段：

  ```ts
  type: 'post_commented' | 'post_liked' | 'user_followed'
      | 'product_commented' | 'product_comment_replied'
  productId?: string
  productCommentId?: string
  ```

- [ ] **Step 4：实现客户端**

  ```ts
  export function listProductComments(
    productId: string,
    page = 1,
    pageSize = 20
  ): Promise<ProductCommentPage>

  export function createProductComment(
    productId: string,
    content: string,
    clientRequestId = createClientRequestId()
  ): Promise<ProductComment>

  export function replyProductComment(
    productId: string,
    commentId: string,
    content: string,
    clientRequestId = createClientRequestId()
  ): Promise<ProductComment>
  ```

  保留同一个 `clientRequestId` 用于 UI 层重试，不在服务函数内部失败后自动生成新 ID。

- [ ] **Step 5：运行类型与契约检查**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  npm --prefix apps/miniapp run typecheck
  ```

  预期：两条命令都 PASS。

- [ ] **Step 6：提交**

  ```powershell
  git add -- scripts/check-secondhand-comments-contract.cjs
  git add -p -- apps/miniapp/src/types.ts apps/miniapp/src/services/c-trade.ts apps/miniapp/src/services/b-community.ts package.json
  git diff --cached --check
  git commit -m "小程序：增加商品留言与通知客户端契约"
  ```

## Task 11：在正式商品详情页实现卖家操作、星形收藏和留言区

**Files:**

- Create: `apps/miniapp/src/pages/secondhand-detail/product-comments.tsx`
- Create: `apps/miniapp/src/pages/secondhand-detail/product-comments.scss`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-detail/index.scss`

- [ ] **Step 1：扩展契约脚本并确认失败**

  新增检查：

  - 页面从 `useSessionStore` 读取真实 `profile.id`。
  - `isSeller = profile.id === product.sellerId`。
  - 卖家不渲染“联系卖家”。
  - 卖家按钮调用 `changeProductStatus`。
  - 星形具有明确的 `☆`/`★` 可访问文本或等价 SVG 路径，不再使用旋转方块伪造星形。
  - 商品留言区传入 `product.id`、`product.status` 和 `targetCommentId`。

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  ```

  预期：FAIL。

- [ ] **Step 2：实现卖家操作**

  从 session 读取用户：

  ```ts
  const currentUserId = useSessionStore((state) => state.profile.id)
  const isSeller = Boolean(product && currentUserId === product.sellerId)
  ```

  状态按钮映射固定为：

  | 当前状态 | 按钮 |
  | --- | --- |
  | `selling` | 标记预订、标记售出 |
  | `reserved` | 重新上架、标记售出 |
  | `sold` | 重新上架 |

  点击后先弹确认，再调用 `changeProductStatus`，成功后重新 `getProduct`，失败时不乐观猜测状态。

- [ ] **Step 3：替换收藏图标**

  JSX 使用：

  ```tsx
  <Text
    className='secondhand-detail-favorite__star'
    aria-label={favorited ? '取消收藏' : '收藏'}
  >
    {favorited ? '★' : '☆'}
  </Text>
  ```

  SCSS：

  ```scss
  .secondhand-detail-favorite__star {
    color: $text-muted;
    font-size: 44rpx;
    line-height: 1;
    transform: scale(1);
    transition: transform 120ms ease, color 120ms ease;
  }

  .secondhand-detail-favorite--active .secondhand-detail-favorite__star {
    color: #f4b740;
  }
  ```

  删除当前旋转方块的 `::before/::after`。

- [ ] **Step 4：实现留言组件**

  组件状态至少包含：

  ```ts
  type CommentLoadState = 'loading' | 'ready' | 'empty' | 'error'
  type ReplyTarget = { commentId: string; authorName: string } | null
  ```

  行为：

  - 首屏读取 20 条顶层留言及 replies。
  - 点击留言或回复设置“回复某某”。
  - 提交时保存本次 `clientRequestId`；网络失败重试沿用同一 ID。
  - 新留言立即插入本人可见列表并显示：
    - `submitted`：审核中。
    - `reviewing`：等待人工审核。
    - `rejected`：未通过，显示服务端安全原因的用户友好文案。
  - `sold` 时历史内容可见，输入区显示“商品已售出，不能继续留言”。
  - `isSeller` 为真时显示“卖家”标签，前端不自行计算。

- [ ] **Step 5：处理通知定位**

  读取：

  ```ts
  const targetCommentId =
    Taro.getCurrentInstance().router?.params?.commentId ?? ''
  ```

  加载完成后用稳定的元素 ID `product-comment-${id}` 调用 `Taro.createSelectorQuery()` 获取位置并滚动；若目标不存在，只显示正常列表，不泄露资源状态。

- [ ] **Step 6：处理安全区和键盘**

  - ScrollView 底部 padding 同时容纳 action bar、留言输入区和 `env(safe-area-inset-bottom)`。
  - 输入区使用 `adjustPosition`，不写死 iPhone 高度。
  - iOS 与 Android 均不让输入框、发送键或卖家按钮被微信胶囊/键盘遮挡。

- [ ] **Step 7：运行聚焦检查**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  npm --prefix apps/miniapp run typecheck
  npm run check:miniapp-capsule-layout
  ```

  预期：PASS。

- [ ] **Step 8：提交**

  ```powershell
  git add -- apps/miniapp/src/pages/secondhand-detail/product-comments.tsx apps/miniapp/src/pages/secondhand-detail/product-comments.scss
  git add -p -- apps/miniapp/src/pages/secondhand-detail/index.tsx apps/miniapp/src/pages/secondhand-detail/index.scss
  git diff --cached --check
  git commit -m "小程序：完成卖家操作和商品留言界面"
  ```

## Task 12：让商品通知跳转并定位留言

**Files:**

- Modify: `apps/miniapp/src/pages/notifications/index.tsx`
- Modify: `apps/miniapp/src/pages/notifications/index.scss`
- Modify: `scripts/check-secondhand-comments-contract.cjs`

- [ ] **Step 1：写跳转失败检查**

  商品通知应生成：

  ```text
  /pages/secondhand-detail/index?id=<productId>&commentId=<productCommentId>
  ```

  帖子通知仍进入 `post-detail`，关注通知保持原行为。

- [ ] **Step 2：运行并确认失败**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  ```

  预期：FAIL，通知页尚未识别商品通知。

- [ ] **Step 3：实现判别跳转**

  ```ts
  if (
    (item.type === 'product_commented' ||
      item.type === 'product_comment_replied') &&
    item.productId
  ) {
    await Taro.navigateTo({
      url: `/pages/secondhand-detail/index?id=${encodeURIComponent(item.productId)}&commentId=${encodeURIComponent(item.productCommentId || '')}`,
    })
    return
  }
  ```

  先标记已读，再导航；导航失败保留当前列表并显示统一提示。

- [ ] **Step 4：运行检查**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  npm --prefix apps/miniapp run typecheck
  ```

  预期：PASS。

- [ ] **Step 5：提交**

  ```powershell
  git add -p -- apps/miniapp/src/pages/notifications/index.tsx apps/miniapp/src/pages/notifications/index.scss scripts/check-secondhand-comments-contract.cjs
  git diff --cached --check
  git commit -m "小程序：支持商品留言通知定位"
  ```

## Task 13：在管理后台展示自动来源并执行改判

**Files:**

- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1：写前端类型和渲染失败检查**

  在现有后台构建中加入 `ReviewTask.decisionSource`，并为 `automatic` 终态显示“自动审核”来源。自动终态必须提供“改判为通过/拒绝”操作，提交到：

  ```text
  POST /api/v1/admin/reviews/:id/override
  ```

- [ ] **Step 2：运行并确认失败**

  ```powershell
  npm --workspace apps/admin run typecheck
  ```

  预期：在加入调用但尚未补齐类型时 FAIL。

- [ ] **Step 3：实现最小后台交互**

  - `decisionSource === automatic` 显示来源徽标。
  - 改判前要求输入非空原因。
  - 成功后刷新当前审核列表。
  - 身份申请页不显示自动审核或改判入口。
  - 不改变后台整体 UI、导航或登录流程。

- [ ] **Step 4：运行构建**

  ```powershell
  npm --workspace apps/admin run typecheck
  npm --workspace apps/admin run build
  ```

  预期：PASS。

- [ ] **Step 5：提交**

  ```powershell
  git add -p -- apps/admin/src/App.tsx apps/admin/src/styles.css
  git diff --cached --check
  git commit -m "后台：展示自动审核来源并支持改判"
  ```

## Task 14：完成聚焦回归与全量验证

**Files:**

- Modify: `package.json`
- Create: `docs/SECONDHAND-COMMENTS-AUTO-REVIEW-ACCEPTANCE.md`

- [ ] **Step 1：把聚焦检查纳入 verify**

  增加：

  ```json
  "check:secondhand-comments-contract": "node scripts/check-secondhand-comments-contract.cjs"
  ```

  并在 `verify` 中放在 typecheck 之前。

- [ ] **Step 2：运行后端聚焦测试**

  ```powershell
  cd backend
  go test ./internal/module/admin/review ./internal/module/trade ./internal/module/community ./cmd/server ./migrations -count=1
  ```

  预期：PASS。

- [ ] **Step 3：运行后端全量测试**

  ```powershell
  cd backend
  go test ./... -count=1
  ```

  预期：PASS；若共享 MySQL、Meilisearch 或环境变量导致既有环境测试失败，必须把“本功能聚焦 PASS”和“共享环境 PENDING/FAIL”分开记录，不可把绿色聚焦测试冒充全量通过。

- [ ] **Step 4：运行 MiniApp 检查和正式构建**

  ```powershell
  node scripts/check-secondhand-comments-contract.cjs
  npm --prefix apps/miniapp run typecheck
  npm run check:miniapp-static-routes
  npm run check:miniapp-capsule-layout
  npm run build:miniapp
  ```

  预期：全部 PASS，`apps/miniapp/dist` 生成成功。

- [ ] **Step 5：运行全仓验证**

  ```powershell
  npm run verify
  ```

  预期：PASS；若 `check:boundaries` 因当前工作树既有红线失败，记录精确失败项和本次差异，禁止顺手修复外卖、支付、IM、部署或身份申请。

- [ ] **Step 6：编写验收记录**

  `docs/SECONDHAND-COMMENTS-AUTO-REVIEW-ACCEPTANCE.md` 必须记录：

  - commit SHA。
  - 迁移应用/回滚结果。
  - 聚焦测试、全量测试、typecheck、构建和边界检查结果。
  - 自动通过、自动拒绝、人工兜底三个审核样例。
  - 两台真机尚未执行时明确标记 `PENDING`，不得标记业务完成。

- [ ] **Step 7：提交验证资料**

  ```powershell
  git add -- docs/SECONDHAND-COMMENTS-AUTO-REVIEW-ACCEPTANCE.md
  git add -p -- package.json
  git diff --cached --check
  git commit -m "验证：记录商品留言与自动审核结果"
  ```

## Task 15：完成两台真机、两个真实账号验收

**Files:**

- Modify: `docs/SECONDHAND-COMMENTS-AUTO-REVIEW-ACCEPTANCE.md`

- [ ] **Step 1：准备两个真实账号**

  - 账号 A：商品卖家。
  - 账号 B：普通买家。
  - 两台手机分别登录两个不同微信账号；不能只扫描同一个开发者身份并把同一业务账号当成双账号。

- [ ] **Step 2：验证买家路径**

  账号 B 完成：

  1. 打开账号 A 的在售商品。
  2. 看到灰色空心星，收藏后变成金色实心星，重新进入仍保持。
  3. 发布明确安全留言。
  4. 联系卖家，确认原腾讯云 IM 文本通道仍可用。

- [ ] **Step 3：验证卖家路径**

  账号 A 完成：

  1. 收到站内新留言通知。
  2. 点击通知直接到对应商品和留言。
  3. 回复账号 B，回复显示“卖家”。
  4. 依次验证标记预订、重新上架、标记售出、重新上架。

- [ ] **Step 4：验证回复通知和历史保留**

  账号 B 收到回复通知并定位；账号 A 标记售出后，双方仍能读取历史留言，但不能新留言、新回复、新收藏或新建联系。

- [ ] **Step 5：验证三类审核结果**

  使用不含真实敏感信息的测试文本分别触发：

  - 微信明确通过：自动公开。
  - 微信明确违规：自动拒绝，只对提交人可见。
  - 微信复核/临时不可用：留在人工审核，不公开。

  管理后台对一个自动终态执行改判，确认站内通知不重复。

- [ ] **Step 6：验证 iOS 与 Android 布局**

  两台设备分别检查：

  - 微信胶囊不遮挡标题、卖家按钮和通知入口。
  - 键盘弹起后留言输入、回复对象和发送按钮仍可见。
  - 评论滚动和目标定位正常。
  - 收藏星形清晰，不再出现旋转方块或畸形符号。

- [ ] **Step 7：更新最终验收结论**

  只有上述步骤全部通过，才把文档状态从 `PENDING` 改成 `PASS`。任何单向消息、单机预览、绿色构建或自动测试都不能替代这一步。

- [ ] **Step 8：提交真机结果**

  ```powershell
  git add -- docs/SECONDHAND-COMMENTS-AUTO-REVIEW-ACCEPTANCE.md
  git diff --cached --check
  git commit -m "验收：完成商品留言双账号真机验证"
  ```

## 最终自检

- [ ] 设计规格中的卖家操作、留言、一级回复、卖家标签、自动审核、人工兜底、人工改判、通知定位、星形收藏和售出历史规则均有对应任务。
- [ ] 身份申请仍只走人工审核；计划没有把 `identity_application` 放入自动对象白名单。
- [ ] 公开读取不会返回其他用户的待审、审核中或拒绝留言。
- [ ] 自动审核失败关闭；任何网络失败、超时、未配置或未知响应都不会公开内容。
- [ ] OpenID、access token、微信原始响应和企业微信 Webhook 不进入日志或 API。
- [ ] 交易写入沿用 `clientRequestId`、`Idempotency-Key` 和 Outbox，不引入第二套幂等语义。
- [ ] 商品留言通知由审核终态副作用创建，重放不会重复。
- [ ] MiniApp 使用服务端真实 `sellerId`、`isSeller` 和审核状态，不使用假数据。
- [ ] 计划没有修改外卖、支付、配送、腾讯云 IM、搜索、身份审核或其他无关页面。
- [ ] 当前工作树的既有修改通过精确/分块暂存保留，没有被整体提交或覆盖。
- [ ] 完成结论严格区分自动化 PASS、共享环境结果和双机真账号验收。
