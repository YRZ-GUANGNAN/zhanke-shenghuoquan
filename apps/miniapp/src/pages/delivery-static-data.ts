export type StaticDeliveryStage = 'waiting' | 'pickup' | 'delivering' | 'completed'

export interface StaticDeliveryContact {
  name: string
  phone: string
  avatar: string
}

export interface StaticDeliveryOrder {
  id: string
  orderTime: string
  pickupName: string
  destination: string
  fee: string
  paymentLabel: string
  statusLabel: string
  statusSub: string
  stage: StaticDeliveryStage
  merchant: StaticDeliveryContact
  rider?: StaticDeliveryContact
}

const teaMerchant: StaticDeliveryContact = {
  name: '茶知味 · 商家',
  phone: '188 5530 2217',
  avatar: '茶'
}

const deliveryOrders: Record<string, StaticDeliveryOrder> = {
  B1085: {
    id: 'B1085',
    orderTime: '12:47',
    pickupName: '茶知味·大学城店',
    destination: '12栋 3楼 305室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '待接单',
    statusSub: '配送费已支付，正在等待骑手接单',
    stage: 'waiting',
    merchant: teaMerchant
  },
  B1083: {
    id: 'B1083',
    orderTime: '12:44',
    pickupName: '茶知味·大学城店',
    destination: '6栋 5楼 512室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '待接单',
    statusSub: '配送费已支付，正在等待骑手接单',
    stage: 'waiting',
    merchant: teaMerchant
  },
  B1079: {
    id: 'B1079',
    orderTime: '12:31',
    pickupName: '茶知味·大学城店',
    destination: '3栋 4楼 412室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '配送中',
    statusSub: '骑手已取货，正在前往送达点',
    stage: 'delivering',
    merchant: teaMerchant,
    rider: { name: '李伟 · 骑手', phone: '137 5560 8814', avatar: '李' }
  },
  B1077: {
    id: 'B1077',
    orderTime: '12:26',
    pickupName: '茶知味·大学城店',
    destination: '7栋 2楼 205室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '待取货',
    statusSub: '骑手已接单，等待到店取货',
    stage: 'pickup',
    merchant: teaMerchant,
    rider: { name: '张强 · 骑手', phone: '150 2318 7745', avatar: '张' }
  },
  B1071: {
    id: 'B1071',
    orderTime: '11:52',
    pickupName: '茶知味·大学城店',
    destination: '9栋 5楼 523室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '已完成',
    statusSub: '配送单已送达并完成',
    stage: 'completed',
    merchant: teaMerchant,
    rider: { name: '李伟 · 骑手', phone: '137 5560 8814', avatar: '李' }
  },
  B1066: {
    id: 'B1066',
    orderTime: '11:20',
    pickupName: '茶知味·大学城店',
    destination: '5栋 1楼 101室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '已完成',
    statusSub: '配送单已送达并完成',
    stage: 'completed',
    merchant: teaMerchant,
    rider: { name: '张强 · 骑手', phone: '150 2318 7745', avatar: '张' }
  },
  A2044: {
    id: 'A2044',
    orderTime: '12:25',
    pickupName: '瑞幸咖啡·图书馆店',
    destination: '11栋 3楼 302室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '待取货',
    statusSub: '骑手已接单，等待到店取货',
    stage: 'pickup',
    merchant: { name: '瑞幸咖啡 · 商家', phone: '186 6670 3020', avatar: '瑞' },
    rider: { name: '李伟 · 骑手', phone: '137 5560 8814', avatar: '李' }
  },
  A2039: {
    id: 'A2039',
    orderTime: '12:18',
    pickupName: '塔斯汀·大学城店',
    destination: '5栋 4楼 408室',
    fee: '1.50',
    paymentLabel: '钱包支付',
    statusLabel: '配送中',
    statusSub: '骑手已取货，正在前往送达点',
    stage: 'delivering',
    merchant: { name: '塔斯汀 · 商家', phone: '188 2305 4080', avatar: '塔' },
    rider: { name: '李伟 · 骑手', phone: '137 5560 8814', avatar: '李' }
  }
}

export function resolveStaticDeliveryOrder(orderId?: string): { order: StaticDeliveryOrder; found: boolean } {
  const normalizedId = String(orderId || 'B1079').replace(/^#/, '')
  const order = deliveryOrders[normalizedId]
  if (order) return { order, found: true }

  return {
    found: false,
    order: {
      id: normalizedId,
      orderTime: '--:--',
      pickupName: '配送单信息待接入',
      destination: '地址待接入',
      fee: '1.50',
      paymentLabel: '支付状态待接入',
      statusLabel: '信息待接入',
      statusSub: '未找到对应的静态配送单数据',
      stage: 'waiting',
      merchant: { name: '商家信息待接入', phone: '暂无电话', avatar: '商' }
    }
  }
}
