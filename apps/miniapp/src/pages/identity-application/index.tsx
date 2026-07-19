import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { getIdentityApplication, submitIdentityApplication } from '../../services/a-user'
import { isApiError } from '../../services/api'
import { chooseAndUploadImages, waitForMediaApproval, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import type { IdentityApplicationRole, IdentityApplicationView } from '../../types'
import './index.scss'

type ProofState = 'stored' | 'uploaded' | 'checking' | 'approved' | 'failed'

interface ProofItem {
  assetId: string
  previewPath: string
  state: ProofState
  message: string
}

const roleCopy: Record<IdentityApplicationRole, { title: string; description: string }> = {
  rider: { title: '申请骑手身份', description: '审核通过后可进入配送抢单工作台' },
  merchant: { title: '申请商家身份', description: '审核通过后可进入商家配送管理工作台' }
}

function readRole(): IdentityApplicationRole | null {
  const value = Taro.getCurrentInstance().router?.params.role
  return value === 'rider' || value === 'merchant' ? value : null
}

function existingProof(assetId: string): ProofItem {
  return {
    assetId,
    previewPath: '',
    state: 'stored',
    message: '已保存，提交时将重新校验证明状态'
  }
}

function uploadedProof(value: UploadedMedia): ProofItem {
  return {
    assetId: value.assetId,
    previewPath: value.previewPath,
    state: value.canUse && value.state === 'approved' ? 'approved' : 'uploaded',
    message: value.message || '图片已上传'
  }
}

function proofStateLabel(value: ProofItem): string {
  if (value.state === 'checking') return value.message || '正在确认审核状态'
  if (value.state === 'approved') return '证明图片已通过安全审核'
  if (value.state === 'failed') return value.message || '图片处理失败，请移除后重新上传'
  return value.message
}

function applicationErrorMessage(error: unknown): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : '申请提交失败，请稍后重试'
  if (error.status === 0) return '网络连接失败，申请内容已保留，请稍后重试'
  if (error.status === 400) return '申请资料不符合要求，请检查后重试'
  if (error.status === 401 || error.status === 403) return '登录状态已失效，请重新登录'
  if (error.status === 409) return '申请状态已发生变化，已为你刷新最新结果'
  return error.message || '申请提交失败，请稍后重试'
}

function readableTime(value?: string): string {
  if (!value) return '--'
  return value.replace('T', ' ').replace(/\.\d+(?:Z|[+-]\d\d:\d\d)?$/, '').replace(/Z$/, '').slice(0, 19)
}

function IdentityApplicationContent() {
  const role = useMemo(readRole, [])
  const [application, setApplication] = useState<IdentityApplicationView | null>(null)
  const [realName, setRealName] = useState('')
  const [phone, setPhone] = useState('')
  const [merchantName, setMerchantName] = useState('')
  const [merchantAddress, setMerchantAddress] = useState('')
  const [proofs, setProofs] = useState<ProofItem[]>([])
  const [loading, setLoading] = useState(Boolean(role))
  const [loadError, setLoadError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [progress, setProgress] = useState('')

  const applyApplication = useCallback((value: IdentityApplicationView) => {
    setApplication(value)
    setRealName(value.realName)
    setPhone(value.phone)
    setMerchantName(value.merchantName)
    setMerchantAddress(value.merchantAddress)
    setProofs((current) => value.assetIds.map((assetId) => current.find((item) => item.assetId === assetId) ?? existingProof(assetId)))
  }, [])

  const loadApplication = useCallback(async () => {
    if (!role) return
    setLoading(true)
    setLoadError('')
    try {
      applyApplication(await getIdentityApplication(role))
    } catch (error) {
      if (isApiError(error) && error.status === 404) {
        setApplication(null)
        setProofs([])
      } else {
        setLoadError(applicationErrorMessage(error))
      }
    } finally {
      setLoading(false)
    }
  }, [applyApplication, role])

  useEffect(() => {
    void loadApplication()
  }, [loadApplication])

  const editable = !application || application.status === 'rejected'
  const busy = uploading || submitting
  const copy = role ? roleCopy[role] : null

  const updateProof = (assetId: string, changes: Partial<ProofItem>) => {
    setProofs((current) => current.map((item) => item.assetId === assetId ? { ...item, ...changes } : item))
  }

  const chooseProofs = async () => {
    if (!editable || busy) return
    if (proofs.length >= 6) {
      await Taro.showToast({ title: '最多上传 6 张证明图片', icon: 'none' })
      return
    }
    setUploading(true)
    setProgress('正在选择证明图片')
    try {
      const uploaded = await chooseAndUploadImages('identity_application', 6 - proofs.length, (value) => {
        setProgress(`第 ${value.index + 1}/${value.total} 张 · ${value.message}`)
      })
      setProofs((current) => [...current, ...uploaded.map(uploadedProof)].slice(0, 6))
      setProgress(uploaded.length > 0 ? '图片已上传，提交时将确认安全审核结果' : '')
    } catch (error) {
      const message = error instanceof Error ? error.message : '证明图片上传失败，请重新选择'
      setProgress(message)
      await Taro.showToast({ title: message, icon: 'none', duration: 2200 })
    } finally {
      setUploading(false)
    }
  }

  const removeProof = (index: number) => {
    if (!editable || busy) return
    setProofs((current) => current.filter((_, currentIndex) => currentIndex !== index))
  }

  const previewProof = async (value: ProofItem) => {
    if (!value.previewPath) {
      await Taro.showToast({ title: '历史证明受隐私保护，请重新上传后预览', icon: 'none' })
      return
    }
    const urls = proofs.map((item) => item.previewPath).filter(Boolean)
    await Taro.previewImage({ urls, current: value.previewPath })
  }

  const validate = (): string => {
    if (!realName.trim()) return '请填写真实姓名'
    if (!/^1[3-9][0-9]{9}$/.test(phone.trim())) return '请填写正确的 11 位手机号'
    if (role === 'merchant' && !merchantName.trim()) return '请填写商家名称'
    if (role === 'merchant' && !merchantAddress.trim()) return '请填写商家地址'
    if (proofs.length < 1 || proofs.length > 6) return '请上传 1 至 6 张证明图片'
    return ''
  }

  const submit = async () => {
    if (!role || !editable || busy) return
    const validation = validate()
    if (validation) {
      await Taro.showToast({ title: validation, icon: 'none' })
      return
    }

    setSubmitting(true)
    setProgress('正在确认所有证明图片的安全审核状态')
    try {
      for (const proof of proofs) {
        updateProof(proof.assetId, { state: 'checking', message: '正在确认安全审核状态' })
        try {
          await waitForMediaApproval(proof.assetId, (status) => {
            updateProof(proof.assetId, { state: 'checking', message: status.message || '正在审核' })
            setProgress(status.message || '正在确认证明图片')
          })
          updateProof(proof.assetId, { state: 'approved', message: '证明图片已通过安全审核' })
        } catch (error) {
          updateProof(proof.assetId, {
            state: 'failed',
            message: error instanceof Error ? error.message : '图片处理失败，请重新上传'
          })
          throw error
        }
      }

      const result = await submitIdentityApplication(role, {
        realName: realName.trim(),
        phone: phone.trim(),
        merchantName: role === 'merchant' ? merchantName.trim() : '',
        merchantAddress: role === 'merchant' ? merchantAddress.trim() : '',
        assetIds: proofs.map((item) => item.assetId),
        expectedRowVersion: application?.status === 'rejected' ? application.rowVersion : 0
      })
      applyApplication(result)
      setProgress('')
      await Taro.showToast({ title: '申请已提交，等待平台审核', icon: 'success', duration: 1800 })
    } catch (error) {
      const message = applicationErrorMessage(error)
      setProgress(message)
      if (isApiError(error) && error.status === 409) await loadApplication()
      await Taro.showToast({ title: message, icon: 'none', duration: 2400 })
    } finally {
      setSubmitting(false)
    }
  }

  if (!role || !copy) {
    return (
      <View className='identity-application-page'>
        <View className='identity-application-state'>
          <Text className='identity-application-state__title'>申请类型无效</Text>
          <Text className='identity-application-state__copy'>只能申请骑手或商家身份。</Text>
          <View className='identity-application-state__action' onClick={() => void Taro.navigateBack()}><Text>返回</Text></View>
        </View>
      </View>
    )
  }

  return (
    <View className='identity-application-page'>
      <View className='identity-application-nav'>
        <View className='identity-application-nav__back' onClick={() => void Taro.navigateBack()}><Text>‹</Text></View>
        <Text className='identity-application-nav__title'>{copy.title}</Text>
      </View>

      <View className='identity-application-content'>
        <View className='identity-application-intro'>
          <Text className='identity-application-intro__title'>{copy.title}</Text>
          <Text className='identity-application-intro__copy'>{copy.description}</Text>
        </View>

        {loading && <View className='identity-application-card identity-application-state-card'><Text>正在加载申请状态…</Text></View>}
        {!loading && loadError && (
          <View className='identity-application-card identity-application-state-card'>
            <Text className='identity-application-state-card__error'>{loadError}</Text>
            <View className='identity-application-inline-action' onClick={() => void loadApplication()}><Text>重新加载</Text></View>
          </View>
        )}

        {!loading && !loadError && application?.status === 'pending' && (
          <View className='identity-application-card identity-application-status identity-application-status--pending'>
            <Text className='identity-application-status__badge'>审核中</Text>
            <Text className='identity-application-status__title'>申请已提交，请等待平台审核</Text>
            <Text className='identity-application-status__copy'>提交时间：{readableTime(application.submittedAt)}</Text>
          </View>
        )}

        {!loading && !loadError && application?.status === 'approved' && (
          <View className='identity-application-card identity-application-status identity-application-status--approved'>
            <Text className='identity-application-status__badge'>已通过</Text>
            <Text className='identity-application-status__title'>身份申请审核通过</Text>
            <Text className='identity-application-status__copy'>返回“我的”刷新后，即可切换到对应身份。</Text>
          </View>
        )}

        {!loading && !loadError && application?.status === 'rejected' && (
          <View className='identity-application-card identity-application-status identity-application-status--rejected'>
            <Text className='identity-application-status__badge'>需修改</Text>
            <Text className='identity-application-status__title'>申请未通过，请修改后重新提交</Text>
            <Text className='identity-application-status__copy'>审核原因：{application.reviewReason || '资料不符合审核要求'}</Text>
          </View>
        )}

        {!loading && !loadError && (
          <>
            <View className={`identity-application-card identity-application-form ${editable ? '' : 'identity-application-form--readonly'}`}>
              <Text className='identity-application-card__title'>申请资料</Text>
              <View className='identity-application-field'>
                <Text className='identity-application-field__label'>真实姓名</Text>
                <Input className='identity-application-field__input' disabled={!editable} value={realName} maxlength={64} placeholder='请输入真实姓名' onInput={(event) => setRealName(String(event.detail.value ?? ''))} />
              </View>
              <View className='identity-application-field'>
                <Text className='identity-application-field__label'>手机号</Text>
                <Input className='identity-application-field__input' disabled={!editable} type='number' value={phone} maxlength={11} placeholder='请输入 11 位手机号' onInput={(event) => setPhone(String(event.detail.value ?? ''))} />
              </View>
              {role === 'merchant' && (
                <>
                  <View className='identity-application-field'>
                    <Text className='identity-application-field__label'>商家名称</Text>
                    <Input className='identity-application-field__input' disabled={!editable} value={merchantName} maxlength={120} placeholder='请输入商家名称' onInput={(event) => setMerchantName(String(event.detail.value ?? ''))} />
                  </View>
                  <View className='identity-application-field identity-application-field--last'>
                    <Text className='identity-application-field__label'>商家地址</Text>
                    <Input className='identity-application-field__input' disabled={!editable} value={merchantAddress} maxlength={240} placeholder='请输入校内经营地址' onInput={(event) => setMerchantAddress(String(event.detail.value ?? ''))} />
                  </View>
                </>
              )}
            </View>

            <View className='identity-application-card identity-application-proofs'>
              <View className='identity-application-card__header'>
                <View>
                  <Text className='identity-application-card__title'>证明图片</Text>
                  <Text className='identity-application-card__hint'>请上传 1 至 6 张，按当前顺序提交</Text>
                </View>
                <Text className='identity-application-card__count'>{proofs.length}/6</Text>
              </View>
              <View className='identity-application-proof-grid'>
                {proofs.map((proof, index) => (
                  <View className={`identity-application-proof identity-application-proof--${proof.state}`} key={proof.assetId}>
                    <View className='identity-application-proof__preview' onClick={() => void previewProof(proof)}>
                      {proof.previewPath
                        ? <Image src={proof.previewPath} mode='aspectFill' />
                        : <View className='identity-application-proof__private'><Text>隐私证明</Text><Text>第 {index + 1} 张</Text></View>}
                    </View>
                    <Text className='identity-application-proof__state'>{proofStateLabel(proof)}</Text>
                    {editable && <View className='identity-application-proof__remove' onClick={() => removeProof(index)}><Text>移除</Text></View>}
                  </View>
                ))}
                {editable && proofs.length < 6 && (
                  <View className={`identity-application-add ${busy ? 'identity-application-add--disabled' : ''}`} onClick={() => void chooseProofs()}>
                    <Text className='identity-application-add__plus'>＋</Text>
                    <Text>{uploading ? '上传中' : '添加证明'}</Text>
                  </View>
                )}
              </View>
              {progress && <Text className='identity-application-progress'>{progress}</Text>}
              <Text className='identity-application-privacy'>证明图片仅用于平台身份审核，不会通过公开媒体接口展示。</Text>
            </View>

            {editable && (
              <View className={`identity-application-submit ${busy ? 'identity-application-submit--disabled' : ''}`} onClick={() => void submit()}>
                <Text>{submitting ? '正在提交…' : application?.status === 'rejected' ? '重新提交申请' : '提交申请'}</Text>
              </View>
            )}
            {!editable && (
              <View className='identity-application-secondary' onClick={() => void Taro.navigateBack()}><Text>返回“我的”</Text></View>
            )}
          </>
        )}
      </View>
    </View>
  )
}

export default function IdentityApplicationPage() {
  return (
    <RuntimeSessionGate>
      <IdentityApplicationContent />
    </RuntimeSessionGate>
  )
}
