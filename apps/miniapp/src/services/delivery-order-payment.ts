import { getMerchantDeliveryOrder, type MerchantDeliveryOrder } from './delivery-merchant'
import {
  getDeliveryOrderFunds, payDeliveryOrderWithWallet, payDeliveryOrderWithWechat,
  pollDeliveryPayment, requestDeliveryOrderRefund,
  type DeliveryOrderFunds, type DeliveryWechatPaymentParameters,
} from './delivery-payment'

export type ManualDeliveryPaymentAction = 'wechat' | 'wallet' | 'refund'

// UI visibility is only a hint. Every command repeats these checks using fresh
// server data, and the server still owns versioning, ownership and accounting.
export function manualDeliveryPaymentActions(order: MerchantDeliveryOrder, funds: DeliveryOrderFunds | null): ManualDeliveryPaymentAction[] {
  if (!funds || funds.deliveryOrderPublicId !== order.deliveryOrderPublicId || funds.refund) return []
  if (funds.settlement) {
    return funds.settlement.status === 'paid' && order.status === 'ready_for_claim' && !order.riderUserId ? ['refund'] : []
  }
  if (order.status !== 'pending_fee_payment' || funds.payment?.status === 'paid') return []
  const payment = funds.payment
  if (payment && payment.status !== 'closed' && payment.status !== 'failed') {
    return payment.provider === 'wechat' && payment.providerStatus !== 'USERPAYING' ? ['wechat'] : []
  }
  return ['wechat', 'wallet']
}

export async function executeManualDeliveryPayment(input: {
  action: ManualDeliveryPaymentAction
  order: MerchantDeliveryOrder
  requestId: string
  isCurrent: () => boolean
  requestPayment: (parameters: DeliveryWechatPaymentParameters) => Promise<unknown>
}) {
  const check = () => { if (!input.isCurrent()) throw new Error('页面或会话已变化，本次操作已中断') }
  check()
  const order = await getMerchantDeliveryOrder(input.order.deliveryOrderPublicId)
  check()
  const funds = await getDeliveryOrderFunds(order.deliveryOrderPublicId)
  check()
  if (order.rowVersion !== input.order.rowVersion || order.deliveryFeeCents !== input.order.deliveryFeeCents) {
    throw new Error('订单状态或配送费用已更新，请刷新后重新确认')
  }
  if (!manualDeliveryPaymentActions(order, funds).includes(input.action)) {
    throw new Error('支付或配送状态已变化，请刷新核实后操作')
  }
  let nativeCancelled = false
  if (input.action === 'refund') {
    await requestDeliveryOrderRefund(order.deliveryOrderPublicId, order.rowVersion, input.requestId)
    check()
  } else if (input.action === 'wallet') {
    await payDeliveryOrderWithWallet(order.deliveryOrderPublicId, order.rowVersion, input.requestId)
    check()
  } else {
    const result = await payDeliveryOrderWithWechat(order.deliveryOrderPublicId, order.rowVersion, input.requestId)
    check()
    if (!result.payment) throw new Error('未取得支付记录，请刷新核实原订单')
    if (result.payment.status === 'created' || result.payment.status === 'prepay_issued') {
      if (result.payment.provider !== 'wechat' || !result.payment.wechat) throw new Error('未取得有效微信支付参数，请刷新核实原订单')
      try {
        await input.requestPayment(result.payment.wechat)
      } catch (error) {
        check()
        const message = error instanceof Error ? error.message : String((error as { errMsg?: string })?.errMsg || '')
        if (!/cancel/i.test(message)) throw error
        nativeCancelled = true
      }
      check()
      if (!nativeCancelled) {
        await pollDeliveryPayment(result.payment.paymentPublicId, input.isCurrent)
        check()
      }
    }
  }
  // Native success and successful HTTP writes do not prove money movement.
  const latestFunds = await getDeliveryOrderFunds(order.deliveryOrderPublicId)
  check()
  const latestOrder = await getMerchantDeliveryOrder(order.deliveryOrderPublicId)
  check()
  return { order: latestOrder, funds: latestFunds, nativeCancelled }
}
