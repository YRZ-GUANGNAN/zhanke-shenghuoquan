const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const nativeRoot = path.join(root, 'deploy', 'native')
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
  'and 443.'
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

console.log('native deployment contract: ok')
