<template>
  <div>
    <!-- 标题 + 时间范围选择器 -->
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-xl font-semibold text-gray-800">概览</h1>
      <div class="flex items-center gap-1 p-1 bg-gray-100 rounded-lg">
        <button
          v-for="p in periods"
          :key="p.value"
          class="px-3 py-1 text-xs font-medium rounded-md transition-all"
          :class="period === p.value ? 'bg-white text-gray-800 shadow-sm' : 'text-gray-500 hover:text-gray-700'"
          @click="period = p.value"
        >
          {{ p.label }}
        </button>
      </div>
    </div>

    <!-- 紧凑统计卡片 -->
    <div class="grid grid-cols-6 gap-3 mb-4">
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">请求</div>
        <div class="text-lg font-semibold text-gray-800">{{ formatNumber(filteredStats.total) }}</div>
      </div>
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">成功率</div>
        <div class="text-lg font-semibold text-green-600">{{ filteredStats.successRate }}%</div>
      </div>
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">负载率</div>
        <div class="text-lg font-semibold text-blue-600">{{ loadRate }}%</div>
      </div>
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">Token池</div>
        <div class="text-lg font-semibold">
          <span class="text-green-600">{{ tokensStore.activeTokens }}</span>
          <span class="text-gray-300">/</span>
          <span class="text-gray-500">{{ tokensStore.totalTokens }}</span>
        </div>
      </div>
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">Credit</div>
        <div class="text-lg font-semibold text-orange-600">{{ filteredStats.credit.toFixed(3) }}</div>
      </div>
      <div class="card px-3 py-2">
        <div class="text-xs text-gray-400">Token消耗</div>
        <div class="text-lg font-semibold text-gray-800">{{ formatNumber(filteredStats.tokens) }}</div>
      </div>
    </div>

    <!-- 模型使用柱状图 + 分组概览 -->
    <div class="grid grid-cols-3 gap-4 mb-4">
      <!-- 模型使用柱状图 -->
      <div class="card p-4 col-span-2">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-medium text-gray-700">模型使用</h3>
          <span class="text-xs text-gray-400">总计: {{ formatNumber(filteredStats.total) }}</span>
        </div>
        <div class="h-36">
          <canvas ref="chartCanvas" class="w-full h-full"></canvas>
        </div>
        <!-- 图例 -->
        <div class="flex flex-wrap gap-x-4 gap-y-1 mt-2 pt-2 border-t border-gray-100">
          <div v-for="(item, i) in filteredModelUsage" :key="item[0]" class="flex items-center gap-1">
            <span class="w-2.5 h-2.5 rounded-sm" :style="{ backgroundColor: chartColors[i % chartColors.length] }"></span>
            <span class="text-xs text-gray-500">{{ shortModel(item[0]) }}</span>
          </div>
        </div>
      </div>

      <!-- 分组概览 -->
      <div class="card p-4">
        <h3 class="text-sm font-medium text-gray-700 mb-3">分组</h3>
        <div class="space-y-1.5 max-h-48 overflow-y-auto">
          <div
            v-for="group in groupsStore.groups"
            :key="group.name"
            class="flex items-center justify-between text-xs py-1"
          >
            <span class="text-gray-600 truncate max-w-[100px]">{{ group.display_name || group.name }}</span>
            <span>
              <span class="text-green-600 font-medium">{{ group.active_count }}</span>
              <span class="text-gray-300 mx-0.5">/</span>
              <span class="text-gray-400">{{ group.token_count }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- 使用记录紧凑分页列表 -->
    <div class="card">
      <div class="px-4 py-2 border-b border-[var(--border-subtle)] flex items-center justify-between">
        <h3 class="text-sm font-medium text-gray-700">使用记录 (持久化)</h3>
        <div class="flex items-center gap-2">
          <span class="text-xs text-gray-400">{{ logRecords.length }} / {{ totalRecords }}</span>
          <select
            v-model.number="pageSize"
            class="text-xs border border-[var(--border-subtle)] rounded px-1.5 py-0.5 bg-gray-50"
          >
            <option :value="10">10</option>
            <option :value="20">20</option>
            <option :value="50">50</option>
          </select>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="bg-gray-50/50">
              <th class="px-2 py-2 text-left font-medium text-gray-400">时间</th>
              <th class="px-2 py-2 text-left font-medium text-gray-400">模型</th>
              <th class="px-2 py-2 text-center font-medium text-gray-400">状态</th>
              <th class="px-2 py-2 text-right font-medium text-gray-400">耗时</th>
              <th class="px-2 py-2 text-right font-medium text-gray-400">Token</th>
              <th class="px-2 py-2 text-right font-medium text-gray-400">Context%</th>
              <th class="px-2 py-2 text-right font-medium text-gray-400">Credit</th>
              <th class="px-2 py-2 text-center font-medium text-gray-400">缓存</th>
              <th class="px-2 py-2 text-center font-medium text-gray-400">号池</th>
              <th class="px-2 py-2 text-left font-medium text-gray-400">会话ID</th>
              <th class="px-2 py-2 text-left font-medium text-gray-400">分组</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border-subtle)]">
            <tr v-for="r in logRecords" :key="r.id" class="hover:bg-gray-50/50">
              <td class="px-2 py-1.5 text-gray-500 whitespace-nowrap">{{ formatTime(r.timestamp) }}</td>
              <td class="px-2 py-1.5 font-medium" :class="r.stream ? 'text-blue-700' : 'text-fuchsia-600'">
                {{ shortModel(r.model) }}
              </td>
              <td class="px-2 py-1.5 text-center">
                <span
                  class="inline-flex px-1.5 py-0.5 text-xs rounded"
                  :class="r.status_code < 400 ? 'bg-green-50 text-green-600' : 'bg-red-50 text-red-600'"
                >
                  {{ r.status_code }}
                </span>
              </td>
              <td class="px-2 py-1.5 text-gray-500 text-right whitespace-nowrap">
                {{ formatLatency(r.latency_ms) }}
              </td>
              <td class="px-2 py-1.5 text-blue-600 text-right whitespace-nowrap font-medium">
                {{ formatNumber(r.actual_input_tokens) }}/{{ formatNumber(r.calculated_output_tokens) }}
              </td>
              <td class="px-2 py-1.5 text-right">
                <span v-if="r.context_usage_percent" :class="getContextClass(r.context_usage_percent)">
                  {{ r.context_usage_percent.toFixed(1) }}%
                </span>
                <span v-else class="text-gray-300">-</span>
              </td>
              <td class="px-2 py-1.5 text-orange-600 text-right font-medium">
                {{ r.credit_usage?.toFixed(4) || '-' }}
              </td>
              <td class="px-2 py-1.5 text-center">
                <span v-if="r.cache_hit" class="text-green-500">✓</span>
                <span v-else class="text-gray-300">-</span>
              </td>
              <td class="px-2 py-1.5 text-gray-500 text-center">{{ r.token_index }}</td>
              <td class="px-2 py-1.5 text-gray-400 text-xs truncate max-w-[80px]" :title="r.conversation_id">
                {{ r.conversation_id?.slice(-8) || '-' }}
              </td>
              <td class="px-2 py-1.5 text-gray-400">{{ r.group || 'default' }}</td>
            </tr>
            <tr v-if="logRecords.length === 0">
              <td colspan="11" class="px-3 py-6 text-center text-gray-400">暂无记录</td>
            </tr>
          </tbody>
        </table>
      </div>
      <!-- 分页控件 -->
      <div v-if="totalPages > 1" class="px-4 py-2 border-t border-[var(--border-subtle)] flex items-center justify-between">
        <span class="text-xs text-gray-400">第 {{ page }} / {{ totalPages }} 页</span>
        <div class="flex items-center gap-1">
          <button
            class="px-2 py-1 text-xs border border-[var(--border-subtle)] rounded hover:bg-gray-50 disabled:opacity-50"
            :disabled="page <= 1"
            @click="page--"
          >上一页</button>
          <button
            class="px-2 py-1 text-xs border border-[var(--border-subtle)] rounded hover:bg-gray-50 disabled:opacity-50"
            :disabled="page >= totalPages"
            @click="page++"
          >下一页</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { statsApi } from '@/api/stats'
import { useTokensStore } from '@/stores/tokens'
import { useGroupsStore } from '@/stores/groups'
import type { Stats, LogRecord, LogsResponse, RequestRecord } from '@/types'

const tokensStore = useTokensStore()
const groupsStore = useGroupsStore()

const stats = ref<Stats | null>(null)
const logs = ref<LogsResponse | null>(null)
const loading = ref(false)
const chartCanvas = ref<HTMLCanvasElement | null>(null)
let refreshInterval: number | null = null

// 时间范围
const periods = [
  { label: '24h', value: 24 },
  { label: '7天', value: 168 },
  { label: '14天', value: 336 },
  { label: '30天', value: 720 },
]
const period = ref(24) // 默认 24h（小时数）

// 分页
const page = ref(1)
const pageSize = ref(20)

// 图表颜色
const chartColors = ['#60a5fa', '#34d399', '#fbbf24', '#f87171', '#a78bfa', '#22d3ee', '#f472b6', '#a3e635']

// 日志记录（来自数据库）
const logRecords = computed<LogRecord[]>(() => logs.value?.records || [])
const totalPages = computed(() => logs.value?.pages || 1)
const totalRecords = computed(() => logs.value?.total || 0)

// 按时间过滤的记录
const filteredRecords = computed<RequestRecord[]>(() => {
  if (!stats.value?.recent_records) return []
  const now = Date.now()
  const cutoff = now - period.value * 60 * 60 * 1000
  return stats.value.recent_records.filter(r => {
    const ts = new Date(r.timestamp).getTime()
    return ts >= cutoff
  })
})

// 根据过滤后的记录计算统计
const filteredStats = computed(() => {
  const records = filteredRecords.value
  const total = records.length
  const success = records.filter(r => r.status_code >= 200 && r.status_code < 400).length
  const successRate = total > 0 ? ((success / total) * 100).toFixed(1) : '0'
  const avgLatency = total > 0 ? records.reduce((sum, r) => sum + r.latency, 0) / total : 0
  const credit = records.reduce((sum, r) => sum + (r.credit_usage || 0), 0)
  const tokens = records.reduce((sum, r) => sum + r.input_tokens + r.output_tokens, 0)
  return { total, success, successRate, avgLatency, credit, tokens }
})

// 按时间过滤的模型使用数据
const filteredModelUsage = computed(() => {
  const records = filteredRecords.value
  const usage: Record<string, number> = {}
  for (const r of records) {
    if (r.model) {
      usage[r.model] = (usage[r.model] || 0) + 1
    }
  }
  const entries = Object.entries(usage)
  return entries.sort((a, b) => b[1] - a[1]).slice(0, 8)
})

// 当期间改变时重置页码
watch(period, () => {
  page.value = 1
})

function formatNumber(num: number): string {
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  // 如果是今天，只显示时间；否则显示日期+时间
  const today = new Date()
  if (date.toDateString() === today.toDateString()) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' +
         date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function shortModel(model: string): string {
  return model.replace('claude-', '').replace(/-20\d{6}/, '')
}

function getContextClass(percent: number): string {
  if (percent >= 80) return 'text-red-500 font-medium'
  if (percent >= 50) return 'text-orange-500'
  return 'text-gray-500'
}

// 负载率 = 正在被请求的Token / 活跃Token
const loadRate = computed(() => {
  const active = tokensStore.activeTokens
  if (active <= 0) return '0'
  return ((tokensStore.tokensWithInFlight / active) * 100).toFixed(1)
})

// 格式化延迟显示
function formatLatency(ms: number): string {
  if (ms >= 1000) {
    return (ms / 1000).toFixed(1) + 's'
  }
  return ms + 'ms'
}

// 绘制柱状图
function drawChart() {
  const canvas = chartCanvas.value
  if (!canvas) return

  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const rect = canvas.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
  canvas.width = rect.width * dpr
  canvas.height = rect.height * dpr
  ctx.scale(dpr, dpr)

  const width = rect.width
  const height = rect.height
  const data = filteredModelUsage.value

  ctx.clearRect(0, 0, width, height)

  if (data.length === 0) {
    ctx.fillStyle = '#9ca3af'
    ctx.font = '12px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText('暂无数据', width / 2, height / 2)
    return
  }

  const padding = { top: 20, right: 10, bottom: 24, left: 40 }
  const chartWidth = width - padding.left - padding.right
  const chartHeight = height - padding.top - padding.bottom
  const barWidth = Math.min(50, (chartWidth / data.length) * 0.6)
  const gap = (chartWidth - barWidth * data.length) / (data.length + 1)
  const maxVal = Math.max(...data.map(d => d[1]), 1)

  // Y 轴刻度
  ctx.fillStyle = '#9ca3af'
  ctx.font = '10px sans-serif'
  ctx.textAlign = 'right'
  const yTicks = 4
  for (let i = 0; i <= yTicks; i++) {
    const y = padding.top + chartHeight * (1 - i / yTicks)
    const val = Math.round(maxVal * i / yTicks)
    ctx.fillText(formatNumber(val), padding.left - 5, y + 3)
    // 网格线
    ctx.strokeStyle = '#f3f4f6'
    ctx.beginPath()
    ctx.moveTo(padding.left, y)
    ctx.lineTo(width - padding.right, y)
    ctx.stroke()
  }

  // 柱子和标签
  data.forEach(([model, count], i) => {
    const x = padding.left + gap + i * (barWidth + gap)
    const barHeight = (count / maxVal) * chartHeight
    const y = padding.top + chartHeight - barHeight

    // 柱子（圆角）
    ctx.fillStyle = chartColors[i % chartColors.length]
    ctx.beginPath()
    const radius = Math.min(4, barWidth / 4)
    ctx.roundRect(x, y, barWidth, barHeight, [radius, radius, 0, 0])
    ctx.fill()

    // 数值（柱子上方）
    ctx.fillStyle = '#374151'
    ctx.font = '10px sans-serif'
    ctx.textAlign = 'center'
    ctx.fillText(formatNumber(count), x + barWidth / 2, y - 5)

    // X 轴标签（水平）
    ctx.fillStyle = '#6b7280'
    ctx.font = '9px sans-serif'
    ctx.textAlign = 'center'
    const label = shortModel(model).slice(0, 10)
    ctx.fillText(label, x + barWidth / 2, height - 8)
  })
}

watch(filteredModelUsage, () => {
  nextTick(drawChart)
})

async function fetchStats() {
  loading.value = true
  try {
    stats.value = await statsApi.get()
    nextTick(drawChart)
  } catch (e) {
    console.error('Failed to fetch stats:', e)
  } finally {
    loading.value = false
  }
}

async function fetchLogs() {
  try {
    logs.value = await statsApi.getLogs({ page: page.value, page_size: pageSize.value })
  } catch (e) {
    console.error('Failed to fetch logs:', e)
  }
}

// 分页变化时重新获取日志
watch([page, pageSize], () => {
  fetchLogs()
})

onMounted(() => {
  fetchStats()
  fetchLogs()
  tokensStore.fetch()
  groupsStore.fetch()
  refreshInterval = window.setInterval(() => {
    fetchStats()
    fetchLogs()
  }, 30000)
  window.addEventListener('resize', drawChart)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  window.removeEventListener('resize', drawChart)
})
</script>
