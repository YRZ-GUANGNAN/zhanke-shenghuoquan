import { useCallback, useEffect, useRef, useState } from 'react'
import { Image, Input, Text, View } from '@tarojs/components'
import { useDidShow } from '@tarojs/taro'
import CapsulePageHeader from '../../components/capsule-page-header'
import AppIcon from '../../components/app-icon'
import { AppButton, AppDialog, AppIconButton, AppSkeleton, AppState } from '../../components/ui'
import Taro from '../../services/taro'
import { cropImageWithWechatEditor } from '../../utils/crop-image'
import {
  getMediaStatus,
  isMediaUsable,
  uploadPickedImage
} from '../../services/media'
import {
  getStudentVerification,
  isStudentVerificationNotFound,
  requestStudentVerificationCancel,
  submitStudentVerification
} from '../../services/student-verification'
import { isApiError } from '../../services/api'
import { useSessionStore } from '../../stores/session'
import type { StudentVerificationView } from '../../types'
import './index.scss'

const MAX_PROOFS = 3
type PageStatus = 'loading' | 'ready' | 'error'

interface ProofDraft {
  assetId: string
  previewPath: string
}

function applicationStatusCopy(view: StudentVerificationView | null) {
  if (!view) return { text: '未认证', tone: 'muted', check: false }
  switch (view.status) {
    case 'pending': return { text: '待审核', tone: 'pending', check: false }
    case 'approved': return view.cancelRequested
      ? { text: '取消申请中', tone: 'pending', check: false }
      : { text: '已认证', tone: 'success', check: true }
    case 'rejected': return { text: '未通过', tone: 'danger', check: false }
    case 'cancelled': return { text: '已解除', tone: 'muted', check: false }
    default: return { text: '未认证', tone: 'muted', check: false }
  }
}

export default function StudentVerifyPage() {
  const token = useSessionStore((state) => state.token)
  const [pageStatus, setPageStatus] = useState<PageStatus>('loading')
  const [application, setApplication] = useState<StudentVerificationView | null>(null)
  const [editing, setEditing] = useState(false)
  const [realName, setRealName] = useState('')
  const [studentNo, setStudentNo] = useState('')
  const [college, setCollege] = useState('')
  const [proofs, setProofs] = useState<ProofDraft[]>([])
  const [uploadBusy, setUploadBusy] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [progress, setProgress] = useState('')
  const [notice, setNotice] = useState('')
  const [cancelConfirmVisible, setCancelConfirmVisible] = useState(false)
  const mountedRef = useRef(true)
  const requestSequenceRef = useRef(0)
  const tokenRef = useRef(token)
  tokenRef.current = token

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      requestSequenceRef.current += 1
    }
  }, [])

  const applyApplication = useCallback((view: StudentVerificationView | null) => {
    setApplication(view)
    setEditing(false)
    setProofs(view ? view.assetIds.map((assetId) => ({ assetId, previewPath: '' })) : [])
    setRealName(view?.realName ?? '')
    setStudentNo(view?.studentNo ?? '')
    setCollege(view?.college ?? '')
  }, [])

  const load = useCallback(async ({ silent = false }: { silent?: boolean } = {}) => {
    const sequence = ++requestSequenceRef.current
    const active = () => mountedRef.current && requestSequenceRef.current === sequence
    if (!silent) setPageStatus('loading')
    setNotice('')
    try {
      const view = await getStudentVerification()
      if (!active()) return
      applyApplication(view)
      setPageStatus('ready')
    } catch (error) {
      if (!active()) return
      if (isStudentVerificationNotFound(error)) {
        applyApplication(null)
        setPageStatus('ready')
        return
      }
      if (!silent) setPageStatus('error')
    }
  }, [applyApplication])

  useEffect(() => { void load() }, [load])
  useDidShow(() => { if (pageStatus === 'ready') void load({ silent: true }) })

  const startReapply = () => {
    setEditing(true)
    setNotice('')
  }

  const pickProofs = async () => {
    if (uploadBusy || !editing) return
    const remaining = MAX_PROOFS - proofs.length
    if (remaining <= 0) return
    let files: Array<{ tempFilePath: string }> = []
    try {
      const media = await Taro.chooseMedia({ count: remaining, mediaType: ['image'], sourceType: ['album', 'camera'] })
      files = media.tempFiles ?? []
    } catch {
      return
    }
    if (files.length === 0) return
    setUploadBusy(true)
    setNotice('')
    try {
      const accepted: ProofDraft[] = []
      for (const file of files) {
        const crop = await cropImageWithWechatEditor(file.tempFilePath)
        if (crop.status === 'cancelled') continue
        if (crop.status === 'unchanged') void Taro.showToast({ title: '当前环境暂不支持裁剪，已使用原图', icon: 'none' })
        setProgress('正在上传学生证照片…')
        const uploaded = await uploadPickedImage(
          crop.path,
          'student_verification',
          (_percent, _stage, message) => { if (mountedRef.current) setProgress(message) },
          () => mountedRef.current
        )
        const status = await getMediaStatus(uploaded.assetId)
        if (!isMediaUsable(status)) {
          void Taro.showToast({ title: '照片未通过校验，请更换后重试', icon: 'none' })
          continue
        }
        accepted.push({ assetId: uploaded.assetId, previewPath: uploaded.previewPath })
      }
      if (mountedRef.current && accepted.length > 0) {
        setProofs((current) => {
          const seen = new Set(current.map((item) => item.assetId))
          return [...current, ...accepted.filter((item) => !seen.has(item.assetId))].slice(0, MAX_PROOFS)
        })
      }
    } catch (error) {
      if (mountedRef.current) {
        void Taro.showToast({ title: error instanceof Error ? error.message : '照片上传失败，请重试', icon: 'none' })
      }
    } finally {
      if (mountedRef.current) {
        setUploadBusy(false)
        setProgress('')
      }
    }
  }

  const removeProof = (assetId: string) => {
    if (!editing) return
    setProofs((current) => current.filter((item) => item.assetId !== assetId))
  }

  const submit = async () => {
    if (!editing || submitting) return
    if (!realName.trim() || !studentNo.trim()) {
      void Taro.showToast({ title: '请填写姓名和学号', icon: 'none' })
      return
    }
    if (proofs.length === 0) {
      void Taro.showToast({ title: '请上传学生证照片', icon: 'none' })
      return
    }
    const expectedRowVersion = application && (application.status === 'rejected' || application.status === 'cancelled')
      ? application.rowVersion
      : 0
    const sequence = ++requestSequenceRef.current
    setSubmitting(true)
    setNotice('')
    try {
      const view = await submitStudentVerification({
        realName: realName.trim(),
        studentNo: studentNo.trim(),
        college: college.trim(),
        assetIds: proofs.map((item) => item.assetId),
        expectedRowVersion
      })
      if (!mountedRef.current || requestSequenceRef.current !== sequence) return
      applyApplication(view)
      void Taro.showToast({ title: '已提交，等待审核', icon: 'success' })
    } catch (error) {
      if (!mountedRef.current || requestSequenceRef.current !== sequence) return
      if (isApiError(error) && error.status === 409) {
        setNotice('认证状态已被更新，已为你刷新最新状态')
        await load({ silent: true })
      } else {
        setNotice(error instanceof Error ? error.message : '提交失败，请重试')
      }
    } finally {
      if (mountedRef.current) setSubmitting(false)
    }
  }

  const requestCancelConfirmed = async () => {
    if (!application || application.status !== 'approved' || application.cancelRequested) return
    setCancelConfirmVisible(false)
    setSubmitting(true)
    setNotice('')
    try {
      const view = await requestStudentVerificationCancel(application.rowVersion)
      if (!mountedRef.current) return
      setApplication(view)
      void Taro.showToast({ title: '取消申请已提交', icon: 'success' })
    } catch (error) {
      if (!mountedRef.current) return
      if (isApiError(error) && error.status === 409) {
        setNotice('认证状态已被更新，已为你刷新最新状态')
        await load({ silent: true })
      } else {
        setNotice(error instanceof Error ? error.message : '申请提交失败，请重试')
      }
    } finally {
      if (mountedRef.current) setSubmitting(false)
    }
  }

  const statusCopy = applicationStatusCopy(application)

  return (
    <View className='student-verify-page'>
      <CapsulePageHeader
        title='学生验证'
        divider={false}
        left={<AppIconButton name='back' label='返回' className='student-verify-nav__back' onClick={() => void Taro.navigateBack()} />}
      />

      <View className='student-verify-content'>
        {pageStatus === 'loading' && <AppSkeleton rows={4} />}
        {pageStatus === 'error' && (
          <AppState kind='error' title='学生认证加载失败' description='请检查网络后重试。' actionText='重新加载' onAction={() => void load()} />
        )}
        {pageStatus === 'ready' && !application && !editing && (
          <View className='student-verify-hero'>
            <View className='student-verify-hero__icon'>
              <AppIcon name='student' className='student-verify-hero__icon-image' />
            </View>
            <Text className='student-verify-hero__title'>点亮学生标识</Text>
            <Text className='student-verify-hero__desc'>提交学生证照片与在校信息，管理员人工核验通过后，你的主页与互动将展示学生标识。</Text>
            <View className='student-verify-hero__points'>
              <Text className='student-verify-hero__point'>认证信息仅用于身份核验，不会公开展示</Text>
              <Text className='student-verify-hero__point'>审核进度与结果会在本页实时更新</Text>
              <Text className='student-verify-hero__point'>认证通过后可随时申请取消</Text>
            </View>
            <View className='student-verify-hero__action'>
              <AppButton variant='primary' block onClick={startReapply}>开始学生认证</AppButton>
            </View>
            <Text className='student-verify-hero__hint'>请准备学生证内页照片，确保姓名、学号清晰可见</Text>
          </View>
        )}
        {pageStatus === 'ready' && (application || editing) && (
          <>
            <View className='student-verify-card'>
              <View className='student-verify-row'>
                <Text className='student-verify-row__title'>认证状态</Text>
                <View className={`student-verify-state student-verify-state--${statusCopy.tone}`}>
                  {statusCopy.check ? <View className='student-verify-state__check' /> : null}
                  <Text>{statusCopy.text}</Text>
                </View>
              </View>
              <>
              <View className='student-verify-row student-verify-row--media'>
                <View className='student-verify-row__main'>
                  <Text className='student-verify-row__title'>学生证照片</Text>
                  <Text className='student-verify-row__subtitle'>
                    {editing
                      ? (uploadBusy ? (progress || '正在处理照片…') : `已上传 ${proofs.length}/${MAX_PROOFS} 张，点击照片可放大预览`)
                      : `已提交 ${proofs.length} 张`}
                  </Text>
                </View>
                {editing ? (
                  <View className='student-verify-proofs'>
                    {proofs.map((proof) => (
                      <View key={proof.assetId} className='student-verify-proof'>
                        {proof.previewPath ? (
                          <Image
                            className='student-verify-proof__image'
                            src={proof.previewPath}
                            mode='aspectFill'
                            onClick={() => void Taro.previewImage({
                              urls: proofs.map((item) => item.previewPath).filter(Boolean),
                              current: proof.previewPath
                            })}
                          />
                        ) : (
                          <View className='student-verify-proof__placeholder'><Text>已上传</Text></View>
                        )}
                        <View className='student-verify-proof__remove' onClick={() => removeProof(proof.assetId)}>
                          <Text>×</Text>
                        </View>
                      </View>
                    ))}
                    {proofs.length < MAX_PROOFS && (
                      <View className='student-verify-proof student-verify-proof--add' onClick={() => void pickProofs()}>
                        <Text className='student-verify-thumb__plus'>+</Text>
                        <Text className='student-verify-thumb__text'>上传学生证</Text>
                      </View>
                    )}
                  </View>
                ) : null}
              </View>
              <View className='student-verify-row'>
                <Text className='student-verify-row__title'>姓名</Text>
                {editing
                  ? <Input className='student-verify-input' value={realName} maxlength={20} placeholder='请输入真实姓名' onInput={(event) => setRealName(String(event.detail.value ?? ''))} />
                  : <Text className='student-verify-value'>{realName || '未填写'}</Text>}
              </View>
              <View className='student-verify-row'>
                <Text className='student-verify-row__title'>学号</Text>
                {editing
                  ? <Input className='student-verify-input' value={studentNo} maxlength={30} placeholder='请输入学号' onInput={(event) => setStudentNo(String(event.detail.value ?? ''))} />
                  : <Text className='student-verify-value'>{studentNo || '未填写'}</Text>}
              </View>
              <View className='student-verify-row student-verify-row--last'>
                <Text className='student-verify-row__title'>学院</Text>
                {editing
                  ? <Input className='student-verify-input' value={college} maxlength={30} placeholder='请输入学院（选填）' onInput={(event) => setCollege(String(event.detail.value ?? ''))} />
                  : <Text className='student-verify-value'>{college || '未填写'}</Text>}
              </View>
              </>
            </View>

            {application?.status === 'rejected' && application.reviewReason ? (
              <View className='student-verify-reason'>
                <Text className='student-verify-reason__label'>驳回原因</Text>
                <Text className='student-verify-reason__text'>{application.reviewReason}</Text>
              </View>
            ) : null}
            {application?.status === 'cancelled' && application.reviewReason ? (
              <View className='student-verify-reason'>
                <Text className='student-verify-reason__label'>解除说明</Text>
                <Text className='student-verify-reason__text'>{application.reviewReason}</Text>
              </View>
            ) : null}

            {notice ? <Text className='student-verify-notice'>{notice}</Text> : null}

            <View className='student-verify-actions'>
              {editing ? (
                <>
                  <AppButton variant='primary' block loading={submitting} onClick={() => void submit()}>
                    {application ? '重新提交认证' : '提交认证'}
                  </AppButton>
                  <View className='student-verify-link' onClick={() => {
                    if (submitting || uploadBusy) return
                    if (application) applyApplication(application)
                    else setEditing(false)
                  }}>
                    <Text>{application ? '取消修改' : '取消'}</Text>
                  </View>
                </>
              ) : null}
              {!editing && !application ? (
                <AppButton variant='primary' block onClick={startReapply}>开始认证</AppButton>
              ) : null}
              {!editing && application?.status === 'pending' ? (
                <Text className='student-verify-pending-hint'>照片与信息已提交，等待后台审核；审核结果会显示在本页。</Text>
              ) : null}
              {!editing && application?.status === 'approved' && !application.cancelRequested ? (
                <View className='student-verify-link student-verify-link--danger' onClick={() => setCancelConfirmVisible(true)}>
                  <Text>{submitting ? '正在提交…' : '申请取消认证'}</Text>
                </View>
              ) : null}
              {!editing && application?.status === 'approved' && application.cancelRequested ? (
                <Text className='student-verify-pending-hint'>取消申请已提交，等待后台处理；处理完成前认证保持有效。</Text>
              ) : null}
              {!editing && (application?.status === 'rejected' || application?.status === 'cancelled') ? (
                <AppButton variant='primary' block onClick={startReapply}>重新提交认证</AppButton>
              ) : null}
            </View>
          </>
        )}
      </View>
      <AppDialog
        visible={cancelConfirmVisible}
        title='申请取消学生认证'
        description='提交后需由后台处理，处理完成前认证保持有效；处理完成后可重新提交认证。'
        confirmText='提交申请'
        loading={submitting}
        onCancel={() => setCancelConfirmVisible(false)}
        onConfirm={() => void requestCancelConfirmed()}
      />
    </View>
  )
}
