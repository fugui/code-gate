import React, { useEffect, useState } from 'react'
import { Plus, Trash2, Copy, Check, Key, ShieldAlert } from 'lucide-react'
import { fetchUserProfile, fetchUserKeys, createAPIKey, deleteAPIKey } from '../../api/client'
import { UserProfile, APIKeyItem } from '../../types'

export const KeysPage: React.FC = () => {
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [keys, setKeys] = useState<APIKeyItem[]>([])
  const [loading, setLoading] = useState(true)

  // 新建 API Key 弹窗
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [keyName, setKeyName] = useState('')
  const [expiresIn, setExpiresIn] = useState(30)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const loadData = async () => {
    try {
      setLoading(true)
      const [pData, kData] = await Promise.all([fetchUserProfile(), fetchUserKeys()])
      setProfile(pData)
      setKeys(kData)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleCreateKey = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!keyName.trim()) return

    try {
      setSubmitting(true)
      const res = await createAPIKey(keyName.trim(), expiresIn)
      setCreatedSecret(res.raw_key)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: number) => {
    if (!window.confirm('确定要删除该 API Key 吗？删除后正在使用该密钥的程序将立即失效。')) {
      return
    }
    try {
      await deleteAPIKey(id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '删除失败')
    }
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="gate-page-container">
      <div>
        <h2 style={{ fontSize: '1.4rem', fontWeight: 700, margin: '0 0 0.5rem 0' }}>算力配额与 API Key 凭证管理</h2>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.9rem' }}>
          CodeGate 基于 Credits 统一算力体系，每日/每周配额自动联动重置。可自主创建个人专用的调用密钥。
        </p>
      </div>

      {/* 配额台账概览网格 */}
      {profile && (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '1rem' }}>
          <div className="gate-card">
            <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginBottom: '0.4rem' }}>当前配额角色</div>
            <div style={{ fontSize: '1.25rem', fontWeight: 700, color: 'var(--color-primary)' }}>
              {profile.role.toUpperCase()}
              {profile.is_custom && <span style={{ fontSize: '0.75rem', marginLeft: '0.5rem', padding: '2px 6px', background: 'var(--color-warning-subtle)', color: 'var(--color-warning)', borderRadius: '4px' }}>定制配额</span>}
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)', marginTop: '0.4rem' }}>
              策略: {profile.policy_name} (频控上限: {profile.rpm_limit} RPM)
            </div>
          </div>

          <div className="gate-card">
            <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginBottom: '0.4rem' }}>自然日算力 (Credits)</div>
            <div style={{ fontSize: '1.25rem', fontWeight: 700 }}>
              {profile.daily_used_credits.toFixed(1)} <span style={{ fontSize: '0.9rem', color: 'var(--color-text-muted)' }}>/ {profile.daily_limit_credits.toFixed(1)}</span>
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-success)', marginTop: '0.4rem' }}>
              剩余可用: {profile.daily_remaining_credits.toFixed(1)} Credits
            </div>
          </div>

          <div className="gate-card">
            <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', marginBottom: '0.4rem' }}>自然周算力 (Credits)</div>
            <div style={{ fontSize: '1.25rem', fontWeight: 700 }}>
              {profile.weekly_used_credits.toFixed(1)} <span style={{ fontSize: '0.9rem', color: 'var(--color-text-muted)' }}>/ {profile.weekly_limit_credits.toFixed(1)}</span>
            </div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-success)', marginTop: '0.4rem' }}>
              剩余可用: {profile.weekly_remaining_credits.toFixed(1)} Credits (4倍日配额)
            </div>
          </div>
        </div>
      )}

      {/* API Key 列表 */}
      <div className="gate-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.25rem' }}>
          <div>
            <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 600 }}>我的 API Keys</h3>
            <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>可在第三方客户端或程序中通过 Authorization: Bearer sk-... 调用</span>
          </div>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => {
              setKeyName('')
              setCreatedSecret(null)
              setIsCreateOpen(true)
            }}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 1rem' }}
          >
            <Plus size={16} />
            <span>新建 API Key</span>
          </button>
        </div>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>加载中...</div>
        ) : keys.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem 1rem', color: 'var(--color-text-muted)' }}>
            暂未生成任何 API Key，点击右上角快速新建
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="gate-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>Key 前缀</th>
                  <th>状态</th>
                  <th>创建时间</th>
                  <th>过期时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <tr key={k.id}>
                    <td style={{ fontWeight: 600 }}>{k.name}</td>
                    <td>
                      <code style={{ background: 'var(--color-bg-muted)', padding: '2px 6px', borderRadius: '4px' }}>
                        {k.key_prefix}
                      </code>
                    </td>
                    <td>
                      <span
                        style={{
                          padding: '2px 8px',
                          borderRadius: '4px',
                          fontSize: '0.75rem',
                          background: k.is_active ? 'var(--color-success-subtle)' : 'var(--color-danger-subtle)',
                          color: k.is_active ? 'var(--color-success)' : 'var(--color-danger)',
                        }}
                      >
                        {k.is_active ? '活跃生效中' : '已停用'}
                      </span>
                    </td>
                    <td style={{ color: 'var(--color-text-secondary)' }}>{new Date(k.created_at).toLocaleString()}</td>
                    <td style={{ color: 'var(--color-text-secondary)' }}>
                      {k.expires_at ? new Date(k.expires_at).toLocaleString() : '永不过期'}
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-danger"
                        onClick={() => handleDelete(k.id)}
                        style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem', display: 'inline-flex', alignItems: 'center', gap: '0.2rem' }}
                      >
                        <Trash2 size={13} />
                        <span>删除</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* 创建 Key 弹窗 Modal */}
      {isCreateOpen && (
        <div
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.5)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
        >
          <div
            className="gate-card"
            style={{
              width: '100%',
              maxWidth: 480,
              backgroundColor: 'var(--color-bg-surface)',
              border: '1px solid var(--color-border-primary)',
              boxShadow: 'var(--shadow-lg)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '1rem' }}>
              <Key size={20} color="var(--color-primary)" />
              <h3 style={{ margin: 0, fontSize: '1.15rem' }}>
                {createdSecret ? 'API Key 创建成功' : '新建 API Key'}
              </h3>
            </div>

            {createdSecret ? (
              <div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.5rem',
                    padding: '0.75rem',
                    backgroundColor: 'var(--color-warning-subtle)',
                    border: '1px solid var(--color-warning-border)',
                    borderRadius: '6px',
                    color: 'var(--color-warning)',
                    fontSize: '0.85rem',
                    marginBottom: '1rem',
                  }}
                >
                  <ShieldAlert size={20} />
                  <span>请妥善复制并保管该 API Key！出于安全机制，密钥明文仅在此展示一次，后续无法再次查阅。</span>
                </div>

                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.5rem',
                    padding: '0.75rem',
                    backgroundColor: 'var(--color-bg-muted)',
                    border: '1px solid var(--color-border-primary)',
                    borderRadius: '6px',
                    marginBottom: '1.25rem',
                  }}
                >
                  <code style={{ wordBreak: 'break-all', flex: 1, fontSize: '0.85rem' }}>{createdSecret}</code>
                  <button
                    type="button"
                    className="btn btn-primary"
                    onClick={() => handleCopy(createdSecret)}
                    style={{ padding: '0.4rem 0.75rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
                  >
                    {copied ? <Check size={14} /> : <Copy size={14} />}
                    <span>{copied ? '已复制' : '复制'}</span>
                  </button>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    onClick={() => setIsCreateOpen(false)}
                    style={{ padding: '0.4rem 1rem' }}
                  >
                    我已复制，关闭窗口
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleCreateKey}>
                <div style={{ marginBottom: '1rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                    密钥备注名称
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="如: Claude Code 本地测试"
                    value={keyName}
                    onChange={(e) => setKeyName(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '0.5rem 0.75rem',
                      borderRadius: '6px',
                      border: '1px solid var(--color-border-primary)',
                      background: 'var(--color-bg-surface)',
                      color: 'var(--color-text-primary)',
                    }}
                  />
                </div>

                <div style={{ marginBottom: '1.25rem' }}>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                    有效期
                  </label>
                  <select
                    value={expiresIn}
                    onChange={(e) => setExpiresIn(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '0.5rem 0.75rem',
                      borderRadius: '6px',
                      border: '1px solid var(--color-border-primary)',
                      background: 'var(--color-bg-surface)',
                      color: 'var(--color-text-primary)',
                    }}
                  >
                    <option value={7}>7 天</option>
                    <option value={30}>30 天</option>
                    <option value={90}>90 天</option>
                    <option value={0}>永不过期</option>
                  </select>
                </div>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
                  <button
                    type="button"
                    className="btn btn-secondary"
                    onClick={() => setIsCreateOpen(false)}
                    style={{ padding: '0.4rem 0.9rem' }}
                  >
                    取消
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                    disabled={submitting || !keyName.trim()}
                    style={{ padding: '0.4rem 1rem' }}
                  >
                    {submitting ? '创建中...' : '确认创建'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
