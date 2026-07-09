import { useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import Taro from '../../services/taro'
import './index.scss'

type LegalTab = 'agreement' | 'privacy'

interface LegalSection {
  title: string
  body: string
}

const tabs: Array<{ key: LegalTab; label: string }> = [
  { key: 'agreement', label: '用户协议' },
  { key: 'privacy', label: '隐私政策' }
]

const agreementSections: LegalSection[] = [
  {
    title: '一、服务范围',
    body: '湛科生活圈为湛江科技学院在校师生提供校园社交、二手信息展示、同城跑腿配送及校园工具聚合服务。部分功能（如外卖点单）由第三方商家小程序提供，本平台仅作入口聚合。'
  },
  {
    title: '二、账号与实名',
    body: '用户需使用本人微信授权登录并绑定校园身份。账号不得转借、出售或用于违法违规用途。因账号泄露造成的损失由用户自行承担。'
  },
  {
    title: '三、内容规范',
    body: '用户对其发布的帖子、二手信息、评论等内容负责，不得发布违法、欺诈、代写代考、虚假交易等信息。平台有权对违规内容进行删除，并对违规账号采取功能限制或封禁措施。'
  },
  {
    title: '四、交易与配送',
    body: '二手交易由买卖双方线下自行完成，平台不担保、不代收款。配送服务由校园骑手承接，配送费由平台统一定价，用户下单即视为同意相应费用。'
  },
  {
    title: '五、责任限制',
    body: '因不可抗力、系统维护或第三方原因导致的服务中断，平台不承担赔偿责任，但会尽力提前告知并及时恢复。'
  }
]

const privacySections: LegalSection[] = [
  {
    title: '一、我们收集的信息',
    body: '为提供服务，我们会收集你的微信昵称与头像、校园身份信息、下单与配送所需的联系方式与地址，以及必要的设备与日志信息。'
  },
  {
    title: '二、信息如何使用',
    body: '收集的信息仅用于账号登录、订单履约、消息通知、内容推荐与安全风控，不会用于与服务无关的用途。'
  },
  {
    title: '三、信息共享',
    body: '配送场景下，取货与送达所需的最小化信息（如门牌、联系方式）仅向承接该订单的骑手与对应商家展示；抢单前收货人信息将做隐藏与打码处理。'
  },
  {
    title: '四、你的权利',
    body: '你可随时在「编辑资料」中修改个人信息，在「设置」中管理通知与个性化推荐，或通过客服申请注销账号与删除数据。'
  },
  {
    title: '五、信息安全',
    body: '我们采取加密传输与访问控制等措施保护你的信息。配送照片等敏感内容仅对订单的相关方可见，不对外公开。'
  }
]

function getInitialTab(): LegalTab {
  const tab = Taro.getCurrentInstance().router?.params?.tab
  return tab === 'privacy' ? 'privacy' : 'agreement'
}

export default function LegalPage() {
  const [activeTab, setActiveTab] = useState<LegalTab>(getInitialTab)
  const sections = activeTab === 'privacy' ? privacySections : agreementSections

  return (
    <View className='student-legal-page'>
      <View className='student-legal-nav'>
        <View className='student-legal-nav__back' onClick={() => void Taro.navigateBack()}>
          <Text>‹</Text>
        </View>
        <Text className='student-legal-nav__title'>法律条款</Text>
        <View className='student-legal-tabs'>
          {tabs.map((tab) => (
            <View
              key={tab.key}
              className={`student-legal-tab ${activeTab === tab.key ? 'student-legal-tab--active' : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              <Text>{tab.label}</Text>
            </View>
          ))}
        </View>
      </View>

      <ScrollView scrollY className='student-legal-scroll'>
        <View className='student-legal-content'>
          {activeTab === 'privacy' && (
            <Text className='student-legal-intro'>生效日期：2026 年 5 月 1 日 · 本文为示意条款</Text>
          )}

          {sections.map((section) => (
            <View className='student-legal-section' key={section.title}>
              <Text className='student-legal-section__title'>{section.title}</Text>
              <Text className='student-legal-section__body'>{section.body}</Text>
            </View>
          ))}

          <Text className='student-legal-end'>— 全文完 —</Text>
        </View>
      </ScrollView>
    </View>
  )
}
