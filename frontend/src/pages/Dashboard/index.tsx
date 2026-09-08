import React, { useEffect, useState, useMemo, useCallback } from 'react'
import { fetchAdminDashboard } from '../../api/client'
import { DashboardData } from '../../types'
import './dashboard.css'

export const DashboardPage: React.FC = () => {
  const [data, setData] = useState<DashboardData | null>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true)
  const [hoveredIdx, setHoveredIdx] = useState<number | null>(null)
  const [lastRefreshedAt, setLastRefreshedAt] = useState<string>('')

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const res = await fetchAdminDashboard()
      setData(res)
      const now = new Date()
      setLastRefreshedAt(
        `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
      )
    } catch (err) {
      console.error('获取监控大屏数据失败:', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  useEffect(() => {
    if (!autoRefresh) return
    const timer = setInterval(() => {
      loadData()
    }, 30000)
    return () => clearInterval(timer)
  }, [autoRefresh, loadData])

  // 计算请求成功率
  const successRate = useMemo(() => {
    if (!data?.status_counts) return 100
    const total = data.status_counts.status_2xx + data.status_counts.status_4xx + data.status_counts.status_5xx
    if (total === 0) return 100
    return Math.round((data.status_counts.status_2xx / total) * 1000) / 10
  }, [data])

  // SVG 趋势图绘制坐标生成
  const trendSvg = useMemo(() => {
    const trends = data?.hourly_trends || []
    if (trends.length === 0) return null

    const width = 800
    const height = 240
    const padL = 40
    const padR = 20
    const padT = 20
    const padB = 40
    const chartW = width - padL - padR
    const chartH = height - padT - padB

    const maxReq = Math.max(...trends.map((t) => t.requests), 10)
    const maxCred = Math.max(...trends.map((t) => t.cost_credits), 10)

    const pointsReq = trends.map((t, i) => {
      const x = padL + (i / (trends.length - 1)) * chartW
      const y = height - padB - (t.requests / maxReq) * chartH
      return { x, y, raw: t }
    })

    const pointsCred = trends.map((t, i) => {
      const x = padL + (i / (trends.length - 1)) * chartW
      const y = height - padB - (t.cost_credits / maxCred) * chartH
      return { x, y, raw: t }
    })

    const pathDReq = pointsReq.reduce((acc, p, i) => {
      return i === 0 ? `M ${p.x} ${p.y}` : `${acc} L ${p.x} ${p.y}`
    }, '')

    const areaDReq = `${pathDReq} L ${pointsReq[pointsReq.length - 1].x} ${height - padB} L ${pointsReq[0].x} ${height - padB} Z`

    const pathDCred = pointsCred.reduce((acc, p, i) => {
      return i === 0 ? `M ${p.x} ${p.y}` : `${acc} L ${p.x} ${p.y}`
    }, '')

    return {
      width,
      height,
      padL,
      padR,
      padT,
      padB,
      chartW,
      chartH,
      pointsReq,
      pointsCred,
      pathDReq,
      areaDReq,
      pathDCred,
      trends,
    }
  }, [data])

  return (
    <div className="gate-page-container">
      <div className="gate-dashboard-container">
        {/* 顶部标题与控制栏 */}
        <div className="gate-dashboard-topbar">
          <div className="gate-dashboard-title-group">
            <h1>网关监控大屏</h1>
            <div className="gate-dashboard-subtitle">
              实时监控全链路调用流量、算力消耗与集群节点健康态
              {lastRefreshedAt && ` · 最近刷新于 ${lastRefreshedAt}`}
            </div>
          </div>
          <div className="gate-dashboard-actions">
            <div className="gate-dashboard-badge">
              <span className="gate-dashboard-pulse-dot" />
              <span>实时探活中</span>
            </div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.85rem', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
              />
              30秒自动刷新
            </label>
            <button
              className="btn btn-secondary"
              onClick={() => loadData()}
              disabled={loading}
              style={{ padding: '0.4rem 0.8rem', fontSize: '0.85rem' }}
            >
              {loading ? '刷新中...' : '立即刷新'}
            </button>
          </div>
        </div>

        {/* 关键 KPI 指标卡 */}
        <div className="gate-kpi-grid">
          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">今日调用量</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value">{data?.summary.today_requests.toLocaleString() ?? '--'}</div>
            <div className="gate-kpi-sub">
              <span>总累计: {data?.summary.total_requests.toLocaleString() ?? '--'} 次</span>
            </div>
          </div>

          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">今日 Credits 消耗</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="10" />
                  <path d="M12 6v6l4 2" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value" style={{ color: 'var(--color-primary)' }}>
              {data?.summary.today_credits.toFixed(2) ?? '--'}
            </div>
            <div className="gate-kpi-sub">
              <span>总累计: {data?.summary.total_credits.toFixed(2) ?? '--'} Credits</span>
            </div>
          </div>

          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">24h 活跃调用者</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
                  <circle cx="9" cy="7" r="4" />
                  <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value">{data?.summary.active_users_24h ?? 0}</div>
            <div className="gate-kpi-sub">
              <span>活跃独立用户 ID 计数</span>
            </div>
          </div>

          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">集群节点健康度</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
                  <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
                  <line x1="6" y1="6" x2="6.01" y2="6" />
                  <line x1="6" y1="18" x2="6.01" y2="18" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value" style={{ color: 'var(--color-success)' }}>
              {data?.summary.healthy_backends ?? 0} / {data?.summary.total_backends ?? 0}
            </div>
            <div className="gate-kpi-sub">
              <span>{data?.summary.total_backends ? Math.round(((data.summary.healthy_backends) / data.summary.total_backends) * 100) : 100}% 实例健康在线</span>
            </div>
          </div>

          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">平均响应延迟</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value">{data?.summary.avg_latency_ms ?? 0} <span style={{ fontSize: '1rem', fontWeight: 'normal' }}>ms</span></div>
            <div className="gate-kpi-sub">
              <span>最近 24 小时全网关平均</span>
            </div>
          </div>

          <div className="gate-kpi-card">
            <div className="gate-kpi-header">
              <span className="gate-kpi-label">请求成功率</span>
              <div className="gate-kpi-icon">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M22 11.08V12a10 10 0 11-5.93-9.14" />
                  <polyline points="22 4 12 14.01 9 11.01" />
                </svg>
              </div>
            </div>
            <div className="gate-kpi-value" style={{ color: successRate >= 95 ? 'var(--color-success)' : 'var(--color-warning)' }}>
              {successRate}%
            </div>
            <div className="gate-kpi-sub">
              <span>2xx / (2xx + 4xx + 5xx)</span>
            </div>
          </div>
        </div>

        {/* 核心趋势图与热门模型排行榜 */}
        <div className="gate-dashboard-row">
          <div className="gate-panel-card">
            <div className="gate-panel-title">
              <span>24 小时请求量与 Credits 走势</span>
              <div className="gate-chart-legend">
                <div className="gate-legend-item">
                  <span className="gate-legend-dot" style={{ backgroundColor: 'var(--color-primary)' }} />
                  <span>请求次数 (次)</span>
                </div>
                <div className="gate-legend-item">
                  <span className="gate-legend-dot" style={{ backgroundColor: 'var(--color-warning)' }} />
                  <span>算力消耗 (Credits)</span>
                </div>
              </div>
            </div>

            <div className="gate-chart-wrapper">
              {trendSvg && (
                <svg
                  viewBox={`0 0 ${trendSvg.width} ${trendSvg.height}`}
                  className="gate-svg-chart"
                  onMouseLeave={() => setHoveredIdx(null)}
                >
                  <defs>
                    <linearGradient id="reqGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor="var(--color-primary)" stopOpacity="0.35" />
                      <stop offset="100%" stopColor="var(--color-primary)" stopOpacity="0.0" />
                    </linearGradient>
                  </defs>

                  {/* 背景网格线 */}
                  {[0, 1, 2, 3].map((step) => {
                    const y = trendSvg.padT + (step / 3) * trendSvg.chartH
                    return (
                      <line
                        key={step}
                        x1={trendSvg.padL}
                        y1={y}
                        x2={trendSvg.width - trendSvg.padR}
                        y2={y}
                        stroke="var(--color-border-subtle)"
                        strokeDasharray="4 4"
                      />
                    )
                  })}

                  {/* 面积与折线 */}
                  <path d={trendSvg.areaDReq} fill="url(#reqGradient)" />
                  <path
                    d={trendSvg.pathDReq}
                    fill="none"
                    stroke="var(--color-primary)"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                  <path
                    d={trendSvg.pathDCred}
                    fill="none"
                    stroke="var(--color-warning)"
                    strokeWidth="2"
                    strokeDasharray="3 3"
                    strokeLinecap="round"
                  />

                  {/* X 轴刻度标签 */}
                  {trendSvg.trends.map((t, i) => {
                    if (i % 3 !== 0 && i !== trendSvg.trends.length - 1) return null
                    const p = trendSvg.pointsReq[i]
                    return (
                      <text
                        key={i}
                        x={p.x}
                        y={trendSvg.height - 12}
                        fill="var(--color-text-muted)"
                        fontSize="11"
                        textAnchor="middle"
                      >
                        {t.hour}
                      </text>
                    )
                  })}

                  {/* 悬停交互透明热区与十字光标 */}
                  {trendSvg.pointsReq.map((p, i) => (
                    <g key={i} onMouseEnter={() => setHoveredIdx(i)}>
                      <rect
                        x={p.x - 15}
                        y={trendSvg.padT}
                        width={30}
                        height={trendSvg.chartH}
                        fill="transparent"
                        style={{ cursor: 'pointer' }}
                      />
                      {hoveredIdx === i && (
                        <>
                          <line
                            x1={p.x}
                            y1={trendSvg.padT}
                            x2={p.x}
                            y2={trendSvg.height - trendSvg.padB}
                            stroke="var(--color-primary)"
                            strokeWidth="1.5"
                            strokeDasharray="2 2"
                          />
                          <circle cx={p.x} cy={p.y} r="5" fill="var(--color-primary)" stroke="var(--color-bg-surface)" strokeWidth="2" />
                          <circle cx={trendSvg.pointsCred[i].x} cy={trendSvg.pointsCred[i].y} r="4" fill="var(--color-warning)" stroke="var(--color-bg-surface)" strokeWidth="2" />
                        </>
                      )}
                    </g>
                  ))}
                </svg>
              )}

              {/* 浮层 Tooltip */}
              {hoveredIdx !== null && trendSvg && (
                <div
                  style={{
                    position: 'absolute',
                    top: '10px',
                    left: `${Math.min(Math.max((hoveredIdx / (trendSvg.trends.length - 1)) * 100, 10), 85)}%`,
                    transform: 'translateX(-50%)',
                    backgroundColor: 'var(--color-bg-surface)',
                    border: '1px solid var(--color-border-primary)',
                    borderRadius: '6px',
                    padding: '0.5rem 0.75rem',
                    boxShadow: 'var(--shadow-md)',
                    fontSize: '0.8rem',
                    pointerEvents: 'none',
                    zIndex: 10,
                    minWidth: '130px',
                  }}
                >
                  <div style={{ fontWeight: 'bold', borderBottom: '1px solid var(--color-border-subtle)', paddingBottom: '3px', marginBottom: '4px' }}>
                    {trendSvg.trends[hoveredIdx].hour} 统计
                  </div>
                  <div style={{ color: 'var(--color-primary)' }}>
                    请求: {trendSvg.trends[hoveredIdx].requests} 次
                  </div>
                  <div style={{ color: 'var(--color-warning)' }}>
                    算力: {trendSvg.trends[hoveredIdx].cost_credits.toFixed(2)} Credits
                  </div>
                  {trendSvg.trends[hoveredIdx].errors > 0 && (
                    <div style={{ color: 'var(--color-danger)' }}>
                      异常: {trendSvg.trends[hoveredIdx].errors} 次
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* 热门模型 TOP 5 */}
          <div className="gate-panel-card">
            <div className="gate-panel-title">
              <span>热门模型 TOP 5</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>过去 24 小时</span>
            </div>

            <div className="gate-top-models-list">
              {(!data?.top_models || data.top_models.length === 0) ? (
                <div className="gate-empty-box">暂无模型调用数据</div>
              ) : (
                data.top_models.map((item, idx) => (
                  <div key={idx} className="gate-model-rank-item">
                    <div className="gate-model-rank-info">
                      <span className="gate-model-name">{item.model}</span>
                      <span className="gate-model-stats">
                        {item.count} 次 · {item.percentage}%
                      </span>
                    </div>
                    <div className="gate-progress-track">
                      <div className="gate-progress-bar" style={{ width: `${item.percentage}%` }} />
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>

        {/* 下层：调用状态分布与最近异常排障 */}
        <div className="gate-dashboard-row">
          <div className="gate-panel-card">
            <div className="gate-panel-title">
              <span>HTTP 响应状态分布</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>过去 24 小时</span>
            </div>
            <div className="gate-status-pills">
              <div className="gate-status-pill gate-status-pill--success">
                <span className="gate-status-pill-title">2xx 正常成功</span>
                <span className="gate-status-pill-val">{data?.status_counts.status_2xx.toLocaleString() ?? 0}</span>
              </div>
              <div className="gate-status-pill gate-status-pill--warn">
                <span className="gate-status-pill-title">4xx 客户端限制/未授权</span>
                <span className="gate-status-pill-val">{data?.status_counts.status_4xx.toLocaleString() ?? 0}</span>
              </div>
              <div className="gate-status-pill gate-status-pill--error">
                <span className="gate-status-pill-title">5xx 上游/网关异常</span>
                <span className="gate-status-pill-val">{data?.status_counts.status_5xx.toLocaleString() ?? 0}</span>
              </div>
            </div>
          </div>

          <div className="gate-panel-card">
            <div className="gate-panel-title">
              <span>最近异常调用排障</span>
              <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>最新 5 条</span>
            </div>

            <div className="gate-error-list">
              {(!data?.recent_errors || data.recent_errors.length === 0) ? (
                <div className="gate-empty-box" style={{ padding: '1rem' }}>
                  近期无异常报错，系统运行良好
                </div>
              ) : (
                data.recent_errors.map((err) => (
                  <div key={err.id} className="gate-error-item">
                    <div>
                      <div style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                        {err.model || 'unknown'} · {err.path}
                      </div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', marginTop: '2px' }}>
                        IP: {err.client_ip} · {err.error_message ? err.error_message.slice(0, 40) : '上游调用失败'}
                      </div>
                    </div>
                    <span className="gate-error-code">HTTP {err.status_code}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default DashboardPage
