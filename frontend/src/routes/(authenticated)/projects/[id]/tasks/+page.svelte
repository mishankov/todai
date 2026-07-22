<script lang="ts">
	import ProjectTasks from '$lib/projects/ProjectTasks.svelte';
	import {
		createProjectSection,
		deleteProjectSection,
		reorderProjectSection,
		type Project,
		type ProjectLayout,
		type ProjectSection,
		updateProject,
		updateProjectSection
	} from '$lib/projects/client';
	import {
		completeTask,
		createTaskWithProperties,
		deleteTask,
		reopenTask,
		reorderTask,
		type Task,
		type TaskCreateDraft,
		type TaskSummary
	} from '$lib/tasks/client';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	function create(draft: TaskCreateDraft): Promise<Task> {
		return createTaskWithProperties(fetch, draft);
	}
	function complete(taskId: string, version: number): Promise<Task> {
		return completeTask(fetch, taskId, version);
	}
	function reopen(taskId: string, version: number): Promise<Task> {
		return reopenTask(fetch, taskId, version);
	}
	function remove(taskId: string, version: number): Promise<void> {
		return deleteTask(fetch, taskId, version);
	}
	function reorder(
		taskId: string,
		version: number,
		sectionId: string | null,
		beforeTaskId: string | null
	): Promise<TaskSummary[]> {
		return reorderTask(fetch, taskId, version, sectionId, beforeTaskId);
	}
	function changeLayout(version: number, layout: ProjectLayout): Promise<Project> {
		return updateProject(fetch, data.project.id, { version, layout });
	}
	function createSection(name: string): Promise<ProjectSection> {
		return createProjectSection(fetch, data.project.id, name);
	}
	function updateSection(
		sectionId: string,
		version: number,
		name: string
	): Promise<ProjectSection> {
		return updateProjectSection(fetch, data.project.id, sectionId, version, name);
	}
	function deleteSection(sectionId: string, version: number): Promise<void> {
		return deleteProjectSection(fetch, data.project.id, sectionId, version);
	}
	function reorderSection(
		sectionId: string,
		version: number,
		beforeSectionId: string | null
	): Promise<ProjectSection[]> {
		return reorderProjectSection(fetch, data.project.id, sectionId, version, beforeSectionId);
	}
</script>

<svelte:head><title>Tasks · {data.project.name} — Todai</title></svelte:head>

{#key data.project.id}
	<ProjectTasks
		project={data.project}
		initialSections={data.sections}
		initialTasks={data.tasks}
		{create}
		{complete}
		{reopen}
		{remove}
		{reorder}
		{changeLayout}
		{createSection}
		{updateSection}
		{deleteSection}
		{reorderSection}
	/>
{/key}
