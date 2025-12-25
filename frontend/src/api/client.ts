// 简单的 HTTP 客户端，基于 fetch

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Record<string, string>
}

interface ApiError {
  error: string
  duplicate?: boolean
  existing?: { id: number; name: string; group: string }
}

// 自定义错误类，保留 API 响应数据
export class ApiRequestError extends Error {
  status: number
  data: ApiError

  constructor(status: number, data: ApiError) {
    super(data.error || `HTTP ${status}`)
    this.name = 'ApiRequestError'
    this.status = status
    this.data = data
  }
}

class HttpClient {
  private baseUrl: string

  constructor(baseUrl = '') {
    this.baseUrl = baseUrl
  }

  private getAuthHeader(): Record<string, string> {
    const token = localStorage.getItem('api_token')
    if (token) {
      return { 'Authorization': `Bearer ${token}` }
    }
    return {}
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { method = 'GET', body, headers = {} } = options

    const config: RequestInit = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...this.getAuthHeader(),
        ...headers,
      },
    }

    if (body !== undefined) {
      config.body = JSON.stringify(body)
    }

    const response = await fetch(`${this.baseUrl}${path}`, config)

    // 204 No Content
    if (response.status === 204) {
      return undefined as T
    }

    const data = await response.json()

    if (!response.ok) {
      throw new ApiRequestError(response.status, data as ApiError)
    }

    return data as T
  }

  get<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'GET' })
  }

  post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, { method: 'POST', body })
  }

  put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, { method: 'PUT', body })
  }

  patch<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, { method: 'PATCH', body })
  }

  delete<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'DELETE' })
  }
}

export const http = new HttpClient()
