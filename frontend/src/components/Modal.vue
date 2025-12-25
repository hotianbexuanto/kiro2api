<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-150"
      leave-active-class="transition-opacity duration-100"
      enter-from-class="opacity-0"
      leave-to-class="opacity-0"
    >
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" @click="$emit('close')"></div>
        <Transition
          enter-active-class="transition-all duration-150"
          leave-active-class="transition-all duration-100"
          enter-from-class="opacity-0 scale-95"
          leave-to-class="opacity-0 scale-95"
        >
          <div v-if="visible" class="relative card max-w-md w-full p-6">
            <h3 v-if="title" class="text-lg font-semibold text-gray-800 mb-4">{{ title }}</h3>
            <slot></slot>
            <div v-if="$slots.footer" class="mt-6 flex justify-end space-x-3">
              <slot name="footer"></slot>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{
  visible: boolean
  title?: string
}>()

defineEmits<{
  close: []
}>()
</script>
