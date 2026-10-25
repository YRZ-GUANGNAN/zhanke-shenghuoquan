import Taro from './taro'
import { isApiError, request } from './api'

export type MediaPurpose = 'post_image' | 'product_image' | 'avatar' | 'profile_cover' | 'dish' | 'merchant_logo' | 'delivery_proof' | 'delivery_im' | 'feedback' | 'identity_application'
export type MediaClientState =
  | 'waiting_upload'
  | 'processing'
  | 'review_pending'
  | 'approved'
  | 'ready'
  | 'rejected'
  | 'review_failed'
  | 'expired'
  | 'upload_failed'
  | 'deleted'

export interface MediaStatus {
  assetId: string
  ticketId: string
  purpose: 'post' | 'product' | 'avatar' | 'profile_cover' | 'dish' | 'merchant_logo' | 'delivery_proof' | 'delivery_im' | 'feedback' | 'identity_application'
  state: MediaClientState
  technicalStatus: string
  reviewStatus: string
  ticketStatus: string
  message: string
  recovery: 'none' | 'poll' | 'new_upload' | 'complete_upload'
  canUse: boolean
  updatedAt: string
}

interface UploadSession {
  ticketId: string
  assetId: string
  uploadUrl: string
  requiredHeaders: {
    contentType: string
    contentMD5: string
    contentLength: number
    forbidOverwrite: string
  }
  expiresAt: string
  state: MediaClientState
  message: string
  recovery: 'none' | 'poll' | 'new_upload' | 'complete_upload'
}

export interface UploadedMedia extends MediaStatus {
  ticketId: string
  previewPath: string
}

export interface UploadedPostMotion {
  kind: 'video' | 'live_photo'
  poster: UploadedMedia
  video: UploadedMedia
  durationMs: number
}

export async function postMotionAvailable(): Promise<boolean> {
  try { return (await request<{ postMotion: boolean }>('/miniapp/media/capabilities')).postMotion === true }
  catch { return false }
}

type SelectedMediaFile = Awaited<ReturnType<typeof Taro.chooseMedia>>['tempFiles'][number]

export type UploadedPostSelection =
  | { kind: 'images'; images: UploadedMedia[] }
  | { kind: 'motion'; motion: UploadedPostMotion }

/** A single picker, with the entire selection checked before any upload starts. */
export async function chooseAndUploadPostMedia(
  count: number, allowMotion: boolean,
  onProgress: (message: string) => void, shouldContinue: () => boolean
): Promise<UploadedPostSelection> {
  assertUploadActive(shouldContinue)
  if (!Number.isInteger(count) || count < 1 || count > 9) throw new Error('最多选择 9 张图片')
  const selected = await Taro.chooseMedia({
    count, mediaType: allowMotion ? ['image', 'video'] : ['image'],
    sourceType: ['album', 'camera'], sizeType: ['original'], maxDuration: 60
  })
  assertUploadActive(shouldContinue)
  const files = selected.tempFiles
  if (!files.length) return { kind: 'images', images: [] }
  if (files.length > count) throw new Error(`最多再选择 ${count} 张图片`)
  const kinds = files.map(file => file.fileType || (!allowMotion ? 'image' : ''))
  if (kinds.some(kind => kind !== 'image' && kind !== 'video') || files.some(file => !file.tempFilePath)) {
    throw new Error('暂时无法识别此媒体，请重新选择照片或视频')
  }
  if (kinds.includes('video')) {
    if (!allowMotion) throw new Error('当前只能添加图片')
    if (files.length !== 1) throw new Error('请选择最多 9 张图片或 1 条视频，不能混合添加')
    return { kind: 'motion', motion: await uploadPostMotionClip('video', files[0], '', onProgress, shouldContinue) }
  }
  const images = await mapConcurrent(files, 3, (file, index) =>
    uploadImage(file.tempFilePath, 'post_image', (_percent, _stage, message) => {
      onProgress(`图片 ${index + 1}/${files.length}：${message}`)
    }, shouldContinue)
  )
  assertUploadActive(shouldContinue)
  return { kind: 'images', images }
}

/** Native Live Photo pairing is not exposed: attach an explicitly selected short clip. */
export async function chooseAndAttachPostLiveClip(
  poster: UploadedMedia, onProgress: (message: string) => void, shouldContinue: () => boolean
): Promise<UploadedPostMotion> {
  assertUploadActive(shouldContinue)
  if (poster.purpose !== 'post' || !poster.assetId || !poster.previewPath) throw new Error('请先选择一张帖子图片')
  const selected = await Taro.chooseMedia({ count: 1, mediaType: ['video'], sourceType: ['album'], maxDuration: 5, sizeType: ['compressed'] })
  assertUploadActive(shouldContinue)
  if (selected.tempFiles.length !== 1) throw new Error('请选择一个对应的动态片段')
  return uploadPostMotionClip('live_photo', selected.tempFiles[0], poster, onProgress, shouldContinue)
}

export async function chooseAndUploadPostMotion(
  kind: UploadedPostMotion['kind'], onProgress: (message: string) => void, shouldContinue: () => boolean
): Promise<UploadedPostMotion> {
  assertUploadActive(shouldContinue)
  let posterPath = ''
  if (kind === 'live_photo') {
    const photo = await Taro.chooseMedia({ count: 1, mediaType: ['image'], sourceType: ['album'], sizeType: ['original'] })
    assertUploadActive(shouldContinue)
    posterPath = photo.tempFiles[0]?.tempFilePath ?? ''
    if (!posterPath) throw new Error('请选择实况照片')
  }
  const selected = await Taro.chooseMedia({ count: 1, mediaType: ['video'], sourceType: ['album', 'camera'], maxDuration: kind === 'live_photo' ? 5 : 60, sizeType: ['compressed'] })
  assertUploadActive(shouldContinue)
  if (selected.tempFiles.length !== 1) throw new Error('请选择一个视频片段')
  return uploadPostMotionClip(kind, selected.tempFiles[0], posterPath, onProgress, shouldContinue)
}

async function uploadPostMotionClip(
  kind: UploadedPostMotion['kind'], clip: SelectedMediaFile, existingPoster: string | UploadedMedia,
  onProgress: (message: string) => void, shouldContinue: () => boolean
): Promise<UploadedPostMotion> {
  assertUploadActive(shouldContinue)
  if (!clip || (clip.fileType && clip.fileType !== 'video')) throw new Error('请选择视频片段')
  const seconds = clip?.duration ?? 0
  if (!clip || !Number.isFinite(seconds) || seconds < .5 || seconds > (kind === 'live_photo' ? 5 : 60)) {
    throw new Error(kind === 'live_photo' ? '请选择 0.5～5 秒的对应实况片段' : '请选择 0.5～60 秒的视频')
  }
  if (clip.size > 30 * 1024 * 1024) throw new Error('视频不能超过 30 MB，请先压缩或裁剪')
  // Validate before creating either upload; extension alone is not trusted.
  const info = await Taro.getFileInfo({ filePath: clip.tempFilePath, digestAlgorithm: 'md5' })
  assertUploadActive(shouldContinue)
  if (info.size <= 0 || info.size > 30 * 1024 * 1024) throw new Error('视频不能超过 30 MB')
  const bytes = readFileBytes(clip.tempFilePath)
  if (bytes.byteLength !== info.size) throw new Error('视频读取失败，请重新选择')
  const header = new Uint8Array(bytes, 0, Math.min(12, bytes.byteLength))
  if (header.length < 12 || String.fromCharCode(...header.slice(4, 8)) !== 'ftyp') throw new Error('请先将视频导出为 MP4 格式')
  let posterPath = typeof existingPoster === 'string' ? existingPoster || clip.thumbTempFilePath || '' : existingPoster.previewPath
  if (!posterPath) {
    onProgress('请选择视频封面')
    const cover = await Taro.chooseMedia({ count: 1, mediaType: ['image'], sourceType: ['album'], sizeType: ['original'] })
    assertUploadActive(shouldContinue)
    posterPath = cover.tempFiles[0]?.tempFilePath ?? ''
  }
  if (!posterPath) throw new Error('请为视频选择封面')
  const poster = typeof existingPoster === 'string'
    ? await uploadImage(posterPath, 'post_image', (_p, _s, message) => onProgress(`封面：${message}`), shouldContinue)
    : existingPoster
  assertUploadActive(shouldContinue)
  onProgress('正在上传视频')
  const session = await request<UploadSession>('/miniapp/media/uploads', {
    url: '', method: 'POST', timeout: 20000,
    data: { purpose: 'post', sizeBytes: info.size, contentMD5: hexMD5ToBase64(info.digest), contentType: 'video/mp4', idempotencyKey: newAttemptKey('post_image', info.digest, info.size) }
  })
  assertUploadActive(shouldContinue)
  await putSignedObject(session, bytes)
  assertUploadActive(shouldContinue)
  onProgress('正在校验视频并提交自动审核')
  let status: MediaStatus
  try { status = await retryUploadCompletion(session.ticketId) }
  catch (error) {
    if (isApiError(error) && error.status >= 400 && error.status < 500 && ![408, 409, 429].includes(error.status)) throw error
    // Keep the immutable ticket when completion or cloud submission times out.
    // A later publish retries completion; never upload the same bytes twice.
    status = { assetId: session.assetId, ticketId: session.ticketId, purpose: 'post', state: 'processing', technicalStatus: 'validating', reviewStatus: 'pending', ticketStatus: 'uploaded', message: '视频已上传，待继续确认', recovery: 'complete_upload', canUse: false, updatedAt: new Date().toISOString() }
  }
  assertUploadActive(shouldContinue)
  return { kind, poster, video: { ...status, previewPath: clip.tempFilePath }, durationMs: Math.round(seconds * 1000) }
}

export async function confirmPostMotion(
  motion: UploadedPostMotion, timeoutMs = 105000,
  onProgress?: (message: string) => void, shouldContinue?: () => boolean
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  const active = () => { assertUploadActive(shouldContinue); if (Date.now() >= deadline) throw new Error('媒体仍在处理中，内容已保留，请稍后再发布') }
  active()
  // Only retry unfinished technical completion. Confirmed files retain their
  // immutable asset identity while the durable review job is polled.
  await Promise.all([motion.poster, motion.video].map(async asset => {
    if (asset.technicalStatus !== 'final_ready' || asset.recovery === 'complete_upload') {
      await retryUploadCompletion(asset.ticketId, Math.max(1, Math.min(30000, deadline - Date.now())))
    }
  }))
  while (true) {
    active()
    const statuses = await Promise.all([motion.poster.assetId, motion.video.assetId].map(getMediaStatus))
    active()
    if (statuses.every(status => status.canUse && status.state === 'approved')) return
    const failed = statuses.find(status => ['rejected', 'review_failed', 'expired', 'upload_failed', 'deleted'].includes(status.state))
    if (failed) throw new Error(failed.message || '媒体处理失败，请更换后重试')
    onProgress?.('视频与封面审核中，完成后自动发布')
    await delay(Math.min(1500, Math.max(0, deadline - Date.now())))
  }
}

export type ImageUploadSettlement =
  | { index: number; status: 'fulfilled'; value: UploadedMedia }
  | { index: number; status: 'rejected'; reason: Error }

export interface MediaHistory {
  items: MediaStatus[]
  page: number
  pageSize: number
  total: number
}

export interface UploadProgress {
  index: number
  total: number
  percent: number
  stage: 'compressing' | 'authorizing' | 'uploading' | 'completing' | 'done'
  message: string
}

const MAX_EDGE: Record<MediaPurpose, number> = {
  post_image: 1920,
  product_image: 1920,
  avatar: 1024,
  profile_cover: 1920,
  dish: 1920,
  merchant_logo: 1024,
  delivery_proof: 1920,
  delivery_im: 1920,
  feedback: 1920,
  identity_application: 1920
}
const DELIVERY_PROOF_MAX_BYTES = 5 * 1024 * 1024

export async function chooseAndUploadImages(
  purpose: MediaPurpose,
  count: number,
  onProgress?: (value: UploadProgress) => void,
  shouldContinue?: () => boolean
): Promise<UploadedMedia[]> {
  const selected = await Taro.chooseMedia({ count, mediaType: ['image'], sourceType: ['album', 'camera'], sizeType: ['original'] })
  assertUploadActive(shouldContinue)
  return mapConcurrent(selected.tempFiles, 3, async (file, index) =>
    uploadImage(file.tempFilePath, purpose, (percent, stage, message) => {
      onProgress?.({ index, total: selected.tempFiles.length, percent, stage, message })
    }, shouldContinue)
  )
}

export async function chooseAndUploadImagesSettled(
  purpose: MediaPurpose,
  count: number,
  onProgress?: (value: UploadProgress) => void
): Promise<ImageUploadSettlement[]> {
  const selected = await Taro.chooseMedia({ count, mediaType: ['image'], sourceType: ['album', 'camera'], sizeType: ['original'] })
  return mapConcurrent(selected.tempFiles, 3, async (file, index) => {
    try {
      const value = await uploadImage(file.tempFilePath, purpose, (percent, stage, message) =>
        onProgress?.({ index, total: selected.tempFiles.length, percent, stage, message })
      )
      return { index, status: 'fulfilled', value } as const
    } catch (reason) {
      return {
        index,
        status: 'rejected',
        reason: reason instanceof Error ? reason : new Error('图片上传失败')
      } as const
    }
  })
}

export async function getMediaStatus(assetId: string): Promise<MediaStatus> {
  return request<MediaStatus>(`/miniapp/media/assets/${assetId}`, { url: '', timeout: 10000 })
}

export async function getMediaHistory(page = 1, pageSize = 20): Promise<MediaHistory> {
  return request<MediaHistory>(`/miniapp/media/uploads?page=${page}&pageSize=${pageSize}`, { url: '', timeout: 10000 })
}

export async function retryUploadCompletion(ticketId: string, timeout = 120000): Promise<MediaStatus> {
  return request<MediaStatus>(`/miniapp/media/uploads/${ticketId}/complete`, {
    url: '', method: 'POST', timeout
  })
}

export function isMediaUsable(status: MediaStatus): boolean {
  if (!status.canUse) return false
  if (status.state === 'approved') return true
  return status.state === 'ready' && status.reviewStatus === 'not_required' &&
    ['avatar', 'profile_cover', 'identity_application', 'dish', 'merchant_logo'].includes(status.purpose)
}

// Direct-use uploads finish their technical checks in the completion request.
// Do not start a 90-second content-review spinner for profile or takeout media.
export async function confirmDirectUpload(uploaded: UploadedMedia, shouldContinue?: () => boolean): Promise<MediaStatus> {
  assertUploadActive(shouldContinue)
  if (!['avatar', 'profile_cover', 'identity_application', 'dish', 'merchant_logo'].includes(uploaded.purpose)) {
    throw new Error('此类图片仍需内容审核')
  }
  const status = isMediaUsable(uploaded) ? uploaded : await getMediaStatus(uploaded.assetId)
  assertUploadActive(shouldContinue)
  if (status.assetId !== uploaded.assetId || status.purpose !== uploaded.purpose || !isMediaUsable(status)) {
    throw new Error(status.message || '图片尚不可用，请重新选择并完成上传')
  }
  return status
}

export async function waitForMediaApproval(
  assetId: string,
  onStatus?: (value: MediaStatus) => void,
  timeoutMs = 90000,
  shouldContinue?: () => boolean
): Promise<MediaStatus> {
  const deadline = Date.now() + timeoutMs
  while (true) {
    if (shouldContinue && !shouldContinue()) throw new Error('图片审核确认已取消')
    const status = await getMediaStatus(assetId)
    if (shouldContinue && !shouldContinue()) throw new Error('图片审核确认已取消')
    onStatus?.(status)
    if (status.canUse && status.state === 'approved') return status
    if (status.state === 'rejected') throw new Error('照片未通过内容安全审核，请重新拍摄')
    if (status.state === 'review_failed') throw new Error('照片审核失败，请重新拍摄')
    if (status.state === 'expired' || status.state === 'upload_failed' || status.state === 'deleted') {
      throw new Error(status.message || '照片处理失败，请重新拍摄')
    }
    if (Date.now() >= deadline) throw new Error('照片仍在审核处理中，请稍后重试')
    if (shouldContinue && !shouldContinue()) throw new Error('图片审核确认已取消')
    await delay(1500)
  }
}

export async function waitForMediaApprovals(
  assetIds: string[],
  onStatus?: (assetId: string, value: MediaStatus) => void,
  timeoutMs = 105000,
  concurrency = 3
): Promise<MediaStatus[]> {
  const deadline = Date.now() + timeoutMs
  const workerCount = Math.max(1, Math.min(3, concurrency))
  return mapConcurrent(assetIds, workerCount, async (assetId) => {
    const remaining = deadline - Date.now()
    if (remaining <= 0) throw new Error('图片审核等待超时，草稿和图片已保留')
    return waitForMediaApproval(
      assetId,
      (status) => onStatus?.(assetId, status),
      remaining,
      () => Date.now() < deadline
    )
  })
}

async function uploadImage(
  sourcePath: string,
  purpose: MediaPurpose,
  progress: (percent: number, stage: UploadProgress['stage'], message: string) => void,
  shouldContinue?: () => boolean
): Promise<UploadedMedia> {
  assertUploadActive(shouldContinue)
  progress(5, 'compressing', '正在压缩图片')
  const prepared = await prepareImage(sourcePath, purpose)
  const file = await Taro.getFileInfo({ filePath: prepared.path, digestAlgorithm: 'md5' })
  assertUploadActive(shouldContinue)
  if (purpose === 'delivery_proof' && file.size > DELIVERY_PROOF_MAX_BYTES) {
    throw new Error('凭证照片压缩后仍超过 5 MiB，请重新拍摄')
  }
  if (purpose === 'profile_cover' && file.size > 10 * 1024 * 1024) {
    throw new Error('封面压缩后仍超过 10 MB，请选择较小的照片')
  }
  const bytes = readFileBytes(prepared.path)
  if (bytes.byteLength !== file.size) throw new Error('压缩文件大小校验失败，请重新选择图片')

  progress(20, 'authorizing', '正在申请安全上传票据')
  const session = await request<UploadSession>('/miniapp/media/uploads', {
    url: '', method: 'POST', timeout: 20000,
    data: {
      purpose: domainPurpose(purpose),
      sizeBytes: file.size,
      contentMD5: hexMD5ToBase64(file.digest),
      contentType: prepared.mimeType,
      idempotencyKey: newAttemptKey(purpose, file.digest, file.size)
    }
  })

  assertUploadActive(shouldContinue)
  progress(35, 'uploading', '正在直传腾讯云 COS')
  await putSignedObject(session, bytes)
  assertUploadActive(shouldContinue)
  progress(85, 'completing', ['avatar', 'profile_cover', 'identity_application', 'dish', 'merchant_logo'].includes(purpose)
    ? '正在确认图片上传' : '正在校验图片并提交自动审核')
  const status = await retryUploadCompletion(session.ticketId)
  assertUploadActive(shouldContinue)
  progress(100, 'done', status.message)
  return { ...status, ticketId: session.ticketId, previewPath: prepared.path }
}

async function prepareImage(sourcePath: string, purpose: MediaPurpose): Promise<{ path: string; mimeType: string }> {
  const info = await Taro.getImageInfo({ src: sourcePath })
  const maxEdge = MAX_EDGE[purpose]
  const ratio = Math.min(1, maxEdge / Math.max(info.width, info.height))
  const width = Math.max(1, Math.round(info.width * ratio))
  const height = Math.max(1, Math.round(info.height * ratio))
  let result: { tempFilePath: string }
  try {
    result = await Taro.compressImage({ src: sourcePath, quality: 78, compressedWidth: width, compressedHeight: height, format: 'webp' })
  } catch {
    result = await Taro.compressImage({ src: sourcePath, quality: 78, compressedWidth: width, compressedHeight: height })
  }
  const compressed = await Taro.getImageInfo({ src: result.tempFilePath })
  const mimeType = mimeFor(compressed.type)
  if (!mimeType) throw new Error('仅支持 JPEG、PNG 或 WebP 图片')
  return { path: result.tempFilePath, mimeType }
}

async function putSignedObject(session: UploadSession, bytes: ArrayBuffer): Promise<void> {
  if (Date.parse(session.expiresAt) <= Date.now()) throw new Error('上传授权已过期，请重新选择图片')
  const headers = session.requiredHeaders
  const response = await Taro.request({
    url: session.uploadUrl,
    method: 'PUT',
    data: bytes,
    timeout: 120000,
    header: {
      'Content-Type': headers.contentType,
      'Content-MD5': headers.contentMD5,
      'Content-Length': String(headers.contentLength),
      'x-cos-forbid-overwrite': headers.forbidOverwrite
    }
  })
  if (response.statusCode < 200 || response.statusCode >= 300) {
    throw new Error(`COS 上传失败（${response.statusCode}），请重新上传`)
  }
}

function readFileBytes(path: string): ArrayBuffer {
  const value = Taro.getFileSystemManager().readFileSync(path)
  if (typeof value === 'string' || !Number.isSafeInteger(value.byteLength) || value.byteLength <= 0) {
    throw new Error('无法读取压缩后的图片')
  }

  let source: Uint8Array
  if (value instanceof ArrayBuffer) {
    source = new Uint8Array(value)
  } else if (ArrayBuffer.isView(value)) {
    source = new Uint8Array(value.buffer, value.byteOffset, value.byteLength)
  } else if (Object.prototype.toString.call(value) === '[object ArrayBuffer]') {
    try {
      source = new Uint8Array(value as ArrayBuffer)
    } catch {
      throw new Error('无法读取压缩后的图片')
    }
  } else {
    throw new Error('无法读取压缩后的图片')
  }

  if (source.byteLength !== value.byteLength) throw new Error('无法读取压缩后的图片')
  const copy = new Uint8Array(source.byteLength)
  copy.set(source)
  if (copy.byteLength !== value.byteLength) throw new Error('无法读取压缩后的图片')
  return copy.buffer
}

function domainPurpose(value: MediaPurpose): MediaStatus['purpose'] {
  if (value === 'post_image') return 'post'
  if (value === 'product_image') return 'product'
  if (value === 'profile_cover') return 'profile_cover'
  if (value === 'dish') return 'dish'
  if (value === 'merchant_logo') return 'merchant_logo'
  if (value === 'delivery_proof') return 'delivery_proof'
  if (value === 'delivery_im') return 'delivery_im'
  if (value === 'feedback') return 'feedback'
  if (value === 'identity_application') return 'identity_application'
  return 'avatar'
}

function assertUploadActive(shouldContinue?: () => boolean): void {
  if (shouldContinue && !shouldContinue()) throw new Error('图片上传已取消')
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds))
}

function mimeFor(type: string): string {
  const normalized = type.toLowerCase().replace(/^image\//, '')
  if (normalized === 'jpg' || normalized === 'jpeg') return 'image/jpeg'
  if (normalized === 'png') return 'image/png'
  if (normalized === 'webp') return 'image/webp'
  return ''
}

function hexMD5ToBase64(value: string): string {
  const normalized = value.trim().toLowerCase()
  if (!/^[0-9a-f]{32}$/.test(normalized)) throw new Error('图片 MD5 计算失败')
  const bytes = new Uint8Array(16)
  for (let index = 0; index < bytes.length; index += 1) {
    bytes[index] = Number.parseInt(normalized.slice(index * 2, index * 2 + 2), 16)
  }
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
  let result = ''
  for (let index = 0; index < bytes.length; index += 3) {
    const a = bytes[index]
    const b = bytes[index + 1]
    const c = bytes[index + 2]
    result += alphabet[a >> 2]
    result += alphabet[((a & 3) << 4) | ((b ?? 0) >> 4)]
    result += index + 1 < bytes.length ? alphabet[((b & 15) << 2) | ((c ?? 0) >> 6)] : '='
    result += index + 2 < bytes.length ? alphabet[c & 63] : '='
  }
  return result
}

function newAttemptKey(purpose: MediaPurpose, digest: string, size: number): string {
  return `${purpose}:${digest}:${size}:${Date.now().toString(36)}:${Math.random().toString(36).slice(2, 10)}`
}

async function mapConcurrent<T, R>(
  values: T[],
  concurrency: number,
  mapper: (value: T, index: number) => Promise<R>
): Promise<R[]> {
  const result = new Array<R>(values.length)
  let next = 0
  async function worker() {
    while (next < values.length) {
      const index = next
      next += 1
      result[index] = await mapper(values[index], index)
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, values.length) }, worker))
  return result
}
