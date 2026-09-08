import React, { useEffect, useState } from 'react'
import { Server, Cpu, Plus, Trash2, Activity, CheckCircle, XCircle } from 'lucide-react'
import { fetchAdminBackends, fetchModels, saveAdminBackend, deleteAdminBackend } from '../../api/client'
import { BackendItem, ModelItem } from '../../types'

export const AdminBackendsPage: React.FC = () => {
  const [backends, setBackends] = useState<BackendItem[]>([])
  const [models, setModels] = useState<ModelItem[]>([])
  const [loading, setLoading] = useState(true)

  // 新增物理后端弹窗
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [weight, setWeight] = useState(10)
  const [maxConnections, setMaxConnections] = useState(50)
  const [submitting, setSubmitting] = useState(false)

  const loadData = async () => {
    try {
      setLoading(true)
      const [bList, mList] = await Promise.all([fetchAdminBackends(), fetchModels()])
      setBackends(bList)
      setModels(mList)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleAddBackend = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !baseUrl.trim()) return

    try {
      setSubmitting(true)
      await saveAdminBackend({
        name: name.trim(),
        base_url: baseUrl.trim(),
        weight,
        max_connections: maxConnections,
      })
      setIsAddOpen(false)
      setName('')
      setBaseUrl('')
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '添加后端失败')
    } finally {
      setSubmitting(false)
    }
  }

  const handleDeleteBackend = async (id: number) => {
    if (!window.confirm('确定要删除该物理后端节点吗？')) return
    try {
      await deleteAdminBackend(id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '删除失败')
    }
  }

  return (
    <div className="gate-page-container">
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <h2 style={{ fontSize: '1.4rem', fontWeight: 700, margin: '0 0 0.5rem 0' }}>模型映射与物理后端探活看板</h2>
          <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.9rem' }}>
            CodeGate 实行物理实例协议能力自动探活与差异分发（chat / responses），并根据模型倍率实施精准算力计费。
          </p>
        </div>

        <button
          type="button"
          className="btn btn-primary"
          onClick={() => setIsAddOpen(true)}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 1rem' }}
        >
          <Plus size={16} />
          <span>接入新后端</span>
        </button>
      </div>

      {/* 物理实例健康探活网格 */}
      <div>
        <h3 style={{ fontSize: '1.1rem', fontWeight: 600, marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Server size={18} color="var(--color-primary)" />
          <span>物理 Backend 实例群与协议能力</span>
        </h3>

        {loading ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>加载后端实例中...</div>
        ) : backends.length === 0 ? (
          <div className="gate-card" style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
            暂未接入物理后端实例
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))', gap: '1rem' }}>
            {backends.map((b) => (
              <div key={b.id} className="gate-card" style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    {b.is_healthy ? (
                      <CheckCircle size={18} color="var(--color-success)" />
                    ) : (
                      <XCircle size={18} color="var(--color-danger)" />
                    )}
                    <strong style={{ fontSize: '1.05rem' }}>{b.name}</strong>
                  </div>

                  <button
                    type="button"
                    className="btn btn-danger"
                    onClick={() => handleDeleteBackend(b.id)}
                    style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                  >
                    <Trash2 size={12} />
                  </button>
                </div>

                <div style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                  <code>{b.base_url}</code>
                </div>

                {/* 协议能力徽章 */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                  <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>探活支持协议:</span>
                  <span
                    style={{
                      padding: '2px 6px',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      background: b.supports_chat ? 'var(--color-success-subtle)' : 'var(--color-bg-muted)',
                      color: b.supports_chat ? 'var(--color-success)' : 'var(--color-text-muted)',
                      border: '1px solid var(--color-border-subtle)',
                    }}
                  >
                    chat
                  </span>
                  <span
                    style={{
                      padding: '2px 6px',
                      borderRadius: '4px',
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      background: b.supports_responses ? 'var(--color-primary-subtle)' : 'var(--color-bg-muted)',
                      color: b.supports_responses ? 'var(--color-primary)' : 'var(--color-text-muted)',
                      border: '1px solid var(--color-border-subtle)',
                    }}
                  >
                    responses
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '0.8rem', color: 'var(--color-text-secondary)', borderTop: '1px dashed var(--color-border-subtle)', paddingTop: '0.5rem' }}>
                  <span>延迟: <strong>{b.latency_ms}ms</strong></span>
                  <span>活跃连接: <strong>{b.active_connections}</strong> / {b.max_connections}</span>
                  <span>权重: <strong>{b.weight}</strong></span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 模型与费率矩阵 */}
      <div className="gate-card" style={{ marginTop: '1rem' }}>
        <h3 style={{ fontSize: '1.1rem', fontWeight: 600, marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Cpu size={18} color="var(--color-primary)" />
          <span>可用大模型与 Credits 算力费率表</span>
        </h3>

        <div style={{ overflowX: 'auto' }}>
          <table className="gate-table">
            <thead>
              <tr>
                <th>模型 ID</th>
                <th>所有者</th>
                <th>算力费率倍率</th>
                <th>输入费率 (Credits/1k)</th>
                <th>缓存命中折扣 (Credits/1k)</th>
                <th>输出费率 (Credits/1k)</th>
              </tr>
            </thead>
            <tbody>
              {models.map((m) => (
                <tr key={m.id}>
                  <td><strong>{m.id}</strong></td>
                  <td style={{ color: 'var(--color-text-secondary)' }}>{m.owned_by}</td>
                  <td>
                    <span
                      style={{
                        padding: '2px 8px',
                        borderRadius: '4px',
                        fontSize: '0.8rem',
                        fontWeight: 600,
                        background: 'var(--color-primary-subtle)',
                        color: 'var(--color-primary)',
                      }}
                    >
                      {m.cost_multiplier}x
                    </span>
                  </td>
                  <td>{m.input_rate}</td>
                  <td>
                    <span style={{ color: 'var(--color-success)', fontWeight: 600 }}>{m.cache_hit_rate}</span>
                    <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}> (省 90%)</span>
                  </td>
                  <td>{m.output_rate}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* 新增后端弹窗 */}
      {isAddOpen && (
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
              <Server size={20} color="var(--color-primary)" />
              <h3 style={{ margin: 0, fontSize: '1.15rem' }}>接入新物理后端实例</h3>
            </div>

            <form onSubmit={handleAddBackend}>
              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                  实例名称
                </label>
                <input
                  type="text"
                  required
                  placeholder="如: vllm-cluster-node-01"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
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

              <div style={{ marginBottom: '1rem' }}>
                <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                  后端 Base URL
                </label>
                <input
                  type="url"
                  required
                  placeholder="如: http://192.168.1.100:8000"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
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

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1.25rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                    调度权重 (Weight)
                  </label>
                  <input
                    type="number"
                    min={1}
                    value={weight}
                    onChange={(e) => setWeight(Number(e.target.value))}
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

                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                    最大连接数
                  </label>
                  <input
                    type="number"
                    min={1}
                    value={maxConnections}
                    onChange={(e) => setMaxConnections(Number(e.target.value))}
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
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setIsAddOpen(false)}
                  style={{ padding: '0.4rem 0.9rem' }}
                >
                  取消
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={submitting || !name.trim() || !baseUrl.trim()}
                  style={{ padding: '0.4rem 1rem' }}
                >
                  {submitting ? '接入中...' : '确认接入并启动探活'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
