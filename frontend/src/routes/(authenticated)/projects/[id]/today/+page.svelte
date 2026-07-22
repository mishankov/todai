<script lang="ts">
	import Today from '$lib/components/Today.svelte';
	import { completeTask, deleteTask, reopenTask, type Task } from '$lib/tasks/client';
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
</script>

<svelte:head><title>Today · {data.project.name} — Todai</title></svelte:head>

<Today
	initialTasks={data.tasks}
	projects={data.projects}
	currentProjectId={data.project.id}
	{complete}
	{reopen}
	{remove}
/>
