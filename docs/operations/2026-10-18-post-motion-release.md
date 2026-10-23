# 帖子视频与实况媒体发布记录

日期：2026-10-18（Asia/Shanghai）

## 范围

本次只扩展帖子媒体表达：支持一条视频，或“封面照片＋对应短片段”的实况媒体。发现流使用静音短预览，详情页由用户主动完整播放；不增加直播频道，不改动二手、私信、现有路由、身份守卫或既有接口语义。没有动态媒体时，原有图片帖子链路保持不变。

## 源码与隔离

- 小程序工作区：`codex/post-motion-preview-20261017`，提交 `c8f9e859278e6d14b54c33a0507d0776526df6f8`。
- 后端工作区：`codex/post-motion-api-20261018`，提交 `81ed84bb5d8141d1dcf039a2c5feabe859966999`。
- 后端分支从线上稳定版本独立接入动态媒体；未带入配送在研改动。两个工作区中已有的未跟踪历史发布产物均保留。

## 后端、数据库与审核

- 已执行增量迁移 `backend/migrations/047_post_motion_media.sql`，只增加可空字段和审核任务结构，不回填历史帖子。
- 迁移前数据库备份：`/var/backups/campus-circle/20261018-post-motion-before.sql.gz`。
- 备份 SHA-256：`9a2713e4d329f000df6a0f5a1e79fd7ceccf8cd34648644cb6d24590f98229bc`。
- 线上运行版本：`2026.10.18.1`，提交前缀 `81ed84bb5d81`。
- 已配置 `ENABLE_POST_VIDEO=true` 与 `VIDEO_REVIEW_BIZ_TYPE=5141a6a2aca67a45697e71840c0821dc`。
- 腾讯云数据万象已授权并绑定 COS 桶 `zkshq-1436381937-1436381937`（`ap-guangzhou`），审核同时覆盖画面和声音。
- 真实 COS 测试使用 H.264、320×240、4 秒样本，审核返回 `Success`、结果 `0`；未创建帖子，生产业务数据写入数为 0。

## 微信开发版本

- AppID：`wx065d8f3ce4a056ee`。
- 版本：`2026.10.18.1`，备注：`优化体验`。
- 冻结目录：`.codex-tmp/release-post-motion-20261018/`。
- 冻结清单：`.codex-tmp/release-post-motion-20261018-manifest.json`；构建包 135 个文件、2,031,421 字节，树 SHA-256 为 `d69d4b8a218f9fffadd37d7eab720e752ebb0480effe569a3dfba88d9be61002`。
- 构建连接真实 API `https://zhanduiwang.top/api/v1`，不含演示页面、模拟用户或生成图片。
- 已上传到微信开发版本；未代替用户提交审核，也未正式发布。

## 验证结果

- `npm run test:post-motion`：通过。覆盖审核等待、同一 ticket 重试、拒绝、取消、单条静音预览、三秒/半时长上限、离屏/切后台/断网/减少动态效果/播放失败回退和详情页主动播放。
- `npm run check:content-motion`：通过。
- `npm run check:post-presentation`：通过。
- `npm run check:media-review-policy`：通过。
- `npm run typecheck`：通过。
- 后端 `go test ./...`：通过。
- `git diff --check`：通过。
- 公网 `https://zhanduiwang.top/version` 返回 `2026.10.18.1`，`/healthz` 返回 200。公网 `/readyz` 当前返回 403，属于就绪端点的访问保护，不能用匿名请求替代主机侧就绪检查。

## 未完成与验收边界

开发者工具的原生播放自动化仍未完成：视口观察器首次触发失败，后续本地测试夹具出现 `MEDIA_ERR_SRC_NOT_SUPPORTED`。这只能说明该本地夹具没有在开发者工具中播放成功，不能推导真实 COS 视频播放失败。真实 COS 上传、媒体识别和视频审核已通过；手机真机播放、原生视频层交互、实况照片对应片段以及微信体验版验收仍需在真实设备中确认。

本记录不表示已送审或已上线。若真机验证发现播放问题，优先关闭 `ENABLE_POST_VIDEO` 并保留已迁移结构和备份，不删除历史图片帖子。
