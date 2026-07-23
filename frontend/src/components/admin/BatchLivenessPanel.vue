<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLivenessStore } from '@/stores'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const liveness = useLivenessStore()
const appStore = useAppStore()
const task = computed(() => liveness.task)
const finished = computed(() => task.value?.state === 'completed')

async function deleteFailed() {
  if (!task.value || !confirm(t('admin.accounts.livenessDeleteConfirm', { count: task.value.dead }))) return
  try {
    const result = await liveness.deleteFailed()
    if (result) appStore.showSuccess(t('admin.accounts.livenessDeleteResult', result))
  } catch (error) {
    appStore.showError(error instanceof Error ? error.message : t('common.unknownError'))
  }
}
</script>

<template>
  <Teleport to="body">
    <section v-if="liveness.visible && task" class="fixed bottom-5 right-5 z-[10000] w-[min(390px,calc(100vw-2rem))] overflow-hidden rounded-lg border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-800" :class="{ 'w-[min(250px,calc(100vw-2rem))]': liveness.minimized }" aria-live="polite">
      <header class="flex items-center gap-3 border-b border-gray-100 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900">
        <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300"><Icon name="play" size="sm" :class="{ 'animate-pulse': liveness.isRunning }" /></span>
        <div class="min-w-0 flex-1"><div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.livenessPanelTitle') }}</div><div class="text-xs text-gray-500 dark:text-gray-400">{{ finished ? t('admin.accounts.livenessCompleted') : t('admin.accounts.livenessRunning') }}</div></div>
        <button class="rounded p-1.5 text-gray-500 hover:bg-gray-200 dark:hover:bg-dark-700" :title="t('admin.accounts.livenessMinimize')" @click="liveness.toggleMinimized"><Icon name="minus" size="sm" /></button>
        <button class="rounded p-1.5 text-gray-500 hover:bg-gray-200 dark:hover:bg-dark-700" :title="t('common.close')" @click="liveness.close"><Icon name="x" size="sm" /></button>
      </header>
      <div v-if="liveness.minimized" class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ task.completed }}/{{ task.total }} · {{ task.dead }} {{ t('admin.accounts.livenessDeadShort') }}</div>
      <div v-else class="space-y-4 p-4">
        <div><div class="mb-2 flex items-baseline justify-between text-sm"><span class="font-medium text-gray-800 dark:text-gray-100">{{ task.completed }} / {{ task.total }}</span><span class="text-gray-500 dark:text-gray-400">{{ liveness.progress }}%</span></div><div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"><div class="h-full bg-primary-500 transition-all duration-500" :style="{ width: `${liveness.progress}%` }"></div></div><p v-if="liveness.isRunning && task.current_account_id" class="mt-2 truncate text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.livenessCurrent', { id: task.current_account_id }) }}</p></div>
        <div class="grid grid-cols-3 gap-2 text-center"><div class="rounded-md bg-emerald-50 px-2 py-2 dark:bg-emerald-900/20"><div class="text-lg font-semibold text-emerald-600">{{ task.alive }}</div><div class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('admin.accounts.livenessAlive') }}</div></div><div class="rounded-md bg-rose-50 px-2 py-2 dark:bg-rose-900/20"><div class="text-lg font-semibold text-rose-600">{{ task.dead }}</div><div class="text-xs text-rose-700 dark:text-rose-300">{{ t('admin.accounts.livenessDead') }}</div></div><div class="rounded-md bg-gray-100 px-2 py-2 dark:bg-dark-700"><div class="text-lg font-semibold text-gray-700 dark:text-gray-100">{{ Math.max(task.total - task.completed, 0) }}</div><div class="text-xs text-gray-600 dark:text-gray-300">{{ t('admin.accounts.livenessPending') }}</div></div></div>
        <div v-if="task.recent.length" class="max-h-44 overflow-y-auto border-t border-gray-100 pt-3 dark:border-dark-700"><div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accounts.livenessRecent') }}</div><div v-for="item in task.recent" :key="`${item.account_id}-${item.status}`" class="flex items-center gap-2 py-1 text-xs"><span class="h-2 w-2 rounded-full" :class="item.status === 'alive' ? 'bg-emerald-500' : 'bg-rose-500'"></span><span class="w-16 shrink-0 text-gray-500">#{{ item.account_id }}</span><span class="truncate" :class="item.status === 'alive' ? 'text-emerald-700 dark:text-emerald-300' : 'text-rose-700 dark:text-rose-300'">{{ item.status === 'alive' ? t('admin.accounts.livenessAlive') : item.error }}</span></div></div>
        <p v-if="liveness.error" class="text-xs text-rose-600">{{ liveness.error }}</p>
        <button v-if="finished && task.dead > task.deleted" class="btn btn-danger w-full" :disabled="liveness.deleting" @click="deleteFailed"><Icon name="trash" size="sm" />{{ liveness.deleting ? t('common.processing') : t('admin.accounts.livenessDeleteFailed', { count: task.dead - task.deleted }) }}</button>
        <p v-else-if="finished" class="text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.livenessFinishedHint') }}</p>
      </div>
    </section>
  </Teleport>
</template>
