import React from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Header } from './components/Header'
import { ChatPage } from './pages/Chat'
import { KeysPage } from './pages/Keys'
import { DashboardPage } from './pages/Dashboard'
import { AdminUsersPage } from './pages/AdminUsers'
import { AdminBackendsPage } from './pages/AdminBackends'
import { LogsPage } from './pages/Logs'

export interface AppProps {
  isEmbedded?: boolean
}

export const App: React.FC<AppProps> = ({ isEmbedded = false }) => {
  // 1. 如果被 code-bench 宿主门户嵌套加载，无需自带 Header 外壳，直接进行路由映射
  if (isEmbedded) {
    return (
      <div className="gate-embedded-container" style={{ width: '100%', minHeight: '100%' }}>
        <Routes>
          <Route path="/" element={<Navigate to="/gate/chat" replace />} />
          <Route path="/gate" element={<Navigate to="/gate/chat" replace />} />
          <Route path="/gate/" element={<Navigate to="/gate/chat" replace />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/gate/chat" element={<ChatPage />} />
          <Route path="/keys" element={<KeysPage />} />
          <Route path="/gate/keys" element={<KeysPage />} />
          <Route path="/admin/dashboard" element={<DashboardPage />} />
          <Route path="/gate/admin/dashboard" element={<DashboardPage />} />
          <Route path="/admin/users" element={<AdminUsersPage />} />
          <Route path="/gate/admin/users" element={<AdminUsersPage />} />
          <Route path="/admin/backends" element={<AdminBackendsPage />} />
          <Route path="/gate/admin/backends" element={<AdminBackendsPage />} />
          <Route path="/logs" element={<LogsPage />} />
          <Route path="/gate/logs" element={<LogsPage />} />
          <Route path="*" element={<Navigate to="/gate/chat" replace />} />
        </Routes>
      </div>
    )
  }

  // 2. 独立访问运行时，带有自带的 BrowserRouter 与 Header 导航栏
  return (
    <BrowserRouter>
      <div className="gate-app-container">
        <Header />
        <main className="gate-main-content">
          <Routes>
            <Route path="/" element={<Navigate to="/chat" replace />} />
            <Route path="/chat" element={<ChatPage />} />
            <Route path="/keys" element={<KeysPage />} />
            <Route path="/admin/dashboard" element={<DashboardPage />} />
            <Route path="/admin/users" element={<AdminUsersPage />} />
            <Route path="/admin/backends" element={<AdminBackendsPage />} />
            <Route path="/logs" element={<LogsPage />} />
            <Route path="*" element={<Navigate to="/chat" replace />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}

export default App
