const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (file) => fs.readFileSync(path.join(root, file), 'utf8')
const page = read('apps/miniapp/src/pages/my-content/index.tsx')
const hook = read('apps/miniapp/src/pages/my-content/use-content-tabs.ts')
const style = read('apps/miniapp/src/pages/my-content/index.scss')
const pager = read('apps/miniapp/src/components/channel-pager.tsx')
const pagerStyle = read('apps/miniapp/src/components/channel-pager.scss')

function has(source, pattern, message) {
  assert.match(source, pattern, message)
}

// The page must use the same native pager as the home channels: this keeps
// horizontal touch handling native and avoids replacing content after release.
has(page, /ChannelPager/, '我的内容必须使用原生 ChannelPager')
has(page, /keys=\{visibleTabs\.map\(tab => tab\.key\)\}/, '分页器只接收当前开放栏目')
has(page, /renderPage=\{renderPage\}/, '分页器必须按栏目渲染页面')
has(page, /useChannelInteraction/, '栏目点击必须共享滑动点击抑制状态')
has(page, /if \(!interaction\.canActivate\(\) \|\| navigating\.current\) return/, '横滑后点击导航必须短暂抑制')
has(page, /navigateBackOrHome/, '返回必须保留分享直达的首页兜底')

// Each page owns a scroll view and delegates loading to the tab key. The hook
// rejects hidden/non-active requests, so neighbouring SwiperItems cannot fetch.
assert.ok((page.match(/className=\{`my-content-body/g) || []).length >= 1, '每个栏目必须拥有独立滚动容器')
has(page, /rememberScrollFor\(tab, event\.detail\.scrollTop\)/, '滚动位置必须按栏目保存')
has(page, /loadMoreFor\(tab\)/, '触底加载必须按栏目归属')
has(hook, /rememberScrollFor/, 'hook 必须导出按栏目保存位置的方法')
has(hook, /getScrollTop/, 'hook 必须导出按栏目恢复位置的方法')
has(hook, /if \(tab === activeRef\.current\) void load\(tab, 'more'\)/, '只有当前栏目允许触底请求')
has(hook, /enabledRef\.current\.includes\(tab\)/, '隐藏栏目不得发起请求')
has(hook, /versions\.current\[tab\] \+= 1/, '栏目关闭时必须失效旧请求')

// The design contract is deliberately bounded: no autoplay/circular looping,
// a short native transition, and a reduced-motion-safe page surface.
has(pager, /duration=\{CHANNEL_DURATION_MS\}/, '分页切换必须有稳定原生时长')
has(pager, /circular=\{false\}/, '栏目分页不得循环')
has(pager, /autoplay=\{false\}/, '栏目分页不得自动轮播')
has(pager, /source !== 'touch'/, '程序化回调不得覆盖点击选择')
has(pagerStyle, /height: 0/, '分页器必须在父级 flex 中约束高度')
has(style, /\.my-content-page[\s\S]*overflow: hidden/, '页面必须裁切横滑过渡溢出')

console.log('content tab gestures: PASS')
