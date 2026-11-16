import Taro from '../services/taro'

// wx.saveFile / wx.removeSavedFile 已废弃，统一下沉到 FileSystemManager 的 Promise 封装。
export function saveLocalFile(tempFilePath: string): Promise<string> {
  return new Promise((resolve, reject) => {
    try {
      Taro.getFileSystemManager().saveFile({
        tempFilePath,
        success: (result) => resolve(result.savedFilePath),
        fail: reject
      })
    } catch (error) {
      reject(error)
    }
  })
}

export function removeLocalFile(filePath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    try {
      Taro.getFileSystemManager().removeSavedFile({
        filePath,
        success: () => resolve(),
        fail: reject
      })
    } catch (error) {
      reject(error)
    }
  })
}
