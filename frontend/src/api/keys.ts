import { http } from './client'
import type { APIKey, CreateAPIKeyRequest } from '@/types'

export const keysApi = {
  list(): Promise<APIKey[]> {
    return http.get('/api/keys')
  },

  create(data: CreateAPIKeyRequest): Promise<APIKey> {
    return http.post('/api/keys', data)
  },

  update(key: string, allowedGroups: string[]): Promise<{ message: string }> {
    return http.patch(`/api/keys/${key}`, { allowed_groups: allowedGroups })
  },

  delete(key: string): Promise<void> {
    return http.delete(`/api/keys/${key}`)
  },
}
