# 帖子动态媒体部署实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 用户已授权完成后续工作；附属执行技能未安装，本会话直接执行既有方案，不另外创建代理。

**Goal:** 备份并迁移帖子动态媒体结构，部署与线上稳定版本兼容的视频后端，验证可用云审核能力并上传微信开发版本。

**Architecture:** 在已有隔离后端工作区的新分支中只引入本次视频相关改动。先检查真实 COS CI 配置与数据库，再备份、执行增量迁移并原子切换应用；云审核未验证前保持视频入口关闭。保留旧二进制和新增结构，失败只回退应用，不覆盖在线数据库。

**Tech Stack:** Go、MySQL 8、COS CI、OpenSSH、Python 3、Taro、微信官方开发者工具 CLI。

## Task 1: 核对目标与审核能力

- [x] 核验 `zhanduiwang.top` 后端为 `2026.10.17.3 / 6347033f061e`，当前目录为 `/opt/campus-circle/releases/20261017-private-messaging`，健康与就绪检查正常。
- [x] 核验当前隔离分支与原项目；已有隔离后端工作区没有已跟踪改动，保留其未跟踪发布产物。
- [ ] 使用 `.codex-tmp/post-motion-preflight.py` 在服务器读取配置存在性、数据库结构与云审核模板；不输出密钥、数据库密码或签名地址。
- [x] 运行微信官方 CLI `islogin`，确认当前登录有效。

## Task 2: 冻结并验证源码

- [ ] 提交本次动态媒体功能的精确文件清单；保留所有原有未跟踪产物。
- [ ] 在 `C:/Users/33141/.codex/worktrees/delivery-api-release/New project` 创建 `codex/post-motion-api-20261018`，应用本次提交中 `backend/` 的差异，不引入配送收件等在研代码。
- [ ] 对独立后端运行相关 Go 定向测试与 Linux amd64 构建；记录源码提交和二进制 SHA256。
- [ ] 复用已通过的 `test:post-motion:mysql` 迁移证据，并对发布源码有变化的部分补充验证，不在现有数据库运行测试。

## Task 3: 备份、迁移和后端发布

- [ ] 部署工具位于 `deploy/native/release-post-motion.py`，默认只读预检；显式 `--deploy` 才进行备份与迁移。
- [ ] 校验应用与迁移账号的数据库 UUID 相同；验证当前版本、磁盘空间、发布锁、迁移及二进制 SHA256。
- [ ] 使用 `/etc/campus-circle/mysql-admin.cnf` 创建 `/var/backups/campus-circle/20261018-post-motion-before.sql.gz`，校验完整 gzip CRC、关键表及导出结束标记。
- [ ] 仅执行 `backend/migrations/047_post_motion_media.sql`；验证三个新增列和审核任务表，历史帖子与图片不回填。
- [ ] 发布目录为 `/opt/campus-circle/releases/20261018-post-motion`。失败回退 `20261017-private-messaging` 的应用链接，保留新增数据库结构及备份。
- [ ] 验证健康、版本、真实接口、身份保护、分享设置与业务策略不变；云服务不可用时 `capabilities` 必须关闭视频。

## Task 4: 微信上传与验收记录

- [ ] 将生产构建冻结到 `.codex-tmp/release-post-motion-20261018/`，验证 AppID `wx065d8f3ce4a056ee`、真实 API、30 个原有页面、不含演示内容或密钥。
- [ ] 上传开发版本 `2026.10.18.1`，备注 `优化体验`，保留上传回执及文件哈希。不代替用户送审或正式发布。
- [ ] 可获得的开发者工具验证与真实 COS 审核分别记录；手机播放、原生视频层交互和真实照片片段配对只有实际完成才标为通过。
- [ ] 将完整结果和未完成条件写入 `docs/operations/2026-10-18-post-motion-release.md`。
