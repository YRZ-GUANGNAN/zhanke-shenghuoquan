const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const read = (...parts) => fs.readFileSync(path.join(root, ...parts), 'utf8')

const community = read('apps', 'miniapp', 'src', 'services', 'b-community.ts')
const media = read('apps', 'miniapp', 'src', 'services', 'media.ts')
const publishPost = read('apps', 'miniapp', 'src', 'pages', 'publish-post', 'index.tsx')
const publishEntry = read('apps', 'miniapp', 'src', 'pages', 'publish-entry', 'index.tsx')

if (!community.includes("import { createClientRequestId } from './client-request-id'")) {
  throw new Error('community service must use the shared Crockford ULID generator')
}
if (/community-\$\{Date\.now\(\)\}/.test(community)) {
  throw new Error('legacy non-ULID community request id is forbidden')
}
if (!/const clientRequestId = input\.clientRequestId \?\? createCommunityClientRequestId\(\)/.test(community)) {
  throw new Error('community create must resolve one request id before building the request')
}
if (!community.includes("header: { 'Idempotency-Key': clientRequestId }")) {
  throw new Error('community create must send the resolved id as Idempotency-Key')
}
if (!publishPost.includes('useRef') || !publishPost.includes('clientRequestIdRef')) {
  throw new Error('publish page must preserve a logical request id across retryable failures')
}
if (!publishPost.includes('isApiError(error)') || !publishPost.includes('error.status >= 400 && error.status < 500')) {
  throw new Error('publish page must discard a request id after a 4xx contract failure')
}
if (!/export async function waitForMediaApprovals\(/.test(media)) {
  throw new Error('media service must support bounded batch approval polling')
}
if (!/concurrency\s*=\s*3/.test(media) || !/Math\.max\(1,\s*Math\.min\(3,\s*concurrency\)\)/.test(media)) {
  throw new Error('media approval polling must cap concurrency at three')
}
for (const fragment of [
  'const SUBMIT_TIMEOUT_MS = 120000',
  'const APPROVAL_TIMEOUT_MS = 105000',
  'waitForMediaApprovals(',
  'Math.min(APPROVAL_TIMEOUT_MS, deadline - Date.now())',
  "setSubmitStage('reviewing')",
  "setSubmitStage('publishing')",
]) {
  if (!publishPost.includes(fragment)) throw new Error(`publish timeout contract is missing: ${fragment}`)
}
if (!/if \(deadline - Date\.now\(\) < 12000\)/.test(publishPost)) {
  throw new Error('post creation must not begin after the total publish deadline is exhausted')
}
for (const forbidden of ["key: 'delivery'", '发布配送单', '/pages/merchant-publish/index']) {
  if (publishEntry.includes(forbidden)) {
    throw new Error(`student publish entry contains forbidden delivery target: ${forbidden}`)
  }
}
for (const required of ["key: 'post'", "key: 'secondhand'", '/pages/publish-post/index', '/pages/publish-secondhand/index']) {
  if (!publishEntry.includes(required)) {
    throw new Error(`student publish entry is missing: ${required}`)
  }
}

console.log('miniapp publish contract check PASS')
