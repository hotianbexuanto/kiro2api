<template>
  <div class="min-h-screen flex bg-[var(--bg-warm)]">
    <Sidebar />
    <main class="flex-1 p-6">
      <router-view />
    </main>
    <Toast />
  </div>
</template>

<script setup lang="ts">
import Sidebar from './Sidebar.vue'
import Toast from './Toast.vue'
</script>
