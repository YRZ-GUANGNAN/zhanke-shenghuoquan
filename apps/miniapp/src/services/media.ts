import COS from 'cos-wx-sdk-v5'
import Taro from './taro'
import { request } from './api'

export type MediaPurpose = 'post_image' | 'product_image' | 'avatar'
type MediaStatus = 'pending_upload' | 'uploaded' | 'submitted' | 'reviewing' | 'approved' | 'rejected' | 'deleted'

interface UploadTicket {
  assetId: string
  bucket: string
  region: string
  key: string
  mimeType: string
  maxBytes: number
  credentials: {
    tmpSecretId: string
    tmpSecretKey: string
    sessionToken: string
    startTime: number
    expiredTime: number
  }
}

interface MediaAsset {
  id: string
  status: MediaStatus
  error?: string
}

export interface UploadedMedia {
  assetId: string
  previewPath: string
}

const MAX_EDGE: Record<MediaPurpose, number> = { post_image: 1920, product_image: 1920, avatar: 1024 }

export async function chooseAndUploadImages(purpose: MediaPurpose, count: number): Promise<UploadedMedia[]> {
  const selected = await Taro.chooseMedia({ count, mediaType: ['image'], sourceType: ['album', 'camera'], sizeType: ['original'] })
  return mapConcurrent(selected.tempFiles, 3, async (file) => uploadImage(file.tempFilePath, purpose))
}

export async function cancelMedia(assetId: string): Promise<void> {
  await request(`/media/uploads/${assetId}`, { url: '', method: 'DELETE', timeout: 10000 })
}

async function uploadImage(sourcePath: string, purpose: MediaPurpose): Promise<UploadedMedia> {
  const prepared = await prepareImage(sourcePath, purpose)
  const file = await Taro.getFileInfo({ filePath: prepared.path, digestAlgorithm: 'md5' })
  const ticket = await request<UploadTicket>('/media/uploads', {
    url: '', method: 'POST', timeout: 10000,
    data: {
      purpose,
      fileName: `upload.${extensionFor(prepared.mimeType)}`,
      mimeType: prepared.mimeType,
      sizeBytes: file.size,
      idempotencyKey: `${purpose}:${file.digest}:${file.size}`
    }
  })
  await putObject(ticket, prepared.path)
  await request<MediaAsset>(`/media/uploads/${ticket.assetId}/complete`, { url: '', method: 'POST', timeout: 15000 })
  const finalAsset = await waitForReview(ticket.assetId)
  if (finalAsset.status !== 'approved') throw new Error(finalAsset.error || '图片未通过安全审核')
  return { assetId: ticket.assetId, previewPath: prepared.path }
}

async function prepareImage(sourcePath: string, purpose: MediaPurpose): Promise<{ path: string; mimeType: string }> {
  const info = await Taro.getImageInfo({ src: sourcePath })
  const maxEdge = MAX_EDGE[purpose]
  const ratio = Math.min(1, maxEdge / Math.max(info.width, info.height))
  const width = Math.max(1, Math.round(info.width * ratio))
  const height = Math.max(1, Math.round(info.height * ratio))
  let result: { tempFilePath: string }
  try {
    // Newer WeChat runtimes may honor WebP output; older runtimes safely fall back to their native compressed format.
    result = await Taro.compressImage({ src: sourcePath, quality: 78, compressedWidth: width, compressedHeight: height, format: 'webp' })
  } catch {
    result = await Taro.compressImage({ src: sourcePath, quality: 78, compressedWidth: width, compressedHeight: height })
  }
  const compressedInfo = await Taro.getImageInfo({ src: result.tempFilePath })
  const mimeType = mimeFor(compressedInfo.type)
  if (!mimeType) throw new Error('仅支持 JPEG、PNG 或 WebP 图片')
  return { path: result.tempFilePath, mimeType }
}

function putObject(ticket: UploadTicket, filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const cos = new COS({
      SimpleUploadMethod: 'putObject',
      getAuthorization: (_options, callback) => callback({
        TmpSecretId: ticket.credentials.tmpSecretId,
        TmpSecretKey: ticket.credentials.tmpSecretKey,
        SecurityToken: ticket.credentials.sessionToken,
        StartTime: ticket.credentials.startTime,
        ExpiredTime: ticket.credentials.expiredTime
      })
    })
    cos.putObject({ Bucket: ticket.bucket, Region: ticket.region, Key: ticket.key, FilePath: filePath, ContentType: ticket.mimeType }, (error) => {
      if (error) reject(new Error(`COS 上传失败：${error.message}`))
      else resolve()
    })
  })
}

async function waitForReview(assetId: string): Promise<MediaAsset> {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    const value = await request<MediaAsset>(`/media/uploads/${assetId}`, { url: '', timeout: 10000 })
    if (value.status === 'approved' || value.status === 'rejected' || value.status === 'deleted') return value
    await delay(1500)
  }
  throw new Error('图片审核仍在处理中，请稍后重试')
}

function mimeFor(type: string): string {
  const normalized = type.toLowerCase().replace(/^image\//, '')
  if (normalized === 'jpg' || normalized === 'jpeg') return 'image/jpeg'
  if (normalized === 'png') return 'image/png'
  if (normalized === 'webp') return 'image/webp'
  return ''
}

function extensionFor(mimeType: string): string {
  if (mimeType === 'image/png') return 'png'
  if (mimeType === 'image/webp') return 'webp'
  return 'jpg'
}

async function mapConcurrent<T, R>(values: T[], concurrency: number, mapper: (value: T) => Promise<R>): Promise<R[]> {
  const result = new Array<R>(values.length)
  let next = 0
  async function worker() {
    while (next < values.length) {
      const index = next
      next += 1
      result[index] = await mapper(values[index])
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, values.length) }, worker))
  return result
}

function delay(ms: number): Promise<void> { return new Promise((resolve) => setTimeout(resolve, ms)) }
