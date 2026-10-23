import { withFeaturePage } from '../../components/feature-page-gate'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Image, ScrollView, Text, View } from '@tarojs/components'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppButton, AppField, AppIconButton, AppNotice, AppSkeleton, AppState } from '../../components/ui'
import { StatusGlyph } from '../../components/ui/status-glyph'
import { ApplicationActivity, ApplicationSummary, ApplicationTimeline, applicationRoleCopy as roleCopy } from '../../components/identity-application-summary'
import RuntimeSessionGate from '../../components/runtime-session-gate'
import { getIdentityApplication, submitIdentityApplication } from '../../services/a-user'
import { isApiError } from '../../services/api'
import { chooseAndUploadImagesSettled, getMediaStatus, isMediaUsable, type UploadedMedia } from '../../services/media'
import Taro from '../../services/taro'
import { useSessionStore } from '../../stores/session'
import type { IdentityApplicationRole, IdentityApplicationView } from '../../types'
import './index.scss'

type ProofState = 'stored' | 'uploaded' | 'checking' | 'ready' | 'failed'

interface ProofItem {
  assetId: string
  previewPath: string
  state: ProofState
  message: string
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
    message: '证明已保存，仅供平台核验'
  }
}

function uploadedProof(value: UploadedMedia): ProofItem {
  return {
    assetId: value.assetId,
    previewPath: value.previewPath,
    state: isMediaUsable(value) ? 'ready' : 'uploaded',
    message: value.message || '图片已上传'
  }
}

function proofStateLabel(value: ProofItem): string {
  if (value.state === 'checking') return value.message || '正在确认上传状态'
  if (value.state === 'ready') return '材料已上传，提交申请后由管理员核验'
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

function IdentityApplicationContent() {
  const role = useMemo(readRole, [])
  const mountedRef = useRef(true)
  const leavingRef = useRef(false)
  const operationGenerationRef = useRef(0)
  const loadSequenceRef = useRef(0)
  const uploadLockRef = useRef(false)
  const submitLockRef = useRef(false)
  const loadLockRef = useRef(false)
  const ownerRef = useRef(useSessionStore.getState())
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
  const [hasLoaded, setHasLoaded] = useState(false)
  const [validationAttempted, setValidationAttempted] = useState(false)

  const sessionIsCurrent = useCallback(() => {
    const session = useSessionStore.getState()
    return session.status === 'authenticated' && session.token === ownerRef.current.token
      && session.profile.id === ownerRef.current.profile.id
  }, [])

  const applyApplication = useCallback((value: IdentityApplicationView) => {
    setApplication(value)
    setRealName(value.realName)
    setPhone(value.phone)
    setMerchantName(value.merchantName)
    setMerchantAddress(value.merchantAddress)
    setProofs((current) => value.assetIds.map((assetId) => current.find((item) => item.assetId === assetId) ?? existingProof(assetId)))
  }, [])

  const isOperationActive = useCallback((generation: number) => (
    mountedRef.current && !leavingRef.current && sessionIsCurrent() && operationGenerationRef.current === generation
  ), [sessionIsCurrent])

  useEffect(() => {
    mountedRef.current = true
    leavingRef.current = false
    return () => {
      mountedRef.current = false
      leavingRef.current = true
      operationGenerationRef.current += 1
      loadSequenceRef.current += 1
      uploadLockRef.current = false
      submitLockRef.current = false
      loadLockRef.current = false
    }
  }, [])

  const loadApplication = useCallback(async () => {
    if (!role || !mountedRef.current || leavingRef.current || !sessionIsCurrent() || loadLockRef.current) return
    loadLockRef.current = true
    const requestId = ++loadSequenceRef.current
    setLoading(true)
    setLoadError('')
    try {
      const value = await getIdentityApplication(role)
      if (!mountedRef.current || leavingRef.current || !sessionIsCurrent() || requestId !== loadSequenceRef.current) return
      if (value.role !== role || value.userId !== ownerRef.current.profile.id) throw new Error('申请资料与当前账号不一致，请重新进入')
      applyApplication(value)
      setHasLoaded(true)
    } catch (error) {
      if (!mountedRef.current || leavingRef.current || !sessionIsCurrent() || requestId !== loadSequenceRef.current) return
      if (isApiError(error) && error.status === 404) {
        setApplication(null)
        setProofs([])
        setHasLoaded(true)
      } else {
        setLoadError(applicationErrorMessage(error))
      }
    } finally {
      if (mountedRef.current && !leavingRef.current && sessionIsCurrent() && requestId === loadSequenceRef.current) {
        loadLockRef.current = false
        setLoading(false)
      }
    }
  }, [applyApplication, role, sessionIsCurrent])

  useEffect(() => {
    void loadApplication()
  }, [loadApplication])

  const editable = !application || application.status === 'rejected'
  const busy = uploading || submitting
  const copy = role ? roleCopy[role] : null

  const updateProof = (assetId: string, changes: Partial<ProofItem>) => {
    if (!mountedRef.current || leavingRef.current) return
    setProofs((current) => current.map((item) => item.assetId === assetId ? { ...item, ...changes } : item))
  }

  const chooseProofs = async () => {
    if (!editable || !hasLoaded || loadError || loadLockRef.current || leavingRef.current || !sessionIsCurrent() || uploadLockRef.current || submitLockRef.current) return
    uploadLockRef.current = true
    const generation = ++operationGenerationRef.current
    try {
      if (proofs.length >= 6) {
        await Taro.showToast({ title: '最多上传 6 张证明图片', icon: 'none' })
        return
      }
      if (!isOperationActive(generation)) return
      setUploading(true)
      setProgress('正在选择证明图片')
      const settlements = await chooseAndUploadImagesSettled('identity_application', 6 - proofs.length, (value) => {
        if (!isOperationActive(generation)) return
        setProgress(`第 ${value.index + 1}/${value.total} 张 · ${value.message}`)
      })
      if (!isOperationActive(generation)) return
      const uploaded = settlements.reduce<UploadedMedia[]>((values, item) => {
        if (item.status === 'fulfilled') values.push(item.value)
        return values
      }, [])
      const failed = settlements.filter((item) => item.status === 'rejected')
      setProofs((current) => {
        const seen = new Set(current.map((item) => item.assetId))
        const next = [...current]
        for (const value of uploaded) {
          if (seen.has(value.assetId)) continue
          seen.add(value.assetId)
          next.push(uploadedProof(value))
        }
        return next.slice(0, 6)
      })
      const failureDetail = failed.map((item) => `第 ${item.index + 1} 张：${item.reason.message}`).join('；')
      const summary = failed.length > 0
        ? `已保留 ${uploaded.length} 张上传成功的图片。${failureDetail}`
        : uploaded.length > 0 ? '材料已上传，可提交申请，由管理员人工核验' : ''
      setProgress(summary)
      if (failed.length > 0 && isOperationActive(generation)) {
        await Taro.showToast({ title: summary, icon: 'none', duration: 2400 })
      }
    } catch (error) {
      if (!isOperationActive(generation)) return
      const message = error instanceof Error ? error.message : '证明图片上传失败，请重新选择'
      setProgress(message)
      await Taro.showToast({ title: message, icon: 'none', duration: 2200 })
    } finally {
      if (isOperationActive(generation)) {
        uploadLockRef.current = false
        setUploading(false)
      }
    }
  }

  const removeProof = (index: number) => {
    if (!editable || loadLockRef.current || leavingRef.current || !sessionIsCurrent() || uploadLockRef.current || submitLockRef.current) return
    setProofs((current) => current.filter((_, currentIndex) => currentIndex !== index))
  }

  const previewProof = async (value: ProofItem) => {
    if (!value.previewPath) {
      await Taro.showToast({ title: '历史证明已安全保存，仅供平台核验', icon: 'none' })
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
    if (!role || !editable || !hasLoaded || loadError || loadLockRef.current || leavingRef.current || !sessionIsCurrent() || uploadLockRef.current || submitLockRef.current) return
    submitLockRef.current = true
    const generation = ++operationGenerationRef.current
    try {
      const validation = validate()
      setValidationAttempted(true)
      if (validation) {
        setProgress(validation)
        await Taro.showToast({ title: validation, icon: 'none' })
        return
      }
      if (!isOperationActive(generation)) return
      setSubmitting(true)
      setProgress('正在确认证明材料是否上传完成')
      for (const proof of proofs) {
        if (!isOperationActive(generation)) return
        updateProof(proof.assetId, { state: 'checking', message: '正在确认上传状态' })
        const status = await getMediaStatus(proof.assetId)
        if (!isOperationActive(generation)) return
        if (status.purpose === 'identity_application' && isMediaUsable(status)) {
          updateProof(proof.assetId, { state: 'ready', message: '材料已上传，提交后由管理员人工核验' })
          continue
        }
        if (status.state === 'rejected' || status.state === 'review_failed' ||
          status.state === 'expired' || status.state === 'upload_failed' || status.state === 'deleted') {
          const message = status.state === 'rejected'
            ? '照片未通过内容安全审核，请重新拍摄'
            : status.message || '图片处理失败，请重新上传'
          updateProof(proof.assetId, {
            state: 'failed',
            message
          })
          throw new Error(message)
        }

        const pendingMessage = status.message || '证明材料尚未上传完成，请稍后重试'
        updateProof(proof.assetId, { state: 'uploaded', message: pendingMessage })
        setProgress(`${pendingMessage}。无需停留在此页，稍后返回重试即可。`)
        await Taro.showToast({ title: '材料尚不可用，请重试或重新上传', icon: 'none', duration: 2400 })
        return
      }

      if (!isOperationActive(generation)) return
      const result = await submitIdentityApplication(role, {
        realName: realName.trim(),
        phone: phone.trim(),
        merchantName: role === 'merchant' ? merchantName.trim() : '',
        merchantAddress: role === 'merchant' ? merchantAddress.trim() : '',
        assetIds: proofs.map((item) => item.assetId),
        expectedRowVersion: application?.status === 'rejected' ? application.rowVersion : 0
      })
      if (!isOperationActive(generation)) return
      applyApplication(result)
      setValidationAttempted(false)
      setProgress('')
      await Taro.showToast({ title: '申请已提交，等待人工审核', icon: 'success', duration: 1800 })
    } catch (error) {
      if (!isOperationActive(generation)) return
      const message = applicationErrorMessage(error)
      setProgress(message)
      if (isApiError(error) && error.status === 409) {
        await loadApplication()
        if (!isOperationActive(generation)) return
      }
      await Taro.showToast({ title: message, icon: 'none', duration: 2400 })
    } finally {
      if (isOperationActive(generation)) {
        submitLockRef.current = false
        setSubmitting(false)
      }
    }
  }

  const restoreAfterNavigationFailure = (cancelGeneration: number) => {
    if (!mountedRef.current || !leavingRef.current || operationGenerationRef.current !== cancelGeneration) return false
    leavingRef.current = false
    uploadLockRef.current = false
    submitLockRef.current = false
    loadLockRef.current = false
    setUploading(false)
    setSubmitting(false)
    setProgress('')
    setLoading(false)
    void loadApplication()
    return true
  }

  const cancelAndBack = async () => {
    if (!mountedRef.current || leavingRef.current) return
    const cancelling = uploadLockRef.current || submitLockRef.current
    leavingRef.current = true
    const cancelGeneration = ++operationGenerationRef.current
    const isCancelCurrent = () => mountedRef.current
      && leavingRef.current
      && operationGenerationRef.current === cancelGeneration
    loadSequenceRef.current += 1
    uploadLockRef.current = false
    submitLockRef.current = false
    loadLockRef.current = false
    if (mountedRef.current) {
      setUploading(false)
      setSubmitting(false)
      setProgress(cancelling ? '当前操作已取消' : '')
    }
    if (cancelling && mountedRef.current) {
      try {
        await Taro.showToast({ title: '已取消当前操作', icon: 'none', duration: 1200 })
      } catch {
        // 提示失败不能阻止返回流程。
      }
    }
    if (!isCancelCurrent()) return
    try {
      await Taro.navigateBack()
      return
    } catch {
    }
    if (!isCancelCurrent()) return
    try {
      await Taro.reLaunch({ url: '/pages/profile/index' })
      return
    } catch {
      // 两种导航都失败后才允许恢复旧页交互。
    }
    if (!isCancelCurrent() || !restoreAfterNavigationFailure(cancelGeneration)) return
    void Taro.showToast({ title: '返回失败，请重试', icon: 'none' }).catch(() => {
      // 最终提示失败不再触碰已恢复页面的任何状态。
    })
  }

  if (!role || !copy) {
    return <View className='identity-application-page'>
      <CapsulePageHeader title='身份申请' left={<AppIconButton name='back' label='返回' className='identity-application-nav__back' onClick={() => void cancelAndBack()} />} />
      <AppState kind='error' title='申请类型无效' description='请选择商家或骑手身份后重新进入。' actionText='返回' onAction={() => void cancelAndBack()} />
    </View>
  }

  const fieldsLocked = busy || loading || Boolean(loadError)
  const errors = validationAttempted ? {
    realName: realName.trim() ? '' : '请填写真实姓名',
    phone: /^1[3-9][0-9]{9}$/.test(phone.trim()) ? '' : '请填写正确的 11 位手机号',
    merchantName: merchantName.trim() ? '' : '请填写商家名称',
    merchantAddress: merchantAddress.trim() ? '' : '请填写商家地址',
    proofs: proofs.length < 1 || proofs.length > 6 ? '请上传 1 至 6 张证明图片' : '',
  } : null
  const details = [
    { label: role === 'merchant' ? '负责人姓名' : '真实姓名', value: realName },
    { label: '联系电话', value: phone },
    ...(role === 'merchant' ? [{ label: '商家名称', value: merchantName }, { label: '经营地址', value: merchantAddress }] : []),
  ]

  return <View className='identity-application-page'>
    <CapsulePageHeader title={copy.title} left={<AppIconButton name='back' label='返回' className='identity-application-nav__back' onClick={() => void cancelAndBack()} />} />
    <ScrollView scrollY className='identity-application-scroll'>
      <View className='identity-application-content'>
        {!hasLoaded && loading && <View className='identity-application-loading' ariaLabel='正在读取申请资料'>
          <AppSkeleton rows={1} /><AppSkeleton rows={3} />
          <View className='identity-application-progress'><ApplicationActivity /><Text>正在读取申请资料</Text></View>
        </View>}
        {!hasLoaded && !loading && loadError && <AppState kind='error' title='申请状态暂未确认' description={loadError} actionText='重新读取' onAction={() => void loadApplication()} />}
        {hasLoaded && <>
          <View className='identity-application-card identity-application-overview'><ApplicationSummary role={role} application={application} /></View>
          {loadError && <View className='identity-application-recovery'>
            <AppNotice tone='warning' title='暂时无法更新申请状态'>{loadError} 已读取的资料仍保留，重试成功后可继续操作。</AppNotice>
            <AppButton variant='secondary' disabled={busy || loading} onClick={() => void loadApplication()}>重新读取</AppButton>
          </View>}
          {application?.status === 'rejected' && <View className='identity-application-review-notice'><AppNotice tone='danger' title='审核意见'>{application.reviewReason || '资料不符合审核要求，请核对后重新提交。'}</AppNotice></View>}
          {application && <View className='identity-application-card'><ApplicationTimeline application={application} /></View>}

          <View className='identity-application-card identity-application-form'>
            <View className='identity-application-section-heading'><Text className='identity-application-section-title'>{editable ? '填写申请资料' : '已提交资料'}</Text><Text className='identity-application-section-meta'>{editable ? '以下均为必填' : '仅本人可见'}</Text></View>
            {editable ? <View className='identity-application-fields'>
              {role === 'merchant' && <AppField label='商家名称' value={merchantName} placeholder='填写实际经营的商家名称' maxLength={120} disabled={fieldsLocked} error={errors?.merchantName} onInput={setMerchantName} />}
              <AppField label={role === 'merchant' ? '负责人姓名' : '真实姓名'} value={realName} placeholder='填写证件上的真实姓名' maxLength={64} disabled={fieldsLocked} error={errors?.realName} onInput={setRealName} />
              <AppField label='联系电话' value={phone} inputType='number' placeholder='填写 11 位手机号码' maxLength={11} disabled={fieldsLocked} error={errors?.phone} onInput={setPhone} />
              {role === 'merchant' && <AppField label='经营地址' value={merchantAddress} multiline placeholder='填写校内经营地址、楼栋与店铺位置' maxLength={240} disabled={fieldsLocked} error={errors?.merchantAddress} onInput={setMerchantAddress} />}
            </View> : <View className='identity-application-details'>{details.map(item => <View className='identity-application-detail' key={item.label}><Text className='identity-application-detail__label'>{item.label}</Text><Text className='identity-application-detail__value' selectable>{item.value || '未填写'}</Text></View>)}</View>}
          </View>

          <View className='identity-application-card identity-application-proofs'>
            <View className='identity-application-section-heading'><Text className='identity-application-section-title'>证明材料</Text><Text className='identity-application-section-meta'>{proofs.length} / 6 张</Text></View>
            <Text className='identity-application-section-description'>{editable ? '上传 1–6 张清晰图片，确保关键信息完整可辨。' : '材料已安全保存，由平台用于身份核验。'}</Text>
            <View className='identity-application-proof-grid'>
              {proofs.map((proof, index) => <View className={`identity-application-proof identity-application-proof--${proof.state}`} key={proof.assetId}>
                {proof.previewPath ? <View className='identity-application-proof__preview' ariaRole='button' ariaLabel={`预览第 ${index + 1} 张证明`} onClick={() => void previewProof(proof)}>
                  <Image src={proof.previewPath} mode='aspectFill' onError={() => updateProof(proof.assetId, { previewPath: '' })} />
                  <Text className='identity-application-proof__number'>证明 {index + 1}</Text>
                </View> : <View className='identity-application-proof__private'><StatusGlyph kind='forbidden' small /><Text>私密证明 {index + 1}</Text><Text>已保存 · 仅供平台核验</Text></View>}
                <Text className='identity-application-proof__state'>{proofStateLabel(proof)}</Text>
                {editable && <AppButton className='identity-application-proof__remove' variant='text' disabled={fieldsLocked} onClick={() => removeProof(index)}>移除图片</AppButton>}
              </View>)}
              {editable && proofs.length < 6 && <Button className='identity-application-add' disabled={fieldsLocked} onClick={() => void chooseProofs()}>
                <View className='identity-application-add__content'><View className='identity-application-add__icon'>{uploading ? <ApplicationActivity /> : <AppIcon name='image' />}</View><Text>{uploading ? '正在上传' : '添加证明图片'}</Text><Text className='identity-application-add__hint'>拍照或从相册选择</Text></View>
              </Button>}
            </View>
            {errors?.proofs && <Text className='identity-application-field-error'>{errors.proofs}</Text>}
            {progress && <View className={`identity-application-progress${busy ? ' identity-application-progress--busy' : ''}`}>{busy && <ApplicationActivity />}<Text>{progress}</Text></View>}
            <View className='identity-application-privacy'><StatusGlyph kind='forbidden' small /><Text>证明材料仅用于身份审核，不会公开展示。</Text></View>
          </View>
        </>}
      </View>
    </ScrollView>
    {hasLoaded && <View className='identity-application-footer'>
      <Text className='identity-application-footer__hint'>{busy ? uploading ? '正在上传材料，请稍候' : '正在核对并提交，请勿重复操作' : editable ? '请确认所填资料真实、准确' : application?.status === 'approved' ? '已通过审核，可返回“我的”切换身份' : application?.status === 'pending' ? '审核中无需重复提交，可刷新查看结果' : '查看当前申请状态'}</Text>
      <View className='identity-application-footer__actions'>
        <AppButton variant='ghost' className='identity-application-secondary' onClick={() => void cancelAndBack()}>{busy ? '取消并返回' : '返回'}</AppButton>
        {editable ? <AppButton className='identity-application-submit' disabled={fieldsLocked} onClick={() => void submit()}>{submitting ? '正在提交申请' : uploading ? '正在上传材料' : application?.status === 'rejected' ? '重新提交申请' : '提交申请'}</AppButton>
          : application?.status === 'pending' ? <AppButton className='identity-application-refresh' disabled={busy || loading} onClick={() => void loadApplication()}>{loading ? '正在刷新状态' : '刷新申请状态'}</AppButton>
            : <AppButton onClick={() => void cancelAndBack()}>返回“我的”</AppButton>}
      </View>
    </View>}
  </View>
}

function IdentityApplicationPage() {
  const accountScope = useSessionStore((state) => `${state.profile.id}:${state.token}`)
  return <RuntimeSessionGate><IdentityApplicationContent key={accountScope} /></RuntimeSessionGate>
}

export default withFeaturePage(IdentityApplicationPage)
