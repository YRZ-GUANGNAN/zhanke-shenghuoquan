import { useSessionStore } from '../stores/session'
import { isApiError } from './api'

export async function requestWithSingleSessionRecovery<T>(operation:()=>Promise<T>):Promise<T>{
  try{
    return await operation()
  }catch(error){
    if(!isApiError(error)||error.status!==401)throw error
    await useSessionStore.getState().retry()
    if(useSessionStore.getState().status!=='authenticated')throw error
    return operation()
  }
}
