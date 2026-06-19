# Campus Circle

项目已按“半成品系统收敛重构”原则固定为六个业务域，不接受未归类功能：

- A 用户：微信登录、token、用户资料、权限中间件
- B 社区：帖子、图片、评论、点赞、收藏、feed
- C 交易：二手商品、图片、搜索、收藏、状态机、联系卖家入口
- D 导航：分类和导航项，只保存 `appid + path` 跳转数据
- E IM：只对接腾讯云 IM，只保存用户映射和会话关系，不保存消息
- F 后台：用户/帖子/商品/举报/导航管理、统计和统一审核流

完整边界、代码归属和依赖规则见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

```powershell
npm install
npm run dev:infra
npm run db:seed
npm run dev:backend
npm run dev:admin
npm run dev:miniapp
npm run verify
```

API 默认为 `http://127.0.0.1:8080/api/v1`。数据库迁移位于 `backend/migrations`；`003_domain_convergence.sql` 用于清理旧原型中的订单、工具链接和分散审核表。
