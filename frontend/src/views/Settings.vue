<template>
  <div>
    <h1 class="text-xl font-semibold text-gray-800 mb-6">全局设置</h1>

    <div v-if="store.loading" class="text-center py-12 text-gray-400">
      加载中...
    </div>
    <div v-else-if="store.settings" class="space-y-6">
      <!-- Token 状态 -->
      <div class="card p-6">
        <h2 class="text-base font-medium text-gray-800 mb-4">Token 状态</h2>
        <div class="flex items-center gap-6">
          <div>
            <span class="text-sm text-gray-500">可用 Token:</span>
            <span class="ml-2 text-2xl font-semibold text-green-600">{{ store.activeTokens }}</span>
          </div>
          <div class="text-sm text-gray-400">
            推荐限流: QPS={{ recommendedQPS }}, Burst={{ recommendedBurst }}
          </div>
          <button
            type="button"
            class="px-3 py-1.5 text-xs font-medium text-blue-600 bg-blue-50 rounded-lg hover:bg-blue-100 transition-all"
            @click="applyRecommended"
          >
            应用推荐值
          </button>
        </div>
      </div>

      <!-- 限流设置 -->
      <div class="card p-6">
        <h2 class="text-base font-medium text-gray-800 mb-4">全局限流</h2>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">QPS</label>
            <input
              v-model.number="form.rate_limit_qps"
              type="number"
              step="0.1"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">每秒请求数限制</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">Burst</label>
            <input
              v-model.number="form.rate_limit_burst"
              type="number"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">突发请求数</p>
          </div>
        </div>
      </div>

      <!-- 请求设置 -->
      <div class="card p-6">
        <h2 class="text-base font-medium text-gray-800 mb-4">请求设置</h2>
        <div class="grid grid-cols-4 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">超时时间 (秒)</label>
            <input
              v-model.number="form.request_timeout_sec"
              type="number"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">最大重试次数</label>
            <input
              v-model.number="form.max_retries"
              type="number"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">冷却时间 (秒)</label>
            <input
              v-model.number="form.cooldown_sec"
              type="number"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">刷新并发数</label>
            <input
              v-model.number="form.refresh_concurrency"
              type="number"
              min="1"
              max="50"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">同时刷新的 Token 数</p>
          </div>
        </div>
      </div>

      <!-- Token 并发控制 -->
      <div class="card p-6">
        <h2 class="text-base font-medium text-gray-800 mb-4">Token 并发控制</h2>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">单 Token 最大并发</label>
            <input
              v-model.number="form.token_max_concurrent"
              type="number"
              min="0"
              max="10"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">0=不限制，推荐值: 2</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">单 Token QPS 限制</label>
            <input
              v-model.number="form.token_rate_limit_qps"
              type="number"
              min="0"
              step="0.1"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">0=不限制</p>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">单 Token Burst</label>
            <input
              v-model.number="form.token_rate_limit_burst"
              type="number"
              min="0"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
            <p class="text-xs text-gray-400 mt-1.5">0=不限制</p>
          </div>
        </div>
      </div>

      <div class="flex justify-end">
        <button
          class="px-6 py-2.5 text-sm font-medium text-white btn-primary rounded-lg disabled:opacity-50"
          :disabled="saving"
          @click="handleSave"
        >
          {{ saving ? '保存中...' : '保存设置' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { useToast } from '@/composables/useToast'
import type { Settings } from '@/types'

const store = useSettingsStore()
const toast = useToast()

const form = ref<Settings>({
  rate_limit_qps: 10,
  rate_limit_burst: 20,
  request_timeout_sec: 120,
  max_retries: 2,
  cooldown_sec: 30,
  token_rate_limit_qps: 0,
  token_rate_limit_burst: 0,
  token_max_concurrent: 0,
  group_max_concurrent: 0,
  refresh_concurrency: 20,
})

const saving = ref(false)

// 1 token = 2 并发
const recommendedQPS = computed(() => Math.max(1, store.activeTokens * 2))
const recommendedBurst = computed(() => Math.max(2, store.activeTokens * 4))

function applyRecommended() {
  form.value.rate_limit_qps = recommendedQPS.value
  form.value.rate_limit_burst = recommendedBurst.value
}

watch(() => store.settings, (settings) => {
  if (settings) {
    form.value = { ...settings }
  }
}, { immediate: true })

async function handleSave() {
  saving.value = true
  try {
    await store.update(form.value)
    toast.success('设置已保存')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  store.fetch()
})
</script>
