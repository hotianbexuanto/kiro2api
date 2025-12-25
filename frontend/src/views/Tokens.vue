<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl font-semibold text-gray-800">Token 池</h1>
      <div class="flex space-x-3">
        <button
          class="flex items-center px-4 py-2 text-sm font-medium text-gray-600 bg-white border border-[var(--border-subtle)] rounded-lg hover:bg-gray-50 transition-all"
          @click="handleRefreshAll"
          :disabled="refreshing"
        >
          <Icon name="refresh" :size="16" color="#6b7280" class="mr-2" />
          {{ refreshing ? '刷新中...' : '刷新全部' }}
        </button>
        <button
          class="flex items-center px-4 py-2 text-sm font-medium text-white btn-primary rounded-lg"
          @click="openAddModal"
        >
          <Icon name="plus" :size="16" color="#ffffff" class="mr-2" />
          添加 Token
        </button>
      </div>
    </div>

    <!-- 分组筛选 + 分页控制 -->
    <div class="card">
      <div class="border-b border-[var(--border-subtle)] px-4 py-3 flex items-center justify-between">
        <div class="flex items-center gap-3 min-w-0 flex-1">
          <label class="text-sm text-gray-500 flex-shrink-0">分组:</label>
          <!-- 普通分组：可滑动 -->
          <div class="flex items-center gap-1 overflow-x-auto scrollbar-hide min-w-0 flex-1 py-1">
            <button
              class="px-2 py-1 text-xs rounded whitespace-nowrap transition-all flex-shrink-0"
              :class="!store.filterGroup
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'"
              @click="store.setFilterGroup(undefined)"
            >
              全部
            </button>
            <button
              v-for="g in normalGroups"
              :key="g"
              class="px-2 py-1 text-xs rounded whitespace-nowrap transition-all flex-shrink-0"
              :class="store.filterGroup === g
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'"
              @click="store.setFilterGroup(g)"
            >
              {{ g }}
            </button>
          </div>
          <!-- 分隔线 -->
          <div class="w-px h-5 bg-gray-200 flex-shrink-0"></div>
          <!-- 特殊分组：固定 -->
          <div class="flex items-center gap-1 flex-shrink-0">
            <button
              v-for="sg in specialGroups"
              :key="sg"
              class="px-2 py-1 text-xs rounded whitespace-nowrap transition-all"
              :class="store.filterGroup === sg
                ? (sg === 'exhausted' ? 'bg-orange-500 text-white' : 'bg-red-500 text-white')
                : 'bg-gray-100 text-gray-500 hover:bg-gray-200'"
              @click="store.setFilterGroup(store.filterGroup === sg ? undefined : sg)"
            >
              {{ sg === 'exhausted' ? '耗尽' : '封禁' }}
            </button>
          </div>
          <span class="text-xs text-gray-400 flex-shrink-0">
            {{ filteredTokens.length }}/{{ store.totalTokens }}
          </span>
        </div>
        <div class="flex items-center space-x-4">
          <label class="text-sm text-gray-500">列:</label>
          <select
            class="text-sm border border-[var(--border-subtle)] rounded-lg px-2 py-1 bg-gray-50 focus:bg-white focus:outline-none"
            v-model.number="columns"
          >
            <option :value="1">1</option>
            <option :value="2">2</option>
            <option :value="3">3</option>
            <option :value="4">4</option>
          </select>
          <label class="text-sm text-gray-500">每页:</label>
          <select
            class="text-sm border border-[var(--border-subtle)] rounded-lg px-2 py-1 bg-gray-50 focus:bg-white focus:outline-none"
            :value="store.pageSize"
            @change="handlePageSizeChange"
          >
            <option :value="50">50</option>
            <option :value="100">100</option>
            <option :value="200">200</option>
            <option :value="500">500</option>
          </select>
        </div>
      </div>

      <!-- Token 网格 -->
      <div class="p-4">
        <div v-if="store.loading || store.isBackendLoading" class="text-center py-12 text-gray-400">
          <span v-if="store.isBackendLoading">后端正在初始化缓存，请稍候...</span>
          <span v-else>加载中...</span>
        </div>
        <div v-else-if="filteredTokens.length === 0" class="text-center py-12 text-gray-400">
          暂无 Token
        </div>
        <div v-else class="grid gap-2" :class="gridClass">
          <div
            v-for="token in filteredTokens"
            :key="token.index"
            class="border border-[var(--border-subtle)] rounded-lg p-2 hover:border-gray-300 transition-all bg-white text-xs"
          >
            <!-- 第一行：名称 + 状态 + 操作 -->
            <div class="flex items-center justify-between gap-1 mb-1">
              <div class="flex items-center gap-1 min-w-0 flex-1">
                <span class="font-medium text-gray-800 truncate">{{ token.name || `#${token.index}` }}</span>
                <StatusBadge :status="token.status" size="sm" />
              </div>
              <div class="flex items-center gap-0.5 flex-shrink-0">
                <button
                  class="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded transition-all"
                  :title="token.status === 'disabled' ? '启用' : '禁用'"
                  @click="handleToggle(token)"
                >
                  <Icon :name="token.status === 'disabled' ? 'check' : 'x'" :size="12" color="currentColor" />
                </button>
                <button
                  class="p-1 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded transition-all"
                  title="删除"
                  @click="handleDelete(token)"
                >
                  <Icon name="trash" :size="12" color="currentColor" />
                </button>
              </div>
            </div>
            <!-- 第二行：进度条 -->
            <div class="flex items-center gap-1">
              <div class="flex-1 h-1.5 bg-gray-100 rounded-full overflow-hidden">
                <div
                  class="h-full transition-all duration-300"
                  :class="getUsageBarClass(token)"
                  :style="{ width: getUsagePercent(token) + '%' }"
                ></div>
              </div>
              <span class="text-gray-500 w-12 text-right">
                {{ formatUsage(token.remaining_usage) }}
              </span>
            </div>
            <!-- 第三行：邮箱 + 成功率 -->
            <div class="flex items-center justify-between mt-1">
              <span class="text-gray-400 truncate" style="max-width: 100px;">{{ token.user_email || '-' }}</span>
              <span
                class="text-xs px-1.5 py-0.5 rounded"
                :class="getSuccessRateClass(token)"
              >
                {{ getSuccessRate(token) }}
              </span>
            </div>
            <!-- 错误信息 -->
            <div v-if="token.error" class="mt-1 text-red-500 truncate">
              {{ token.error }}
            </div>
          </div>
        </div>
      </div>

      <!-- 分页控件 -->
      <div v-if="store.totalPages > 1" class="border-t border-[var(--border-subtle)] px-4 py-3 flex items-center justify-between">
        <div class="text-sm text-gray-500">
          第 {{ store.page }} / {{ store.totalPages }} 页
        </div>
        <div class="flex items-center space-x-2">
          <button
            class="px-3 py-1.5 text-sm border border-[var(--border-subtle)] rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50 transition-all"
            :disabled="store.page <= 1"
            @click="store.setPage(store.page - 1)"
          >
            上一页
          </button>
          <!-- 页码按钮 -->
          <template v-for="p in visiblePages" :key="p">
            <span v-if="p === '...'" class="px-2 text-gray-400">...</span>
            <button
              v-else
              class="px-3 py-1.5 text-sm border rounded-lg transition-all"
              :class="p === store.page ? 'border-blue-500 bg-blue-50 text-blue-600' : 'border-[var(--border-subtle)] hover:bg-gray-50'"
              @click="store.setPage(p as number)"
            >
              {{ p }}
            </button>
          </template>
          <button
            class="px-3 py-1.5 text-sm border border-[var(--border-subtle)] rounded-lg disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-50 transition-all"
            :disabled="store.page >= store.totalPages"
            @click="store.setPage(store.page + 1)"
          >
            下一页
          </button>
        </div>
      </div>
    </div>

    <!-- 添加 Token 弹窗 -->
    <Modal :visible="showAddModal" title="添加 Token" @close="showAddModal = false">
      <!-- 模式切换 -->
      <div class="flex mb-4 p-1 bg-gray-100 rounded-lg">
        <button
          class="flex-1 py-2 text-sm font-medium rounded-md transition-all"
          :class="addMode === 'single' ? 'bg-white text-gray-800 shadow-sm' : 'text-gray-500'"
          @click="addMode = 'single'"
        >
          单个添加
        </button>
        <button
          class="flex-1 py-2 text-sm font-medium rounded-md transition-all"
          :class="addMode === 'batch' ? 'bg-white text-gray-800 shadow-sm' : 'text-gray-500'"
          @click="addMode = 'batch'"
        >
          批量导入
        </button>
      </div>

      <!-- 单个添加 -->
      <form v-if="addMode === 'single'" @submit.prevent="handleAdd">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">名称（可选）</label>
            <input
              v-model="addForm.name"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              placeholder="Token 备注名称"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">认证类型</label>
            <select
              v-model="addForm.auth"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            >
              <option value="Social">Social</option>
              <option value="IdC">IdC</option>
            </select>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">Refresh Token *</label>
            <textarea
              v-model="addForm.refreshToken"
              rows="3"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all font-mono text-sm"
              placeholder="输入 Refresh Token"
              required
            ></textarea>
          </div>
          <div v-if="addForm.auth === 'IdC'">
            <label class="block text-sm font-medium text-gray-600 mb-1.5">Client ID</label>
            <input
              v-model="addForm.clientId"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
          </div>
          <div v-if="addForm.auth === 'IdC'">
            <label class="block text-sm font-medium text-gray-600 mb-1.5">Client Secret</label>
            <input
              v-model="addForm.clientSecret"
              type="password"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">分组</label>
            <select
              v-model="addForm.group"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
            >
              <option v-for="g in normalGroups" :key="g" :value="g">{{ g }}</option>
            </select>
          </div>
        </div>
        <div class="mt-6 flex justify-end space-x-3">
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            @click="showAddModal = false"
          >
            取消
          </button>
          <button
            type="submit"
            class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg disabled:opacity-50"
            :disabled="adding"
          >
            {{ adding ? '添加中...' : '添加' }}
          </button>
        </div>
      </form>

      <!-- 批量导入 -->
      <div v-else class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-600 mb-1.5">JSON 数据</label>
          <div
            class="relative"
            @dragover.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="handleDrop"
          >
            <textarea
              v-model="batchJson"
              rows="8"
              class="w-full px-3 py-2.5 border rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all font-mono text-xs"
              :class="isDragging ? 'border-blue-400 bg-blue-50/50' : 'border-[var(--border-subtle)]'"
              placeholder='粘贴 JSON 或拖放文件到此处'
              @paste="handlePaste"
            ></textarea>
            <input
              ref="fileInput"
              type="file"
              accept=".json,.txt"
              class="hidden"
              @change="handleFileSelect"
            />
            <button
              type="button"
              class="absolute top-2 right-2 px-2.5 py-1.5 text-xs font-medium text-gray-500 bg-white border border-[var(--border-subtle)] rounded-md hover:bg-gray-50 transition-all"
              @click="fileInput?.click()"
            >
              选择文件
            </button>
            <!-- 拖放提示遮罩 -->
            <div
              v-if="isDragging"
              class="absolute inset-0 flex items-center justify-center bg-blue-50/80 border-2 border-dashed border-blue-400 rounded-lg pointer-events-none"
            >
              <span class="text-sm font-medium text-blue-600">释放以导入文件</span>
            </div>
          </div>
          <p class="text-xs text-gray-400 mt-1.5">
            支持粘贴/拖放 JSON 文件，字段: refreshToken, authMethod
          </p>
        </div>

        <!-- 解析预览 -->
        <div v-if="parsedTokens.length > 0" class="border border-[var(--border-subtle)] rounded-lg overflow-hidden">
          <div class="px-3 py-2 bg-gray-50 border-b border-[var(--border-subtle)] flex items-center justify-between">
            <span class="text-sm font-medium text-gray-600">解析结果: {{ parsedTokens.length }} 个</span>
            <button
              type="button"
              class="text-xs text-gray-400 hover:text-gray-600"
              @click="parsedTokens = []"
            >
              清空
            </button>
          </div>
          <div class="max-h-40 overflow-y-auto">
            <div
              v-for="(t, i) in parsedTokens"
              :key="i"
              class="px-3 py-2 text-xs border-b border-[var(--border-subtle)] last:border-0 flex items-center justify-between"
            >
              <span class="font-mono text-gray-600 truncate flex-1">{{ t.refreshToken.slice(0, 40) }}...</span>
              <span class="ml-2 px-1.5 py-0.5 rounded text-xs" :class="t.auth === 'Social' ? 'bg-blue-50 text-blue-600' : 'bg-purple-50 text-purple-600'">
                {{ t.auth }}
              </span>
            </div>
          </div>
        </div>

        <div v-if="parseError" class="text-sm text-red-500 p-3 bg-red-50 rounded-lg">
          {{ parseError }}
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-600 mb-1.5">目标分组</label>
          <select
            v-model="batchGroup"
            class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
          >
            <option v-for="g in normalGroups" :key="g" :value="g">{{ g }}</option>
          </select>
        </div>

        <div class="flex justify-end space-x-3">
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            @click="showAddModal = false"
          >
            取消
          </button>
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-white border border-[var(--border-subtle)] rounded-lg hover:bg-gray-50 transition-all"
            @click="parseJson"
            :disabled="!batchJson.trim()"
          >
            解析
          </button>
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg disabled:opacity-50"
            :disabled="parsedTokens.length === 0 || adding"
            @click="handleBatchAdd"
          >
            {{ adding ? '导入中...' : `导入 ${parsedTokens.length} 个` }}
          </button>
        </div>
      </div>
    </Modal>

    <!-- 删除确认 -->
    <ConfirmDialog
      :visible="showDeleteConfirm"
      title="确认删除"
      :message="`确定要删除 Token #${tokenToDelete?.index} 吗？此操作不可撤销。`"
      @confirm="confirmDelete"
      @cancel="showDeleteConfirm = false"
    />

    <!-- 刷新进度弹窗 -->
    <Modal :visible="showRefreshModal" title="刷新 Token" @close="closeRefreshModal">
      <div class="space-y-4">
        <!-- 进度状态 -->
        <div v-if="refreshing" class="text-center py-6">
          <div class="inline-flex items-center justify-center w-12 h-12 bg-blue-50 rounded-full mb-4">
            <Icon name="refresh" :size="24" color="#2563eb" class="animate-spin" />
          </div>
          <p class="text-gray-600">正在刷新 Token...</p>
          <p class="text-sm text-gray-400 mt-2">并发数: {{ refreshConcurrency }}</p>
        </div>

        <!-- 刷新结果 -->
        <div v-else-if="refreshResult">
          <div class="grid grid-cols-3 gap-3 mb-4">
            <div class="text-center p-3 bg-gray-50 rounded-lg">
              <div class="text-2xl font-semibold text-gray-700">{{ refreshResult.total }}</div>
              <div class="text-xs text-gray-400">总数</div>
            </div>
            <div class="text-center p-3 bg-green-50 rounded-lg">
              <div class="text-2xl font-semibold text-green-600">{{ refreshResult.refreshed }}</div>
              <div class="text-xs text-gray-400">成功</div>
            </div>
            <div class="text-center p-3 bg-red-50 rounded-lg">
              <div class="text-2xl font-semibold text-red-500">{{ refreshResult.failed }}</div>
              <div class="text-xs text-gray-400">失败</div>
            </div>
          </div>

          <!-- 失败详情 -->
          <div v-if="failedResults.length > 0" class="border border-[var(--border-subtle)] rounded-lg overflow-hidden">
            <div class="px-3 py-2 bg-red-50 border-b border-[var(--border-subtle)] text-sm font-medium text-red-700">
              失败详情
            </div>
            <div class="max-h-48 overflow-y-auto">
              <div
                v-for="r in failedResults"
                :key="r.id"
                class="px-3 py-2 text-xs border-b border-[var(--border-subtle)] last:border-0"
              >
                <div class="flex items-center justify-between">
                  <span class="font-medium text-gray-700">{{ r.name || `#${r.id}` }}</span>
                  <span class="text-red-500 truncate max-w-[200px]">{{ r.error }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 成功详情（可折叠） -->
          <details v-if="successResults.length > 0" class="border border-[var(--border-subtle)] rounded-lg overflow-hidden mt-3">
            <summary class="px-3 py-2 bg-green-50 text-sm font-medium text-green-700 cursor-pointer hover:bg-green-100 transition-colors">
              成功详情 ({{ successResults.length }})
            </summary>
            <div class="max-h-48 overflow-y-auto">
              <div
                v-for="r in successResults"
                :key="r.id"
                class="px-3 py-2 text-xs border-b border-[var(--border-subtle)] last:border-0 flex items-center justify-between"
              >
                <span class="font-medium text-gray-700">{{ r.name || `#${r.id}` }}</span>
                <div class="flex items-center gap-2">
                  <span class="text-gray-500">{{ r.user_email }}</span>
                  <span class="text-green-600">{{ formatUsage(r.remaining_usage || 0) }}</span>
                </div>
              </div>
            </div>
          </details>
        </div>

        <div class="flex justify-end">
          <button
            class="px-4 py-2 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            :disabled="refreshing"
            @click="closeRefreshModal"
          >
            {{ refreshing ? '请等待...' : '关闭' }}
          </button>
        </div>
      </div>
    </Modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useTokensStore } from '@/stores/tokens'
import { useGroupsStore } from '@/stores/groups'
import { useToast } from '@/composables/useToast'
import { ApiRequestError } from '@/api/client'
import StatusBadge from '@/components/StatusBadge.vue'
import Modal from '@/components/Modal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import Icon from '@/components/Icon.vue'
import type { Token, RefreshTokensResponse } from '@/types'

const store = useTokensStore()
const groupsStore = useGroupsStore()
const toast = useToast()

const showAddModal = ref(false)
const showDeleteConfirm = ref(false)
const tokenToDelete = ref<Token | null>(null)
const adding = ref(false)
const refreshing = ref(false)
const columns = ref(3)  // 默认 3 列

// 刷新弹窗相关
const showRefreshModal = ref(false)
const refreshResult = ref<RefreshTokensResponse | null>(null)
const refreshConcurrency = ref(5)

// 添加模式
const addMode = ref<'single' | 'batch'>('single')
const fileInput = ref<HTMLInputElement | null>(null)

// 单个添加表单
const addForm = ref({
  name: '',
  auth: 'Social' as 'Social' | 'IdC',
  refreshToken: '',
  clientId: '',
  clientSecret: '',
  group: 'default',
})

// 批量导入
const batchJson = ref('')
const batchGroup = ref('default')
const parsedTokens = ref<Array<{ refreshToken: string; auth: 'Social' | 'IdC' }>>([])
const parseError = ref('')
const isDragging = ref(false)

// 特殊分组
const specialGroups = ['exhausted', 'banned']

// 从分组 store 获取普通分组名
const normalGroups = computed(() => {
  const names = groupsStore.groups.map(g => g.name).filter(n => !specialGroups.includes(n))
  if (!names.includes('default')) names.unshift('default')
  return names
})

// 过滤 token：默认视图（无分组筛选）不显示耗尽/封禁
const filteredTokens = computed(() => {
  if (store.filterGroup) {
    // 选了特定分组，显示全部
    return store.tokens
  }
  // 默认视图：过滤掉 exhausted 和 banned
  return store.tokens.filter(t => t.status !== 'exhausted' && t.status !== 'banned')
})

// 网格样式
const gridClass = computed(() => {
  switch (columns.value) {
    case 1: return 'grid-cols-1'
    case 2: return 'grid-cols-2'
    case 3: return 'grid-cols-3'
    case 4: return 'grid-cols-4'
    default: return 'grid-cols-3'
  }
})

// 刷新结果分类
const failedResults = computed(() =>
  refreshResult.value?.results.filter(r => r.status === 'error') || []
)
const successResults = computed(() =>
  refreshResult.value?.results.filter(r => r.status !== 'error') || []
)

// 分页按钮
const visiblePages = computed(() => {
  const total = store.totalPages
  const current = store.page
  const pages: (number | string)[] = []

  if (total <= 7) {
    for (let i = 1; i <= total; i++) pages.push(i)
  } else {
    pages.push(1)
    if (current > 3) pages.push('...')

    const start = Math.max(2, current - 1)
    const end = Math.min(total - 1, current + 1)
    for (let i = start; i <= end; i++) pages.push(i)

    if (current < total - 2) pages.push('...')
    pages.push(total)
  }

  return pages
})

function handlePageSizeChange(e: Event) {
  const value = parseInt((e.target as HTMLSelectElement).value)
  store.setPageSize(value)
}

function formatUsage(value: number): string {
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'k'
  }
  return value.toFixed(2)
}

function getUsagePercent(token: Token): number {
  if (!token.usage_limits || token.usage_limits.total_limit <= 0) {
    // 无限制信息时，根据 remaining_usage 估算
    return token.remaining_usage > 0 ? Math.min(100, token.remaining_usage / 10) : 0
  }
  const used = token.usage_limits.current_usage
  const total = token.usage_limits.total_limit
  const remaining = Math.max(0, total - used)
  return Math.min(100, (remaining / total) * 100)
}

function getUsageBarClass(token: Token): string {
  if (token.status === 'banned') return 'bg-gray-400'
  if (token.status === 'exhausted' || token.remaining_usage <= 0) return 'bg-red-400'
  if (token.status === 'disabled') return 'bg-gray-300'

  const percent = getUsagePercent(token)
  if (percent > 50) return 'bg-green-500'
  if (percent > 20) return 'bg-yellow-500'
  return 'bg-orange-500'
}

// 基于请求统计显示成功率
function getSuccessRate(token: Token): string {
  const reqCount = token.request_count ?? 0
  const successCount = token.success_count ?? 0
  return `${successCount}/${reqCount}`
}

function getSuccessRateClass(token: Token): string {
  const reqCount = token.request_count ?? 0
  const successCount = token.success_count ?? 0

  // 无请求记录
  if (reqCount === 0) {
    return 'bg-gray-50 text-gray-400'
  }

  // 基于成功率设置样式
  const rate = (successCount / reqCount) * 100
  if (rate >= 95) return 'bg-green-50 text-green-600'
  if (rate >= 80) return 'bg-yellow-50 text-yellow-600'
  return 'bg-red-50 text-red-500'
}

function openAddModal() {
  addMode.value = 'single'
  batchJson.value = ''
  parsedTokens.value = []
  parseError.value = ''
  // 使用当前筛选的分组，如果是特殊分组或空则用 default
  const targetGroup = store.filterGroup && !['exhausted', 'banned'].includes(store.filterGroup)
    ? store.filterGroup
    : 'default'
  addForm.value.group = targetGroup
  batchGroup.value = targetGroup
  showAddModal.value = true
}

function parseJson() {
  parseError.value = ''
  parsedTokens.value = []

  const text = batchJson.value.trim()
  if (!text) return

  try {
    // 尝试解析 JSON
    let data: unknown
    // 处理可能不完整的 JSON（末尾缺少 ]）
    let jsonText = text
    if (!jsonText.endsWith(']') && jsonText.startsWith('[')) {
      // 尝试补全
      const lastBrace = jsonText.lastIndexOf('}')
      if (lastBrace > 0) {
        jsonText = jsonText.slice(0, lastBrace + 1) + ']'
      }
    }
    data = JSON.parse(jsonText)

    // 支持单个对象或数组
    const items = Array.isArray(data) ? data : [data]

    for (const item of items) {
      if (!item || typeof item !== 'object') continue

      const rt = (item as Record<string, unknown>).refreshToken || (item as Record<string, unknown>).refresh_token
      if (!rt || typeof rt !== 'string') continue

      // 解析 authMethod
      const am = String((item as Record<string, unknown>).authMethod || (item as Record<string, unknown>).auth_method || 'social').toLowerCase()
      const auth: 'Social' | 'IdC' = am === 'idc' || am === 'oidc' ? 'IdC' : 'Social'

      parsedTokens.value.push({ refreshToken: rt, auth })
    }

    if (parsedTokens.value.length === 0) {
      parseError.value = '未找到有效的 Token（需要 refreshToken 字段）'
    }
  } catch (e) {
    parseError.value = 'JSON 解析失败: ' + (e instanceof Error ? e.message : '格式错误')
  }
}

function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  readFile(file)
  input.value = ''
}

function handleDrop(e: DragEvent) {
  isDragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file) readFile(file)
}

function handlePaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return

  for (const item of items) {
    if (item.kind === 'file') {
      e.preventDefault()
      const file = item.getAsFile()
      if (file) readFile(file)
      return
    }
  }
  // 文本粘贴由 v-model 处理
}

function readFile(file: File) {
  const reader = new FileReader()
  reader.onload = () => {
    batchJson.value = reader.result as string
    parseJson()
  }
  reader.readAsText(file)
}

async function handleAdd() {
  if (!addForm.value.refreshToken.trim()) {
    toast.error('请输入 Refresh Token')
    return
  }

  adding.value = true
  try {
    await store.add({
      refreshToken: addForm.value.refreshToken.trim(),
      auth: addForm.value.auth,
      clientId: addForm.value.clientId || undefined,
      clientSecret: addForm.value.clientSecret || undefined,
      group: addForm.value.group,
      name: addForm.value.name || undefined,
    })
    toast.success('Token 已添加')
    showAddModal.value = false
    addForm.value = {
      name: '',
      auth: 'Social',
      refreshToken: '',
      clientId: '',
      clientSecret: '',
      group: 'default',
    }
  } catch (e) {
    if (e instanceof ApiRequestError && e.data.duplicate) {
      const existing = e.data.existing
      toast.error(`Token 已存在${existing?.name ? ` (${existing.name})` : ''}，分组: ${existing?.group || 'default'}`)
    } else {
      toast.error(e instanceof Error ? e.message : '添加失败')
    }
  } finally {
    adding.value = false
  }
}

async function handleBatchAdd() {
  if (parsedTokens.value.length === 0) return

  adding.value = true
  try {
    const tokens = parsedTokens.value.map(t => ({
      refreshToken: t.refreshToken,
      auth: t.auth,
      group: batchGroup.value,
    }))
    const result = await store.addBulk(tokens)
    const parts: string[] = []
    if (result.added > 0) parts.push(`${result.added} 成功`)
    if (result.duplicates > 0) parts.push(`${result.duplicates} 重复`)
    if (result.skipped?.length > 0) parts.push(`${result.skipped.length} 跳过`)
    toast.success(`导入完成: ${parts.join(', ')}`)
    showAddModal.value = false
    parsedTokens.value = []
    batchJson.value = ''
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '导入失败')
  } finally {
    adding.value = false
  }
}

async function handleToggle(token: Token) {
  try {
    await store.update(token.index, { disabled: token.status !== 'disabled' })
    toast.success(token.status === 'disabled' ? 'Token 已启用' : 'Token 已禁用')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '操作失败')
  }
}

function handleDelete(token: Token) {
  tokenToDelete.value = token
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  if (!tokenToDelete.value) return
  try {
    await store.remove(tokenToDelete.value.index)
    toast.success('Token 已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    showDeleteConfirm.value = false
    tokenToDelete.value = null
  }
}

async function handleRefreshAll() {
  showRefreshModal.value = true
  refreshResult.value = null
  refreshing.value = true
  try {
    const result = await store.refresh()
    refreshResult.value = result
    refreshConcurrency.value = result.concurrency || 5
    // store.refresh() 已经自带 fetch()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '刷新失败')
    showRefreshModal.value = false
  } finally {
    refreshing.value = false
  }
}

function closeRefreshModal() {
  if (refreshing.value) return
  showRefreshModal.value = false
  refreshResult.value = null
}

onMounted(() => {
  store.fetch()
  groupsStore.fetch()
})
</script>
