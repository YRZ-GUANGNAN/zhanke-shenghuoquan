const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const page = read('apps', 'miniapp', 'src', 'pages', 'search', 'index.tsx')
const home = read('apps', 'miniapp', 'src', 'pages', 'home', 'index.tsx')
const secondhand = read('apps', 'miniapp', 'src', 'pages', 'student-secondhand', 'index.tsx')
const community = read('apps', 'miniapp', 'src', 'services', 'b-community.ts')
const trade = read('apps', 'miniapp', 'src', 'services', 'c-trade.ts')
const navigation = read('apps', 'miniapp', 'src', 'services', 'd-navigation.ts')

if (!home.includes('/pages/search/index?scope=community')) {
  throw new Error('home search must use community scope')
}
if (!secondhand.includes('/pages/search/index?scope=secondhand')) {
  throw new Error('secondhand search must use secondhand scope')
}
for (const scope of ["'community'", "'secondhand'", "'takeout'"]) {
  if (!page.includes(scope)) throw new Error(`search page is missing scope ${scope}`)
}
if (!page.includes('搜索入口无效') || !page.includes('isSearchScope')) {
  throw new Error('invalid or missing scope must fail closed')
}
for (const forbidden of ['historyKeywords', 'popularKeywords', '搜帖子、二手好物', '静态预览']) {
  if (page.includes(forbidden)) throw new Error(`search page contains static/global behavior: ${forbidden}`)
}
for (const [source, endpoint] of [
  [community, '/community/searches'],
  [trade, '/trade/searches'],
  [navigation, '/navigation/external-takeout/searches']
]) {
  if (!source.includes(endpoint)) throw new Error(`domain search service is missing ${endpoint}`)
}

console.log('miniapp domain search check PASS')
