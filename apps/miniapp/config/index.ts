import { defineConfig } from '@tarojs/cli'
import path from 'node:path'
import { NormalModuleReplacementPlugin } from 'webpack'

export default defineConfig(async (merge) => {
  const appMode = process.env.TARO_APP_MODE || process.env.NODE_ENV
  const visualDemo = process.env.TARO_APP_VISUAL_DEMO === 'true'
  const demoRoot = path.resolve(__dirname, '../src/demo')
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
    framework: 'react',
    defineConstants: { TARO_APP_VISUAL_DEMO: JSON.stringify(visualDemo) },
    ...(visualDemo ? { copy: { patterns: [{ from: path.join(demoRoot, 'photos'), to: 'dist/demo-media' }], options: {} } } : {}),
    compiler: {
      type: 'webpack5' as const,
      prebundle: {
        enable: false
      }
    },
    mini: {
      ...(visualDemo ? { webpackChain(chain) {
        const replacements = [
          [/[/\\]services[/\\]taro$|^\.\/taro$/, 'taro.ts'],
          [/[/\\]services[/\\]e-im$|^\.\/e-im$/, 'im.ts'],
          [/[/\\]use-page-share$/, 'share.ts'],
          [/[/\\]resource-url$/, 'resource-url.ts'],
        ] as const
        replacements.forEach(([pattern, file], i) => chain.plugin(`visual-demo-${i}`).use(NormalModuleReplacementPlugin, [pattern, path.join(demoRoot, file)]))
      } } : {}),
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
