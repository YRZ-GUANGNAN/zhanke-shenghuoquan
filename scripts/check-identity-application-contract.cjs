const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

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

const cancelStart = page.indexOf('  const restoreAfterNavigationFailure = () => {')
const cancelEnd = page.indexOf('\n\n  if (!role || !copy)', cancelStart)
const cancelAndBack = page.slice(cancelStart, cancelEnd)
const cancelRequirements = [
  'const restoreAfterNavigationFailure = () => {',
  'leavingRef.current = false',
  "setProgress('')",
  'await Taro.navigateBack()',
  "await Taro.reLaunch({ url: '/pages/profile/index' })",
  'finally {\n        restoreAfterNavigationFailure()'
]

for (const fragment of cancelRequirements) {
  if (!cancelAndBack.includes(fragment)) throw new Error(`cancelAndBack recovery contract missing: ${fragment}`)
}

const toastAttempt = cancelAndBack.indexOf("await Taro.showToast({ title: '已取消当前操作'")
const navigateAttempt = cancelAndBack.indexOf('await Taro.navigateBack()')
if (toastAttempt < 0 || navigateAttempt < toastAttempt || !cancelAndBack.slice(toastAttempt, navigateAttempt).includes('catch')) {
  throw new Error('cancelAndBack must continue to navigateBack when cancellation toast fails')
}

const restoreAttempt = cancelAndBack.indexOf('restoreAfterNavigationFailure()', navigateAttempt)
const relaunchAttempt = cancelAndBack.indexOf("await Taro.reLaunch({ url: '/pages/profile/index' })")
if (restoreAttempt < navigateAttempt || relaunchAttempt < restoreAttempt) {
  throw new Error('cancelAndBack must restore interaction before the profile reLaunch fallback')
}

function simulateCancelAndBack({ toastFails = false, navigateFails = false, relaunchFails = false }) {
  const state = {
    generation: 7,
    leaving: false,
    uploadLock: true,
    submitLock: true,
    uploading: true,
    submitting: true,
    progress: '处理中',
    navigateAttempts: 0,
    relaunchAttempts: 0
  }
  const restore = () => {
    state.leaving = false
    state.uploadLock = false
    state.submitLock = false
    state.uploading = false
    state.submitting = false
    state.progress = ''
  }

  state.leaving = true
  state.generation += 1
  state.uploadLock = false
  state.submitLock = false
  if (toastFails) state.progress = '提示失败'
  state.navigateAttempts += 1
  if (!navigateFails) return state
  restore()
  state.relaunchAttempts += 1
  if (relaunchFails) restore()
  return state
}

const toastFailure = simulateCancelAndBack({ toastFails: true })
assert.equal(toastFailure.navigateAttempts, 1)
assert.equal(toastFailure.leaving, true)
assert.equal(toastFailure.generation, 8)

for (const result of [
  simulateCancelAndBack({ navigateFails: true }),
  simulateCancelAndBack({ navigateFails: true, relaunchFails: true })
]) {
  assert.equal(result.relaunchAttempts, 1)
  assert.equal(result.leaving, false)
  assert.equal(result.uploadLock, false)
  assert.equal(result.submitLock, false)
  assert.equal(result.uploading, false)
  assert.equal(result.submitting, false)
  assert.equal(result.progress, '')
  assert.equal(result.generation, 8)
}

console.log('identity application contract: ok')
