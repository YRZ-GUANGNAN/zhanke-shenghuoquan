# 微信审核代理验收修订与重新部署 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不修改业务代码和不降低代理安全要求的前提下，修正不可控的生产 HEAD 计数门禁，重新部署既有构建，并用全新图片完成真实微信审核和最终收口。

**Architecture:** 自动化层继续强制证明同一有效 token 的 HEAD/GET 固定版本能力；生产层强制验证真实 GET、callback、严格 approved 状态、数据库、日志、服务和 SHA。重新部署使用全新 deployId、明确分离的 stageDir/stageFile、全新精确备份和三态 CAS 回滚。

**Tech Stack:** Go 1.25、Gin、PowerShell、SSH、systemd、Nginx、MySQL、微信开发者工具 CLI、miniprogram-automator。

---

## 基线与边界

- 修订设计：`docs/superpowers/specs/2026-07-17-wechat-review-acceptance-method-design.md`
- 业务实现 commit：`25231194eacde3eaaf5fb3a567f45f03787e3910`
- 已验证 artifact：`.release/campus-api-review-proxy`
- artifact SHA256：`94fa228ae9be8a301db10180ce82268b1bcdfe906a15e5561fd48cd6adcd01a5`
- 当前生产旧 SHA256：`1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132`
- 当前 release：`/opt/campus-circle/releases/90b62c320598`
- 旧失败部署、验收收据、stage 和 exact backup 全部保留，不覆盖、不删除。
- 不修改 `backend/migrations/**`、`apps/miniapp/src/**`、Nginx、环境文件或任何凭据。
- 用户既有工作树改动只读保护，禁止 `git add .`、`git add -A`、reset 或清理。

### Task 1: 冻结修订设计并重跑代码能力门禁

**Files:**
- Verify: `backend/internal/module/media/miniappflow/wechat_review_proxy.go`
- Test: `backend/internal/module/media/miniappflow/wechat_review_proxy_test.go`
- Test: `backend/internal/middleware/cors_test.go`
- Test: `backend/cmd/server/router_logging_test.go`
- Verify: `.release/campus-api-review-proxy`

- [ ] **Step 1: 确认设计之后没有业务源码漂移**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
$codeDiff=git diff --name-only 25231194..HEAD -- backend
if ($codeDiff) { $codeDiff; throw '业务实现 commit 之后出现 backend 漂移' }
if (git diff --name-only -- backend/internal/module/media/miniappflow backend/internal/middleware backend/cmd/server) {
  throw '目标源码存在未提交改动'
}
git diff --check
```

Expected: 目标源码无漂移；只保留用户既有无关改动。

- [ ] **Step 2: 强制验证同 token HEAD/GET 能力**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run '^TestWechatReviewProxyUsesSameTokenForHeadAndGet$' -count=1
go test ./internal/module/media/miniappflow -run '^TestWechatReviewProxy(RejectsBeforeStorage|MapsNotFoundAndTemporaryStorageErrors|MapsFirstBodyReadFailureTo503|MapsTypedNilBodyTo503|LogsFixedCategoryAfterCommittedShortRead|StreamsCompleteBodyPastFirstBlock|RejectsUnsupportedMethods|OptionsReachesHandlerThroughProductionCORS|ResponsesDoNotLeakSensitiveValues)$' -count=1
go test ./internal/middleware ./cmd/server ./internal/module/media/miniappflow -shuffle=on -count=10
go vet ./internal/middleware ./cmd/server ./internal/module/media/miniappflow
```

Expected: 全部 PASS；同一 token 的 HEAD 与 GET 均为 200，HEAD 不打开正文。任何失败都停止部署。

- [ ] **Step 3: 运行仓库边界门禁并核对 artifact**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
npm run check:boundaries
npm run check:miniapp-static-routes
npm --prefix apps/miniapp run typecheck
$artifact=(Resolve-Path '.release\campus-api-review-proxy').Path
$sha=(Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant()
$bytes=(Get-Item -LiteralPath $artifact).Length
if ($sha -ne '94fa228ae9be8a301db10180ce82268b1bcdfe906a15e5561fd48cd6adcd01a5' -or $bytes -ne 16404642) {
  throw 'artifact 身份漂移'
}
Write-Output "CAPABILITY_GATE_OK sha=$sha bytes=$bytes"
```

Expected: 三项仓库门禁 PASS，并输出固定 SHA 和 `16404642` 字节。

### Task 2: 用全新部署身份上传并原子替换

**Files:**
- Read: `.release/campus-api-review-proxy`
- Create at runtime: `.release/review-proxy-redeploy.json`
- Remote create: 由 Step 1 的 `$deployId` 计算 `$stageDir/campus-api`
- Remote create: 由 Step 1 的 `$deployId` 计算 `/opt/cutover-backups/review-proxy-$deployId/`
- Replace: `/opt/campus-circle/releases/90b62c320598/bin/campus-api`

- [ ] **Step 1: 生成新 deployId，创建明确的 stageDir/stageFile 并上传**

在同一个 PowerShell 会话中执行：

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$target='admin@8.138.227.107'
$artifact=(Resolve-Path '.release\campus-api-review-proxy').Path
$localSha=(Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant()
$deployId="$(Get-Date -Format yyyyMMdd_HHmmss)-$([guid]::NewGuid().ToString('N').Substring(0,12))"
if ($deployId -notmatch '^[0-9]{8}_[0-9]{6}-[0-9a-f]{12}$') { throw '部署 ID 非法' }
$stageDir="/home/admin/.campus-review-proxy-$deployId"
$stageFile="$stageDir/campus-api"

$prepare="set -euo pipefail; test ! -e '$stageDir'; install -d -o admin -g admin -m 0700 '$stageDir'; echo STAGE_DIR_READY"
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($prepare))
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | sudo -n bash"
if ($LASTEXITCODE -ne 0) { throw '创建唯一 stageDir 失败' }
scp -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -- $artifact "${target}:$stageFile"
if ($LASTEXITCODE -ne 0) { throw '上传 artifact 失败' }
Write-Output "STAGE_UPLOAD_OK deploy_id=$deployId stage_dir=$stageDir stage_file=$stageFile sha=$localSha"
```

Expected: 新目录唯一，artifact 上传成功；输出值用于本次独立部署收据，不覆盖旧收据。

- [ ] **Step 2: 执行生产预检**

使用上一步同一会话的变量执行只读预检：

```powershell
$preflight=@'
set -euo pipefail
stage_dir='__STAGE_DIR__'
stage_file='__STAGE_FILE__'
expected_new='__NEW_SHA__'
expected_current='1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132'
release='/opt/campus-circle/releases/90b62c320598'
current="$release/bin/campus-api"
exec 9>/run/lock/campus-api-deploy.lock
flock -n 9
test "$(readlink -f /opt/campus-circle/current)" = "$release"
test -d "$stage_dir" && test ! -L "$stage_dir"
test -f "$stage_file" && test ! -L "$stage_file"
test "$(stat -c %U "$stage_file")" = admin
chmod 0600 "$stage_file"
test "$(stat -c %a "$stage_file")" = 600
test "$(sha256sum "$stage_file" | awk '{print $1}')" = "$expected_new"
test -f "$current" && test ! -L "$current"
test "$(sha256sum "$current" | awk '{print $1}')" = "$expected_current"
pid="$(systemctl show campus-api -p MainPID --value)"
test "$(sha256sum "/proc/$pid/exe" | awk '{print $1}')" = "$expected_current"
for unit in campus-api nginx mysql campus-meilisearch; do systemctl is-active --quiet "$unit"; done
curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null
echo "PREFLIGHT_OK release=$release stage_dir=$stage_dir stage_file=$stage_file"
'@
$preflight=$preflight.Replace('__STAGE_DIR__',$stageDir).Replace('__STAGE_FILE__',$stageFile).Replace('__NEW_SHA__',$localSha)
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($preflight))
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | sudo -n bash"
if ($LASTEXITCODE -ne 0) { throw '生产预检失败，停止替换' }
```

Expected: 当前磁盘和进程仍是旧 SHA，四项服务 active，新 stage 身份精确。

- [ ] **Step 3: 创建全新 exact backup，并用三态 CAS 原子替换**

远端脚本必须满足：

```bash
set -eEuo pipefail
deploy_id='__DEPLOY_ID__'
stage_file='__STAGE_FILE__'
expected_new='__NEW_SHA__'
expected_current="1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132"
release="/opt/campus-circle/releases/90b62c320598"
current="$release/bin/campus-api"
backup="/opt/cutover-backups/review-proxy-$deploy_id"
candidate="$current.new-$deploy_id"
backup_ready=0

exec 9>/run/lock/campus-api-deploy.lock
flock -n 9
test "$(sha256sum "$current" | awk '{print $1}')" = "$expected_current"
test "$(sha256sum "$stage_file" | awk '{print $1}')" = "$expected_new"
test ! -e "$backup" && test ! -e "$candidate"
install -d -o root -g root -m 0700 "$backup"
cp -a -- "$current" "$backup/campus-api"
sha256sum "$backup/campus-api" >"$backup/campus-api.sha256.before"
printf 'deploy_id=%s\nrelease=%s\nbefore_sha=%s\nartifact_sha=%s\n' "$deploy_id" "$release" "$expected_current" "$expected_new" >"$backup/manifest.txt"
chmod 0600 "$backup/manifest.txt"
: >"$backup/READY"
chmod 0600 "$backup/READY"
backup_ready=1

rollback() {
  rc=$?
  trap - ERR
  set +e
  rm -f -- "$candidate"
  live_sha="$(sha256sum "$current" 2>/dev/null | awk '{print $1}')"
  if [ "$live_sha" = "$expected_new" ] && [ "$backup_ready" -eq 1 ]; then
    install -o root -g campus -m 0750 "$backup/campus-api" "$current.rollback-$deploy_id"
    mv -fT -- "$current.rollback-$deploy_id" "$current"
    systemctl restart campus-api
    echo "ROLLBACK_OK deploy_id=$deploy_id"
  elif [ "$live_sha" = "$expected_current" ]; then
    echo "ROLLBACK_NOT_NEEDED deploy_id=$deploy_id"
  else
    echo "ROLLBACK_REFUSED deploy_id=$deploy_id reason=live_sha_cas" >&2
    exit "$rc"
  fi
  for _ in $(seq 1 45); do
    systemctl is-active --quiet campus-api && curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null && break
    sleep 1
  done
  exit "$rc"
}
trap rollback ERR

install -o root -g campus -m 0750 "$stage_file" "$candidate"
test "$(sha256sum "$candidate" | awk '{print $1}')" = "$expected_new"
mv -fT -- "$candidate" "$current"
systemctl restart campus-api
ready=0
for _ in $(seq 1 45); do
  if systemctl is-active --quiet campus-api && curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then ready=1; break; fi
  sleep 1
done
test "$ready" -eq 1
disk_sha="$(sha256sum "$current" | awk '{print $1}')"
pid="$(systemctl show campus-api -p MainPID --value)"
process_sha="$(sha256sum "/proc/$pid/exe" | awk '{print $1}')"
test "$disk_sha" = "$expected_new" && test "$process_sha" = "$expected_new"
for unit in campus-api nginx mysql campus-meilisearch; do systemctl is-active --quiet "$unit"; done
trap - ERR
echo "DEPLOY_OK deploy_id=$deploy_id backup=$backup sha=$disk_sha"
```

将上面完整 Bash 文本赋给 `$remote` 后执行：

```powershell
$remote=$remote.Replace('__DEPLOY_ID__',$deployId).Replace('__STAGE_FILE__',$stageFile).Replace('__NEW_SHA__',$localSha)
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($remote))
$deployOutput=ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | sudo -n bash"
$deployOutput
if ($LASTEXITCODE -ne 0) { throw '原子部署失败；检查精确 ROLLBACK 输出后停止' }
if (($deployOutput -join "`n") -notmatch [regex]::Escape("DEPLOY_OK deploy_id=$deployId")) { throw '缺少本次部署成功凭据' }
```

不通过命令行输出任何凭据。Expected: `DEPLOY_OK`，新 exact backup 与旧 backup 并存。

### Task 3: 在真实窗口之前完成公网安全门禁

**Files:**
- Create at runtime: `.release/review-proxy-redeploy.json`
- Create at runtime: `.release/review-proxy-acceptance-window-v2.json`

- [ ] **Step 1: 先执行故意的负向代理探针**

Run:

```powershell
$health=Invoke-RestMethod -Uri 'https://zhanduiwang.top/healthz' -TimeoutSec 20
$products=Invoke-RestMethod -Uri 'https://zhanduiwang.top/api/v1/trade/products?page=1&pageSize=1' -TimeoutSec 20
$head=(Invoke-WebRequest -Uri 'https://zhanduiwang.top/api/v1/miniapp/media/wechat/review-content' -Method Head -SkipHttpErrorCheck -TimeoutSec 20).StatusCode
$get=(Invoke-WebRequest -Uri 'https://zhanduiwang.top/api/v1/miniapp/media/wechat/review-content' -Method Get -SkipHttpErrorCheck -TimeoutSec 20).StatusCode
$post=(Invoke-WebRequest -Uri 'https://zhanduiwang.top/api/v1/miniapp/media/wechat/review-content' -Method Post -SkipHttpErrorCheck -TimeoutSec 20).StatusCode
$options=(Invoke-WebRequest -Uri 'https://zhanduiwang.top/api/v1/miniapp/media/wechat/review-content' -Method Options -SkipHttpErrorCheck -TimeoutSec 20).StatusCode
if ($health.code -ne 0 -or $health.data.status -ne 'ok' -or $products.code -ne 0) { throw '公网基础能力失败' }
if ($head -ne 404 -or $get -ne 404 -or $post -ne 405 -or $options -ne 405) { throw '代理负向方法门禁失败' }
Write-Output "PUBLIC_PREWINDOW_OK head=$head get=$get post=$post options=$options"
```

Expected: `404/404/405/405`。这些请求必须发生在验收窗口开始前。

- [ ] **Step 2: 等待日志落盘后记录新窗口**

等待上述四条请求写入后，读取 Nginx 当前 inode、总行数和当前 UTC epoch。使用 `apply_patch` 新建独立收据 `.release/review-proxy-acceptance-window-v2.json`，包含：本次 deployId、release、artifactSha、acceptanceEpoch、nginxStartLine、nginxInode、startedAtUtc 和独立结果路径。不得覆盖第一次窗口收据。

Expected: 窗口起点晚于所有故意 404/405，inode 与行号均为正整数。

### Task 4: 使用全新 PNG 执行真实微信审核

**Files:**
- Create runtime script: `.release/automator/review-proxy-acceptance-v2.js`
- Create runtime result: 由窗口收据中的 `$deployId` 计算 `.release/automator/review-proxy-$deployId.json`

- [ ] **Step 1: 通过 apply_patch 写入无秘密输出的自动化脚本**

使用 `apply_patch` 创建 `.release/automator/review-proxy-acceptance-v2.js`，完整内容如下：

```javascript
'use strict';

const crypto = require('crypto');
const zlib = require('zlib');
const automator = require('./node_modules/miniprogram-automator');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

const crcTable = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1);
    table[n] = c >>> 0;
  }
  return table;
})();

function crc32(buffer) {
  let c = 0xffffffff;
  for (const byte of buffer) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function pngChunk(type, data) {
  const typeBuffer = Buffer.from(type, 'ascii');
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length, 0);
  const checksum = Buffer.alloc(4);
  checksum.writeUInt32BE(crc32(Buffer.concat([typeBuffer, data])), 0);
  return Buffer.concat([length, typeBuffer, data, checksum]);
}

function createSafePNG() {
  const width = 512;
  const height = 512;
  const stride = 1 + width * 4;
  const raw = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y += 1) {
    const row = y * stride;
    raw[row] = 0;
    for (let x = 0; x < width; x += 1) {
      const p = row + 1 + x * 4;
      raw[p] = 58 + Math.floor((x / width) * 30);
      raw[p + 1] = 126 + Math.floor((y / height) * 30);
      raw[p + 2] = 184;
      raw[p + 3] = 255;
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;
  ihdr[9] = 6;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    pngChunk('IHDR', ihdr),
    pngChunk('IDAT', zlib.deflateSync(raw, {level: 9})),
    pngChunk('IEND', Buffer.alloc(0)),
  ]);
}

async function main() {
  const image = createSafePNG();
  const app = await automator.connect({wsEndpoint: 'ws://127.0.0.1:9420'});
  try {
    const token = await app.callWxMethod('getStorageSync', 'domainA.token');
    if (typeof token !== 'string' || token.length < 32) throw new Error('AUTH_TOKEN_NOT_AVAILABLE');
    const base = 'https://zhanduiwang.top/api/v1';
    async function api(path, options = {}) {
      const response = await fetch(base + path, {
        ...options,
        headers: {
          'content-type': 'application/json',
          Authorization: `Bearer ${token}`,
          ...(options.headers || {}),
        },
      });
      const body = await response.json();
      if (!response.ok || body.code !== 0) throw new Error(`API_FAILED path=${path} status=${response.status} code=${body.code}`);
      return body.data;
    }
    await api('/auth/me');
    const session = await api('/miniapp/media/uploads', {
      method: 'POST',
      body: JSON.stringify({
        purpose: 'product',
        sizeBytes: image.length,
        contentMD5: crypto.createHash('md5').update(image).digest('base64'),
        contentType: 'image/png',
        idempotencyKey: `review_proxy_v2:${Date.now()}:${crypto.randomBytes(8).toString('hex')}`,
      }),
    });
    console.log(`AUTHORIZE_OK asset=${session.assetId} bytes=${image.length}`);
    const upload = await fetch(session.uploadUrl, {
      method: 'PUT',
      body: image,
      headers: {
        'Content-Type': session.requiredHeaders.contentType,
        'Content-MD5': session.requiredHeaders.contentMD5,
        'Content-Length': String(session.requiredHeaders.contentLength),
        'x-cos-forbid-overwrite': session.requiredHeaders.forbidOverwrite,
      },
    });
    if (!upload.ok) throw new Error(`COS_PUT_FAILED status=${upload.status}`);
    const complete = await api(`/miniapp/media/uploads/${session.ticketId}/complete`, {method: 'POST', body: '{}'});
    console.log(`COMPLETE_OK state=${complete.state} review=${complete.reviewStatus}`);
    const deadline = Date.now() + 30 * 60 * 1000;
    while (Date.now() < deadline) {
      const status = await api(`/miniapp/media/assets/${session.assetId}`);
      console.log(`MEDIA_STATUS state=${status.state} review=${status.reviewStatus} canUse=${status.canUse}`);
      if (status.state === 'approved' && status.canUse === true && status.technicalStatus === 'final_ready' &&
          status.reviewStatus === 'approved' && status.ticketStatus === 'consumed' && status.recovery === 'none') {
        console.log(`MEDIA_APPROVED asset=${session.assetId} ticket=${session.ticketId} state=${status.state} canUse=${status.canUse} technical=${status.technicalStatus} review=${status.reviewStatus} ticketStatus=${status.ticketStatus} recovery=${status.recovery}`);
        return;
      }
      if (['rejected', 'review_failed', 'expired', 'upload_failed', 'deleted'].includes(status.state)) {
        throw new Error(`MEDIA_TERMINAL state=${status.state}`);
      }
      await sleep(5000);
    }
    throw new Error(`MEDIA_TIMEOUT asset=${session.assetId}`);
  } finally {
    await Promise.resolve(app.disconnect());
  }
}

main().catch((error) => {
  console.error(error.message || String(error));
  process.exitCode = 1;
});
```

该脚本只在内存使用 token 和上传 URL，只打印新 assetId、ticketId 和封闭状态。

- [ ] **Step 2: 启动微信开发者工具并运行脚本**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
& 'D:\3314\微信web开发者工具\cli.bat' auto --project 'C:\Users\33141\Documents\New project\apps\miniapp' --auto-port 9420 --trust-project --lang zh
if ($LASTEXITCODE -ne 0) { throw '微信开发者工具自动化启动失败' }
& 'D:\3314\微信web开发者工具\node-18.exe' '.release\automator\review-proxy-acceptance-v2.js'
if ($LASTEXITCODE -ne 0) { throw '真实媒体验收失败' }
```

Expected: 依次出现 `AUTHORIZE_OK`、`COMPLETE_OK`、若干 `MEDIA_STATUS` 和 `MEDIA_APPROVED`；新 asset/ticket 与第一次不同。

### Task 5: 严格日志、数据库、运行身份和清理门禁

**Files:**
- Update runtime receipt: `.release/review-proxy-acceptance-window-v2.json`
- Create runtime result: 由窗口收据中的 `$deployId` 计算 `.release/automator/review-proxy-$deployId.json`
- Update runtime deploy receipt: `.release/review-proxy-redeploy.json`

- [ ] **Step 1: 冻结窗口终点并只统计窗口内流量**

先读取当前 inode 和总行数。inode 必须等于起点 inode；若发生轮转，停止、保留 stage，并使用另一张全新图片开启新窗口，不拼接不完整日志。

对闭区间 `nginxStartLine+1 .. nginxEndLine` 解析 `$request_method $uri $server_protocol`，必须满足：

- 代理 GET 200 至少 1 次；
- callback POST 200 至少 1 次；
- 代理全部非 200 为 0；
- callback 全部非 200 为 0；
- HEAD 可以为 0；如果 HEAD 大于 0，则全部为 200；
- Nginx 与 journald 的 `token=|signature=|RawQuery|versionId=|final/` 计数为 0；
- `panic|fatal|bind: address|permission denied` 计数为 0。

- [ ] **Step 2: 使用新 asset ULID 执行唯一只读 SQL**

Run SQL:

```sql
SELECT a.storage_stage,a.technical_status,t.status,p.status,p.applied_decision_version
FROM media_assets a
JOIN media_upload_tickets t ON t.asset_id=a.id
JOIN media_review_projections p ON p.asset_id=a.id
WHERE a.asset_public_id=?;
```

Expected: 唯一一行 `final / final_ready / consumed / approved / version>=1`。参数只允许新 asset ULID；不查询 OpenID、key/version、trace 或凭据。

- [ ] **Step 3: 最终复核服务、SHA、备份和用户工作树边界**

必须同时确认：

- 磁盘与 `/proc/$pid/exe` SHA 均为新 SHA；
- 四项服务 active，内网和公网 `/healthz` 正常，商品列表正常；
- 本次新 backup 的 `campus-api` 和 `READY` 存在，backup SHA 为部署前旧 SHA；
- 第一次 backup、stage 和三份旧收据仍存在且未改；
- Git 暂存区为空，目标源码无新差异，用户既有脏文件保持原样。

任一失败使用本次新 backup 执行 exact CAS rollback；仅当 live SHA 等于本次新 SHA 才允许恢复。

- [ ] **Step 4: 只清理本次新 staging 并写最终收据**

在 `flock` 下再次验证本次 `stageFile` 为普通文件、非 symlink、owner `admin`、mode `600`、SHA 等于新 SHA，且 live SHA 仍是新 SHA。然后只执行：

```bash
rm -f -- "$stageFile"
rmdir -- "$stageDir"
```

不清理第一次失败 stage。通过 `apply_patch` 写入本次部署和验收收据：`stagingCleaned=true`、`rolledBack=false`、严格媒体状态、GET/callback/条件 HEAD 计数、数据库状态、日志计数、磁盘/进程 SHA 和完成时间。

Expected: 输出 `FINAL_CLOSEOUT_OK`；两个 exact backup 永久保留，第一次失败证据保持不变，生产运行新 SHA。
