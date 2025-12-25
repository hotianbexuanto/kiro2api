import { defineStore } from 'pinia'
import { ref } from 'vue'
import { keysApi } from '@/api/keys'
import type { APIKey } from '@/types'

export const useKeysStore = defineStore('keys', () => {
  const keys = ref<APIKey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      keys.value = await keysApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  async function create(data: Parameters<typeof keysApi.create>[0]) {
    const newKey = await keysApi.create(data)
    await fetch()
    return newKey
  }

  async function update(key: string, allowedGroups: string[]) {
    await keysApi.update(key, allowedGroups)
    await fetch()
  }

  async function remove(key: string) {
    await keysApi.delete(key)
    await fetch()
  }

  return {
    keys,
    loading,
    error,
    fetch,
    create,
    update,
    remove,
  }
})
