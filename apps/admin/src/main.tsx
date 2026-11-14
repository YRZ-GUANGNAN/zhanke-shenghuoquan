import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './preview/App'
import { AdminDialogProvider } from './components'
import 'antd/dist/antd.css'
import './styles.css'
import './preview/styles.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AdminDialogProvider><App /></AdminDialogProvider>
  </React.StrictMode>
)
