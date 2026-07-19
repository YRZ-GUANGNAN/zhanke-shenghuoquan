const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (value) => fs.readFileSync(path.join(root, value), 'utf8')
const app = read('apps/miniapp/src/app.config.ts')
const profile = read('apps/miniapp/src/pages/profile/index.tsx')
const page = read('apps/miniapp/src/pages/identity-application/index.tsx')
const userClient = read('apps/miniapp/src/services/a-user.ts')
const mediaClient = read('apps/miniapp/src/services/media.ts')
const types = read('apps/miniapp/src/types.ts')

const requirements = [
  [app, 'pages/identity-application/index'],
  [profile, '/pages/identity-application/index?role='],
  [page, "chooseAndUploadImagesSettled('identity_application'"],
  [page, 'waitForMediaApproval('],
  [page, 'mountedRef'],
  [page, 'operationGenerationRef'],
  [page, 'uploadLockRef.current = true'],
  [page, 'submitLockRef.current = true'],
  [page, 'cancelAndBack'],
  [page, 'settlements.reduce<UploadedMedia[]>'],
  [page, 'item.reason.message'],
  [page, "application?.status === 'rejected' ? application.rowVersion : 0"],
  [page, "application?.status === 'pending'"],
  [page, "application?.status === 'approved'"],
  [page, "application?.status === 'rejected'"],
  [page, "role === 'merchant'"],
  [page, 'proofs.map((item) => item.assetId)'],
  [userClient, "request(`/auth/identity-applications/${role}`)"],
  [userClient, "method: 'PUT'"],
  [mediaClient, 'export async function chooseAndUploadImagesSettled'],
  [mediaClient, 'shouldContinue?: () => boolean'],
  [profile, "availability?.status === 'missing'"],
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

if (!/if \(!isOperationActive\(generation\)\) return\s+const result = await submitIdentityApplication/.test(page)) {
  throw new Error('identity application must recheck cancellation immediately before PUT')
}

for (const lock of ['uploadLockRef', 'submitLockRef']) {
  const acquire = page.indexOf(`${lock}.current = true`)
  const firstAwait = page.indexOf('await ', acquire)
  if (acquire < 0 || firstAwait < acquire) throw new Error(`${lock} must be acquired before the first await`)
}

console.log('identity application contract: ok')
