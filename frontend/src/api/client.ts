import { UserProfile, APIKeyItem, ModelItem, BackendItem, QuotaPolicyItem, UserQuotaDTO, AccessLogItem } from '../types'

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

// 获取个人配额资产
export async function fetchUserProfile(): Promise<UserProfile> {
  const res = await apiRequest<{ data: UserProfile }>(`${getBaseApiPrefix()}/user/profile`)
  return res.data
}

// 获取 API Keys
export async function fetchUserKeys(): Promise<APIKeyItem[]> {
  const res = await apiRequest<{ data: APIKeyItem[] }>(`${getBaseApiPrefix()}/user/keys`)
  return res.data
}

// 创建 API Key
export async function createAPIKey(name: string, expiresIn: number): Promise<APIKeyItem & { raw_key: string }> {
  const res = await apiRequest<{ data: APIKeyItem & { raw_key: string } }>(`${getBaseApiPrefix()}/user/keys`, {
    method: 'POST',
    body: JSON.stringify({ name, expires_in: expiresIn }),
  })
  return res.data
}

// 删除 API Key
export async function deleteAPIKey(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/user/keys/${id}`, {
    method: 'DELETE',
  })
}

// 获取模型列表
export async function fetchModels(): Promise<ModelItem[]> {
  const res = await apiRequest<{ data: ModelItem[] }>(`${getBaseApiPrefix()}/models`)
  return res.data
}

// 管理员：获取用户列表
export async function fetchAdminUsers(page = 1, pageSize = 25, search = ''): Promise<{ data: UserQuotaDTO[]; total: number }> {
  const params = new URLSearchParams({
    page: String(page),
    pageSize: String(pageSize),
  })
  if (search) params.append('search', search)
  return apiRequest(`${getBaseApiPrefix()}/admin/users?${params.toString()}`)
}

// 管理员：调整用户配额
export async function updateUserQuota(
  userId: number,
  payload: { role: string; custom_daily_credits?: number; custom_weekly_credits?: number }
): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/users/${userId}/quota`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

// 管理员：查询策略
export async function fetchAdminPolicies(): Promise<QuotaPolicyItem[]> {
  const res = await apiRequest<{ data: QuotaPolicyItem[] }>(`${getBaseApiPrefix()}/admin/policies`)
  return res.data
}

// 管理员：查询后端
export async function fetchAdminBackends(): Promise<BackendItem[]> {
  const res = await apiRequest<{ data: BackendItem[] }>(`${getBaseApiPrefix()}/admin/backends`)
  return res.data
}

// 管理员：保存后端
export async function saveAdminBackend(backend: Partial<BackendItem>): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/backends`, {
    method: 'POST',
    body: JSON.stringify(backend),
  })
}

// 管理员：删除后端
export async function deleteAdminBackend(id: number): Promise<void> {
  await apiRequest(`${getBaseApiPrefix()}/admin/backends/${id}`, {
    method: 'DELETE',
  })
}

// 管理员：查询日志
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
