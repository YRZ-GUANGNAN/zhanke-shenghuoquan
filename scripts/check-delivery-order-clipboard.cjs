const assert = require('node:assert/strict')
const fs = require('node:fs')
const Module = require('node:module')
const path = require('node:path')
const ts = require('typescript')

const projectRoot = path.resolve(__dirname, '..')
const modulePath = path.join(projectRoot, 'apps', 'miniapp', 'src', 'utils', 'delivery-order-clipboard.ts')
const source = fs.readFileSync(modulePath, 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2020,
    strict: true,
  },
  reportDiagnostics: true,
  fileName: modulePath,
})

const errors = (compiled.diagnostics ?? []).filter((diagnostic) => diagnostic.category === ts.DiagnosticCategory.Error)
assert.equal(errors.length, 0, `剪贴板解析器转译失败：${errors.map((item) => item.messageText).join('; ')}`)

const loadedModule = new Module(modulePath, module)
loadedModule.filename = modulePath
loadedModule.paths = Module._nodeModulePaths(path.dirname(modulePath))
loadedModule._compile(compiled.outputText, modulePath)

const { parseDeliveryOrderClipboard } = loadedModule.exports

const publishPagePath = path.join(projectRoot, 'apps', 'miniapp', 'src', 'pages', 'merchant-publish', 'index.tsx')
const taroFacadePath = path.join(projectRoot, 'apps', 'miniapp', 'src', 'services', 'taro.ts')
const publishPageSource = fs.readFileSync(publishPagePath, 'utf8')
const taroFacadeSource = fs.readFileSync(taroFacadePath, 'utf8')
assert.match(taroFacadeSource, /getClipboardData\(\): Promise<\{ data: string \}>/)
assert.match(publishPageSource, /parseDeliveryOrderClipboard\(source\)/)
assert.match(publishPageSource, /title='确认导入配送信息'/)
assert.match(publishPageSource, /applyClipboardPreview/)
assert.match(publishPageSource, /deliveryClipboardTemplate/)

const standard = parseDeliveryOrderClipboard([
  '取货联系人：张店长',
  '取货电话: 13800138000',
  '取货地址：一号食堂 2 楼',
  '收货人：李同学',
  '收货电话：13900139000',
  '送货地址：3 号宿舍楼 201',
  '备注：请放前台',
].join('\n'))
assert.deepEqual(standard.values, {
  pickupContact: '张店长',
  pickupPhone: '13800138000',
  pickupAddress: '一号食堂 2 楼',
  receiverName: '李同学',
  receiverPhone: '13900139000',
  receiverAddress: '3 号宿舍楼 201',
  remark: '请放前台',
})

const sectioned = parseDeliveryOrderClipboard([
  '取货信息',
  '联系人 王师傅',
  '电话：13700137000',
  '地址：校南门',
  '便利店门口',
  '',
  '收货信息',
  '姓名：周同学',
  '电话：13600136000',
  '地址：图书馆',
  '东侧服务台',
  '说明：不要打电话',
  '到达后发消息',
].join('\r\n'))
assert.deepEqual(sectioned.values, {
  pickupContact: '王师傅',
  pickupPhone: '13700137000',
  pickupAddress: '校南门 便利店门口',
  receiverName: '周同学',
  receiverPhone: '13600136000',
  receiverAddress: '图书馆 东侧服务台',
  remark: '不要打电话\n到达后发消息',
})

const contextualPhone = parseDeliveryOrderClipboard([
  '取货信息',
  '联系电话：13500135000',
  '收货信息',
  '联系电话：13400134000',
].join('\n'))
assert.equal(contextualPhone.values.pickupPhone, '13500135000')
assert.equal(contextualPhone.values.receiverPhone, '13400134000')

const invalid = parseDeliveryOrderClipboard('13800138000\n三号宿舍楼\n随便写的一段话')
assert.deepEqual(invalid.values, {})
assert.equal(invalid.recognizedFields.length, 0)
assert.equal(invalid.ignoredLines.length, 3)

const empty = parseDeliveryOrderClipboard('  \r\n\t')
assert.deepEqual(empty.values, {})
assert.equal(empty.recognizedFields.length, 0)

console.log('PASS delivery order clipboard parser')
