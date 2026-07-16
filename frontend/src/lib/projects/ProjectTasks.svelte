<script lang="ts">
	import TaskView from '$lib/components/TaskView.svelte';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import type { Project } from './client';

	interface Props {
		project: Project;
		projects: Project[];
		initialTasks: Task[];
		create: (title: string) => Promise<Task>;
		complete: (taskId: string) => Promise<Task>;
		reopen: (taskId: string) => Promise<Task>;
		update: (taskId: string, changes: TaskUpdate) => Promise<Task>;
		remove: (taskId: string) => Promise<void>;
	}

	let { project, projects, initialTasks, create, complete, reopen, update, remove }: Props =
		$props();
</script>

<TaskView
	{initialTasks}
	{projects}
	{create}
	{complete}
	{reopen}
	{update}
	{remove}
	currentProjectId={project.id}
	eyebrow="Project"
	heading={project.name}
	emptyTitle="No tasks here yet."
	emptyMessage="Add the first task above, or move one here from Inbox."
	listLabel={`${project.name} tasks`}
/>
