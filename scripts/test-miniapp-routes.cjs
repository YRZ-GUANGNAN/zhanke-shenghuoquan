const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { readMiniappRoutes } = require('./miniapp-routes.cjs')

assert.deepEqual(readMiniappRoutes(`export default defineAppConfig({
  // 'pages/comment-only/index' must never register a page.
  pages: ["pages/home/index", 'pages/help/detail'],
  subPackages: [
    { root: 'wallet', pages: ['index', 'history/detail'] },
    { root: 'account', pages: ['profile'] },
  ],
  window: { navigationBarTitleText: 'pages/title/index' },
})`), ['pages/home/index', 'pages/help/detail', 'wallet/index', 'wallet/history/detail', 'account/profile'])
assert.deepEqual(readMiniappRoutes("export default defineAppConfig({pages:['home'],subpackages:[{root:'secondary',pages:['detail']}]})"), ['home', 'secondary/detail'])
assert.deepEqual(readMiniappRoutes("export default defineAppConfig({pages:['home']})"), ['home'])
for (const value of [
  "{pages: dynamicPages}", "{pages: ['home', ...dynamicPages]}",
  "{pages:['home'],subPackages:[{root: getRoot(),pages:['index']}]}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:routes}]}",
  "{pages:['home'],subPackages:[{root:'../wallet',pages:['index']}]}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:['../index']}]}",
  "{pages:['/home']}", "{pages:['a//b']}", "{pages:['a/./b']}",
  "{pages:['home','home']}", "{pages:[]}", "{pages:['home'],pages:['other']}",
  "{pages:['home'],['pages']:['hidden']}",
  "{pages:['home'],['subPackages']:[{root:'wallet',pages:['index']}]}",
  "{pages:['home'],[key]:['hidden']}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:['index'],['pages']:['hidden']}]}",
  "{pages:['home'],subPackages:[{root:'wallet',['root']:'other',pages:['index']}]}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:[]}]}",
  "{pages:['home'],subPackages:[],subpackages:[]}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:['index']},{root:'wallet',pages:['detail']}]}",
  "{pages:['home'],subPackages:[{root:'wallet',pages:['index']},{root:'wallet/nested',pages:['detail']}]}",
  "{pages:['wallet/index'],subPackages:[{root:'wallet',pages:['history']}]}",
]) assert.throws(() => readMiniappRoutes(`export default defineAppConfig(${value})`), value)
assert.throws(() => readMiniappRoutes('export default arbitraryCode()'))
assert.throws(() => readMiniappRoutes('export default defineAppConfig({pages: [invalid syntax)'))
const root = path.resolve(__dirname, '..')
const actual = readMiniappRoutes(fs.readFileSync(path.join(root, 'apps/miniapp/src/app.config.ts'), 'utf8'))
assert.equal(actual.length, 33)
for (const route of ['pages/merchant-wallet/index', 'pages/rider-wallet/index']) {
  assert.ok(actual.includes(route), `must inspect sub-package page: ${route}`)
}
for (const file of ['check-miniapp-dark-theme.cjs', 'generate-miniapp-dark-theme.cjs']) {
  const source = fs.readFileSync(path.join(__dirname, file), 'utf8')
  assert.match(source, /readMiniappRoutes\(/, `${file} must inspect all parsed routes`)
  assert.doesNotMatch(source, /pages\.filter\(/, `${file} must not silently exclude sub-packages`)
}
console.log('PASS 主包/多页分包静态解析、注释隔离、非法配置拒绝；两个钱包参与主题验收')
