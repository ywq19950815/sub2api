import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import * as accountsAPI from '@/api/admin/accounts'
import type { BatchLivenessTask, BatchLivenessTestFilters } from '@/api/admin/accounts'

export const useLivenessStore = defineStore('liveness', () => {
	const task = ref<BatchLivenessTask | null>(null)
	const visible = ref(false)
	const minimized = ref(false)
	const deleting = ref(false)
	const error = ref('')
	let pollTimer: ReturnType<typeof setTimeout> | undefined

	const isRunning = computed(() => task.value?.state === 'running')
	const progress = computed(() => task.value?.total ? Math.round((task.value.completed / task.value.total) * 100) : 0)

	function stopPolling() {
		if (pollTimer) clearTimeout(pollTimer)
		pollTimer = undefined
	}

	async function poll() {
		if (!task.value) return
		try {
			task.value = await accountsAPI.getBatchLivenessTask(task.value.id)
			error.value = ''
			if (task.value.state === 'running') pollTimer = setTimeout(poll, 1000)
		} catch (err) {
			error.value = err instanceof Error ? err.message : 'Unable to load liveness progress'
			pollTimer = setTimeout(poll, 3000)
		}
	}

	async function start(filters: BatchLivenessTestFilters) {
		stopPolling()
		error.value = ''
		task.value = await accountsAPI.batchTestLiveness(filters)
		visible.value = true
		minimized.value = false
		void poll()
	}

	async function deleteFailed() {
		if (!task.value || task.value.state !== 'completed') return null
		deleting.value = true
		try {
			const result = await accountsAPI.deleteBatchLivenessFailed(task.value.id)
			await poll()
			return result
		} finally {
			deleting.value = false
		}
	}

	function close() { visible.value = false }
	function open() { visible.value = true }
	function toggleMinimized() { minimized.value = !minimized.value }

	return { task, visible, minimized, deleting, error, isRunning, progress, start, deleteFailed, close, open, toggleMinimized }
})
