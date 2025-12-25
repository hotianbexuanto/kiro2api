import { http } from './client'
import type { Stats, RequestRecord, LogsResponse, LogStats } from '@/types'

export const statsApi = {
  get(): Promise<Stats> {
    return http.get('/api/stats')
  },

  getRecords(): Promise<RequestRecord[]> {
    return http.get('/api/stats/records')
  },

  // 持久化日志 API
  getLogs(params?: { page?: number; page_size?: number; model?: string; group?: string }): Promise<LogsResponse> {
    const query = new URLSearchParams()
    if (params?.page) query.set('page', String(params.page))
    if (params?.page_size) query.set('page_size', String(params.page_size))
    if (params?.model) query.set('model', params.model)
    if (params?.group) query.set('group', params.group)
    const qs = query.toString()
    return http.get(`/api/logs${qs ? '?' + qs : ''}`)
  },

  getLogStats(): Promise<LogStats> {
    return http.get('/api/logs/stats')
  },

  clearLogs(days?: number): Promise<{ deleted?: number; message: string }> {
    const qs = days !== undefined ? `?days=${days}` : ''
    return http.delete(`/api/logs${qs}`)
  },
}
