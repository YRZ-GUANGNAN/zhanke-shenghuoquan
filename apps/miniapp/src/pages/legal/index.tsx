import { useState } from 'react'
import { ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import { AppIconButton } from '../../components/ui'
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
    body: '当前版本提供校园指南阅读、个人资料管理和私密意见反馈。指南依据公开资料独立整理，附来源和核验日期；本应用非学校官方应用，不代办图书借阅、预约或账号登录。社区、二手交易、外卖及新增跑腿业务不在当前版本范围内。'
  },
  {
    title: '二、账号与资料',
    body: '账号通过微信登录建立。用户可自行填写昵称、头像等个人资料；填写学校或校区不等于学校官方认证。不得转借、出售账号或用于违法用途。本应用不会要求你提供学校图书馆密码。'
  },
  {
    title: '三、内容规范',
    body: '意见反馈仅供运营人员处理问题，不形成公开留言或用户交流。请提供与问题有关的信息，不提交他人隐私、违法内容或与反馈无关的敏感资料。发现指南信息变化，可通过意见反馈申请核实更正。'
  },
  {
    title: '四、历史业务处理',
    body: '收缩新业务范围不会删除历史数据或免除既有履约义务。已有订单、退款和资金记录仍由原有授权相关方依原流程处理；历史内容不因此重新公开。后续新增业务需另行发布对应版本并完成适用的审核。'
  },
  {
    title: '五、责任限制',
    body: '指南反映核验时可查到的公开资料，不提供实时开放状态或到校通行保证。临时调整以学校、馆方最新通知为准。我们会处理信息更正与服务问题反馈；依法应承担的责任不因本说明而免除。'
  }
]

const privacySections: LegalSection[] = [
  {
    title: '一、我们收集的信息',
    body: '微信登录用于建立账号并校验会话。你主动填写或授权的昵称、头像、学校与校区资料用于账号展示；反馈文字及你主动上传的图片用于处理问题。服务还处理必要的访问与安全日志。当前指南阅读不要求提供定位、通讯录或图书馆账号密码。'
  },
  {
    title: '二、信息如何使用',
    body: '当前版本将信息用于账号登录、资料维护、问题反馈与安全保障，不基于阅读行为提供个性化资讯推荐。过去业务产生的订单、联系方式和资金记录仅用于必要的履约、退款、争议处理与安全审计。'
  },
  {
    title: '三、信息共享',
    body: '指南来源链接仅用于查证。本应用不向学校图书馆提交你的账号或反馈。历史订单履约时，必要信息仍按原权限向相关方提供；意见反馈供运营人员处理，不对其他用户公开。微信登录与图片服务按实际使用的微信接口处理必要数据。'
  },
  {
    title: '四、你的权利',
    body: '你可以在「编辑资料」修改个人信息，在「意见反馈」提出查阅、更正或删除请求，也可在「设置」发起注销。存在未完成订单或资金处理事项时，系统会说明阻碍注销的具体原因。注销后会话失效，可删除的设置与绑定关系清理；必要历史交易和内容按现有规则匿名保留。'
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
        left={<AppIconButton name='back' label='返回' className='student-legal-nav__back' onClick={() => void Taro.navigateBack()} />}
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
            <Text className='student-legal-intro'>更新日期：2026 年 10 月 13 日</Text>
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
