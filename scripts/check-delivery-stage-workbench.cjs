const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')

function read(...parts) {
  return fs.readFileSync(path.join(root, ...parts), 'utf8')
}

const riderSource = read('apps', 'miniapp', 'src', 'pages', 'delivery-rider', 'index.tsx')
const merchantHomeSource = read('apps', 'miniapp', 'src', 'pages', 'merchant-home', 'index.tsx')
const detailSource = read('apps', 'miniapp', 'src', 'pages', 'merchant-order-detail', 'index.tsx')
const riderServiceSource = read('apps', 'miniapp', 'src', 'services', 'delivery-rider.ts')
const merchantServiceSource = read('apps', 'miniapp', 'src', 'services', 'delivery-merchant.ts')

// 骑手动作必须按订单隔离，并在动作成功后安全刷新列表。
assert.match(riderSource, /busyByOrder/, '骑手页应使用按订单记录的忙碌状态')
assert.match(riderSource, /refreshAfterAction/, '骑手页应区分动作结果与列表刷新结果')
assert.doesNotMatch(
  riderSource,
  /disabled=\{!!busyOrderId\}/,
  '骑手页不得用全局 busyOrderId 锁定所有订单',
)
assert.doesNotMatch(
  riderSource,
  /if\s*\(busyOrderId\)/,
  '骑手动作入口不得用全局 busyOrderId 拦截其他订单',
)

// 分页从第 1 页开始；总量和加载状态必须独立，不把页码当作订单数。
assert.match(
  merchantHomeSource,
  /function initialPagesByTab\([^)]*\)[^{]*\{[\s\S]*?return \{ pending: 1, delivering: 1, completed: 1 \}/,
  '商家订单分页应保留第 1 页起算',
)
assert.match(
  merchantHomeSource,
  /function emptyTotalsByTab\([^)]*\)[^{]*\{[\s\S]*?return \{ pending: 0, delivering: 0, completed: 0 \}/,
  '商家订单总量应独立从 0 初始化',
)
assert.match(merchantHomeSource, /loadedByTab\[tab\.key\]\s*\?\s*ordersTotalByTab\[tab\.key\]\s*:\s*['"]—['"]/, '商家标签数量应区分尚未加载与真实总量')
assert.match(merchantHomeSource, /pending:\s*pendingResult\.ok\s*\?\s*pendingResult\.value\.total\s*:\s*current\.pending/, '商家刷新失败应保留已读取的总量')

// 详情页联系骑手入口复用成熟图标，保持配送私聊能力和现有服务边界。
assert.match(detailSource, /<MerchantHomeIcon\b[^>]*\bname=['"]message['"]/, '配送详情页应使用 message 图标')

// service 通过 BASE 拼接路径；检查现有前缀与订单路径，避免依赖并不存在的完整字面量。
assert.match(riderServiceSource, /const BASE = ['"]\/delivery\/rider['"]/, '保留骑手配送服务路径')
assert.match(riderServiceSource, /\$\{BASE\}\/orders/, '保留骑手配送订单路径')
assert.match(merchantServiceSource, /const BASE = ['"]\/delivery\/merchant['"]/, '保留商家配送服务路径')
assert.match(merchantServiceSource, /\$\{BASE\}\/orders/, '保留商家配送订单路径')
assert.match(riderServiceSource, /return riderCommand\(order\.deliveryOrderPublicId, 'claim', order\.rowVersion, 'POST'\)/, '抢单应保留 POST 与 rowVersion')
assert.match(riderServiceSource, /method: 'POST' \| 'PATCH' = 'PATCH'/, '普通骑手阶段动作应继续使用 PATCH')
assert.match(riderServiceSource, /method: 'PATCH',\s*data: \{ expectedRowVersion, proofAssetPublicId \}/, '凭证动作应保留 PATCH、版本与媒体字段')
assert.match(riderServiceSource, /data: \{ expectedStatus: order\.status, expectedRowVersion: order\.rowVersion \}/, '释放订单应保留状态与版本检查')
assert.match(merchantServiceSource, /method: 'POST',\s*data: input,\s*header: \{ 'Idempotency-Key': input\.clientRequestId \}/, '创建配送单应保留 POST 与幂等键')
assert.match(merchantServiceSource, /method: 'PATCH',\s*data: \{ expectedStatus: order\.status, expectedRowVersion: order\.rowVersion \}/, '取消配送单应保留 PATCH、状态与版本检查')

console.log('delivery stage workbench contract: ok')
