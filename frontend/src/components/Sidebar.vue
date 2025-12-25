<template>
  <aside class="sidebar w-56 flex flex-col">
    <div class="p-5 border-b border-[var(--border-subtle)]">
      <h1 class="text-lg font-semibold text-gray-800">Kiro2API</h1>
      <p class="text-xs text-gray-400 mt-0.5">管理后台</p>
    </div>
    <nav class="flex-1 p-3">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="flex items-center px-3 py-2.5 mb-1 rounded-lg text-sm font-medium transition-all duration-150"
        :class="isActive(item.path)
          ? 'bg-blue-50 text-blue-600 shadow-sm'
          : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'"
      >
        <Icon :name="item.icon" :size="18" :color="isActive(item.path) ? '#2563eb' : '#6b7280'" class="mr-3" />
        {{ item.name }}
      </router-link>
    </nav>
    <div class="p-3 border-t border-[var(--border-subtle)]">
      <button
        class="w-full flex items-center justify-center px-3 py-2.5 text-sm font-medium text-gray-500 bg-gray-50 rounded-lg hover:bg-gray-100 hover:text-gray-700 transition-all duration-150"
        @click="handleLogout"
      >
        <Icon name="logout" :size="16" color="#6b7280" class="mr-2" />
        退出登录
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import Icon from './Icon.vue'

const route = useRoute()
const router = useRouter()

const navItems = [
  { path: '/', name: '概览', icon: 'dashboard' },
  { path: '/tokens', name: 'Token 池', icon: 'key' },
  { path: '/groups', name: '分组管理', icon: 'folder' },
  { path: '/keys', name: 'API Keys', icon: 'lock' },
  { path: '/settings', name: '设置', icon: 'settings' },
]

function isActive(path: string): boolean {
  if (path === '/') {
    return route.path === '/'
  }
  return route.path.startsWith(path)
}

function handleLogout() {
  localStorage.removeItem('api_token')
  router.push('/login')
}
</script>
