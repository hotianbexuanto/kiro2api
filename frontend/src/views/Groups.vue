<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl font-semibold text-gray-800">分组管理</h1>
      <div class="flex items-center gap-2">
        <button
          class="flex items-center px-3 py-2 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
          :disabled="store.loading"
          @click="store.fetch"
        >
          <Icon name="refresh" :size="16" color="currentColor" class="mr-1.5" :class="{ 'animate-spin': store.loading }" />
          刷新
        </button>
        <button
          class="flex items-center px-4 py-2 text-sm font-medium text-white btn-primary rounded-lg"
          @click="showCreateModal = true"
        >
          <Icon name="plus" :size="16" color="#ffffff" class="mr-2" />
          创建分组
        </button>
      </div>
    </div>

    <div v-if="store.loading" class="text-center py-12 text-gray-400">
      加载中...
    </div>
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="group in editableGroups"
        :key="group.name"
        class="card p-4 hover:shadow-sm transition-all"
        :class="group.name === 'default'
          ? 'bg-blue-50/50 border-blue-200 hover:border-blue-300'
          : 'hover:border-gray-300'"
      >
        <div class="flex items-center justify-between mb-3">
          <div>
            <h3 class="font-medium text-gray-800">{{ group.display_name || group.name }}</h3>
            <p class="text-sm text-gray-400">{{ group.name }}</p>
          </div>
          <div v-if="group.name !== 'default'" class="flex items-center space-x-1">
            <button
              class="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-50 rounded-lg transition-all"
              title="重命名"
              @click="openRename(group)"
            >
              <Icon name="edit" :size="16" color="currentColor" />
            </button>
            <button
              class="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-all"
              title="删除"
              @click="openDelete(group)"
            >
              <Icon name="trash" :size="16" color="currentColor" />
            </button>
          </div>
          <span v-else class="text-xs text-blue-600 bg-blue-100 px-2 py-1 rounded">系统默认</span>
        </div>
        <div class="flex items-center space-x-4 text-sm text-gray-500">
          <span>Token: <span class="font-medium text-gray-700">{{ group.token_count }}</span></span>
          <span>活跃: <span class="font-medium text-green-600">{{ group.active_count }}</span></span>
        </div>
        <div v-if="group.settings" class="mt-2 text-xs text-gray-400">
          <span v-if="group.settings.priority">优先级: {{ group.settings.priority }}</span>
          <span v-if="group.settings.disabled" class="ml-2 text-red-500">已禁用</span>
        </div>
      </div>
    </div>

    <!-- 创建分组 -->
    <Modal :visible="showCreateModal" title="创建分组" @close="showCreateModal = false">
      <form @submit.prevent="handleCreate">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">分组名称 *</label>
            <input
              v-model="createForm.name"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              placeholder="英文标识，如 production"
              required
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">显示名称</label>
            <input
              v-model="createForm.displayName"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              placeholder="中文名称，如 生产环境"
            />
          </div>
        </div>
        <div class="mt-6 flex justify-end space-x-3">
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            @click="showCreateModal = false"
          >
            取消
          </button>
          <button
            type="submit"
            class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg"
          >
            创建
          </button>
        </div>
      </form>
    </Modal>

    <!-- 重命名分组 -->
    <Modal :visible="showRenameModal" title="重命名分组" @close="showRenameModal = false">
      <form @submit.prevent="handleRename">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">新名称 *</label>
            <input
              v-model="renameForm.newName"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              required
            />
          </div>
        </div>
        <div class="mt-6 flex justify-end space-x-3">
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            @click="showRenameModal = false"
          >
            取消
          </button>
          <button
            type="submit"
            class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg"
          >
            确认
          </button>
        </div>
      </form>
    </Modal>

    <!-- 删除确认 -->
    <ConfirmDialog
      :visible="showDeleteConfirm"
      title="确认删除"
      :message="`确定要删除分组「${groupToDelete?.name}」吗？该分组下的 Token 将被移至 default 分组。`"
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useGroupsStore } from '@/stores/groups'
import { useToast } from '@/composables/useToast'
import Modal from '@/components/Modal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import Icon from '@/components/Icon.vue'
import type { Group } from '@/types'

const store = useGroupsStore()
const toast = useToast()

// 过滤掉特殊分组（banned/exhausted），default 排第一
const editableGroups = computed(() => {
  const groups = store.groups.filter(g => g.name !== 'banned' && g.name !== 'exhausted')
  return groups.sort((a, b) => {
    if (a.name === 'default') return -1
    if (b.name === 'default') return 1
    return 0
  })
})

const showCreateModal = ref(false)
const showRenameModal = ref(false)
const showDeleteConfirm = ref(false)
const groupToDelete = ref<Group | null>(null)
const groupToRename = ref<Group | null>(null)

const createForm = ref({ name: '', displayName: '' })
const renameForm = ref({ newName: '' })

async function handleCreate() {
  if (!createForm.value.name.trim()) {
    toast.error('请输入分组名称')
    return
  }
  try {
    await store.create(createForm.value.name.trim(), createForm.value.displayName.trim() || undefined)
    toast.success('分组已创建')
    showCreateModal.value = false
    createForm.value = { name: '', displayName: '' }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '创建失败')
  }
}

function openRename(group: Group) {
  groupToRename.value = group
  renameForm.value.newName = group.name
  showRenameModal.value = true
}

async function handleRename() {
  if (!groupToRename.value || !renameForm.value.newName.trim()) return
  try {
    await store.rename(groupToRename.value.name, renameForm.value.newName.trim())
    toast.success('分组已重命名')
    showRenameModal.value = false
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '重命名失败')
  }
}

function openDelete(group: Group) {
  groupToDelete.value = group
  showDeleteConfirm.value = true
}

async function handleDelete() {
  if (!groupToDelete.value) return
  try {
    await store.remove(groupToDelete.value.name)
    toast.success('分组已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    showDeleteConfirm.value = false
    groupToDelete.value = null
  }
}

onMounted(() => {
  store.fetch()
})
</script>
