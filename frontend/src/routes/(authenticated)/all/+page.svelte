<script lang="ts">
	import AllTasks from '$lib/components/AllTasks.svelte';
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

	function complete(taskId: string): Promise<Task> {
		return completeTask(fetch, taskId);
	}

	function reopen(taskId: string): Promise<Task> {
		return reopenTask(fetch, taskId);
	}

	function remove(taskId: string): Promise<void> {
		return deleteTask(fetch, taskId);
	}

	function update(taskId: string, changes: TaskUpdate): Promise<Task> {
		return updateTask(fetch, taskId, changes);
	}
</script>

<svelte:head>
	<title>All tasks — Todai</title>
</svelte:head>

<AllTasks
	initialTasks={data.tasks}
	projects={data.projects}
	{complete}
	{reopen}
	{update}
	{remove}
/>
