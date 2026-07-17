<script lang="ts">
	import {
		completeTask,
		createTask,
		deleteTask,
		reopenTask,
		type Task,
		type TaskUpdate,
		updateTask
	} from '$lib/tasks/client';
	import Inbox from '$lib/components/Inbox.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	function create(title: string): Promise<Task> {
		return createTask(fetch, title);
	}

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
	<title>Inbox — Todai</title>
</svelte:head>

<Inbox
	initialTasks={data.tasks}
	projects={data.projects}
	{create}
	{complete}
	{reopen}
	{update}
	{remove}
/>
