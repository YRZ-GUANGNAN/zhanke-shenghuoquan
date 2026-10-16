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

// 商家数量只能来自真实接口结果，加载期不能闪现 1/1/1 的占位数量。
assert.doesNotMatch(
  merchantHomeSource,
  /pending:\s*1,\s*delivering:\s*1,\s*completed:\s*1/,
  '商家工作台不得预置 pending/delivering/completed 数量',
)
assert.match(merchantHomeSource, /ordersTotalByTab/, '商家工作台应使用真实订单总数')
assert.match(merchantHomeSource, /loading/, '商家工作台应有加载态语义')

// 详情页联系骑手入口复用成熟图标，保持配送私聊能力和现有服务边界。
assert.match(detailSource, /MerchantHomeIcon[\s\S]{0,240}name=['"]message['"]|name=['"]message['"][\s\S]{0,240}MerchantHomeIcon/, '配送详情页应使用 message 图标')

// 只允许继续调用现有配送 service 路径，不因视觉优化引入新 API。
assert.match(riderServiceSource, /\/delivery\/rider\/orders/)
assert.match(merchantServiceSource, /\/delivery\/merchant\/orders/)

console.log('delivery stage workbench contract: ok')
