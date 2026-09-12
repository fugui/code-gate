import React, { useEffect, useState } from 'react'
import {
  Layers,
  Server,
  Plus,
  Trash2,
  Edit2,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  DownloadCloud,
  CheckCircle,
  XCircle,
  Clock,
  Zap,
} from 'lucide-react'
import { Drawer } from '@code/common'
import {
  fetchAdminModels,
  saveAdminModel,
  toggleAdminModel,
  deleteAdminModel,
  importGatewayModels,
  saveAdminBackend,
  toggleAdminBackend,
  deleteAdminBackend,
} from '../../api/client'
import { ModelDetailItem, BackendItem } from '../../types'

export const AdminBackendsPage: React.FC = () => {
  const [models, setModels] = useState<ModelDetailItem[]>([])
  const [loading, setLoading] = useState(true)
  const [expandedModelIds, setExpandedModelIds] = useState<Record<number, boolean>>({})

  // 1. 新建/编辑模型 Drawer
  const [modelDrawerOpen, setModelDrawerOpen] = useState(false)
  const [editingModel, setEditingModel] = useState<ModelDetailItem | null>(null)
  const [modelForm, setModelForm] = useState({
    name: '',
    description: '',
    multiplier: 1.0,
    default_model: '',
    model_params_str: '{}',
    // 首次创建时可选关联首个后端
    initial_base_url: '',
    initial_api_key: '',
    initial_weight: 10,
    initial_max_concurrency: 50,
  })
  const [savingModel, setSavingModel] = useState(false)

  // 2. 批量从上游网关导入 Drawer
  const [importDrawerOpen, setImportDrawerOpen] = useState(false)
  const [importForm, setImportForm] = useState({
    prefix: '',
    base_url: '',
    api_key: '',
  })
  const [importing, setImporting] = useState(false)

  // 3. 为指定模型添加后端实例 Drawer
  const [backendDrawerOpen, setBackendDrawerOpen] = useState(false)
  const [targetModelForBackend, setTargetModelForBackend] = useState<ModelDetailItem | null>(null)
  const [backendForm, setBackendForm] = useState({
    name: '',
    base_url: '',
    api_key: '',
    weight: 10,
    max_concurrency: 50,
  })
  const [savingBackend, setSavingBackend] = useState(false)

  const loadData = async () => {
    try {
      setLoading(true)
      const list = await fetchAdminModels()
      setModels(list)
      // 默认全部展开
      const expandMap: Record<number, boolean> = {}
      list.forEach((m) => {
        expandMap[m.id] = true
      })
      setExpandedModelIds((prev) => ({ ...expandMap, ...prev }))
    } catch (err: unknown) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const toggleExpand = (id: number) => {
    setExpandedModelIds((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  // 打开创建模型
  const handleOpenCreateModel = () => {
    setEditingModel(null)
    setModelForm({
      name: '',
      description: '',
      multiplier: 1.0,
      default_model: '',
      model_params_str: '{\n  "enable_thinking": false\n}',
      initial_base_url: '',
      initial_api_key: '',
      initial_weight: 10,
      initial_max_concurrency: 50,
    })
    setModelDrawerOpen(true)
  }

  // 打开编辑模型
  const handleOpenEditModel = (m: ModelDetailItem, e: React.MouseEvent) => {
    e.stopPropagation()
    setEditingModel(m)
    setModelForm({
      name: m.name,
      description: m.description || '',
      multiplier: m.multiplier || 1.0,
      default_model: m.default_model || '',
      model_params_str: JSON.stringify(m.model_params || {}, null, 2),
      initial_base_url: '',
      initial_api_key: '',
      initial_weight: 10,
      initial_max_concurrency: 50,
    })
    setModelDrawerOpen(true)
  }

  // 保存模型
  const handleSaveModel = async (e: React.FormEvent) => {
    e.preventDefault()
    let parsedParams = {}
    try {
      parsedParams = JSON.parse(modelForm.model_params_str || '{}')
    } catch {
      alert('模型参数 JSON 格式不合法，请检查')
      return
    }

    try {
      setSavingModel(true)
      const payload: any = {
        id: editingModel ? editingModel.id : 0,
        name: modelForm.name.trim(),
        description: modelForm.description.trim(),
        multiplier: Number(modelForm.multiplier) || 1.0,
        default_model: modelForm.default_model.trim(),
        model_params: parsedParams,
      }

      if (!editingModel && modelForm.initial_base_url.trim()) {
        payload.initial_backend = {
          base_url: modelForm.initial_base_url.trim(),
          api_key: modelForm.initial_api_key.trim(),
          weight: Number(modelForm.initial_weight) || 10,
          max_concurrency: Number(modelForm.initial_max_concurrency) || 50,
        }
      }

      await saveAdminModel(payload)
      setModelDrawerOpen(false)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '保存模型失败')
    } finally {
      setSavingModel(false)
    }
  }

  // 切换模型启停
  const handleToggleModel = async (m: ModelDetailItem, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await toggleAdminModel(m.id)
      setModels((prev) =>
        prev.map((item) => (item.id === m.id ? { ...item, is_enabled: !item.is_enabled } : item))
      )
    } catch (err: unknown) {
      alert((err as Error).message || '切换模型状态失败')
    }
  }

  // 删除模型
  const handleDeleteModel = async (m: ModelDetailItem, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!window.confirm(`确定要删除模型 [${m.name}] 及其关联的全部物理后端节点吗？此操作不可逆！`)) return
    try {
      await deleteAdminModel(m.id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '删除模型失败')
    }
  }

  // 批量从网关导入
  const handleImportGateway = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!importForm.prefix.trim() || !importForm.base_url.trim()) {
      alert('请填写前缀与上游 BaseURL')
      return
    }

    try {
      setImporting(true)
      const res = await importGatewayModels({
        prefix: importForm.prefix.trim(),
        base_url: importForm.base_url.trim(),
        api_key: importForm.api_key.trim(),
      })
      alert(res.message || '导入成功')
      setImportDrawerOpen(false)
      setImportForm({ prefix: '', base_url: '', api_key: '' })
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '导入上游模型失败')
    } finally {
      setImporting(false)
    }
  }

  // 为模型添加后端
  const handleOpenAddBackend = (m: ModelDetailItem, e: React.MouseEvent) => {
    e.stopPropagation()
    setTargetModelForBackend(m)
    setBackendForm({
      name: `${m.name}-node-${(m.backends?.length || 0) + 1}`,
      base_url: '',
      api_key: '',
      weight: 10,
      max_concurrency: 50,
    })
    setBackendDrawerOpen(true)
  }

  const handleSaveBackend = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!targetModelForBackend || !backendForm.base_url.trim()) return

    try {
      setSavingBackend(true)
      await saveAdminBackend({
        model_id: targetModelForBackend.id,
        name: backendForm.name.trim(),
        base_url: backendForm.base_url.trim(),
        api_key: backendForm.api_key.trim() as any,
        weight: Number(backendForm.weight) || 10,
        max_concurrency: Number(backendForm.max_concurrency) || 50,
      })
      setBackendDrawerOpen(false)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '保存后端实例失败')
    } finally {
      setSavingBackend(false)
    }
  }

  // 切换后端启停
  const handleToggleBackend = async (b: BackendItem) => {
    try {
      await toggleAdminBackend(b.id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '切换后端状态失败')
    }
  }

  // 删除后端
  const handleDeleteBackend = async (b: BackendItem) => {
    if (!window.confirm(`确定要删除物理后端 [${b.name}] 吗？`)) return
    try {
      await deleteAdminBackend(b.id)
      loadData()
    } catch (err: unknown) {
      alert((err as Error).message || '删除物理后端失败')
    }
  }

  return (
    <div className="gate-page-container">
      {/* 顶部标题与操作栏 */}
      <div className="gate-page-header">
        <div>
          <h2 className="gate-page-title">模型与物理后端层次化治理</h2>
          <p className="gate-page-subtitle">
            支持 1:N 逻辑模型与物理实例解耦编排、算力倍率乘数配置、参数透明注入及上游网关批量自动导入。
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <button
            type="button"
            className="btn btn-secondary"
            onClick={() => setImportDrawerOpen(true)}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <DownloadCloud size={16} />
            <span>网关批量导入</span>
          </button>

          <button
            type="button"
            className="btn btn-primary"
            onClick={handleOpenCreateModel}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}
          >
            <Plus size={16} />
            <span>新建逻辑模型</span>
          </button>
        </div>
      </div>

      {/* 模型列表折叠树 */}
      {loading ? (
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
          <RefreshCw size={24} className="spin" style={{ marginBottom: '0.5rem' }} />
          <div>正在加载模型拓扑树...</div>
        </div>
      ) : models.length === 0 ? (
        <div className="gate-card" style={{ textAlign: 'center', padding: '3rem', color: 'var(--color-text-muted)' }}>
          暂无逻辑模型，点击右上角“新建逻辑模型”或“网关批量导入”开始接入
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          {models.map((m) => {
            const isExpanded = !!expandedModelIds[m.id]
            const backends = m.backends || []
            const activeCount = backends.filter((b) => b.is_enabled && b.is_healthy).length

            return (
              <div
                key={m.id}
                className="gate-card"
                style={{
                  padding: 0,
                  overflow: 'hidden',
                  opacity: m.is_enabled ? 1 : 0.65,
                  transition: 'opacity 0.2s',
                }}
              >
                {/* 逻辑模型表头栏 */}
                <div
                  onClick={() => toggleExpand(m.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '1rem 1.25rem',
                    background: 'var(--color-bg-muted)',
                    borderBottom: isExpanded ? '1px solid var(--color-border-primary)' : 'none',
                    cursor: 'pointer',
                    userSelect: 'none',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                    <span style={{ color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}>
                      {isExpanded ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
                    </span>

                    <Layers size={20} color="var(--color-primary)" />

                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
                        <strong style={{ fontSize: '1.1rem', color: 'var(--color-text-primary)' }}>{m.name}</strong>

                        <span
                          className="status-badge"
                          style={{
                            background: 'var(--color-primary-subtle, rgba(59, 130, 246, 0.1))',
                            color: 'var(--color-primary)',
                            fontSize: '0.75rem',
                            padding: '2px 8px',
                            borderRadius: '12px',
                            fontWeight: 600,
                          }}
                        >
                          倍率: {m.multiplier.toFixed(1)}x
                        </span>

                        {m.default_model && (
                          <span
                            style={{
                              fontSize: '0.75rem',
                              color: 'var(--color-text-muted)',
                              border: '1px dashed var(--color-border-primary)',
                              padding: '2px 6px',
                              borderRadius: '4px',
                            }}
                          >
                            降级至: {m.default_model}
                          </span>
                        )}

                        {!m.is_enabled && (
                          <span
                            style={{
                              fontSize: '0.75rem',
                              color: 'var(--color-text-muted)',
                              background: 'var(--color-bg-input)',
                              padding: '2px 6px',
                              borderRadius: '4px',
                            }}
                          >
                            [已下线]
                          </span>
                        )}
                      </div>

                      {m.description && (
                        <div style={{ fontSize: '0.825rem', color: 'var(--color-text-secondary)', marginTop: '0.25rem' }}>
                          {m.description}
                        </div>
                      )}
                    </div>
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }} onClick={(e) => e.stopPropagation()}>
                    <div style={{ fontSize: '0.85rem', color: 'var(--color-text-muted)' }}>
                      挂载实例:{' '}
                      <span style={{ fontWeight: 600, color: activeCount > 0 ? 'var(--color-success)' : 'var(--color-danger)' }}>
                        {activeCount} / {backends.length}
                      </span>
                    </div>

                    <button
                      type="button"
                      className="btn btn-secondary"
                      onClick={(e) => handleOpenAddBackend(m, e)}
                      style={{ padding: '0.35rem 0.7rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
                    >
                      <Plus size={14} />
                      <span>添加实例</span>
                    </button>

                    <button
                      type="button"
                      className="btn btn-secondary"
                      onClick={(e) => handleOpenEditModel(m, e)}
                      style={{ padding: '0.35rem 0.7rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
                    >
                      <Edit2 size={13} />
                      <span>编辑</span>
                    </button>

                    <button
                      type="button"
                      className={m.is_enabled ? 'btn btn-secondary' : 'btn btn-primary'}
                      onClick={(e) => handleToggleModel(m, e)}
                      style={{ padding: '0.35rem 0.7rem', fontSize: '0.8rem' }}
                    >
                      {m.is_enabled ? '下线模型' : '上线模型'}
                    </button>

                    <button
                      type="button"
                      className="btn btn-danger"
                      onClick={(e) => handleDeleteModel(m, e)}
                      style={{ padding: '0.35rem 0.6rem', fontSize: '0.8rem' }}
                      title="删除模型"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>

                {/* 展开内容：物理后端实例列表 */}
                {isExpanded && (
                  <div style={{ padding: '1rem 1.25rem', background: 'var(--color-bg-surface)' }}>
                    {backends.length === 0 ? (
                      <div
                        style={{
                          textAlign: 'center',
                          padding: '1.5rem',
                          color: 'var(--color-text-muted)',
                          border: '1px dashed var(--color-border-primary)',
                          borderRadius: '6px',
                        }}
                      >
                        暂无挂载的物理后端实例，请点击上方“添加实例”接入推理节点
                      </div>
                    ) : (
                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(360px, 1fr))', gap: '0.85rem' }}>
                        {backends.map((b) => (
                          <div
                            key={b.id}
                            style={{
                              border: '1px solid var(--color-border-primary)',
                              borderRadius: '8px',
                              padding: '0.85rem',
                              background: 'var(--color-bg-muted)',
                              display: 'flex',
                              flexDirection: 'column',
                              gap: '0.5rem',
                              opacity: b.is_enabled ? 1 : 0.6,
                            }}
                          >
                            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                              <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                                {b.is_healthy && b.is_enabled ? (
                                  <CheckCircle size={16} color="var(--color-success)" />
                                ) : (
                                  <XCircle size={16} color="var(--color-danger)" />
                                )}
                                <strong style={{ fontSize: '0.95rem', color: 'var(--color-text-primary)' }}>{b.name}</strong>
                              </div>

                              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                                <button
                                  type="button"
                                  className="btn btn-secondary"
                                  onClick={() => handleToggleBackend(b)}
                                  style={{ padding: '0.2rem 0.5rem', fontSize: '0.75rem' }}
                                >
                                  {b.is_enabled ? '禁用' : '启用'}
                                </button>
                                <button
                                  type="button"
                                  className="btn btn-danger"
                                  onClick={() => handleDeleteBackend(b)}
                                  style={{ padding: '0.2rem 0.45rem', fontSize: '0.75rem' }}
                                >
                                  <Trash2 size={12} />
                                </button>
                              </div>
                            </div>

                            <div style={{ fontSize: '0.825rem', color: 'var(--color-text-secondary)', wordBreak: 'break-all' }}>
                              <code>{b.base_url}</code>
                            </div>

                            {/* 状态徽标与调度参数 */}
                            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap', fontSize: '0.75rem' }}>
                              <span style={{ color: 'var(--color-text-muted)' }}>协议:</span>
                              <span
                                style={{
                                  padding: '1px 5px',
                                  borderRadius: '3px',
                                  fontWeight: 600,
                                  background: 'var(--color-success-subtle, rgba(16, 185, 129, 0.1))',
                                  color: 'var(--color-success)',
                                }}
                              >
                                chat
                              </span>
                              {b.supports_responses && (
                                <span
                                  style={{
                                    padding: '1px 5px',
                                    borderRadius: '3px',
                                    fontWeight: 600,
                                    background: 'var(--color-primary-subtle, rgba(59, 130, 246, 0.1))',
                                    color: 'var(--color-primary)',
                                  }}
                                >
                                  responses
                                </span>
                              )}

                              <span style={{ color: 'var(--color-border-primary)', margin: '0 2px' }}>|</span>

                              <span style={{ color: 'var(--color-text-muted)' }}>权重: {b.weight}</span>
                              <span style={{ color: 'var(--color-text-muted)' }}>最大并发: {b.max_concurrency || b.max_connections || 50}</span>

                              {b.latency_ms > 0 && (
                                <span style={{ color: 'var(--color-text-muted)', display: 'flex', alignItems: 'center', gap: '2px' }}>
                                  <Zap size={11} color="var(--color-warning)" />
                                  {b.latency_ms}ms
                                </span>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* 1. 新建/编辑逻辑模型 Drawer */}
      <Drawer
        open={modelDrawerOpen}
        onClose={() => setModelDrawerOpen(false)}
        title={editingModel ? `编辑逻辑模型: ${editingModel.name}` : '接入新逻辑模型'}
        width="560px"
      >
        <form onSubmit={handleSaveModel} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              模型统一标识 Name <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={modelForm.name}
              onChange={(e) => setModelForm({ ...modelForm, name: e.target.value })}
              placeholder="例如: deepseek-v3, claude-3-7-sonnet"
              required
            />
            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
              客户端通过 `/v1/chat/completions` 请求时指定的模型名称{editingModel ? '（修改后客户端请求需使用新标识）' : ''}
            </span>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              模型用途说明 Description
            </label>
            <input
              type="text"
              className="gate-input"
              value={modelForm.description}
              onChange={(e) => setModelForm({ ...modelForm, description: e.target.value })}
              placeholder="如：旗舰代码推理模型、轻量摘要补全模型"
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                算力倍率乘数 Multiplier <span style={{ color: 'var(--color-danger)' }}>*</span>
              </label>
              <input
                type="number"
                step="0.1"
                min="0.1"
                className="gate-input"
                value={modelForm.multiplier}
                onChange={(e) => setModelForm({ ...modelForm, multiplier: parseFloat(e.target.value) || 1.0 })}
                required
              />
              <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
                基准为 1.0，旗舰模型建议 5~10x，轻量 0.5x
              </span>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                保底降级模型 DefaultModel
              </label>
              <select
                className="gate-input"
                value={modelForm.default_model}
                onChange={(e) => setModelForm({ ...modelForm, default_model: e.target.value })}
              >
                <option value="">(无保底降级)</option>
                {models
                  .filter((m) => !editingModel || m.id !== editingModel.id)
                  .map((m) => (
                    <option key={m.id} value={m.name}>
                      {m.name}
                    </option>
                  ))}
              </select>
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              透明参数注入 ModelParams (JSON)
            </label>
            <textarea
              className="gate-input"
              rows={4}
              value={modelForm.model_params_str}
              onChange={(e) => setModelForm({ ...modelForm, model_params_str: e.target.value })}
              style={{ fontFamily: 'monospace', fontSize: '0.85rem' }}
            />
            <span style={{ fontSize: '0.75rem', color: 'var(--color-text-muted)' }}>
              网关转发时自动注入该模型的参数，例如：{`{ "enable_thinking": false }`}
            </span>
          </div>

          {!editingModel && (
            <div style={{ borderTop: '1px dashed var(--color-border-primary)', paddingTop: '1.25rem' }}>
              <div style={{ fontSize: '0.9rem', fontWeight: 600, marginBottom: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <Server size={16} color="var(--color-primary)" />
                <span>快捷挂载首个物理实例 (可选)</span>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--color-text-secondary)', marginBottom: '0.25rem' }}>
                    上游根路径 BaseURL
                  </label>
                  <input
                    type="text"
                    className="gate-input"
                    value={modelForm.initial_base_url}
                    onChange={(e) => setModelForm({ ...modelForm, initial_base_url: e.target.value })}
                    placeholder="https://api.deepseek.com"
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--color-text-secondary)', marginBottom: '0.25rem' }}>
                    上游 API Key (可选脱敏保存)
                  </label>
                  <input
                    type="password"
                    className="gate-input"
                    value={modelForm.initial_api_key}
                    onChange={(e) => setModelForm({ ...modelForm, initial_api_key: e.target.value })}
                    placeholder="sk-..."
                  />
                </div>

                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--color-text-secondary)', marginBottom: '0.25rem' }}>
                      权重 Weight
                    </label>
                    <input
                      type="number"
                      className="gate-input"
                      value={modelForm.initial_weight}
                      onChange={(e) => setModelForm({ ...modelForm, initial_weight: parseInt(e.target.value, 10) || 10 })}
                    />
                  </div>
                  <div>
                    <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--color-text-secondary)', marginBottom: '0.25rem' }}>
                      最大并发 MaxConcurrency
                    </label>
                    <input
                      type="number"
                      className="gate-input"
                      value={modelForm.initial_max_concurrency}
                      onChange={(e) => setModelForm({ ...modelForm, initial_max_concurrency: parseInt(e.target.value, 10) || 50 })}
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
            <button type="button" className="btn btn-secondary" onClick={() => setModelDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={savingModel}>
              {savingModel ? '正在保存...' : '确认提交'}
            </button>
          </div>
        </form>
      </Drawer>

      {/* 2. 批量从网关自动导入 Drawer */}
      <Drawer
        open={importDrawerOpen}
        onClose={() => setImportDrawerOpen(false)}
        title="上游网关批量自动导入 (Gateway Auto-Import)"
        width="520px"
      >
        <form onSubmit={handleImportGateway} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <p style={{ fontSize: '0.875rem', color: 'var(--color-text-secondary)', margin: 0 }}>
            系统将调用上游供应商的 <code>/v1/models</code> 接口，批量读取并自动幂等建立对应的逻辑模型与后端物理实例。
          </p>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              实例命名前缀 Prefix <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={importForm.prefix}
              onChange={(e) => setImportForm({ ...importForm, prefix: e.target.value })}
              placeholder="例如: deepseek, vllm, azure"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              上游 BaseURL <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={importForm.base_url}
              onChange={(e) => setImportForm({ ...importForm, base_url: e.target.value })}
              placeholder="例如: https://api.deepseek.com 或 http://192.168.56.18:8000"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              上游鉴权 API Key (可选)
            </label>
            <input
              type="password"
              className="gate-input"
              value={importForm.api_key}
              onChange={(e) => setImportForm({ ...importForm, api_key: e.target.value })}
              placeholder="sk-..."
            />
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
            <button type="button" className="btn btn-secondary" onClick={() => setImportDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={importing}>
              {importing ? '正在连接上游导入...' : '开始自动拉取并导入'}
            </button>
          </div>
        </form>
      </Drawer>

      {/* 3. 为模型添加物理后端 Drawer */}
      <Drawer
        open={backendDrawerOpen}
        onClose={() => setBackendDrawerOpen(false)}
        title={targetModelForBackend ? `为 [${targetModelForBackend.name}] 接入物理后端` : '接入物理实例'}
        width="520px"
      >
        <form onSubmit={handleSaveBackend} style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              实例名称 Name <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={backendForm.name}
              onChange={(e) => setBackendForm({ ...backendForm, name: e.target.value })}
              placeholder="例如: node-aliyun-01"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              接口根地址 BaseURL <span style={{ color: 'var(--color-danger)' }}>*</span>
            </label>
            <input
              type="text"
              className="gate-input"
              value={backendForm.base_url}
              onChange={(e) => setBackendForm({ ...backendForm, base_url: e.target.value })}
              placeholder="http://192.168.56.18:8000"
              required
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
              节点调用 API Key (可选)
            </label>
            <input
              type="password"
              className="gate-input"
              value={backendForm.api_key}
              onChange={(e) => setBackendForm({ ...backendForm, api_key: e.target.value })}
              placeholder="sk-..."
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                负载权重 Weight
              </label>
              <input
                type="number"
                className="gate-input"
                value={backendForm.weight}
                onChange={(e) => setBackendForm({ ...backendForm, weight: parseInt(e.target.value, 10) || 10 })}
              />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: '0.875rem', fontWeight: 600, marginBottom: '0.4rem' }}>
                最大连接并发
              </label>
              <input
                type="number"
                className="gate-input"
                value={backendForm.max_concurrency}
                onChange={(e) => setBackendForm({ ...backendForm, max_concurrency: parseInt(e.target.value, 10) || 50 })}
              />
            </div>
          </div>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem', marginTop: '1rem' }}>
            <button type="button" className="btn btn-secondary" onClick={() => setBackendDrawerOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn btn-primary" disabled={savingBackend}>
              {savingBackend ? '正在保存...' : '添加后端实例'}
            </button>
          </div>
        </form>
      </Drawer>
    </div>
  )
}
