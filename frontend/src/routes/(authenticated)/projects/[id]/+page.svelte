<script lang="ts">
	import ProjectTasks from '$lib/projects/ProjectTasks.svelte';
	import {
		completeTask,
		createTask,
		deleteTask,
		reopenTask,
		type Task,
		type TaskUpdate,
		updateTask
	} from '$lib/tasks/client';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	function create(title: string): Promise<Task> {
		return createTask(fetch, title, data.project.id);
	}
	function complete(taskId: string): Promise<Task> {
		return completeTask(fetch, taskId);
	}
	function reopen(taskId: string): Promise<Task> {
		return reopenTask(fetch, taskId);
	}
	function update(taskId: string, changes: TaskUpdate): Promise<Task> {
		return updateTask(fetch, taskId, changes);
	}
	function remove(taskId: string): Promise<void> {
		return deleteTask(fetch, taskId);
	}
</script>

<svelte:head><title>{data.project.name} — Todai</title></svelte:head>

<ProjectTasks
	project={data.project}
	projects={data.projects}
	initialTasks={data.tasks}
	{create}
	{complete}
	{reopen}
	{update}
	{remove}
/>
