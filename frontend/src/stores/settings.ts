import { defineStore } from 'pinia'
import { ref } from 'vue'
import { settingsApi } from '@/api/settings'
import type { Settings, RateLimiterStats } from '@/types'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings | null>(null)
  const rateLimiter = ref<RateLimiterStats | null>(null)
  const activeTokens = ref<number>(0)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      const res = await settingsApi.get()
      settings.value = res.settings
      rateLimiter.value = res.rate_limiter
      activeTokens.value = res.active_tokens || 0
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  async function update(data: Partial<Settings>) {
    const res = await settingsApi.update(data)
    settings.value = res.settings
  }

  return {
    settings,
    rateLimiter,
    activeTokens,
    loading,
    error,
    fetch,
    update,
  }
})
