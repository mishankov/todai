<script lang="ts">
	import Today from '$lib/components/Today.svelte';
	import {
		completeTask,
		deleteTask,
		reopenTask,
		type Task,
		type TaskUpdate,
		updateTask
	} from '$lib/tasks/client';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	function complete(taskId: string, version: number): Promise<Task> {
		return completeTask(fetch, taskId, version);
	}

	function reopen(taskId: string, version: number): Promise<Task> {
		return reopenTask(fetch, taskId, version);
	}

	function remove(taskId: string, version: number): Promise<void> {
		return deleteTask(fetch, taskId, version);
	}

	function update(taskId: string, changes: TaskUpdate): Promise<Task> {
		return updateTask(fetch, taskId, changes);
	}
</script>

<svelte:head>
	<title>Today — Todai</title>
</svelte:head>

<Today initialTasks={data.tasks} projects={data.projects} {complete} {reopen} {update} {remove} />
