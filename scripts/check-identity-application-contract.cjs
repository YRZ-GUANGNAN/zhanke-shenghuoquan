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
  [page, 'getMediaStatus(proof.assetId)'],
  [page, '材料尚不可用，请重试或重新上传'],
  [page, "status.purpose === 'identity_application' && isMediaUsable(status)"],
  [page, '无需停留在此页，稍后返回重试即可'],
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

if (page.includes('waitForMediaApproval(')) {
  throw new Error('identity application submit must not block while polling media review')
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

const cancelStart = page.indexOf('  const restoreAfterNavigationFailure = (cancelGeneration: number) => {')
const cancelEnd = page.indexOf('\n\n  if (!role || !copy)', cancelStart)
const cancelAndBack = page.slice(cancelStart, cancelEnd)
const cancelRequirements = [
  'const restoreAfterNavigationFailure = (cancelGeneration: number) => {',
  'operationGenerationRef.current !== cancelGeneration',
  'leavingRef.current = false',
  'setLoading(false)',
  'void loadApplication()',
  "setProgress('')",
  'await Taro.navigateBack()',
  "await Taro.reLaunch({ url: '/pages/profile/index' })",
  'restoreAfterNavigationFailure(cancelGeneration)'
]

for (const fragment of cancelRequirements) {
  if (!cancelAndBack.includes(fragment)) throw new Error(`cancelAndBack recovery contract missing: ${fragment}`)
}

const toastAttempt = cancelAndBack.indexOf("await Taro.showToast({ title: '已取消当前操作'")
const navigateAttempt = cancelAndBack.indexOf('await Taro.navigateBack()')
if (toastAttempt < 0 || navigateAttempt < toastAttempt || !cancelAndBack.slice(toastAttempt, navigateAttempt).includes('catch')) {
  throw new Error('cancelAndBack must continue to navigateBack when cancellation toast fails')
}

const relaunchAttempt = cancelAndBack.indexOf("await Taro.reLaunch({ url: '/pages/profile/index' })")
const restoreAttempt = cancelAndBack.indexOf('restoreAfterNavigationFailure(cancelGeneration)', relaunchAttempt)
if (relaunchAttempt < navigateAttempt || restoreAttempt < relaunchAttempt) {
  throw new Error('cancelAndBack must restore only after both navigation attempts fail')
}
if (cancelAndBack.includes('finally {')) {
  throw new Error('cancelAndBack must not restore the old page from finally')
}

function simulateCancelAndBack({ toastFails = false, navigateFails = false, relaunchFails = false, startNewOperation = false }) {
  const state = {
    generation: 7,
    loadSequence: 1,
    leaving: false,
    uploadLock: true,
    submitLock: true,
    uploading: true,
    submitting: true,
    loading: true,
    progress: '处理中',
    navigateAttempts: 0,
    relaunchAttempts: 0,
    reloads: 0,
    interactiveDuringRelaunch: null
  }
  const restore = (cancelGeneration) => {
    if (state.generation !== cancelGeneration || !state.leaving) return false
    state.leaving = false
    state.uploadLock = false
    state.submitLock = false
    state.uploading = false
    state.submitting = false
    state.loading = false
    state.progress = ''
    state.loadSequence += 1
    state.reloads += 1
    state.loading = true
    return true
  }

  state.leaving = true
  const cancelGeneration = ++state.generation
  state.loadSequence += 1
  state.uploadLock = false
  state.submitLock = false
  if (toastFails) state.progress = '提示失败'
  state.navigateAttempts += 1
  if (!navigateFails) return state
  state.interactiveDuringRelaunch = !state.leaving && !state.uploadLock && !state.submitLock
  state.relaunchAttempts += 1
  if (!relaunchFails) return state
  restore(cancelGeneration)
  if (startNewOperation) {
    state.generation += 1
    state.uploadLock = true
    state.uploading = true
  }
  return state
}

const toastFailure = simulateCancelAndBack({ toastFails: true })
assert.equal(toastFailure.navigateAttempts, 1)
assert.equal(toastFailure.leaving, true)
assert.equal(toastFailure.generation, 8)
assert.equal(toastFailure.loadSequence, 2)

const relaunchSuccess = simulateCancelAndBack({ navigateFails: true })
assert.equal(relaunchSuccess.relaunchAttempts, 1)
assert.equal(relaunchSuccess.interactiveDuringRelaunch, false)
assert.equal(relaunchSuccess.leaving, true)
assert.equal(relaunchSuccess.reloads, 0)

const totalFailure = simulateCancelAndBack({ navigateFails: true, relaunchFails: true })
assert.equal(totalFailure.interactiveDuringRelaunch, false)
assert.equal(totalFailure.leaving, false)
assert.equal(totalFailure.loading, true)
assert.equal(totalFailure.loadSequence, 3)
assert.equal(totalFailure.reloads, 1)

const newOperation = simulateCancelAndBack({ navigateFails: true, relaunchFails: true, startNewOperation: true })
assert.equal(newOperation.uploadLock, true)
assert.equal(newOperation.uploading, true)
assert.equal(newOperation.generation, 9)

console.log('identity application contract: ok')
