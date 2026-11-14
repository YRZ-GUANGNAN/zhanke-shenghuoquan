import { formatMoney } from '../data'
import { usePreviewStore } from '../store'
import {
  Button,
  PageHeader,
  Panel,
  StatCard,
  StatusPill
} from '../ui'

export function LegacyTakeoutView() {
  const merchants = usePreviewStore(state => state.takeoutMerchants)
  const enabled = usePreviewStore(state => state.settings.features.takeout)
  const setSection = usePreviewStore(state => state.setSection)
  return <div>
    <PageHeader
      eyebrow="历史模块"
      title="旧版外卖"
      description="只保留存量商家与订单概览，避免和新配送域产生两套并行业务。"
      actions={<Button variant="ghost" onClick={() => setSection('feature-policy')}>管理功能开关</Button>}
    />
    {!enabled && <div className="preview-legacy-notice">
      <span>当前未启用</span>
      <p>旧版外卖入口目前关闭。下方数据仅用于查看历史配置，不提供真实业务操作。</p>
    </div>}
    <section className="preview-stat-grid">
      <StatCard label="存量商家" value={merchants.length} meta="只读概览" tone="accent" marker="商" />
      <StatCard label="今日订单" value={merchants.reduce((sum, item) => sum + item.ordersToday, 0)} meta="历史演示数据" tone="amber" marker="单" />
      <StatCard label="今日营收" value={formatMoney(merchants.reduce((sum, item) => sum + item.revenueCents, 0))} meta="仅作展示" tone="mint" marker="¥" />
      <StatCard label="业务模式" value="只读" meta="不支持新增操作" tone="ink" marker="静" />
    </section>
    <Panel eyebrow="LEGACY DATA" title="历史商家概览" description="此页不触发订单、结算或支付操作。">
      <div className="preview-table-wrap">
        <table className="preview-table"><thead><tr><th>商家</th><th>分类</th><th>今日订单</th><th>今日营收</th><th>状态</th></tr></thead>
          <tbody>{merchants.map(merchant => <tr key={merchant.id}><td><strong>{merchant.name}</strong><span>{merchant.id}</span></td><td>{merchant.category}</td><td>{merchant.ordersToday}</td><td>{formatMoney(merchant.revenueCents)}</td><td><StatusPill>{merchant.status}</StatusPill></td></tr>)}</tbody>
        </table>
      </div>
      <div className="preview-policy-note">
        <span>迁移原则</span>
        <p>新业务继续使用“配送与结算”模块。旧版外卖只用于存量数据查看，避免重复编码业务规则。</p>
      </div>
    </Panel>
  </div>
}
