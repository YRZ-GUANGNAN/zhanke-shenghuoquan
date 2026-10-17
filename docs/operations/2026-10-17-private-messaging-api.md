# 主页私信配套后端发布

用户授权补充后端并上传微信、GitHub。本分支 `codex/private-messaging-api-20261017` 从线上已部署公开主页版本的记录提交 `aed2a25` 建立，仅移植 `d5f883f` 的微信绑定状态、`fa006ae` 的会话头像，以及 `5175092` 的用户私信接口、双向拉黑和功能权限检查。

新增 `PUT /api/v1/im/users/:userId/conversation`，使用现有 `im_conversations` 的无序用户对和 `reference_type=user`；商品会话保留原接口。消息列表同时支持两种关系并返回公开对方资料。未更改数据库结构、其他业务模块或当前开关。

安全后端测试通过；测试环境清空全部 MySQL 测试凭据。实际线上部署使用 `deploy/native/release-private-messaging.py`：校验线上版本和旧二进制、检查关系唯一索引与拉黑表、核对腾讯 IM 配置存在，先压缩备份数据库，再原子切换新版本，失败恢复旧二进制。发布期间不写测试数据、不发送真实消息。

版本 `2026.10.17.3`，源码 `6347033f061e`，2026-10-17 16:42:47（Asia/Shanghai）部署成功，替换原版本 `2026.10.17.1 / 092ddc352125`。新二进制 SHA256 `e264f8fa230d6c565fd7b3c261b9049bae20bffc057b6ad513e3c256f586e140`。

数据库备份 `/var/backups/campus-circle/20261017-private-messaging-before.sql.gz`，SHA256 `929a19e06856309d3c7dae6ad2ef5abfef34da49b804cb216f435fe05b93c851`，完整解压校验 312,773 字节。部署回执 `/var/backups/campus-circle/20261017-private-messaging-result.json`，verified=true，未触发回滚。

公网健康和版本、主机就绪正常；新用户会话、会话列表及 IM 凭据接口匿名访问为 401，既有受保护接口状态未变，功能策略、分享设置、运行环境摘要和旧二进制保持一致。腾讯 IM 双账号实际收发验收仍待真机完成。
