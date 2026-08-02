const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const publishSource = read('apps', 'miniapp', 'src', 'pages', 'publish-entry', 'index.tsx')
const publishStyle = read('apps', 'miniapp', 'src', 'pages', 'publish-entry', 'index.scss')
const takeoutSource = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.tsx')
const takeoutStyle = read('apps', 'miniapp', 'src', 'pages', 'student-takeout', 'index.scss')

function assertIncludes(source, value) {
  if (!source.includes(value)) throw new Error(`publish/takeout UI contract is missing: ${value}`)
}

function assertExcludes(source, value) {
  if (source.includes(value)) throw new Error(`publish/takeout UI contract contains forbidden value: ${value}`)
}

for (const required of [
  '/pages/publish-post/index',
  '/pages/publish-secondhand/index',
  '想发布什么？',
  '选择一种内容类型开始发布',
  '发帖子',
  '卖闲置',
  "icon: 'comment'",
  "icon: 'market'",
]) assertIncludes(publishSource, required)

for (const forbidden of [
  'publish-entry-hero__orb',
  'publish-entry-note',
  "order: '01'",
  "order: '02'",
  'publish-entry-card__index',
  'publish-entry-card__action',
  '去发布',
  '把内容发到正确的频道',
  'linear-gradient',
  '$gradient-primary',
]) {
  assertExcludes(`${publishSource}\n${publishStyle}`, forbidden)
}

for (const required of [
  "type MerchantFilter = 'all' | 'open' | 'resting'",
  'listExternalTakeoutMerchants',
  'navigateToMiniProgram',
  '全部',
  '营业中',
  '休息中',
  'filteredMerchants',
  '进入商家',
]) assertIncludes(takeoutSource, required)

for (const forbidden of [
  '评分',
  '月售',
  '配送时间',
  '预计送达',
  '起送价',
  '优惠券',
]) assertExcludes(takeoutSource, forbidden)

assertExcludes(takeoutStyle, '.student-bottom-nav')

console.log('publish and takeout UI contract PASS')
