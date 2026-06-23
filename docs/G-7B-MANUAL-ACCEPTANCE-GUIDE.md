# G-7b 商家端 + Media 图片链路本地真机验收操作手册

版本：v1.0  
目的：指导 owner 在本地真实后端、真实 MySQL、真实腾讯云 COS/STS、真实微信开发者工具环境中，手动验证 G-7b 商家端与 media 图片链路。

## 0. 重要结论与验收边界

本手册基于当前真实代码与配置整理，不假设尚未合并或尚未实现的能力。

当前后端 `master` 已具备：

- `/api/v1/takeout/merchant/*` 商家角色 API；
- media 上传票据、STS 授权、COS 上传完成、审核投影、Consumer；
- media reference 从 `hidden` 推进到 `visible` 的能力；
- 商家审核通过、菜品上架时触发 media reference 发布；
- admin 审核与管理能力。

当前主仓库 `master` 尚未包含完整商家小程序页面；商家端页面位于独立 worktree/分支：

- `C:\Users\33141\Documents\g7b-ui-merchant`
- 分支：`feat/g7b-ui-merchant`

因此，如果要在微信开发者工具里操作“商家中心”页面，请使用该 worktree，或先由 owner 合并对应分支后再使用主仓库。

另外，当前代码的 MediaGateway 是只读领域服务，返回媒体可见性/展示元数据，不直接暴露 COS 原始 URL，也没有面向小程序的通用“媒体展示 URL”HTTP 接口。G-7b 商家小程序页面可以证明：

- 图片被选择并本地预览；
- 图片上传成功并拿到 `assetId`；
- `assetId` 被真实传给商家/菜品接口；
- 后端创建 media reference；
- 审核/上架后 reference 变为 `active + visible`；
- MediaGateway 可见性条件成立。

但以当前代码，严格意义上的“远端 final 图片通过业务 UI 重新拉取并渲染”尚无完整展示入口。若 owner 要求最终证据必须是“刷新页面后远端图片在业务 UI 中加载出来”，需要后续单独授权一个展示端点/买家外卖页面/商家远端图展示位。

## 1. 环境准备

### 1.1 必须准备的后端环境变量

完整后端由 `backend/cmd/server` 启动，配置读取来自 `backend/internal/config/config.go`。本地启动推荐把后端配置写入：

```text
backend/.env
```

不要提交 `.env`，不要把真实 Secret 写入报告。

后端会读取以下配置：

```dotenv
APP_ENV=dev
HTTP_ADDR=:8080

MYSQL_DSN=campus:campus@tcp(127.0.0.1:3306)/campus_circle?charset=utf8mb4&parseTime=True&loc=Local

JWT_SECRET=replace-with-at-least-32-characters

WECHAT_APP_ID=your-wechat-miniapp-appid
WECHAT_APP_SECRET=your-wechat-miniapp-secret
WECHAT_API_BASE=https://api.weixin.qq.com
ALLOW_MOCK_WECHAT_LOGIN=true

TENCENT_IM_SDK_APP_ID=your-im-sdkappid
TENCENT_IM_SECRET_KEY=your-im-secret
TENCENT_IM_ADMIN_USER=administrator
TENCENT_IM_REST_ENDPOINT=https://console.tim.qq.com

MEILI_HOST=http://127.0.0.1:7700
MEILI_MASTER_KEY=your-meili-master-key
MEILI_INDEX=products

COS_SECRET_ID=your-cos-secret-id
COS_SECRET_KEY=your-cos-secret-key
COS_BUCKET=your-bucket-name-with-appid-suffix
COS_REGION=ap-xxxxx
COS_APP_ID=your-tencent-cloud-appid

MEDIA_PUBLIC_BASE_URL=http://127.0.0.1:8080
WECHAT_CALLBACK_TOKEN=local-callback-token
```

关键要求：

- `HTTP_ADDR` 默认监听 `:8080`，本地验收统一使用 `http://127.0.0.1:8080`。
- `JWT_SECRET` 不能使用默认值，长度必须不少于 32 字符，否则启动失败。
- `COS_BUCKET` 必须是完整 bucket 名，通常包含腾讯云 AppID 后缀，例如 `xxx-1234567890`。
- `COS_REGION` 必须与 bucket 实际地域一致。
- `COS_SECRET_ID/COS_SECRET_KEY` 必须属于该 bucket 所在腾讯云账号。
- COS bucket 必须保持 P0-4 验收时的真实配置，尤其 Versioning 不应被关闭。
- 完整 `cmd/server` 当前会初始化微信内容安全客户端；即使本地使用 mock 登录，`WECHAT_APP_ID/WECHAT_APP_SECRET` 缺失也可能导致媒体审核链路无法正常运行。
- 真实 COS/STS 凭证失效时，图片上传会失败；不要用编译通过代替图片链路验收。

### 1.2 MySQL、Meilisearch、Redis

当前代码运行依赖：

- MySQL：必须；
- Meilisearch：后端启动/商品搜索链路会初始化；
- Redis：当前配置与代码中未作为本地启动必需项。

仓库中有 `deploy/docker-compose.yml`。其中 MySQL 映射为：

```text
host 127.0.0.1:3307 -> container 3306
database: campus_circle
user/password: campus/campus
```

可启动 MySQL：

```powershell
npm run dev:infra
```

如果使用 compose 的 MySQL，请把 `MYSQL_DSN` 改为：

```dotenv
MYSQL_DSN=campus:campus@tcp(127.0.0.1:3307)/campus_circle?charset=utf8mb4&parseTime=True&loc=Local
```

`deploy/docker-compose.yml` 中的 Meilisearch 服务没有对宿主机暴露 `7700` 端口。如果后端在宿主机通过 `npm run dev:backend` 启动，并配置 `MEILI_HOST=http://127.0.0.1:7700`，需要另行启动一个对宿主机暴露端口的 Meilisearch，例如：

```powershell
docker run --rm -p 7700:7700 -e MEILI_MASTER_KEY=your-meili-master-key getmeili/meilisearch:v1.46.0
```

### 1.3 后端启动

从仓库根目录启动：

```powershell
npm run dev:backend
```

启动成功后检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

预期：

```json
{"ok":true}
```

如果启动失败，优先检查：

- `backend/.env` 是否存在；
- `JWT_SECRET` 是否有效；
- MySQL DSN 是否连得上；
- Meilisearch 是否可达；
- COS 配置是否完整；
- 微信 AppID/Secret 是否完整；
- 腾讯云 IM 配置是否完整。

## 2. 小程序连接本地后端

### 2.1 使用正确的小程序代码位置

如果主仓库尚未合并 G-7b 商家小程序页面，请使用：

```text
C:\Users\33141\Documents\g7b-ui-merchant
```

否则，在主仓库根目录使用已合并后的 `apps/miniapp`。

### 2.2 小程序开发态启动

在小程序代码所在仓库根目录执行：

```powershell
$env:TARO_APP_API_BASE="http://127.0.0.1:8080/api/v1"
npm run dev:miniapp
```

当前 miniapp dev 配置默认也会指向：

```text
http://127.0.0.1:8080/api/v1
```

显式设置 `TARO_APP_API_BASE` 可以避免环境混淆。

### 2.3 微信开发者工具导入

在微信开发者工具中导入：

```text
apps/miniapp
```

项目配置：

- `miniprogramRoot`: `dist/`
- AppID：使用当前项目配置中的小程序 AppID，或 owner 的真实测试 AppID；
- 本地调试时关闭合法域名校验。

微信开发者工具本地调试建议：

- 关闭“校验合法域名、web-view 域名、TLS 版本以及 HTTPS 证书”；
- 确认小程序请求能访问 `http://127.0.0.1:8080/api/v1`；
- 如果使用真机预览而不是模拟器，`127.0.0.1` 指向手机自身，不是电脑。真机需要改成电脑局域网 IP，并确保防火墙放行。

### 2.4 Token 获取与注入

小程序 API 读取的 token key 是：

```text
domainA.token
```

后端登录接口：

```http
POST /api/v1/auth/wechat-login
```

请求体：

```json
{
  "code": "g7b-owner",
  "nickname": "G7B Owner",
  "avatar": "",
  "school": "Local",
  "campus": "Local"
}
```

PowerShell 示例：

```powershell
$body = @{
  code = "g7b-owner"
  nickname = "G7B Owner"
  avatar = ""
  school = "Local"
  campus = "Local"
} | ConvertTo-Json

$resp = Invoke-RestMethod `
  -Method Post `
  -Uri http://127.0.0.1:8080/api/v1/auth/wechat-login `
  -ContentType "application/json" `
  -Body $body

$resp.data.token
```

把返回的 token 写入微信开发者工具 Storage：

```javascript
wx.setStorageSync('domainA.token', '<token>')
```

注意：

- token 有有效期，若出现 401，重新登录并重新写入；
- 如果后端未允许 mock 登录，需要使用真实 `wx.login()` 返回的 code；
- admin 后台也使用 `domainA.token`，但需要 admin/superadmin 权限。

## 3. Admin 后台准备

启动 admin：

```powershell
$env:VITE_PROXY_API_TARGET="http://127.0.0.1:8080"
npm run dev:admin
```

打开：

```text
http://127.0.0.1:5173
```

admin 前端 token 存在浏览器 localStorage：

```javascript
localStorage.setItem('domainA.token', '<admin-token>')
```

如果本地没有 admin 用户，可在本地测试库中把一个测试用户提升为 admin/superadmin，然后重新登录获取新 token。示例仅供本地验收使用：

```sql
UPDATE users
SET role = 'superadmin',
    status = 'active',
    token_version = token_version + 1
WHERE open_id = 'mock_openid_g7b-admin';
```

然后重新调用 `/api/v1/auth/wechat-login` 获取 admin token。

## 4. 逐步验收清单

每一步都包含“操作 / 预期 / 在哪看”。

### a. Profile → 商家中心，确认未入驻态

操作：

1. 打开微信开发者工具；
2. 确认 Storage 中存在 `domainA.token`；
3. 进入“我的/Profile”页；
4. 点击“商家中心”。

预期：

- 若当前用户没有商家记录，显示“成为商家”或入驻表单；
- 若显示找不到页面，说明当前小程序代码不是 G-7b 商家 UI 分支或尚未合并；
- 若返回 401，说明 token 缺失或过期；
- 若返回 403，说明当前接口鉴权或角色状态不满足。

在哪看：

- 微信开发者工具界面；
- Network 面板中 `GET /api/v1/takeout/merchant/me` 返回。

### b. 提交商家入驻，包含 logo 上传

操作：

1. 在商家入驻表单填写店名、联系方式、位置、营业时间；
2. 选择 logo 图片；
3. 等待上传完成；
4. 提交入驻。

预期：

- 选择 logo 后，表单里出现本地预览；
- 上传链路完成后前端拿到 media `assetId`；
- 提交入驻时，请求体携带 `logoAssetPublicId`；
- 后端创建商家，状态为 `pending_review`；
- 后端创建 F 审核任务；
- 页面显示“审核中”。

在哪看：

- 微信开发者工具 Network：
  - 创建上传票据；
  - STS 授权；
  - COS PUT；
  - Complete Upload；
  - merchant onboarding。
- MySQL：

```sql
SELECT id, merchant_public_id, owner_user_id, name, logo_asset_public_id, status, row_version
FROM takeout_merchants
WHERE name = '<店名>'
ORDER BY id DESC
LIMIT 1;
```

```sql
SELECT asset_public_id, purpose, storage_stage, technical_status, review_status
FROM media_assets
WHERE asset_public_id = '<logoAssetPublicId>';
```

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status, a.review_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
WHERE r.reference_type = 'merchant_logo'
  AND r.reference_id = '<merchant_public_id>';
```

```sql
SELECT id, object_type, object_id, status, reason
FROM review_tasks
WHERE object_type = 'takeout_merchant'
ORDER BY id DESC
LIMIT 5;
```

阶段性预期：

- 商家状态：`pending_review`；
- logo asset：应进入 `final + final_ready + approved`，或仍在审核链路中；
- reference：审核通过前可能仍是 `pending/hidden`；
- review task：存在 `takeout_merchant` 任务。

### c. Admin 审核通过商家，确认商家 active 与 logo 可见性

操作：

1. 打开 admin：`http://127.0.0.1:5173`；
2. 注入 admin token；
3. 进入统一审核/Moderation；
4. 找到 `takeout_merchant` 审核任务；
5. Start Review；
6. Approve。

预期：

- 审核任务变为 approved；
- `takeout_merchants.status` 从 `pending_review` 变为 `active`；
- logo 对应 media reference 被发布：
  - `status = 'active'`
  - `parent_visibility = 'visible'`
- MediaGateway 可见性条件成立。

在哪看：

```sql
SELECT merchant_public_id, status, logo_asset_public_id, row_version
FROM takeout_merchants
WHERE merchant_public_id = '<merchant_public_id>';
```

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status, a.review_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
WHERE r.reference_type = 'merchant_logo'
  AND r.reference_id = '<merchant_public_id>';
```

严格通过条件：

```text
takeout_merchants.status = active
media_assets.storage_stage = final
media_assets.technical_status = final_ready
media_assets.review_status = approved
media_asset_references.status = active
media_asset_references.parent_visibility = visible
```

关于“logo 在哪确认已可见”：

- 当前 admin Media Console 可查看 asset/reference 元数据；
- 当前代码没有面向小程序的通用 MediaGateway 图片 URL HTTP 接口；
- 当前 G-7b 商家 UI 不保证在刷新后重新渲染远端 logo 图片；
- 所以本阶段能确认的是“logo reference 已达到可见状态”，而不是“业务 UI 已重新加载远端 logo 图片”。

若 owner 要求必须截图证明远端 logo 在 UI 渲染，需要后续补充展示入口或买家端展示页。

### d. 商家中心进入 active 管理态

操作：

1. 回到微信开发者工具；
2. 重新进入“商家中心”；
3. 如页面缓存旧状态，手动下拉刷新或重新打开页面。

预期：

- 不再显示“审核中”；
- 显示商家管理态；
- 能看到分类、菜品、订单相关入口。

在哪看：

- 微信开发者工具 UI；
- Network 面板中 `GET /api/v1/takeout/merchant/me` 返回商家状态 `active`。

### e. 创建分类、创建带图菜品

操作：

1. 在商家中心创建一个分类；
2. 新建菜品；
3. 选择菜品图片；
4. 等待上传完成；
5. 填写名称、价格、描述、分类；
6. 保存菜品。

预期：

- 图片选择后出现本地预览；
- 上传完成后前端拿到 media `assetId`；
- 创建菜品请求携带 `imageAssetPublicId`；
- 后端 `takeout_dishes.image_asset_public_id` 存储该 id；
- 后端创建 `reference_type='dish'` 的 media reference。

在哪看：

```sql
SELECT dish_public_id, merchant_public_id, category_public_id,
       name, price_cents, image_asset_public_id, status, row_version
FROM takeout_dishes
WHERE name = '<菜品名>'
ORDER BY id DESC
LIMIT 1;
```

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status, a.review_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
WHERE r.reference_type = 'dish'
  AND r.reference_id = '<dish_public_id>';
```

阶段性预期：

- 菜品初始状态通常为 `draft`；
- 图片 reference 可能仍为 `pending/hidden`；
- `image_asset_public_id` 不应为空；
- 这一步证明“不再是空数组/空值绑定”。

### f. 菜品上架，确认菜品图 reference 可见

操作：

1. 在菜品列表点击“上架”；
2. 如前端要求确认，确认操作；
3. 等待请求完成；
4. 刷新菜品列表。

预期：

- 菜品状态变为 `on_shelf`；
- 后端调用 media publish reference 能力；
- 菜品图 reference 变为：
  - `status = 'active'`
  - `parent_visibility = 'visible'`
- MediaGateway 可见性条件成立。

在哪看：

```sql
SELECT dish_public_id, name, status, image_asset_public_id, row_version
FROM takeout_dishes
WHERE dish_public_id = '<dish_public_id>';
```

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status, a.review_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
WHERE r.reference_type = 'dish'
  AND r.reference_id = '<dish_public_id>';
```

严格通过条件：

```text
takeout_dishes.status = on_shelf
media_assets.storage_stage = final
media_assets.technical_status = final_ready
media_assets.review_status = approved
media_asset_references.status = active
media_asset_references.parent_visibility = visible
```

关于“在哪个界面确认那张菜品图真的加载显示”：

- 当前商家小程序页面在上传后会显示本地预览，这是本地文件预览，不等于远端 final 图片重新加载；
- 当前代码没有买家外卖列表/详情页，也没有通用 media display URL HTTP 接口；
- 因此当前代码无法仅靠 UI 完成“远端 final 图片在业务界面重新加载显示”的严格证据；
- 当前可交付证据是：
  - 上传时本地预览截图；
  - `image_asset_public_id` 入库；
  - reference 创建；
  - 上架后 reference `active + visible`；
  - MediaGateway 条件闭合。

如果 owner 的最终验收标准必须包含 UI 远端图渲染截图，则应在本步骤停止，申请后续授权：增加一个基于 MediaGateway 的小程序展示视图或买家端外卖列表。

### g. 构造 paid 订单，商家接单 → 备餐 → 出餐

当前 G-7b 商家端负责商家处理订单，但买家下单小程序入口不在本阶段范围内。如果没有现成 paid 订单，需要通过测试夹具、后端领域测试方式或本地数据库种子构造一个 `paid` 订单。

操作：

1. 准备一个属于该商家的 `paid` 订单；
2. 打开商家中心订单看板；
3. 对 `paid` 订单点击“接单”；
4. 对 `accepted` 订单点击“开始备餐”；
5. 对 `preparing` 订单点击“出餐/待取餐”。

预期：

- `paid -> accepted -> preparing -> ready_for_pickup`；
- 每一步请求使用当前商家 token；
- 非该商家 owner 不应能操作。

在哪看：

```sql
SELECT order_public_id, merchant_public_id, status,
       accepted_at, preparing_at, ready_at, row_version
FROM takeout_orders
WHERE order_public_id = '<order_public_id>';
```

## 5. 失败原因与排查

### 5.1 图片上传失败

典型原因：

- `COS_SECRET_ID/COS_SECRET_KEY` 无效；
- `COS_BUCKET` 不是完整 bucket 名；
- `COS_REGION` 与 bucket 实际地域不一致；
- `COS_APP_ID` 与 bucket 所属账号不一致；
- COS bucket Versioning 被关闭；
- STS 临时凭证过期；
- 微信开发者工具无法访问 COS 域名；
- 本地网络或代理拦截 PUT；
- 微信内容安全 AppID/Secret 无效，导致审核无法完成。

排查：

- 看后端日志中 STS / COS / Completion 错误；
- 看微信开发者工具 Network 中 COS PUT 状态码；
- 确认 `media_assets.technical_status` 是否停在上传前状态；
- 确认 `media_upload_tickets.status` 是否过期或取消；
- 重新获取 token 和上传票据，不复用旧 STS。

### 5.2 图片上传成功，但界面不显示

先区分两类“不显示”：

1. 本地预览不显示：通常是小程序文件读取、压缩或临时路径问题；
2. 远端 final 图不显示：当前代码没有完整业务 UI 远端展示入口。

排查：

```sql
SELECT asset_public_id, purpose, storage_stage, technical_status, review_status
FROM media_assets
WHERE asset_public_id = '<asset_public_id>';
```

```sql
SELECT reference_type, reference_id, slot, status, parent_visibility
FROM media_asset_references
WHERE reference_id IN ('<merchant_public_id>', '<dish_public_id>');
```

必须满足：

```text
asset: final + final_ready + approved
reference: active + visible
```

若 reference 仍是 `hidden`：

- 商家是否已经审核通过；
- 菜品是否已经上架；
- 后端是否运行在包含 media publish wiring 的基线；
- reference 坐标是否正确：
  - 商家 logo：`reference_type='merchant_logo'`，`reference_id=merchant_public_id`
  - 菜品图：`reference_type='dish'`，`reference_id=dish_public_id`

### 5.3 接口 401/403

典型原因：

- `domainA.token` 未写入；
- token 过期；
- token 属于另一个用户；
- 用户被封禁；
- 商家状态不是 `active`；
- admin 页面使用的 token 不是 admin/superadmin。

排查：

- 小程序 Storage：确认 `domainA.token`；
- 重新调用 `/api/v1/auth/wechat-login`；
- 后端 `/api/v1/auth/me` 检查当前用户；
- MySQL 检查 `users.role/status/token_version`；
- 检查 `takeout_merchants.owner_user_id` 是否等于当前用户 id。

### 5.4 跨域、域名校验、真机访问失败

微信开发者工具：

- 关闭合法域名校验；
- 模拟器可访问 `127.0.0.1`；
- 真机预览不能用电脑的 `127.0.0.1`，需要使用电脑局域网 IP；
- Windows 防火墙需允许 8080 端口。

Admin H5：

- Vite 代理默认走 `VITE_PROXY_API_TARGET=http://127.0.0.1:8080`；
- 若请求没有进后端，检查 Vite 终端日志与浏览器 Network。

### 5.5 Admin 看不到审核任务

排查：

```sql
SELECT id, object_type, object_id, status, reason, created_at, updated_at
FROM review_tasks
ORDER BY id DESC
LIMIT 20;
```

商家入驻任务应为：

```text
object_type = takeout_merchant
```

如果没有任务：

- onboarding 请求可能失败；
- 商家可能已经存在；
- 当前用户 token 不是你正在验收的用户；
- 后端不是 G-7b 商家 API 基线。

## 6. 最小验证路径

如果 owner 时间有限，最少做以下步骤即可确认“上传 → 绑定 → 发布可见”核心链路：

1. 启动真实后端，确保真实 MySQL、真实 COS/STS、真实微信内容安全可用；
2. 使用 G-7b 商家 UI worktree 打开小程序；
3. 注入一个已是 `active` 商家的用户 token；
4. 创建菜品时选择图片；
5. 确认上传完成后创建菜品请求携带 `imageAssetPublicId`；
6. 上架菜品；
7. 查询数据库确认：

```sql
SELECT dish_public_id, status, image_asset_public_id
FROM takeout_dishes
WHERE dish_public_id = '<dish_public_id>';
```

```sql
SELECT r.reference_type, r.reference_id, r.slot, r.status, r.parent_visibility,
       a.asset_public_id, a.purpose, a.storage_stage, a.technical_status, a.review_status
FROM media_asset_references r
JOIN media_assets a ON a.id = r.asset_id
WHERE r.reference_type = 'dish'
  AND r.reference_id = '<dish_public_id>';
```

最小通过标准：

```text
takeout_dishes.status = on_shelf
takeout_dishes.image_asset_public_id 非空
media_assets.storage_stage = final
media_assets.technical_status = final_ready
media_assets.review_status = approved
media_asset_references.status = active
media_asset_references.parent_visibility = visible
```

若 owner 只接受“UI 上实际显示远端图片”的证据，则当前代码不能完全满足，应暂停验收并申请后续展示入口授权。

## 7. 验收记录模板

建议 owner 手动验收时记录：

```text
验收日期：
代码基线：
后端仓库/commit：
小程序仓库/commit：
微信 AppID：
后端地址：http://127.0.0.1:8080
COS Bucket：
COS Region：

商家用户：
merchant_public_id：
logo_asset_public_id：

菜品：
dish_public_id：
dish image_asset_public_id：

核心 SQL 结果：
- merchant status：
- dish status：
- media asset technical/review：
- media reference status/visibility：

截图：
- 小程序商家中心未入驻/审核中/active：
- logo 上传本地预览：
- 菜品图上传本地预览：
- 菜品上架后状态：
- admin 审核通过：

结论：
PASS / CONDITIONAL PASS / FAIL

若 CONDITIONAL PASS：
缺口：
下一步授权建议：
```

