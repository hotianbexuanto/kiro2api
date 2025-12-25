<template>
  <Modal :visible="visible" :title="title" @close="$emit('cancel')">
    <p class="text-sm text-gray-500">{{ message }}</p>
    <template #footer>
      <button
        class="px-4 py-2.5 text-sm font-medium text-gray-600 bg-gray-100 rounded-lg hover:bg-gray-200 transition-all"
        @click="$emit('cancel')"
      >
        取消
      </button>
      <button
        class="px-4 py-2.5 text-sm font-medium text-white bg-red-500 rounded-lg hover:bg-red-600 transition-all shadow-sm"
        @click="$emit('confirm')"
      >
        确认
      </button>
    </template>
  </Modal>
</template>

<script setup lang="ts">
import Modal from './Modal.vue'

defineProps<{
  visible: boolean
  title?: string
  message: string
}>()

defineEmits<{
  confirm: []
  cancel: []
}>()
</script>
