const fs = require('node:fs')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const root = path.resolve(__dirname, '..')
const nativeRoot = path.join(root, 'deploy', 'native')
const packageJSON = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'))
const packageLock = JSON.parse(fs.readFileSync(path.join(root, 'package-lock.json'), 'utf8'))
const adminDockerfile = fs.readFileSync(path.join(root, 'apps', 'admin', 'Dockerfile'), 'utf8')
const backendDockerfile = fs.readFileSync(path.join(root, 'backend', 'Dockerfile'), 'utf8')
const compose = fs.readFileSync(path.join(root, 'deploy', 'docker-compose.yml'), 'utf8')
const dockerignorePath = path.join(root, '.dockerignore')
const nativeBuildScriptPath = path.join(root, 'scripts', 'build-native-backend.ps1')
const requiredFiles = [
  '00-campus-log-format.conf',
  'campus-api.service',
  'campus-journald.conf',
  'campus-meilisearch.service',
  'nginx-zhanduiwang.top.conf',
  'README.md'
]

const files = new Map()
for (const name of requiredFiles) {
  const target = path.join(nativeRoot, name)
  if (!fs.existsSync(target)) throw new Error(`missing native deployment asset: ${name}`)
  files.set(name, fs.readFileSync(target, 'utf8'))
}

if (!fs.existsSync(nativeBuildScriptPath)) throw new Error('missing fixed native backend build script')
const nativeBuildScript = fs.readFileSync(nativeBuildScriptPath, 'utf8')
for (const token of [
  '[Parameter(Mandatory = $true)]',
  '$env:GOOS = "linux"',
  '$env:GOARCH = "amd64"',
  '$env:CGO_ENABLED = "0"',
  '-X main.buildVersion=$Version',
  '-X main.buildCommit=$commit',
  '-X main.buildTime=$buildTime',
  'Get-FileHash -Algorithm SHA256'
]) {
  if (!nativeBuildScript.includes(token)) throw new Error(`scripts/build-native-backend.ps1: missing ${token}`)
}
for (const token of [
  'status --porcelain --untracked-files=all -- backend scripts/build-native-backend.ps1',
  'Release source is dirty'
]) {
  if (!nativeBuildScript.includes(token)) throw new Error(`scripts/build-native-backend.ps1: missing ${token}`)
}
if (process.platform === 'win32') {
  const escapedPath = nativeBuildScriptPath.replaceAll("'", "''")
  const parseCommand = [
    '$tokens = $null',
    '$errors = $null',
    `[System.Management.Automation.Language.Parser]::ParseFile('${escapedPath}', [ref]$tokens, [ref]$errors) | Out-Null`,
    'if ($errors.Count -gt 0) { $errors | ForEach-Object { Write-Error $_.ToString() }; exit 1 }'
  ].join('; ')
  const parsed = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', parseCommand], {
    encoding: 'utf8'
  })
  if (parsed.status !== 0) {
    throw new Error(`scripts/build-native-backend.ps1 is not parseable by Windows PowerShell 5.1:\n${parsed.stderr || parsed.stdout}`)
  }
}

function requireTokens(name, tokens) {
  const source = files.get(name)
  for (const token of tokens) {
    if (!source.includes(token)) throw new Error(`${name}: missing ${token}`)
  }
}

requireTokens('campus-api.service', [
  'User=campus',
  'Group=campus',
  'EnvironmentFile=/etc/campus-circle/campus-api.env',
  'ExecStart=/opt/campus-circle/current/bin/campus-api',
  'Restart=on-failure',
  'NoNewPrivileges=true',
  'ProtectSystem=strict',
  'PrivateTmp=true'
])

requireTokens('campus-meilisearch.service', [
  'User=campus',
  'Group=campus',
  'EnvironmentFile=/etc/campus-circle/meilisearch.env',
  'ExecStart=/opt/campus-circle/bin/meilisearch',
  'ReadWritePaths=/var/lib/campus-meilisearch',
  'NoNewPrivileges=true',
  'ProtectSystem=strict'
])

requireTokens('nginx-zhanduiwang.top.conf', [
  'server_name zhanduiwang.top www.zhanduiwang.top;',
  'ssl_protocols TLSv1.2 TLSv1.3;',
  'root /var/www/campus-admin/current;',
  'proxy_pass http://127.0.0.1:8080/healthz;',
  'allow 127.0.0.1;',
  'allow ::1;',
  'deny all;',
  'location ^~ /api/',
  'proxy_pass http://127.0.0.1:8080;',
  'Strict-Transport-Security',
  'try_files $uri $uri/ /index.html;'
])

requireTokens('00-campus-log-format.conf', [
  'log_format campus',
  '$request_time',
  '$upstream_response_time'
])

requireTokens('campus-journald.conf', [
  'Storage=persistent',
  'SystemMaxUse=200M',
  'MaxRetentionSec=14day',
  'RateLimitBurst=2000'
])

requireTokens('README.md', [
  'Secrets are intentionally excluded.',
  '`002_seed_dev.sql` explicitly skipped',
  'TARO_APP_API_BASE=https://zhanduiwang.top/api/v1',
  'Only Nginx exposes ports 80',
  'and 443.',
  '.\\scripts\\build-native-backend.ps1 -Version 2026.08.01'
])

const combined = [...files.values()].join('\n')
const forbidden = [
  /WECHAT_APP_SECRET\s*=\s*\S+/i,
  /(?:PASSWORD|SECRET|TOKEN|PRIVATE_KEY)\s*=\s*\S+/i,
  /AKID[A-Za-z0-9]{12,}/,
  /-----BEGIN [A-Z ]+PRIVATE KEY-----/
]
for (const pattern of forbidden) {
  if (pattern.test(combined)) throw new Error(`native deployment assets contain forbidden secret material: ${pattern}`)
}

if (packageJSON.engines?.node !== '^20.19.0 || >=22.12.0') {
  throw new Error('root package.json must pin the Node range required by Vite 8')
}
if (packageLock.packages?.['']?.engines?.node !== packageJSON.engines.node) {
  throw new Error('package-lock.json must preserve the same root Node engine range')
}

for (const token of [
  'COPY package.json package-lock.json ./',
  'COPY apps/admin/package.json apps/admin/package.json',
  'npm ci --workspace apps/admin',
  '/workspace/apps/admin/dist',
  'COPY apps/admin/nginx.conf'
]) {
  if (!adminDockerfile.includes(token)) throw new Error(`apps/admin/Dockerfile: missing ${token}`)
}
if (/\bnpm install\b/.test(adminDockerfile)) throw new Error('apps/admin/Dockerfile must use reproducible npm ci')

for (const token of [
  'ARG BUILD_VERSION=dev',
  '-X main.buildVersion=${BUILD_VERSION}',
  '-X main.buildCommit=${BUILD_COMMIT}',
  '-X main.buildTime=${BUILD_TIME}',
  'http://127.0.0.1:8080/readyz'
]) {
  if (!backendDockerfile.includes(token)) throw new Error(`backend/Dockerfile: missing ${token}`)
}

if (!/^# Local development stack only\./.test(compose)) {
  throw new Error('deploy/docker-compose.yml must identify itself as development-only')
}
for (const token of [
  'name: campus-circle-dev',
  'container_name: campus_dev_mysql',
  'container_name: campus_dev_backend',
  'context: ..',
  'dockerfile: apps/admin/Dockerfile',
  'BUILD_VERSION: ${BUILD_VERSION:-dev}',
  'http://127.0.0.1:8080/readyz',
  'condition: service_healthy'
]) {
  if (!compose.includes(token)) throw new Error(`deploy/docker-compose.yml: missing ${token}`)
}

const nativeNginx = fs.readFileSync(path.join(nativeRoot, 'nginx-zhanduiwang.top.conf'), 'utf8')
for (const endpoint of ['/healthz', '/readyz', '/version']) {
  if (!nativeNginx.includes(`location = ${endpoint}`)) throw new Error(`native nginx must proxy ${endpoint}`)
}
if (/APP_ENV:\s*prod\b/.test(compose)) throw new Error('development compose must not claim to run the production profile')

const readyBlock = nativeNginx.match(/location\s*=\s*\/readyz\s*\{([\s\S]*?)\n\s*\}/)?.[1] ?? ''
for (const token of ['allow 127.0.0.1;', 'allow ::1;', 'deny all;', 'proxy_pass http://127.0.0.1:8080/readyz;']) {
  if (!readyBlock.includes(token)) throw new Error(`native nginx /readyz block must contain ${token}`)
}

if (!fs.existsSync(dockerignorePath)) throw new Error('missing root .dockerignore for repository-root Docker builds')
const dockerignore = fs.readFileSync(dockerignorePath, 'utf8')
for (const token of ['**/node_modules', '**/dist', '**/.env', 'apps/miniapp', 'backend']) {
  if (!dockerignore.includes(token)) throw new Error(`.dockerignore: missing ${token}`)
}

console.log('native deployment contract: ok')
