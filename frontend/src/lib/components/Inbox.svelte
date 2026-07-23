<script lang="ts">
	import type { Task, TaskCreateDraft, TaskSummary } from '$lib/tasks/client';
	import type { Project, ProjectSection } from '$lib/projects/client';
	import TaskView from './TaskView.svelte';

	interface Props {
		initialTasks: TaskSummary[];
		create: (draft: TaskCreateDraft) => Promise<Task>;
		complete: (taskId: string, version: number) => Promise<Task>;
		reopen: (taskId: string, version: number) => Promise<Task>;
		remove: (taskId: string, version: number) => Promise<void>;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		currentProjectId?: string;
		openTask?: (task: Task) => void;
	}

	let {
		initialTasks,
		create,
		complete,
		reopen,
		remove,
		projects = [],
		sections = [],
		loadSections,
		currentProjectId,
		openTask
	}: Props = $props();
</script>

<TaskView
	{initialTasks}
	{create}
	{complete}
	{reopen}
	{remove}
	{projects}
	{sections}
	{loadSections}
	{currentProjectId}
	{openTask}
	currentSectionId={null}
	eyebrow="Project"
	heading="Inbox"
	emptyTitle="Inbox clear."
	emptyMessage="Add something above when it needs your attention."
	listLabel="Inbox tasks"
/>
