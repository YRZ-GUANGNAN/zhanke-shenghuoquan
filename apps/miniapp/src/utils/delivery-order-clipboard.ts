export type DeliveryClipboardField =
  | 'pickupContact'
  | 'pickupPhone'
  | 'pickupAddress'
  | 'receiverName'
  | 'receiverPhone'
  | 'receiverAddress'
  | 'remark'

export type DeliveryClipboardValues = Partial<Record<DeliveryClipboardField, string>>

export interface DeliveryClipboardParseResult {
  values: DeliveryClipboardValues
  recognizedFields: DeliveryClipboardField[]
  ignoredLines: string[]
}

type DeliverySection = 'pickup' | 'receiver' | null

const explicitLabels: Record<string, DeliveryClipboardField> = {
  取货联系人: 'pickupContact',
  取件人: 'pickupContact',
  商家联系人: 'pickupContact',
  取货电话: 'pickupPhone',
  取件电话: 'pickupPhone',
  商家电话: 'pickupPhone',
  取货地址: 'pickupAddress',
  取件地址: 'pickupAddress',
  商家地址: 'pickupAddress',
  收货人: 'receiverName',
  收件人: 'receiverName',
  收货电话: 'receiverPhone',
  收件电话: 'receiverPhone',
  送货地址: 'receiverAddress',
  收货地址: 'receiverAddress',
  配送地址: 'receiverAddress',
  备注: 'remark',
  配送备注: 'remark',
  说明: 'remark',
}

const genericLabels: Record<Exclude<DeliverySection, null>, Record<string, DeliveryClipboardField>> = {
  pickup: {
    联系人: 'pickupContact',
    姓名: 'pickupContact',
    电话: 'pickupPhone',
    联系电话: 'pickupPhone',
    地址: 'pickupAddress',
  },
  receiver: {
    联系人: 'receiverName',
    姓名: 'receiverName',
    电话: 'receiverPhone',
    联系电话: 'receiverPhone',
    地址: 'receiverAddress',
  },
}

const continuationFields = new Set<DeliveryClipboardField>(['pickupAddress', 'receiverAddress', 'remark'])

export const deliveryClipboardTemplate = [
  '取货信息',
  '联系人：',
  '电话：',
  '地址：',
  '',
  '收货信息',
  '收货人：',
  '收货电话：',
  '送货地址：',
  '备注：',
].join('\n')

function cleanLine(value: string) {
  return value
    .replace(/\u00a0/g, ' ')
    .replace(/^[\s•·*#—-]+/, '')
    .trim()
}

function sectionFromLine(line: string): DeliverySection | undefined {
  const compact = line.replace(/[\s:：]/g, '')
  if (/^(取货|取件|商家)信息$/.test(compact)) return 'pickup'
  if (/^(收货|收件|配送)信息$/.test(compact)) return 'receiver'
  return undefined
}

function normalizeLabel(value: string) {
  return value.replace(/\s+/g, '').trim()
}

function fieldForLabel(label: string, section: DeliverySection) {
  const normalized = normalizeLabel(label)
  return (section ? genericLabels[section][normalized] : undefined)
    ?? explicitLabels[normalized]
    ?? (normalized === '联系电话' ? 'receiverPhone' : undefined)
}

function splitLabeledLine(line: string, section: DeliverySection) {
  const colonMatch = line.match(/^([^:：]{1,16})\s*[:：]\s*(.*)$/)
  if (colonMatch) {
    const field = fieldForLabel(colonMatch[1], section)
    return field ? { field, value: colonMatch[2].trim() } : null
  }

  const knownLabels = [
    ...Object.keys(explicitLabels),
    ...(section ? Object.keys(genericLabels[section]) : []),
  ].sort((left, right) => right.length - left.length)

  for (const label of knownLabels) {
    const match = line.match(new RegExp(`^${label}\\s+(.+)$`))
    if (!match) continue
    const field = fieldForLabel(label, section)
    if (field) return { field, value: match[1].trim() }
  }
  return null
}

export function parseDeliveryOrderClipboard(source: string): DeliveryClipboardParseResult {
  const values: DeliveryClipboardValues = {}
  const ignoredLines: string[] = []
  let section: DeliverySection = null
  let continuationField: DeliveryClipboardField | null = null

  String(source ?? '')
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .forEach((rawLine) => {
      const line = cleanLine(rawLine)
      if (!line) {
        continuationField = null
        return
      }

      const nextSection = sectionFromLine(line)
      if (nextSection !== undefined) {
        section = nextSection
        continuationField = null
        return
      }

      const labeled = splitLabeledLine(line, section)
      if (labeled) {
        if (labeled.value) values[labeled.field] = labeled.value
        continuationField = continuationFields.has(labeled.field) ? labeled.field : null
        return
      }

      if (continuationField && values[continuationField]) {
        const separator = continuationField === 'remark' ? '\n' : ' '
        values[continuationField] = `${values[continuationField]}${separator}${line}`
        return
      }

      ignoredLines.push(line)
      continuationField = null
    })

  const recognizedFields = (Object.keys(values) as DeliveryClipboardField[])
    .filter((field) => Boolean(values[field]?.trim()))

  return { values, recognizedFields, ignoredLines }
}
