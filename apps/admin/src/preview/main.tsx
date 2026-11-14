import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider } from 'antd'
import zhCN from 'antd/lib/locale/zh_CN'
import 'antd/dist/antd.css'
import App from './App'
import './styles.css'

ReactDOM.createRoot(document.getElementById('preview-root')!).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN} pageHeader={{ ghost: false }}>
      <App />
    </ConfigProvider>
  </React.StrictMode>
)

