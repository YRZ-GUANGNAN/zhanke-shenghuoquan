import { request } from './api'
import { parseCampusGuide, parseCampusGuides, validGuideId } from './campus-info-model'
export type { CampusGuide } from './campus-info-model'

export async function listCampusGuides() {
  const result = parseCampusGuides(await request<unknown>('/campus-info/guides'))
  if (!result) throw new Error('指南内容暂时无法读取，请稍后重试')
  return result
}

export async function getCampusGuide(id: string) {
  if (!validGuideId(id)) throw new Error('指南链接无效，请返回首页')
  const result = parseCampusGuide(await request<unknown>(`/campus-info/guides/${encodeURIComponent(id)}`))
  if (!result || result.id !== id) throw new Error('指南内容暂时无法读取，请稍后重试')
  return result
}
