import { closeDeliveryWalletRecharge, closeDeliveryWalletRechargeByRequest, getDeliveryPayment, getDeliveryWalletRechargeByRequest, pollDeliveryPayment, startDeliveryWalletRecharge, type DeliveryPayment, type DeliveryWechatPaymentParameters } from '../../services/delivery-payment'
import { savePendingWalletRecharge, type PendingWalletRecharge } from './wallet-state'

export async function runWalletRecharge(input: {
  pending: PendingWalletRecharge
  verifyOnly?: boolean
  cancelOnly?: boolean
  isCurrent: () => boolean
  shouldCancel?: () => boolean
  onCancelRequested?: (pending: PendingWalletRecharge) => void
  requestPayment: (parameters: DeliveryWechatPaymentParameters) => Promise<unknown>
}): Promise<DeliveryPayment> {
  let pending = { ...input.pending }
  const check = () => { if (!input.isCurrent()) throw new Error('页面或会话已变化，充值操作已中断') }
  const validate = (payment: DeliveryPayment) => {
    if (payment.purpose !== 'wallet_recharge' || payment.amountCents !== input.pending.amountCents
      || !payment.paymentPublicId || (pending.paymentPublicId && payment.paymentPublicId !== pending.paymentPublicId)) {
      throw new Error('充值订单或金额不一致，请核实原单')
    }
  }
  const terminal = (payment: DeliveryPayment) => ['paid', 'failed', 'closed'].includes(payment.status)
  const cancelling = () => Boolean(pending.cancelRequested || input.cancelOnly || input.shouldCancel?.())
  const markCancellation = () => {
    check()
    pending = { ...pending, cancelRequested: true }
    savePendingWalletRecharge(pending)
    input.onCancelRequested?.(pending)
  }
  const close = async (payment?: DeliveryPayment): Promise<DeliveryPayment> => {
    check()
    if (payment) {
      validate(payment)
      if (terminal(payment)) return payment
      if (payment.provider !== 'wechat') throw new Error('充值订单不是微信支付，请核实原单')
    }
    markCancellation()
    // Never wait for a query or issue another prepay before persisting intent.
    // The by-request endpoint atomically closes a missing/in-flight attempt.
    const latest = pending.paymentPublicId
      ? await closeDeliveryWalletRecharge(pending.paymentPublicId)
      : await closeDeliveryWalletRechargeByRequest(pending.clientRequestId, pending.amountCents)
    check(); validate(latest)
    pending = { ...pending, paymentPublicId: latest.paymentPublicId }
    savePendingWalletRecharge(pending)
    return latest
  }
  const poll = async (payment: DeliveryPayment): Promise<DeliveryPayment> => {
    const latest = await pollDeliveryPayment(payment.paymentPublicId, () => input.isCurrent() && !cancelling())
    check(); validate(latest)
    return cancelling() || latest.cancelRequestedAt ? close(latest) : latest
  }
  check()
  if (cancelling()) return close()
  let payment: DeliveryPayment | undefined
  if (input.pending.paymentPublicId) {
    payment = await getDeliveryPayment(input.pending.paymentPublicId)
    check(); validate(payment)
    if (terminal(payment)) return payment
    if (cancelling() || payment.cancelRequestedAt) return close(payment)
    if (input.verifyOnly) return payment
    if (payment.providerStatus === 'USERPAYING') return poll(payment)
  }
  if (input.verifyOnly) {
    payment = await getDeliveryWalletRechargeByRequest(pending.clientRequestId, pending.amountCents) || undefined
    check()
    if (!payment) throw new Error('充值记录尚未确认，请稍后查询或取消充值')
    validate(payment)
    pending = { ...pending, paymentPublicId: payment.paymentPublicId }
    savePendingWalletRecharge(pending)
    return payment.cancelRequestedAt || cancelling() ? close(payment) : payment
  }
  const result = await startDeliveryWalletRecharge(input.pending.amountCents, input.pending.clientRequestId)
  check()
  payment = result.payment
  if (!payment) throw new Error('未取得充值记录，请核实原单')
  validate(payment)
  pending = { ...pending, paymentPublicId: payment.paymentPublicId }
  savePendingWalletRecharge(pending)
  if (terminal(payment)) return payment
  if (cancelling() || payment.cancelRequestedAt) return close(payment)
  if (input.verifyOnly) return payment
  if (payment.provider !== 'wechat' || !payment.wechat) throw new Error('未取得微信支付参数，请核实原单')
  try {
    await input.requestPayment(payment.wechat)
  } catch (error) {
    check()
    const message = error instanceof Error ? error.message : String((error as { errMsg?: string })?.errMsg || '')
    if (!/cancel/i.test(message)) throw error
    return close(payment)
  }
  check()
  if (cancelling()) return close(payment)
  return poll(payment)
}
