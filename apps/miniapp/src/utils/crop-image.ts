// 微信原生图片编辑（wx.editImage）：进入后可裁剪/旋转/涂鸦。
// Taro 4.2 的 weapp 运行时对 editImage 的透传不稳定，这里直接访问小程序全局 wx，
// 并在能力缺失或环境不支持时显式回退为原图（由调用方提示用户）。
type WxEditImage = (options: {
  src: string
  success: (result: { tempFilePath: string }) => void
  fail: (error: unknown) => void
}) => void

export type CropChoice =
  | { status: 'edited'; path: string }
  | { status: 'unchanged'; path: string }
  | { status: 'cancelled'; path: '' }

export async function cropImageWithWechatEditor(src: string): Promise<CropChoice> {
  const editor = (globalThis as { wx?: { editImage?: WxEditImage } }).wx?.editImage
  if (typeof editor !== 'function') return { status: 'unchanged', path: src }
  try {
    const result = await new Promise<{ tempFilePath: string }>((resolve, reject) => {
      try {
        editor({ src, success: resolve, fail: reject })
      } catch (error) {
        reject(error)
      }
    })
    return { status: 'edited', path: result.tempFilePath || src }
  } catch (error) {
    const message = String((error as { errMsg?: string })?.errMsg || (error as Error)?.message || '')
    if (message.includes('cancel')) return { status: 'cancelled', path: '' }
    return { status: 'unchanged', path: src }
  }
}
