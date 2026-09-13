import React, { useEffect, useState } from 'react'
import {
  Shield,
  Plus,
  Edit2,
  Trash2,
  Clock,
  Users,
  Zap,
  CheckCircle2,
  Calendar,
  Sparkles,
  Moon,
  Sun,
  AlertCircle,
  ToggleLeft,
  ToggleRight,
  Layers,
  Info,
} from 'lucide-react'
import { Drawer } from '@code/common'
import {
  fetchAdminPolicies,
  saveAdminPolicy,
  deleteAdminPolicy,
  fetchAdminTimeMultipliers,
  updateAdminTimeMultipliers,
} from '../../api/client'
import { QuotaPolicyItem, TimeMultiplierRule, TimeMultipliersData } from '../../types'

interface TimeRangeRow {
  start: string
  end: string
}

const DAY_LABELS = [
  { day: 1, label: '周一' },
  { day: 2, label: '周二' },
  { day: 3, label: '周三' },
  { day: 4, label: '周四' },
  { day: 5, label: '周五' },
  { day: 6, label: '周六' },
  { day: 7, label: '周日' },
]

export const AdminPoliciesPage: React.FC = () => {
  // 1. 配额策略池状态
  const [policies, setPolicies] = useState<QuotaPolicyItem[]>([])
  const [loadingPolicies, setLoadingPolicies] = useState(true)
  const [policyDrawerOpen, setPolicyDrawerOpen] = useState(false)
  const [editingPolicy, setEditingPolicy] = useState<QuotaPolicyItem | null>(null)
  const [savingPolicy, setSavingPolicy] = useState(false)

  // 策略表单字段
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [dailyCredits, setDailyCredits] = useState(500)
  const [weeklyCredits, setWeeklyCredits] = useState(2000)
  const [rpm, setRpm] = useState(120)
  const [defaultModel, setDefaultModel] = useState('')
  const [modelWhitelistStr, setModelWhitelistStr] = useState('*')
  const [timeRanges, setTimeRanges] = useState<TimeRangeRow[]>([])

  // 2. 全局时段算力倍率状态
  const [multiplierData, setMultiplierData] = useState<TimeMultipliersData | null>(null)
  const [loadingMultipliers, setLoadingMultipliers] = useState(true)
  const [multiplierDrawerOpen, setMultiplierDrawerOpen] = useState(false)
  const [editingMultiplier, setEditingMultiplier] = useState<TimeMultiplierRule | null>(null)
  const [savingMultiplier, setSavingMultiplier] = useState(false)
  const [hotReloadSuccess, setHotReloadSuccess] = useState(false)

  // 倍率规则表单字段
  const [multName, setMultName] = useState('')
  const [multDesc, setMultDesc] = useState('')
  const [multDays, setMultDays] = useState<number[]>([1, 2, 3, 4, 5, 6, 7])
  const [multStart, setMultStart] = useState('21:00')
  const [multEnd, setMultEnd] = useState('09:00')
  const [multValue, setMultValue] = useState(0.2)
  const [multEnabled, setMultEnabled] = useState(true)

  // 数据初始化加载
  const loadPolicies = async () => {
    try {
      setLoadingPolicies(true)
      const list = await fetchAdminPolicies()
      setPolicies(list)
    } catch (err: unknown) {
      console.error('加载配额策略失败:', err)
    } finally {
      setLoadingPolicies(false)
    }
  }

  const loadMultipliers = async () => {
    try {
      setLoadingMultipliers(true)
      const data = await fetchAdminTimeMultipliers()
      setMultiplierData(data)
    } catch (err: unknown) {
      console.error('加载全局时段倍率失败:', err)
    } finally {
      setLoadingMultipliers(false)
    }
  }

  useEffect(() => {
    loadPolicies()
    loadMultipliers()
  }, [])

  // =========================================================================
  // 全局时段倍率逻辑
  // =========================================================================

  const handleOpenCreateMultiplier = () => {
    setEditingMultiplier(null)
    setMultName('')
    setMultDesc('')
    setMultDays([1, 2, 3, 4, 5, 6, 7])
    setMultStart('21:00')
    setMultEnd('09:00')
    setMultValue(0.2)
    setMultEnabled(true)
    setMultiplierDrawerOpen(true)
  }

  const handleOpenEditMultiplier = (rule: TimeMultiplierRule) => {
    setEditingMultiplier(rule)
    setMultName(rule.name)
    setMultDesc(rule.description || '')
    setMultDays(rule.days_of_week && rule.days_of_week.length > 0 ? rule.days_of_week : [1, 2, 3, 4, 5, 6, 7])
    setMultStart(rule.start_time)
    setMultEnd(rule.end_time)
    setMultValue(rule.multiplier)
    setMultEnabled(rule.is_enabled)
    setMultiplierDrawerOpen(true)
  }

  const handleSaveMultiplier = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!multName.trim()) {
      alert('请输入规则名称')
      return
    }
    if (multValue <= 0) {
      alert('算力倍率必须大于 0')
      return
    }

    const currentRules = multiplierData ? [...multiplierData.rules] : []
    const newRule: TimeMultiplierRule = {
      id: editingMultiplier ? editingMultiplier.id : `rule-${Date.now()}`,
      name: multName.trim(),
      description: multDesc.trim(),
      days_of_week: multDays.length === 7 ? [1, 2, 3, 4, 5, 6, 7] : multDays,
      start_time: multStart.trim(),
      end_time: multEnd.trim(),
      multiplier: Number(multValue),
      is_enabled: multEnabled,
    }

    let updatedRules: TimeMultiplierRule[] = []
    if (editingMultiplier) {
      updatedRules = currentRules.map((r) => (r.id === editingMultiplier.id ? newRule : r))
    } else {
      updatedRules = [...currentRules, newRule]
    }

    try {
      setSavingMultiplier(true)
      const res = await updateAdminTimeMultipliers(updatedRules)
      setMultiplierData(res.data)
      setMultiplierDrawerOpen(false)
      triggerHotReloadAlert()
    } catch (err: unknown) {
      alert((err as Error).message || '保存规则失败')
    } finally {
      setSavingMultiplier(false)
    }
  }

  const handleToggleMultiplier = async (rule: TimeMultiplierRule) => {
    if (!multiplierData) return
    const updatedRules = multiplierData.rules.map((r) =>
      r.id === rule.id ? { ...r, is_enabled: !r.is_enabled } : r
    )
    try {
      const res = await updateAdminTimeMultipliers(updatedRules)
      setMultiplierData(res.data)
      triggerHotReloadAlert()
    } catch (err: unknown) {
      alert((err as Error).message || '切换状态失败')
    }
  }

  const handleDeleteMultiplier = async (rule: TimeMultiplierRule) => {
    if (!multiplierData) return
    if (!window.confirm(`确定要删除倍率规则 [${rule.name}] 吗？`)) return
    const updatedRules = multiplierData.rules.filter((r) => r.id !== rule.id)
    try {
      const res = await updateAdminTimeMultipliers(updatedRules)
      setMultiplierData(res.data)
      triggerHotReloadAlert()
    } catch (err: unknown) {
      alert((err as Error).message || '删除规则失败')
    }
  }

  const triggerHotReloadAlert = () => {
    setHotReloadSuccess(true)
    setTimeout(() => setHotReloadSuccess(false), 3000)
  }

  const toggleDaySelection = (day: number) => {
    if (multDays.includes(day)) {
      if (multDays.length === 1) return // 至少保留一天
      setMultDays(multDays.filter((d) => d !== day))
    } else {
      setMultDays([...multDays, day].sort((a, b) => a - b))
    }
  }

  const formatDaysLabel = (days?: number[]) => {
    if (!days || days.length === 0 || days.length === 7) return '每天 (周一至周日)'
    const sorted = [...days].sort((a, b) => a - b)
    if (sorted.length === 5 && sorted.every((d, i) => d === i + 1)) return '工作日 (周一至周五)'
    if (sorted.length === 2 && sorted.includes(6) && sorted.includes(7)) return '周末 (周六/周日)'
    return sorted.map((d) => DAY_LABELS.find((item) => item.day === d)?.label || d).join(', ')
  }

  // 跨午夜检测
  const isCrossMidnight = (start: string, end: string) => {
    return start > end
  }

  // =========================================================================
  // 配额策略池逻辑
  // =========================================================================

  const handleDailyCreditsChange = (val: number) => {
    setDailyCredits(val)
    setWeeklyCredits(val * 4)
  }

  const handleOpenCreatePolicy = () => {
    setEditingPolicy(null)
    setName('')
    setDescription('')
    setDailyCredits(500)
    setWeeklyCredits(2000)
    setRpm(120)
    setDefaultModel('')
    setModelWhitelistStr('*')
    setTimeRanges([])
    setPolicyDrawerOpen(true)
  }

  const handleOpenEditPolicy = (p: QuotaPolicyItem) => {
    setEditingPolicy(p)
    setName(p.name)
    setDescription(p.description || '')
    setDailyCredits(p.daily_credits_limit)
    setWeeklyCredits(p.weekly_credits_limit)
    setRpm(p.rate_limit_rpm || 60)
    setDefaultModel(p.default_model || '')

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
    setPolicyDrawerOpen(true)
  }

  const handleSavePolicy = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    const wl = modelWhitelistStr
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)

    try {
      setSavingPolicy(true)
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
      setPolicyDrawerOpen(false)
      loadPolicies()
    } catch (err: unknown) {
      alert((err as Error).message || '保存策略失败')
    } finally {
      setSavingPolicy(false)
    }
  }

  const handleDeletePolicy = async (p: QuotaPolicyItem) => {
    if (!window.confirm(`确定要删除策略 [${p.name}] 吗？`)) return
    try {
      await deleteAdminPolicy(p.id)
      loadPolicies()
    } catch (err: unknown) {
      alert((err as Error).message || '删除策略失败')
    }
  }

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

  const curMultiplier = multiplierData?.current_multiplier ?? 1.0
  const matchedRule = multiplierData?.matched_rule

  return (
    <div className="gate-page-container">
      {/* 顶部主标题 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">配额策略池与时段治理 (Quota Policy & Time-Based Pricing)</h2>
          <p className="gate-page-subtitle">
            建立统一配额模板库与全局时段算力倍率排期。支持精细到 Credits 的 4 倍联动管控，以及夜间闲时打折 (如 0.2x) 与工作日高峰上浮 (如 1.5x) 的智能调峰。
          </p>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          {hotReloadSuccess && (
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '0.4rem',
                color: 'var(--color-success)',
                fontSize: '0.85rem',
                background: 'var(--color-bg-muted)',
                padding: '0.35rem 0.75rem',
                borderRadius: '6px',
                border: '1px solid var(--color-border-primary)',
              }}
            >
              <CheckCircle2 size={16} />
              <span>时段倍率已热生效！</span>
            </div>
          )}

          <button
            type="button"
            className="btn btn-secondary"
            onClick={handleOpenCreateMultiplier}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <Clock size={16} color="var(--color-primary)" />
            <span>+ 新增倍率排期</span>
          </button>

          <button
            type="button"
            className="btn btn-primary"
            onClick={handleOpenCreatePolicy}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <Plus size={16} />
            <span>+ 新建配额策略</span>
          </button>
        </div>
      </div>

      {/* =================================================================== */}
      {/* 板块一：全局动态时段算力倍率排期看板 (Global Multipliers) */}
      {/* =================================================================== */}
      <div className="gate-card" style={{ padding: '1.25rem', marginBottom: '2rem' }}>
        {/* 当前实时倍率指示器 */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: '1rem',
            padding: '1rem 1.25rem',
            borderRadius: '8px',
            background: curMultiplier < 1.0
              ? 'rgba(16, 185, 129, 0.08)'
              : curMultiplier > 1.0
              ? 'rgba(245, 158, 11, 0.08)'
              : 'var(--color-bg-muted)',
            border: `1px solid ${
              curMultiplier < 1.0
                ? 'var(--color-success)'
                : curMultiplier > 1.0
                ? 'var(--color-warning)'
                : 'var(--color-border-primary)'
            }`,
            marginBottom: '1.25rem',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <div
              style={{
                width: 44,
                height: 44,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: curMultiplier < 1.0
                  ? 'var(--color-success)'
                  : curMultiplier > 1.0
                  ? 'var(--color-warning)'
                  : 'var(--color-primary)',
                color: 'var(--color-text-white, #ffffff)',
                boxShadow: curMultiplier !== 1.0 ? '0 0 12px rgba(16, 185, 129, 0.4)' : 'none',
              }}
            >
              <Zap size={24} />
            </div>

            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)', fontWeight: 500 }}>
                  当前系统全网算力倍率
                </span>
                <span
                  style={{
                    fontSize: '1.25rem',
                    fontWeight: 700,
                    color: curMultiplier < 1.0
                      ? 'var(--color-success)'
                      : curMultiplier > 1.0
                      ? 'var(--color-warning)'
                      : 'var(--color-text-primary)',
                  }}
                >
                  {curMultiplier.toFixed(2)}x
                </span>
                {curMultiplier < 1.0 && (
                  <span
                    style={{
                      fontSize: '0.75rem',
                      padding: '2px 8px',
                      borderRadius: '12px',
                      background: 'var(--color-success)',
                      color: 'var(--color-text-white, #ffffff)',
                      fontWeight: 600,
                    }}
                  >
                    闲时特惠 {(curMultiplier * 10).toFixed(1)} 折
                  </span>
                )}
                {curMultiplier > 1.0 && (
                  <span
                    style={{
                      fontSize: '0.75rem',
                      padding: '2px 8px',
                      borderRadius: '12px',
                      background: 'var(--color-warning)',
                      color: 'var(--color-text-white, #ffffff)',
                      fontWeight: 600,
                    }}
                  >
                    高峰上浮 +{Math.round((curMultiplier - 1) * 100)}%
                  </span>
                )}
              </div>

              <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '2px' }}>
                {matchedRule ? (
                  <span>
                    命中生效规则：<strong>{matchedRule.name}</strong> ({matchedRule.start_time} ~ {matchedRule.end_time}
                    {isCrossMidnight(matchedRule.start_time, matchedRule.end_time) ? ' 跨午夜' : ''})
                    {matchedRule.description ? ` · ${matchedRule.description}` : ''}
                  </span>
                ) : (
                  <span>当前未命中任何特定时段规则，以 1.00x 标准基准算力计费</span>
                )}
              </div>
            </div>
          </div>

          <div style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)', textAlign: 'right' }}>
            <div>所有模型最终消耗 Credits = 基础消耗 × 模型倍率 × <strong>当前时段倍率</strong></div>
            <div style={{ color: 'var(--color-text-muted)', marginTop: '2px' }}>
              规则即刻秒级热重载生效，无需重启服务
            </div>
          </div>
        </div>

        {/* 规则表格 */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.75rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Calendar size={18} color="var(--color-primary)" />
            <h3 style={{ margin: 0, fontSize: '1rem', fontWeight: 600 }}>全局时段倍率排期规则表</h3>
          </div>
          <span style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)' }}>
            规则自上而下优先命中生效（第一条命中的规则将决定当前倍率）
          </span>
        </div>

        {loadingMultipliers ? (
          <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>加载倍率排期中...</div>
        ) : !multiplierData || multiplierData.rules.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '2rem', color: 'var(--color-text-muted)' }}>
            暂未配置特定时段倍率规则，点击右上角“+ 新增倍率排期”进行添加
          </div>
        ) : (
          <div style={{ border: '1px solid var(--color-border-primary)', borderRadius: '6px', overflow: 'hidden' }}>
            <table className="gate-table" style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ background: 'var(--color-bg-muted)', textAlign: 'left', borderBottom: '1px solid var(--color-border-primary)' }}>
                  <th style={{ padding: '0.65rem 1rem', width: '50px' }}>序号</th>
                  <th style={{ padding: '0.65rem 1rem' }}>规则名称 / 描述</th>
                  <th style={{ padding: '0.65rem 1rem' }}>适用星期</th>
                  <th style={{ padding: '0.65rem 1rem' }}>生效时段 (24H)</th>
                  <th style={{ padding: '0.65rem 1rem', width: '130px' }}>算力倍率乘数</th>
                  <th style={{ padding: '0.65rem 1rem', width: '90px' }}>状态</th>
                  <th style={{ padding: '0.65rem 1rem', width: '110px' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {multiplierData.rules.map((r, idx) => {
                  const crossMid = isCrossMidnight(r.start_time, r.end_time)
                  const isCurrentActive = matchedRule?.id === r.id

                  return (
                    <tr
                      key={r.id || idx}
                      style={{
                        borderBottom: '1px solid var(--color-border-primary)',
                        background: isCurrentActive ? 'rgba(16, 185, 129, 0.04)' : undefined,
                      }}
                    >
                      <td style={{ padding: '0.65rem 1rem', color: 'var(--color-text-muted)', fontSize: '0.85rem' }}>
                        #{idx + 1}
                      </td>
                      <td style={{ padding: '0.65rem 1rem' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                          <strong style={{ fontSize: '0.9rem', color: 'var(--color-text-primary)' }}>{r.name}</strong>
                          {isCurrentActive && (
                            <span
                              style={{
                                fontSize: '0.7rem',
                                padding: '1px 6px',
                                borderRadius: '10px',
                                background: 'var(--color-success)',
                                color: 'var(--color-text-white, #ffffff)',
                                fontWeight: 600,
                              }}
                            >
                              当前正在生效
                            </span>
                          )}
                        </div>
                        {r.description && (
                          <div style={{ fontSize: '0.8rem', color: 'var(--color-text-muted)', marginTop: '2px' }}>
                            {r.description}
                          </div>
                        )}
                      </td>
                      <td style={{ padding: '0.65rem 1rem', fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                        <span
                          style={{
                            padding: '2px 6px',
                            borderRadius: '4px',
                            background: 'var(--color-bg-muted)',
                            border: '1px solid var(--color-border-primary)',
                          }}
                        >
                          {formatDaysLabel(r.days_of_week)}
                        </span>
                      </td>
                      <td style={{ padding: '0.65rem 1rem', fontSize: '0.85rem' }}>
                        <span style={{ fontWeight: 600, color: 'var(--color-text-primary)' }}>
                          {r.start_time} ~ {r.end_time}
                        </span>
                        {crossMid && (
                          <span
                            style={{
                              marginLeft: '0.4rem',
                              fontSize: '0.7rem',
                              padding: '1px 5px',
                              borderRadius: '3px',
                              background: 'var(--color-primary-subtle)',
                              color: 'var(--color-primary)',
                            }}
                          >
                            跨午夜
                          </span>
                        )}
                      </td>
                      <td style={{ padding: '0.65rem 1rem' }}>
                        <span
                          style={{
                            display: 'inline-block',
                            padding: '2px 8px',
                            borderRadius: '4px',
                            fontWeight: 700,
                            fontSize: '0.85rem',
                            background: r.multiplier < 1.0
                              ? 'rgba(16, 185, 129, 0.12)'
                              : r.multiplier > 1.0
                              ? 'rgba(245, 158, 11, 0.12)'
                              : 'var(--color-bg-muted)',
                            color: r.multiplier < 1.0
                              ? 'var(--color-success)'
                              : r.multiplier > 1.0
                              ? 'var(--color-warning)'
                              : 'var(--color-text-secondary)',
                            border: `1px solid ${
                              r.multiplier < 1.0
                                ? 'var(--color-success)'
                                : r.multiplier > 1.0
                                ? 'var(--color-warning)'
                                : 'var(--color-border-primary)'
                            }`,
                          }}
                        >
                          {r.multiplier.toFixed(2)}x
                          {r.multiplier < 1.0 ? ` (${(r.multiplier * 10).toFixed(1)}折)` : r.multiplier > 1.0 ? ` (+${Math.round((r.multiplier - 1) * 100)}%)` : ''}
                        </span>
                      </td>
                      <td style={{ padding: '0.65rem 1rem' }}>
                        <button
                          type="button"
                          onClick={() => handleToggleMultiplier(r)}
                          style={{
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            display: 'flex',
                            alignItems: 'center',
                            color: r.is_enabled ? 'var(--color-success)' : 'var(--color-text-muted)',
                            padding: 0,
                          }}
                          title={r.is_enabled ? '点击停用' : '点击启用'}
                        >
                          {r.is_enabled ? <ToggleRight size={26} /> : <ToggleLeft size={26} />}
                        </button>
                      </td>
                      <td style={{ padding: '0.65rem 1rem' }}>
                        <div style={{ display: 'flex', gap: '0.35rem' }}>
                          <button
                            type="button"
                            className="btn btn-secondary"
                            onClick={() => handleOpenEditMultiplier(r)}
                            style={{ padding: '0.2rem 0.45rem', fontSize: '0.75rem' }}
                            title="编辑此规则"
                          >
                            <Edit2 size={13} />
                          </button>
                          <button
                            type="button"
                            className="btn btn-danger"
                            onClick={() => handleDeleteMultiplier(r)}
                            style={{ padding: '0.2rem 0.45rem', fontSize: '0.75rem' }}
                            title="删除此规则"
                          >
                            <Trash2 size={13} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* =================================================================== */}
      {/* 板块二：配额策略池模板库 (Quota Policy Templates) */}
      {/* =================================================================== */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Layers size={20} color="var(--color-primary)" />
          <h3 style={{ margin: 0, fontSize: '1.15rem', fontWeight: 600 }}>配额策略池模板 (Quota Policy Templates)</h3>
        </div>
        <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
          可在【用户配额台账】中将以下策略模版直接指派给用户
        </span>
      </div>

      {/* 策略列表网格 */}
      {loadingPolicies ? (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>加载策略中...</div>
      ) : policies.length === 0 ? (
        <div className="gate-card" style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
          暂无配额策略，点击右上角“+ 新建配额策略”进行配置
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
                      onClick={() => handleOpenEditPolicy(p)}
                      style={{ padding: '0.25rem 0.5rem', fontSize: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.2rem' }}
                    >
                      <Edit2 size={12} />
                      <span>编辑</span>
                    </button>
                    <button
                      type="button"
                      className="btn btn-danger"
                      onClick={() => handleDeletePolicy(p)}
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

      {/* =================================================================== */}
      {/* 抽屉 1：全局时段倍率排期规则 Drawer */}
      {/* =================================================================== */}
      <Drawer
        open={multiplierDrawerOpen}
        onClose={() => setMultiplierDrawerOpen(false)}
        title={editingMultiplier ? `编辑倍率排期: ${editingMultiplier.name}` : '新建时段算力倍率排期'}
        width="540px"
      >
        <form onSubmit={handleSaveMultiplier} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              规则名称 Name <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={multName}
              onChange={(e) => setMultName(e.target.value)}
              placeholder="例如: 夜间空闲算力特惠 (0.2x) 或 工作日早高峰 (1.5x)"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              规则说明 Description
            </label>
            <input
              type="text"
              className="gate-input"
              value={multDesc}
              onChange={(e) => setMultDesc(e.target.value)}
              placeholder="如：鼓励夜间执行自动化批量评测或研发生成任务"
            />
          </div>

          {/* 适用星期 */}
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem' }}>
              <label style={{ fontSize: '0.875rem', fontWeight: 600 }}>适用星期 Days of Week</label>
              <div style={{ display: 'flex', gap: '0.35rem' }}>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setMultDays([1, 2, 3, 4, 5, 6, 7])}
                  style={{ padding: '0.15rem 0.5rem', fontSize: '0.75rem' }}
                >
                  每天
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setMultDays([1, 2, 3, 4, 5])}
                  style={{ padding: '0.15rem 0.5rem', fontSize: '0.75rem' }}
                >
                  工作日
                </button>
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setMultDays([6, 7])}
                  style={{ padding: '0.15rem 0.5rem', fontSize: '0.75rem' }}
                >
                  周末
                </button>
              </div>
            </div>

            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              {DAY_LABELS.map((item) => {
                const selected = multDays.includes(item.day)
                return (
                  <button
                    key={item.day}
                    type="button"
                    onClick={() => toggleDaySelection(item.day)}
                    style={{
                      padding: '0.35rem 0.75rem',
                      borderRadius: '6px',
                      fontSize: '0.85rem',
                      cursor: 'pointer',
                      border: `1px solid ${selected ? 'var(--color-primary)' : 'var(--color-border-primary)'}`,
                      background: selected ? 'var(--color-primary-subtle)' : 'var(--color-bg-surface)',
                      color: selected ? 'var(--color-primary)' : 'var(--color-text-secondary)',
                      fontWeight: selected ? 600 : 400,
                    }}
                  >
                    {item.label}
                  </button>
                )
              })}
            </div>
          </div>

          {/* 生效时间段 */}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                起始时间 Start Time <span style={{ color: 'var(--color-danger)' }}>*</span>
              </label>
              <input
                type="time"
                className="gate-input"
                value={multStart}
                onChange={(e) => setMultStart(e.target.value)}
                required
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                结束时间 End Time <span style={{ color: 'var(--color-danger)' }}>*</span>
              </label>
              <input
                type="time"
                className="gate-input"
                value={multEnd}
                onChange={(e) => setMultEnd(e.target.value)}
                required
              />
            </div>
          </div>

          {isCrossMidnight(multStart, multEnd) && (
            <div
              style={{
                display: 'flex',
                gap: '0.5rem',
                alignItems: 'flex-start',
                padding: '0.65rem 0.85rem',
                borderRadius: '6px',
                background: 'rgba(59, 130, 246, 0.08)',
                border: '1px solid var(--color-border-primary)',
                fontSize: '0.8rem',
                color: 'var(--color-primary)',
              }}
            >
              <Info size={15} style={{ flexShrink: 0, marginTop: '2px' }} />
              <div>
                已识别为<strong>跨午夜时段 ({multStart} 至次日 {multEnd})</strong>。系统将在当天深夜延续生效至次日清晨。
              </div>
            </div>
          )}

          {/* 算力倍率乘数 */}
          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              算力倍率乘数 Multiplier <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
              <input
                type="number"
                step="0.05"
                min="0.01"
                className="gate-input"
                value={multValue}
                onChange={(e) => setMultValue(parseFloat(e.target.value) || 0)}
                style={{ width: '140px' }}
                required
              />
              <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>
                {multValue < 1.0 ? (
                  <span style={{ color: 'var(--color-success)', fontWeight: 600 }}>
                    打折特惠：原算力消耗乘以 {multValue.toFixed(2)} (即 {(multValue * 10).toFixed(1)} 折)
                  </span>
                ) : multValue > 1.0 ? (
                  <span style={{ color: 'var(--color-warning)', fontWeight: 600 }}>
                    高峰加成：原算力消耗上浮 {Math.round((multValue - 1) * 100)}%
                  </span>
                ) : (
                  <span>常规基准费率 (1.00x)</span>
                )}
              </span>
            </div>
            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)', display: 'block', marginTop: '0.35rem' }}>
              例如设置 0.2 时，原扣除 100 Credits 的推理任务在此时间段仅扣除 20 Credits。
            </span>
          </div>

          {/* 是否启用 */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
            <input
              type="checkbox"
              id="multEnabledCheck"
              checked={multEnabled}
              onChange={(e) => setMultEnabled(e.target.checked)}
              style={{ width: 16, height: 16, cursor: 'pointer' }}
            />
            <label htmlFor="multEnabledCheck" style={{ fontSize: '0.875rem', fontWeight: 500, cursor: 'pointer' }}>
              立即启用此倍率排期规则
            </label>
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
            <button type="button" className="btn btn-secondary" onClick={() => setMultiplierDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={savingMultiplier}>
              {savingMultiplier ? '正在保存...' : '确认保存规则'}
            </button>
          </div>
        </form>
      </Drawer>

      {/* =================================================================== */}
      {/* 抽屉 2：创建/编辑配额策略 Drawer */}
      {/* =================================================================== */}
      <Drawer
        open={policyDrawerOpen}
        onClose={() => setPolicyDrawerOpen(false)}
        title={editingPolicy ? `编辑策略: ${editingPolicy.name}` : '新建配额策略'}
        width="540px"
      >
        <form onSubmit={handleSavePolicy} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
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
            <button type="button" className="btn btn-secondary" onClick={() => setPolicyDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={savingPolicy}>
              {savingPolicy ? '正在保存...' : '确认保存策略'}
            </button>
          </div>
        </form>
      </Drawer>
    </div>
  )
}
