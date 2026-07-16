<script lang="ts">
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import type { Project } from '$lib/projects/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: Task[];
		create: (title: string) => Promise<Task>;
		complete: (taskId: string) => Promise<Task>;
		reopen: (taskId: string) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string) => Promise<void>;
		projects?: Project[];
	}

	let { initialTasks, create, complete, reopen, update, remove, projects = [] }: Props = $props();
</script>

<TaskView
	{initialTasks}
	{create}
	{complete}
	{reopen}
	{update}
	{remove}
	{projects}
	currentProjectId={null}
	eyebrow="My tasks"
	heading="Inbox"
	emptyTitle="Inbox clear."
	emptyMessage="Add something above when it needs your attention."
	listLabel="Inbox tasks"
/>
