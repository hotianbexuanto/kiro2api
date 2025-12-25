import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { groupsApi } from '@/api/groups'
import type { Group } from '@/types'

export const useGroupsStore = defineStore('groups', () => {
  const groups = ref<Group[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  const groupNames = computed(() => groups.value.map(g => g.name))

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      const res = await groupsApi.list()
      groups.value = res.groups
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  async function create(name: string, displayName?: string) {
    await groupsApi.create(name, displayName)
    await fetch()
  }

  async function update(name: string, displayName?: string, settings?: Parameters<typeof groupsApi.update>[2]) {
    await groupsApi.update(name, displayName, settings)
    await fetch()
  }

  async function rename(oldName: string, newName: string) {
    await groupsApi.rename(oldName, newName)
    await fetch()
  }

  async function remove(name: string) {
    await groupsApi.delete(name)
    await fetch()
  }

  return {
    groups,
    loading,
    error,
    groupNames,
    fetch,
    create,
    update,
    rename,
    remove,
  }
})
