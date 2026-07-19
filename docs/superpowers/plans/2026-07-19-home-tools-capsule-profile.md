# 首页真实工具、胶囊安全区与资料入口 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改版整体 UI 的前提下，将“工具”加入首页第三页签、过滤全部占位入口、避开微信胶囊，并让“我的”资料上半区可直接编辑。

**Architecture:** 把工具真实性判断放在 `d-navigation` 服务层，首页只消费已验证的 `ToolLink`。胶囊安全区计算抽成无副作用函数并由 Taro 运行时数据驱动 CSS 变量；资料 Hero 只增加单一导航处理器，避免嵌套点击重复触发。

**Tech Stack:** React 18、Taro 4、TypeScript、SCSS、Node.js 静态合同检查

---

## 文件结构

- Modify: `apps/miniapp/src/services/d-navigation.ts` — 验证普通工具的真实 AppID、path 和占位值。
- Create: `scripts/check-home-real-tools-contract.cjs` — 工具过滤、第三页签和胶囊适配静态合同。
- Modify: `package.json` — 将合同加入根 `verify`。
- Create: `apps/miniapp/src/pages/home/header-layout.ts` — 纯函数计算自定义导航栏安全尺寸。
- Modify: `apps/miniapp/src/services/taro.ts` — 增加胶囊与窗口信息类型。
- Modify: `apps/miniapp/src/taro-shim.d.ts` — 同步 Taro 类型声明。
- Modify: `apps/miniapp/src/pages/home/index.tsx` — 第三页签、工具状态、真实跳转和安全区样式变量。
- Modify: `apps/miniapp/src/pages/home/index.scss` — 工具卡片状态和胶囊安全布局。
- Modify: `apps/miniapp/src/pages/profile/index.tsx` — Hero 单击进入资料编辑。
- Modify: `apps/miniapp/src/pages/profile/index.scss` — 只增加点击反馈，不改变布局。
- Modify: `apps/miniapp/src/app.config.ts` — 若无引用则移除旧独立 tools 路由，保持首页工具为唯一入口。

### Task 1: 普通工具真实性过滤

**Files:**
- Modify: `apps/miniapp/src/services/d-navigation.ts`
- Create: `scripts/check-home-real-tools-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写失败的工具真实性合同**

```js
const fs=require('node:fs')
const assert=require('node:assert/strict')
const navigation=fs.readFileSync('apps/miniapp/src/services/d-navigation.ts','utf8')
assert.match(navigation,/legacy-missing-appid/)
assert.match(navigation,/validMiniProgramAppID/)
assert.match(navigation,/validMiniProgramPath/)
assert.match(navigation,/filter\(isRealTool\)/)
console.log('home real tools contract: ok')
```

- [ ] **Step 2: 运行合同确认当前未过滤占位项**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: FAIL，首个缺失断言指向 `legacy-missing-appid`。

- [ ] **Step 3: 实现服务层过滤**

```ts
const placeholderAppIDs=new Set(['legacy-missing-appid','wx0000000000000000'])
const miniProgramAppIDPattern=/^wx[a-zA-Z0-9]{16}$/
const miniProgramPathPattern=/^(?!\/)(?!.*(?:^|\/)\.\.(?:\/|$))(?!.*\/\/)[a-zA-Z0-9_\-/]+(?:\?[a-zA-Z0-9_%=&.\-]+)?$/

export function validMiniProgramAppID(value:string):boolean{
	const normalized=value.trim()
	return miniProgramAppIDPattern.test(normalized)&&!placeholderAppIDs.has(normalized)
}

export function validMiniProgramPath(value:string):boolean{
	return miniProgramPathPattern.test(value.trim())
}

function isRealTool(item:BackendItem):boolean{
	return validMiniProgramAppID(item.appId)&&validMiniProgramPath(item.path)
}

export async function listNavigationItems():Promise<ToolLink[]>{
	const categories=await listNavigationCategories()
	return categories
		.filter((category)=>category.systemKey!==externalTakeoutSystemKey)
		.flatMap((category)=>category.items.filter(isRealTool).map((item)=>({
			id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,
			appId:item.appId.trim(),path:item.path.trim(),color:item.color
		})))
}
```

不允许在 Home 或 tools 页面再次构造静态项目数组。

- [ ] **Step 4: 运行合同和类型检查**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: `home real tools contract: ok`。

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交过滤器**

```powershell
git add -- apps/miniapp/src/services/d-navigation.ts scripts/check-home-real-tools-contract.cjs package.json
git commit -m "修复：过滤虚拟校园工具入口"
```

### Task 2: 胶囊安全区计算

**Files:**
- Create: `apps/miniapp/src/pages/home/header-layout.ts`
- Modify: `apps/miniapp/src/services/taro.ts`
- Modify: `apps/miniapp/src/taro-shim.d.ts`
- Modify: `scripts/check-home-real-tools-contract.cjs`

- [ ] **Step 1: 扩展失败合同，要求运行时胶囊 API**

```js
const home=fs.readFileSync('apps/miniapp/src/pages/home/index.tsx','utf8')
const taro=fs.readFileSync('apps/miniapp/src/services/taro.ts','utf8')
assert.match(home,/getMenuButtonBoundingClientRect/)
assert.match(home,/getWindowInfo/)
assert.match(home,/--home-header-right/)
assert.match(taro,/getMenuButtonBoundingClientRect/)
```

- [ ] **Step 2: 运行合同确认当前固定布局失败**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: FAIL，错误指向 `getMenuButtonBoundingClientRect`。

- [ ] **Step 3: 增加 Taro 类型和纯布局函数**

在 `apps/miniapp/src/services/taro.ts` 导出运行时类型：

```ts
export interface MenuButtonRect{top:number;right:number;bottom:number;left:number;width:number;height:number}
export interface WindowInfo{windowWidth:number;statusBarHeight:number;pixelRatio:number;safeArea?:{top:number;right:number;bottom:number;left:number;width:number;height:number}}
```

在 `header-layout.ts` 从服务层导入类型并导出纯计算函数：

```ts
import type {MenuButtonRect,WindowInfo} from '../../services/taro'

export interface HomeHeaderLayout{top:number;minHeight:number;right:number}

export function calculateHomeHeaderLayout(windowInfo:WindowInfo,menu?:MenuButtonRect):HomeHeaderLayout{
	const fallbackTop=Math.max(20,windowInfo.statusBarHeight||20)
	if(!menu||menu.width<=0||menu.height<=0||menu.left<=0){
		return{top:fallbackTop,minHeight:fallbackTop+44,right:96}
	}
	const capsuleGap=Math.max(8,windowInfo.windowWidth-menu.right)
	return{
		top:Math.max(fallbackTop,menu.top),
		minHeight:Math.max(menu.bottom+8,fallbackTop+44),
		right:Math.max(96,windowInfo.windowWidth-menu.left+capsuleGap)
	}
}
```

在 `TaroFacade` 和 `TaroStatic` 增加：

```ts
getMenuButtonBoundingClientRect():MenuButtonRect
getWindowInfo():WindowInfo
```

- [ ] **Step 4: 在 Home 注入 CSS 变量**

```tsx
const [headerLayout]=useState(()=>{
  try{return calculateHomeHeaderLayout(Taro.getWindowInfo(),Taro.getMenuButtonBoundingClientRect())}
  catch{return{top:20,minHeight:64,right:96}}
})
const headerStyle={
  '--home-header-top':`${headerLayout.top}px`,
  '--home-header-height':`${headerLayout.minHeight}px`,
  '--home-header-right':`${headerLayout.right}px`
} as React.CSSProperties
```

`.student-home-header` 使用这些变量控制 `padding-top`、`min-height`、`padding-right`；不得继续使用 `112rpx 36rpx 0` 固定右侧布局。

- [ ] **Step 5: 运行类型检查并提交**

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

```powershell
git add -- apps/miniapp/src/pages/home/header-layout.ts apps/miniapp/src/services/taro.ts apps/miniapp/src/taro-shim.d.ts apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss scripts/check-home-real-tools-contract.cjs
git commit -m "修复：首页避开微信胶囊按钮"
```

### Task 3: 首页“工具”第三页签与三态反馈

**Files:**
- Modify: `apps/miniapp/src/pages/home/index.tsx`
- Modify: `apps/miniapp/src/pages/home/index.scss`
- Modify: `scripts/check-home-real-tools-contract.cjs`

- [ ] **Step 1: 扩展失败合同**

```js
assert.match(home,/type HomeTab = 'messages' \| 'discover' \| 'tools'/)
assert.match(home,/key: 'tools', label: '工具'/)
assert.match(home,/暂无可用工具/)
assert.match(home,/工具加载失败，点击重试/)
assert.match(home,/navigateToMiniProgram/)
assert.doesNotMatch(home,/特价影票|学生兼职|生活服务|校园群聊|校园地图/)
```

- [ ] **Step 2: 运行合同确认第三页签缺失**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: FAIL，错误指向 `HomeTab` 未包含 `tools`。

- [ ] **Step 3: 实现工具加载状态与真实跳转**

```ts
type ToolStatus='idle'|'loading'|'ready'|'empty'|'error'
const [tools,setTools]=useState<ToolLink[]>([])
const [toolStatus,setToolStatus]=useState<ToolStatus>('idle')

const loadTools=useCallback(async()=>{
  setToolStatus('loading')
  try{
    const values=await listNavigationItems()
    if(!mountedRef.current)return
    setTools(values);setToolStatus(values.length?'ready':'empty')
  }catch{
    if(!mountedRef.current)return
    setTools([]);setToolStatus('error')
  }
},[])

const openTool=async(tool:ToolLink)=>{
  if(!tool.appId||!tool.path)return
  try{await Taro.navigateToMiniProgram({appId:tool.appId,path:tool.path})}
  catch(error){await Taro.showToast({title:error instanceof Error?error.message:'工具打开失败',icon:'none'})}
}
```

首次切到 `tools` 且状态为 `idle` 时加载；失败状态可点击重试；空状态不创建卡片。工具卡只显示 API 返回的 title、subtitle、iconText 和 color。

- [ ] **Step 4: 修复发现页哨兵创建时机**

仅在 `activeTab==='discover' && initialStatus==='ready'` 且渲染完成后创建 observer。使用下一微任务或 `Taro.nextTick`（需要补类型）后确认 `#home-discover-sentinel` 存在，避免截图中的 `Node is not found` 警告；切走页签立即 disconnect。

- [ ] **Step 5: 运行合同、类型检查、构建并提交**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: `home real tools contract: ok`。

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

Run: `npm.cmd --prefix apps/miniapp run build:weapp`

Expected: PASS。

```powershell
git add -- apps/miniapp/src/pages/home/index.tsx apps/miniapp/src/pages/home/index.scss apps/miniapp/src/services/taro.ts apps/miniapp/src/taro-shim.d.ts scripts/check-home-real-tools-contract.cjs
git commit -m "功能：首页增加真实工具页签"
```

### Task 4: “我的”上半区单次进入资料编辑

**Files:**
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/profile/index.scss`
- Modify: `scripts/check-home-real-tools-contract.cjs`

- [ ] **Step 1: 扩展失败合同**

```js
const profile=fs.readFileSync('apps/miniapp/src/pages/profile/index.tsx','utf8')
assert.match(profile,/student-profile-hero[^\n]*onClick=\{openProfileEditor\}/)
assert.match(profile,/const openProfileEditor = \(\) =>/)
assert.doesNotMatch(profile,/student-profile-user__edit' onClick=/)
```

- [ ] **Step 2: 运行合同确认 Hero 当前不可点击**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: FAIL，错误指向 Hero 缺少 `onClick`。

- [ ] **Step 3: 统一为单一导航处理器**

```tsx
const openProfileEditor=()=>{
  void Taro.navigateTo({url:'/pages/edit-profile/index'})
}

<View className='student-profile-hero' onClick={openProfileEditor}>
  <View className='student-profile-hero__orb student-profile-hero__orb--small' />
  <View className='student-profile-hero__orb student-profile-hero__orb--large' />
  <View className='student-profile-user'>
    <View className='student-profile-user__avatar'>
      <Text>{profile.nickname.trim().slice(0, 1) || '同'}</Text>
    </View>
    <View className='student-profile-user__main'>
      <View className='student-profile-user__title-row'>
        <Text className='student-profile-user__name'>{profile.nickname || '校园用户'}</Text>
        <Text className='student-profile-user__badge'>{roleName}</Text>
      </View>
      <Text className='student-profile-user__meta'>
        用户号：{profile.id || '--'} · {profile.campus || profile.school || '校园用户'}
      </Text>
    </View>
    <View className='student-profile-user__edit'>
      <Text>编辑资料</Text>
    </View>
  </View>
</View>
```

SCSS 仅增加按下态透明度或 `cursor`，不改变尺寸、色彩和层级。

- [ ] **Step 4: 运行合同与类型检查**

Run: `node scripts/check-home-real-tools-contract.cjs`

Expected: PASS。

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交资料入口**

```powershell
git add -- apps/miniapp/src/pages/profile/index.tsx apps/miniapp/src/pages/profile/index.scss scripts/check-home-real-tools-contract.cjs
git commit -m "功能：支持点击个人资料区编辑"
```

### Task 5: 清理旧工具入口并完成验收

**Files:**
- Modify: `apps/miniapp/src/app.config.ts`
- Delete only if unreferenced: `apps/miniapp/src/pages/tools/index.tsx`
- Delete only if unreferenced: `apps/miniapp/src/pages/tools/index.scss`
- Create: `docs/HOME-TOOLS-CAPSULE-ACCEPTANCE-REPORT.md`

- [ ] **Step 1: 检查旧工具页面引用**

Run: `rg -n "pages/tools|ToolsPage|active='tools'" apps/miniapp/src scripts`

Expected: 除旧页面本身外没有入口；若仍有真实调用，保留文件但从 `app.config.ts` 移除路由前先改调用到首页 `tools` 页签。

- [ ] **Step 2: 移除无引用旧路由和旧页面**

从 `app.config.ts` 删除 `pages/tools/index`。只有 Step 1 证明无引用后才删除两个旧页面文件；不删除共享 `BottomNav` 或其他旧页面。

- [ ] **Step 3: 运行完整静态验证**

Run: `npm.cmd run check:miniapp-static-routes`

Expected: PASS。

Run: `npm.cmd run check:miniapp-business-closure`

Expected: PASS，首页没有静态假工具。

Run: `npm.cmd run typecheck && npm.cmd run build:miniapp`

Expected: PASS。

- [ ] **Step 4: 开发者工具人工验收**

验证 iPhone 12/13、一个窄屏 Android 和一个带不同状态栏高度的设备：顶部搜索与胶囊不重叠；三页签均可点击；后台只有占位数据时工具显示空状态；配置真实 AppID/path 后只显示真实工具；Hero 任意上半区点击只进入一次编辑页。

- [ ] **Step 5: 记录并提交验收结果**

报告逐项写 PASS/FAIL/PENDING。真机未执行时胶囊真机项保持 `PENDING`。

```powershell
git add -- apps/miniapp/src/app.config.ts apps/miniapp/src/pages/tools docs/HOME-TOOLS-CAPSULE-ACCEPTANCE-REPORT.md
git commit -m "文档：记录首页工具与胶囊验收结果"
```
