import TaroRuntime from '@tarojs/taro'

interface NavigateOptions {
  url: string
}

interface LoginResult {
  code: string
  errMsg?: string
}

interface ToastOptions {
  title: string
  icon?: 'success' | 'error' | 'loading' | 'none'
  duration?: number
}

interface ModalOptions {
	title: string
	content: string
	confirmText?: string
	cancelText?: string
	confirmColor?: string
	showCancel?: boolean
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
  reLaunch(options: NavigateOptions): Promise<unknown>
  navigateBack(options?: { delta?: number }): Promise<unknown>
  showToast(options: ToastOptions): Promise<unknown>
	showModal(options: ModalOptions): Promise<{ confirm: boolean; cancel: boolean }>
  login(options?: { timeout?: number }): Promise<LoginResult>
  request<T = unknown>(options: RequestOptions): Promise<RequestResult<T>>
	getStorageSync<T = unknown>(key: string): T
	setStorageSync<T = unknown>(key: string, value: T): void
	removeStorageSync(key: string): void
	navigateToMiniProgram(options: { appId: string; path: string }): Promise<unknown>
  getCurrentInstance(): { router?: { path?: string; params: Record<string, string | undefined> } }
  chooseMedia(options: { count: number; mediaType: ['image']; sourceType: Array<'album' | 'camera'>; sizeType?: Array<'original' | 'compressed'> }): Promise<{ tempFiles: Array<{ tempFilePath: string; size: number; fileType?: string }> }>
  previewImage(options: { urls: string[]; current?: string }): Promise<unknown>
  getImageInfo(options: { src: string }): Promise<{ width: number; height: number; type: string; path: string }>
  compressImage(options: { src: string; quality: number; compressedWidth?: number; compressedHeight?: number; format?: 'webp' }): Promise<{ tempFilePath: string }>
  getFileInfo(options: { filePath: string; digestAlgorithm?: 'md5' | 'sha1' }): Promise<{ size: number; digest: string }>
  getFileSystemManager(): { readFileSync(filePath: string): ArrayBuffer | ArrayBufferView | string }
	requestPayment(options: RequestPaymentOptions): Promise<unknown>
	getUpdateManager(): {
		onCheckForUpdate(callback: (value: { hasUpdate: boolean }) => void): void
		onUpdateReady(callback: () => void): void
		onUpdateFailed(callback: () => void): void
		applyUpdate(): void
	}
	getAccountInfoSync(): { miniProgram: { appId: string; envVersion: string; version: string } }
}

export default TaroRuntime as unknown as TaroFacade
export type { RequestOptions }
