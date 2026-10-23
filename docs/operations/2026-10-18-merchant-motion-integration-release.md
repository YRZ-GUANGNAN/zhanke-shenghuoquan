# 商家端与动态媒体整合开发版上传

2026-10-18 15:30:16（Asia/Shanghai），微信官方开发者工具 CLI 成功上传开发版本 `2026.10.18.2`，AppID 为 `wx065d8f3ce4a056ee`，备注为 `优化体验`，回执包体为 2,034,064 字节（CLI 显示 1.9 MB）。本次没有提交审核或发布上线，也没有代替用户设置体验版。

## 完成条件与源码

用户要求等待商家端当前修改完成后再整合。源任务“审查并总结待优化项”（`01a14d4a-5356-7931-ab05-d68af7d7f48d`）完成本轮工作后，本任务核对其终稿报告和干净工作区，再接收最终提交。

- 商家端终稿：`codex/delivery-workspace-redesign-20261018`，`f15f9fec65e85b389a7a842b762c5a6d6c55edf0`。
- 动态媒体基线：`b21b97c3f8c3ff75fe9cf33f48aca9ef0aba1dd5`，包含功能提交 `c8f9e859278e6d14b54c33a0507d0776526df6f8`。
- 最终整合提交：`35a9e61e266ef802b5a339420863618f6a8cee18`，分支 `codex/post-motion-preview-20261017`。
- 两部分采用 Git 合并保留历史，无冲突。原项目和其他工作区的未完成工作、已有未跟踪发布产物未纳入上传包。

## 本包包含

- 商家/骑手工作区排版、订单来源与单号层级、发单和订单详情布局。
- 下拉与手动刷新、骨架与等待反馈、弱网恢复、分页失败重试；保留已有订单、身份资格、版本校验与重复点击保护。
- 商家/骑手身份申请状态摘要弹窗、申请详情、审核意见、本次进度、私密证明、固定底部操作和安全区；输入触控高度至少 44px。
- 帖子视频和封面照片＋短片段的实况媒体：发现流单条静音短预览，详情主动完整播放，保留审核等待与失败恢复。

## 联合验证

在最终整合提交上运行以下 21 组命令，退出码均为 0：

`check:identity-application-ui`、`check:identity-application-contract`、`check:profile-presentation`、`check:delivery-stage-workbench`、`check:merchant-home-clarity`、`check:rider-workbench-clarity`、`check:role-workbench-ui`、`test:post-motion`、`check:media-review-policy`、`check:post-presentation`、`check:content-motion`、`check:miniapp-static-routes`、`check:miniapp-publish-contract`、`check:miniapp-business-closure`、`check:profile-center-ui`、`check:feature-policy`、`check:feature-gate-feedback`、`check:miniapp-capsule-layout`、`check:miniapp-regression-fixes`、`check:real-wechat-secret-hygiene`、`typecheck`。

`npm run build:miniapp` 与 `git diff --check` 通过。构建仍有既有公共 CSS 引入顺序和 `vendors.js` 约 732 KiB 的体积建议。构建日志：`.codex-tmp/merchant-motion-final-build.log`。

商家任务已对实际 TSX/SCSS 进行 106 组独立布局检查，覆盖 320/390/430px、长文案、放大文字和减少动效，记录中布局问题为 0；详情见 `docs/reviews/2026-10-18-identity-application-ui-refresh.md`。该证据来自浏览器中的隔离样本，不是微信真机验收。

仓库级 `verify` 不标为通过：`check:boundaries` 仍报告 `backend/internal/module/delivery/external_intake.go` 在 repository 外访问数据库。该文件及边界检查器相对本轮整合基线未变，此文件此前最近提交为 `343ad6f`。本轮只上传小程序，不发布此后端文件，也未放宽边界检查。

## 冻结包与上传证据

- 上传目录：`.codex-tmp/release-merchant-post-motion-20261018-2-final/`。
- 文件清单：`.codex-tmp/release-merchant-post-motion-20261018-2-final-manifest.json`。
- 上传回执：`.codex-tmp/upload-2026.10.18.2.json`；官方 CLI 返回 `upload` 成功，进程退出码 0。
- 冻结文件数：135，合计 2,043,622 字节；文件树 SHA-256：`0721435ff80c7965b5fbf37de449a05ecaf1e379d72923b1c4f214ac66ea9a90`。
- 上传前检查了 30 条原有路由、真实 API `https://zhanduiwang.top/api/v1`、开启域名校验、身份申请和动态媒体内容均存在，且没有演示用户、演示素材或本地 API。
- 上传后逐文件重新计算 SHA-256，并检查无额外文件；135 个文件与冻结清单完全一致。
- 此前初步冻结目录不作为交付包，未上传。本记录指向上述 final 目录。

## 后端与剩余验收

本轮相对动态媒体基线的 `backend/`、MiniApp 服务、状态仓库、路由和项目配置无差异。公网 `/version` 确认为 `2026.10.18.1 / 81ed84bb5d81`，`/healthz` 为 `ok`；未重新部署后端、执行数据库迁移或切换线上业务开关。

微信原生键盘、字体倍率、安全区、真实账号审核链路和手机视频播放仍需设备验收。开发者工具先前的本地视频夹具播放失败限制仍保留，不因本次构建或上传成功而改为通过。此次完成范围是最终源码合并和微信开发版本上传，未推送 GitHub。
