import { http } from './client'
import type { Settings, SettingsResponse } from '@/types'

export const settingsApi = {
  get(): Promise<SettingsResponse> {
    return http.get('/api/settings')
  },

  update(data: Partial<Settings>): Promise<{ message: string; settings: Settings }> {
    return http.post('/api/settings', data)
  },
}
