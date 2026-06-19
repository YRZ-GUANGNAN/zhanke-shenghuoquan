const apiBase = process.env.TARO_APP_API_BASE || ''

export default {
  env: {
    NODE_ENV: '"production"',
    TARO_APP_API_BASE: JSON.stringify(apiBase)
  },
  defineConstants: {
    TARO_APP_API_BASE: JSON.stringify(apiBase)
  },
  mini: {},
  h5: {}
}
