const fs = require('node:fs')
const path = require('node:path')

const rootDir = path.resolve(__dirname, '..')
const scriptPath = path.join(rootDir, 'scripts', 'local-secondhand-demo.cjs')
const source = fs.readFileSync(scriptPath, 'utf8')

const requiredFragments = [
  "new Set(['127.0.0.1', 'localhost', '::1'])",
  "appEnv === 'production'",
  "const DEMO_PREFIX = '[本机预览]'",
  "!['seed', 'status', 'clear'].includes(mode)",
  'START TRANSACTION',
  'COMMIT;',
  'DEMO_PRODUCTS.length',
  "category: '数码'",
  "category: '书籍资料'",
  "category: '生活用品'",
  "category: '代步'",
  "category: '其他'"
]

for (const fragment of requiredFragments) {
  if (!source.includes(fragment)) throw new Error(`本机二手演示脚本缺少安全契约：${fragment}`)
}

const productCount = (source.match(/title: `\$\{DEMO_PREFIX\}/g) || []).length
if (productCount !== 8) throw new Error(`演示商品定义数量应为 8，实际为 ${productCount}`)
if (/zhanduiwang\.top|https?:\/\//i.test(source)) throw new Error('本机二手演示脚本不得引用线上地址')
if (/INSERT\s+INTO\s+product_images/i.test(source)) throw new Error('本机二手演示脚本不得伪造商品图片')

console.log('本机二手演示数据契约检查通过')
