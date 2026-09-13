// 前端数据类型定义

export interface UserProfile {
  user_id: number
  policy_id?: number
  policy_name: string
  is_custom: boolean
  is_admin?: boolean
  rpm_limit: number
  daily_limit_credits: number
  weekly_limit_credits: number
  daily_used_credits: number
  weekly_used_credits: number
  daily_remaining_credits: number
  weekly_remaining_credits: number
}

export interface APIKeyItem {
  id: number
  user_id: number
  name: string
  key_prefix: string
  raw_key?: string
  allowed_models: string[]
  expires_at?: string
  is_active: boolean
  last_used_at?: string
  created_at: string
}

export interface ModelItem {
  id: string
  object: string
  owned_by: string
  cost_multiplier: number
  input_rate: number
  cache_hit_rate: number
  output_rate: number
  created?: number
}

export interface BackendItem {
  id: number
  model_id: number
  name: string
  base_url: string
  api_key?: string
  weight: number
  max_concurrency?: number
  max_connections?: number
  active_connections: number
  is_healthy: boolean
  is_enabled: boolean
  last_check_at?: string
  latency_ms: number
  consecutive_failures: number
  declared_protocols?: string[]
  detected_protocols?: string[]
  supports_chat?: boolean
  supports_responses?: boolean
}

export interface ModelDetailItem {
  id: number
  name: string
  description: string
  multiplier: number
  default_model: string
  model_params: Record<string, any>
  is_enabled: boolean
  backends?: BackendItem[]
  backends_count?: number
  active_backends_count?: number
  created_at: string
  updated_at: string
}

export interface QuotaPolicyItem {
  id: number
  name: string
  description?: string
  daily_credits_limit: number
  weekly_credits_limit: number
  rate_limit_rpm: number
  time_ranges?: Array<{ start: string; end: string }> | string
  model_whitelist?: string[] | string
  default_model?: string
  user_count?: number
  created_at?: string
}

export interface UserQuotaDTO {
  user_id: number
  username: string
  name: string
  email: string
  department?: string
  policy_id?: number
  policy_name: string
  is_custom: boolean
  daily_limit: number
  weekly_limit: number
  daily_consumed: number
  weekly_consumed: number
  custom_daily_credits?: number
  custom_weekly_credits?: number
}

export interface AccessLogItem {
  id: number
  user_id: number
  api_key_id?: number
  client_ip: string
  user_agent: string
  path: string
  protocol: string
  model: string
  input_tokens: number
  cache_hit_tokens: number
  output_tokens: number
  cost_credits: number
  duration_ms: number
  ttft_ms: number
  status_code: number
  error_message?: string
  created_at: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  reasoning_content?: string
  isThinking?: boolean
  costCredits?: number
  durationMs?: number
  ttftMs?: number
  tokens?: {
    input: number
    output: number
    cacheHit?: number
  }
}

export interface DashboardSummary {
  total_requests: number
  today_requests: number
  total_credits: number
  today_credits: number
  active_users_24h: number
  total_backends: number
  healthy_backends: number
  avg_latency_ms: number
}

export interface HourlyTrendItem {
  hour: string
  requests: number
  cost_credits: number
  errors: number
}

export interface TopModelItem {
  model: string
  count: number
  cost_credits: number
  percentage: number
}

export interface StatusCounts {
  status_2xx: number
  status_4xx: number
  status_5xx: number
}

export interface DashboardData {
  summary: DashboardSummary
  hourly_trends: HourlyTrendItem[]
  top_models: TopModelItem[]
  status_counts: StatusCounts
  recent_errors: AccessLogItem[]
}

// 4. 健康与实时并发大盘数据结构
export interface BackendHealthItem {
  id: number
  model_id: number
  model_name: string
  name: string
  base_url: string
  weight: number
  max_concurrency: number
  active_connections: number
  utilization_ratio: number
  is_healthy: boolean
  is_enabled: boolean
  latency_ms: number
  consecutive_failures: number
  last_check_at?: string
  declared_protocols?: string[]
  detected_protocols?: string[]
}

export interface HealthMatrixData {
  summary: {
    total_backends: number
    healthy_backends: number
    fault_backends: number
    avg_latency_ms: number
  }
  backends: BackendHealthItem[]
}

// 5. 系统运行时配置
export interface SystemConfigData {
  blocked_user_agents: string[]
  read_timeout: string
  write_timeout: string
  idle_timeout: string
  max_header_bytes: number
}

// 6. 7天算力逐日交叉透视矩阵
export interface DailyConsumerMetric {
  date: string
  requests: number
  input_tokens: number
  output_tokens: number
  cost_credits: number
}

export interface UserTopConsumerRow {
  user_id: number
  name: string
  username: string
  email: string
  department: string
  daily: Record<string, DailyConsumerMetric>
  total_req: number
  total_tokens: number
  total_credits: number
}

export interface GrandTotalRow {
  daily: Record<string, DailyConsumerMetric>
  total_req: number
  total_tokens: number
  total_credits: number
}

export interface TopConsumersData {
  dates: string[]
  users: UserTopConsumerRow[]
  grand_total: GrandTotalRow
}

// 7. 全局时段算力倍率
export interface TimeMultiplierRule {
  id: string
  name: string
  days_of_week?: number[] // 1=周一, ..., 7=周日；若空表示每天
  start_time: string // "21:00"
  end_time: string // "09:00"
  multiplier: number // 0.2
  is_enabled: boolean
  description?: string
}

export interface TimeMultipliersData {
  current_multiplier: number
  matched_rule?: TimeMultiplierRule | null
  rules: TimeMultiplierRule[]
}
