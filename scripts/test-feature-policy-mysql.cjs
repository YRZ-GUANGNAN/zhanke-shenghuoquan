const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const net = require('node:net')
const { spawn, spawnSync } = require('node:child_process')
const root = path.resolve(__dirname, '..')
const shareSettings = process.argv.includes('--share-settings')
const profileCover = process.argv.includes('--profile-cover')
const commentInteractions = process.argv.includes('--comment-interactions')
const mysqlDirectory = 'C:\\Program Files\\MySQL\\MySQL Server 8.4\\bin'
const mysql = path.join(mysqlDirectory, 'mysql.exe')
const mysqld = path.join(mysqlDirectory, 'mysqld.exe')
const port = 33308
const connection = ['--no-defaults', '--protocol=tcp', '--host=127.0.0.1', `--port=${port}`, '--user=root']
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds))
const sql = query => spawnSync(mysql, [...connection, '--execute', query], { encoding: 'utf8', windowsHide: true })

async function run() {
  if (!fs.existsSync(mysqld)) throw new Error('Installed MySQL 8.4 engine is required.')
  await new Promise((resolve, reject) => {
    const probe = net.createServer()
    probe.once('error', reject)
    probe.listen(port, '127.0.0.1', () => probe.close(resolve))
  })
  const temporaryRoot = fs.realpathSync(os.tmpdir())
  const dataDirectory = fs.mkdtempSync(path.join(temporaryRoot, 'campus-policy-mysql-'))
  const logPath = path.join(dataDirectory, 'engine.log')
  let server
  try {
    const initialized = spawnSync(mysqld, ['--no-defaults', '--initialize-insecure', `--datadir=${dataDirectory}`, `--log-error=${logPath}`], { encoding: 'utf8', windowsHide: true })
    if (initialized.status !== 0) throw new Error(initialized.stderr || fs.readFileSync(logPath, 'utf8'))
    server = spawn(mysqld, ['--no-defaults', `--datadir=${dataDirectory}`, `--port=${port}`, '--bind-address=127.0.0.1', '--mysqlx=OFF', '--innodb-buffer-pool-size=64M', `--log-error=${logPath}`], { windowsHide: true, stdio: 'ignore' })
    let ready = false
    for (let attempt = 0; attempt < 100; attempt++) {
      if (server.exitCode !== null) throw new Error(fs.readFileSync(logPath, 'utf8'))
      if (sql('SELECT 1').status === 0) { ready = true; break }
      await wait(250)
    }
    if (!ready) throw new Error('Isolated MySQL startup timed out.')
    const database = `${commentInteractions ? 'campus_comments_test_' : profileCover ? 'campus_cover_test_' : shareSettings ? 'campus_share_test_' : 'campus_feature_policy_test_'}${process.pid}`
    const created = sql(`CREATE DATABASE ${database} CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci`)
    if (created.status !== 0) throw new Error(created.stderr)
    const testDSN = `root@tcp(127.0.0.1:${port})/${database}?charset=utf8mb4&parseTime=True&loc=${shareSettings ? 'Local' : 'UTC'}`
    const moduleName = commentInteractions ? 'community' : profileCover ? 'user' : shareSettings ? 'sharesettings' : 'featureflags'
    const testName = commentInteractions ? 'TestCommentInteractionsTemporaryMySQL' : profileCover ? 'TestProfileCoverTemporaryMySQL' : shareSettings ? 'TestTemporaryMySQLPersistenceCASAndAuditRollback' : 'TestGormTemporaryMySQLPersistenceAndAudit'
    const testKey = commentInteractions ? 'COMMENT_INTERACTIONS_TEST_MYSQL_DSN' : profileCover ? 'PROFILE_COVER_TEST_MYSQL_DSN' : shareSettings ? 'SHARE_SETTINGS_TEST_MYSQL_DSN' : 'FEATURE_POLICY_TEST_MYSQL_DSN'
    const result = spawnSync('go.exe', ['test', `./internal/module/${moduleName}`, '-run', testName, '-count=1', '-v'], { cwd: path.join(root, 'backend'), env: { ...process.env, [testKey]: testDSN }, stdio: 'inherit', windowsHide: true })
    if (result.status !== 0) throw new Error(`${moduleName} MySQL integration test failed.`)
  } finally {
    if (server && server.exitCode === null) {
      spawnSync(path.join(mysqlDirectory, 'mysqladmin.exe'), [...connection, 'shutdown'], { windowsHide: true, stdio: 'ignore' })
      for (let attempt = 0; attempt < 40 && server.exitCode === null; attempt++) await wait(250)
      if (server.exitCode === null) server.kill()
    }
    const resolved = fs.realpathSync(dataDirectory)
    if (path.dirname(resolved) !== temporaryRoot || !path.basename(resolved).startsWith('campus-policy-mysql-')) throw new Error('Refusing unexpected temporary directory cleanup.')
    fs.rmSync(resolved, { recursive: true, force: true })
    console.log('Isolated MySQL engine and temporary database cleaned up; existing MySQL service/data were untouched.')
  }
}
run().catch(error => { console.error(error.message); process.exitCode = 1 })
