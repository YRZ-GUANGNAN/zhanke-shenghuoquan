# A–F 收敛架构

## 业务代码归属

| 域 | 后端目录 | 小程序/后台入口 | 自有数据 |
| --- | --- | --- | --- |
| A 用户 | `internal/module/user` | `services/a-user.ts`、`stores/session.ts`、profile | `users` |
| B 社区 | `internal/module/community` | `services/b-community.ts`、home/post/publish(post) | `posts`、`post_images`、`comments`、`post_likes`、`post_favorites` |
| C 交易 | `internal/module/trade` | `services/c-trade.ts`、market/product/publish(product) | `products`、`product_images`、`product_favorites` |
| D 导航 | `internal/module/navigation` | `services/d-navigation.ts`、tools 页面（历史页面 URL） | `navigation_categories`、`navigation_items` |
| E IM | `internal/module/im` | `services/e-im.ts`；C 域联系卖家入口 | `im_user_mappings`、`im_conversations` |
| F 后台 | `internal/module/admin` | `apps/admin`、`services/f-admin.ts` | `review_tasks`、`reports` |

`config`、`database`、`response`、CORS 和程序启动代码是无业务语义的技术底座。它们只能负责装配、连接和统一协议，不能承载业务判断。

## 强制边界

- 仓储接口和 Gorm 实现位于所属域，只有所属域仓储可访问其表。
- handler 只做输入输出；状态机、权限归属和审核规则在 service。
- A 可向 B/C/E/F 提供身份与公开资料；B/C 不修改 A 数据。
- B、C 提交审核到 F；F 通过注册的领域服务回写审核状态，不直接访问帖子/商品表。
- C 通过 E 的 `ConversationGateway` 创建腾讯云 IM 会话关系；E 不读取商品表。
- D 不调用 A/B/C/E/F，不保存业务配置，只输出分类、`appid` 和 `path`。
- F 管理 A–D 时调用各域 service；F 自有仓储只访问 `review_tasks` 和 `reports`。

## 统一审核流

```text
submitted（提交） -> reviewing（审核） -> approved（通过，允许展示）
                                      -> rejected（拒绝，不展示）
```

帖子、评论、商品、举报全部创建 `review_tasks`。社区 feed 和商品列表只查询 `approved`；商品业务状态与审核状态分离。

商品状态机固定为：

```text
selling -> reserved -> sold
selling ------------> sold
reserved -> selling
```

`sold` 为终态。旧的订单模型已删除，当前系统不承担支付或自建订单能力。

## API 与返回规范

所有接口返回：

```json
{"code": 0, "message": "ok", "data": {}}
```

分页统一为 `items/page/pageSize/total`。路由根固定为 `/auth`、`/community`、`/trade`、`/navigation`、`/im`、`/admin`；举报提交为 `/reports`，归属 F。

## 删除和禁止项

- 已删除旧 `auth/market/tools/upload` 混合模块、全局 model 和全局 DB 依赖容器。
- 已删除订单表/接口、JSON 商品图片、工具链接单表、分散审核日志、mock 混合数据服务。
- 不创建聊天消息表，不接收或保存腾讯云 IM 消息内容。
- 不恢复未归类的关注、粉丝、支付、公告、通用上传或系统设置功能。
