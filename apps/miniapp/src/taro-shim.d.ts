declare module '@tarojs/taro' {
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
    params: Record<string, string | undefined>
  }

  interface CurrentInstance {
    router: Router
  }

  interface TaroStatic {
    navigateTo(options: NavigateOptions): Promise<unknown>
    redirectTo(options: NavigateOptions): Promise<unknown>
    navigateBack(options?: { delta?: number }): Promise<unknown>
    showToast(options: ToastOptions): Promise<unknown>
    request<T = unknown>(options: RequestOptions): Promise<RequestResult<T>>
    getCurrentInstance(): CurrentInstance
    chooseMedia(options: { count: number; mediaType: ['image']; sourceType: Array<'album' | 'camera'>; sizeType?: Array<'original' | 'compressed'> }): Promise<{ tempFiles: Array<{ tempFilePath: string; size: number; fileType?: string }> }>
    getImageInfo(options: { src: string }): Promise<{ width: number; height: number; type: string; path: string }>
    compressImage(options: { src: string; quality: number; compressedWidth?: number; compressedHeight?: number; format?: 'webp' }): Promise<{ tempFilePath: string }>
    getFileInfo(options: { filePath: string; digestAlgorithm?: 'md5' | 'sha1' }): Promise<{ size: number; digest: string }>
  }
}

declare module 'cos-wx-sdk-v5' {
  interface Authorization {
    TmpSecretId: string
    TmpSecretKey: string
    SecurityToken: string
    StartTime: number
    ExpiredTime: number
  }
  interface COSOptions {
    SimpleUploadMethod?: 'putObject'
    getAuthorization(options: { Bucket: string; Region: string; Key: string }, callback: (value: Authorization) => void): void
  }
  interface PutObjectOptions {
    Bucket: string
    Region: string
    Key: string
    FilePath: string
    ContentType: string
    onProgress?: (info: { percent?: number }) => void
  }
  export default class COS {
    constructor(options: COSOptions)
    putObject(options: PutObjectOptions, callback: (error?: Error | null, data?: unknown) => void): void
  }
}
