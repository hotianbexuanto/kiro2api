import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { tokensApi } from '@/api/tokens'
import type { TokenListResponse, TokenListParams } from '@/types'

export const useTokensStore = defineStore('tokens', () => {
  const data = ref<TokenListResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // 分页状态
  const page = ref(1)
  const pageSize = ref(100)
  const filterGroup = ref<string | undefined>(undefined)

  const tokens = computed(() => data.value?.tokens || [])
  const totalTokens = computed(() => data.value?.total_tokens || 0)
  const activeTokens = computed(() => data.value?.active_tokens || 0)
  const globalInFlight = computed(() => data.value?.pool_stats?.global_in_flight || 0)
  const tokensWithInFlight = computed(() => data.value?.pool_stats?.tokens_with_in_flight || 0)
  const isBackendLoading = computed(() => data.value?.loading === true)
  const totalPages = computed(() => Math.ceil(totalTokens.value / pageSize.value) || 1)

  async function fetch(params?: TokenListParams, retryCount = 0) {
    loading.value = true
    error.value = null

    // 合并参数
    const p = {
      page: params?.page ?? page.value,
      page_size: params?.page_size ?? pageSize.value,
      group: params?.group ?? filterGroup.value,
    }

    try {
      const response = await tokensApi.list(p)
      data.value = response

      // 更新分页状态
      page.value = response.page || 1
      pageSize.value = response.page_size || 100

      // 如果后端还在预热缓存，自动重试（最多3次，每次间隔2秒）
      if (response.loading && retryCount < 3) {
        setTimeout(() => fetch(p, retryCount + 1), 2000)
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  function setPage(p: number) {
    page.value = p
    fetch({ page: p })
  }

  function setPageSize(size: number) {
    pageSize.value = size
    page.value = 1
    fetch({ page: 1, page_size: size })
  }

  function setFilterGroup(group: string | undefined) {
    filterGroup.value = group
    page.value = 1
    fetch({ page: 1, group })
  }

  async function add(params: Parameters<typeof tokensApi.add>[0]) {
    await tokensApi.add(params)
    await fetch()
  }

  async function addBulk(tokens: Parameters<typeof tokensApi.add>[0][]) {
    const result = await tokensApi.addBulk(tokens)
    await fetch()
    return result
  }

  async function remove(id: number) {
    await tokensApi.delete(id)
    await fetch()
  }

  async function update(id: number, params: Parameters<typeof tokensApi.update>[1]) {
    await tokensApi.update(id, params)
    await fetch()
  }

  async function move(id: number, group: string) {
    await tokensApi.move(id, group)
    await fetch()
  }

  async function refresh(params?: Parameters<typeof tokensApi.refresh>[0]) {
    const result = await tokensApi.refresh(params)
    await fetch()
    return result
  }

  return {
    data,
    loading,
    error,
    tokens,
    totalTokens,
    activeTokens,
    globalInFlight,
    tokensWithInFlight,
    isBackendLoading,
    page,
    pageSize,
    totalPages,
    filterGroup,
    fetch,
    setPage,
    setPageSize,
    setFilterGroup,
    add,
    addBulk,
    remove,
    update,
    move,
    refresh,
  }
})
