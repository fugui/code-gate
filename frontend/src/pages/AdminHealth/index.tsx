import React, { useEffect, useState, useRef } from 'react'
import {
  Activity,
  Server,
  Zap,
  CheckCircle,
  AlertTriangle,
  XCircle,
  RefreshCw,
  Clock,
  Gauge,
} from 'lucide-react'
import { fetchAdminHealth, triggerAdminProbe, toggleAdminBackend } from '../../api/client'
import { HealthMatrixData, BackendHealthItem } from '../../types'

export const AdminHealthPage: React.FC = () => {
  const [data, setData] = useState<HealthMatrixData | null>(null)
  const [loading, setLoading] = useState(true)
  const [probing, setProbing] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const timerRef = useRef<any>(null)

  const loadHealth = async (silent = false) => {
    try {
      if (!silent) setLoading(true)
      const res = await fetchAdminHealth()
      setData(res)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      if (!silent) setLoading(false)
    }
  }

  useEffect(() => {
    loadHealth()
  }, [])

  // 30 秒自动静默轮询
  useEffect(() => {
    if (autoRefresh) {
      timerRef.current = setInterval(() => {
        loadHealth(true)
      }, 30000)
    } else if (timerRef.current) {
      clearInterval(timerRef.current)
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [autoRefresh])

  // 手动触发全量探活
  const handleTriggerProbe = async () => {
    try {
      setProbing(true)
      await triggerAdminProbe()
      await loadHealth(true)
    } catch (err: unknown) {
      alert((err as Error).message || '探活失败')
    } finally {
      setProbing(false)
    }
  }

  // 切换节点启用状态
  const handleToggle = async (b: BackendHealthItem) => {
    try {
      await toggleAdminBackend(b.id)
      await loadHealth(true)
    } catch (err: unknown) {
      alert((err as Error).message || '切换状态失败')
    }
  }

  // 并发利用率色彩
  const getUtilizationColor = (ratio: number) => {
    if (ratio >= 0.9) return 'var(--color-danger)'
    if (ratio >= 0.7) return 'var(--color-warning)'
    return 'var(--color-success)'
  }

  const summary = data?.summary || {
    total_backends: 0,
    healthy_backends: 0,
    fault_backends: 0,
    avg_latency_ms: 0,
  }
  const backends = data?.backends || []

  return (
    <div className="gate-page-container">
      {/* 顶部标题与控制 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">物理后端实例全景健康与并发水位监控</h2>
          <p className="gate-page-subtitle">
            多维度实时监控物理节点在线状态、网络探测延迟、协议支持（chat / responses）以及原子 CAS 并发负荷水位。
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.875rem', cursor: 'pointer', color: 'var(--color-text-secondary)' }}>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              style={{ accentColor: 'var(--color-primary)' }}
            />
            <span>30秒静默轮询</span>
          </label>

          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => loadHealth(false)}
            disabled={loading}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <RefreshCw size={15} className={loading ? 'spin' : ''} />
            <span>刷新</span>
          </button>

          <button
            type="button"
            className="btn btn-primary"
            onClick={handleTriggerProbe}
            disabled={probing}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <Zap size={16} />
            <span>{probing ? '正在全量探活...' : '立即手动探测'}</span>
          </button>
        </div>
      </div>

      {/* 顶部总体指标卡 */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>
        <div className="gate-card" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <div style={{ background: 'var(--color-bg-muted)', padding: '0.75rem', borderRadius: '8px' }}>
            <Server size={24} color="var(--color-primary)" />
          </div>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>接入实例总数</div>
            <strong style={{ fontSize: '1.4rem', color: 'var(--color-text-primary)' }}>{summary.total_backends}</strong>
          </div>
        </div>

        <div className="gate-card" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <div style={{ background: 'var(--color-bg-muted)', padding: '0.75rem', borderRadius: '8px' }}>
            <CheckCircle size={24} color="var(--color-success)" />
          </div>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>在线健康节点</div>
            <strong style={{ fontSize: '1.4rem', color: 'var(--color-success)' }}>{summary.healthy_backends}</strong>
          </div>
        </div>

        <div className="gate-card" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <div style={{ background: 'var(--color-bg-muted)', padding: '0.75rem', borderRadius: '8px' }}>
            <AlertTriangle size={24} color="var(--color-danger)" />
          </div>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>异常或下线节点</div>
            <strong style={{ fontSize: '1.4rem', color: summary.fault_backends > 0 ? 'var(--color-danger)' : 'var(--color-text-muted)' }}>
              {summary.fault_backends}
            </strong>
          </div>
        </div>

        <div className="gate-card" style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
          <div style={{ background: 'var(--color-bg-muted)', padding: '0.75rem', borderRadius: '8px' }}>
            <Activity size={24} color="var(--color-info)" />
          </div>
          <div>
            <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>平均探测网络延迟</div>
            <strong style={{ fontSize: '1.4rem', color: 'var(--color-info)' }}>{summary.avg_latency_ms} ms</strong>
          </div>
        </div>
      </div>

      {/* 实例详细矩阵表格 */}
      <div className="gate-card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ padding: '1rem 1.25rem', borderBottom: '1px solid var(--color-border-primary)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Gauge size={18} color="var(--color-primary)" />
          <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 600 }}>物理后端实例实时并发水位与健康明细</h3>
        </div>

        {loading && !data ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>加载健康数据中...</div>
        ) : backends.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>暂无物理后端节点</div>
        ) : (
          <div className="gate-table-container">
            <table className="gate-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: 'var(--color-bg-muted)', textAlign: 'left', borderBottom: '1px solid var(--color-border-primary)' }}>
                  <th style={{ padding: '0.75rem 1rem' }}>状态</th>
                  <th style={{ padding: '0.75rem 1rem' }}>实例名称 / 所属模型</th>
                  <th style={{ padding: '0.75rem 1rem' }}>BaseURL 节点地址</th>
                  <th style={{ padding: '0.75rem 1rem' }}>协议能力</th>
                  <th style={{ padding: '0.75rem 1rem', width: '220px' }}>并发负荷水位 (CAS Slot)</th>
                  <th style={{ padding: '0.75rem 1rem' }}>网络延迟</th>
                  <th style={{ padding: '0.75rem 1rem' }}>连续失败</th>
                  <th style={{ padding: '0.75rem 1rem' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {backends.map((b) => {
                  const ratio = b.utilization_ratio || 0
                  const pct = Math.min(Math.round(ratio * 100), 100)
                  const utilColor = getUtilizationColor(ratio)

                  return (
                    <tr
                      key={b.id}
                      style={{
                        borderBottom: '1px solid var(--color-border-primary)',
                        opacity: b.is_enabled ? 1 : 0.6,
                      }}
                    >
                      {/* 健康状态灯 */}
                      <td style={{ padding: '0.75rem 1rem', verticalAlign: 'middle' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                          {b.is_healthy && b.is_enabled ? (
                            <CheckCircle size={18} color="var(--color-success)" />
                          ) : (
                            <XCircle size={18} color="var(--color-danger)" />
                          )}
                          <span
                            style={{
                              fontSize: '0.75rem',
                              fontWeight: 600,
                              color: b.is_healthy && b.is_enabled ? 'var(--color-success)' : 'var(--color-danger)',
                            }}
                          >
                            {!b.is_enabled ? '已禁用' : b.is_healthy ? '正常' : '异常熔断'}
                          </span>
                        </div>
                      </td>

                      {/* 实例名称 */}
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <strong style={{ display: 'block', fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>{b.name}</strong>
                        <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>模型: {b.model_name}</span>
                      </td>

                      {/* BaseURL */}
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <code style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)' }}>{b.base_url}</code>
                      </td>

                      {/* 协议徽标 */}
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <div style={{ display: 'flex', gap: '0.3rem' }}>
                          <span
                            style={{
                              padding: '1px 5px',
                              borderRadius: '3px',
                              fontSize: '0.75rem',
                              fontWeight: 600,
                              background: 'var(--color-success-subtle, rgba(16, 185, 129, 0.1))',
                              color: 'var(--color-success)',
                            }}
                          >
                            chat
                          </span>
                          {b.declared_protocols?.includes('responses') || b.detected_protocols?.includes('responses') ? (
                            <span
                              style={{
                                padding: '1px 5px',
                                borderRadius: '3px',
                                fontSize: '0.75rem',
                                fontWeight: 600,
                                background: 'var(--color-primary-subtle, rgba(59, 130, 246, 0.1))',
                                color: 'var(--color-primary)',
                              }}
                            >
                              responses
                            </span>
                          ) : null}
                        </div>
                      </td>

                      {/* 细粒度并发实时水位监控 */}
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.3rem', fontSize: '0.75rem' }}>
                          <span style={{ color: 'var(--color-text-secondary)' }}>
                            {b.active_connections} / {b.max_concurrency} 连接
                          </span>
                          <strong style={{ color: utilColor }}>{pct}%</strong>
                        </div>
                        <div
                          style={{
                            height: '6px',
                            background: 'var(--color-bg-input)',
                            borderRadius: '3px',
                            overflow: 'hidden',
                          }}
                        >
                          <div
                            style={{
                              width: `${pct}%`,
                              height: '100%',
                              background: utilColor,
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </div>
                      </td>

                      {/* 网络延迟 */}
                      <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>
                        {b.latency_ms > 0 ? (
                          <span style={{ display: 'flex', alignItems: 'center', gap: '0.2rem', color: 'var(--color-text-primary)' }}>
                            <Zap size={12} color="var(--color-warning)" />
                            {b.latency_ms} ms
                          </span>
                        ) : (
                          <span style={{ color: 'var(--color-text-muted)' }}>-</span>
                        )}
                      </td>

                      {/* 连续失败次数 */}
                      <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>
                        <span
                          style={{
                            color: b.consecutive_failures > 0 ? 'var(--color-danger)' : 'var(--color-text-muted)',
                            fontWeight: b.consecutive_failures > 0 ? 600 : 400,
                          }}
                        >
                          {b.consecutive_failures} 次
                        </span>
                      </td>

                      {/* 操作 */}
                      <td style={{ padding: '0.75rem 1rem' }}>
                        <button
                          type="button"
                          className={b.is_enabled ? 'btn btn-secondary' : 'btn btn-primary'}
                          onClick={() => handleToggle(b)}
                          style={{ padding: '0.25rem 0.6rem', fontSize: '0.75rem' }}
                        >
                          {b.is_enabled ? '下线' : '上线'}
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
