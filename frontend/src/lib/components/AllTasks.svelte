<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import type { Task, TaskSummary, TaskUpdate } from '$lib/tasks/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: TaskSummary[];
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		currentProjectId?: string;
	}

	let {
		initialTasks,
		complete,
		reopen,
		update,
		remove,
		projects = [],
		sections = [],
		loadSections,
		currentProjectId
	}: Props = $props();
</script>

<TaskView
	{initialTasks}
	{complete}
	{reopen}
	{update}
	{remove}
	{projects}
	{sections}
	{loadSections}
	{currentProjectId}
	eyebrow="Overview"
	heading="All tasks"
	emptyTitle="No tasks yet."
	emptyMessage="Tasks from this project's Inbox and sections will appear here."
	listLabel="All tasks"
/>
