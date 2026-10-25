const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const dist = path.join(root, 'apps/miniapp/dist')
const limit = 2 * 1024 * 1024

function filesIn(directory) {
  if (!fs.existsSync(directory)) return []
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const file = path.join(directory, entry.name)
    return entry.isDirectory() ? filesIn(file) : [file]
  })
}

const files = filesIn(dist)
const total = files.reduce((sum, file) => sum + fs.statSync(file).size, 0)
if (total > limit) {
  console.error(`miniapp package: FAIL (${total} bytes > ${limit} byte limit)`)
  process.exitCode = 1
} else {
  console.log(`miniapp package: PASS (${total} bytes; ${limit - total} bytes remaining)`)
}
