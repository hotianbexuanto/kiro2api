// Token 相关类型
export interface UsageLimits {
  total_limit: number
  current_usage: number
  is_exceeded: boolean
}

export interface Token {
  index: number
  user_email: string
  token_preview: string
  auth_type: 'social' | 'idc'
  remaining_usage: number
  expires_at: string
  last_used: string
  status: 'active' | 'disabled' | 'error' | 'banned' | 'exhausted'
  error?: string
  group: string
  name: string
  client_id?: string
  usage_limits?: UsageLimits
  // 运行时统计
  request_count: number
  success_count: number
  failure_count: number
  in_flight: number
  avg_latency: number
}

export interface TokenListResponse {
  timestamp: string
  total_tokens: number
  active_tokens: number
  tokens: Token[]
  loading?: boolean // 后端缓存预热中
  pool_stats: {
    total_tokens: number
    active_tokens: number
    global_in_flight: number
    tokens_with_in_flight: number
  }
  // 分页
  page: number
  page_size: number
}

export interface TokenListParams {
  page?: number
  page_size?: number
  group?: string
}

export interface AddTokenRequest {
  refreshToken: string
  auth?: 'Social' | 'IdC'
  clientId?: string
  clientSecret?: string
  group?: string
  name?: string
}

export interface UpdateTokenRequest {
  disabled?: boolean
  group?: string
  name?: string
}

export interface RefreshTokensRequest {
  group?: string
  status?: 'banned' | 'exhausted' | 'active' | ''
}

export interface RefreshTokenResult {
  id: number
  index?: number  // 兼容旧接口
  group: string
  name: string
  status: string
  user_email?: string
  remaining_usage?: number
  error?: string
}

export interface RefreshTokensResponse {
  refreshed: number
  failed: number
  total: number
  concurrency: number
  results: RefreshTokenResult[]
  message: string
}

// 分组相关类型
export interface GroupSettings {
  priority?: number
  disabled?: boolean
}

export interface Group {
  name: string
  display_name: string
  settings: GroupSettings
  token_count: number
  active_count: number
}

export interface GroupListResponse {
  groups: Group[]
}

// 设置相关类型
export interface Settings {
  rate_limit_qps: number
  rate_limit_burst: number
  request_timeout_sec: number
  max_retries: number
  cooldown_sec: number
  token_rate_limit_qps: number
  token_rate_limit_burst: number
  token_max_concurrent: number
  group_max_concurrent: number
  refresh_concurrency: number
  session_duration_min: number
}

export interface RateLimiterStats {
  qps: number
  burst: number
  available_tokens: number
}

export interface SettingsResponse {
  settings: Settings
  rate_limiter: RateLimiterStats
  active_tokens: number
  global_in_flight: number
  tokens_with_in_flight: number
}

// API Key 相关类型
export interface APIKey {
  key: string
  masked_key: string
  name: string
  allowed_groups: string[]
}

export interface CreateAPIKeyRequest {
  key?: string
  name?: string
  allowed_groups?: string[]
}

// 统计相关类型
export interface RequestRecord {
  id: string
  timestamp: string
  method: string
  path: string
  request_type?: string
  model: string
  stream: boolean
  status_code: number
  latency: number
  ttfb?: number  // 首字时间 (ms)，仅流式请求
  input_tokens: number
  output_tokens: number
  cache_read_input_tokens?: number
  cache_creation_input_tokens?: number
  credit_usage?: number
  context_usage_percent?: number
  token_index: number
  group: string
  error?: string
}

// 持久化日志记录（来自数据库）
export interface LogRecord {
  id: number
  request_id: string
  timestamp: string
  model: string
  stream: boolean
  status_code: number
  latency_ms: number
  actual_input_tokens: number
  calculated_output_tokens: number
  credit_usage: number
  context_usage_percent: number
  cache_hit: boolean
  token_index: number
  conversation_id: string
  group: string
}

export interface LogsResponse {
  total: number
  page: number
  pages: number
  records: LogRecord[]
}

export interface LogStats {
  total_records: number
  db_size_mb: number
  oldest_record?: string
  newest_record?: string
  total_credit_usage: number
}

export interface Stats {
  total_requests: number
  success_requests: number
  failed_requests: number
  avg_latency: number
  model_usage: Record<string, number>
  path_usage: Record<string, number>
  hourly_requests: number[]
  hourly_by_model: Record<string, number[]>
  hourly_credit: number[]
  recent_records: RequestRecord[]
}
