declare const TARO_APP_ENABLE_LEGACY_TAKEOUT: boolean | undefined
declare const TARO_APP_ENABLE_DELIVERY: boolean | undefined
declare const TARO_APP_ENABLE_DELIVERY_MOCK_FEE: boolean | undefined
declare const TARO_APP_ENABLE_DELIVERY_PAYMENT: boolean | undefined
declare const TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK: boolean | undefined

export const legacyTakeoutEnabled =
  typeof TARO_APP_ENABLE_LEGACY_TAKEOUT !== 'undefined' &&
  TARO_APP_ENABLE_LEGACY_TAKEOUT === true

export const deliveryEnabled =
  typeof TARO_APP_ENABLE_DELIVERY === 'undefined' ||
  TARO_APP_ENABLE_DELIVERY === true

export const deliveryMockFeeEnabled =
  typeof TARO_APP_ENABLE_DELIVERY_MOCK_FEE !== 'undefined' &&
  TARO_APP_ENABLE_DELIVERY_MOCK_FEE === true

export const deliveryPaymentEnabled =
  typeof TARO_APP_ENABLE_DELIVERY_PAYMENT !== 'undefined' &&
  TARO_APP_ENABLE_DELIVERY_PAYMENT === true

export const deliveryPaymentMockEnabled =
  typeof TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK !== 'undefined' &&
  TARO_APP_ENABLE_DELIVERY_PAYMENT_MOCK === true
