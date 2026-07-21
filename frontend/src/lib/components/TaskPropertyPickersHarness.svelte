<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { untrack } from 'svelte';
	import TaskPropertyPickers from './TaskPropertyPickers.svelte';

	interface Props {
		projects: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		initialProjectId?: string;
		initialSectionId?: string | null;
		initialPriority?: number;
		initialDueDate?: string | null;
		initialDueTime?: string | null;
		initialDueTimezone?: string | null;
	}

	let {
		projects,
		sections = [],
		loadSections,
		initialProjectId = projects[0]?.id ?? '',
		initialSectionId = null,
		initialPriority = 0,
		initialDueDate = null,
		initialDueTime = null,
		initialDueTimezone = null
	}: Props = $props();
	let projectId = $state(untrack(() => initialProjectId));
	let sectionId = $state<string | null>(untrack(() => initialSectionId));
	let priority = $state(untrack(() => initialPriority));
	let dueDate = $state<string | null>(untrack(() => initialDueDate));
	let dueTime = $state<string | null>(untrack(() => initialDueTime));
	let dueTimezone = $state<string | null>(untrack(() => initialDueTimezone));
</script>

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

<output data-testid="property-draft">
	{JSON.stringify({ projectId, sectionId, priority, dueDate, dueTime, dueTimezone })}
</output>
