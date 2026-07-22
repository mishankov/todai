<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { untrack } from 'svelte';
	import RichTaskTitle from './RichTaskTitle.svelte';
	import TaskPropertyPickers from './TaskPropertyPickers.svelte';

	interface Props {
		projects: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		initialTitle?: string;
		initialProjectId?: string;
		initialSectionId?: string | null;
		initialPriority?: number;
		initialDueDate?: string | null;
		initialDueTime?: string | null;
		initialDueTimezone?: string | null;
		showControls?: boolean;
	}

	let {
		projects,
		sections = [],
		loadSections,
		initialTitle = '',
		initialProjectId = projects[0]?.id ?? '',
		initialSectionId = null,
		initialPriority = 0,
		initialDueDate = null,
		initialDueTime = null,
		initialDueTimezone = null,
		showControls = false
	}: Props = $props();
	let title = $state(untrack(() => initialTitle));
	let projectId = $state(untrack(() => initialProjectId));
	let sectionId = $state<string | null>(untrack(() => initialSectionId));
	let priority = $state(untrack(() => initialPriority));
	let dueDate = $state<string | null>(untrack(() => initialDueDate));
	let dueTime = $state<string | null>(untrack(() => initialDueTime));
	let dueTimezone = $state<string | null>(untrack(() => initialDueTimezone));
</script>

<RichTaskTitle
	bind:title
	bind:projectId
	bind:sectionId
	bind:priority
	bind:dueDate
	bind:dueTime
	bind:dueTimezone
	{projects}
	{sections}
	{loadSections}
/>

{#if showControls}
	<TaskPropertyPickers
		bind:projectId
		bind:sectionId
		bind:priority
		bind:dueDate
		bind:dueTime
		bind:dueTimezone
		{projects}
		{sections}
		{loadSections}
	/>
{/if}

<output data-testid="rich-title-draft">
	{JSON.stringify({ title, projectId, sectionId, priority, dueDate, dueTime, dueTimezone })}
</output>
