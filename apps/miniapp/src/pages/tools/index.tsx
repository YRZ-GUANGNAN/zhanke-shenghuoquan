import React, { useEffect, useState } from 'react'
import { Button, Text, View } from '@tarojs/components'
import { BottomNav } from '../../components'
import { listNavigationItems } from '../../services/d-navigation'
import Taro from '../../services/taro'
import type { ToolLink } from '../../types'
import './index.scss'

export default function ToolsPage() {
	const [tools, setTools] = useState<ToolLink[]>([])

  useEffect(() => {
    let active = true
	listNavigationItems().then((nextTools) => {
      if (active) setTools(nextTools)
	}).catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

	const openTool = (tool: ToolLink) => {
	  if (!tool.appId || !tool.path) {
		Taro.showToast({ title: '跳转入口未配置', icon: 'none' })
		return
	  }
	  Taro.navigateToMiniProgram({ appId: tool.appId, path: tool.path })
  }

  return (
    <View className='page tools-page'>
      <View className='tools-header'>
        <Text className='tools-header__title'>小程序导航</Text>
        <Text className='tools-header__desc'>保留社区和二手主功能，其他校内工具只做跳转聚合。</Text>
      </View>

      <View className='tools-grid'>
        {tools.map((tool) => (
          <View key={tool.id} className='tool-card card'>
            <View className='tool-card__icon' style={{ background: tool.color }}>{tool.iconText}</View>
            <View className='tool-card__copy'>
              <Text className='tool-card__title'>{tool.title}</Text>
              <Text className='tool-card__subtitle'>{tool.subtitle}</Text>
            </View>
			<Button className='cc-button cc-button--small' onClick={() => openTool(tool)}>打开</Button>
          </View>
        ))}
      </View>

      <View className='integration card'>
        <Text className='integration__title'>跳转配置说明</Text>
        <Text className='integration__text'>后台可配置 appId、path、启停状态与排序。小程序端通过 wx.navigateToMiniProgram 打开外部服务。</Text>
      </View>

      <BottomNav active='tools' />
    </View>
  )
}
