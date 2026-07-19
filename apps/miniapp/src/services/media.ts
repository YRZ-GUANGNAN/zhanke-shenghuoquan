import Taro from './taro'
import { request } from './api'

export type MediaPurpose = 'post_image' | 'product_image' | 'avatar' | 'dish' | 'merchant_logo' | 'delivery_proof' | 'delivery_im' | 'feedback' | 'identity_application'
export type MediaClientState =
  | 'waiting_upload'
  | 'processing'
  | 'review_pending'
  | 'approved'
  | 'rejected'
  | 'review_failed'
  | 'expired'
  | 'upload_failed'
  | 'deleted'

export interface MediaStatus {
  assetId: string
  ticketId: string
  purpose: 'post' | 'product' | 'avatar' | 'dish' | 'merchant_logo' | 'delivery_proof' | 'delivery_im' | 'feedback' | 'identity_application'
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
  onProgress?: (value: UploadProgress) => void
): Promise<UploadedMedia[]> {
  const selected = await Taro.chooseMedia({ count, mediaType: ['image'], sourceType: ['album', 'camera'], sizeType: ['original'] })
  return mapConcurrent(selected.tempFiles, 3, async (file, index) =>
    uploadImage(file.tempFilePath, purpose, (percent, stage, message) =>
      onProgress?.({ index, total: selected.tempFiles.length, percent, stage, message })
    )
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

export async function retryUploadCompletion(ticketId: string): Promise<MediaStatus> {
  return request<MediaStatus>(`/miniapp/media/uploads/${ticketId}/complete`, {
    url: '', method: 'POST', timeout: 120000
  })
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

async function uploadImage(
  sourcePath: string,
  purpose: MediaPurpose,
  progress: (percent: number, stage: UploadProgress['stage'], message: string) => void
): Promise<UploadedMedia> {
  progress(5, 'compressing', '正在压缩图片')
  const prepared = await prepareImage(sourcePath, purpose)
  const file = await Taro.getFileInfo({ filePath: prepared.path, digestAlgorithm: 'md5' })
  if (purpose === 'delivery_proof' && file.size > DELIVERY_PROOF_MAX_BYTES) {
    throw new Error('凭证照片压缩后仍超过 5 MiB，请重新拍摄')
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

  progress(35, 'uploading', '正在直传腾讯云 COS')
  await putSignedObject(session, bytes)
  progress(85, 'completing', '正在校验图片并提交审核')
  const status = await retryUploadCompletion(session.ticketId)
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

function domainPurpose(value: MediaPurpose): 'post' | 'product' | 'avatar' | 'dish' | 'merchant_logo' | 'delivery_proof' | 'delivery_im' | 'feedback' | 'identity_application' {
  if (value === 'post_image') return 'post'
  if (value === 'product_image') return 'product'
  if (value === 'dish') return 'dish'
  if (value === 'merchant_logo') return 'merchant_logo'
  if (value === 'delivery_proof') return 'delivery_proof'
  if (value === 'delivery_im') return 'delivery_im'
  if (value === 'feedback') return 'feedback'
  if (value === 'identity_application') return 'identity_application'
  return 'avatar'
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
