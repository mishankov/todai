<script lang="ts">
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: Task[];
		complete: (taskId: string) => Promise<Task>;
		reopen: (taskId: string) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string) => Promise<void>;
	}

	let { initialTasks, complete, reopen, update, remove }: Props = $props();
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
	eyebrow={date}
	heading="Today"
	countNoun="remaining"
	emptyTitle="Nothing due today."
	emptyMessage="Enjoy the space, or add a due date to a task in Inbox."
	listLabel="Today tasks"
/>
