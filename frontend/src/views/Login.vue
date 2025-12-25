<template>
  <div class="min-h-screen flex items-center justify-center bg-[var(--bg-warm)]">
    <div class="card p-8 w-full max-w-md">
      <div class="text-center mb-8">
        <div class="w-12 h-12 bg-blue-50 rounded-xl flex items-center justify-center mx-auto mb-4">
          <Icon name="lock" :size="24" color="#2563eb" />
        </div>
        <h1 class="text-xl font-semibold text-gray-800">Kiro2API</h1>
        <p class="text-sm text-gray-400 mt-1">管理后台</p>
      </div>

      <form @submit.prevent="handleLogin">
        <div class="mb-5">
          <label class="block text-sm font-medium text-gray-600 mb-2">API Token</label>
          <input
            v-model="token"
            type="password"
            class="w-full px-4 py-3 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 focus:bg-white transition-all"
            placeholder="输入 API Token"
            autofocus
          />
        </div>

        <div v-if="error" class="mb-4 p-3 bg-red-50 text-red-600 text-sm rounded-lg border border-red-100">
          {{ error }}
        </div>

        <button
          type="submit"
          class="w-full py-3 btn-primary text-white rounded-lg font-medium transition-all duration-150 disabled:opacity-50"
          :disabled="loading"
        >
          {{ loading ? '验证中...' : '登录' }}
        </button>
      </form>

      <p class="mt-6 text-center text-xs text-gray-400">
        Token 仅存储在浏览器本地
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { http } from '@/api/client'
import Icon from '@/components/Icon.vue'

const router = useRouter()
const token = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  if (!token.value.trim()) {
    error.value = '请输入 API Token'
    return
  }

  loading.value = true
  error.value = ''

  localStorage.setItem('api_token', token.value.trim())

  try {
    await http.get('/api/settings')
    router.push('/')
  } catch (e) {
    localStorage.removeItem('api_token')
    error.value = e instanceof Error ? e.message : '认证失败'
  } finally {
    loading.value = false
  }
}
</script>
