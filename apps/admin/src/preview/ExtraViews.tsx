import { useState } from 'react'
import {
  Button as AntButton,
  Drawer as AntDrawer,
  Form,
  Input,
  InputNumber,
  Modal as AntModal,
  Radio,
  Select,
  Space,
  Steps,
  Table,
  Tabs,
  Tag
} from 'antd'
import { formatMoney } from './data'
import { useExtraStore, type AdRow, type ChatSessionRow, type CouponRow, type LaunchStage, type MiniAppSettings, type WithdrawalRow } from './extra-data'
import { EmptyState, PageHeader, Panel, SearchBox, StatCard, StatusPill, usePreviewToast } from './ui'

export function LaunchProcessView() {
  const stages = useExtraStore(state => state.launchStages)
  const toggleLaunchStep = useExtraStore(state => state.toggleLaunchStep)
  const { notify } = usePreviewToast()
  const total = stages.reduce((sum, stage) => sum + stage.steps.length, 0)
  const done = stages.reduce((sum, stage) => sum + stage.steps.filter(step => step.done).length, 0)

  const items = stages.map(stage => ({
    title: stage.title,
    key: stage.key,
    description: stage.steps.map(step => (
      <div key={step.key} className="preview-launch-step">
        <span>{step.done ? '✓' : '·'}</span>
        <span>{step.title}</span>
        <AntButton
          type="link"
          size="small"
          onClick={() => {
            toggleLaunchStep(stage.key, step.key)
            notify(step.done ? `${step.title} 已恢复为未完成` : `${step.title} 已完成`, 'info')
          }}
        >
          {step.action}
        </AntButton>
      </div>
    ))
  }))

  return <div>
    <PageHeader title="上线流程" actions={<AntButton type="primary" disabled={done < total}>下一步</AntButton>} />
    <section className="preview-stat-grid">
      <StatCard label="总步骤" value={total} meta="全部上线步骤" tone="accent" marker="步" />
      <StatCard label="已完成" value={done} meta="完成进度" tone="mint" marker="✓" />
      <StatCard label="阶段数" value={stages.length} meta="基础资料到发布" tone="ink" marker="段" />
      <StatCard label="完成度" value={`${Math.round((done / Math.max(total, 1)) * 100)}%`} meta="当前进度" tone="amber" marker="%" />
    </section>
    <Panel title="小程序上线进度">
      <Steps direction="vertical" current={done} items={items} />
    </Panel>
  </div>
}

export function SiteManagementView() {
  const siteTypes = useExtraStore(state => state.siteTypes)
  const sites = useExtraStore(state => state.sites)
  const [tab, setTab] = useState('types')

  return <div>
    <PageHeader title="站点管理" actions={<AntButton type="primary">新增站点类型</AntButton>} />
    <Tabs
      activeKey={tab}
      onChange={setTab}
      items={[
        {
          key: 'types',
          label: '站点类型',
          children: <Table
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={siteTypes}
            columns={[
              { title: '编号', dataIndex: 'id' },
              { title: '类型', dataIndex: 'name' },
              { title: '平台费率', render: (_, row) => `${(row.platformRate * 100).toFixed(1)}%` },
              { title: '代理费率', render: (_, row) => `${(row.agentRate * 100).toFixed(1)}%` },
              { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
              { title: '操作', render: () => <Space><AntButton type="link" size="small">编辑</AntButton><AntButton type="link" size="small">删除</AntButton></Space> }
            ]}
          />
        },
        {
          key: 'sites',
          label: '我的站点',
          children: <Table
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={sites}
            columns={[
              { title: '编号', dataIndex: 'id' },
              { title: '站点名称', dataIndex: 'name' },
              { title: '站点类型', dataIndex: 'type' },
              { title: '余额', render: (_, row) => formatMoney(row.balanceCents) },
              { title: '累计收入', render: (_, row) => formatMoney(row.incomeCents) },
              { title: '代理', dataIndex: 'agents' },
              { title: '接单员', dataIndex: 'takers' },
              { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
              { title: '最近更新', dataIndex: 'updatedAt' },
              { title: '操作', render: () => <AntButton type="link" size="small">编辑</AntButton> }
            ]}
          />
        },
        {
          key: 'settings',
          label: '设置',
          children: <Form layout="vertical" className="preview-form-stack" style={{ maxWidth: 520 }}>
            <Form.Item label="站点名称"><Input defaultValue="湛科生活圈" /></Form.Item>
            <Form.Item label="默认接单范围"><Select defaultValue="校园内" options={[{ value: '校园内' }, { value: '校外周边' }]} /></Form.Item>
            <AntButton type="primary">提交保存</AntButton>
          </Form>
        }
      ]}
    />
  </div>
}

export function FormManagementView() {
  const forms = useExtraStore(state => state.forms)
  const setFormStatus = useExtraStore(state => state.setFormStatus)
  const [query, setQuery] = useState('')
  const [detail, setDetail] = useState<(typeof forms)[number] | null>(null)
  const filtered = forms.filter(item => !query || `${item.id}${item.name}${item.user}`.includes(query))
  return <div>
    <PageHeader title="表单管理" actions={<SearchBox value={query} onChange={setQuery} placeholder="搜索表单" />} />
    <Panel title="表单列表">
      <Table
        rowKey="id"
        size="small"
        dataSource={filtered}
        pagination={false}
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '表单信息', render: (_, row) => <><strong>{row.name}</strong><div>{row.designNo}</div></> },
          { title: '提交用户', dataIndex: 'user' },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '时间', dataIndex: 'createdAt' },
          {
            title: '操作',
            render: (_, row) => <Space>
              <AntButton type="link" size="small" onClick={() => setDetail(row)}>查看</AntButton>
              {row.status === '待处理' && <AntButton type="link" size="small" onClick={() => setFormStatus(row.id)}>标记已处理</AntButton>}
            </Space>
          }
        ]}
      />
    </Panel>
    <AntModal open={Boolean(detail)} title="表单详情" footer={<AntButton onClick={() => setDetail(null)}>关闭</AntButton>} onCancel={() => setDetail(null)}>
      {detail && <div>
        <p><strong>表单：</strong>{detail.name}</p>
        <p><strong>用户：</strong>{detail.user}</p>
        <p><strong>时间：</strong>{detail.createdAt}</p>
      </div>}
    </AntModal>
  </div>
}

export function TakerManagementView() {
  const takers = useExtraStore(state => state.takers)
  const setTakerStatus = useExtraStore(state => state.setTakerStatus)
  return <div>
    <PageHeader title="接单员管理" actions={<AntButton type="primary">添加接单员</AntButton>} />
    <section className="preview-stat-grid">
      <StatCard label="接单员" value={takers.length} meta="当前人数" tone="accent" marker="骑" />
      <StatCard label="总完成量" value={takers.reduce((sum, row) => sum + row.completed, 0)} meta="累计完成订单" tone="mint" marker="单" />
      <StatCard label="平均评分" value={takers.reduce((sum, row) => sum + row.rating, 0) / Math.max(takers.length, 1)} meta="五星制" tone="amber" marker="评" />
      <StatCard label="总收入" value={formatMoney(takers.reduce((sum, row) => sum + row.incomeCents, 0))} meta="接单员收入" tone="ink" marker="¥" />
    </section>
    <Panel title="接单员列表">
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={takers}
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '姓名', dataIndex: 'name' },
          { title: '手机号', dataIndex: 'phone' },
          { title: '等级', dataIndex: 'level' },
          { title: '总完成量', dataIndex: 'completed' },
          { title: '总收入', render: (_, row) => formatMoney(row.incomeCents) },
          { title: '准时率', render: (_, row) => `${row.onTimeRate}%` },
          { title: '评分', dataIndex: 'rating' },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          {
            title: '操作',
            render: (_, row) => <AntButton
              type="link"
              size="small"
              onClick={() => setTakerStatus(row.id, row.status === '正常' ? '停用' : '正常')}
            >
              {row.status === '正常' ? '停用' : '恢复'}
            </AntButton>
          }
        ]}
      />
    </Panel>
  </div>
}

export function PromoterManagementView() {
  const promoters = useExtraStore(state => state.promoters)
  const setPromoterStatus = useExtraStore(state => state.setPromoterStatus)
  return <div>
    <PageHeader title="推广员管理" actions={<AntButton type="primary">添加推广员</AntButton>} />
    <section className="preview-stat-grid">
      <StatCard label="推广员" value={promoters.length} meta="当前人数" tone="accent" marker="推" />
      <StatCard label="推广订单" value={promoters.reduce((sum, row) => sum + row.orders, 0)} meta="累计订单" tone="mint" marker="单" />
      <StatCard label="推广奖励" value={formatMoney(promoters.reduce((sum, row) => sum + row.rewardCents, 0))} meta="累计发放" tone="amber" marker="¥" />
    </section>
    <Panel title="推广员列表">
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={promoters}
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '昵称', dataIndex: 'nickname' },
          { title: '推广订单', dataIndex: 'orders' },
          { title: '奖励', render: (_, row) => formatMoney(row.rewardCents) },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '时间', dataIndex: 'createdAt' },
          {
            title: '操作',
            render: (_, row) => <AntButton type="link" size="small" onClick={() => setPromoterStatus(row.id, row.status === '启用' ? '停用' : '启用')}>{row.status === '启用' ? '停用' : '启用'}</AntButton>
          }
        ]}
      />
    </Panel>
  </div>
}

export function InventoryView() {
  const skus = useExtraStore(state => state.skus)
  const updateSkuStock = useExtraStore(state => state.updateSkuStock)
  const toggleSkuStatus = useExtraStore(state => state.toggleSkuStatus)
  const [editing, setEditing] = useState<(typeof skus)[number] | null>(null)
  const [stock, setStock] = useState(0)
  return <div>
    <PageHeader title="商品库存管理" actions={<Space><AntButton>导入商品</AntButton><AntButton type="primary">批量修改库存</AntButton></Space>} />
    <Panel title="库存管理">
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={skus}
        columns={[
          { title: '商品编号', dataIndex: 'id' },
          { title: '商品名称', dataIndex: 'name' },
          { title: '库存', dataIndex: 'stock' },
          { title: '已售', dataIndex: 'sold' },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          {
            title: '操作',
            render: (_, row) => <Space>
              <AntButton type="link" size="small" onClick={() => { setEditing(row); setStock(row.stock) }}>修改库存</AntButton>
              <AntButton type="link" size="small" onClick={() => toggleSkuStatus(row.id)}>{row.status === '上架' ? '下架' : '上架'}</AntButton>
            </Space>
          }
        ]}
      />
    </Panel>
    <AntModal
      open={Boolean(editing)}
      title={`修改库存 · ${editing?.name || ''}`}
      onCancel={() => setEditing(null)}
      footer={<Space><AntButton onClick={() => setEditing(null)}>取消</AntButton><AntButton type="primary" onClick={() => {
        if (editing) updateSkuStock(editing.id, stock)
        setEditing(null)
      }}>保存</AntButton></Space>}
    >
      <Form layout="vertical">
        <Form.Item label="库存数量"><InputNumber min={0} value={stock} onChange={value => setStock(Number(value || 0))} style={{ width: '100%' }} /></Form.Item>
      </Form>
    </AntModal>
  </div>
}

export function MarketingView() {
  const coupons = useExtraStore(state => state.coupons)
  const onceCards = useExtraStore(state => state.onceCards)
  const addCoupon = useExtraStore(state => state.addCoupon)
  const toggleCoupon = useExtraStore(state => state.toggleCoupon)
  const toggleOnceCard = useExtraStore(state => state.toggleOnceCard)
  const [addOpen, setAddOpen] = useState(false)
  const [form] = Form.useForm()

  const submitCoupon = async () => {
    const values = await form.validateFields()
    addCoupon({
      id: `CP-${Date.now().toString(36).toUpperCase()}`,
      name: values.name,
      scope: values.scope || '全部服务',
      discountCents: Math.round((Number(values.discount) || 0) * 100),
      conditionCents: Math.round((Number(values.condition) || 0) * 100),
      validDays: Number(values.validDays) || 7,
      claimed: 0,
      used: 0,
      status: '启用'
    })
    setAddOpen(false)
    form.resetFields()
  }

  const tabItems = [
    {
      key: 'coupons',
      label: '优惠券列表',
      children: <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={coupons}
        columns={[
          { title: '优惠券名称', dataIndex: 'name' },
          { title: '适用范围', dataIndex: 'scope' },
          { title: '优惠金额', render: (_, row) => formatMoney(row.discountCents) },
          { title: '满足条件', render: (_, row) => `满 ${formatMoney(row.conditionCents)}` },
          { title: '有效天数', dataIndex: 'validDays' },
          { title: '领取/使用', render: (_, row) => `${row.claimed} / ${row.used}` },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '操作', render: (_, row) => <AntButton type="link" size="small" onClick={() => toggleCoupon(row.id)}>{row.status === '启用' ? '停用' : '启用'}</AntButton> }
        ]}
      />
    },
    {
      key: 'oncecards',
      label: '次卡列表',
      children: <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={onceCards}
        columns={[
          { title: '次卡名称', dataIndex: 'name' },
          { title: '有效天数', dataIndex: 'validDays' },
          { title: '单次价格', render: (_, row) => formatMoney(row.priceCents) },
          { title: '卡简介', dataIndex: 'description' },
          { title: '购买人次', dataIndex: 'buyers' },
          { title: '使用次数', dataIndex: 'used' },
          { title: '收入', render: (_, row) => formatMoney(row.incomeCents) },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '操作', render: (_, row) => <AntButton type="link" size="small" onClick={() => toggleOnceCard(row.id)}>{row.status === '启用' ? '停用' : '启用'}</AntButton> }
        ]}
      />
    }
  ]

  return <div>
    <PageHeader title="营销管理" actions={<AntButton type="primary" onClick={() => setAddOpen(true)}>添加优惠券</AntButton>} />
    <Panel title="营销管理">
      <Tabs items={tabItems} />
    </Panel>
    <AntModal
      open={addOpen}
      title="添加优惠券"
      onCancel={() => setAddOpen(false)}
      footer={<Space><AntButton onClick={() => setAddOpen(false)}>取消</AntButton><AntButton type="primary" onClick={submitCoupon}>保存</AntButton></Space>}
    >
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="优惠券名称" rules={[{ required: true, message: '请输入名称' }]}><Input /></Form.Item>
        <Form.Item name="scope" label="适用范围"><Input /></Form.Item>
        <Form.Item name="discount" label="优惠金额（元）" rules={[{ required: true, message: '请输入金额' }]}><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="condition" label="满足条件（元）"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="validDays" label="有效天数"><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
      </Form>
    </AntModal>
  </div>
}

export function ChatManagementView() {
  const sessions = useExtraStore(state => state.chatSessions)
  const setChatStatus = useExtraStore(state => state.setChatStatus)
  const [detail, setDetail] = useState<ChatSessionRow | null>(null)
  return <div>
    <PageHeader title="私信管理" actions={<SearchBox value="" onChange={() => undefined} placeholder="搜索会话" />} />
    <Panel title="会话列表">
      <Table
        rowKey="id"
        size="small"
        dataSource={sessions}
        pagination={false}
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '用户1', dataIndex: 'user1' },
          { title: '用户2', dataIndex: 'user2' },
          { title: '最新消息', dataIndex: 'lastMessage' },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '最新更新', dataIndex: 'updatedAt' },
          {
            title: '操作',
            render: (_, row) => <Space>
              <AntButton type="link" size="small" onClick={() => setDetail(row)}>查看</AntButton>
              <AntButton type="link" size="small" onClick={() => setChatStatus(row.id, row.status === '正常' ? '已封禁' : '正常')}>{row.status === '正常' ? '封禁' : '恢复'}</AntButton>
            </Space>
          }
        ]}
      />
    </Panel>
    <AntDrawer open={Boolean(detail)} title={detail?.id || '会话详情'} onClose={() => setDetail(null)}>
      {detail && <div>
        <p><strong>{detail.user1}</strong> ↔ <strong>{detail.user2}</strong></p>
        <p>最新消息：{detail.lastMessage}</p>
        <p>更新时间：{detail.updatedAt}</p>
      </div>}
    </AntDrawer>
  </div>
}

export function AdsView() {
  const ads = useExtraStore(state => state.ads)
  const toggleAd = useExtraStore(state => state.toggleAd)
  const columns = [
    { title: '广告位ID', dataIndex: 'id' },
    { title: '广告名称', dataIndex: 'name' },
    { title: '观看次数', dataIndex: 'views' },
    { title: '完整观看数', dataIndex: 'completed' },
    { title: '发放奖励数', dataIndex: 'rewards' },
    { title: '状态', render: (_: unknown, row: AdRow) => <StatusPill>{row.status}</StatusPill> },
    { title: '操作', render: (_: unknown, row: AdRow) => <AntButton type="link" size="small" onClick={() => toggleAd(row.id)}>{row.status === '启用' ? '停用' : '启用'}</AntButton> }
  ]
  return <div>
    <PageHeader title="广告管理" actions={<AntButton type="primary">添加广告</AntButton>} />
    <Panel title="广告管理">
      <Tabs
        items={[
          { key: 'reward', label: '激励广告', children: <Table rowKey="id" size="small" pagination={false} dataSource={ads.filter(item => item.kind === '激励广告')} columns={columns} /> },
          { key: 'free', label: '免单广告', children: <Table rowKey="id" size="small" pagination={false} dataSource={ads.filter(item => item.kind === '免单广告')} columns={columns} /> }
        ]}
      />
    </Panel>
  </div>
}

export function MiniProgramView() {
  const miniApp = useExtraStore(state => state.miniApp)
  const saveMiniApp = useExtraStore(state => state.saveMiniApp)
  const [form] = Form.useForm<MiniAppSettings>()
  return <div>
    <PageHeader title="小程序设置" actions={<AntButton type="primary" onClick={() => saveMiniApp(form.getFieldsValue())}>提交保存</AntButton>} />
    <div className="preview-settings-grid">
      <Panel title="小程序信息">
        <Form form={form} initialValues={miniApp} layout="vertical">
          <Form.Item name="appId" label="AppID"><Input /></Form.Item>
          <Form.Item name="name" label="小程序名称"><Input /></Form.Item>
          <Form.Item name="version" label="当前版本"><Input /></Form.Item>
          <Form.Item name="domains" label="业务域名"><Input /></Form.Item>
        </Form>
      </Panel>
      <Panel title="支付设置">
        <Form form={form} layout="vertical">
          <Form.Item name="mchId" label="微信商户ID"><Input /></Form.Item>
          <Form.Item name="apiKeyVersion" label="密钥版本"><Radio.Group options={[{ label: 'APIv2', value: 'v2' }, { label: 'APIv3', value: 'v3' }]} /></Form.Item>
          <Form.Item name="callbackUrl" label="回调地址"><Input /></Form.Item>
        </Form>
      </Panel>
    </div>
  </div>
}

export function ServiceAccountView() {
  const serviceAccount = useExtraStore(state => state.serviceAccount)
  const saveServiceAccount = useExtraStore(state => state.saveServiceAccount)
  const [form] = Form.useForm()
  return <div>
    <PageHeader title="服务号设置" actions={<AntButton type="primary" onClick={() => saveServiceAccount(form.getFieldsValue())}>保存</AntButton>} />
    <Panel title="服务号设置">
      <Form form={form} initialValues={serviceAccount} layout="vertical" style={{ maxWidth: 620 }}>
        <Form.Item name="appId" label="服务号 AppID"><Input /></Form.Item>
        <Form.Item name="name" label="服务号名称"><Input /></Form.Item>
        <Form.Item name="followers" label="关注人数"><InputNumber style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="templateIds" label="模板消息 ID"><Input.TextArea rows={4} /></Form.Item>
      </Form>
    </Panel>
  </div>
}

export function WithdrawalsView() {
  const withdrawals = useExtraStore(state => state.withdrawals)
  const processWithdrawal = useExtraStore(state => state.processWithdrawal)
  return <div>
    <PageHeader title="提现管理" actions={<AntButton type="primary">支付宝提现批量处理</AntButton>} />
    <section className="preview-stat-grid">
      <StatCard label="待处理" value={withdrawals.filter(item => item.status === '待处理').length} meta="需要审核" tone="amber" marker="待" />
      <StatCard label="已处理" value={withdrawals.filter(item => item.status === '已处理').length} meta="已完成打款" tone="mint" marker="✓" />
      <StatCard label="提现金额" value={formatMoney(withdrawals.reduce((sum, item) => sum + item.amountCents, 0))} meta="当前记录合计" tone="ink" marker="¥" />
    </section>
    <Panel title="提现列表">
      <Table
        rowKey="id"
        size="small"
        pagination={false}
        dataSource={withdrawals}
        columns={[
          { title: '编号', dataIndex: 'id' },
          { title: '提现用户', dataIndex: 'user' },
          { title: '提现金额', render: (_, row) => formatMoney(row.amountCents) },
          { title: '提现账户', dataIndex: 'account' },
          { title: '状态', render: (_, row) => <StatusPill>{row.status}</StatusPill> },
          { title: '时间', dataIndex: 'createdAt' },
          {
            title: '操作',
            render: (_, row: WithdrawalRow) => row.status === '待处理' ? <Space>
              <AntButton type="link" size="small" onClick={() => processWithdrawal(row.id, '已处理')}>通过</AntButton>
              <AntButton type="link" size="small" onClick={() => processWithdrawal(row.id, '已拒绝')}>拒绝</AntButton>
            </Space> : null
          }
        ]}
      />
    </Panel>
  </div>
}

export function GeneralSettingsView() {
  const general = useExtraStore(state => state.general)
  const saveGeneral = useExtraStore(state => state.saveGeneral)
  const [form] = Form.useForm()
  return <div>
    <PageHeader title="通用设置" actions={<AntButton type="primary" onClick={() => saveGeneral(form.getFieldsValue())}>提交保存</AntButton>} />
    <Tabs
      items={[
        {
          key: 'system',
          label: '系统设置',
          children: <Form form={form} initialValues={general} layout="vertical" style={{ maxWidth: 620 }}>
            <Form.Item name="systemName" label="系统名称"><Input /></Form.Item>
            <Form.Item name="company" label="公司简称"><Input /></Form.Item>
            <Form.Item name="icp" label="备案号"><Input /></Form.Item>
          </Form>
        },
        {
          key: 'map',
          label: '地图设置',
          children: <Form form={form} layout="vertical" style={{ maxWidth: 620 }}>
            <Form.Item name="tencentMapKey" label="腾讯地图 key"><Input /></Form.Item>
            <Form.Item name="baiduMapAk" label="百度地图 AK"><Input /></Form.Item>
            <Form.Item name="amapKey" label="高德地图 key"><Input /></Form.Item>
          </Form>
        },
        {
          key: 'words',
          label: '违禁词设置',
          children: <Form form={form} layout="vertical" style={{ maxWidth: 620 }}>
            <Form.Item name="prohibitedWords" label="违禁词"><Input.TextArea rows={6} /></Form.Item>
          </Form>
        }
      ]}
    />
  </div>
}
