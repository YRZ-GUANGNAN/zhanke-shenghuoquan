const { spawn } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')

const rootDir = path.resolve(__dirname, '..')
const backendDir = path.join(rootDir, 'backend')
const envFiles = [
  path.join(backendDir, '.env'),
  path.join(backendDir, '.env.example'),
  path.join(rootDir, '.env'),
  path.join(rootDir, '.env.example')
]

function loadEnvValue(key) {
  if (process.env[key]) return process.env[key]

  for (const file of envFiles) {
    if (!fs.existsSync(file)) continue
    const content = fs.readFileSync(file, 'utf8')
    for (const line of content.split(/\r?\n/)) {
      const trimmed = line.trim()
      if (!trimmed || trimmed.startsWith('#')) continue
      const index = trimmed.indexOf('=')
      if (index === -1) continue
      const envKey = trimmed.slice(0, index).trim()
      if (envKey === key) return trimmed.slice(index + 1).trim()
    }
  }

  return ''
}

function parseMysqlDsn(dsn) {
  const match = dsn.match(/^([^:@/]+):([^@]*)@tcp\(([^:)]+):(\d+)\)\/([^?]+)/)
  if (!match) {
    throw new Error(`Unsupported MYSQL_DSN format: ${dsn || '<empty>'}`)
  }
  return {
    user: match[1],
    password: match[2],
    host: match[3],
    port: match[4],
    database: match[5]
  }
}

function mysqlBinary() {
  const candidates = [
    process.env.MYSQL_BIN,
    String.raw`C:\Program Files\MySQL\MySQL Server 8.4\bin\mysql.exe`,
    String.raw`C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe`,
    'mysql'
  ].filter(Boolean)

  return candidates.find((candidate) => candidate === 'mysql' || fs.existsSync(candidate))
}

function runMysqlFile(mysqlBin, connection, file) {
  return new Promise((resolve, reject) => {
    const child = spawn(mysqlBin, [
      `--user=${connection.user}`,
      `--host=${connection.host}`,
      '--protocol=TCP',
      `--port=${connection.port}`,
      '--default-character-set=utf8mb4',
      `--database=${connection.database}`
    ], {
      env: {
        ...process.env,
        MYSQL_PWD: connection.password
      },
      stdio: ['pipe', 'pipe', 'pipe']
    })

    let stdout = ''
    let stderr = ''
    child.stdout.on('data', (chunk) => {
      stdout += chunk
    })
    child.stderr.on('data', (chunk) => {
      stderr += chunk
    })
    child.on('error', reject)
    child.on('close', (code) => {
      if (code === 0) {
        if (stdout.trim()) process.stdout.write(stdout)
        resolve()
      } else {
        reject(new Error(`${path.relative(rootDir, file)} failed with code ${code}\n${stderr}`))
      }
    })

    child.stdin.end(fs.readFileSync(file))
  })
}

async function main() {
  const dsn = loadEnvValue('MYSQL_DSN')
  const connection = parseMysqlDsn(dsn)
  const mysqlBin = mysqlBinary()
  if (!mysqlBin) {
    throw new Error('mysql client not found. Set MYSQL_BIN to mysql.exe if it is installed in a custom path.')
  }

  const migrationsDir = path.join(backendDir, 'migrations')
  const migrations = fs.readdirSync(migrationsDir)
    .filter((name) => /^\d+_.*\.sql$/.test(name))
    .sort()
    .map((name) => path.join(migrationsDir, name))

  for (const file of migrations) {
    console.log(`Applying ${path.relative(rootDir, file)}`)
    await runMysqlFile(mysqlBin, connection, file)
  }

  console.log(`Seed complete: ${connection.user}@${connection.host}:${connection.port}/${connection.database}`)
}

main().catch((error) => {
  console.error(error.message)
  process.exitCode = 1
})
