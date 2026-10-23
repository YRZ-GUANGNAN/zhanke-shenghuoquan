import { useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
import Taro from '../../services/taro'
import { navigateBackOrHome } from '../../utils/navigation'
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
    body: '湛科生活圈提供校园信息与生活服务。产品支持原版和校园资讯两种展示模式，对所有用户统一采用当前配置。原版按已开放模块提供校园社交、二手信息展示、跑腿配送及校园工具入口；校园资讯模式提供指南阅读、账号与私密反馈，暂停上述新业务，保留已有订单的必要处理。指南根据公开资料独立整理，附来源与核验日期，非学校官方服务。部分服务由第三方商家小程序提供。'
  },
  {
    title: '二、账号与实名',
    body: '用户通过微信登录建立账号，可主动填写昵称、头像、学校和校区；填写学校资料不等于学校官方认证。骑手和商家身份按相应申请流程审核。账号不得转借、出售或用于违法违规用途。本应用不会要求提供图书馆账号密码。'
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
    body: '我们会处理服务问题与信息更正反馈，并尽力提前告知维护安排。指南反映核验时可查到的资料，实际服务以学校和馆方最新通知为准。业务暂停不删除历史数据，也不免除既有履约义务；依法应承担的责任不因本说明而免除。'
  }
]

const privacySections: LegalSection[] = [
  {
    title: '一、我们收集的信息',
    body: '微信登录用于建立账号并校验会话。你主动填写或授权的昵称、头像、学校和校区用于资料展示；反馈文字与主动上传的图片用于处理问题。下单和配送按实际业务需要处理联系方式与地址，以及必要的安全日志。校园指南阅读不要求提供定位、通讯录或图书馆账号密码。'
  },
  {
    title: '二、信息如何使用',
    body: '信息仅用于你实际使用的账号、资料、内容、反馈、消息与订单服务及安全保障。校园资讯模式不按阅读行为提供个性化推荐；原业务暂停后，历史订单和资金信息继续用于必要履约、退款、争议处理与审计。'
  },
  {
    title: '三、信息共享',
    body: '配送场景下，取货与送达所需的最小化信息（如门牌、联系方式）仅向承接该订单的骑手与对应商家展示；抢单前收货人信息将做隐藏与打码处理。'
  },
  {
    title: '四、你的权利',
    body: '你可随时在「编辑资料」中修改个人信息，在「设置」中管理通知与个性化推荐，并可直接发起账号注销。为履行交易、审计与安全义务，已完成交易和历史内容会按规则匿名保留；可删除的个人设置、绑定关系与搜索历史会在注销时清理。'
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
      <CapsulePageHeader
        title='法律条款'
        left={<AppIconButton name='back' label='返回' className='student-legal-nav__back' onClick={() => void navigateBackOrHome()} />}
      />
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

      <ScrollView scrollY className='student-legal-scroll'>
        <View className='student-legal-content'>
          {activeTab === 'privacy' && (
            <Text className='student-legal-intro'>生效日期：2026 年 7 月 19 日</Text>
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
