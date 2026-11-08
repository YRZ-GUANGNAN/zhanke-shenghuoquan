const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const net = require('node:net')
const { spawn, spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const bin = 'C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin'
const engine = path.join(bin, 'mysqld.exe')
const client = path.join(bin, 'mysql.exe')
const admin = path.join(bin, 'mysqladmin.exe')
const port = 33318
const connection = ['--no-defaults', '--no-login-paths', '--protocol=tcp', '--host=127.0.0.1', '--port=' + port, '--user=root', '--connect-timeout=2', '--batch', '--skip-column-names']
const wait = ms => new Promise(resolve => setTimeout(resolve, ms))
const env = { ...process.env }
for (const name of Object.keys(env)) if (/MYSQL|DSN|DATABASE_URL/i.test(name)) delete env[name]
const runSync = (exe, args, options = {}) => spawnSync(exe, args, { encoding: 'utf8', windowsHide: true, env, timeout: 120000, ...options })

function collectRiderMoneyTests(directory) {
  const names = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name)
    if (entry.isDirectory()) names.push(...collectRiderMoneyTests(absolute))
    else if (entry.isFile() && entry.name.endsWith('_test.go')) {
      const source = fs.readFileSync(absolute, 'utf8')
      for (const match of source.matchAll(/^func (Test[A-Za-z0-9_]+AgainstIsolatedMySQL)\(/gm)) names.push(match[1])
    }
  }
  return [...new Set(names)].sort()
}

async function main() {
  if (!fs.existsSync(engine) || !fs.existsSync(client) || !fs.existsSync(admin)) throw new Error('Installed MySQL 8.4 engine/client are required.')
  const probe = net.createServer()
  await new Promise((resolve, reject) => { probe.once('error', reject); probe.listen(port, '127.0.0.1', () => probe.close(resolve)) })
  const temporaryRoot = fs.realpathSync(os.tmpdir())
  const directory = fs.mkdtempSync(path.join(temporaryRoot, 'campus-rider-money-mysql-'))
  const logPath = path.join(directory, 'engine.log')
  const sql = query => runSync(client, [...connection, '--execute', query], { timeout: 10000 })
  let server
  let stopped = false
  let verifiedServer = false
  try {
    console.log('Initialize fresh local MySQL datadir; existing services/databases are not used.')
    const initialized = runSync(engine, ['--no-defaults', '--initialize-insecure', '--datadir=' + directory, '--log-error=' + logPath])
    if (initialized.error || initialized.status !== 0) throw new Error('Isolated MySQL initialization failed: ' + (initialized.error?.message || initialized.stderr || fs.readFileSync(logPath, 'utf8')))
    server = spawn(engine, ['--no-defaults', '--datadir=' + directory, '--port=' + port, '--bind-address=127.0.0.1', '--mysqlx=OFF', '--innodb-buffer-pool-size=64M', '--log-error=' + logPath], { windowsHide: true, stdio: 'ignore', env })
    let spawnError
    server.once('error', error => { spawnError = error; stopped = true })
    server.once('exit', () => { stopped = true })
    for (let attempt = 0; attempt < 120; attempt++) {
      if (spawnError) throw spawnError
      if (stopped) throw new Error('Isolated MySQL stopped during startup: ' + fs.readFileSync(logPath, 'utf8'))
      const probeResult = sql('SELECT @@datadir')
      if (probeResult.status === 0) {
        if (path.resolve(probeResult.stdout.trim()).toLowerCase() !== path.resolve(directory).toLowerCase()) throw new Error('Unexpected MySQL datadir; refusing to test or shut down that server.')
        verifiedServer = true
        break
      }
      await wait(250)
    }
    if (!verifiedServer) throw new Error('Isolated MySQL startup timed out.')
    console.log('ISOLATED_READY port=' + port + ' datadir_match=true version=' + sql('SELECT VERSION()').stdout.trim())
    const sentinel = 'campus_rider_money_runner_' + process.pid
    const testEnv = { ...env, DELIVERY_PAYOUT_QUEUE_MYSQL_DSN: 'root@tcp(127.0.0.1:' + port + ')/' + sentinel + '?charset=utf8mb4&parseTime=True&loc=UTC', DELIVERY_RIDER_MONEY_TEST_DATADIR: directory }
    const result = runSync('go.exe', ['test', './internal/module/delivery', '-run', 'TestRider.*AgainstIsolatedMySQL|TestSupportsEnforcedCheckConstraints', '-count=1', '-timeout=180s', '-v'], { cwd: path.join(root, 'backend'), env: testEnv, stdio: 'pipe', timeout: 240000 })
    process.stdout.write(result.stdout || '')
    process.stderr.write(result.stderr || '')
    const output = (result.stdout || '') + (result.stderr || '')
    if (/^--- SKIP:/m.test(output)) throw new Error('Rider-money integration suite contains a skipped test.')
    const expectedTests = collectRiderMoneyTests(path.join(root, 'backend', 'internal', 'module', 'delivery'))
    const missingTests = expectedTests.filter(name => !new RegExp(`^--- PASS: ${name}(?:\\s|$)`, 'm').test(output))
    if (missingTests.length) throw new Error('Rider-money integration suite did not run required tests: ' + missingTests.join(', '))
    if (result.error || result.status !== 0) throw new Error('Isolated rider-money integration suite failed.')
    const residual = sql("SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'delivery_rider_money_%'")
    if (residual.status !== 0 || residual.stdout.trim() !== '') throw new Error('Test database cleanup verification failed.')
    console.log('ISOLATED_TEST_DATABASES_REMAINING=0')
  } finally {
    if (verifiedServer) {
      const identity = sql('SELECT @@datadir')
      if (identity.status === 0 && path.resolve(identity.stdout.trim()).toLowerCase() !== path.resolve(directory).toLowerCase()) throw new Error('Server identity changed; refusing shutdown.')
      if (identity.status === 0) {
        const shutdown = runSync(admin, connection.filter(arg => arg !== '--batch' && arg !== '--skip-column-names').concat('shutdown'), { timeout: 15000 })
        if (shutdown.status !== 0) throw new Error('Isolated shutdown failed; temporary data retained: ' + shutdown.stderr)
      }
      let socketClosed = false
      for (let attempt = 0; attempt < 60; attempt++) {
        socketClosed = await new Promise(resolve => { const socket = net.connect({ host: '127.0.0.1', port }); socket.once('connect', () => { socket.destroy(); resolve(false) }); socket.once('error', () => { socket.destroy(); resolve(true) }) })
        if (socketClosed) break
        await wait(250)
      }
      if (!socketClosed) throw new Error('Isolated MySQL port remains open; temporary data retained.')
      for (let attempt = 0; attempt < 40 && !stopped; attempt++) await wait(250)
      if (!stopped) throw new Error('Isolated MySQL process remains alive; temporary data retained.')
    } else if (server && !stopped) {
      server.kill()
      for (let attempt = 0; attempt < 40 && !stopped; attempt++) await wait(250)
      if (!stopped) throw new Error('Unready MySQL process remains alive; temporary data retained.')
    }
    const resolved = fs.realpathSync(directory)
    if (path.dirname(resolved) !== temporaryRoot || !path.basename(resolved).startsWith('campus-rider-money-mysql-')) throw new Error('Refusing unexpected cleanup path.')
    let cleanupError
    for (let attempt = 0; attempt < 20; attempt++) {
      try { await fs.promises.rm(resolved, { recursive: true, force: true }); cleanupError = undefined; break } catch (error) { cleanupError = error; if (!['EPERM', 'EBUSY', 'ENOTEMPTY'].includes(error.code)) break; await wait(500) }
    }
    if (cleanupError || fs.existsSync(resolved)) throw new Error('Temporary data cleanup failed at ' + directory + ': ' + (cleanupError?.message || 'path remains'))
    console.log('ISOLATED_CLEANUP=PASS engine_stopped=true temporary_data_removed=true existing_mysql_service_untouched=true')
  }
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
