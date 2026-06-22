# P0-4 Module ⑩ Final Acceptance Report v1.0

- 验收日期：2026-06-21
- 验收范围：Module ⑩ Final E2E Verification
- 最终结论：**FAIL**
- Module ⑩ 状态：**维持 CONDITIONAL PASS，不升级为 PASS**
- P0-4 Media System Final Closure Report：**不发布**
- P0-5：**未进入**

## 1. 验收边界

本轮仅执行同 AppID 微信开发者工具原生链路复验，未新增或修改生产功能、迁移、模型、状态机、Repository、Service、Worker、审核接线、GC 能力或业务接线。临时 AppID 对齐配置已在验收结束后恢复。

## 2. 最终验收矩阵

| 验收项 | 结果 | 真实环境证据 |
|---|---|---|
| 同 AppID | PASS | 微信开发者工具运行 AppID 与后端微信登录配置临时对齐；真实 `wx.login` code 被后端 `jscode2session` 接受 |
| `wx.login` | PASS | 返回 32 字符 code；`POST /api/v1/auth/wechat-login` HTTP 200，测试 owner 为 9 |
| 原生 `chooseMedia` | PASS | 微信开发者工具原生文件选择框成功打开，用户完成 JPEG 选择 |
| 本地压缩 | FAIL | 压缩流程产生输出后，读取压缩图片时报错“无法读取压缩后的图片” |
| Create Upload Ticket | NOT REACHED | 媒体历史接口返回 `total=0`；网络记录中没有媒体票据请求 |
| STS Authorization | NOT REACHED | 未创建 Ticket，未签发媒体 STS 授权 |
| COS PUT | NOT REACHED | 未获得 staging Key，未发起 COS PUT |
| Complete Upload | NOT REACHED | 无已上传对象可完成 |
| Review Projection | NOT REACHED | 无媒体资产进入审核投影 |
| Outbox / Consumer | NOT REACHED | 无媒体事件产生 |
| Final Asset State | NOT REACHED | 无媒体资产产生 |
| MiniApp 状态刷新 | NOT REACHED | 上传在客户端读取阶段终止 |
| 重复 Complete 幂等 | NOT REACHED | 首次 Complete 未发生 |
| Owner 隔离 | NOT REACHED | 本轮未创建可供跨 Owner 访问的资产；既有自动化基线不能替代本轮原生证据 |
| 数据库残留 | PASS | 本轮媒体历史 `total=0`，未产生 Ticket、Asset 或后续事实记录 |
| COS Version / DeleteMarker 残留 | PASS | 本轮未签发 staging Key、未请求 STS、未发起 COS 操作，因此本次验收对象残留为 0 |

## 3. 阻断根因

阻断点位于小程序客户端读取压缩产物阶段，不是临时 STS 配置过期，也未到达后端或 COS。

真实微信开发者工具运行时探针结果：

```text
Object.prototype.toString(value) = [object ArrayBuffer]
value.constructor.name          = ArrayBuffer
value instanceof ArrayBuffer    = false
ArrayBuffer.isView(value)       = false
value.byteLength                = 4
```

`apps/miniapp/src/services/media.ts` 的 `readFileBytes()` 当前只接受 `instanceof ArrayBuffer` 或 `ArrayBuffer.isView(value)`。微信开发者工具返回的跨运行域 ArrayBuffer 两项判断均为 false，因而进入 fail-closed 分支并抛出“无法读取压缩后的图片”。

该失败发生在 Create Upload Ticket 之前，所以没有凭证泄漏、数据库写入或 COS 残留。

## 4. 必须整改项

当前授权明确禁止修改代码，因此本轮未实施修复。后续需单独授权一个最小兼容性修复：

1. 仅调整 `readFileBytes()` 对跨运行域 ArrayBuffer 的识别与安全拷贝；
2. 保留类型、长度与字节边界校验，禁止用无条件类型断言绕过验证；
3. 增加微信开发者工具真实运行时回归证据；
4. 修复后从 `wx.login` 开始重新执行完整原生链路及全部残留检查。

## 5. 环境清理

- 临时同 AppID 私有配置：已恢复；
- Module ⑩ 临时验收服务：已停止；
- 微信开发者工具自动化监听：已停止；
- 本次 JPEG/PNG、自动化脚本及日志临时目录：已删除；
- 临时目录残留：0；
- 数据库媒体残留：0；
- 本次 COS 对象版本及 DeleteMarker 残留：0。

## 6. 最终判定

同 AppID、真实 `wx.login` 与原生 `chooseMedia` 已取得真实证据，但完整链路在客户端读取压缩图片时中断。Ticket、STS、COS PUT、Completion、Projection、Consumer 和最终状态刷新均未发生，不能认定 Final E2E 通过。

因此：

- **Module ⑩ Final E2E Verification：FAIL**
- **Module ⑩：继续保持 CONDITIONAL PASS**
- **P0-4：不得正式关闭**
- **P0-4 Media System Final Closure Report：不发布**
- **P0-5：不得启动**
