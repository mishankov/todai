<script lang="ts">
	import type { Project } from '$lib/projects/client';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: Task[];
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		projects?: Project[];
	}

	let { initialTasks, complete, reopen, update, remove, projects = [] }: Props = $props();
</script>

<TaskView
	{initialTasks}
	{complete}
	{reopen}
	{update}
	{remove}
	{projects}
	eyebrow="Overview"
	heading="All tasks"
	emptyTitle="No tasks yet."
	emptyMessage="Tasks from Inbox and projects will appear here."
	listLabel="All tasks"
/>
