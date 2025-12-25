<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-xl font-semibold text-gray-800">API Keys</h1>
      <button
        class="flex items-center px-4 py-2 text-sm font-medium text-white btn-primary rounded-lg"
        @click="showCreateModal = true"
      >
        <Icon name="plus" :size="16" color="#ffffff" class="mr-2" />
        创建 API Key
      </button>
    </div>

    <div v-if="store.loading" class="text-center py-12 text-gray-400">
      加载中...
    </div>
    <div v-else class="card">
      <table class="w-full">
        <thead>
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">Key</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">名称</th>
            <th class="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase">允许的分组</th>
            <th class="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-[var(--border-subtle)]">
          <tr v-for="key in store.keys" :key="key.key" class="hover:bg-gray-50/50">
            <td class="px-4 py-3 font-mono text-sm text-gray-800">{{ key.masked_key }}</td>
            <td class="px-4 py-3 text-sm text-gray-500">{{ key.name || '-' }}</td>
            <td class="px-4 py-3 text-sm">
              <span v-if="!key.allowed_groups || key.allowed_groups.length === 0" class="text-green-600 font-medium">
                全部
              </span>
              <span v-else class="text-gray-500">{{ key.allowed_groups.join(', ') }}</span>
            </td>
            <td class="px-4 py-3 text-right">
              <button
                class="p-2 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-all mr-1"
                title="编辑"
                @click="openEdit(key)"
              >
                <Icon name="edit" :size="16" color="currentColor" />
              </button>
              <button
                class="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-all"
                title="删除"
                @click="openDelete(key)"
              >
                <Icon name="trash" :size="16" color="currentColor" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 创建 API Key -->
    <Modal :visible="showCreateModal" title="创建 API Key" @close="showCreateModal = false">
      <form @submit.prevent="handleCreate">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">名称</label>
            <input
              v-model="createForm.name"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              placeholder="用于标识这个 Key"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">自定义 Key（可选）</label>
            <input
              v-model="createForm.key"
              type="text"
              class="w-full px-3 py-2.5 border border-[var(--border-subtle)] rounded-lg bg-gray-50/50 focus:bg-white focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-all"
              placeholder="留空则自动生成"
            />
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">允许的分组（留空表示全部）</label>
            <div class="flex flex-wrap gap-3 mt-2">
              <label
                v-for="group in groupsStore.groups"
                :key="group.name"
                class="inline-flex items-center cursor-pointer"
              >
                <input
                  type="checkbox"
                  :value="group.name"
                  v-model="createForm.allowed_groups"
                  class="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
                />
                <span class="ml-2 text-sm text-gray-600">{{ group.name }}</span>
              </label>
            </div>
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

    <!-- 编辑 API Key -->
    <Modal :visible="showEditModal" title="编辑 API Key" @close="showEditModal = false">
      <form @submit.prevent="handleEdit">
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-600 mb-1.5">允许的分组（留空表示全部）</label>
            <div class="flex flex-wrap gap-3 mt-2">
              <label
                v-for="group in groupsStore.groups"
                :key="group.name"
                class="inline-flex items-center cursor-pointer"
              >
                <input
                  type="checkbox"
                  :value="group.name"
                  v-model="editForm.allowed_groups"
                  class="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
                />
                <span class="ml-2 text-sm text-gray-600">{{ group.name }}</span>
              </label>
            </div>
          </div>
        </div>
        <div class="mt-6 flex justify-end space-x-3">
          <button
            type="button"
            class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
            @click="showEditModal = false"
          >
            取消
          </button>
          <button
            type="submit"
            class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg"
          >
            保存
          </button>
        </div>
      </form>
    </Modal>

    <!-- 创建成功 -->
    <Modal :visible="showNewKeyModal" title="API Key 已创建" @close="showNewKeyModal = false">
      <div class="text-center">
        <p class="text-sm text-gray-500 mb-4">请复制并保存此 Key，关闭后将无法再次查看完整内容</p>
        <div class="bg-gray-50 border border-[var(--border-subtle)] p-4 rounded-lg font-mono text-sm break-all text-gray-800">
          {{ newKey }}
        </div>
      </div>
      <template #footer>
        <button
          class="px-4 py-2.5 text-sm font-medium text-white btn-primary rounded-lg"
          @click="copyKey"
        >
          复制
        </button>
        <button
          class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
          @click="showNewKeyModal = false"
        >
          关闭
        </button>
      </template>
    </Modal>

    <!-- 删除确认 -->
    <ConfirmDialog
      :visible="showDeleteConfirm"
      title="确认删除"
      :message="`确定要删除 API Key「${keyToDelete?.masked_key}」吗？`"
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useKeysStore } from '@/stores/keys'
import { useGroupsStore } from '@/stores/groups'
import { useToast } from '@/composables/useToast'
import Modal from '@/components/Modal.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import Icon from '@/components/Icon.vue'
import type { APIKey } from '@/types'

const store = useKeysStore()
const groupsStore = useGroupsStore()
const toast = useToast()

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showNewKeyModal = ref(false)
const showDeleteConfirm = ref(false)
const keyToDelete = ref<APIKey | null>(null)
const keyToEdit = ref<APIKey | null>(null)
const newKey = ref('')

const createForm = ref({
  name: '',
  key: '',
  allowed_groups: [] as string[],
})

const editForm = ref({
  allowed_groups: [] as string[],
})

async function handleCreate() {
  try {
    const result = await store.create({
      name: createForm.value.name || undefined,
      key: createForm.value.key || undefined,
      allowed_groups: createForm.value.allowed_groups.length > 0 ? createForm.value.allowed_groups : undefined,
    })
    newKey.value = result.key
    showCreateModal.value = false
    showNewKeyModal.value = true
    createForm.value = { name: '', key: '', allowed_groups: [] }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '创建失败')
  }
}

function openEdit(key: APIKey) {
  keyToEdit.value = key
  editForm.value.allowed_groups = key.allowed_groups ? [...key.allowed_groups] : []
  showEditModal.value = true
}

async function handleEdit() {
  if (!keyToEdit.value) return
  try {
    await store.update(keyToEdit.value.key, editForm.value.allowed_groups)
    toast.success('已更新')
    showEditModal.value = false
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '更新失败')
  }
}

function openDelete(key: APIKey) {
  keyToDelete.value = key
  showDeleteConfirm.value = true
}

async function handleDelete() {
  if (!keyToDelete.value) return
  try {
    await store.remove(keyToDelete.value.key)
    toast.success('已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    showDeleteConfirm.value = false
    keyToDelete.value = null
  }
}

function copyKey() {
  navigator.clipboard.writeText(newKey.value)
  toast.success('已复制到剪贴板')
}

onMounted(() => {
  store.fetch()
  groupsStore.fetch()
})
</script>
