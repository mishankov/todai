<script lang="ts">
	import type { Task, TaskSummary } from '$lib/tasks/client';
	import type { Project } from '$lib/projects/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: TaskSummary[];
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		projects?: Project[];
		currentProjectId?: string;
	}

	let { initialTasks, complete, reopen, remove, projects = [], currentProjectId }: Props = $props();
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
	{remove}
	{projects}
	{currentProjectId}
	eyebrow={date}
	heading="Today"
	countNoun="remaining"
	emptyTitle="Nothing due today."
	emptyMessage="Enjoy the space, or add a due date to a task in Inbox."
	listLabel="Today tasks"
/>
