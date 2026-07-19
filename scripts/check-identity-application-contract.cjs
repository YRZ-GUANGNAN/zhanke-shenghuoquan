const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (value) => fs.readFileSync(path.join(root, value), 'utf8')
const app = read('apps/miniapp/src/app.config.ts')
const profile = read('apps/miniapp/src/pages/profile/index.tsx')
const page = read('apps/miniapp/src/pages/identity-application/index.tsx')
const userClient = read('apps/miniapp/src/services/a-user.ts')
const types = read('apps/miniapp/src/types.ts')

const requirements = [
  [app, 'pages/identity-application/index'],
  [profile, '/pages/identity-application/index?role='],
  [page, "chooseAndUploadImages('identity_application'"],
  [page, 'waitForMediaApproval('],
  [page, "application?.status === 'rejected' ? application.rowVersion : 0"],
  [page, "application?.status === 'pending'"],
  [page, "application?.status === 'approved'"],
  [page, "application?.status === 'rejected'"],
  [page, "role === 'merchant'"],
  [page, 'proofs.map((item) => item.assetId)'],
  [userClient, "request(`/auth/identity-applications/${role}`)"],
  [userClient, "method: 'PUT'"],
  [types, 'userId: string'],
  [types, 'reviewerUserId?: string']
]

for (const [source, fragment] of requirements) {
  if (!source.includes(fragment)) throw new Error(`identity application contract missing: ${fragment}`)
}

if (!page.includes("value === 'rider' || value === 'merchant'")) {
  throw new Error('identity application route must reject unsupported roles')
}

if (!page.includes('proofs.length < 1 || proofs.length > 6')) {
  throw new Error('identity application proof count must stay within 1..6')
}

console.log('identity application contract: ok')
