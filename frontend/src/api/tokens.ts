import { http } from './client'
import type {
  TokenListResponse,
  TokenListParams,
  AddTokenRequest,
  UpdateTokenRequest,
  RefreshTokensRequest,
  RefreshTokensResponse,
} from '@/types'

export const tokensApi = {
  list(params?: TokenListParams): Promise<TokenListResponse> {
    const query = new URLSearchParams()
    if (params?.page) query.set('page', String(params.page))
    if (params?.page_size) query.set('page_size', String(params.page_size))
    if (params?.group) query.set('group', params.group)
    const qs = query.toString()
    return http.get('/api/tokens' + (qs ? `?${qs}` : ''))
  },

  add(data: AddTokenRequest): Promise<{ message: string }> {
    return http.post('/api/tokens', data)
  },

  addBulk(tokens: AddTokenRequest[]): Promise<{ added: number; duplicates: number; skipped: unknown[]; message: string }> {
    return http.post('/api/tokens/bulk', { tokens })
  },

  delete(id: number): Promise<void> {
    return http.delete(`/api/tokens/${id}`)
  },

  update(id: number, data: UpdateTokenRequest): Promise<{ message: string }> {
    return http.patch(`/api/tokens/${id}`, data)
  },

  move(id: number, group: string): Promise<{ message: string }> {
    return http.put(`/api/tokens/${id}/move`, { group })
  },

  refresh(params?: RefreshTokensRequest): Promise<RefreshTokensResponse> {
    return http.post('/api/tokens/refresh', params || {})
  },
}
