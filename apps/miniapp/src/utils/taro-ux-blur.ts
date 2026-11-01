// Port: nanxiaobei/taro-ux-kits @ f85a5d714639006a9c3f12c67838087a30255c0b
// blur-loading/BlurLoading.jsx. ScrollView owns gesture/request triggering here.
export const UX_BLUR_START = 40
export const UX_BLUR_END = 100
export const UX_BLUR_LOADING = 60

export function taroUxBlurStyle(distance: number) {
  if (!Number.isFinite(distance) || distance < UX_BLUR_START) return undefined
  return { '--blur': `blur(${Math.floor((distance - UX_BLUR_START) / 3)}px)` }
}
