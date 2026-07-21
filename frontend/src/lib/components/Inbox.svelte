<script lang="ts">
	import type { Task, TaskSummary, TaskUpdate } from '$lib/tasks/client';
	import type { Project } from '$lib/projects/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: TaskSummary[];
		create: (title: string) => Promise<Task>;
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		projects?: Project[];
		currentProjectId?: string;
	}

	let {
		initialTasks,
		create,
		complete,
		reopen,
		update,
		remove,
		projects = [],
		currentProjectId
	}: Props = $props();
</script>

<TaskView
	{initialTasks}
	{create}
	{complete}
	{reopen}
	{update}
	{remove}
	{projects}
	{currentProjectId}
	currentSectionId={null}
	eyebrow="Project"
	heading="Inbox"
	emptyTitle="Inbox clear."
	emptyMessage="Add something above when it needs your attention."
	listLabel="Inbox tasks"
/>
