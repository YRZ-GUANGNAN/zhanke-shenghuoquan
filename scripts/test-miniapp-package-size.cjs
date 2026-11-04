const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { analyzePackage, formatReport } = require('./check-miniapp-package-size.cjs')

function fixture(manifest, files = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'miniapp-package-'))
  fs.writeFileSync(path.join(dir, 'app.json'), JSON.stringify(manifest))
  for (const [name, value] of Object.entries(files)) {
    const file = path.join(dir, name)
    fs.mkdirSync(path.dirname(file), { recursive: true })
    fs.writeFileSync(file, typeof value === 'number' ? Buffer.alloc(value) : value)
  }
  return dir
}
function cleanup(dir) { fs.rmSync(dir, { recursive: true, force: true }) }
function pageFiles(root, page = 'index', bytes = 1) { return { [`${root}/${page}.js`]: Buffer.alloc(bytes), [`${root}/${page}.json`]: '{}', [`${root}/${page}.wxml`]: '' } }
function expectFailure(fn, pattern) { assert.throws(fn, error => pattern.test(error.message), pattern) }

const base = { pages: ['pages/home/index'], subPackages: [{ root: 'pages/alpha', pages: ['index'] }, { root: 'pages/alpha-other', pages: ['index'] }, { root: 'pages/renamed', pages: ['entry/home'] }] }
const files = { 'app.js': Buffer.alloc(1), 'common.js': Buffer.alloc(1), 'vendors.js': Buffer.alloc(1), ...pageFiles('pages/home'), ...pageFiles('pages/alpha'), ...pageFiles('pages/alpha-other'), ...pageFiles('pages/renamed', 'entry/home') }
let dir = fixture(base, files)
try {
  const report = analyzePackage(dir)
  assert.equal(report.packages.length, 4)
  assert.match(formatReport(report), /main=/)
  assert.equal(report.packages.find(pkg => pkg.name === 'pages/alpha').bytes, 3)
  console.log('PASS 改名、多分包、主包/分包边界与逐包统计')
} finally { cleanup(dir) }

dir = fixture({ pages: ['pages/home/index'] }, { 'app.js': Buffer.alloc(1), 'common.js': Buffer.alloc(1), 'vendors.js': Buffer.alloc(1), ...pageFiles('pages/home') })
try { assert.equal(analyzePackage(dir).packages.length, 1); console.log('PASS 无分包时只统计主包') } finally { cleanup(dir) }

for (const missing of ['app.js', 'common.js', 'vendors.js', 'pages/home/index.wxml']) {
  const missingFiles = { ...files }; delete missingFiles[missing]
  dir = fixture(base, missingFiles)
  try { expectFailure(() => analyzePackage(dir), /required build output|missing page output/) } finally { cleanup(dir) }
}
console.log('PASS 必需运行时文件与页面 WXML 缺失检查')

dir = fixture({ pages: ['pages/home/index'], subPackages: [{ root: 'pages/alpha', pages: ['index'] }] }, { ...files, ...pageFiles('pages/alpha', 'extra', 2 * 1024 * 1024) })
try { expectFailure(() => formatReport(analyzePackage(dir)), /pages\/alpha=/); console.log('PASS 分包超过 2MiB 拒绝') } finally { cleanup(dir) }

dir = fixture({ pages: ['pages/home/index'] }, { ...files, 'large.js': Buffer.alloc(2 * 1024 * 1024) })
try { expectFailure(() => formatReport(analyzePackage(dir)), /main=/); console.log('PASS 主包超过 2MiB 拒绝') } finally { cleanup(dir) }

dir = fixture({ pages: ['pages/home/index'] }, files)
try {
  const report = analyzePackage(dir)
  const main = report.packages.find(pkg => pkg.name === 'main')
  main.bytes = 2 * 1024 * 1024 - 64 * 1024
  assert.match(formatReport(report), /main=/)
  main.bytes++
  expectFailure(() => formatReport(report), /main=.*headroom/)
  console.log('PASS 主包保留64KiB微信编译空间，临界值通过，超出即拒绝')
} finally { cleanup(dir) }

dir = fixture({ pages: ['pages/home/index'] }, files)
try { const report = analyzePackage(dir); report.totalBytes = 20 * 1024 * 1024 + 1; expectFailure(() => formatReport(report), /total=/); console.log('PASS 总包上限拒绝') } finally { cleanup(dir) }

for (const manifest of [
  { pages: ['pages/home/index'], subPackages: [{ root: '../escape', pages: ['index'] }] },
  { pages: ['pages/home/index'], subPackages: [{ root: '/absolute', pages: ['index'] }] },
  { pages: ['pages/home/index'], subPackages: [{ root: 'pages/a', pages: ['index'] }, { root: 'pages/a', pages: ['index'] }] },
  { pages: ['pages/home/index'], subPackages: [{ root: 'pages/a', pages: ['index'] }, { root: 'pages/a/nested', pages: ['index'] }] },
  { pages: ['pages/home/index'], subPackages: [{ root: 'pages/missing', pages: ['index'] }] },
]) {
  dir = fixture(manifest, files)
  try { expectFailure(() => analyzePackage(dir), /absolute|escape|contain|unique|overlap|missing page output/) } finally { cleanup(dir) }
}
console.log('PASS 非法 root、重复/重叠 root、缺页产物检查')
