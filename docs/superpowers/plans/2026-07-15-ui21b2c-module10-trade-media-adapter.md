# UI-21b-2c Module 10 交易媒体适配 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Module 10 商品图片安全接入交易创建、审核、更新、删除和固定版本展示，同时保持前端 API、legacy 读取及数据库结构不变。

**Architecture:** Core Media 新增通用引用绑定服务，使用现有 reservation 列与 `media_asset_references` 实现预留、精确提交、发布和撤销；Trade 通过混合 adapter 在 Core Media 与 legacy gateway 间 fail-closed 路由。现有公开 content URL 先走 legacy，not found 时回退到 Core Media 固定版本 resolver。

**Tech Stack:** Go 1.x、Gin、GORM、MySQL 8.4、现有 review/write-outbox、腾讯 COS、Meilisearch、Taro/TypeScript。

---

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `backend/internal/module/media/reference_binding_types.go` | 绑定 DTO、错误、repository 端口与服务配置。 |
| `backend/internal/module/media/reference_binding_service.go` | 通用预留、精确提交、取消、发布和撤销编排。 |
| `backend/internal/module/media/reference_binding_repository.go` | MySQL 行锁、reservation、引用集合和 active ref count 原子操作。 |
| `backend/internal/module/media/reference_binding_service_test.go` | 绑定服务校验、分类和幂等单元测试。 |
| `backend/internal/module/media/reference_binding_mysql_test.go` | MySQL 并发、事务和零残留测试。 |
| `backend/internal/module/media/display_types.go` | 增加按 asset public ID 查固定版本对象的只读端口。 |
| `backend/internal/module/media/display_repository.go` | 按 asset public ID 执行 fail-closed 可见对象查询。 |
| `backend/internal/module/media/display_service.go` | 增加 `ContentURLByMediaID`。 |
| `backend/internal/module/media/display_service_test.go` | 固定版本 asset resolver 单元测试。 |
| `backend/internal/pkg/media/service.go` | 增加可选 Core Media content fallback，保持构造器向后兼容。 |
| `backend/internal/pkg/media/service_test.go` | legacy 成功、not-found fallback 和非 not-found 禁止回退测试。 |
| `backend/internal/module/trade/media_adapter.go` | `HybridMediaGateway` 与 legacy/Core DTO、token、错误映射。 |
| `backend/internal/module/trade/media_adapter_test.go` | Core、legacy、mixed、重放与更新 keep/new 路由测试。 |
| `backend/internal/module/trade/service.go` | 更新预留扩展、审核发布、删除/驳回/下架安全顺序。 |
| `backend/internal/module/trade/service_test.go` | 完整商品图片生命周期单元测试。 |
| `backend/internal/module/trade/handler.go` | 将媒体拒绝稳定映射为 409，保留 5xx/504 语义。 |
| `backend/internal/module/trade/handler_test.go` | keyed/non-keyed 媒体错误 HTTP 状态测试。 |
| `backend/internal/module/trade/media_lifecycle_mysql_test.go` | 真实 MySQL 商品、引用、审核状态生命周期测试。 |
| `backend/cmd/server/main.go` | 构造 Core binding、hybrid gateway 与 content fallback；Trade/outbox 改用 hybrid。 |
| `docs/UI-21B-2C-TRADE-WRITES-*.md` | 更新 Development、Acceptance、Residue 真实证据。 |

### Task 1: 定义 Core Media 引用绑定契约

**Files:**
- Create: `backend/internal/module/media/reference_binding_types.go`
- Create: `backend/internal/module/media/reference_binding_service.go`
- Create: `backend/internal/module/media/reference_binding_service_test.go`

- [ ] **Step 1: 写服务校验与幂等失败测试**

测试必须覆盖：非法 token、重复 asset ID、非 product purpose、legacy-only 分类、mixed 分类、exact commit replay、mismatched replay、越权 cancel、publish/revoke 重放。

```go
func TestReferenceBindingReserveRejectsDuplicateAssetIDs(t *testing.T) {
    repo := &bindingRepositoryStub{}
    service, err := NewReferenceBindingService(repo, func() time.Time {
        return time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
    })
    if err != nil { t.Fatal(err) }
    _, err = service.Reserve(context.Background(), BindingReserveInput{
        OwnerID: 7, Purpose: PurposeProduct,
        OrderedAssetPublicIDs: []string{testBindingULID1, testBindingULID1},
        Token: "m10_" + testBindingULID2,
        ExpiresAt: time.Date(2026, 7, 15, 10, 2, 0, 0, time.UTC),
    })
    if !errors.Is(err, ErrBindingInvalidAsset) {
        t.Fatalf("err=%v want ErrBindingInvalidAsset", err)
    }
    if repo.reserveCalls != 0 { t.Fatal("invalid input reached repository") }
}

func TestReferenceBindingEnsureCommittedRequiresExactCoordinates(t *testing.T) {
    repo := &bindingRepositoryStub{committed: true, committedIDs: []string{testBindingULID1}}
    service, _ := NewReferenceBindingService(repo, time.Now)
    err := service.EnsureCommitted(context.Background(), BindingCommitInput{
        OwnerID: 7, Token: "m10_" + testBindingULID2,
        ReferenceType: PurposeProduct, ReferenceID: "42",
        OrderedAssetPublicIDs: []string{testBindingULID2},
    })
    if !errors.Is(err, ErrBindingMismatch) {
        t.Fatalf("err=%v want ErrBindingMismatch", err)
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/media -run 'TestReferenceBinding' -count=1`

Expected: FAIL，提示 `NewReferenceBindingService`、`BindingReserveInput` 等尚未定义。

- [ ] **Step 3: 写绑定类型与 repository 端口**

`reference_binding_types.go` 至少定义以下精确契约：

```go
var (
    ErrBindingInvalidAsset = errors.New("invalid media asset")
    ErrBindingConflict     = errors.New("media asset already bound")
    ErrBindingMismatch     = errors.New("media binding coordinates mismatch")
    ErrBindingLegacySet    = errors.New("legacy media asset set")
)

type BindingAsset struct {
    InternalID    uint64
    AssetPublicID string
    Slot          uint16
    Kept          bool
}

type BindingReservation struct {
    Token      string
    References []BindingAsset
}

type BindingReserveInput struct {
    OwnerID              uint64
    Purpose              Purpose
    OrderedAssetPublicIDs []string
    Token                string
    ExpiresAt            time.Time
    ExistingReferenceID  string
}

type BindingCommitInput struct {
    OwnerID               uint64
    Token                 string
    ReferenceType         Purpose
    ReferenceID           string
    OrderedAssetPublicIDs []string
}

type ReferenceBindingRepository interface {
    ReserveBinding(context.Context, BindingReserveInput) ([]BindingAsset, error)
    CommitBinding(context.Context, BindingCommitInput) error
    ExactCommittedBinding(context.Context, BindingCommitInput) (bool, error)
    CancelBinding(context.Context, uint64, string) error
    PublishBinding(context.Context, Purpose, string) error
    RevokeBinding(context.Context, Purpose, string, []uint64, string) error
}
```

Token 校验必须严格接受 `m10_` 加合法 26 位 ULID；`PurposeProduct` 是当前 adapter 唯一调用 purpose，但服务保持 generic `Purpose.Valid()` 校验。

- [ ] **Step 4: 实现最小服务编排**

`ReferenceBindingService` 负责输入校验、exact replay 和 repository 错误透传，不包含 GORM：

```go
func (s *ReferenceBindingService) EnsureCommitted(ctx context.Context, input BindingCommitInput) error {
    if err := validateBindingCommit(input); err != nil { return err }
    if err := s.repository.CommitBinding(ctx, input); err == nil { return nil }
    exact, readErr := s.repository.ExactCommittedBinding(ctx, input)
    if readErr != nil { return readErr }
    if exact { return nil }
    return ErrBindingMismatch
}
```

`Revoke` 每次生成新的 ULID revocation fence；repository 对已经 exact revoked 的集合必须返回 nil。

- [ ] **Step 5: 运行单元测试**

Run: `cd backend; go test ./internal/module/media -run 'TestReferenceBinding' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交契约与服务**

```powershell
git add backend/internal/module/media/reference_binding_types.go backend/internal/module/media/reference_binding_service.go backend/internal/module/media/reference_binding_service_test.go
git commit -m "feat(UI-21b-2c): 定义媒体引用绑定契约"
```

### Task 2: 实现 MySQL reservation 与引用集合原子操作

**Files:**
- Create: `backend/internal/module/media/reference_binding_repository.go`
- Create: `backend/internal/module/media/reference_binding_mysql_test.go`

- [ ] **Step 1: 写 MySQL 并发与状态失败测试**

测试 fixture 创建两个 `purpose=product`、final/final_ready、固定 key/version 完整且 projection approved 的 Module 10 资产。用两个 goroutine 同时预留同一资产：

```go
func TestReferenceBindingOnlyOneConcurrentReservationWinsAgainstMySQL84(t *testing.T) {
    db := openModule6MySQL(t)
    fixture := createApprovedProductBindingFixture(t, db, 99_000_901)
    service, _ := NewReferenceBindingService(NewReferenceBindingRepository(db), time.Now)
    start := make(chan struct{})
    results := make(chan error, 2)
    for _, token := range []string{"m10_" + testBindingULID1, "m10_" + testBindingULID2} {
        go func(token string) {
            <-start
            _, err := service.Reserve(context.Background(), BindingReserveInput{
                OwnerID: fixture.Asset.OwnerID, Purpose: PurposeProduct,
                OrderedAssetPublicIDs: []string{fixture.Asset.AssetPublicID},
                Token: token, ExpiresAt: time.Now().Add(2 * time.Minute),
            })
            results <- err
        }(token)
    }
    close(start)
    first, second := <-results, <-results
    if (first == nil) == (second == nil) { t.Fatalf("first=%v second=%v", first, second) }
}
```

另写测试覆盖反向多资产输入不死锁、过期 reservation 接管、exact commit 重放、mismatch、publish、revoke、active ref count 与 residue=0。

- [ ] **Step 2: 运行 MySQL 测试确认失败**

Run: `cd backend; go test ./internal/module/media -run 'TestReferenceBinding.*MySQL84' -count=1`

Expected: FAIL，提示 repository constructor/method 尚未定义；若 `MYSQL_DSN` 未加载，测试必须明确 SKIP 而不是伪 PASS。

- [ ] **Step 3: 实现 reservation 查询与行锁**

新增专用 repository，继续复用同一个 `*gorm.DB`，但不扩展通用 `Repository` 接口：

```go
type GormReferenceBindingRepository struct{ db *gorm.DB }

func NewReferenceBindingRepository(db *gorm.DB) *GormReferenceBindingRepository {
    return &GormReferenceBindingRepository{db: db}
}
```

`ReserveBinding` 必须：

1. 将 asset public IDs 排序后使用 `clause.Locking{Strength: "UPDATE"}` 查询；
2. LEFT JOIN `media_review_projections`，显式 select Module 10 allowlist；
3. 全部记录存在且属于单一实现；没有 projection 且均有 legacy `public_id` 时返回 `ErrBindingLegacySet`；部分 Core/部分 legacy 返回 `ErrBindingInvalidAsset`；
4. Core 资产只接受 owner、product、final/final_ready、final key/version、projection approved、未删除；
5. 查询非 revoked 引用；只有 `ExistingReferenceID` 对应的同商品 exact keep 可以保留；
6. 有效他方 reservation 或其他父对象引用返回 `ErrBindingConflict`；
7. 事务内按输入全集更新 token 与 expiry，RowsAffected 必须等于新预留资产数。

- [ ] **Step 4: 实现 commit、cancel、publish、revoke**

`CommitBinding` 在单事务内重新锁定资产并验证 owner/token/ordered set。对每个 slot 创建：

```go
AssetReference{
    AssetID: asset.ID, ReferenceType: input.ReferenceType,
    ReferenceID: input.ReferenceID, Slot: uint16(index),
    Status: ReferencePending, ParentVisibility: ParentHidden,
    AggregateVersion: 1, OperationID: strings.TrimPrefix(input.Token, "m10_"),
    RowVersion: 1, CreatedAt: now, UpdatedAt: now,
}
```

冲突时回读 exact set；只有 asset、reference type/id、slot 和顺序完全相同才算重放。成功后只清除 owner/token 匹配的 reservation。

`PublishBinding` 在同一事务将 exact pending 引用转 active，再将 hidden 转 visible；已经 active/visible 时不增加 row version。`RevokeBinding` 将非 keep 引用 hidden，并按 pending→revoked 或 active→revoking→revoked 写入 fence。每次状态变化后按引用事实重建各资产 `active_ref_count`。

- [ ] **Step 5: 运行 MySQL 与 media 全包测试**

Run: `cd backend; go test ./internal/module/media -run 'ReferenceBinding' -count=1`

Expected: PASS，包括 concurrent winner=1、deadlock=0、residue=0。

Run: `cd backend; go test ./internal/module/media -count=1`

Expected: PASS。

- [ ] **Step 6: 提交 repository 与集成测试**

```powershell
git add backend/internal/module/media/reference_binding_repository.go backend/internal/module/media/reference_binding_mysql_test.go
git commit -m "feat(UI-21b-2c): 实现媒体引用并发预留"
```

### Task 3: 增加固定版本 asset resolver 与 legacy fallback

**Files:**
- Modify: `backend/internal/module/media/display_types.go`
- Modify: `backend/internal/module/media/display_repository.go`
- Modify: `backend/internal/module/media/display_service.go`
- Modify: `backend/internal/module/media/display_service_test.go`
- Modify: `backend/internal/pkg/media/service.go`
- Modify: `backend/internal/pkg/media/service_test.go`

- [ ] **Step 1: 写 Core Media asset resolver 失败测试**

```go
func TestMediaDisplayContentURLByMediaIDUsesFixedVersion(t *testing.T) {
    gateway := &displayGatewayStub{metadata: MediaMetadataView{MediaID: testBindingULID1}}
    repository := &displayRepositoryStub{assetObject: DisplayObjectIdentity{
        MediaID: testBindingULID1, FinalKey: "final/product.webp", FinalVersionID: "version-7",
    }}
    signer := &displaySignerStub{}
    service, _ := NewMediaDisplayService(gateway, repository, signer, MediaDisplayURLTTL)
    _, err := service.ContentURLByMediaID(context.Background(), testBindingULID1)
    if err != nil { t.Fatal(err) }
    if signer.key != "final/product.webp" || signer.version != "version-7" {
        t.Fatalf("signer=%+v", signer)
    }
}
```

补充不 approved、无 active/visible 引用、revoked、缺 final version 均返回 `ErrMediaNotVisible`/`ErrNotFound` 的测试。

- [ ] **Step 2: 写 legacy fallback 失败测试**

`internal/pkg/media/service_test.go` 增加 resolver stub：legacy `FindPublic` 返回 `ErrNotFound` 时调用 Core；legacy 成功时不调用 Core；legacy 数据库错误时禁止回退。

```go
type fallbackContentResolver struct { calls int; value string; err error }
func (r *fallbackContentResolver) ContentURLByMediaID(context.Context, string) (string, error) {
    r.calls++
    return r.value, r.err
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/media ./internal/pkg/media -run 'ContentURLByMediaID|ContentURLFallback' -count=1`

Expected: FAIL，缺少 resolver 方法与 option。

- [ ] **Step 4: 实现 Core Media 固定版本查询**

在 `MediaDisplayReadRepository` 增加：

```go
FindVisibleDisplayObjectByAsset(context.Context, string) (DisplayObjectIdentity, error)
```

SQL 必须 JOIN projection/reference，并复用 reference 查询的全部 fail-closed 条件。`ContentURLByMediaID` 先调用 gateway `GetVisibleMetadata`，再查固定 identity，最后调用 `SignedGetURL(ctx, finalKey, finalVersionID, ttl)`。

- [ ] **Step 5: 实现 legacy 可选 fallback**

为 legacy Service 增加向后兼容的 variadic option：

```go
type PublicContentResolver interface {
    ContentURLByMediaID(context.Context, string) (string, error)
}
type ServiceOption func(*Service)
func WithPublicContentResolver(value PublicContentResolver) ServiceOption {
    return func(service *Service) { service.contentResolver = value }
}
```

`NewService` 现有调用不传 option 仍能编译。`ContentURL` 只有在 `FindPublic` 返回 `ErrNotFound` 且 resolver 非 nil 时回退；其他 legacy 错误原样返回。

- [ ] **Step 6: 运行显示与 legacy 测试**

Run: `cd backend; go test ./internal/module/media ./internal/pkg/media -run 'Display|ContentURL' -count=1`

Expected: PASS。

- [ ] **Step 7: 提交 resolver**

```powershell
git add backend/internal/module/media/display_types.go backend/internal/module/media/display_repository.go backend/internal/module/media/display_service.go backend/internal/module/media/display_service_test.go backend/internal/pkg/media/service.go backend/internal/pkg/media/service_test.go
git commit -m "feat(UI-21b-2c): 支持固定版本商品图片解析"
```

### Task 4: 实现 Trade 混合媒体网关

**Files:**
- Create: `backend/internal/module/trade/media_adapter.go`
- Create: `backend/internal/module/trade/media_adapter_test.go`

- [ ] **Step 1: 写 Core、legacy、mixed 与 token 路由失败测试**

测试必须断言：

- Core Reserve 返回 `m10_` token、保持输入顺序和原 content URL；
- `ErrBindingLegacySet` 时只调用 legacy Reserve；
- mixed/invalid 不调用 legacy 宽松回退；
- Commit/EnsureCommitted/Cancel 根据 token 前缀路由；
- DeleteReference 同时安全清理 Core 与 legacy；
- `ReserveForProduct` 正确返回 keep/new；
- Core 错误只映射为交易域可识别的 `ErrMediaInvalid` 或 `ErrMediaConflict`。

```go
func TestHybridMediaGatewayRejectsMixedSetWithoutLegacyFallback(t *testing.T) {
    core := &bindingGatewayStub{reserveErr: coremedia.ErrBindingInvalidAsset}
    legacy := &legacyGatewayStub{}
    gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
    _, err := gateway.Reserve(context.Background(), 7, legacymedia.PurposeProductImage,
        []string{testBindingULID1, "legacy-id"})
    if !errors.Is(err, ErrMediaInvalid) { t.Fatalf("err=%v", err) }
    if legacy.reserveCalls != 0 { t.Fatal("mixed set reached legacy gateway") }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/trade -run 'TestHybridMediaGateway' -count=1`

Expected: FAIL，`NewHybridMediaGateway` 尚未定义。

- [ ] **Step 3: 定义交易媒体端口与错误**

```go
var (
    ErrMediaInvalid  = errors.New("invalid media asset")
    ErrMediaConflict = errors.New("media asset already bound")
)

type productMediaUpdater interface {
    ReserveForProduct(context.Context, uint, uint, legacymedia.Purpose, []string) (legacymedia.Reservation, error)
}

type productMediaPublisher interface {
    PublishProduct(context.Context, uint64) error
}
```

`HybridMediaGateway` 必须实现 legacy `Gateway` 和 trade `MediaEnsurer`，不能让 Service 或 handler 导入 Core Media 错误。

- [ ] **Step 4: 实现 Reserve 与 token 路由**

Core token 使用 `m10_` + `coremedia.NewULID()`。Core reference URL 必须保持：

```go
strings.TrimRight(publicBase, "/") + "/api/v1/media/assets/" + assetPublicID + "/content"
```

Core `ErrBindingLegacySet` 是唯一允许调用 legacy Reserve 的信号。`ErrBindingInvalidAsset`、mixed、owner/purpose/review 失败均映射为 `ErrMediaInvalid`；有效 reservation/binding 冲突映射为 `ErrMediaConflict`。

- [ ] **Step 5: 实现生命周期方法**

- `EnsureCommitted`：Core 调用 binding service exact commit，legacy 调用原 service `EnsureCommitted` 端口；
- `Commit`：按 token 路由同步路径；
- `Cancel`：只取消对应实现；
- `DeleteReference`：Core 按 product ID/keep IDs revoke，legacy 保持原删除；
- `PublishProduct`：只发布 Core product references；没有 Core 引用视为 idempotent nil；
- `ReserveForProduct`：向 Core 传 `ExistingReferenceID=strconv.FormatUint(productID, 10)`，legacy 回退保持原 Reserve。

- [ ] **Step 6: 运行 adapter 测试**

Run: `cd backend; go test ./internal/module/trade -run 'TestHybridMediaGateway' -count=1`

Expected: PASS。

- [ ] **Step 7: 提交混合网关**

```powershell
git add backend/internal/module/trade/media_adapter.go backend/internal/module/trade/media_adapter_test.go
git commit -m "feat(UI-21b-2c): 增加交易混合媒体网关"
```

### Task 5: 接通交易审核、更新、删除和 HTTP 错误语义

**Files:**
- Modify: `backend/internal/module/trade/service.go`
- Modify: `backend/internal/module/trade/service_test.go`
- Modify: `backend/internal/module/trade/handler.go`
- Modify: `backend/internal/module/trade/handler_test.go`

- [ ] **Step 1: 写完整生命周期失败测试**

扩展 `tradeMediaFake` 记录 reserve-for-product、publish、revoke 和调用顺序。新增测试：

```go
func TestProductApprovalPublishesMediaAfterParentApproval(t *testing.T) {
    repo := newTradeMemoryRepo()
    gateway := &tradeMediaLifecycleFake{}
    service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
    repo.values[42] = Product{ID: 42, SellerID: 7, ReviewStatus: review.StatusReviewing}
    if err := service.SetReviewStatus(context.Background(), 42, review.StatusApproved); err != nil { t.Fatal(err) }
    if !slices.Equal(gateway.calls, []string{"publish:42"}) { t.Fatalf("calls=%v", gateway.calls) }
    if repo.values[42].ReviewStatus != review.StatusApproved { t.Fatal("parent was not approved") }
}
```

其余四个测试使用带回调的 fake：`DeleteReference` 回调读取 repository 当前值，驳回时必须仍为 reviewing、用户删除时必须仍未 soft-delete、管理员下架时必须仍非 taken_down；任何一个父状态已经变化都立即 `t.Fatal`。更新测试输入 `[keptAsset,newAsset]`，断言调用 `ReserveForProduct(owner=7, product=42)`，传给 `DeleteReference` 的 keep internal IDs 精确等于这两个资产，并且被移除旧资产不在 keep 集合。

Handler 测试分别让 keyed 与 non-keyed create 返回 `ErrMediaInvalid`/`ErrMediaConflict`，断言 HTTP 409；deadline 保持 504，未知内部错误保持 500。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/trade -run 'ProductApprovalPublishes|ProductRejectionHides|ProductDeleteHides|ProductAdminTakedownHides|ProductUpdateUsesReserve|Media.*Status' -count=1`

Expected: FAIL，生命周期方法尚未接线或状态码仍为 500/400。

- [ ] **Step 3: 修改 Update 预留**

`Update` 在 gateway 实现 `productMediaUpdater` 时调用 `ReserveForProduct`；否则保留 legacy `Reserve`。从 reservation references 计算 keep internal IDs，新增资产 commit 为 pending/hidden；失败路径必须 cancel 新 reservation。

- [ ] **Step 4: 修改审核与隐藏顺序**

- approved：先 `repo.SetReviewStatus`，再 `PublishProduct`；publish 失败返回错误，图片保持 404；
- rejected：先 `DeleteReference(..., nil)`，再 `repo.SetReviewStatus`；
- reviewing/submitted：只更新父对象，不发布；
- `Delete`：验证 owner/sold 后先隐藏媒体，再 soft delete；
- `AdminTakedown`：验证参数后先隐藏媒体，再 taken_down；
- `Update`：在替换父对象前隐藏 remove 集合，安全优先；
- sold 状态变化不撤销媒体。

所有 Core media 错误经 trade 错误归一化，service 不向 handler 泄露 Core DTO。

- [ ] **Step 5: 修改 HTTP 映射**

在 keyed create、non-keyed create 和通用 `write` 中，将 `ErrMediaInvalid`、`ErrMediaConflict` 映射为 409。不要把数据库、outbox 或签名错误误映射为 409。

- [ ] **Step 6: 运行 trade 全包测试**

Run: `cd backend; go test ./internal/module/trade -count=1`

Expected: PASS。

- [ ] **Step 7: 提交交易生命周期**

```powershell
git add backend/internal/module/trade/service.go backend/internal/module/trade/service_test.go backend/internal/module/trade/handler.go backend/internal/module/trade/handler_test.go
git commit -m "feat(UI-21b-2c): 收口商品图片完整生命周期"
```

### Task 6: 接入 composition root 并做真实 MySQL/HTTP 回归

**Files:**
- Modify: `backend/cmd/server/main.go`
- Create: `backend/internal/module/trade/media_lifecycle_mysql_test.go`

- [ ] **Step 1: 写真实 MySQL 商品生命周期测试**

fixture 创建 seller、两个 approved Module 10 product assets、product 和 review facts，断言：

- reserve → keyed create → outbox commit 后 references 为 pending/hidden；
- product approved 后 active/visible；
- fixed content resolver 返回 final key/version；
- update keep/new/remove 正确；
- rejection/delete/takedown 后 content 不可见；
- sold 保持可见；
- 两个并发商品只能绑定同一资产一次；
- cleanup 后 products、images、outbox、assets、projections、references、users residue=0。

- [ ] **Step 2: 运行测试确认接线前失败**

Run: `cd backend; go test ./internal/module/trade -run 'TestTradeMediaLifecycleAgainstMySQL84' -count=1`

Expected: FAIL，hybrid/composition fixture 尚未完成。

- [ ] **Step 3: 修改 server 构造顺序**

1. 构造 `coreMediaGateway` 与 `mediaDisplay`；
2. 构造 legacy `mediaService` 时传 `legacymedia.WithPublicContentResolver(mediaDisplay)`；
3. 构造 `coremedia.NewReferenceBindingService(coremedia.NewReferenceBindingRepository(db), nil)`；
4. 构造 `trade.NewHybridMediaGateway(bindingService, mediaService, cfg.MediaPublicBaseURL)`；
5. `trade.NewService` 和 `trade.NewWriteOutboxProcessor` 都改用 hybrid；
6. community、legacy handler、user avatar 继续使用 legacy `mediaService`；
7. Module 10 miniapp、takeout 和 delivery wiring 保持原实例。

- [ ] **Step 4: 运行边界、后端与 MySQL 测试**

Run: `npm run check:boundaries`

Expected: `ok A-G boundaries plus independent delivery and isolated P0-4 media domains`。

Run: `cd backend; go test ./... -count=1`

Expected: 全部 PASS；真实外部测试按现有显式环境开关决定 RUN/SKIP。

- [ ] **Step 5: 提交 wiring 与 MySQL 测试**

```powershell
git add backend/cmd/server/main.go backend/internal/module/trade/media_lifecycle_mysql_test.go
git commit -m "feat(UI-21b-2c): 接通 Module 10 交易媒体运行时"
```

### Task 7: 执行真实验收并更新交付报告

**Files:**
- Modify: `docs/UI-21B-2C-TRADE-WRITES-DEVELOPMENT.md`
- Modify: `docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md`
- Modify: `docs/UI-21B-2C-TRADE-WRITES-RESIDUE.md`

- [ ] **Step 1: 构建并重启当前分支后端**

在不打印 `.env` 或 secret 的前提下，将 feature worktree 后端构建到临时 runtime；停止当前旧后端 PID，只替换 `8080` 服务，保持 MySQL `3306` 和 Meilisearch `7700` 运行。

Run: `cd backend; go build -o "$env:TEMP\codex-ui21b2c-runtime\campus-backend-ui21b2c.exe" ./cmd/server`

Expected: exit 0；新后端 `/healthz` 返回 `code=0,status=ok`。

- [ ] **Step 2: 运行真实带图创建链路**

使用隔离 seller/admin 测试身份：

1. Module 10 ticket；
2. COS PUT；
3. complete；
4. 媒体审核 approved/canUse；
5. 带图商品首次创建 201；
6. 同键同体重放 200 + `Idempotency-Replayed: true`；
7. 同键异体 409；
8. 商品审核通过；
9. Meilisearch 命中；
10. content URL 302，短签 URL 实际 GET 成功且固定 version 参数存在。

测试脚本只输出脱敏 marker、HTTP 状态、asset/product/reference ID 和 PASS/FAIL；禁止输出 JWT、UserSig、COS 签名 URL、SecretID/SecretKey 或完整 `.env`。

- [ ] **Step 3: 运行更新、撤销和 legacy 回归**

- 新资产替换后，旧 asset content 404；新 asset 在复审前 404、通过后 302；
- 用户删除、审核驳回、管理员下架后 content 404；
- sold 商品 content 保持 302；
- legacy fixture content 保持原行为；
- 收藏、联系卖家、viewer state 和腾讯 IM 双向文字 REST 回归继续 PASS；
- 微信开发者工具打开详情页，图片显示和 `previewImage` 成功。

- [ ] **Step 4: 清理真实验收数据**

通过正常 API/状态机下架或删除测试商品、撤销引用、恢复临时管理员角色、删除临时腾讯 IM 账号。新鲜 COS 资产不满足标准 GC 时记录精确数量与状态，不直接改库或强删固定版本。

- [ ] **Step 5: 运行最终自动化门禁**

Run: `npm run verify`

Expected: boundaries、Admin/MiniApp typecheck、`go test ./...` 全部 PASS。

Run: `npm --workspace apps/miniapp run build:weapp`

Expected: Webpack compiled successfully；只允许既有 Sass 与 724 KiB 包体警告。

Run: `npm run check:miniapp-static-routes`

Expected: `PASS (25 routes)`。

Run: `git diff --check`

Expected: 无输出。

Run: `git diff --name-only -- backend/migrations`

Expected: 无输出。

- [ ] **Step 6: 更新三份报告**

Development 记录实现边界与测试；Acceptance 将带图创建、固定版本访问、更新/撤销、legacy 回退和微信开发者工具结果逐项标为 PASS/FAIL；Residue 记录本地用户/商品/引用/媒体/COS 数量、运行服务 PID 与仍需标准 GC 的对象。

- [ ] **Step 7: 提交报告并验证干净状态**

```powershell
git add docs/UI-21B-2C-TRADE-WRITES-DEVELOPMENT.md docs/UI-21B-2C-TRADE-WRITES-ACCEPTANCE.md docs/UI-21B-2C-TRADE-WRITES-RESIDUE.md
git commit -m "docs(UI-21b-2c): 完成 Module 10 交易媒体真实验收"
git status --short
```

Expected: 提交成功，`git status --short` 无输出，后端与 Meilisearch 健康；只有完整真实验收全绿后才允许 owner 合并或打标签。
