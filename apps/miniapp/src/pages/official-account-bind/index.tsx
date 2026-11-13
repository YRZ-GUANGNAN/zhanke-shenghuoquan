import { useEffect, useState } from 'react'
import { Text, View, WebView } from '@tarojs/components'
import Taro from '../../services/taro'
import { createOfficialAccountOAuthURL } from '../../services/notification'

export default function OfficialAccountBindPage() {
  const [url, setUrl] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true
    void createOfficialAccountOAuthURL().then(value => {
      if (alive) setUrl(value)
    }).catch(value => {
      if (alive) setError(value instanceof Error ? value.message : '公众号绑定暂不可用')
    })
    return () => { alive = false }
  }, [])

  if (error) {
    return <View className='official-account-bind official-account-bind--state'>
      <Text className='official-account-bind__title'>暂时无法绑定</Text>
      <Text className='official-account-bind__text'>{error}</Text>
      <View className='official-account-bind__button' onClick={() => void Taro.navigateBack()}>返回设置</View>
    </View>
  }
  if (!url) {
    return <View className='official-account-bind official-account-bind--state'>
      <Text className='official-account-bind__title'>正在准备公众号授权</Text>
      <Text className='official-account-bind__text'>完成后会显示绑定结果，请返回设置页确认。</Text>
    </View>
  }
  return <WebView src={url} />
}
