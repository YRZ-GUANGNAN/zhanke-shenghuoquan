const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8')

const home = read('apps/miniapp/src/pages/home/index.tsx')
const search = read('apps/miniapp/src/pages/search/index.tsx')
const nav = read('apps/miniapp/src/components/student-bottom-nav.tsx')

function mustInclude(source, value, message) {
  if (!source.includes(value)) throw new Error(message)
}

function mustExclude(source, pattern, message) {
  if (pattern.test(source)) throw new Error(message)
}

mustInclude(home, "className='student-home-search-row'", '首页缺少整行搜索区域')
mustInclude(home, "className='student-home-search'", '首页缺少内容型搜索框')
mustInclude(home, '/pages/search/index?scope=community', '首页搜索必须进入帖子领域')
mustInclude(home, 'student-post-card--text', '首页纯文字帖子必须使用文字卡')
mustExclude(home, /student-home-(?:banner|advert|ad-slot|promo)/i, '首页不得包含广告式占位')

for (const scope of ['community', 'secondhand', 'takeout']) {
  mustInclude(search, `${scope}:`, `搜索页缺少 ${scope} 分域配置`)
}
mustInclude(search, "type SearchScope = 'community' | 'secondhand' | 'takeout'", '搜索领域不得合并')

for (const route of [
  '/pages/home/index',
  '/pages/student-takeout/index',
  '/pages/publish-entry/index',
  '/pages/student-secondhand/index',
  '/pages/profile/index',
]) {
  mustInclude(nav, route, `学生端导航缺少 ${route}`)
}

console.log('miniapp content UI contract ok')
