declare const TARO_APP_ENABLE_LEGACY_TAKEOUT: boolean | undefined

export const legacyTakeoutEnabled =
  typeof TARO_APP_ENABLE_LEGACY_TAKEOUT !== 'undefined' &&
  TARO_APP_ENABLE_LEGACY_TAKEOUT === true
