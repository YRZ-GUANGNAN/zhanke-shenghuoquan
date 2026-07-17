# 二手卖家安全私信入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为二手买卖双方提供服务端授权的会话恢复入口，让卖家从首页真实私信列表进入同一腾讯 C2C 会话，并阻断第三方枚举及 URL 会话坐标注入。

**Architecture:** 后端按 `relationId + product + 当前用户参与` 单次过滤，并按调用者重新计算腾讯对端；不存在与越权共享 IM 域 `404`。小程序首页只显示后端会话列表，聊天页只接受互斥的 `productId` 或 `relationId`，腾讯坐标全部来自授权响应；首次 `401` 只允许恢复并重放一次。

**Tech Stack:** Go 1.25、Gin、GORM/MySQL、Taro 4、React 18、TypeScript、Zustand、Tencent Cloud Chat SDK、微信开发者工具 `miniprogram-automator`

---

## 文件边界

- Modify: `backend/internal/module/im/repository.go` — 参与者受限查询。
- Modify: `backend/internal/module/im/service.go` — 单关系读取和调用者对端转换。
- Modify: `backend/internal/module/im/service_test.go` — 领域测试与内存仓储。
- Modify: `backend/internal/module/im/stable_conversation_mysql_test.go` — 真实 MySQL 条件验证。
- Modify: `backend/internal/module/im/handler.go`; Create: `backend/internal/module/im/handler_test.go` — GET 契约。
- Modify: `apps/miniapp/src/services/e-im.ts`, `apps/miniapp/src/services/c-trade.ts`; Create: `apps/miniapp/src/services/session-recovery.ts` — 客户端契约与一次恢复。
- Modify: `apps/miniapp/src/components/runtime-session-gate.tsx`, `apps/miniapp/src/pages/home/index.tsx`, `apps/miniapp/src/pages/home/index.scss` — 恢复期间保持已认证首页挂载并展示真实私信列表。
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`, `apps/miniapp/src/pages/secondhand-chat/index.scss` — 双安全入口。
- Modify: `scripts/check-miniapp-static-routes.cjs` — 旧路由边界。
- Create: `.release/automator/seller-safe-conversation-smoke.js` — 本机验收脚本，不加入提交。

不修改迁移、`app.config.ts`、旧 `pages/chat`、旧 `pages/conversations`、contact 写契约或消息存储。

### Task 1: 后端参与者查询与调用者对端

**Files:**
- Modify: `backend/internal/module/im/repository.go`
- Modify: `backend/internal/module/im/service.go`
- Test: `backend/internal/module/im/service_test.go`
- Test: `backend/internal/module/im/stable_conversation_mysql_test.go`

- [ ] **Step 1: 扩展测试仓储并写失败测试**

给 `imMemoryRepo` 增加 `conversationLookupErr error`，并实现待新增接口：

```go
func (r *imMemoryRepo) FindConversationForParticipant(_ context.Context, relationID, userID uint) (Conversation, bool, error) {
	if r.conversationLookupErr != nil { return Conversation{}, false, r.conversationLookupErr }
	for _, value := range r.conversations {
		if value.ID == relationID && value.ReferenceType == "product" && (value.BuyerID == userID || value.SellerID == userID) {
			return value, true, nil
		}
	}
	return Conversation{}, false, nil
}
```

把同一内存仓储的 `ListConversations` 收紧为只返回二手商品关系，确保服务测试与生产查询遵守同一边界：

```go
func (r *imMemoryRepo) ListConversations(_ context.Context, userID uint, _, _ int) ([]Conversation, int64, error) {
	values := []Conversation{}
	for _, value := range r.conversations {
		if value.ReferenceType == "product" && (value.BuyerID == userID || value.SellerID == userID) {
			values = append(values, value)
		}
	}
	return values, int64(len(values)), nil
}
```

在 `service_test.go` 加入：

```go
func TestConversationForParticipantUsesCallerSpecificPeer(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[2] = UserMapping{UserID: 2, IMUserID: "campus_2"}
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, ConversationID: "stale", BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9}}
	service := NewService(repo, &importerFake{}, "1400000000", "secret")
	buyer, err := service.Conversation(context.Background(), 7, 2)
	if err != nil || buyer.PeerUserID != "campus_3" || buyer.ConversationID != "C2Ccampus_3" { t.Fatalf("buyer=%+v err=%v", buyer, err) }
	seller, err := service.Conversation(context.Background(), 7, 3)
	if err != nil || seller.PeerUserID != "campus_2" || seller.ConversationID != "C2Ccampus_2" { t.Fatalf("seller=%+v err=%v", seller, err) }
}

func TestConversationForParticipantHidesMissingUnauthorizedAndNonProduct(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.conversations = []Conversation{
		{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9},
		{ID: 8, BuyerID: 2, SellerID: 3, ReferenceType: "delivery", ReferenceID: 10},
	}
	service := NewService(repo, &importerFake{}, "1400000000", "secret")
	for _, input := range [][2]uint{{7, 99}, {999, 2}, {8, 2}} {
		if _, err := service.Conversation(context.Background(), input[0], input[1]); !errors.Is(err, ErrConversationNotFound) { t.Fatalf("input=%v err=%v", input, err) }
	}
}

func TestConversationForParticipantIsReadOnly(t *testing.T) {
	stamp := time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC)
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9, CreatedAt: stamp, UpdatedAt: stamp}}
	before := append([]Conversation(nil), repo.conversations...)
	importer := &importerFake{}
	if _, err := NewService(repo, importer, "1400000000", "secret").Conversation(context.Background(), 7, 2); err != nil { t.Fatal(err) }
	if len(importer.users) != 0 || !reflect.DeepEqual(before, repo.conversations) { t.Fatalf("imports=%v before=%+v after=%+v", importer.users, before, repo.conversations) }
}

func TestConversationsOnlyReturnsProductRelations(t *testing.T) {
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{
		{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9},
		{ID: 8, BuyerID: 2, SellerID: 3, ReferenceType: "delivery", ReferenceID: 10},
	}
	page, err := NewService(repo, &importerFake{}, "1400000000", "secret").Conversations(context.Background(), 2, 1, 20)
	if err != nil || len(page.Items) != 1 || page.Items[0].RelationID != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
```

- [ ] **Step 2: 确认测试先失败**

Run（工作目录 `backend`）: `go test ./internal/module/im -run 'TestConversationForParticipant|TestConversationsOnlyReturnsProductRelations' -count=1`

Expected: FAIL，包含 `service.Conversation undefined` 或 Repository 接口缺方法。

- [ ] **Step 3: 实现仓储单次过滤**

`Repository` 增加：

```go
FindConversationForParticipant(context.Context, uint, uint) (Conversation, bool, error)
```

`GormRepository` 增加：

```go
func (r *GormRepository) FindConversationForParticipant(ctx context.Context, relationID, userID uint) (Conversation, bool, error) {
	var value Conversation
	err := r.db.WithContext(ctx).Where("id = ? AND reference_type = ? AND (buyer_id = ? OR seller_id = ?)", relationID, "product", userID, userID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) { return Conversation{}, false, nil }
	if err != nil { return Conversation{}, false, err }
	return value, true, nil
}
```

同时把现有 `ListConversations` 的查询收紧为二手商品关系；括号必须保留，避免 `OR` 绕过类型过滤：

```go
query := r.db.WithContext(ctx).Model(&Conversation{}).
	Where("reference_type = ? AND (buyer_id = ? OR seller_id = ?)", "product", userID, userID)
```

- [ ] **Step 4: 实现服务哨兵与共享转换**

`service.go` 增加：

```go
var ErrConversationNotFound = errors.New("conversation not found or access denied")

func (s *Service) Conversation(ctx context.Context, relationID, userID uint) (ConversationContext, error) {
	value, found, err := s.repo.FindConversationForParticipant(ctx, relationID, userID)
	if err != nil { return ConversationContext{}, err }
	if !found { return ConversationContext{}, ErrConversationNotFound }
	return s.contextForParticipant(ctx, value, userID)
}

func (s *Service) contextForParticipant(ctx context.Context, value Conversation, userID uint) (ConversationContext, error) {
	var peerID uint
	switch userID { case value.BuyerID: peerID = value.SellerID; case value.SellerID: peerID = value.BuyerID; default: return ConversationContext{}, ErrConversationNotFound }
	mapping, err := s.repo.FindMapping(ctx, peerID)
	if err != nil { return ConversationContext{}, fmt.Errorf("find IM peer mapping: %w", err) }
	return toContext(value, mapping.IMUserID), nil
}
```

把 `Conversations` 循环替换为：

```go
for _, value := range values {
	item, err := s.contextForParticipant(ctx, value, userID)
	if err != nil { return ConversationPage{}, err }
	result = append(result, item)
}
```

- [ ] **Step 5: 验证领域层并补真实 MySQL 断言**

Run（工作目录 `backend`）: `go test ./internal/module/im -count=1`

Expected: PASS；未设置 `TEST_MYSQL_DSN` 时真实 MySQL 用例明确 SKIP。

先把 `TestStableProductConversationAgainstMySQL` 的 conversation cleanup 收紧到本测试的买卖双方，并在持久化 `persistedFirst` 后加入 participant、类型过滤与只读断言：

```go
_ = db.Where("buyer_id = ? AND seller_id = ?", buyerID, sellerID).Delete(&Conversation{}).Error
```

```go
nonProduct := Conversation{ConversationID: "C2Ccampus_delivery", BuyerID: buyerID, SellerID: sellerID, ReferenceType: "delivery", ReferenceID: productID + 10}
if err := db.Create(&nonProduct).Error; err != nil { t.Fatal(err) }
value, found, err := repo.FindConversationForParticipant(ctx, persistedFirst.ID, buyerID)
if err != nil || !found || value.ID != persistedFirst.ID { t.Fatalf("participant lookup: found=%v value=%+v err=%v", found, value, err) }
if _, found, err := repo.FindConversationForParticipant(ctx, persistedFirst.ID, seed+99); err != nil || found { t.Fatalf("third-party leak: found=%v err=%v", found, err) }
listed, total, err := repo.ListConversations(ctx, buyerID, 1, 20)
if err != nil || total != 1 || len(listed) != 1 || listed[0].ReferenceType != "product" { t.Fatalf("product list: total=%d values=%+v err=%v", total, listed, err) }
var unchanged Conversation
if err := db.First(&unchanged, persistedFirst.ID).Error; err != nil { t.Fatal(err) }
if !unchanged.CreatedAt.Equal(persistedFirst.CreatedAt) || !unchanged.UpdatedAt.Equal(persistedFirst.UpdatedAt) { t.Fatalf("read changed timestamps: before=%+v after=%+v", persistedFirst, unchanged) }
```

Run（工作目录 `backend`）: `go test ./internal/module/im -run TestStableProductConversationAgainstMySQL -count=1`

Expected: 有 DSN 时 PASS；无 DSN 时 SKIP，不记为真实 MySQL PASS。

- [ ] **Step 6: 提交领域层**

```bash
git add backend/internal/module/im/repository.go backend/internal/module/im/service.go backend/internal/module/im/service_test.go backend/internal/module/im/stable_conversation_mysql_test.go
git commit -m "feat: 增加参与者安全会话查询"
```

### Task 2: HTTP 单关系契约

**Files:**
- Modify: `backend/internal/module/im/handler.go`
- Create: `backend/internal/module/im/handler_test.go`

- [ ] **Step 1: 写失败的 Handler 契约测试**

创建测试辅助函数；成功场景直接注入 `domainA.user`，未认证场景走真实 `AuthRequired`：

```go
func performConversation(handler *Handler, path string, userID uint) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/im/conversations/:relationId", func(c *gin.Context) { c.Set("domainA.user", user.User{ID: userID}); handler.conversation(c) })
	out := httptest.NewRecorder()
	router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
	return out
}

func TestConversationHandlerStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newIMMemoryRepo()
	repo.mappings[3] = UserMapping{UserID: 3, IMUserID: "campus_3"}
	repo.conversations = []Conversation{{ID: 7, BuyerID: 2, SellerID: 3, ReferenceType: "product", ReferenceID: 9}}
	h := NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil)
	if out := performConversation(h, "/im/conversations/7", 2); out.Code != http.StatusOK { t.Fatalf("body=%s", out.Body.String()) }
	for _, out := range []*httptest.ResponseRecorder{performConversation(h, "/im/conversations/7", 99), performConversation(h, "/im/conversations/999", 2)} {
		var body response.Body; _ = json.Unmarshal(out.Body.Bytes(), &body)
		if out.Code != http.StatusNotFound || body.Message != "conversation not found or access denied" { t.Fatalf("status=%d body=%s", out.Code, out.Body.String()) }
	}
	if out := performConversation(h, "/im/conversations/0", 2); out.Code != http.StatusBadRequest { t.Fatalf("body=%s", out.Body.String()) }
}

func TestConversationHandlerSanitizesStorageFailure(t *testing.T) {
	repo := newIMMemoryRepo(); repo.conversationLookupErr = errors.New("SELECT secret")
	out := performConversation(NewHandler(NewService(repo, &importerFake{}, "1400000000", "secret"), nil), "/im/conversations/7", 2)
	if out.Code != 500 || strings.Contains(out.Body.String(), "SELECT") || !strings.Contains(out.Body.String(), "get conversation failed") { t.Fatalf("body=%s", out.Body.String()) }
}

func TestConversationRouteRequiresAuthentication(t *testing.T) {
	auth := user.NewService(nil, "test-secret", "", "", false)
	router := gin.New(); NewHandler(NewService(newIMMemoryRepo(), &importerFake{}, "1400000000", "secret"), auth).Register(router.Group("/im"))
	out := httptest.NewRecorder(); router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/im/conversations/7", nil))
	if out.Code != http.StatusUnauthorized { t.Fatalf("body=%s", out.Body.String()) }
}
```

Imports：`encoding/json`、`errors`、`net/http`、`net/http/httptest`、`strings`、`testing`、`user`、`response`、`gin`。

- [ ] **Step 2: 确认测试先失败**

Run（工作目录 `backend`）: `go test ./internal/module/im -run 'TestConversationHandler|TestConversationRoute' -count=1`

Expected: FAIL，包含 `handler.conversation undefined`。

- [ ] **Step 3: 注册并实现 Handler**

`Register` 增加 `auth.GET("/conversations/:relationId", h.conversation)`；imports 增加 `errors`；实现：

```go
func (h *Handler) conversation(c *gin.Context) {
	relationID, err := strconv.ParseUint(c.Param("relationId"), 10, strconv.IntSize)
	if err != nil || relationID == 0 { response.Fail(c, http.StatusBadRequest, "invalid relation id"); return }
	value, err := h.service.Conversation(c, uint(relationID), user.CurrentUserID(c))
	if errors.Is(err, ErrConversationNotFound) { response.Fail(c, http.StatusNotFound, "conversation not found or access denied"); return }
	if err != nil { response.Fail(c, http.StatusInternalServerError, "get conversation failed"); return }
	response.OK(c, value)
}
```

- [ ] **Step 4: 验证并提交 HTTP 契约**

Run（工作目录 `backend`）: `go test ./internal/module/im -count=1`

Expected: PASS，401/400/404/500 与规格一致。

```bash
git add backend/internal/module/im/handler.go backend/internal/module/im/handler_test.go
git commit -m "feat: 暴露安全会话恢复接口"
```

### Task 3: 前端服务与一次性 session 恢复

**Files:**
- Modify: `apps/miniapp/src/services/e-im.ts`
- Modify: `apps/miniapp/src/services/c-trade.ts`
- Create: `apps/miniapp/src/services/session-recovery.ts`

- [ ] **Step 1: 收紧 DTO 并增加 GET 客户端**

`c-trade.ts` 改为：

```ts
export interface IMConversationContext {
  relationId:number; conversationId:string; peerUserId:string; productId:number; buyerId:number; sellerId:number
}
```

`e-im.ts` 保留列表并改为：

```ts
interface ConversationPage { items:IMConversationContext[]; page:number; pageSize:number; total:number }
export async function listIMConversations(page=1,pageSize=20):Promise<IMConversationContext[]>{
  return(await request<ConversationPage>(`/im/conversations?page=${page}&pageSize=${pageSize}`)).items
}
export function getIMConversation(relationId:number):Promise<IMConversationContext>{return request(`/im/conversations/${relationId}`)}
```

- [ ] **Step 2: 创建明确的一次恢复器**

`session-recovery.ts` 完整内容：

```ts
import { isApiError } from './api'
import { useSessionStore } from '../stores/session'

export async function requestWithSingleSessionRecovery<T>(operation:()=>Promise<T>):Promise<T>{
  try{return await operation()}catch(error){
    if(!isApiError(error)||error.status!==401)throw error
    await useSessionStore.getState().retry()
    if(useSessionStore.getState().status!=='authenticated')throw error
    return operation()
  }
}
```

`retry()` 会把失败写入 store 后正常 resolve；因此必须检查 `status === 'authenticated'`，不能以“没有 reject”作为恢复成功。

- [ ] **Step 3: 类型检查并提交**

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS。

```bash
git add apps/miniapp/src/services/e-im.ts apps/miniapp/src/services/c-trade.ts apps/miniapp/src/services/session-recovery.ts
git commit -m "feat: 增加会话读取与单次登录恢复"
```

### Task 4: 首页真实私信四态

**Files:**
- Modify: `apps/miniapp/src/components/runtime-session-gate.tsx`
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`

- [ ] **Step 1: 让首页会话恢复不丢失重放结果**

给 `RuntimeSessionGate` 增加 opt-in 属性，只有从 `authenticated` 直接进入的那一段 `booting` 才保持 children；初次登录、匿名、最终错误及错误态手动重试仍显示原门禁：

```tsx
import { useRef, type ReactNode } from 'react'

interface RuntimeSessionGateProps {
  children: ReactNode
  preserveAuthenticatedChildrenDuringRetry?: boolean
}

export default function RuntimeSessionGate({children,preserveAuthenticatedChildrenDuringRetry=false}:RuntimeSessionGateProps){
  const status=useSessionStore((state)=>state.status)
  const error=useSessionStore((state)=>state.error)
  const retry=useSessionStore((state)=>state.retry)
  const previousStatus=useRef(status)
  const preserveCurrentBoot=useRef(false)
  if(status==='booting'&&previousStatus.current==='authenticated')preserveCurrentBoot.current=true
  if(status!=='booting')preserveCurrentBoot.current=false
  previousStatus.current=status
  if(status==='booting'&&preserveAuthenticatedChildrenDuringRetry&&preserveCurrentBoot.current)return <>{children}</>
  if(status==='idle'||status==='booting'){
    return <View className='runtime-session-state'><Text className='runtime-session-state__title'>正在登录...</Text></View>
  }
  if(status!=='authenticated'){
    return <View className='runtime-session-state'>
      <Text className='runtime-session-state__title'>暂时无法登录</Text>
      <Text className='runtime-session-state__message'>{error||'请重新尝试微信登录'}</Text>
      <View className='runtime-session-state__retry' onClick={()=>void retry()}><Text>重试</Text></View>
    </View>
  }
  return <>{children}</>
}
```

首页最外层改为 `<RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry>`。这样列表首次 `401` 的 `retry()` 不会卸载 `HomeContent`，重放结果仍能写回当前 loading/empty/ready/error 状态；其他使用该组件的页面不传属性，行为不变。

- [ ] **Step 2: 删除静态数据并加入状态**

删除 `Conversation`、`const conversations` 与 `openConversation`；把 Taro import 改成 `import TaroRuntime, { useDidShow } from '@tarojs/taro'`，并增加 `isApiError`、`getProduct`、`listIMConversations`、`IMConversationContext`、`requestWithSingleSessionRecovery` imports。加入：

```ts
type ConversationItem={context:IMConversationContext;product:Product|null}
type ConversationStatus='idle'|'loading'|'empty'|'ready'|'error'
const [conversationItems,setConversationItems]=useState<ConversationItem[]>([])
const [conversationStatus,setConversationStatus]=useState<ConversationStatus>('idle')
const conversationLoadingRef=useRef(false)
const conversationRequestVersion=useRef(0)
```

- [ ] **Step 3: 实现加载、一次恢复与重新可见刷新**

```ts
const loadConversations=useCallback(async()=>{
  if(!mountedRef.current||conversationLoadingRef.current)return
  conversationLoadingRef.current=true
  const version=++conversationRequestVersion.current
  setConversationStatus('loading')
  try{
    const contexts=await requestWithSingleSessionRecovery(()=>listIMConversations(1,20))
    const items=await Promise.all(contexts.map(async(context)=>{
      try{return{context,product:await getProduct(String(context.productId))}}catch{return{context,product:null}}
    }))
    if(!mountedRef.current||conversationRequestVersion.current!==version)return
    setConversationItems(items);setConversationStatus(items.length?'ready':'empty')
  }catch(error){
    if(!mountedRef.current||conversationRequestVersion.current!==version)return
    setConversationItems([]);setConversationStatus('error')
    if(isApiError(error)&&error.status===401)void Taro.showToast({title:'登录恢复失败，请点击重试',icon:'none'})
  }finally{if(conversationRequestVersion.current===version)conversationLoadingRef.current=false}
},[])

useEffect(()=>{if(activeTab==='messages'&&conversationStatus==='idle')void loadConversations()},[activeTab,conversationStatus,loadConversations])
useDidShow(()=>{if(activeTab==='messages')void loadConversations()})
```

卸载 cleanup 增加 `conversationRequestVersion.current += 1` 和 `conversationLoadingRef.current = false`。

- [ ] **Step 4: 替换私信 UI**

```tsx
{activeTab==='messages'&&(
  <View className='student-message-panel'>
    {conversationStatus==='loading'||conversationStatus==='idle'?<View className='student-message-state'><Text>正在读取真实私信...</Text></View>:
    conversationStatus==='error'?<View className='student-message-state student-message-state--retry' onClick={()=>void loadConversations()}><Text>私信加载失败，点击重试</Text></View>:
    conversationStatus==='empty'?<View className='student-message-state'><Text>还没有二手交易私信</Text></View>:
    conversationItems.map((item)=><View key={item.context.relationId} className='student-message-row' onClick={()=>void Taro.navigateTo({url:`/pages/secondhand-chat/index?relationId=${encodeURIComponent(String(item.context.relationId))}`})}>
      <View className='student-message-row__avatar'>{item.product?.images[0]?<Image className='student-message-row__avatar-image' src={item.product.images[0]} mode='aspectFill'/>:<Text>聊</Text>}</View>
      <View className='student-message-row__main'><Text className='student-message-row__name'>{item.product?.title||`二手商品 #${item.context.productId}`}</Text><Text className='student-message-row__preview'>二手交易私信</Text></View>
    </View>)}
    {conversationStatus==='ready'&&<Text className='student-home-footnote'>仅展示最近 20 个会话</Text>}
  </View>
)}
```

- [ ] **Step 5: 样式、检查和提交**

```scss
.student-message-row__avatar-image{width:100%;height:100%;border-radius:inherit}
.student-message-state{display:flex;align-items:center;justify-content:center;min-height:240rpx;color:$text-muted;font-size:25rpx;text-align:center}
.student-message-state--retry{color:$brand-primary;font-weight:700}
```

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS；首页无硬编码会话、时间、未读或伪摘要。

```bash
git add apps/miniapp/src/components/runtime-session-gate.tsx apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss
git commit -m "feat: 首页展示真实二手私信"
```

### Task 5: 聊天页双安全入口

**Files:**
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.tsx`
- Modify: `apps/miniapp/src/pages/secondhand-chat/index.scss`

- [ ] **Step 1: 增加入口解析和参与者校验**

增加 `getIMConversation`、`requestWithSingleSessionRecovery`、`useSessionStore` imports，并加入：

```ts
type ChatEntry={kind:'product'|'relation';id:string}
type ViewerRole='buyer'|'seller'
function parseChatEntry(params:Record<string,string|undefined>):ChatEntry|null{
  const productId=params.productId?.trim()||'',relationId=params.relationId?.trim()||''
  if(Boolean(productId)===Boolean(relationId))return null
  const id=productId||relationId
  return /^[1-9]\d*$/.test(id)?{kind:productId?'product':'relation',id}:null
}
function viewerRole(context:IMConversationContext,currentUserId:string):ViewerRole|null{
  if(String(context.buyerId)===currentUserId)return'buyer'
  if(String(context.sellerId)===currentUserId)return'seller'
  return null
}
```

- [ ] **Step 2: 重写授权初始化顺序**

组件改用路由快照，不订阅恢复期间会清空再恢复的 `profile.id`：

```ts
const params=Taro.getCurrentInstance().router?.params??{}
const entry=parseChatEntry(params)
const [role,setRole]=useState<ViewerRole|null>(null)
```

在 effect 外增加会话门禁；它会复用正在进行的 bootstrap，且只有最终状态为 `authenticated` 并有用户 ID 时才允许请求会话上下文：

```ts
async function requireAuthenticatedUserId():Promise<string>{
  let session=useSessionStore.getState()
  if(session.status!=='authenticated'){
    await session.retry()
    session=useSessionStore.getState()
  }
  if(session.status!=='authenticated'||!session.profile.id)throw new Error('登录恢复失败')
  return session.profile.id
}
```

effect 先清空 `context/role/messages/error`；无入口直接错误，绝不调 SDK。异步主体替换为：

```ts
if(!entry){setStatus('error');setErrorMessage('私信入口无效，请从商品详情或首页重新进入。');return()=>{active=false}}
void(async()=>{
  try{
    await requireAuthenticatedUserId()
    const nextContext=await requestWithSingleSessionRecovery(()=>entry.kind==='product'?contactSeller(entry.id):getIMConversation(Number(entry.id)))
    const recoveredSession=useSessionStore.getState()
    if(recoveredSession.status!=='authenticated'||!recoveredSession.profile.id)throw new Error('登录恢复失败')
    const nextRole=viewerRole(nextContext,recoveredSession.profile.id)
    if(!nextRole||!nextContext.conversationId||!nextContext.peerUserId)throw new Error('会话参与者校验失败')
    let nextProduct:Product|null=null
    try{nextProduct=await getProduct(String(nextContext.productId))}catch{nextProduct=null}
    if(!active)return
    setContext(nextContext);setProduct(nextProduct);setRole(nextRole)
    unsubscribe=await subscribeTencentMessages(nextContext,(incoming)=>{if(active)setMessages((current)=>mergeMessages(current,incoming.filter((message)=>Boolean(messageText(message)))))})
    if(!active){unsubscribe();unsubscribe=undefined;return}
    const history=await loadTencentMessages(nextContext)
    if(active){setMessages((current)=>mergeMessages(current,history.filter((message)=>Boolean(messageText(message)))));setStatus('ready')}
  }catch(error){unsubscribe?.();unsubscribe=undefined;if(!active)return;setContext(null);setRole(null);setStatus('error');const message=chatErrorMessage(error);setErrorMessage(message);showToast(message)}
})()
```

effect 依赖仅为 `[entry?.id,entry?.kind,reloadKey]`；不要订阅 `profile.id` 或 `status`，避免 `retry()` 在清空/恢复 store 时重入初始化。cleanup 仍设 `active=false` 并解除订阅。

- [ ] **Step 3: 固定错误、视角和发送门禁**

API 错误映射：400“私信入口无效…”、401/403“登录状态已失效…”、404“会话不存在或无权访问”、409/422“当前宝贝无法发起私信”。派生值：

```ts
const peerName=role==='seller'?'交易对方':product?.sellerName||'校园卖家'
const peerAvatar=role==='seller'?'买':peerName.trim().charAt(0)||'校'
const peerAvatarImage=role==='buyer'?product?.sellerAvatar:''
const peerState=role==='seller'?'买家 · 腾讯云 IM':`${product?.campus||'校内'} · 腾讯云 IM`
const canOpenSellerProfile=role==='buyer'&&Boolean(product?.sellerId)
const resolvedProductId=context?String(context.productId):entry?.kind==='product'?entry.id:''
```

`openSellerProfile` 首行改为 `if(!canOpenSellerProfile||!product?.sellerId)return`。头像只能使用 `peerAvatarImage`，卖家侧不得继续读取自己的 `product.sellerAvatar`；副标题 ready 态使用 `peerState`。主页按钮替换为：

```tsx
<View
  className={`secondhand-chat-header__home ${canOpenSellerProfile?'':'secondhand-chat-header__home--disabled'}`}
  onClick={canOpenSellerProfile?openSellerProfile:undefined}
>
  <Text>{canOpenSellerProfile?'主页':'交易中'}</Text>
</View>
```

商品 fallback 为 ``二手商品 #${resolvedProductId||'-'}``；查看按钮只用 `resolvedProductId`，并在为空时附加 `secondhand-chat-product__button--disabled`。空会话提示按角色显示“回复买家确认交易”或“发送第一条文字消息，与卖家确认宝贝情况”。发送门禁改为：

```ts
if(!context||!role||status!=='ready'||sending)return
```

- [ ] **Step 4: 样式、检查和提交**

```scss
.secondhand-chat-header__home--disabled{color:$text-faint;background:$bg-screen}
.secondhand-chat-product__button--disabled{color:$text-faint;background:$bg-screen}
```

Run: `npm --prefix apps/miniapp run typecheck`

Expected: PASS；无效入口和非参与者上下文在 subscribe/load/send 前失败；商品补充失败不阻断已授权会话。

```bash
git add apps/miniapp/src/pages/secondhand-chat/index.tsx apps/miniapp/src/pages/secondhand-chat/index.scss
git commit -m "feat: 支持买卖双方安全进入私信"
```

### Task 6: 静态边界与完整代码门禁

**Files:**
- Modify: `scripts/check-miniapp-static-routes.cjs`

- [ ] **Step 1: 增加旧路由与首页安全检查**

```js
const forbiddenLegacyRoutes=['pages/chat/index','pages/conversations/index']
const registeredLegacyRoutes=forbiddenLegacyRoutes.filter((route)=>registered.has(route))
if(registeredLegacyRoutes.length)throw new Error(`legacy chat routes must stay unregistered: ${registeredLegacyRoutes.join(', ')}`)
```

在 broken target 检查后加入：

```js
const homeSource=fs.readFileSync(path.join(src,'pages','home','index.tsx'),'utf8')
if(/const\s+conversations\s*[:=]/.test(homeSource))throw new Error('home messages must not use a hard-coded conversations array')
if(/conversationId|peerUserId|buyerId|sellerId|peerName|peerAvatar|scene=/.test(homeSource))throw new Error('home conversation navigation must carry relationId only')
if(!/RuntimeSessionGate\s+preserveAuthenticatedChildrenDuringRetry/.test(homeSource))throw new Error('home must preserve an authenticated mount during one-shot session recovery')
```

- [ ] **Step 2: 运行精确及全仓门禁**

```bash
npm run check:miniapp-static-routes
npm --prefix apps/miniapp run typecheck
npm run check:boundaries
npm run build:miniapp
npm run verify
npm run build
Set-Location backend
go test ./...
```

Expected: 全部退出码 0；静态路由输出 `PASS (25 routes)`。仓库没有 Jest/Vitest miniapp runner，不能声称前端单元测试通过。

- [ ] **Step 3: 提交边界检查**

```bash
git add scripts/check-miniapp-static-routes.cjs
git commit -m "test: 锁定私信安全路由边界"
```

### Task 7: 微信开发者工具自动化与双账号 UAT

**Files:**
- Create: `.release/automator/seller-safe-conversation-smoke.js`（不 `git add`）

- [ ] **Step 1: 创建不输出凭证的自动化脚本**

```js
'use strict'
const automator=require('./node_modules/miniprogram-automator')
async function main(){
  const app=await automator.connect({wsEndpoint:'ws://127.0.0.1:9420'}),checks=[]
  for(const url of ['/pages/secondhand-chat/index','/pages/secondhand-chat/index?productId=1&relationId=1','/pages/secondhand-chat/index?relationId=0','/pages/secondhand-chat/index?conversationId=C2Cforged&peerUserId=forged']){
    const page=await app.reLaunch(url);await new Promise((resolve)=>setTimeout(resolve,1200))
    checks.push({case:url.split('?')[1]||'missing',errorVisible:Boolean(await page.$('.secondhand-chat-state__title'))})
  }
  const home=await app.reLaunch('/pages/home/index');await new Promise((resolve)=>setTimeout(resolve,1500))
  const tabs=await home.$$('.student-home-header__tab');if(!tabs[0])throw new Error('messages tab missing')
  await tabs[0].tap();await new Promise((resolve)=>setTimeout(resolve,2500))
  const rows=await home.$$('.student-message-row'),state=await home.$('.student-message-state')
  console.log(JSON.stringify({invalid:checks,home:rows.length?'ready':state?'empty-or-error':'unknown'}));app.disconnect()
}
main().catch((error)=>{console.error(error.message);process.exitCode=1})
```

- [ ] **Step 2: 运行自动化并如实记录边界**

先在开发者工具开启 9420 端口并导入 `apps/miniapp`。

Run: `& 'D:\3314\微信web开发者工具\node-18.exe' '.release\automator\seller-safe-conversation-smoke.js'`

Expected: 4 个 `errorVisible=true`；首页为 `ready` 或 `empty-or-error`，Console 无未捕获异常。该脚本只证明路由 fail-closed 和当前首页状态，不能替代双账号收发。

- [ ] **Step 3: 开发者工具补齐状态/重放验收**

1. 验证首页 empty/error/retry/ready；断网恢复必须重新请求。
2. 作废一次 Token：首页保持当前私信页签挂载，只有 `retry()` 后 store 为 `authenticated` 才重放一次并落入四态；连续两个 401 停在 error，无第三次请求。
3. 点击真实项后 URL 只含 `relationId`，首页未调用 contact；product 入口调用 contact，relation 入口调用 GET。
4. 伪造坐标不成为 SDK 参数；页面卸载后订阅释放。

Expected: 可观察项逐项 PASS；无法复现的状态写 PENDING 和阻断，不写假 PASS，证据不含 Token/UserSig。

- [ ] **Step 4: 部署后执行真实双账号 UAT**

1. 买家从已审核商品 `1` 发送 `UAT-BUYER-20260717-01`。
2. 卖家从首页 `relationId` 进入，显示“交易对方/买家”、禁用主页，看到买家标记。
3. 卖家回复 `UAT-SELLER-20260717-01`；买家实时收到并重进可读历史。
4. 第三账号访问同一 relation GET，得到与不存在完全相同的 404。
5. 服务日志和验收证据不含 Authorization、Token、UserSig 或消息正文。

Expected: 全部通过才记“真实双账号 PASS”；缺账号或腾讯环境异常时记“代码 PASS / 真实 UAT PENDING”。

- [ ] **Step 5: 检查最终范围**

Run: `git status --short`

Expected: 受控代码均已在前述中文提交中提交；只剩被忽略的 `.release` 验收产物，无迁移、旧聊天页或 `app.config.ts` 改动。
