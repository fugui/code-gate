import React, { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Search, Edit3, UserCheck, ShieldCheck } from 'lucide-react'
import { Pagination, Drawer } from '@code/common'
import { fetchAdminUsers, updateUserQuota } from '../../api/client'
import { UserQuotaDTO } from '../../types'

export const AdminUsersPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams()
  const page = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = parseInt(searchParams.get('pageSize') || '25', 10)
  const search = searchParams.get('search') || ''

  const [searchInput, setSearchInput] = useState(search)
  const [users, setUsers] = useState<UserQuotaDTO[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  // 抽屉编辑状态
  const [editingUser, setEditingUser] = useState<UserQuotaDTO | null>(null)
  const [selectedRole, setSelectedRole] = useState<string>('guest')
  const [customDaily, setCustomDaily] = useState<number>(100)
  const [customWeekly, setCustomWeekly] = useState<number>(400)
  const [submitting, setSubmitting] = useState(false)

  const loadUsers = async () => {
    try {
      setLoading(true)
      const res = await fetchAdminUsers(page, pageSize, search)
      setUsers(res.data)
      setTotal(res.total)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadUsers()
  }, [page, pageSize, search])

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    setSearchParams({
      page: '1',
      pageSize: String(pageSize),
      search: searchInput.trim(),
    })
  }

  const handlePageChange = (newPage: number) => {
    setSearchParams({
      page: String(newPage),
      pageSize: String(pageSize),
      search,
    })
  }

  const handlePageSizeChange = (newPageSize: number) => {
    setSearchParams({
      page: '1',
      pageSize: String(newPageSize),
      search,
    })
  }

  const openEditDrawer = (user: UserQuotaDTO) => {
    setEditingUser(user)
    setSelectedRole(user.role)
    const daily = user.custom_daily_credits ?? user.daily_limit
    const weekly = user.custom_weekly_credits ?? user.weekly_limit
    setCustomDaily(daily)
    setCustomWeekly(weekly)
  }

  const handleDailyChange = (val: number) => {
    setCustomDaily(val)
    // 自动 4 倍联动
    setCustomWeekly(Math.round(val * 4))
  }

  const handleSaveQuota = async () => {
    if (!editingUser) return
    try {
      setSubmitting(true)
      const isCustom = selectedRole === 'custom'
      await updateUserQuota(editingUser.user_id, {
        role: selectedRole,
        custom_daily_credits: isCustom ? customDaily : undefined,
        custom_weekly_credits: isCustom ? customWeekly : undefined,
      })
      setEditingUser(null)
      loadUsers()
    } catch (err: unknown) {
      alert((err as Error).message || '保存失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="gate-page-container">
      <div>
        <h2 style={{ fontSize: '1.4rem', fontWeight: 700, margin: '0 0 0.5rem 0' }}>CodeBench 用户配额台账管理</h2>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.9rem' }}>
          共享 CodeBench 用户认证系统。新用户初次进入自动分配 guest 访客保底额度，管理员可随时提权或自定义日/周 4 倍联动算力配额。
        </p>
      </div>

      <div className="gate-card">
        {/* 顶部搜索 */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
          <form onSubmit={handleSearch} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', width: 320 }}>
            <div style={{ position: 'relative', width: '100%' }}>
              <Search size={16} style={{ position: 'absolute', left: 10, top: 10, color: 'var(--color-text-muted)' }} />
              <input
                type="text"
                placeholder="搜索用户名、姓名或邮箱..."
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                style={{
                  width: '100%',
                  padding: '0.45rem 0.75rem 0.45rem 2rem',
                  borderRadius: '6px',
                  border: '1px solid var(--color-border-primary)',
                  background: 'var(--color-bg-surface)',
                  color: 'var(--color-text-primary)',
                  fontSize: '0.85rem',
                }}
              />
            </div>
            <button type="submit" className="btn btn-secondary" style={{ padding: '0.45rem 0.85rem', whiteSpace: 'nowrap' }}>
              查询
            </button>
          </form>

          <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)' }}>
            共匹配到 <strong style={{ color: 'var(--color-text-primary)' }}>{total}</strong> 位用户
          </div>
        </div>

        {/* 用户配额表格 */}
        {loading ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>加载用户列表中...</div>
        ) : users.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>未找到符合条件的用户</div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table className="gate-table">
              <thead>
                <tr>
                  <th>用户 ID</th>
                  <th>用户名 / 姓名</th>
                  <th>配额角色</th>
                  <th>关联策略</th>
                  <th>日配额 / 已用</th>
                  <th>周配额 / 已用</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.user_id}>
                    <td><code>#{u.user_id}</code></td>
                    <td>
                      <div style={{ fontWeight: 600 }}>{u.username}</div>
                      <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>{u.name || u.email || '未填'}</div>
                    </td>
                    <td>
                      <span
                        style={{
                          padding: '2px 8px',
                          borderRadius: '4px',
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          background: u.role === 'developer' ? 'var(--color-primary-subtle)' : u.role === 'custom' ? 'var(--color-warning-subtle)' : 'var(--color-bg-muted)',
                          color: u.role === 'developer' ? 'var(--color-primary)' : u.role === 'custom' ? 'var(--color-warning)' : 'var(--color-text-secondary)',
                          border: '1px solid var(--color-border-subtle)',
                        }}
                      >
                        {u.role.toUpperCase()}
                      </span>
                    </td>
                    <td style={{ color: 'var(--color-text-secondary)' }}>{u.policy_name}</td>
                    <td>
                      <span style={{ fontWeight: 600 }}>{u.daily_consumed.toFixed(1)}</span>
                      <span style={{ color: 'var(--color-text-muted)', fontSize: '0.8rem' }}> / {u.daily_limit.toFixed(1)}</span>
                    </td>
                    <td>
                      <span style={{ fontWeight: 600 }}>{u.weekly_consumed.toFixed(1)}</span>
                      <span style={{ color: 'var(--color-text-muted)', fontSize: '0.8rem' }}> / {u.weekly_limit.toFixed(1)}</span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn btn-secondary"
                        onClick={() => openEditDrawer(u)}
                        style={{ padding: '0.35rem 0.65rem', fontSize: '0.8rem', display: 'inline-flex', alignItems: 'center', gap: '0.3rem' }}
                      >
                        <Edit3 size={13} />
                        <span>配置配额</span>
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

      {/* 调整用户配额抽屉 */}
      <Drawer
        open={Boolean(editingUser)}
        onClose={() => setEditingUser(null)}
        title={editingUser ? `调整用户配额: ${editingUser.username}` : ''}
        width="md"
      >
        {editingUser && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
            <div style={{ padding: '0.75rem', backgroundColor: 'var(--color-bg-muted)', borderRadius: '6px', fontSize: '0.85rem' }}>
              <div><strong>用户 ID：</strong> #{editingUser.user_id}</div>
              <div><strong>用户名：</strong> {editingUser.username} ({editingUser.name || '无名'})</div>
              <div><strong>目前已用：</strong> 今日 {editingUser.daily_consumed.toFixed(1)} Credits / 本周 {editingUser.weekly_consumed.toFixed(1)} Credits</div>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                目标配额角色
              </label>
              <select
                value={selectedRole}
                onChange={(e) => setSelectedRole(e.target.value)}
                style={{
                  width: '100%',
                  padding: '0.5rem 0.75rem',
                  borderRadius: '6px',
                  border: '1px solid var(--color-border-primary)',
                  background: 'var(--color-bg-surface)',
                  color: 'var(--color-text-primary)',
                }}
              >
                <option value="guest">guest (保底体验策略: 日 100 / 周 400 Credits)</option>
                <option value="developer">developer (开发者策略: 日 500 / 周 2000 Credits)</option>
                <option value="vip">vip (VIP 策略: 日 2000 / 周 8000 Credits)</option>
                <option value="custom">custom (独立定制 Credits 配额)</option>
              </select>
            </div>

            {selectedRole === 'custom' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', padding: '1rem', border: '1px dashed var(--color-border-primary)', borderRadius: '6px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.3rem' }}>
                    自定义每日算力限额 (Credits)
                  </label>
                  <input
                    type="number"
                    min={1}
                    value={customDaily}
                    onChange={(e) => handleDailyChange(Number(e.target.value))}
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
                  <label style={{ display: 'block', fontSize: '0.85rem', fontWeight: 600, marginBottom: '0.3rem' }}>
                    自定义每周算力限额 (自动 4 倍联动)
                  </label>
                  <input
                    type="number"
                    min={1}
                    value={customWeekly}
                    onChange={(e) => setCustomWeekly(Number(e.target.value))}
                    style={{
                      width: '100%',
                      padding: '0.5rem 0.75rem',
                      borderRadius: '6px',
                      border: '1px solid var(--color-border-primary)',
                      background: 'var(--color-bg-surface)',
                      color: 'var(--color-text-primary)',
                    }}
                  />
                  <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', marginTop: '0.25rem', display: 'block' }}>
                    按规范，每周配额默认自动维持为每日配额的 4 倍，也可按需手动微调。
                  </span>
                </div>
              </div>
            )}

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1.5rem' }}>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => setEditingUser(null)}
                style={{ padding: '0.4rem 1rem' }}
              >
                取消
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSaveQuota}
                disabled={submitting}
                style={{ padding: '0.4rem 1.2rem' }}
              >
                {submitting ? '保存中...' : '确认生效'}
              </button>
            </div>
          </div>
        )}
      </Drawer>
    </div>
  )
}
