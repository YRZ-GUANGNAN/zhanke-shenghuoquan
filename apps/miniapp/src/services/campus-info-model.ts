export interface CampusGuide {
  id: string
  category: string
  title: string
  summary: string
  verifiedAt: string
  sourceName: string
  sourceUrl: string
  sections: Array<{ title: string; paragraphs: string[] }>
}

export function validGuideId(value: unknown): value is string {
  return typeof value === 'string' && /^[a-z][a-z0-9-]{0,63}$/.test(value)
}

export function parseCampusGuide(value: unknown): CampusGuide | null {
  if (!value || typeof value !== 'object') return null
  const guide = value as CampusGuide
  if (!validGuideId(guide.id) || (['category', 'title', 'summary', 'sourceName'] as const).some(key => typeof guide[key] !== 'string' || !guide[key].trim()) ||
    typeof guide.verifiedAt !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(guide.verifiedAt) ||
    typeof guide.sourceUrl !== 'string' || !/^https:\/\/www\.zjkju\.edu\.cn\/[a-zA-Z0-9/_.-]+$/.test(guide.sourceUrl) ||
    !Array.isArray(guide.sections) || !guide.sections.length || guide.sections.some(section => !section || typeof section.title !== 'string' || !section.title.trim() || !Array.isArray(section.paragraphs) || !section.paragraphs.length || section.paragraphs.some(text => typeof text !== 'string' || !text.trim()))) return null
  return guide
}

export function parseCampusGuides(value: unknown): CampusGuide[] | null {
  if (!value || typeof value !== 'object' || !Array.isArray((value as { items?: unknown }).items)) return null
  const items = (value as { items: unknown[] }).items.map(parseCampusGuide)
  if (items.some(item => !item) || new Set(items.map(item => item?.id)).size !== items.length) return null
  return items as CampusGuide[]
}
