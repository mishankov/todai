<script lang="ts">
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import type { Project } from '$lib/projects/client';
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
	const date = new Intl.DateTimeFormat(undefined, {
		weekday: 'long',
		month: 'long',
		day: 'numeric'
	}).format(new Date());
</script>

<TaskView
	{initialTasks}
	{complete}
	{reopen}
	{update}
	{remove}
	{projects}
	eyebrow={date}
	heading="Today"
	countNoun="remaining"
	emptyTitle="Nothing due today."
	emptyMessage="Enjoy the space, or add a due date to a task in Inbox."
	listLabel="Today tasks"
/>
