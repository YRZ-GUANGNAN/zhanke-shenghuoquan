const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const net = require('node:net')
const { spawn, spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const mysqlDirectory = 'C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin'
const mysql = path.join(mysqlDirectory, 'mysql.exe')
const mysqld = path.join(mysqlDirectory, 'mysqld.exe')
const port = 33308
const connection = ['--no-defaults', '--protocol=tcp', '--host=127.0.0.1', `--port=${port}`, '--user=root', '--default-character-set=utf8mb4']
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))

async function run() {
  if (!fs.existsSync(mysqld)) throw new Error('Installed MySQL 8.4 engine is required.')
  await new Promise((resolve, reject) => {
    const probe = net.createServer()
    probe.once('error', reject)
    probe.listen(port, '127.0.0.1', () => probe.close(resolve))
  })
  const temporaryRoot = fs.realpathSync(os.tmpdir())
  const dataDirectory = fs.mkdtempSync(path.join(temporaryRoot, 'campus-delivery-source-mysql-'))
  const logPath = path.join(dataDirectory, 'engine.log')
  const environment = { ...process.env }
  for (const key of Object.keys(environment)) {
    if (key === 'MYSQL_PWD' || key.endsWith('MYSQL_DSN')) delete environment[key]
  }
  const sql = query => spawnSync(mysql, [...connection, '--execute', query], { encoding: 'utf8', windowsHide: true, env: environment })
  let server
  try {
    const initialized = spawnSync(mysqld, ['--no-defaults', '--initialize-insecure', `--datadir=${dataDirectory}`, `--log-error=${logPath}`], { encoding: 'utf8', windowsHide: true, env: environment })
    if (initialized.status !== 0) throw new Error(initialized.stderr || fs.readFileSync(logPath, 'utf8'))
    server = spawn(mysqld, ['--no-defaults', `--datadir=${dataDirectory}`, `--port=${port}`, '--bind-address=127.0.0.1', '--mysqlx=OFF', '--innodb-buffer-pool-size=64M', `--log-error=${logPath}`], { windowsHide: true, stdio: 'ignore', env: environment })
    let ready = false
    for (let attempt = 0; attempt < 120; attempt++) {
      if (server.exitCode !== null) throw new Error(fs.readFileSync(logPath, 'utf8'))
      if (sql('SELECT 1').status === 0) { ready = true; break }
      await wait(250)
    }
    if (!ready) throw new Error('Isolated MySQL startup timed out.')
    const database = `campus_delivery_source_test_${process.pid}`
    const created = sql(`CREATE DATABASE ${database} CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`)
    if (created.status !== 0) throw new Error(created.stderr)
    const migrationDirectory = path.join(root, 'backend', 'migrations')
    const baseline = fs.readdirSync(migrationDirectory).filter(name => /^\d{3}_.+\.sql$/.test(name) && Number(name.slice(0, 3)) < 45).sort()
    for (const name of baseline) {
      const applied = spawnSync(mysql, [...connection, database], { input: fs.readFileSync(path.join(migrationDirectory, name), 'utf8'), encoding: 'utf8', windowsHide: true, env: environment })
      if (applied.status !== 0) throw new Error(`Baseline migration ${name}: ${applied.stderr}`)
    }
    console.log(`Applied ${baseline.length} baseline migrations to an isolated temporary database.`)
    const result = spawnSync('go.exe', ['test', './internal/module/admin/deliverymerchant', '-run', 'TestOrderSourceTemporaryMySQL', '-count=1', '-v'], {
      cwd: path.join(root, 'backend'), env: { ...environment, DELIVERY_SOURCE_TEST_MYSQL_DSN: `root@tcp(127.0.0.1:${port})/${database}?charset=utf8mb4&parseTime=True&loc=UTC` }, stdio: 'inherit', windowsHide: true,
    })
    if (result.status !== 0) throw new Error('Delivery-source MySQL integration test failed.')
  } finally {
    if (server && server.exitCode === null) {
      spawnSync(path.join(mysqlDirectory, 'mysqladmin.exe'), [...connection, 'shutdown'], { windowsHide: true, stdio: 'ignore', env: environment })
      for (let attempt = 0; attempt < 40 && server.exitCode === null; attempt++) await wait(250)
      if (server.exitCode === null) {
        server.kill()
        for (let attempt = 0; attempt < 40 && server.exitCode === null; attempt++) await wait(250)
      }
      if (server.exitCode === null) throw new Error('Isolated engine did not stop; temporary data retained.')
    }
    const resolved = fs.realpathSync(dataDirectory)
    if (path.dirname(resolved) !== temporaryRoot || !path.basename(resolved).startsWith('campus-delivery-source-mysql-')) throw new Error('Refusing unexpected temporary directory cleanup.')
    fs.rmSync(resolved, { recursive: true, force: true })
    console.log('Isolated engine and temporary database removed; existing MySQL services and business data were untouched.')
  }
}
run().catch(error => { console.error(error.message); process.exitCode = 1 })
