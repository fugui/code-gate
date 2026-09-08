import React, { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Filter, Eye, FileText, CheckCircle2, AlertTriangle, XCircle } from 'lucide-react'
import { Pagination, Drawer } from '@code/common'
import { fetchAdminLogs } from '../../api/client'
import { AccessLogItem } from '../../types'

export const LogsPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('pageSize') || '25', 10)
  const modelFilter = searchParams.get('model') || ''
  const statusFilter = searchParams.get('status') || ''

  const [logs, setLogs] = useState<AccessLogItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  // 抽屉查看详情
  const [activeLog, setActiveLog] = useState<AccessLogItem | null>(null)

  const loadLogs = async () => {
    try {
      setLoading(true)
      const res = await fetchAdminLogs(page, pageSize, modelFilter, statusFilter)
      setLogs(res.data)
      setTotal(res.total)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadLogs()
  }, [page, pageSize, modelFilter, statusFilter])

  const handlePageChange = (newPage: number) => {
    setSearchParams({
      page: String(newPage),
      pageSize: String(pageSize),
      model: modelFilter,
      status: statusFilter,
    })
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setSearchParams({
      page: '1',
      pageSize: String(newPageSize),
      model: modelFilter,
      status: statusFilter,
    })
  }

  const handleFilterChange = (model: string, status: string) => {
    setSearchParams({
      page: '1',
      pageSize: String(pageSize),
      model,
      status,
    })
  }

  const renderStatus = (code: number) => {
    if (code >= 200 && code < 300) {
      return (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', color: 'var(--color-success)', fontWeight: 600 }}>
          <CheckCircle2 size={14} />
          <span>{code}</span>
        </span>
      )
    }
    if (code === 429) {
      return (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', color: 'var(--color-warning)', fontWeight: 600 }}>
          <AlertTriangle size={14} />
          <span>429 限额</span>
        </span>
      )
    }
    return (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.25rem', color: 'var(--color-danger)', fontWeight: 600 }}>
        <XCircle size={14} />
        <span>{code}</span>
      </span>
    )
  }

  return (
    <div className="gate-page-container">
      <div>
        <h2 style={{ fontSize: '1.4rem', fontWeight: 700, margin: '0 0 0.5rem 0' }}>网关全链路调用审计日志</h2>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.9rem' }}>
          记录原始直通传输耗时、TTFT 首字延迟、缓存命中与 Credits 算力精准扣减明细，日志默认自动轮转留存 7 天。
        </p>
      </div>

      <div className="gate-card">
        {/* 顶部过滤器 */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <Filter size={16} color="var(--color-text-muted)" />
              <input
                type="text"
                placeholder="按模型过滤 (如 deepseek)..."
                value={modelFilter}
                onChange={(e) => handleFilterChange(e.target.value, statusFilter)}
                style={{
                  padding: '0.4rem 0.75rem',
                  borderRadius: '6px',
                  border: '1px solid var(--color-border-primary)',
                  background: 'var(--color-bg-surface)',
                  color: 'var(--color-text-primary)',
                  fontSize: '0.85rem',
                }}
              />
            </div>

            <select
              value={statusFilter}
              onChange={(e) => handleFilterChange(modelFilter, e.target.value)}
              style={{
                padding: '0.4rem 0.75rem',
                borderRadius: '6px',
                border: '1px solid var(--color-border-primary)',
                background: 'var(--color-bg-surface)',
                color: 'var(--color-text-primary)',
                fontSize: '0.85rem',
              }}
            >
              <option value="">全部状态码</option>
              <option value="200">200 正常</option>
              <option value="429">429 配额拦截</option>
              <option value="500">500 异常</option>
            </select>
          </div>

          <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)' }}>
            共检索到 <strong style={{ color: 'var(--color-text-primary)' }}>{total}</strong> 条调用记录
          </div>
        </div>

        {/* 表格 */}
        {loading ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>检索日志中...</div>
        ) : logs.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>暂无调用记录</div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="gate-table">
              <thead>
                <tr>
                  <th>请求时间</th>
                  <th>用户 / 凭证</th>
                  <th>模型</th>
                  <th>协议</th>
                  <th>状态</th>
                  <th>耗时 / TTFT</th>
                  <th>Token 用量 (入/缓/出)</th>
                  <th>扣减 Credits</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {logs.map((log) => (
                  <tr key={log.id}>
                    <td style={{ color: 'var(--color-text-secondary)', fontSize: '0.8rem' }}>
                      {new Date(log.created_at).toLocaleString()}
                    </td>
                    <td>
                      <div>用户 <code>#{log.user_id}</code></div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>{log.client_ip}</div>
                    </td>
                    <td><strong>{log.model}</strong></td>
                    <td>
                      <span
                        style={{
                          padding: '2px 6px',
                          borderRadius: '4px',
                          fontSize: '0.75rem',
                          background: log.protocol === 'responses' ? 'var(--color-primary-subtle)' : 'var(--color-bg-muted)',
                          color: log.protocol === 'responses' ? 'var(--color-primary)' : 'var(--color-text-secondary)',
                        }}
                      >
                        {log.protocol}
                      </span>
                    </td>
                    <td>{renderStatus(log.status_code)}</td>
                    <td style={{ fontSize: '0.85rem' }}>
                      <div>{log.duration_ms}ms</div>
                      {log.ttft_ms > 0 && <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>首字: {log.ttft_ms}ms</div>}
                    </td>
                    <td style={{ fontSize: '0.85rem' }}>
                      <span>{log.input_tokens}</span> / <span style={{ color: 'var(--color-success)' }}>{log.cache_hit_tokens}</span> / <span>{log.output_tokens}</span>
                    </td>
                    <td>
                      <span style={{ fontWeight: 600, color: 'var(--color-primary)' }}>
                        {log.cost_credits.toFixed(3)}
                      </span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        onClick={() => setActiveLog(log)}
                        style={{ padding: '0.3rem 0.6rem', fontSize: '0.8rem', display: 'inline-flex', alignItems: 'center', gap: '0.25rem' }}
                      >
                        <Eye size={13} />
                        <span>排障报文</span>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* 分页栏 */}
        <div style={{ marginTop: '1.25rem' }}>
          <Pagination
            page={page}
            totalItems={total}
            pageSize={pageSize}
            onPageChange={handlePageChange}
            onPageSizeChange={handlePageSizeChange}
          />
        </div>
      </div>

      {/* 4 阶段原始报文转储查看抽屉 */}
      <Drawer
        open={Boolean(activeLog)}
        onClose={() => setActiveLog(null)}
        title={activeLog ? `排障报文明细 - 日志 #${activeLog.id}` : ''}
        width="lg"
      >
        {activeLog && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ padding: '0.85rem', backgroundColor: 'var(--color-bg-muted)', borderRadius: '6px', fontSize: '0.85rem' }}>
              <div><strong>模型：</strong> {activeLog.model} ({activeLog.protocol} 协议)</div>
              <div><strong>调用端 IP：</strong> {activeLog.client_ip} | <strong>User-Agent:</strong> {activeLog.user_agent}</div>
              <div><strong>用量：</strong> 输入 {activeLog.input_tokens} | 命中缓存 {activeLog.cache_hit_tokens} | 输出 {activeLog.output_tokens}</div>
              <div><strong>总算力扣减：</strong> {activeLog.cost_credits.toFixed(3)} Credits</div>
              {activeLog.error_message && (
                <div style={{ color: 'var(--color-danger)', marginTop: '0.4rem' }}>
                  <strong>异常原因：</strong> {activeLog.error_message}
                </div>
              )}
            </div>

            <div>
              <h4 style={{ margin: '0 0 0.5rem 0', fontSize: '0.95rem' }}>4 阶段原始排障报文架构说明</h4>
              <p style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)', margin: '0 0 0.75rem 0' }}>
                当请求遭遇异常或启用排障转储时，系统自动在本地保留以下 4 阶段原始二进制/文本报文快照：
              </p>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                <div style={{ padding: '0.75rem', border: '1px solid var(--color-border-primary)', borderRadius: '6px' }}>
                  <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--color-primary)' }}>1. 客户端原始请求 (Raw Request)</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>包含客户端真实发送的原始 Body、Header 与 Session-ID。</div>
                </div>
                <div style={{ padding: '0.75rem', border: '1px solid var(--color-border-primary)', borderRadius: '6px' }}>
                  <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--color-primary)' }}>2. 后端转换请求 (Forwarded Request)</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>直通发往目标物理实例的经过 Header 剥离与规范化的报文。</div>
                </div>
                <div style={{ padding: '0.75rem', border: '1px solid var(--color-border-primary)', borderRadius: '6px' }}>
                  <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--color-primary)' }}>3. 后端原始响应 (Backend Response)</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>物理后端回传的原始字节流与 SSE 片段。</div>
                </div>
                <div style={{ padding: '0.75rem', border: '1px solid var(--color-border-primary)', borderRadius: '6px' }}>
                  <div style={{ fontWeight: 600, fontSize: '0.85rem', color: 'var(--color-primary)' }}>4. 网关转换输出 (Client Response)</div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '0.25rem' }}>附加网关签名与保活心跳后回传客户端的最终报文。</div>
                </div>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1rem' }}>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setActiveLog(null)}
                style={{ padding: '0.4rem 1.2rem' }}
              >
                关闭
              </button>
            </div>
          </div>
        )}
      </Drawer>
    </div>
  )
}
