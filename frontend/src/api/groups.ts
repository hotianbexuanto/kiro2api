import { http } from './client'
import type { GroupListResponse, GroupSettings } from '@/types'

export const groupsApi = {
  list(): Promise<GroupListResponse> {
    return http.get('/api/groups')
  },

  create(name: string, displayName?: string): Promise<{ message: string }> {
    return http.post('/api/groups', { name, display_name: displayName })
  },

  update(name: string, displayName?: string, settings?: GroupSettings): Promise<{ message: string }> {
    return http.put(`/api/groups/${name}`, { display_name: displayName, settings })
  },

  rename(oldName: string, newName: string): Promise<{ message: string }> {
    return http.post(`/api/groups/${oldName}/rename`, { new_name: newName })
  },

  delete(name: string): Promise<void> {
    return http.delete(`/api/groups/${name}`)
  },
}
