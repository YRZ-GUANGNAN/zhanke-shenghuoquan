export interface GalleryViewport {
  width: number
  height: number
  headerHeight: number
  safeBottom: number
}

export interface MediaSize { width: number; height: number }

/** Independent Mini Program policy: logical window units, never Android screenshot pixels. */
export function galleryViewportHeight(
  media: MediaSize | undefined,
  viewport: GalleryViewport,
  mode: 'post' | 'product',
  actionbarRpx: number,
): number {
  const width = Number.isFinite(viewport.width) && viewport.width > 0 ? viewport.width : 375
  const height = Number.isFinite(viewport.height) && viewport.height > 0 ? viewport.height : 667
  const nonNegative = (value: number) => Number.isFinite(value) ? Math.max(0, value) : 0
  const available = Math.max(0, height - nonNegative(viewport.headerHeight) - nonNegative(viewport.safeBottom) - nonNegative(actionbarRpx) * width / 750)
  const ratio = mode === 'product' ? 1 : media && Number.isFinite(media.width) && Number.isFinite(media.height) && media.width > 0 && media.height > 0 ? media.width / media.height : 4 / 3
  const preferred = width / Math.min(2, Math.max(.75, ratio))
  // Reserve readable body space on short/landscape windows. No device-brand heuristics.
  const maximum = Math.max(0, available - Math.min(96, available * .3))
  return Math.floor(Math.min(preferred, maximum))
}

export const GALLERY_COUNTER_TIMING = { fadeMs: 200, holdMs: 5000 } as const
type Timer = ReturnType<typeof setTimeout>
interface CounterClock {
  set: (callback: () => void, delay: number) => Timer
  clear: (timer: Timer) => void
}

/** View state only. Business/media state does not depend on the counter timer. */
export function createGalleryCounter(
  onVisible: (visible: boolean) => void,
  clock: CounterClock = { set: (callback, delay) => setTimeout(callback, delay), clear: (timer) => clearTimeout(timer) },
) {
  let timer: Timer | undefined
  let generation = 0
  const clear = () => {
    generation += 1
    if (timer !== undefined) clock.clear(timer)
    timer = undefined
  }
  return {
    hold() { clear(); onVisible(true) },
    reveal() {
      clear()
      onVisible(true)
      const current = generation
      timer = clock.set(() => {
        if (current !== generation) return
        timer = undefined
        onVisible(false)
      }, GALLERY_COUNTER_TIMING.fadeMs + GALLERY_COUNTER_TIMING.holdMs)
    },
    hide() { clear(); onVisible(false) },
    dispose: clear,
  }
}
