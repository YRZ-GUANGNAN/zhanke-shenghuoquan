const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const backendRoot = path.join(root, 'backend')
const unsafeEnvironmentKeys = new Set(['MYSQL_DSN'])

function collectGoFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolutePath = path.join(directory, entry.name)
    if (entry.isDirectory()) return collectGoFiles(absolutePath)
    return entry.isFile() && entry.name.endsWith('.go') ? [absolutePath] : []
  })
}

function isUnsafeEnvironmentKey(key) {
  return key === 'MYSQL_DSN' ||
    key.endsWith('_MYSQL_DSN') ||
    key.endsWith('_MYSQL_ALLOW_IN_PLACE') ||
    key.startsWith('RUN_REAL_') ||
    key.includes('_RUN_REAL_') ||
    /^MEDIA_.*_ENABLE_/.test(key) ||
    key.includes('_SERVE_WECHAT') ||
    key.includes('READ_RECEIPT') ||
    /(^|_)(COS|TENCENT_IM|WECHAT|WECOM|MEILI)(_|$)/.test(key)
}

for (const file of collectGoFiles(backendRoot)) {
  const source = fs.readFileSync(file, 'utf8')
  for (const match of source.matchAll(/["`]([A-Z][A-Z0-9_]{2,})["`]/g)) {
    if (isUnsafeEnvironmentKey(match[1])) unsafeEnvironmentKeys.add(match[1])
  }
}

const environment = { ...process.env }
for (const key of unsafeEnvironmentKeys) {
  // Default verification must never reach a shared database or a real external service.
  environment[key] = ''
}
environment.APP_ENV = 'dev'

const executable = process.platform === 'win32' ? 'go.exe' : 'go'
const result = spawnSync(executable, ['test', '-count=1', './...'], {
  cwd: backendRoot,
  env: environment,
  stdio: 'inherit',
})

if (result.error) {
  throw result.error
}
process.exit(result.status ?? 1)
