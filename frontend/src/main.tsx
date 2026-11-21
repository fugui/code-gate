import React from 'react'
import ReactDOM from 'react-dom/client'
import { setupChunkErrorReloader } from '@code/common'
import './index.css'
import App from './App'

setupChunkErrorReloader()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
