const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const backendRoot = path.join(root, 'backend')
const mysqlEnvironmentKeys = new Set(['MYSQL_DSN'])

function collectTestFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const absolutePath = path.join(directory, entry.name)
    if (entry.isDirectory()) return collectTestFiles(absolutePath)
    return entry.isFile() && entry.name.endsWith('_test.go') ? [absolutePath] : []
  })
}

for (const file of collectTestFiles(backendRoot)) {
  const source = fs.readFileSync(file, 'utf8')
  for (const match of source.matchAll(/\b[A-Z][A-Z0-9_]*MYSQL_DSN\b/g)) {
    mysqlEnvironmentKeys.add(match[0])
  }
}

const environment = { ...process.env }
for (const key of mysqlEnvironmentKeys) {
  // Default verification must never run destructive migration tests on a shared database.
  environment[key] = ''
}

const executable = process.platform === 'win32' ? 'go.exe' : 'go'
const result = spawnSync(executable, ['test', './...'], {
  cwd: backendRoot,
  env: environment,
  stdio: 'inherit',
})

if (result.error) {
  throw result.error
}
process.exit(result.status ?? 1)
