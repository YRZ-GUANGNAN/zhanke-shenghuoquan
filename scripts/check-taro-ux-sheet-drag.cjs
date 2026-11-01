const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const Module = require('node:module')
const ts = require('typescript')
const source = path.resolve(__dirname, '../apps/miniapp/src/utils/taro-ux-sheet-drag.ts')
const loaded = new Module(source, module)
loaded._compile(ts.transpileModule(fs.readFileSync(source, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText, source)
const { createTaroUxSheetDrag } = loaded.exports
let clock = 1000
const tracker = createTaroUxSheetDrag(() => clock)
const point = (x, y, identifier = 1) => ({ identifier, pageX: x, pageY: y, clientX: x, clientY: y })
const start = () => { clock = 1000; tracker.start([point(0, 0)]) }

// This is the unmodified closure criterion/getDiffY from upstream Modal.jsx
// f85a5d714639006a9c3f12c67838087a30255c0b, independent of the port under test.
function upstream(x, y, elapsed) {
  if (y <= 0) return false
  const angle = Math.atan2(y, x) * (180 / Math.PI)
  if (angle < 45 || angle > 135) return false
  return y > 200 || (y > 10 && elapsed < 200)
}
for (const x of [-201, -200, -100, -10, 0, 10, 100, 200, 201]) {
  for (const y of [-10, 0, 10, 11, 100, 200, 201, 500]) {
    for (const elapsed of [0, 199, 200, 500]) {
      start(); clock += elapsed
      assert.equal(tracker.end([point(x, y)], []), upstream(x, y, elapsed), `原算法边界 x=${x} y=${y} ms=${elapsed}`)
    }
  }
}
start()
assert.equal(tracker.move([point(0, 500)]).offset, 500, '无0.72阻尼或180px上限')
tracker.move([point(0, 5)])
assert.equal(tracker.end([point(0, 5)], []), false, '拖回起点不能误关')
start(); tracker.move([point(0, 250)])
assert.equal(tracker.move([point(400, 250)]).offset, 250, '偏离角度途中保持上游上次正向位移')
assert.equal(tracker.end([point(400, 250)], []), false, '偏离角度抬手只复位，不使用旧位移误关')
start(); tracker.move([point(80, 10)]); clock += 50
assert.equal(tracker.end([point(80, 200)], []), false, '保留应用横向手势隔离')
start(); tracker.move([point(0, 200), point(0, 201, 2)])
assert.equal(tracker.end([point(0, 300)], []), false, '多指取消整次手势')
start(); tracker.move([point(0, 200, 2)])
assert.equal(tracker.end([point(0, 300, 2)], []), false, '触点标识变化取消')
start(); tracker.move([point(NaN, 201)])
assert.equal(tracker.end([point(0, 300)], []), false, '非法坐标取消')
start(); tracker.cancel()
assert.equal(tracker.end([point(0, 300)], []), false, '取消后旧抬手不复活')
class NativeTouch {
  get identifier() { return 1 }
  get pageX() { return 25 }
  get pageY() { return 30 }
  get clientX() { return 25 }
  get clientY() { return 30 }
}
tracker.start([new NativeTouch()])
assert.equal(tracker.move([point(25, 61)]).offset, 31, '读取原生Touch原型getter，不spread')
console.log('taro ux sheet drag: PASS (288 upstream boundary comparisons, full displacement, rebound, direction, multitouch, identity, cancellation, prototype Touch)')
