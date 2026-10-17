const apiBase = process.env.TARO_APP_VISUAL_DEMO === 'true' ? 'https://visual-demo.invalid/api/v1' : process.env.TARO_APP_API_BASE || 'https://zhanduiwang.top/api/v1'
const enableLegacyTakeout = process.env.TARO_APP_ENABLE_LEGACY_TAKEOUT === 'true'
const enableDelivery = process.env.TARO_APP_ENABLE_DELIVERY !== 'false'
const enableDeliveryMockFee = process.env.TARO_APP_ENABLE_DELIVERY_MOCK_FEE === 'true'
const enableDeliveryPayment = process.env.TARO_APP_ENABLE_DELIVERY_PAYMENT === 'true'
const enableDeliveryPaymentMock = process.env.TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK === 'true'

export default {
  env: {
    NODE_ENV: '"production"',
    TARO_APP_API_BASE: JSON.stringify(apiBase)
  },
  defineConstants: {
    TARO_APP_API_BASE: JSON.stringify(apiBase),
    TARO_APP_ENABLE_LEGACY_TAKEOUT: JSON.stringify(enableLegacyTakeout),
    TARO_APP_ENABLE_DELIVERY: JSON.stringify(enableDelivery),
    TARO_APP_ENABLE_DELIVERY_MOCK_FEE: JSON.stringify(enableDeliveryMockFee),
    TARO_APP_ENABLE_DELIVERY_PAYMENT: JSON.stringify(enableDeliveryPayment),
    TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK: JSON.stringify(enableDeliveryPaymentMock)
  },
  mini: {},
  h5: {}
}
