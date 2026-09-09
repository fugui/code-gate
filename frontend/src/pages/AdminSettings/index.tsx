import React, { useEffect, useState } from 'react'
import { ShieldAlert, Plus, Trash2, Save, Info, Sliders, CheckCircle2 } from 'lucide-react'
import { fetchAdminSystemConfig, updateAdminSystemConfig } from '../../api/client'
import { SystemConfigData } from '../../types'

export const AdminSettingsPage: React.FC = () => {
  const [config, setConfig] = useState<SystemConfigData | null>(null)
  const [blockedUAs, setBlockedUAs] = useState<string[]>([])
  const [newUA, setNewUA] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savedSuccess, setSavedSuccess] = useState(false)

  const loadConfig = async () => {
    try {
      setLoading(true)
      const data = await fetchAdminSystemConfig()
      setConfig(data)
      setBlockedUAs(data.blocked_user_agents || [])
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadConfig()
  }, [])

  // 添加规则
  const handleAddUA = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = newUA.trim().toLowerCase()
    if (!trimmed) return
    if (blockedUAs.includes(trimmed)) {
      alert('该 User-Agent 关键词已在规则列表中')
      return
    }
    setBlockedUAs([...blockedUAs, trimmed])
    setNewUA('')
  }

  // 删除规则
  const handleRemoveUA = (idx: number) => {
    setBlockedUAs(blockedUAs.filter((_, i) => i !== idx))
  }

  // 保存并热重载
  const handleSave = async () => {
    try {
      setSaving(true)
      await updateAdminSystemConfig({ blocked_user_agents: blockedUAs })
      setSavedSuccess(true)
      setTimeout(() => setSavedSuccess(false), 3000)
    } catch (err: unknown) {
      alert((err as Error).message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="gate-page-container">
      {/* 顶部标题与说明 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">系统配置与动态客户端安全过滤 (System & Security Control)</h2>
          <p className="gate-page-subtitle">
            在线维护客户端 User-Agent 黑名单过滤规则（0 停机秒级热重载），并监控系统核心传输超时基准。
          </p>
        </div>

        <button
          type="button"
          className="btn btn-primary"
          onClick={handleSave}
          disabled={saving}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
        >
          {savedSuccess ? <CheckCircle2 size={16} color="#fff" /> : <Save size={16} />}
          <span>{saving ? '正在生效中...' : savedSuccess ? '已热重载生效！' : '保存并热生效'}</span>
        </button>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 340px', gap: '1.5rem', alignItems: 'start' }}>
        {/* 左侧：动态 User-Agent 黑名单在线规则表 */}
        <div className="gate-card" style={{ padding: '1.25rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '1rem' }}>
            <ShieldAlert size={20} color="var(--color-danger)" />
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 600 }}>客户端 User-Agent 安全阻断表</h3>
          </div>

          <p style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)', marginBottom: '1.25rem' }}>
            若客户端请求的 User-Agent 包含以下任一关键词（大小写不敏感），网关将立即返回 <code>403 Forbidden (client_blocked)</code>，有效阻断恶意自动化扫描器与非法爬虫。
          </p>

          {/* 新增输入框 */}
          <form onSubmit={handleAddUA} style={{ display: 'flex', gap: '0.5rem', marginBottom: '1.25rem' }}>
            <input
              type="text"
              className="gate-input"
              value={newUA}
              onChange={(e) => setNewUA(e.target.value)}
              placeholder="输入待阻断的 UA 关键词 (例如: sqlmap, bad-bot, spider)"
              style={{ flex: 1 }}
            />
            <button type="submit" className="btn btn-secondary" style={{ display: 'flex', alignItems: 'center', gap: '0.3rem' }}>
              <Plus size={15} />
              <span>添加规则</span>
            </button>
          </form>

          {/* 规则表 */}
          {loading ? (
            <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>加载规则中...</div>
          ) : blockedUAs.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>暂无激活的拦截规则</div>
          ) : (
            <div style={{ border: '1px solid var(--color-border-primary)', borderRadius: '6px', overflow: 'hidden' }}>
              <table className="gate-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ background: 'var(--color-bg-muted)', textAlign: 'left', borderBottom: '1px solid var(--color-border-primary)' }}>
                    <th style={{ padding: '0.6rem 1rem', width: '60px' }}>序号</th>
                    <th style={{ padding: '0.6rem 1rem' }}>阻断 User-Agent 包含关键词</th>
                    <th style={{ padding: '0.6rem 1rem', width: '90px' }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {blockedUAs.map((ua, idx) => (
                    <tr key={idx} style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
                      <td style={{ padding: '0.6rem 1rem', color: 'var(--color-text-muted)', fontSize: '0.85rem' }}>
                        #{idx + 1}
                      </td>
                      <td style={{ padding: '0.6rem 1rem' }}>
                        <code style={{ fontSize: '0.9rem', color: 'var(--color-danger)' }}>{ua}</code>
                      </td>
                      <td style={{ padding: '0.6rem 1rem' }}>
                        <button
                          type="button"
                          className="btn btn-danger"
                          onClick={() => handleRemoveUA(idx)}
                          style={{ padding: '0.2rem 0.45rem', fontSize: '0.75rem' }}
                          title="删除该规则"
                        >
                          <Trash2 size={13} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* 右侧：核心传输超时基准面板 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <div className="gate-card" style={{ padding: '1.25rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '1rem' }}>
              <Sliders size={18} color="var(--color-primary)" />
              <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 600 }}>核心传输超时基准</h3>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px dashed var(--color-border-primary)', paddingBottom: '0.5rem' }}>
                <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>读请求超时 (read_timeout)</span>
                <strong style={{ fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>{config?.read_timeout || '30m'}</strong>
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px dashed var(--color-border-primary)', paddingBottom: '0.5rem' }}>
                <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>长流式写响应超时 (write_timeout)</span>
                <strong style={{ fontSize: '0.9rem', color: 'var(--color-primary)' }}>{config?.write_timeout || '30m'}</strong>
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px dashed var(--color-border-primary)', paddingBottom: '0.5rem' }}>
                <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>空闲保活连接 (idle_timeout)</span>
                <strong style={{ fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>{config?.idle_timeout || '120s'}</strong>
              </div>

              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingBottom: '0.2rem' }}>
                <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>请求头限制 (max_header_bytes)</span>
                <strong style={{ fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>
                  {config?.max_header_bytes ? `${Math.round(config.max_header_bytes / 1024)} KB` : '1 MB'}
                </strong>
              </div>
            </div>
          </div>

          <div
            style={{
              padding: '0.85rem',
              borderRadius: '6px',
              background: 'var(--color-bg-muted)',
              border: '1px solid var(--color-border-primary)',
              display: 'flex',
              gap: '0.5rem',
              alignItems: 'flex-start',
            }}
          >
            <Info size={16} color="var(--color-info)" style={{ flexShrink: 0, marginTop: '2px' }} />
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>
              User-Agent 黑名单拦截规则支持<strong>秒级内存热重载</strong>；传输超时配置涉及底层网络 Socket，若在 <code>config.yaml</code> 调整需重启服务方可生效。
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
