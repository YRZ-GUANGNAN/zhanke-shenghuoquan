export type DeliveryClipboardField =
  | 'sourcePlatform'
  | 'pickupCode'
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
  来源平台: 'sourcePlatform',
  订单来源: 'sourcePlatform',
  外卖平台: 'sourcePlatform',
  取餐号: 'pickupCode',
  取餐编号: 'pickupCode',
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
  '来源平台：',
  '取餐号：',
  '',
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

const sourcePlatforms: Record<string, string> = {
  本店: 'local', 本店自发: 'local', 校内配送: 'local', 自行发单: 'local', local: 'local',
  淘宝: 'taobao', 淘宝闪购: 'taobao', taobao: 'taobao',
  京东: 'jd', 京东外卖: 'jd', jd: 'jd',
  美团: 'meituan', 美团外卖: 'meituan', meituan: 'meituan',
}

function normalizedPickupCode(value: string): string | undefined {
  const candidate = value.trim().replace(/^#/, '').replace(/号$/, '').trim()
  return /^[a-zA-Z0-9][a-zA-Z0-9-]{0,15}$/.test(candidate) ? candidate : undefined
}

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
        const value = labeled.field === 'sourcePlatform'
          ? sourcePlatforms[labeled.value.toLowerCase()]
          : labeled.field === 'pickupCode' ? normalizedPickupCode(labeled.value) : labeled.value
        if (value) values[labeled.field] = value
        else if (labeled.value) ignoredLines.push(line)
        continuationField = continuationFields.has(labeled.field) ? labeled.field : null
        return
      }

      // A full platform order ID is not a pickup number. Keep it out of both
      // pickup labels and multiline addresses; only the external adapter owns it.
      if (/^(?:(?:淘宝闪购|淘宝|京东外卖|京东|美团外卖|美团)\s*)?(?:平台订单号|外部订单号|订单号|配送单号|订单ID|deliveryOrderPublicId|sourceOrderId)\s*[:：]/i.test(line)) {
        ignoredLines.push(line)
        continuationField = null
        return
      }

      // Recognize standalone counter labels only. Do not mine ordinary remarks
      // or address continuation lines for numbers that happen to name a platform.
      const counter = !continuationField && line.match(/^(淘宝闪购|淘宝|京东外卖|京东|美团外卖|美团)\s*#?\s*([a-zA-Z0-9][a-zA-Z0-9-]{0,15})\s*号$/)
      if (counter) {
        values.sourcePlatform = sourcePlatforms[counter[1]]
        values.pickupCode = counter[2]
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
