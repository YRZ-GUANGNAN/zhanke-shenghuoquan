export interface PostMedia {
  src: string
  width: number
  height: number
}

export interface PostGalleryState {
  items: PostMedia[]
  index: number
}

/** A failed neighbour must not change the identity of the image being read. */
export function removeFailedPostMedia(gallery: PostGalleryState, failedSrc: string): PostGalleryState {
  const failedIndex = gallery.items.findIndex((item) => item.src === failedSrc)
  if (failedIndex < 0) return gallery
  const activeSrc = gallery.items[gallery.index]?.src
  const items = gallery.items.filter((item) => item.src !== failedSrc)
  const preservedIndex = items.findIndex((item) => item.src === activeSrc)
  const index = preservedIndex >= 0 ? preservedIndex : Math.max(0, Math.min(failedIndex, items.length - 1))
  return { items, index }
}

/** Check existing attachments without reordering them when requests finish out of order. */
export async function preparePostMedia(
  images: string[],
  getInfo: (src: string) => Promise<{ width: number; height: number }>,
): Promise<PostMedia[]> {
  const sources = Array.from(new Set(images.map((src) => src.trim()).filter(Boolean)))
  const results = await Promise.all(sources.map(async (src) => {
    try {
      const info = await getInfo(src)
      if (!Number.isFinite(info.width) || !Number.isFinite(info.height) || info.width <= 0 || info.height <= 0) return null
      return { src, width: info.width, height: info.height }
    } catch {
      return null
    }
  }))
  return results.filter((item): item is PostMedia => item !== null)
}

/** Use one viewport for the whole post; changing images must not move the body. */
export function postMediaHeight(media?: Pick<PostMedia, 'width' | 'height'>): number {
  if (!media || !Number.isFinite(media.width) || !Number.isFinite(media.height) || media.width <= 0 || media.height <= 0) return 560
  return Math.round(Math.min(820, Math.max(360, 750 * media.height / media.width)))
}

export function postBodyWithoutTitle(title: string, content: string): string {
  const lines = content.trim().split(/\r?\n/)
  // Publishing derives a title from the first line. Only omit an exact duplicate.
  if (title.trim() && lines[0]?.trim() === title.trim()) return lines.slice(1).join('\n').trim()
  return content.trim()
}

export function formatPostDate(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}.${pad(date.getMonth() + 1)}.${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}
