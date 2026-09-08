// 前端数据类型定义

export interface UserProfile {
  user_id: number
  role: string
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
  name: string
  base_url: string
  weight: number
  max_connections: number
  active_connections: number
  is_healthy: boolean
  last_check_at: string
  latency_ms: number
  consecutive_failures: number
  supports_chat: boolean
  supports_responses: boolean
  capabilities?: {
    chat: boolean
    responses: boolean
  }
}

export interface QuotaPolicyItem {
  id: number
  name: string
  daily_credits_limit: number
  weekly_credits_limit: number
  rate_limit_rpm: number
  model_whitelist: string[]
}

export interface UserQuotaDTO {
  user_id: number
  username: string
  name: string
  email: string
  role: string
  policy_name: string
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
