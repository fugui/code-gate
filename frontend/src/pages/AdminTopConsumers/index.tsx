import React, { useEffect, useState } from 'react'
import { Award, RefreshCw, TrendingUp, User, Building, ArrowUpRight, ArrowDownRight } from 'lucide-react'
import { fetchAdminTopConsumers } from '../../api/client'
import { TopConsumersData } from '../../types'

export const AdminTopConsumersPage: React.FC = () => {
  const [data, setData] = useState<TopConsumersData | null>(null)
  const [loading, setLoading] = useState(true)

  const loadData = async () => {
    try {
      setLoading(true)
      const res = await fetchAdminTopConsumers()
      setData(res)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  // 格式化数字：如 12,345
  const fmtNum = (n?: number) => (n || 0).toLocaleString()

  // 友好日期标题展示：如 "09-03"
  const fmtDateHeader = (dStr: string) => {
    const parts = dStr.split('-')
    if (parts.length >= 3) {
      return `${parts[1]}-${parts[2]}`
    }
    return dStr
  }

  const dates = data?.dates || []
  const users = data?.users || []
  const grandTotal = data?.grand_total

  return (
    <div className="gate-page-container">
      {/* 顶部标题栏 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">7天算力消耗透视大账 (Top Consumers Matrix)</h2>
          <p className="gate-page-subtitle">
            按日交叉透视团队与核心用户的算力点数（Credits）、请求吞吐与 Token 支出，打通统一人员画像反查。
          </p>
        </div>

        <button
          type="button"
          className="btn btn-secondary"
          onClick={loadData}
          disabled={loading}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
        >
          <RefreshCw size={15} className={loading ? 'spin' : ''} />
          <span>刷新台账</span>
        </button>
      </div>

      {/* 交叉大账表格 */}
      <div className="gate-card" style={{ padding: 0, overflow: 'hidden' }}>
        <div style={{ padding: '1rem 1.25rem', borderBottom: '1px solid var(--color-border-primary)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <TrendingUp size={18} color="var(--color-primary)" />
            <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 600 }}>最近 7 天全员算力消耗逐日透视矩阵</h3>
          </div>
          <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>
            单位说明：主数值为请求数 / Credits 消耗点数，次行为输入与输出 Token 统计
          </span>
        </div>

        {loading && !data ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>正在聚合计算 7 天消耗大账...</div>
        ) : users.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>过去 7 天暂无审计日志与算力消耗记录</div>
        ) : (
          <div className="gate-table-container" style={{ overflowX: 'auto' }}>
            <table className="gate-table" style={{ width: '100%', borderCollapse: 'collapse', minWidth: '1100px' }}>
              <thead>
                <tr style={{ background: 'var(--color-bg-muted)', textAlign: 'left', borderBottom: '1px solid var(--color-border-primary)' }}>
                  <th style={{ padding: '0.75rem 0.85rem', width: '50px', textAlign: 'center' }}>排名</th>
                  <th style={{ padding: '0.75rem 1rem', minWidth: '220px' }}>用户画像 / 部门</th>
                  {dates.map((d, i) => (
                    <th key={d} style={{ padding: '0.75rem 0.75rem', minWidth: '105px', textAlign: 'right' }}>
                      <div style={{ fontSize: '0.85rem', fontWeight: 600 }}>{fmtDateHeader(d)}</div>
                      <div style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)', fontWeight: 400 }}>
                        {i === 6 ? '今天' : i === 5 ? '昨天' : `D-${6 - i}`}
                      </div>
                    </th>
                  ))}
                  <th style={{ padding: '0.75rem 1rem', minWidth: '130px', textAlign: 'right', background: 'var(--color-bg-hover)' }}>
                    <div style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--color-primary)' }}>7天累计支出</div>
                  </th>
                </tr>
              </thead>
              <tbody>
                {users.map((u, idx) => (
                  <tr key={u.user_id} style={{ borderBottom: '1px solid var(--color-border-primary)' }}>
                    {/* 排名 */}
                    <td style={{ padding: '0.75rem 0.85rem', textAlign: 'center', verticalAlign: 'middle' }}>
                      {idx === 0 ? (
                        <span style={{ color: '#f59e0b', fontWeight: 700, display: 'flex', justifyContent: 'center' }}>
                          <Award size={18} />
                        </span>
                      ) : (
                        <span style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)', fontWeight: 600 }}>
                          #{idx + 1}
                        </span>
                      )}
                    </td>

                    {/* 用户身份与部门 */}
                    <td style={{ padding: '0.75rem 1rem', verticalAlign: 'middle' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                        <div
                          style={{
                            width: '28px',
                            height: '28px',
                            borderRadius: '50%',
                            background: 'var(--color-primary-subtle, rgba(59, 130, 246, 0.1))',
                            color: 'var(--color-primary)',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            fontWeight: 600,
                            fontSize: '0.8rem',
                            flexShrink: 0,
                          }}
                        >
                          {u.name.slice(0, 1).toUpperCase()}
                        </div>
                        <div style={{ overflow: 'hidden' }}>
                          <div style={{ fontWeight: 600, fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>
                            {u.name}
                            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', marginLeft: '0.4rem', fontWeight: 400 }}>
                              (@{u.username})
                            </span>
                          </div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', gap: '0.3rem', marginTop: '1px' }}>
                            <Building size={11} />
                            <span>{u.department || '研发中心'}</span>
                          </div>
                        </div>
                      </div>
                    </td>

                    {/* 逐日消耗列 */}
                    {dates.map((d) => {
                      const m = u.daily[d]
                      const hasReq = m && m.requests > 0

                      return (
                        <td key={d} style={{ padding: '0.75rem 0.75rem', textAlign: 'right', verticalAlign: 'middle' }}>
                          {hasReq ? (
                            <div>
                              <div style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>
                                {fmtNum(m.requests)} 次
                              </div>
                              <div style={{ fontSize: '0.75rem', color: 'var(--color-primary)', fontWeight: 600 }}>
                                {m.cost_credits.toFixed(1)} pts
                              </div>
                              <div style={{ fontSize: '0.7rem', color: 'var(--color-text-muted)', marginTop: '1px' }}>
                                ↑{fmtNum(m.input_tokens)} / ↓{fmtNum(m.output_tokens)}
                              </div>
                            </div>
                          ) : (
                            <span style={{ color: 'var(--color-text-muted)', fontSize: '0.8rem' }}>-</span>
                          )}
                        </td>
                      )
                    })}

                    {/* 7天总计 */}
                    <td style={{ padding: '0.75rem 1rem', textAlign: 'right', verticalAlign: 'middle', background: 'var(--color-bg-hover)' }}>
                      <div style={{ fontSize: '0.95rem', fontWeight: 700, color: 'var(--color-primary)' }}>
                        {u.total_credits.toFixed(1)} pts
                      </div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--color-text-secondary)', marginTop: '1px' }}>
                        共 {fmtNum(u.total_req)} 请求
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>

              {/* 表尾 Grand Total 全局汇总行 */}
              {grandTotal && (
                <tfoot>
                  <tr style={{ background: 'var(--color-bg-muted)', borderTop: '2px solid var(--color-border-primary)', fontWeight: 600 }}>
                    <td colSpan={2} style={{ padding: '0.85rem 1rem', textAlign: 'left' }}>
                      <strong style={{ fontSize: '0.95rem', color: 'var(--color-text-primary)' }}>全局总计 (Grand Total)</strong>
                    </td>
                    {dates.map((d) => {
                      const gt = grandTotal.daily[d]
                      return (
                        <td key={d} style={{ padding: '0.85rem 0.75rem', textAlign: 'right' }}>
                          <div style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--color-text-primary)' }}>
                            {fmtNum(gt?.requests)} 次
                          </div>
                          <div style={{ fontSize: '0.75rem', color: 'var(--color-primary)', fontWeight: 700 }}>
                            {(gt?.cost_credits || 0).toFixed(1)} pts
                          </div>
                        </td>
                      )
                    })}
                    <td style={{ padding: '0.85rem 1rem', textAlign: 'right', background: 'var(--color-bg-hover)' }}>
                      <div style={{ fontSize: '1rem', fontWeight: 800, color: 'var(--color-primary)' }}>
                        {grandTotal.total_credits.toFixed(1)} pts
                      </div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--color-text-secondary)' }}>
                        全量 {fmtNum(grandTotal.total_req)} 请求
                      </div>
                    </td>
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
