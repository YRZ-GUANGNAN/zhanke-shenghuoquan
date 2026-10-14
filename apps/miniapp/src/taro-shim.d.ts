declare module '@tarojs/taro' {
  export function nextTick(callback: () => void): void
  export function useDidShow(callback: () => void): void
  export function useDidHide(callback: () => void): void
  export function useResize(callback: () => void): void
  export function useShareAppMessage(callback: (payload: { from?: string; target?: unknown; webViewUrl?: string }) => { title?: string; path?: string; imageUrl?: string }): void
  export function getCurrentInstance(): CurrentInstance
  export function showShareMenu(options?: { menus?: Array<'shareAppMessage' | 'shareTimeline'> }): Promise<unknown>
  export function hideShareMenu(options?: { menus?: Array<'shareAppMessage' | 'shareTimeline'> }): Promise<unknown>
  export function useRouter(): { path: string; $taroPath?: string; params: Record<string, string | undefined> }

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

  interface Router {
    path?: string
    $taroPath?: string
    params: Record<string, string | undefined>
  }

  interface CurrentInstance {
    router: Router
  }

  interface MenuButtonRect {
    top: number
    right: number
    bottom: number
    left: number
    width: number
    height: number
  }

  interface WindowInfo {
    windowWidth: number
    windowHeight?: number
    screenHeight?: number
    statusBarHeight: number
    pixelRatio: number
    safeArea?: {
      top: number
      right: number
      bottom: number
      left: number
      width: number
      height: number
    }
  }

  interface TaroStatic {
    navigateTo(options: NavigateOptions): Promise<unknown>
    redirectTo(options: NavigateOptions): Promise<unknown>
    reLaunch(options: NavigateOptions): Promise<unknown>
    navigateBack(options?: { delta?: number }): Promise<unknown>
    showToast(options: ToastOptions): Promise<unknown>
    login(options?: { timeout?: number }): Promise<{ code: string; errMsg?: string }>
    request<T = unknown>(options: RequestOptions): Promise<RequestResult<T>>
    getStorageSync<T = unknown>(key: string): T
    setStorageSync<T = unknown>(key: string, value: T): void
    removeStorageSync(key: string): void
    getCurrentInstance(): CurrentInstance
    showShareMenu(options?: { menus?: Array<'shareAppMessage' | 'shareTimeline'> }): Promise<unknown>
    hideShareMenu(options?: { menus?: Array<'shareAppMessage' | 'shareTimeline'> }): Promise<unknown>
    chooseMedia(options: { count: number; mediaType: ['image']; sourceType: Array<'album' | 'camera'>; sizeType?: Array<'original' | 'compressed'> }): Promise<{ tempFiles: Array<{ tempFilePath: string; size: number; fileType?: string }> }>
    previewImage(options: { urls: string[]; current?: string }): Promise<unknown>
    getImageInfo(options: { src: string }): Promise<{ width: number; height: number; type: string; path: string }>
    compressImage(options: { src: string; quality: number; compressedWidth?: number; compressedHeight?: number; format?: 'webp' }): Promise<{ tempFilePath: string }>
    getFileInfo(options: { filePath: string; digestAlgorithm?: 'md5' | 'sha1' }): Promise<{ size: number; digest: string }>
    getFileSystemManager(): { readFileSync(filePath: string): ArrayBuffer | ArrayBufferView | string }
    getMenuButtonBoundingClientRect(): MenuButtonRect
    getWindowInfo(): WindowInfo
  }
}
