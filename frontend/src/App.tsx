import React from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Header } from './components/Header'
import { ChatPage } from './pages/Chat'
import { KeysPage } from './pages/Keys'
import { AdminUsersPage } from './pages/AdminUsers'
import { AdminBackendsPage } from './pages/AdminBackends'
import { LogsPage } from './pages/Logs'

export const App: React.FC = () => {
  return (
    <BrowserRouter>
      <div className="gate-app-container">
        <Header />
        <main className="gate-main-content">
          <Routes>
            <Route path="/" element={<Navigate to="/chat" replace />} />
            <Route path="/chat" element={<ChatPage />} />
            <Route path="/keys" element={<KeysPage />} />
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
