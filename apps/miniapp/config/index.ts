import { defineConfig } from '@tarojs/cli'

export default defineConfig(async (merge) => {
  const appMode = process.env.TARO_APP_MODE || process.env.NODE_ENV
  const baseConfig = {
    projectName: 'campus-circle',
    date: '2026-06-17',
    designWidth: 750,
    deviceRatio: {
      640: 2.34 / 2,
      750: 1,
      828: 1.81 / 2
    },
    sourceRoot: 'src',
    outputRoot: 'dist',
    copy: { patterns: [{ from: 'src/assets/share/default-share.png', to: 'dist/assets/share/default-share.png' }] },
    framework: 'react',
    compiler: {
      type: 'webpack5' as const,
      prebundle: {
        enable: false
      }
    },
    mini: {
      postcss: {
        pxtransform: {
          enable: true,
          config: {}
        },
        cssModules: {
          enable: false,
          config: {
            namingPattern: 'module',
            generateScopedName: '[name]__[local]___[hash:base64:5]'
          }
        }
      }
    },
    h5: {}
  }

  if (appMode === 'development') {
    const developmentConfig = require('./dev')
    return merge({}, baseConfig, developmentConfig.default || developmentConfig)
  }
  const productionConfig = require('./prod')
  return merge({}, baseConfig, productionConfig.default || productionConfig)
})
