import React, { useEffect, useState } from 'react'
import { Shield, Plus, Edit2, Trash2, Clock, Users, Zap, CheckCircle2 } from 'lucide-react'
import { Drawer } from '@code/common'
import { fetchAdminPolicies, saveAdminPolicy, deleteAdminPolicy } from '../../api/client'
import { QuotaPolicyItem } from '../../types'

interface TimeRangeRow {
  start: string
  end: string
}

export const AdminPoliciesPage: React.FC = () => {
  const [policies, setPolicies] = useState<QuotaPolicyItem[]>([])
  const [loading, setLoading] = useState(true)

  // Drawer 状态
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [editingPolicy, setEditingPolicy] = useState<QuotaPolicyItem | null>(null)
  const [saving, setSaving] = useState(false)

  // 表单状态
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [dailyCredits, setDailyCredits] = useState(500)
  const [weeklyCredits, setWeeklyCredits] = useState(2000)
  const [rpm, setRpm] = useState(120)
  const [defaultModel, setDefaultModel] = useState('')
  const [modelWhitelistStr, setModelWhitelistStr] = useState('*')
  const [timeRanges, setTimeRanges] = useState<TimeRangeRow[]>([])

  const loadData = async () => {
    try {
      setLoading(true)
      const list = await fetchAdminPolicies()
      setPolicies(list)
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  // 日限额联动周限额
  const handleDailyCreditsChange = (val: number) => {
    setDailyCredits(val)
    // 自动按 4 倍联动填充
    setWeeklyCredits(val * 4)
  }

  // 打开创建
  const handleOpenCreate = () => {
    setEditingPolicy(null)
    setName('')
    setDescription('')
    setDailyCredits(500)
    setWeeklyCredits(2000)
    setRpm(120)
    setDefaultModel('')
    setModelWhitelistStr('*')
    setTimeRanges([])
    setDrawerOpen(true)
  }

  // 打开编辑
  const handleOpenEdit = (p: QuotaPolicyItem) => {
    setEditingPolicy(p)
    setName(p.name)
    setDescription(p.description || '')
    setDailyCredits(p.daily_credits_limit)
    setWeeklyCredits(p.weekly_credits_limit)
    setRpm(p.rate_limit_rpm || 60)
    setDefaultModel(p.default_model || '')

    // 解析白名单
    if (Array.isArray(p.model_whitelist)) {
      setModelWhitelistStr(p.model_whitelist.join(', '))
    } else if (typeof p.model_whitelist === 'string') {
      try {
        const parsed = JSON.parse(p.model_whitelist)
        setModelWhitelistStr(Array.isArray(parsed) ? parsed.join(', ') : '*')
      } catch {
        setModelWhitelistStr('*')
      }
    } else {
      setModelWhitelistStr('*')
    }

    // 解析时段
    let trs: TimeRangeRow[] = []
    if (Array.isArray(p.time_ranges)) {
      trs = p.time_ranges as TimeRangeRow[]
    } else if (typeof p.time_ranges === 'string') {
      try {
        trs = JSON.parse(p.time_ranges)
      } catch {
        trs = []
      }
    }
    setTimeRanges(trs || [])
    setDrawerOpen(true)
  }

  // 保存策略
  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    // 格式化白名单数组
    const wl = modelWhitelistStr
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)

    try {
      setSaving(true)
      await saveAdminPolicy({
        id: editingPolicy ? editingPolicy.id : 0,
        name: name.trim(),
        description: description.trim(),
        daily_credits_limit: dailyCredits,
        weekly_credits_limit: weeklyCredits,
        rate_limit_rpm: rpm,
        default_model: defaultModel.trim(),
        model_whitelist: wl.length > 0 ? (wl as any) : ['*'],
        time_ranges: timeRanges as any,
      })
      setDrawerOpen(false)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '保存策略失败')
    } finally {
      setSaving(false)
    }
  }

  // 删除策略
  const handleDelete = async (p: QuotaPolicyItem) => {
    if (!window.confirm(`确定要删除策略 [${p.name}] 吗？`)) return
    try {
      await deleteAdminPolicy(p.id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '删除策略失败')
    }
  }

  // 添加时段行
  const handleAddTimeRange = () => {
    setTimeRanges([...timeRanges, { start: '08:00', end: '20:00' }])
  }

  const handleRemoveTimeRange = (idx: number) => {
    setTimeRanges(timeRanges.filter((_, i) => i !== idx))
  }

  const handleUpdateTimeRange = (idx: number, field: 'start' | 'end', val: string) => {
    const updated = [...timeRanges]
    updated[idx][field] = val
    setTimeRanges(updated)
  }

  return (
    <div className="gate-page-container">
      {/* 顶部标题与操作 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">配额策略池与时段治理 (Quota Policy Management)</h2>
          <p className="gate-page-subtitle">
            建立统一配额策略模板库，支持精确到 Credits 的日/周总量 4 倍联动管控、RPM 频次限制与跨午夜时间段编排。
          </p>
        </div>

        <button
          type="button"
          className="btn btn-primary"
          onClick={handleOpenCreate}
          style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
        >
          <Plus size={16} />
          <span>新建配额策略</span>
        </button>
      </div>

      {/* 策略列表网格 */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>加载策略中...</div>
      ) : policies.length === 0 ? (
        <div className="gate-card" style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
          暂无配额策略，点击右上角“新建配额策略”进行配置
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))', gap: '1rem' }}>
          {policies.map((p) => {
            let trList: TimeRangeRow[] = []
            if (Array.isArray(p.time_ranges)) {
              trList = p.time_ranges as TimeRangeRow[]
            } else if (typeof p.time_ranges === 'string') {
              try {
                trList = JSON.parse(p.time_ranges)
              } catch {
                trList = []
              }
            }

            return (
              <div key={p.id} className="gate-card" style={{ display: 'flex', flexDirection: 'column', gap: '0.85rem' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <Shield size={18} color="var(--color-primary)" />
                    <strong style={{ fontSize: '1.05rem', color: 'var(--color-text-primary)' }}>{p.name}</strong>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <button
                      type="button"
                      className="btn btn-secondary"
                      onClick={() => handleOpenEdit(p)}
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.2rem' }}
                    >
                      <Edit2 size={12} />
                      <span>编辑</span>
                    </button>
                    <button
                      type="button"
                      className="btn btn-danger"
                      onClick={() => handleDelete(p)}
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem' }}
                      title="删除策略"
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                </div>

                {p.description && (
                  <div style={{ fontSize: '0.825rem', color: 'var(--color-text-secondary)' }}>{p.description}</div>
                )}

                {/* 核心指标统计 */}
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1fr 1fr',
                    gap: '0.5rem',
                    background: 'var(--color-bg-muted)',
                    padding: '0.75rem',
                    borderRadius: '6px',
                  }}
                >
                  <div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>每日 Credits 限额</div>
                    <strong style={{ fontSize: '1.1rem', color: 'var(--color-primary)' }}>
                      {p.daily_credits_limit.toLocaleString()}
                    </strong>
                  </div>
                  <div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>每周 Credits 预算</div>
                    <strong style={{ fontSize: '1.1rem', color: 'var(--color-success)' }}>
                      {p.weekly_credits_limit.toLocaleString()}
                    </strong>
                  </div>
                </div>

                {/* 附加限制 */}
                <div style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)', display: 'flex', flexDirection: 'column', gap: '0.35rem' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <Zap size={13} color="var(--color-warning)" />
                    <span>请求频次上限: <strong>{p.rate_limit_rpm} RPM</strong></span>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <Users size={13} color="var(--color-primary)" />
                    <span>
                      已绑定用户: <strong>{p.user_count || 0} 位</strong>
                    </span>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'flex-start', gap: '0.4rem' }}>
                    <Clock size={13} color="var(--color-info)" style={{ marginTop: '2px' }} />
                    <div>
                      可用时段:{' '}
                      {trList.length === 0 ? (
                        <span>全天 24 小时可用</span>
                      ) : (
                        trList.map((tr, i) => (
                          <span
                            key={i}
                            style={{
                              marginRight: '0.4rem',
                              padding: '1px 5px',
                              borderRadius: '3px',
                              background: 'var(--color-bg-input)',
                              fontSize: '0.75rem',
                            }}
                          >
                            {tr.start} ~ {tr.end}
                          </span>
                        ))
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 创建/编辑 Drawer */}
      <Drawer
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        title={editingPolicy ? `编辑策略: ${editingPolicy.name}` : '新建配额策略'}
        width="540px"
      >
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              策略名称 Name <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如: developer_vip_policy"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              策略描述 Description
            </label>
            <input
              type="text"
              className="gate-input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="如：适用于高频研发与夜间自动化执行的策略"
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                每日 Credits 限额 <span style={{ color: 'var(--color-danger)' }}>*</span>
              </label>
              <input
                type="number"
                className="gate-input"
                value={dailyCredits}
                onChange={(e) => handleDailyCreditsChange(parseFloat(e.target.value) || 0)}
                required
              />
              <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                输入日限额时，周限额将自动联动乘以 4
              </span>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                每周 Credits 限额 <span style={{ color: 'var(--color-danger)' }}>*</span>
              </label>
              <input
                type="number"
                className="gate-input"
                value={weeklyCredits}
                onChange={(e) => setWeeklyCredits(parseFloat(e.target.value) || 0)}
                required
              />
              <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>建议维持日限额的 4 倍联动</span>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                请求频次上限 (RPM)
              </label>
              <input
                type="number"
                className="gate-input"
                value={rpm}
                onChange={(e) => setRpm(parseInt(e.target.value, 10) || 60)}
                min="1"
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                默认首选模型
              </label>
              <input
                type="text"
                className="gate-input"
                value={defaultModel}
                onChange={(e) => setDefaultModel(e.target.value)}
                placeholder="如: deepseek-v3"
              />
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              模型白名单 (以逗号分隔，* 代表全部)
            </label>
            <input
              type="text"
              className="gate-input"
              value={modelWhitelistStr}
              onChange={(e) => setModelWhitelistStr(e.target.value)}
              placeholder="*, deepseek-v3, claude-3-7-sonnet"
            />
          </div>

          {/* 跨午夜时段动态配置 */}
          <div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
              <label style={{ fontSize: '0.875rem', fontWeight: 600 }}>可用时段区间 (未配置代表全天 24 小时)</label>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={handleAddTimeRange}
                style={{ padding: '0.2rem 0.6rem', fontSize: '0.75rem' }}
              >
                + 添加时段
              </button>
            </div>

            {timeRanges.length === 0 ? (
              <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', background: 'var(--color-bg-muted)', padding: '0.5rem', borderRadius: '4px' }}>
                当前无时段限制，全天随时可用
              </div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                {timeRanges.map((tr, idx) => (
                  <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                    <input
                      type="time"
                      className="gate-input"
                      value={tr.start}
                      onChange={(e) => handleUpdateTimeRange(idx, 'start', e.target.value)}
                      style={{ width: '130px' }}
                      required
                    />
                    <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.85rem' }}>至</span>
                    <input
                      type="time"
                      className="gate-input"
                      value={tr.end}
                      onChange={(e) => handleUpdateTimeRange(idx, 'end', e.target.value)}
                      style={{ width: '130px' }}
                      required
                    />
                    <button
                      type="button"
                      className="btn btn-danger"
                      onClick={() => handleRemoveTimeRange(idx)}
                      style={{ padding: '0.3rem 0.5rem', fontSize: '0.75rem' }}
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
            <button type="button" className="btn btn-secondary" onClick={() => setDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? '正在保存...' : '确认保存策略'}
            </button>
          </div>
        </form>
      </Drawer>
    </div>
  )
}
