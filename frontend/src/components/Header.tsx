import React, { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Shield, MessageSquare, Key, Users, Server, FileText, Sun, Moon } from 'lucide-react'
import { useTheme } from '@code/common'
import { fetchUserProfile } from '../api/client'
import { UserProfile } from '../types'

export const Header: React.FC = () => {
  const location = useLocation()
  const { theme, toggleTheme } = useTheme()
  const [profile, setProfile] = useState<UserProfile | null>(null)

  useEffect(() => {
    fetchUserProfile()
      .then(setProfile)
      .catch(() => {
        // 忽略未登录或未连接状态
      })
  }, [location.pathname])

  const calcPercent = (used: number, limit: number) => {
    if (limit <= 0) return 0
    const p = Math.round((used / limit) * 100)
    return Math.min(p, 100)
  }

  const getBarColorClass = (percent: number) => {
    if (percent >= 90) return 'gate-progress-bar-fill--red'
    if (percent >= 70) return 'gate-progress-bar-fill--yellow'
    return 'gate-progress-bar-fill--green'
  }

  const dailyPercent = profile ? calcPercent(profile.daily_used_credits, profile.daily_limit_credits) : 0
  const weeklyPercent = profile ? calcPercent(profile.weekly_used_credits, profile.weekly_limit_credits) : 0

  return (
    <header className="gate-navbar">
      <div style={{ display: 'flex', alignItems: 'center', gap: '2rem' }}>
        <Link to="/chat" className="gate-logo-area">
          <div
            style={{
              width: 32,
              height: 32,
              borderRadius: '8px',
              backgroundColor: 'var(--color-primary-subtle)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--color-primary)',
            }}
          >
            <Shield size={20} />
          </div>
          <span>CodeGate <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)', fontWeight: 400 }}>码界</span></span>
        </Link>

        <nav className="gate-nav-links">
          <Link to="/chat" className={`gate-nav-link ${location.pathname === '/chat' || location.pathname === '/' ? 'active' : ''}`}>
            <MessageSquare size={16} />
            <span>极速对话</span>
          </Link>
          <Link to="/keys" className={`gate-nav-link ${location.pathname === '/keys' ? 'active' : ''}`}>
            <Key size={16} />
            <span>算力与密钥</span>
          </Link>
          <Link to="/admin/users" className={`gate-nav-link ${location.pathname === '/admin/users' ? 'active' : ''}`}>
            <Users size={16} />
            <span>用户配额</span>
          </Link>
          <Link to="/admin/backends" className={`gate-nav-link ${location.pathname === '/admin/backends' ? 'active' : ''}`}>
            <Server size={16} />
            <span>模型与后端</span>
          </Link>
          <Link to="/logs" className={`gate-nav-link ${location.pathname === '/logs' ? 'active' : ''}`}>
            <FileText size={16} />
            <span>审计日志</span>
          </Link>
        </nav>
      </div>

      <div className="gate-header-actions">
        {profile && (
          <div className="gate-quota-capsule" title={`日配额: ${profile.daily_used_credits.toFixed(1)} / ${profile.daily_limit_credits.toFixed(1)} Credits\n周配额: ${profile.weekly_used_credits.toFixed(1)} / ${profile.weekly_limit_credits.toFixed(1)} Credits`}>
            <span style={{ color: 'var(--color-text-muted)' }}>本日</span>
            <div className="gate-progress-bar-wrap">
              <div
                className={`gate-progress-bar-fill ${getBarColorClass(dailyPercent)}`}
                style={{ width: `${dailyPercent}%` }}
              />
            </div>
            <span style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>{dailyPercent}%</span>

            <span style={{ color: 'var(--color-border-primary)', margin: '0 2px' }}>|</span>

            <span style={{ color: 'var(--color-text-muted)' }}>本周</span>
            <div className="gate-progress-bar-wrap">
              <div
                className={`gate-progress-bar-fill ${getBarColorClass(weeklyPercent)}`}
                style={{ width: `${weeklyPercent}%` }}
              />
            </div>
            <span style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>{weeklyPercent}%</span>
          </div>
        )}

        <button
          type="button"
          onClick={toggleTheme}
          className="btn btn-secondary"
          style={{ padding: '0.4rem 0.6rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          title={`切换为${theme === 'dark' ? '亮色' : '暗色'}主题`}
        >
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
        </button>
      </div>
    </header>
  )
}
