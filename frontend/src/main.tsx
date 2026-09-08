import React from 'react'
import ReactDOM from 'react-dom/client'
import { setupChunkErrorReloader } from '@code/common'
import App from './App'
import './index.css'

setupChunkErrorReloader()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
