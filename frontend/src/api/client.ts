import {
  UserProfile,
  APIKeyItem,
  ModelItem,
  BackendItem,
  ModelDetailItem,
  QuotaPolicyItem,
  UserQuotaDTO,
  AccessLogItem,
  DashboardData,
  HealthMatrixData,
  SystemConfigData,
  TopConsumersData,
} from '../types'

export const getBaseApiPrefix = (): string => {
  if (typeof window !== 'undefined') {
    if ((window as any).__POWERED_BY_PORTAL__ || window.location.pathname.startsWith('/gate')) {
      return '/gate/v1'
    }
  }
  return '/v1'
}

export const getAuthToken = (): string => {
  if (typeof window === 'undefined') return ''
  let token =
    localStorage.getItem('code_shield_token') ||
    localStorage.getItem('token') ||
    localStorage.getItem('gate_api_key') ||
    ''
  if (!token && typeof document !== 'undefined') {
    const match = document.cookie.match(/(?:^|;\s*)code_shield_token=([^;]*)/)
    if (match) {
      token = decodeURIComponent(match[1])
    }
  }
  return token
}

export async function apiRequest<T>(url: string, options: RequestInit = {}): Promise<T> {
  const token = getAuthToken()
  const headers = new Headers(options.headers || {})
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', token.startsWith('Bearer ') || token.startsWith('sk-') ? token : `Bearer ${token}`)
  }
  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const res = await fetch(url, {
    ...options,
    headers,
  })

  if (!res.ok) {
    let errorMsg = `HTTP 错误: ${res.status}`
    try {
      const errJson = await res.json()
      if (errJson?.error?.message) {
        errorMsg = errJson.error.message
      } else if (errJson?.error) {
        errorMsg = typeof errJson.error === 'string' ? errJson.error : JSON.stringify(errJson.error)
      } else if (errJson?.message) {
        errorMsg = errJson.message
      }
    } catch {
      // ignore json parse error
    }
    throw new Error(errorMsg)
  }

  return res.json()
}

// 1. 用户资产与密钥
export async function fetchUserProfile(): Promise<UserProfile> {
  const res = await apiRequest<{ data: UserProfile }>(`${getBaseApiPrefix()}/user/profile`)
  return res.data
}

export async function fetchUserKeys(): Promise<APIKeyItem[]> {
  const res = await apiRequest<{ data: APIKeyItem[] }>(`${getBaseApiPrefix()}/user/keys`)
  return res.data
}

export async function createAPIKey(name: string, expiresIn: number): Promise<APIKeyItem & { raw_key: string }> {
  const res = await apiRequest<{ data: APIKeyItem & { raw_key: string } }>(`${getBaseApiPrefix()}/user/keys`, {
    method: 'POST',
    body: JSON.stringify({ name, expires_in: expiresIn }),
  })
  return res.data
}

export async function deleteAPIKey(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/user/keys/${id}`, {
    method: 'DELETE',
  })
}

export async function fetchModels(): Promise<ModelItem[]> {
  const res = await apiRequest<{ data: ModelItem[] }>(`${getBaseApiPrefix()}/models`)
  return res.data
}

// 2. 用户管理
export async function fetchAdminUsers(page = 1, pageSize = 25, search = ''): Promise<{ data: UserQuotaDTO[]; total: number }> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  if (search) params.append('search', search)
  return apiRequest(`${getBaseApiPrefix()}/admin/users?${params.toString()}`)
}

export async function updateUserQuota(
  userId: number,
  payload: { role: string; policy_id?: number; custom_daily_credits?: number; custom_weekly_credits?: number }
): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/users/${userId}/quota`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

// 3. 配额策略池
export async function fetchAdminPolicies(): Promise<QuotaPolicyItem[]> {
  const res = await apiRequest<{ data: QuotaPolicyItem[] }>(`${getBaseApiPrefix()}/admin/policies`)
  return res.data
}

export async function saveAdminPolicy(policy: Partial<QuotaPolicyItem>): Promise<QuotaPolicyItem> {
  const res = await apiRequest<{ data: QuotaPolicyItem }>(`${getBaseApiPrefix()}/admin/policies`, {
    method: 'POST',
    body: JSON.stringify(policy),
  })
  return res.data
}

export async function deleteAdminPolicy(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/policies/${id}`, {
    method: 'DELETE',
  })
}

// 4. 逻辑模型 1:N 治理与网关批量导入
export async function fetchAdminModels(): Promise<ModelDetailItem[]> {
  const res = await apiRequest<{ data: ModelDetailItem[] }>(`${getBaseApiPrefix()}/admin/models`)
  return res.data
}

export async function saveAdminModel(model: Partial<ModelDetailItem> & { initial_backend?: any }): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/models`, {
    method: 'POST',
    body: JSON.stringify(model),
  })
}

export async function toggleAdminModel(id: number): Promise<{ is_enabled: boolean }> {
  return apiRequest(`${getBaseApiPrefix()}/admin/models/${id}/toggle`, {
    method: 'PATCH',
  })
}

export async function deleteAdminModel(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/models/${id}`, {
    method: 'DELETE',
  })
}

export async function importGatewayModels(req: { prefix: string; base_url: string; api_key?: string }): Promise<{ message: string; imported_count: number }> {
  return apiRequest(`${getBaseApiPrefix()}/admin/models/import`, {
    method: 'POST',
    body: JSON.stringify(req),
  })
}

// 5. 物理后端实例
export async function fetchAdminBackends(): Promise<BackendItem[]> {
  const res = await apiRequest<{ data: BackendItem[] }>(`${getBaseApiPrefix()}/admin/backends`)
  return res.data
}

export async function saveAdminBackend(backend: Partial<BackendItem>): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/backends`, {
    method: 'POST',
    body: JSON.stringify(backend),
  })
}

export async function toggleAdminBackend(id: number): Promise<{ is_enabled: boolean }> {
  return apiRequest(`${getBaseApiPrefix()}/admin/backends/${id}/toggle`, {
    method: 'PATCH',
  })
}

export async function deleteAdminBackend(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/backends/${id}`, {
    method: 'DELETE',
  })
}

// 6. 全景健康与细粒度并发实时水位大盘
export async function fetchAdminHealth(): Promise<HealthMatrixData> {
  const res = await apiRequest<{ data: HealthMatrixData }>(`${getBaseApiPrefix()}/admin/health`)
  return res.data
}

export async function triggerAdminProbe(): Promise<{ message: string }> {
  return apiRequest(`${getBaseApiPrefix()}/admin/health/probe`, {
    method: 'POST',
  })
}

// 7. 系统运行时配置与动态客户端过滤热重载
export async function fetchAdminSystemConfig(): Promise<SystemConfigData> {
  const res = await apiRequest<{ data: SystemConfigData }>(`${getBaseApiPrefix()}/admin/config/system`)
  return res.data
}

export async function updateAdminSystemConfig(payload: { blocked_user_agents: string[] }): Promise<{ message: string }> {
  return apiRequest(`${getBaseApiPrefix()}/admin/config/system`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

// 8. 7天 TOP 算力消费者交叉透视矩阵大账
export async function fetchAdminTopConsumers(): Promise<TopConsumersData> {
  const res = await apiRequest<{ data: TopConsumersData }>(`${getBaseApiPrefix()}/admin/top-consumers`)
  return res.data
}

// 9. 审计日志与监控大屏
export async function fetchAdminLogs(
  page = 1,
  pageSize = 25,
  model = '',
  statusCode = ''
): Promise<{ data: AccessLogItem[]; total: number }> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  if (model) params.append('model', model)
  if (statusCode) params.append('statusCode', statusCode)
  return apiRequest(`${getBaseApiPrefix()}/admin/logs?${params.toString()}`)
}

export async function fetchAdminDashboard(): Promise<DashboardData> {
  const res = await apiRequest<{ data: DashboardData }>(`${getBaseApiPrefix()}/admin/dashboard`)
  return res.data
}
