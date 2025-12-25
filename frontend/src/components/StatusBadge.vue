<template>
  <span
    class="inline-flex items-center rounded font-medium"
    :class="[statusClass, sizeClass]"
  >
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: 'active' | 'disabled' | 'error' | 'banned' | 'exhausted'
  size?: 'sm' | 'md'
}>()

const statusConfig: Record<string, { class: string; label: string }> = {
  active: { class: 'bg-green-100 text-green-800', label: '正常' },
  disabled: { class: 'bg-gray-100 text-gray-800', label: '已禁用' },
  error: { class: 'bg-red-100 text-red-800', label: '错误' },
  banned: { class: 'bg-red-100 text-red-800', label: '已封禁' },
  exhausted: { class: 'bg-yellow-100 text-yellow-800', label: '已耗尽' },
}

const statusClass = computed(() => statusConfig[props.status]?.class || 'bg-gray-100 text-gray-800')
const label = computed(() => statusConfig[props.status]?.label || props.status)
const sizeClass = computed(() => props.size === 'sm' ? 'px-1 py-0 text-[10px]' : 'px-2 py-0.5 text-xs')
</script>
