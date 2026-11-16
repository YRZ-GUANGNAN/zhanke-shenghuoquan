// 学生认证闭环：提交、查询与取消申请。
import type { StudentVerificationSubmitInput, StudentVerificationView } from '../types'
import { isApiError, request } from './api'

export async function getStudentVerification(): Promise<StudentVerificationView> {
  return request<StudentVerificationView>('/auth/student-verification', { url: '', timeout: 12000 })
}

export async function submitStudentVerification(input: StudentVerificationSubmitInput): Promise<StudentVerificationView> {
  return request<StudentVerificationView>('/auth/student-verification', {
    url: '',
    method: 'PUT',
    data: input,
    timeout: 20000
  })
}

export async function requestStudentVerificationCancel(expectedRowVersion: number): Promise<StudentVerificationView> {
  return request<StudentVerificationView>('/auth/student-verification/cancel-request', {
    url: '',
    method: 'POST',
    data: { expectedRowVersion },
    timeout: 15000
  })
}

export function isStudentVerificationNotFound(error: unknown): boolean {
  return isApiError(error) && error.status === 404
}
