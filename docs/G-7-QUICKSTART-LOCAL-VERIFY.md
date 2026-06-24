# G-7 本地最小验收快速指引：最快看到买家端菜品图

## 0. 这份指引的目标

用最少步骤，让微信开发者工具里的买家端“外卖”tab 显示出一个带图菜品，肉眼验证：

`商家上传菜品图 → 绑定 dish reference → 上架 PublishReference → 买家接口返回 imageContentUrl → /miniapp/media/resolve → COS 短签 URL → <Image> 渲染成功`

前置假设：

- 你有真实可用的腾讯云 COS 凭证，且 Bucket 已开启 Versioning。
- 本地有 MySQL 8.x，或可用本仓库 `deploy/docker-compose.yml` 起 MySQL。
- 本指引只为“最快看到菜品图”，不是完整 G-7 手工验收。

## 1. 起后端（最小配置）

后端真实启动命令来自根目录 `package.json`：

```powershell
npm run dev:backend
```

它等价于：

```powershell
cd backend
go run ./cmd/server
```

`cmd/server` 默认监听 `HTTP_ADDR=:8080`，健康检查是：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

预期返回信封：

```json
{"code":0,"message":"ok","data":{"status":"ok"}}
```

### backend/.env 最小项

后端通过 `backend/internal/config/config.go` 读取配置。因为完整 `cmd/server` 会初始化旧 media、新 media、Meilisearch、COS、微信安全客户端，所以最小本地 `.env` 不能只配 MySQL。

建议复制根目录 `.env.example` 或 `backend/.env.example` 到 `backend/.env` 后修改：

```dotenv
APP_ENV=dev
HTTP_ADDR=:8080
JWT_SECRET=请填至少32位随机字符串

# 如果用 docker compose 的 MySQL，宿主机端口是 3307；如果你本机已有 MySQL 3306，就按你的真实端口改。
MYSQL_DSN=campus:campus@tcp(127.0.0.1:3307)/campus_circle?charset=utf8mb4&parseTime=True&loc=Local

# 后端启动时必需：trade.NewMeiliSearch 要求 host/key 非空。
MEILI_HOST=http://127.0.0.1:7700
MEILI_MASTER_KEY=请填至少16位字符串，并与启动 Meilisearch 时一致
MEILI_INDEX=products

# 真实 COS；COS_BUCKET 必须是完整 bucket 名，包含 AppID 后缀，例如 xxx-1234567890。
COS_SECRET_ID=你的腾讯云 SecretId
COS_SECRET_KEY=你的腾讯云 SecretKey
COS_BUCKET=你的完整Bucket名-1234567890
COS_REGION=ap-guangzhou
COS_APP_ID=1234567890

# 本地可填 127.0.0.1；旧 media 服务会读取。
MEDIA_PUBLIC_BASE_URL=http://127.0.0.1:8080
WECHAT_CALLBACK_TOKEN=请填至少16位随机字符串
WECHAT_API_BASE=https://api.weixin.qq.com

# 注意：完整 cmd/server 会初始化 WeChat media safety，NewWechatSafety 要求 AppID/Secret 都存在。
# 所以完整后端本地验收推荐填真实 WECHAT_APP_ID + WECHAT_APP_SECRET，用微信开发者工具真实 wx.login。
WECHAT_APP_ID=你的小程序AppID
WECHAT_APP_SECRET=你的小程序AppSecret
ALLOW_MOCK_WECHAT_LOGIN=false
```

一个容易踩的坑：`config.Validate()` 要求 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET` 必须同时为空或同时有值；但完整 `cmd/server` 又会初始化 `NewWechatSafety()`，它要求二者都非空。因此这条最小指引建议使用真实微信 AppID/Secret，而不是任意 code 的 mock 登录。

### 依赖怎么起

仓库的 `deploy/docker-compose.yml` 提供了 `mysql` 和 `meilisearch`。没有 Redis 服务；当前 `config.Load()` 也没有 Redis 配置项。

PowerShell：

```powershell
$env:MEILI_MASTER_KEY="请填至少16位字符串，并与 backend/.env 一致"
docker compose -f deploy/docker-compose.yml up -d mysql meilisearch
```

注意：

- compose MySQL 映射为宿主机 `127.0.0.1:3307`。
- 如果是首次创建 compose volume，MySQL 会加载 `backend/migrations`。
- 不要对已有验收库随手 `down -v`，会删库。

## 2. 编译小程序让“外卖”tab 出现

小程序真实命令来自根目录 `package.json`：

```powershell
npm run dev:miniapp
```

它会执行 `apps/miniapp` 的 Taro watch 编译，输出到 `apps/miniapp/dist`。

微信开发者工具：

1. 导入目录：`apps/miniapp`
2. `apps/miniapp/project.config.json` 已配置：
   - `miniprogramRoot: "dist/"`
   - `appid: "wx27a4c1be22695ed6"`（如你的真实 AppID 不同，请确认本地项目配置）
   - `setting.urlCheck: false`
3. 本地调试时仍建议在开发者工具里关闭“校验合法域名、web-view、TLS 版本以及 HTTPS 证书”。
4. 如果看不到“外卖”tab：
   - 停掉旧 watch，重新执行 `npm run dev:miniapp`
   - 微信开发者工具点“编译”
   - 仍旧没有就点“清缓存 / 清除全部缓存”，再重新打开项目。

预期：底部自定义导航出现第 5 个 tab：`外卖`。代码位置：

- `apps/miniapp/src/app.config.ts` 注册了 `pages/takeout/index`
- `apps/miniapp/src/components.tsx` 的 `BottomNav` 包含 `takeout`

## 3. 拿到登录 token

### 推荐：真实 wx.login

因为完整后端需要真实 `WECHAT_APP_ID` / `WECHAT_APP_SECRET`，推荐直接在微信开发者工具里走真实登录。小程序 API client 会从 storage 读取：

```text
domainA.token
```

请求会自动加：

```text
Authorization: Bearer <token>
```

代码位置：`apps/miniapp/src/services/api.ts`。

### 本地手动注入 token

如果你已经通过 `/api/v1/auth/wechat-login` 拿到了 token，可在微信开发者工具 Console 执行：

```javascript
wx.setStorageSync('domainA.token', '这里粘贴JWT')
```

如果 admin 页面也要用同一个 token，在浏览器控制台执行：

```javascript
localStorage.setItem('domainA.token', '这里粘贴JWT')
```

登录接口真实契约：

```http
POST http://127.0.0.1:8080/api/v1/auth/wechat-login
Content-Type: application/json

{
  "code": "wx.login 得到的 code",
  "nickname": "Local User",
  "school": "本地",
  "campus": "本地"
}
```

如果你只是本地临时需要管理员权限，可在本地库把当前用户升为管理员，然后重新登录拿新 token：

```sql
UPDATE users
SET role = 'superadmin',
    status = 'active',
    token_version = token_version + 1,
    updated_at = NOW()
WHERE id = <你的用户ID>;
```

## 4. ★造一个带图菜品（最快路径）

最终买家端只需要两件事：

1. `takeout_merchants.status = 'active'`
2. `takeout_dishes.status = 'on_shelf'`，且 `image_asset_public_id` 指向一个真实上传、审核通过、已绑定并 publish 的 media asset。

不要只靠 SQL 给 `takeout_dishes.image_asset_public_id` 塞字符串；那不会创建 `media_asset_references`，图片不会真正可见。

### 路径甲：走 UI，最真实

1. 打开小程序 “我的” → “商家中心”。
2. 未入驻态下填写店名、联系方式、位置、营业时间。
3. 上传 logo。上传按钮会走真实链路：

   `chooseMedia → compressImage → /miniapp/media/uploads → COS PUT → /uploads/:ticketId/complete`

4. 提交入驻，预期商家为 `pending_review`。
5. 打开 admin：

   ```powershell
   npm run dev:admin
   ```

   浏览器访问 `http://127.0.0.1:5173/`，注入管理员 token。

6. 在“审核”里找到 `objectType = takeout_merchant` 的任务：
   - 点“开始审核”
   - 点“通过”

   真实端点是：

   ```http
   POST /api/v1/admin/reviews/:id/start
   POST /api/v1/admin/reviews/:id/approve
   ```

7. 回到小程序商家中心，商家应进入 `active` 管理态。
8. 创建分类。
9. 创建菜品，点击“上传菜品图”。上传成功后页面会显示 image asset 短号。
10. 保存菜品，此时菜品还是 `draft`。
11. 点击“上架”。真实端点：

    ```http
    PATCH /api/v1/takeout/merchant/dishes/:dishPublicId/on-shelf
    ```

    这一步后端会调用新 media 的 `PublishReference()`，把该菜品图 reference 从 `pending/hidden` 推到 `active/visible`。

### 路径乙：最快混合路径（推荐用于只看菜品图）

这条路线把“商家审核”用本地 SQL 快捷处理，但菜品图仍然必须走小程序上传按钮，保证 COS/media 真实链路成立。

1. 用小程序商家中心提交入驻；logo 可以不传。
2. 在数据库把当前商家置为 active：

   ```sql
   SELECT id, merchant_public_id, owner_user_id, status, row_version
   FROM takeout_merchants
   WHERE owner_user_id = <你的用户ID>
   ORDER BY id DESC
   LIMIT 1;

   UPDATE takeout_merchants
   SET status = 'active',
       row_version = row_version + 1,
       updated_at = NOW()
   WHERE owner_user_id = <你的用户ID>
     AND status = 'pending_review';
   ```

3. 回小程序商家中心，进入 active 管理态。
4. 创建分类。
5. 创建菜品，必须点“上传菜品图”，等待上传完成并显示 image asset。
6. 保存菜品。
7. 点击“上架”。

这条路径的关键是：菜品图不是 SQL 伪造的，而是商家页面调用 `chooseAndUploadImages('dish', 1)` 后拿到真实 `assetId`，再由创建菜品接口传给后端：

```json
{
  "name": "测试鸡腿饭",
  "description": "本地验收用",
  "priceCents": 1200,
  "imageAssetPublicId": "真实上传返回的26位assetId",
  "sortOrder": 0
}
```

### API 方式的真实契约（可选）

如果你要自己写脚本而不是点 UI，图片上传的 HTTP 契约是：

```http
POST /api/v1/miniapp/media/uploads
Authorization: Bearer <token>

{
  "purpose": "dish",
  "sizeBytes": 12345,
  "contentMD5": "base64格式MD5",
  "contentType": "image/jpeg",
  "idempotencyKey": "本次上传唯一key"
}
```

返回：

```json
{
  "ticketId": "...",
  "assetId": "...",
  "uploadUrl": "https://...cos...签名PUT地址...",
  "requiredHeaders": {
    "contentType": "image/jpeg",
    "contentMD5": "...",
    "contentLength": 12345,
    "forbidOverwrite": "true"
  }
}
```

然后必须用返回的 `uploadUrl` 和 `requiredHeaders` 原样执行 COS `PUT`，再调用：

```http
POST /api/v1/miniapp/media/uploads/:ticketId/complete
Authorization: Bearer <token>
```

完成后拿 `assetId` 创建菜品、上架。手写这段比 UI 容易出错；本指引建议用商家页上传按钮完成这一步。

## 5. 在买家端看到菜品图

1. 微信开发者工具里点底部“外卖”tab。
2. 预期：
   - 顶部出现营业中的商家。
   - 自动选中第一个 active 商家。
   - “已上架菜品”区域出现刚才创建的菜品。
   - 菜品卡片左侧图片真实加载，不是占位块。

买家端真实渲染链路：

- `GET /api/v1/takeout/buyer/merchants`
- `GET /api/v1/takeout/buyer/merchants/:merchantPublicId/dishes`
- 后端返回 `imageContentUrl`，形如：

  ```text
  /api/v1/miniapp/media/display/dish/<dishPublicId>/0/content
  ```

- 前端 `ResolvedMediaImage` 会把它转换为：

  ```text
  /api/v1/miniapp/media/resolve/dish/<dishPublicId>/0
  ```

- resolve 返回：

  ```json
  {
    "url": "https://<bucket>.cos.<region>.myqcloud.com/...?q-signature=...",
    "expiresInSeconds": 300
  }
  ```

- `<Image>` 加载这个短签 COS URL。

在微信开发者工具 Network 面板里看：

```text
/api/v1/takeout/buyer/merchants
/api/v1/takeout/buyer/merchants/<merchantPublicId>/dishes
/api/v1/miniapp/media/resolve/dish/<dishPublicId>/0
https://...cos...myqcloud.com/...
```

重复渲染时，`apps/miniapp/src/media-image.tsx` 会按 TTL 做内存缓存；不一定每次都重新 resolve。

## 6. 卡点排查

### 小程序满屏 `ERR_CONNECTION_REFUSED`

原因：后端没起来，或不是 `127.0.0.1:8080`。

解法：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

如果不通，先看 `npm run dev:backend` 的 fatal 日志，常见是 MySQL、Meilisearch、COS、微信 AppID/Secret 配置不完整。

### 看不到“外卖”tab

原因：开发者工具加载的是旧 `dist`。

解法：

```powershell
npm run dev:miniapp
```

然后开发者工具：

1. 点“编译”
2. 不行就“清缓存 / 清除全部缓存”
3. 重新导入 `apps/miniapp`

预期 `BottomNav` 有第 5 个 tab “外卖”。

### 外卖 tab 进去是空的

原因：没有 active 商家。

检查：

```sql
SELECT merchant_public_id, name, status
FROM takeout_merchants
ORDER BY id DESC
LIMIT 10;
```

必须至少有一条 `status = 'active'`。

### 有商家，但菜品列表空

原因：没有 `on_shelf` 菜品，或菜品不属于该 active 商家。

检查：

```sql
SELECT dish_public_id, merchant_public_id, name, status, image_asset_public_id
FROM takeout_dishes
ORDER BY id DESC
LIMIT 10;
```

必须是 `status = 'on_shelf'`。

### 菜品有名字没图

常见原因：

1. `image_asset_public_id` 为空：创建菜品时没传真实 asset。
2. reference 没 publish：没有点“上架”，或上架接口失败。
3. media 不满足可见条件：reference 不是 `active/visible`，asset 不是 `final_ready + approved`。
4. COS 短签失败：COS Secret/Bucket/Region/AppID 不对。

快速查 reference：

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status,
       p.status AS projection_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
LEFT JOIN media_review_projections p ON p.asset_id = a.id
WHERE r.reference_type = 'dish'
ORDER BY r.id DESC
LIMIT 10;
```

目标状态：

```text
reference_type=dish
status=active
parent_visibility=visible
purpose=dish
storage_stage=final
technical_status=final_ready
projection_status=approved
```

### resolve 请求 401

原因：小程序没有 `domainA.token`，或 token 过期。

解法：重新登录，或在开发者工具 Console 重新注入：

```javascript
wx.setStorageSync('domainA.token', '新token')
```

### resolve 返回 404

原因：媒体网关判定不可见。回到“菜品有名字没图”的 SQL 检查 reference/asset/projection 状态。

### resolve 200，但 `<Image>` 不显示

原因多半在 COS URL：

- Bucket/Region/AppID 不匹配。
- Bucket 未开启 Versioning。
- COS 签名 URL 过期。
- 微信开发者工具网络/域名设置挡住了真实 COS 请求。

Network 面板里直接点开 COS URL；如果浏览器也打不开，看后端日志和 COS 返回码。

## 最小验证路径

如果时间很少，只做这条：

1. 起后端，`/healthz` 通。
2. `npm run dev:miniapp`，开发者工具看到“外卖”tab。
3. 用商家中心提交入驻。
4. 本地 SQL 把该商家置为 `active`。
5. 商家中心创建菜品，点“上传菜品图”，保存。
6. 点“上架”。
7. 买家端点“外卖”tab，看到该菜品图片真实显示。
8. Network 确认出现：

   ```text
   /miniapp/media/resolve/dish/<dishPublicId>/0
   https://...cos...myqcloud.com/...
   ```

看到第 7 步的图片，就已经验证了这份 quickstart 的核心目标。
