const fs = require('node:fs')
const net = require('node:net')
const os = require('node:os')
const path = require('node:path')
const { spawn, spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const backend = path.join(root, 'backend')
const mysqlBin = 'C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin'
const engine = path.join(mysqlBin, 'mysqld.exe')
const client = path.join(mysqlBin, 'mysql.exe')
const admin = path.join(mysqlBin, 'mysqladmin.exe')
const port = 33329
const connection = [
  '--no-defaults',
  '--no-login-paths',
  '--protocol=tcp',
  '--host=127.0.0.1',
  `--port=${port}`,
  '--user=root',
  '--connect-timeout=2',
  '--batch',
  '--skip-column-names',
  '--default-character-set=utf8mb4',
]
const expectedTests = [
  'TestD2DeliveryRealHTTPMySQLAuthConcurrencyOutboxAndGates',
  'TestD3OrderFundsReadOnlyRealHTTPMySQL',
  'TestD3DeliveryPaymentRealHTTPMySQLAuthAndGate',
  'TestD4PrivateProofParticipantFenceAgainstRealHTTPMySQL84',
  'TestUI21AIdentityMigrationApplyRollbackApply',
  'TestUI21AIdentityRealHTTPMySQL',
]

const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

async function reservePort() {
  await new Promise((resolve, reject) => {
    const probe = net.createServer()
    probe.once('error', reject)
    probe.listen(port, '127.0.0.1', () => probe.close(resolve))
  })
}

function createEnvironment(tempRoot) {
  const environment = { ...process.env }
  for (const key of Object.keys(environment)) {
    if (/MYSQL|DSN|DATABASE_URL/i.test(key)) delete environment[key]
  }
  environment.TEMP = process.env.CAMPUS_TEST_TMPDIR || tempRoot
  environment.TMP = environment.TEMP
  environment.GOTMPDIR = path.join(environment.TEMP, 'go-tmp')
  environment.GOCACHE = path.join(environment.TEMP, 'go-cache')
  fs.mkdirSync(environment.GOTMPDIR, { recursive: true })
  fs.mkdirSync(environment.GOCACHE, { recursive: true })
  return environment
}

function runSync(executable, args, options = {}) {
  return spawnSync(executable, args, {
    encoding: 'utf8',
    windowsHide: true,
    timeout: 300000,
    ...options,
  })
}

async function main() {
  if (!fs.existsSync(engine) || !fs.existsSync(client) || !fs.existsSync(admin)) {
    throw new Error('Installed MySQL 8.4 engine/client are required.')
  }
  await reservePort()

  const configuredTemp = process.env.CAMPUS_TEST_TMPDIR
  if (configuredTemp) fs.mkdirSync(configuredTemp, { recursive: true })
  const temporaryRoot = fs.realpathSync(configuredTemp ? path.resolve(configuredTemp) : os.tmpdir())
  const directory = fs.mkdtempSync(path.join(temporaryRoot, 'campus-release-simulation-mysql-'))
  const logPath = path.join(directory, 'engine.log')
  const environment = createEnvironment(temporaryRoot)
  const sql = query => runSync(client, [...connection, '--execute', query], {
    timeout: 15000,
    env: environment,
  })

  let server
  let stopped = false
  let verifiedServer = false
  try {
    console.log('Initialize isolated MySQL for release-simulation acceptance.')
    const initialized = runSync(engine, [
      '--no-defaults',
      '--initialize-insecure',
      `--datadir=${directory}`,
      `--log-error=${logPath}`,
    ], { env: environment })
    if (initialized.error || initialized.status !== 0) {
      throw new Error('Isolated MySQL initialization failed: ' +
        (initialized.error?.message || initialized.stderr || fs.readFileSync(logPath, 'utf8')))
    }

    server = spawn(engine, [
      '--no-defaults',
      `--datadir=${directory}`,
      `--port=${port}`,
      '--bind-address=127.0.0.1',
      '--mysqlx=OFF',
      '--innodb-buffer-pool-size=64M',
      `--log-error=${logPath}`,
    ], { windowsHide: true, stdio: 'ignore', env: environment })

    let spawnError
    server.once('error', error => {
      spawnError = error
      stopped = true
    })
    server.once('exit', () => {
      stopped = true
    })

    for (let attempt = 0; attempt < 120; attempt++) {
      if (spawnError) throw spawnError
      if (stopped) throw new Error('Isolated MySQL stopped during startup: ' + fs.readFileSync(logPath, 'utf8'))
      const probe = sql('SELECT @@datadir')
      if (probe.status === 0) {
        if (path.resolve(probe.stdout.trim()).toLowerCase() !== path.resolve(directory).toLowerCase()) {
          throw new Error('Unexpected MySQL datadir; refusing to test or shut down that server.')
        }
        verifiedServer = true
        break
      }
      await wait(250)
    }
    if (!verifiedServer) throw new Error('Isolated MySQL startup timed out.')
    console.log(`ISOLATED_MYSQL_READY port=${port} datadir_match=true version=${sql('SELECT VERSION()').stdout.trim()}`)

    const database = `campus_release_simulation_${process.pid}`
    const databaseCreated = sql(
      `CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`,
    )
    if (databaseCreated.status !== 0) throw new Error(databaseCreated.stderr)

    const migrationDirectory = path.join(backend, 'migrations')
    const migrations = fs.readdirSync(migrationDirectory)
      .filter(name => /^\d{3}_.+\.sql$/.test(name))
      .sort()
    for (const name of migrations) {
      const applied = runSync(client, [...connection, database], {
        input: fs.readFileSync(path.join(migrationDirectory, name)),
        encoding: 'utf8',
        env: environment,
      })
      if (applied.error || applied.status !== 0) {
        throw new Error(`Migration ${name} failed: ${applied.error?.message || applied.stderr}`)
      }
    }
    console.log(`MIGRATIONS_APPLIED=${migrations.length}`)

    const testEnvironment = {
      ...environment,
      DELIVERY_D2_MYSQL_DSN:
        `root@tcp(127.0.0.1:${port})/${database}?charset=utf8mb4&parseTime=True&loc=UTC`,
      TEST_MYSQL_DSN:
        `root@tcp(127.0.0.1:${port})/?charset=utf8mb4&parseTime=True&loc=UTC`,
    }
    const runPattern = '^(' + expectedTests.join('|') + ')$'
    const result = runSync('go.exe', [
      'test',
      './cmd/server',
      '-run',
      runPattern,
      '-count=1',
      '-timeout=300s',
      '-v',
    ], {
      cwd: backend,
      env: testEnvironment,
      stdio: 'pipe',
      timeout: 360000,
    })
    process.stdout.write(result.stdout || '')
    process.stderr.write(result.stderr || '')
    const output = (result.stdout || '') + (result.stderr || '')
    if (/^[ \t]*--- SKIP:/m.test(output)) {
      throw new Error('Release-simulation suite contains a skipped scenario.')
    }
    const missing = expectedTests.filter(name =>
      !new RegExp(`^--- PASS: ${name}(?:\\s|$)`, 'm').test(output)
    )
    if (missing.length) {
      throw new Error('Release-simulation suite did not run required scenarios: ' + missing.join(', '))
    }
    if (result.error || result.status !== 0) {
      throw new Error('Release-simulation HTTP/MySQL suite failed.')
    }

    const residual = sql(
      "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'identity_http_%'",
    )
    if (residual.status !== 0 || residual.stdout.trim() !== '') {
      throw new Error('Identity temporary database cleanup verification failed.')
    }
    console.log(`RELEASE_SIMULATION_SCENARIOS_PASS=${expectedTests.length}`)
    console.log('RELEASE_SIMULATION_SKIPS=0')
    console.log('IDENTITY_TEMP_DATABASES_REMAINING=0')
  } finally {
    if (verifiedServer) {
      const identity = sql('SELECT @@datadir')
      if (identity.status === 0 &&
        path.resolve(identity.stdout.trim()).toLowerCase() !== path.resolve(directory).toLowerCase()) {
        throw new Error('MySQL server identity changed; refusing shutdown.')
      }
      if (identity.status === 0) {
        const shutdown = runSync(
          admin,
          connection.filter(arg => arg !== '--batch' && arg !== '--skip-column-names').concat('shutdown'),
          { timeout: 15000, env: environment },
        )
        if (shutdown.status !== 0) {
          throw new Error('Isolated MySQL shutdown failed; temporary data retained: ' + shutdown.stderr)
        }
      }
      for (let attempt = 0; attempt < 60 && !stopped; attempt++) await wait(250)
      if (!stopped) throw new Error('Isolated MySQL process remains alive; temporary data retained.')
    } else if (server && !stopped) {
      server.kill()
      for (let attempt = 0; attempt < 40 && !stopped; attempt++) await wait(250)
      if (!stopped) throw new Error('Unready MySQL process remains alive; temporary data retained.')
    }

    const resolved = fs.realpathSync(directory)
    if (path.dirname(resolved) !== temporaryRoot ||
      !path.basename(resolved).startsWith('campus-release-simulation-mysql-')) {
      throw new Error('Refusing unexpected temporary directory cleanup.')
    }
    fs.rmSync(resolved, { recursive: true, force: true })
    console.log('ISOLATED_CLEANUP=PASS engine_stopped=true temporary_data_removed=true existing_mysql_service_untouched=true')
  }
}

main().catch(error => {
  console.error(error.message)
  process.exitCode = 1
})
