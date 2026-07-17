# 外卖商家小程序动态目录设计

> 状态：方案已确认，待实施计划
> 日期：2026-07-17

## 1. 背景

当前学生端正式外卖入口是 `pages/student-takeout/index`。页面硬编码六个商家，点击后只显示“即将跳转商家小程序”，没有读取后台数据，也没有调用微信跨小程序跳转。

仓库内已经存在导航域 D：

- `navigation_categories`、`navigation_items` 保存分类和跳转配置；
- `GET /api/v1/navigation/categories` 提供公开导航树；
- `/api/v1/admin/navigation/*` 已有管理员 CRUD 路由；
- `Taro.navigateToMiniProgram({ appId, path })` 已在历史工具页中验证过调用方式。

本功能把学生外卖页改造成后台驱动的外部商家小程序目录。它不恢复旧外卖交易域 G，也不承接第三方订单回传。

## 2. 已确认产品决策

1. 商家入口只负责跳转外部小程序，不在平台内下单。
2. 所有 `admin` 与 `superadmin` 均可新增、编辑、切换状态、隐藏、恢复和删除商家。
3. 商家暂停营业后仍显示为“休息中”，但不可跳转。
4. 商家头像使用文字简称和配置颜色，不上传 Logo。
5. 新建商家默认“营业中”且立即展示。
6. 采用扩展导航域的方式，不新建独立外卖商家模块。
7. 允许一个小型数据库迁移，以分离“营业状态”和“是否展示”。

## 3. 目标与成功标准

### 3.1 目标

- 管理员可在后台维护真实商家名称、简介、文字简称、颜色、AppID、页面路径、排序和状态。
- 学生端从公开 API 动态读取商家，不再包含硬编码品牌。
- 营业商家可真实调用微信跨小程序跳转。
- 休息商家保留展示但不可跳转。
- 隐藏商家立即从学生端消失，且后台仍可恢复。

### 3.2 成功标准

- 后台新增一个合法商家后，无需重新发布小程序即可出现在学生端。
- 后台修改商家资料后，学生端下次加载显示新值。
- `open` 商家点击后仅使用服务端返回的 `appId/path` 调用目标小程序。
- `resting` 商家不会调用 `navigateToMiniProgram`。
- `enabled=false` 的商家不会出现在公开响应。
- 非管理员不能调用任何写接口。
- 旧外卖交易、购物车、订单、支付和骑手路由继续保持关闭。

## 4. 架构归属与边界

### 4.1 归属导航域 D

外部商家小程序是跳转配置，不是平台商户账户，因此归属 `internal/module/navigation`。导航域只保存展示和跳转所需数据，不保存：

- 商家所有者或入驻申请；
- 菜单、价格、库存；
- 购物车和订单；
- 支付、退款、结算；
- 骑手、配送和第三方订单状态。

`internal/module/navigation` 不得依赖任何其他业务域。边界检查必须禁止 D 导入 A、B、C、E、F、G、Delivery、媒体业务编排或支付实现；D 只依赖自身接口和无业务语义的技术底座。

### 4.2 旧外卖域保持关闭

以下开关继续为关闭状态，且本功能不读取它们：

- `ENABLE_LEGACY_TAKEOUT`
- `TARO_APP_ENABLE_LEGACY_TAKEOUT`
- `VITE_ENABLE_LEGACY_TAKEOUT`

外部目录独立使用导航分类和条目的 `enabled` 开关。启用外部目录不得恢复旧外卖路由、支付回调或后台 `TakeoutConsole`。

## 5. 数据模型与迁移

### 5.1 导航分类内部标识

`navigation_categories` 增加可空且唯一的 `system_key`：

```text
system_key = external_takeout_merchants
```

普通历史分类保持 `NULL`。系统分类的 `system_key` 不通过管理员请求写入，也不可被重命名或删除。后台和学生端按 `system_key` 定位外卖商家分类，不依赖中文名称或数据库 ID。

迁移幂等插入系统分类：

```text
name = 外卖商家
system_key = external_takeout_merchants
enabled = true
```

### 5.2 条目营业状态

`navigation_items` 增加：

```text
operating_status = open | resting
default = open
```

状态含义固定：

| `enabled` | `operating_status` | 学生端行为 |
| --- | --- | --- |
| `true` | `open` | 显示“营业中”，允许跳转 |
| `true` | `resting` | 显示“休息中”，禁止跳转 |
| `false` | 任意 | 不在公开接口返回 |

分类 `enabled=false` 是全部外卖商家的紧急总开关。

### 5.3 外卖商家字段映射

复用现有导航条目字段：

| 字段 | 用途 |
| --- | --- |
| `title` | 商家名称 |
| `subtitle` | 品类、门店或推荐说明 |
| `iconText` | 文字简称 |
| `color` | 头像背景色 |
| `appId` | 目标微信小程序 AppID |
| `path` | 目标小程序内部页面路径，可带非敏感静态查询参数 |
| `sort` | 升序排序 |
| `enabled` | 是否展示 |
| `operatingStatus` | 营业或休息 |

同一分类内的 `app_id + path` 不允许重复。迁移增加数据库唯一约束；服务层在写入前返回可读错误。

### 5.4 生产迁移策略

- 新增下一个可用编号的独立顺序迁移，不修改历史迁移。
- 部署前检查列、索引、系统分类、同名分类和重复目标。
- 若全部新对象均不存在，才执行迁移；若全部对象已存在且签名正确，视为已应用并跳过；若只存在一部分或签名不一致，立即中止。
- 已有且唯一的“外卖商家”分类可以被迁移认领并写入系统标识；同名分类超过一个、系统标识已被其他分类占用或同一分类存在重复 `app_id + path` 时，迁移必须 fail-closed，不得静默合并或新建第二个分类。
- 备份 `navigation_categories` 与 `navigation_items`。
- 生产只执行本次新增迁移，不重放旧迁移集合。
- 生产模式采用 forward-only schema：新二进制异常时只回滚二进制，新增列继续保留，旧版本可忽略。
- 不提供会删除商家或删除新增列的自动生产 rollback；表备份只在明确人工授权的灾难恢复中使用。

## 6. 后端设计

### 6.1 公开读取

保留：

```text
GET /api/v1/navigation/categories
```

公开树仅返回 `enabled=true` 的分类和条目，按 `sort ASC, id ASC` 排序。分类响应增加只读 `systemKey`，条目响应增加 `operatingStatus`。

学生端只消费 `systemKey=external_takeout_merchants` 的条目。接口不返回隐藏条目，不返回数据库内部时间或管理员信息。

### 6.2 外卖商家专用管理路由

导航域内增加专用管理路由：

```text
GET    /api/v1/admin/navigation/external-takeout-merchants
POST   /api/v1/admin/navigation/external-takeout-merchants
PUT    /api/v1/admin/navigation/external-takeout-merchants/:id
DELETE /api/v1/admin/navigation/external-takeout-merchants/:id
PATCH  /api/v1/admin/navigation/external-takeout-merchants/:id/operating-status
PATCH  /api/v1/admin/navigation/external-takeout-merchants/:id/visibility
PATCH  /api/v1/admin/navigation/external-takeout-merchants/category-visibility
```

专用路由仍由现有 `AuthRequired + AdminRequired` 保护，并调用导航域 service/repository，不创建新业务模块。创建和完整编辑请求使用专门 DTO，只允许写入：

```text
title, subtitle, iconText, appId, path, color, sort
```

请求不接收 `categoryId`、`enabled` 或 `operatingStatus`。状态只通过两个专用 PATCH 路由写入；服务端按 `system_key=external_takeout_merchants` 定位分类。禁止请求写入 `id`、`systemKey`、创建时间、更新时间或分类内嵌条目。

创建商家固定写入外卖系统分类，并默认：

```text
enabled = true
operatingStatus = open
```

外卖系统分类的 `systemKey`、分类 ID 和名称均由服务端维护。现有通用 category/item CRUD 遇到该系统分类或其条目时返回 `409`，要求使用专用路由；因此条目不能被移入或移出系统分类，也不能绕过专用校验。系统分类不能通过通用路由重命名或删除，分类总开关只通过专用 `category-visibility` 路由切换。

### 6.3 更新与零值语义

Repository 使用显式字段映射或 `Select` 更新，不能继续依赖会跳过零值的 `Updates(struct)`。以下更新必须真实落库：

- `enabled=false`；
- `sort=0`；
- `subtitle=''`；
- `color=''` 在校验允许时；
- `operatingStatus=resting`。

不存在的条目返回 `404`；重复目标返回 `409`；字段非法返回 `400`；数据库异常返回通用 `500`，不得透出 SQL。

### 6.4 输入校验

本节严格校验只应用于外卖商家专用路由，不追溯修改普通导航条目，也不要求清理开发种子中的历史占位 AppID。通用导航 CRUD 保持原有兼容行为，但不得操作外卖系统分类及其条目。

- 所有文本先 `TrimSpace`。
- AppID 必须匹配 `wx` 加 16 位英文字母或数字，总长度 18。
- 路径归一化为无前导斜杠的小程序内部路径。
- 路径拒绝 `http://`、`https://`、`//`、反斜杠、`..`、`#`、控制字符和空路径。
- 查询参数名拒绝 `token`、`secret`、`openid`、`session`、`authorization` 及其大小写变体。
- 不保存或发送 `extraData`。
- `title`、`subtitle`、`iconText` 遵守现有数据库长度。
- `color` 只允许 `#RRGGBB`。
- `sort` 限定在 `0..10000`。
- `operatingStatus` 只允许 `open/resting`。
- 外卖商家分类由服务端按系统标识解析，客户端不得提交或改变 `categoryId`。

## 7. 后台设计

现有“导航管理”增加独立的“外卖商家”面板，不复用受旧外卖开关控制的 `TakeoutConsole`。

### 7.1 列表

列表展示：

- 文字头像与颜色；
- 商家名称和简介；
- AppID 与页面路径；
- 营业状态；
- 展示状态；
- 排序；
- 编辑、营业/休息、隐藏/恢复和删除操作。

### 7.2 新增与编辑

表单字段为商家名称、简介、文字简称、颜色、AppID、页面路径和排序。新建不显示状态选择，保存后直接进入“营业中且展示”。

编辑立即生效。客户端先做与服务端一致的基础校验，但服务端结果为最终依据。保存成功后重新读取后台树，不在前端猜测数据库状态。

本期不增加 `rowVersion` 或多人编辑锁；并发操作采用最后一个成功写入为准。后台每次写入后重新读取服务端状态，避免继续展示本地推测值。

### 7.3 状态与删除

- “暂停营业”只写 `operatingStatus=resting`。
- “开始营业”只写 `operatingStatus=open`。
- “隐藏”只写 `enabled=false`。
- “恢复显示”只写 `enabled=true`，不改变营业状态。
- 删除必须二次确认，成功后立即刷新列表。

所有管理员均可执行以上操作。操作失败保留当前表单和列表，并显示服务端错误；不得显示假成功。

## 8. 学生端设计

### 8.1 数据来源

`pages/student-takeout/index` 删除硬编码 `merchants` 数组，通过 `services/d-navigation.ts` 读取公开导航树，并选择外卖系统分类。

页面不调用 G 域的商家、菜品、购物车、订单或支付接口。

### 8.2 页面状态

- `loading`：显示“正在读取合作商家”。
- `empty`：显示“暂无合作商家”。
- `error`：显示“商家加载失败”，提供点击重试。
- `ready`：按服务端排序展示商家卡片。

不得在 API 失败时回退到静态品牌样例。

### 8.3 点击行为

营业商家调用：

```ts
Taro.navigateToMiniProgram({ appId: merchant.appId, path: merchant.path })
```

参数只来自本次公开 API 响应，不接受页面 query 覆盖。调用成功前不显示“已跳转”。调用拒绝时提示“商家小程序暂时无法打开”。

休息商家不调用微信 API，只提示“商家休息中”。隐藏商家不会进入页面数据。

### 8.4 展示语义

学生端继续显示文字头像、商家名称、简介和状态。页面文案只说明“点击进入商家小程序”，不宣称平台已接收第三方订单，也不承诺自动生成配送单。

## 9. 错误处理与安全行为

- 公开接口失败进入可重试错误态。
- 后台 `401/403` 显示权限错误，不重试写操作。
- 非法 AppID/path 在写入阶段拒绝，不把风险数据发布给学生端。
- 微信目标不存在、未关联或当前账号无权限时，仅显示通用跳转失败提示。
- 日志可记录操作者 ID、条目 ID、操作类型和结果，但不得记录 token、OpenID、secret 或包含敏感查询参数的完整路径。
- 系统分类不可被通用分类 CRUD 重命名或删除。
- 分类总开关关闭后，公开接口立即不返回任何外卖商家。

## 10. 测试与验收

### 10.1 后端

- 迁移前置检查、升级和非破坏性兼容验证。
- 合法 AppID/path 创建成功，新商家默认 `open + enabled`。
- 非法格式、scheme、host、遍历、控制字符和敏感查询参数返回 `400`。
- 重复 `AppID + path` 返回 `409`。
- 普通用户写接口返回 `403`，未登录返回 `401`。
- `false/0/空字符串/resting` 更新真实落库。
- 公开树包含营业和休息条目，不包含隐藏条目。
- 禁用系统分类后公开目录为空，后台仍可读取。
- 系统分类不能重命名或删除。

### 10.2 后台

- TypeScript typecheck 与生产构建通过。
- 新增、编辑、营业/休息、隐藏/恢复和删除均刷新服务端结果。
- 表单校验、冲突、权限和服务端错误有明确反馈。

### 10.3 小程序

- TypeScript typecheck、静态路由检查与生产构建通过。
- `loading/empty/error/ready` 四态可验证。
- 营业条目调用一次 `navigateToMiniProgram`，参数与响应一致。
- 休息条目不调用微信 API。
- API 失败不展示硬编码商家。
- 生产包继续使用 `https://zhanduiwang.top/api/v1`。

### 10.4 架构与运行时

- `npm run check:boundaries` 通过，并明确禁止 D 依赖任何其他业务域。
- `ENABLE_LEGACY_TAKEOUT=false` 时外部目录仍可使用，旧 G 路由仍为关闭状态。
- 后端目标包测试、server 测试和 `go vet ./...` 通过。
- 不对共享非空数据库执行破坏性 rollback 测试。

### 10.5 真机验收

至少录入一个业务方提供的真实商家 AppID/path，并验证：

1. 后台新增后学生端出现。
2. 营业状态可打开目标正式小程序页面。
3. 休息状态保留卡片且不可跳转。
4. 隐藏后学生端消失。
5. 恢复显示后保留原营业状态。
6. 无效或无权限目标显示安全错误提示。

未提供真实商家 AppID/path 时，只能判定代码与模拟调用通过，不能宣称真机跳转验收完成。

## 11. 明确不做

- 不恢复旧外卖交易页面。
- 不新增平台购物车、订单或支付。
- 不接收第三方商家订单回调。
- 不向商家小程序传递用户身份或登录凭据。
- 不上传或审核商家 Logo。
- 不增加定时营业、自动探活、地理距离或个性化排序。
- 不把“休息中”解释为第三方商家的实时营业状态；它只由本平台管理员手动维护。

## 12. 发布边界

本功能完成代码、迁移、自动化测试和生产部署后，还需要真实商家 AppID/path 才能完成最后的微信真机跳转验收。上线顺序固定为：备份与 schema 预检 → 执行扩展迁移 → 部署并验证新后端 → 部署后台 → 构建并上传小程序。迁移是 forward-only，后端二进制使用独立回滚点；旧外卖开关始终保持关闭。
