# 骑手与商家工作区 B 方向 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将已确认的 B 方向角色资料页落到真实 Taro 页面，保持已经落地的 v10 订单、发单、消息界面不变。

**Architecture:** 只调整 `delivery-profile` 的渲染树和页面级样式。接口、真实数据、账户隔离、身份确认锁和横滑保护原样保留；复用当前项目的 token 与组件，不增加依赖。

**Tech Stack:** Taro / React / TypeScript / Sass，现有 Node 隔离组件测试。

---

## 范围与执行方式

实际实现仓库：`C:/Users/33141/Documents/New project`，基线 `0393fd5`。用户已确认 B 方向及完整总览并要求立即实施，沿用已确认的设计、不再次选择视觉方案。

本会话使用协作子任务实现、根任务复核，不创建新工作树、不动旧 a494 产品代码。`superpowers:subagent-driven-development` / `executing-plans` 不在已提供技能中，采用现有协作工具和逐项检查；未安装额外技能。

允许修改：

- `apps/miniapp/src/pages/delivery-profile/index.tsx`：封面并入滚动内容、编辑入口归入身份行、审核状态归入资料标题。
- `apps/miniapp/src/pages/delivery-profile/index.scss`：已批准 B 样式、320px 小屏、减少动态效果偏好。
- `scripts/test-delivery-workspace.cjs`：增量测试，不削弱原有 32 组账户隔离、导航和异步回流用例。
- `scripts/check-role-workbench-ui.cjs`：把旧渐变边界断言替换为 B 方案实色身份区衔接边界断言，其余业务合同保持。
- 本次规格、计划及验收记录。

明确不改：订单工作台、发单、消息、编辑资料、全局 token、后端、接口、路由、依赖及开关；不上传微信、不 push、不部署。

## Task 1：先加回归测试

- [ ] 扩展 `profileHarness` 的可选 `gestureCanActivate` / `swipeHandlers`，默认行为保持原样，测试根节点手势及点击防误触。
- [ ] 新增有图/无图/图失败/换新图、编辑归属与既有路由、真实审核三态与读取/失败、底栏角色、商家钱包关闭说明、骑手无钱包、拒绝手势导航等用例。
- [ ] 执行 `node scripts/test-delivery-workspace.cjs` 确认新结构用例在旧实现上失败（RED），原有用例仍通过。

代表性新增结构断言（完整增量测试由协作任务在测试文件内实现）：

```javascript
const app = profileHarness({ role: 'merchant', paymentEnabled: false });
await app.mount();
const scroll = app.one('delivery-profile__scroll');
check.equal(nodes(scroll, n => hasClass(n, 'delivery-profile__hero')).length, 1);
check.equal(nodes(app.one('delivery-profile__identity'), n => hasClass(n, 'delivery-profile__edit')).length, 1);
app.one('delivery-profile__edit').props.onClick();
await app.flush();
check.equal(app.calls.navigate[0].url, '/pages/edit-profile/index');
await app.unmount();
```

## Task 2：应用唯一批准的页面结构

- [ ] 使用 `apply_patch` 将资料页完整更新为以下源码；业务代码到 `if (!allowed)` 前逐字不变。
- [ ] 编辑入口显示“编辑资料”，保留说明头像、昵称、背景图的 `ariaLabel` 和既有路由，不再保留第二个编辑入口。
- [ ] 资料审核 chip 只呈现真实状态；加载/失败显示对应文案，不在事实行重复。

```tsx
import React, { useEffect, useRef, useState } from 'react'
import { Image, ScrollView, Text, View } from '@tarojs/components'
import { useDidHide, useDidShow } from '@tarojs/taro'
import { CapsuleSystemRow } from '../../components/capsule-page-header'
import { AppButton, AppDialog } from '../../components/ui'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import DeliveryWorkspaceNav, { useDeliveryWorkspaceNavigation } from '../../components/delivery-workspace-nav'
import ChatAvatar from '../../components/chat-avatar'
import AppIcon from '../../components/app-icon'
import { identityRoute, useIdentityGuard } from '../../hooks/use-identity-guard'
import { getIdentityApplication } from '../../services/a-user'
import { useDeliveryPayment } from '../../hooks/use-delivery-payment'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { IdentityApplicationView } from '../../types'
import './index.scss'

function DeliveryProfileContent() {
  const { enabled: deliveryPaymentEnabled } = useDeliveryPayment()
  const role = Taro.getCurrentInstance().router?.params.role === 'merchant' ? 'merchant' : 'rider'
  const allowed = useIdentityGuard(role)
  const profile = useSessionStore(state => state.profile)
  const switchIdentity = useSessionStore(state => state.switchIdentity)
  const [application, setApplication] = useState<IdentityApplicationView | null>(null)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)
  const [confirmSwitch, setConfirmSwitch] = useState(false)
  const [switching, setSwitching] = useState(false)
  const [switchError, setSwitchError] = useState('')
  const [failedCover, setFailedCover] = useState('')
  const workspace = useDeliveryWorkspaceNavigation({ role, active: 'mine' }, allowed && !confirmSwitch && !switching)
  const lock = useRef(false)
  const visible = useRef(true)
  const epoch = useRef(0)
  const token = useSessionStore(state => state.token)
  useDidHide(() => { visible.current = false; epoch.current += 1; lock.current = false })
  useDidShow(() => {
    visible.current = true
    setSwitching(false)
    setConfirmSwitch(false)
    setReload(value => value + 1)
  })
  useEffect(() => {
    let active = true
    const version = ++epoch.current
    const current = () => active && visible.current && epoch.current === version
    lock.current = false
    setSwitching(false)
    setConfirmSwitch(false)
    setSwitchError('')
    setApplication(null)
    setError('')
    if (allowed) void getIdentityApplication(role).then(value => {
      if (current()) setApplication(value)
    }).catch(reason => { if (current()) setError(reason instanceof Error ? reason.message : '资料读取失败') })
    return () => { active = false; epoch.current += 1; lock.current = false }
  }, [allowed, role, profile.id, token, reload])
  const switchToStudent = async () => {
    if (lock.current || !visible.current || !allowed) return
    const version = epoch.current
    const accountId = profile.id
    const current = () => {
      const session = useSessionStore.getState()
      return visible.current && version === epoch.current && session.status === 'authenticated'
        && session.profile.id === accountId && session.token === token
    }
    lock.current = true
    setSwitching(true)
    setSwitchError('')
    try {
      const identity = await switchIdentity('student')
      if (!current() || useSessionStore.getState().identity !== identity || identity.currentIdentity !== 'student') return
      await Taro.reLaunch({ url: identityRoute('student') })
    } catch (reason) {
      if (current()) setSwitchError(reason instanceof Error ? reason.message : '切换失败，请重试')
    } finally { if (current()) { lock.current = false; setSwitching(false) } }
  }
  const open = (url: string) => {
    if (!workspace.swipe.canActivate()) return
    void Taro.navigateTo({ url }).catch(() => Taro.showToast({ title: '页面打开失败，请重试', icon: 'none' }))
  }
  if (!allowed) return <View className='delivery-profile' />
  const roleName = role === 'merchant' ? '商家' : '骑手'
  const name = (role === 'merchant' ? application?.merchantName : application?.realName) || profile.nickname || roleName
  const hasCover = Boolean(profile.cover && failedCover !== profile.cover)
  return <View className={`delivery-profile delivery-profile--${role}`} {...workspace.swipe.handlers}>
    <ScrollView scrollY enhanced showScrollbar={false} className='delivery-profile__scroll'>
      <View className={`delivery-profile__hero${hasCover ? ' delivery-profile__hero--cover' : ''}`}>
        {hasCover ? <Image src={profile.cover || ''} mode='aspectFill' className='delivery-profile__hero-image' ariaLabel={`${roleName}个人封面`} onError={() => setFailedCover(profile.cover || '')} /> : null}
        <CapsuleSystemRow className='delivery-profile__system-row' />
        <View className={`delivery-profile__hero-title${hasCover ? ' delivery-profile__hero-title--cover' : ''}`}><Text>我的</Text></View>
      </View>
      <View className={`delivery-profile__identity${hasCover ? ' delivery-profile__identity--cover' : ''}`}>
        <ChatAvatar className='delivery-profile__avatar' src={profile.avatar} name={name} />
        <View className='delivery-profile__name'><Text className='delivery-profile__role'>{roleName}工作区</Text><Text>{name}</Text><Text className='delivery-profile__caption'>{role === 'merchant' ? '管理店铺资料与配送资金' : '管理骑手资料与配送身份'}</Text></View>
        <View className='delivery-profile__edit' ariaRole='button' ariaLabel='编辑头像、昵称与背景图' hoverClass='delivery-profile__edit--pressed' onClick={() => open('/pages/edit-profile/index')}><AppIcon name='document' /><Text>编辑资料</Text></View>
      </View>
      <View className='delivery-profile__section delivery-profile__section--funds'>
        <Text className='delivery-profile__heading'>{role === 'merchant' ? '配送资金' : '配送收入'}</Text>
        {role === 'merchant' ? <View className='delivery-profile__link' ariaRole='button' ariaLabel='查看商家钱包与账单' onClick={() => open('/pages/merchant-wallet/index')}>
          <AppIcon name='wallet' /><View className='delivery-profile__link-copy'><Text>钱包与账单</Text><Text className='delivery-profile__caption'>{deliveryPaymentEnabled ? '查看真实余额、配送支出与充值记录' : '支付功能未开通，查看资金说明'}</Text></View><AppIcon name='chevron-right' />
        </View> : <Text className='delivery-profile__caption'>配送收入以后端结算记录为准，当前未开放骑手端余额查询与提现。</Text>}
      </View>
      <View className='delivery-profile__section'>
        <View className='delivery-profile__heading-row'>
          <Text className='delivery-profile__heading'>{roleName}资料</Text>
          <Text className='delivery-profile__status' ariaLabel='资料审核状态'>
            {error ? '资料待重试' : !application ? '资料读取中' : application.status === 'approved' ? '已通过' : application.status === 'pending' ? '审核中' : '未通过'}
          </Text>
        </View>
        {error ? <Text className='delivery-profile__retry' ariaRole='button' ariaLabel='重新读取资料' onClick={() => setReload(value => value + 1)}>{error} · 点击重试</Text> : !application ? <Text className='delivery-profile__caption'>正在读取已提交资料…</Text> : <>
          <View className='delivery-profile__fact'><Text>联系人</Text><Text>{application.realName || '未填写'}</Text></View>
          <View className='delivery-profile__fact'><Text>联系电话</Text><Text>{application.phone || '未填写'}</Text></View>
          {role === 'merchant' ? <View className='delivery-profile__fact'><Text>取货地址</Text><Text>{application.merchantAddress || '未填写'}</Text></View> : null}
        </>}
      </View>
      <View className='delivery-profile__section delivery-profile__section--account'>
        <Text className='delivery-profile__heading'>账号与身份</Text>
        <Text className='delivery-profile__caption'>在本工作区处理{roleName === '商家' ? '商家订单和配送消息' : '接单、取送货和配送消息'}。切换身份后可返回学生端。</Text>
        <AppButton variant='secondary' onClick={() => { if (workspace.swipe.canActivate()) { setSwitchError(''); setConfirmSwitch(true) } }}>切换到学生端</AppButton>
      </View>
    </ScrollView>
    <DeliveryWorkspaceNav role={role} active='mine' gestureGuard={workspace.swipe.canActivate} />
    <AppDialog visible={confirmSwitch} title='切换到学生端' description='配送订单会保留，再次进入对应身份即可继续处理。'
      confirmText='确认切换' loading={switching} error={switchError}
      onConfirm={() => void switchToStudent()} onCancel={() => { if (!switching) setConfirmSwitch(false) }} />
  </View>
}

export default function DeliveryProfilePage() {
  return <RuntimeSessionGate preserveAuthenticatedChildrenDuringRetry><DeliveryProfileContent /></RuntimeSessionGate>
}
```

## Task 3：落地 B 样式与边界合同

- [ ] 使用 `apply_patch` 将页面样式替换为以下完整规则。相对批准稿仅添加封面旧引擎高度回退、实际像素断点、hoverClass 等效反馈及状态标题换行。

```scss
@use '../../styles/design-tokens.scss' as *;
@use '../../styles/reference-layout.scss' as ref;

// B 方向：资料与封面一体，订单及履约仍留在各自工作台。
.delivery-profile {
  @include ref.page-foundation;
  display: flex;
  flex-direction: column;
  height: 100vh;
  height: 100dvh;
  min-width: 0;
  overflow: hidden;
  background: $bg-app;
  color: $text-primary;
}
.delivery-profile__scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-bottom: 12PX;
}
.delivery-profile__hero {
  position: relative;
  min-height: var(--capsule-system-row-height, 72px);
  overflow: hidden;
  background: $bg-surface;
}
.delivery-profile__hero--cover {
  height: 51vw;
  min-height: 172PX;
  max-height: 218PX;
  height: clamp(172PX, 51vw, 218PX);
  background: $border-default;
}
.delivery-profile__hero-image {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.delivery-profile__system-row { position: relative; z-index: 1; }
.delivery-profile__hero-title { display: none; }
.delivery-profile__identity {
  position: relative;
  display: grid;
  grid-template-columns: 56PX minmax(0, 1fr) auto;
  align-items: center;
  gap: 12PX;
  padding: 24PX 18PX 22PX;
  background: $bg-surface;
}
.delivery-profile__identity--cover {
  margin-top: -20PX;
  border-radius: 20PX 20PX 0 0;
}
.delivery-profile__avatar {
  width: 56PX;
  height: 56PX;
  border: 0;
  border-radius: 16PX;
}
.delivery-profile__avatar .chat-avatar__initial { font-size: 22PX; }
.delivery-profile__name {
  display: flex;
  flex-direction: column;
  gap: 4PX;
  min-width: 0;
  font-size: 23PX;
  line-height: 1.35;
  font-weight: 600;
  color: $text-strong;
  overflow-wrap: anywhere;
}
.delivery-profile__role {
  display: block;
  color: $brand-primary-strong;
  font-size: 11PX;
  line-height: 18PX;
  font-weight: 500;
}
.delivery-profile__name > .delivery-profile__caption { display: none; }
.delivery-profile__edit {
  @include ref.touch;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5PX;
  padding: 0 9PX;
  border: 1PX solid $border-default;
  border-radius: 9PX;
  background: $bg-surface;
  color: $text-primary;
  font-size: 12PX;
  line-height: 18PX;
  font-weight: 500;
  white-space: nowrap;
  cursor: pointer;
}
.delivery-profile__edit .app-icon { width: 15PX; height: 15PX; flex: 0 0 15PX; }
.delivery-profile__section {
  margin: 16PX 16PX 0;
  padding: 16PX;
  border-radius: 14PX;
  background: $bg-surface;
}
.delivery-profile__heading {
  display: block;
  margin-bottom: 12PX;
  color: $text-secondary;
  font-size: 12PX;
  line-height: 18PX;
  font-weight: 500;
}
.delivery-profile__heading-row { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8PX 12PX; margin-bottom: 4PX; }
.delivery-profile__heading-row .delivery-profile__heading { margin: 0; }
.delivery-profile__status {
  padding: 3PX 7PX;
  border-radius: 5PX;
  background: $bg-surface-subtle;
  color: $text-secondary;
  font-size: 11PX;
  line-height: 16PX;
}
.delivery-profile__section--funds { padding-top: 14PX; padding-bottom: 16PX; }
.delivery-profile__link { display: flex; align-items: center; gap: 12PX; min-height: 56PX; cursor: pointer; }
.delivery-profile__link > .app-icon:first-child {
  width: 22PX;
  height: 22PX;
  box-sizing: content-box;
  padding: 11PX;
  border-radius: 12PX;
  background: $brand-tint;
  flex: 0 0 22PX;
}
.delivery-profile__link > .app-icon:last-child { width: 18PX; height: 18PX; flex: 0 0 18PX; }
.delivery-profile__link-copy { flex: 1; min-width: 0; color: $text-strong; font-size: 16PX; line-height: 24PX; font-weight: 600; }
.delivery-profile__caption {
  display: block;
  min-width: 0;
  color: $text-secondary;
  font-size: 12PX;
  line-height: 19PX;
  font-weight: 400;
  overflow-wrap: anywhere;
}
.delivery-profile__link-copy .delivery-profile__caption { margin-top: 3PX; }
.delivery-profile--rider .delivery-profile__section--funds { background: $bg-surface-subtle; }
.delivery-profile--rider .delivery-profile__section--funds .delivery-profile__heading { color: $text-strong; font-size: 15PX; font-weight: 600; margin-bottom: 6PX; }
.delivery-profile__fact {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18PX;
  min-height: 50PX;
  padding: 14PX 0;
  border-bottom: 1PX solid $border-subtle;
  font-size: 14PX;
  line-height: 21PX;
}
.delivery-profile__fact:last-child { border-bottom: 0; padding-bottom: 4PX; min-height: 40PX; }
.delivery-profile__fact > text:first-child { color: $text-secondary; flex: 0 0 4em; }
.delivery-profile__fact > text:last-child { min-width: 0; text-align: right; overflow-wrap: anywhere; color: $text-strong; font-weight: 500; }
.delivery-profile__section--account { margin-bottom: 8PX; }
.delivery-profile__section--account .app-button {
  display: flex;
  width: 100%;
  min-height: 48PX;
  margin-top: 14PX;
  padding: 10PX 16PX;
  border: 0;
  border-radius: 9PX;
  background: $bg-screen;
  color: $text-primary;
  font-size: 14PX;
  line-height: 20PX;
  font-weight: 500;
  cursor: pointer;
}
.delivery-profile__retry { display: flex; align-items: center; min-height: 48PX; padding-top: 8PX; color: $danger; font-size: 14PX; line-height: 22PX; cursor: pointer; }
.delivery-profile__edit, .delivery-profile__link { transition: transform $duration-fast $ease-standard, background-color $duration-fast $ease-standard; }
.delivery-profile__edit--pressed, .delivery-profile__edit:active, .delivery-profile__link:active { transform: scale(.98); background-color: $bg-surface-subtle; }
.delivery-profile__identity, .delivery-profile__section { animation: profile-panel-in $duration-standard $ease-standard both; }
@keyframes profile-panel-in { from { opacity: 0; transform: translateY(6PX); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 340PX) {
  .delivery-profile__identity { grid-template-columns: 48PX minmax(0, 1fr) auto; gap: 9PX; padding-left: 14PX; padding-right: 14PX; }
  .delivery-profile__avatar { width: 48PX; height: 48PX; border-radius: 14PX; }
  .delivery-profile__name { font-size: 21PX; }
  .delivery-profile__edit { padding: 0 8PX; }
  .delivery-profile__edit .app-icon { display: none; }
  .delivery-profile__section { margin-left: 12PX; margin-right: 12PX; padding-left: 14PX; padding-right: 14PX; }
}
@media (prefers-reduced-motion: reduce) {
  .delivery-profile__identity, .delivery-profile__section { animation: none; }
  .delivery-profile__edit, .delivery-profile__link { transition: none; }
  .delivery-profile__edit--pressed, .delivery-profile__edit:active, .delivery-profile__link:active { transform: none; }
}
```

- [ ] 旧检查强制渐变，本次已改为实色身份区覆盖封面下缘；将旧 `profileStyle` gradient 断言替换为下列覆盖等价边界的断言，保留其它断言。

```javascript
assert.match(profileStyle, /\.delivery-profile__hero\s*\{[^}]*overflow:\s*hidden/, 'cover must clip image edges');
assert.match(profileStyle, /\.delivery-profile__identity\s*\{[^}]*background:\s*\$bg-surface/, 'identity must form an opaque cover boundary');
assert.match(profileStyle, /\.delivery-profile__identity--cover\s*\{[^}]*margin-top:\s*-20PX/, 'cover and identity must overlap without a seam');
```

- [ ] 运行 `node scripts/test-delivery-workspace.cjs`、`node scripts/check-role-workbench-ui.cjs`：预期全部通过。

## Task 4：验证、实际代码预览与提交

- [ ] 逐条运行以下命令，必须记录真实 exit code；webpack 已有体积/顺序警告不算失败。

```powershell
npm --workspace apps/miniapp run typecheck
node scripts/check-rider-workbench-clarity.cjs
node scripts/check-role-workbench-ui.cjs
node scripts/check-delivery-stage-workbench.cjs
npm run test:delivery-stage-workbench
node scripts/test-delivery-workspace.cjs
node scripts/check-private-messaging.cjs
npm run build:miniapp
git diff --check
```

- [ ] 用改后真实 TSX/SCSS 生成新的离线资料页预览，不再用候选重排函数；在 320/390/430 宽度检查长昵称、无图/失败回退、编辑按钮、底栏、滚动和真实样式。
- [ ] 根任务复核差异；独立子任务进行规格一致性与代码审查。未能验证的真机手势、安全区、键盘和线上数据明确保留待验收，不将浏览器预览当作真机验收。
- [ ] 执行 `git status --short` 和 `git diff --name-only`，保留原有三个未跟踪计划文件。只暂存本次允许的文件并以中文提交。

```powershell
git add apps/miniapp/src/pages/delivery-profile/index.tsx apps/miniapp/src/pages/delivery-profile/index.scss scripts/test-delivery-workspace.cjs scripts/check-role-workbench-ui.cjs docs/superpowers/specs/2026-10-24-delivery-role-ui-refresh-design.md docs/superpowers/plans/2026-10-24-delivery-role-ui-refresh.md
git commit -m "优化：落地骑手与商家我的页 B 方向"
git log -1 --format="%h %s"
git status --short
```
