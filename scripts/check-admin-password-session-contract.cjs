const fs = require('fs')
const path = require('path')

const root = path.resolve(__dirname, '..')
const sourceRoot = path.join(root, 'apps', 'admin', 'src')
const files = []

function collect(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name)
    if (entry.isDirectory()) collect(target)
    else if (/\.(ts|tsx)$/.test(entry.name)) files.push(target)
  }
}

collect(sourceRoot)
const source = files.map(file => `${path.relative(root, file)}\n${fs.readFileSync(file, 'utf8')}`).join('\n')
const failures = []

if (/domainA\.token|localStorage\.(getItem|setItem)\s*\(/.test(source)) {
  failures.push('admin source must not use the legacy localStorage token')
}

const required = [
  ["credentials: 'same-origin'", 'same-origin cookie fetch'],
  ["headers.set('X-CSRF-Token'", 'CSRF header injection'],
  ["'/admin-auth/login'", 'password login endpoint'],
  ["'/admin-auth/session'", 'session restore endpoint'],
  ["'/admin-auth/logout'", 'logout endpoint'],
  ["'/admin-auth/password/change'", 'password change endpoint'],
  ["key:'identities'", 'identity applications navigation'],
  ['<IdentityApplications/>', 'identity applications page']
]

for (const [token, description] of required) {
  if (!source.includes(token)) failures.push(`missing ${description}: ${token}`)
}

if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log('admin password session contract: PASS')
