const apiBase = process.env.TARO_APP_API_BASE || 'http://127.0.0.1:8080/api/v1'
const enableLegacyTakeout = process.env.TARO_APP_ENABLE_LEGACY_TAKEOUT === 'true'

export default {
  env: {
    NODE_ENV: '"development"',
    TARO_APP_API_BASE: JSON.stringify(apiBase)
  },
  defineConstants: {
    TARO_APP_API_BASE: JSON.stringify(apiBase),
    TARO_APP_ENABLE_LEGACY_TAKEOUT: JSON.stringify(enableLegacyTakeout)
  },
  mini: {},
  h5: {}
}
