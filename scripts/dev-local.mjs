// 一键拉起本地开发三件套：Meilisearch、后端 API、管理后台前端。
// 用法：npm run dev:local
// Meilisearch 的 master key 从 backend/.env 读取，与后端配置保持一致。
import { spawn } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const win = process.platform === 'win32'

let meiliKey = ''
const envPath = resolve(root, 'backend/.env')
if (existsSync(envPath)) {
  for (const line of readFileSync(envPath, 'utf8').split(/\r?\n/)) {
    const match = line.match(/^MEILI_MASTER_KEY=(.*)$/)
    if (match) meiliKey = match[1].trim().replace(/^["']|["']$/g, '')
  }
}

const meiliBinary = resolve(root, 'meilisearch.exe')
if (!existsSync(meiliBinary)) {
  console.error(`找不到 ${meiliBinary}，请先把 meilisearch.exe 放到仓库根目录。`)
  process.exit(1)
}

const children = []
let stopping = false

// shell 只在必须时开启（npm 是 .cmd，需要 shell；exe 一律不经过 shell，
// 否则含空格的路径会被 cmd.exe 按空格拆开）。
function run(name, command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: root,
    stdio: ['ignore', 'pipe', 'pipe'],
    shell: false,
    ...options
  })
  const prefix = `[${name}] `
  for (const stream of [child.stdout, child.stderr]) {
    stream.on('data', buffer => {
      for (const line of String(buffer).split(/\r?\n/)) {
        if (line.trim()) console.log(prefix + line)
      }
    })
  }
  child.on('error', error => {
    console.error(`${prefix}启动失败：${error.message}`)
    process.exitCode = process.exitCode || 1
    shutdown()
  })
  child.on('exit', code => {
    if (stopping) return
    console.log(`${prefix}进程意外退出（code=${code}），停止其余服务`)
    if (code) process.exitCode = code
    shutdown()
  })
  children.push(child)
  return child
}

function shutdown() {
  if (stopping) return
  stopping = true
  for (const child of children) {
    if (child.exitCode !== null || !child.pid) continue
    if (win) {
      spawn('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' })
    } else {
      child.kill('SIGTERM')
    }
  }
  // 给 taskkill 留出清理整棵进程树的时间，再按真实退出码结束。
  setTimeout(() => process.exit(process.exitCode ?? 0), 1500)
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', () => {
  process.exitCode = process.exitCode ?? 0
  shutdown()
})

const meiliArgs = ['--db-path', resolve(root, 'data.ms')]
if (meiliKey) meiliArgs.unshift('--master-key', meiliKey)
else console.warn('[meili] backend/.env 中未找到 MEILI_MASTER_KEY，将以无鉴权模式启动')

run('meili', meiliBinary, meiliArgs)
run('api', 'go', ['run', './cmd/server'], { cwd: resolve(root, 'backend') })
run('web', win ? 'npm.cmd' : 'npm', ['--workspace', 'apps/admin', 'run', 'dev'], { shell: win })

console.log('本地开发环境启动中：Meilisearch :7700 · API :8080 · 管理后台 :5173（Ctrl+C 停止全部）')
