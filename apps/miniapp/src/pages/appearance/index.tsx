import { useState } from 'react'
import { Image, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppIconButton } from '../../components/ui'
import Taro from '../../services/taro'
import {
  readHomeHeaderBackdrop,
  readHomeHeaderBackdropEffect,
  readHomeHeaderBackdropImage,
  resetHomeHeaderBackdropImage,
  saveHomeHeaderBackdropImage,
  writeHomeHeaderBackdrop,
  writeHomeHeaderBackdropEffect,
  type HomeHeaderBackdropEffect
} from '../../utils/ui-preferences'
import { cropImageWithWechatEditor } from '../../utils/crop-image'
import './index.scss'

async function compressWithFallback(src: string): Promise<string> {
  try {
    const compressed = await Taro.compressImage({ src, quality: 75, compressedWidth: 1080 })
    return compressed.tempFilePath || src
  } catch {
    return src
  }
}

export default function AppearancePage() {
  const [enabled, setEnabled] = useState(readHomeHeaderBackdrop)
  const [image, setImage] = useState(readHomeHeaderBackdropImage)
  const [effect, setEffect] = useState<HomeHeaderBackdropEffect>(readHomeHeaderBackdropEffect)
  const [busy, setBusy] = useState(false)

  const toggle = () => {
    const next = !enabled
    setEnabled(next)
    writeHomeHeaderBackdrop(next)
  }

  const chooseEffect = (next: HomeHeaderBackdropEffect) => {
    if (next === effect) return
    setEffect(next)
    writeHomeHeaderBackdropEffect(next)
  }

  const pickImage = async () => {
    if (busy) return
    const hasImage = Boolean(image)
    const items = hasImage ? ['选择图片（可裁剪）', '恢复默认图片'] : ['选择图片（可裁剪）']
    let tapIndex = -1
    try {
      tapIndex = (await Taro.showActionSheet({ itemList: items })).tapIndex
    } catch {
      return
    }
    if (hasImage && tapIndex === 1) {
      try {
        await resetHomeHeaderBackdropImage()
        setImage('')
        void Taro.showToast({ title: '已恢复默认图片', icon: 'success' })
      } catch {
        void Taro.showToast({ title: '恢复默认图片失败，请重试', icon: 'none' })
      }
      return
    }
    if (tapIndex !== 0) return
    setBusy(true)
    try {
      const media = await Taro.chooseMedia({ count: 1, mediaType: ['image'], sourceType: ['album', 'camera'] })
      const file = media.tempFiles && media.tempFiles[0]
      if (!file || !file.tempFilePath) return
      const crop = await cropImageWithWechatEditor(file.tempFilePath)
      if (crop.status === 'cancelled') return
      if (crop.status === 'unchanged') void Taro.showToast({ title: '当前环境暂不支持裁剪，已使用原图', icon: 'none' })
      const savedPath = await saveHomeHeaderBackdropImage(await compressWithFallback(crop.path))
      setImage(savedPath)
      if (!enabled) {
        setEnabled(true)
        writeHomeHeaderBackdrop(true)
      }
      void Taro.showToast({ title: '背景图已更新', icon: 'success' })
    } catch {
      void Taro.showToast({ title: '保存背景图失败，请重试', icon: 'none' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <View className='appearance-page'>
      <CapsulePageHeader
        title='首页背景'
        divider={false}
        left={<AppIconButton name='back' label='返回' className='appearance-nav__back' onClick={() => void Taro.navigateBack()} />}
      />

      <View className='appearance-content'>
        <View className='appearance-card'>
          <View className='appearance-row'>
            <View className='appearance-row__main'>
              <Text className='appearance-row__title'>首页背景图</Text>
              <Text className='appearance-row__subtitle'>在首页顶部显示照片背景与毛玻璃效果；关闭时为纯白</Text>
            </View>
            <View
              className='appearance-switch-target'
              ariaRole='button'
              ariaLabel={`首页背景图，${enabled ? '已开启' : '已关闭'}`}
              onClick={toggle}
            >
              <View className={`appearance-switch ${enabled ? 'appearance-switch--on' : ''}`}><View className='appearance-switch__knob' /></View>
            </View>
          </View>
          <View className='appearance-row appearance-row--last' onClick={() => void pickImage()}>
            <View className='appearance-row__main'>
              <Text className='appearance-row__title'>背景图片</Text>
              <Text className='appearance-row__subtitle'>选择后可拖动裁剪范围；建议约 2.9:1 的横图（如 1170×408）</Text>
            </View>
            <View className='appearance-row__right'>
              {image ? <Image className='appearance-row__thumb' src={image} mode='aspectFill' /> : null}
              <Text className='appearance-row__value'>{busy ? '处理中…' : image ? '自定义图片' : '默认图片'}</Text>
              <AppIcon name='chevron-right' className='appearance-row__arrow app-icon--chevron' />
            </View>
          </View>
        </View>

        <Text className='appearance-label'>展示效果</Text>
        <View className='appearance-effect-track'>
          <View
            className={`appearance-effect-option${effect === 'glass' ? ' appearance-effect-option--active' : ''}`}
            ariaRole='button'
            ariaLabel={`毛玻璃淡化${effect === 'glass' ? '，已选中' : ''}`}
            onClick={() => chooseEffect('glass')}
          >
            <Text>毛玻璃淡化</Text>
          </View>
          <View
            className={`appearance-effect-option${effect === 'plain' ? ' appearance-effect-option--active' : ''}`}
            ariaRole='button'
            ariaLabel={`原图直显${effect === 'plain' ? '，已选中' : ''}`}
            onClick={() => chooseEffect('plain')}
          >
            <Text>原图直显</Text>
          </View>
        </View>

        <Text className='appearance-label'>效果预览</Text>
        <View className='appearance-preview'>
          <View className='appearance-preview__screen'>
            {image ? (
              <>
                <Image className='appearance-preview__image' src={image} mode='aspectFill' />
                {effect === 'glass' ? (
                  <>
                    <View className='appearance-preview__blur'>
                      <Image className='appearance-preview__image appearance-preview__image--blur' src={image} mode='aspectFill' />
                    </View>
                    <View className='appearance-preview__veil' />
                  </>
                ) : <View className='appearance-preview__veil appearance-preview__veil--plain' />}
              </>
            ) : <Text className='appearance-preview__empty'>默认纯白头部</Text>}
            <View className='appearance-preview__tabs'>
              <Text className='appearance-preview__tab appearance-preview__tab--active'>发现</Text>
              <Text className='appearance-preview__tab'>二手</Text>
              <Text className='appearance-preview__tab'>工具</Text>
              <View className='appearance-preview__search' />
            </View>
          </View>
        </View>
      </View>
    </View>
  )
}
