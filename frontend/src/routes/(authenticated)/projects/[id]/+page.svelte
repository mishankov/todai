<script lang="ts">
	import Inbox from '$lib/components/Inbox.svelte';
	import {
		completeTask,
		createTaskWithProperties,
		deleteTask,
		reopenTask,
		type Task,
		type TaskCreateDraft
	} from '$lib/tasks/client';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	function create(draft: TaskCreateDraft): Promise<Task> {
		return createTaskWithProperties(fetch, draft);
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
</script>

<svelte:head>
	<title>Inbox · {data.project.name} — Todai</title>
</svelte:head>

<Inbox
	initialTasks={data.tasks}
	projects={data.projects}
	currentProjectId={data.project.id}
	{create}
	{complete}
	{reopen}
	{remove}
/>
