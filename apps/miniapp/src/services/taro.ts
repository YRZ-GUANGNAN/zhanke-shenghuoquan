import TaroRuntime from '@tarojs/taro'

interface NavigateOptions {
  url: string
}

interface ToastOptions {
  title: string
  icon?: 'success' | 'error' | 'loading' | 'none'
  duration?: number
}

interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  data?: unknown
  header?: Record<string, string>
  timeout?: number
}

interface RequestResult<T = unknown> {
  statusCode: number
  data: T
}

interface RequestPaymentOptions {
  timeStamp: string
  nonceStr: string
  package: string
  signType: string
  paySign: string
}

interface TaroFacade {
  navigateTo(options: NavigateOptions): Promise<unknown>
  redirectTo(options: NavigateOptions): Promise<unknown>
  navigateBack(options?: { delta?: number }): Promise<unknown>
  showToast(options: ToastOptions): Promise<unknown>
  request<T = unknown>(options: RequestOptions): Promise<RequestResult<T>>
	getStorageSync<T = unknown>(key: string): T
	setStorageSync<T = unknown>(key: string, value: T): void
	navigateToMiniProgram(options: { appId: string; path: string }): Promise<unknown>
  getCurrentInstance(): { router?: { params: Record<string, string | undefined> } }
  chooseMedia(options: { count: number; mediaType: ['image']; sourceType: Array<'album' | 'camera'>; sizeType?: Array<'original' | 'compressed'> }): Promise<{ tempFiles: Array<{ tempFilePath: string; size: number; fileType?: string }> }>
  getImageInfo(options: { src: string }): Promise<{ width: number; height: number; type: string; path: string }>
  compressImage(options: { src: string; quality: number; compressedWidth?: number; compressedHeight?: number; format?: 'webp' }): Promise<{ tempFilePath: string }>
  getFileInfo(options: { filePath: string; digestAlgorithm?: 'md5' | 'sha1' }): Promise<{ size: number; digest: string }>
  getFileSystemManager(): { readFileSync(filePath: string): ArrayBuffer | ArrayBufferView | string }
  requestPayment(options: RequestPaymentOptions): Promise<unknown>
}

export default TaroRuntime as unknown as TaroFacade
export type { RequestOptions }
