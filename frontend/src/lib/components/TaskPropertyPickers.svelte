<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import {
		entityItems,
		formatDate,
		formatTime,
		priorityOptions,
		readRecentIds,
		relativeDateOptions,
		rememberRecentId,
		timeOptions,
		type QuickPickItem
	} from '$lib/tasks/quick-picks';
	import { onMount, untrack } from 'svelte';
	import QuickPick from './QuickPick.svelte';

	interface Props {
		projectId: string;
		sectionId: string | null;
		priority: number;
		dueDate: string | null;
		dueTime: string | null;
		dueTimezone: string | null;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		compact?: boolean;
	}

	let {
		projectId = $bindable(),
		sectionId = $bindable(),
		priority = $bindable(),
		dueDate = $bindable(),
		dueTime = $bindable(),
		dueTimezone = $bindable(),
		projects = [],
		sections = [],
		loadSections,
		compact = false
	}: Props = $props();
	let cachedSections = $state<ProjectSection[]>([]);
	let loadedProjectIds = $state<string[]>([]);
	let loadingProjectIds = $state<string[]>([]);
	let recentProjectIds = $state<string[]>([]);
	let recentSectionIds = $state<string[]>([]);
	let dateReference = $state(new Date());
	const projectStorageKey = 'todai.quick-picks.recent-projects';
	const sectionStorageKeyPrefix = 'todai.quick-picks.recent-sections';
	let currentSections = $derived(
		cachedSections.filter((section) => section.projectId === projectId)
	);
	let projectItems = $derived(entityItems(projects, projectId, recentProjectIds));
	let sectionItems = $derived<QuickPickItem[]>([
		{ id: '__inbox__', label: 'No section (Inbox)' },
		...entityItems(currentSections, sectionId ?? '', recentSectionIds)
	]);
	let relativeDates = $derived(relativeDateOptions(dateReference));
	let dateItems = $derived<QuickPickItem[]>([
		...relativeDates.map((option) => ({
			id: option.value,
			label: option.label,
			detail: formatDate(option.date)
		})),
		{ id: '__no_date__', label: 'No date' },
		{ id: '__custom_date__', label: 'Choose date…', custom: true }
	]);
	let dueTimeItems = $derived<QuickPickItem[]>([
		...timeOptions.map((option) => ({
			id: option.value,
			label: option.label,
			detail: formatTime(option.value)
		})),
		{ id: '__no_time__', label: 'No time' },
		{ id: '__custom_time__', label: 'Choose time…', custom: true }
	]);
	let projectName = $derived(
		projects.find((project) => project.id === projectId)?.name ?? 'Choose project'
	);
	let sectionName = $derived(
		sectionId === null
			? 'No section (Inbox)'
			: (currentSections.find((section) => section.id === sectionId)?.name ?? 'Choose section')
	);
	let dateName = $derived(dueDate ? formatDate(dueDate) : 'No date');
	let timeName = $derived(dueTime ? formatTime(dueTime) : 'No time');

	$effect(() => {
		const incoming = sections;
		untrack(() => mergeSections(incoming));
	});

	$effect(() => {
		const selectedProject = projectId;
		if (selectedProject) void ensureSections(selectedProject);
	});

	$effect(() => {
		const selectedProject = projectId;
		const availableSectionIds = currentSections.map((section) => section.id);
		const loaded = loadedProjectIds.includes(selectedProject);
		if (!selectedProject || !loaded) return;
		untrack(() => {
			recentSectionIds = readRecentIds(
				localStorage,
				sectionStorageKey(selectedProject),
				availableSectionIds
			);
		});
	});

	onMount(() => {
		recentProjectIds = readRecentIds(
			localStorage,
			projectStorageKey,
			projects.map((project) => project.id)
		);
	});

	function sectionStorageKey(selectedProjectId: string): string {
		return `${sectionStorageKeyPrefix}.${selectedProjectId}`;
	}

	function mergeSections(nextSections: ProjectSection[]) {
		if (nextSections.length === 0) return;
		const merged = [...cachedSections];
		for (const section of nextSections) {
			const index = merged.findIndex((candidate) => candidate.id === section.id);
			if (index === -1) merged.push(section);
			else merged[index] = section;
		}
		cachedSections = merged;
		for (const section of nextSections) {
			if (!loadedProjectIds.includes(section.projectId)) {
				loadedProjectIds = [...loadedProjectIds, section.projectId];
			}
		}
	}

	async function ensureSections(selectedProjectId: string) {
		if (
			!loadSections ||
			loadedProjectIds.includes(selectedProjectId) ||
			loadingProjectIds.includes(selectedProjectId)
		)
			return;
		loadingProjectIds = [...loadingProjectIds, selectedProjectId];
		try {
			const loaded = await loadSections(selectedProjectId);
			cachedSections = [
				...cachedSections.filter((section) => section.projectId !== selectedProjectId),
				...loaded
			];
			loadedProjectIds = [...loadedProjectIds, selectedProjectId];
		} finally {
			loadingProjectIds = loadingProjectIds.filter((id) => id !== selectedProjectId);
		}
	}

	function chooseProject(value: string) {
		if (value === projectId) return;
		projectId = value;
		sectionId = null;
		recentSectionIds = [];
		recentProjectIds = rememberRecentId(localStorage, projectStorageKey, value);
		void ensureSections(value);
	}

	function chooseSection(value: string) {
		sectionId = value === '__inbox__' ? null : value;
		if (sectionId) {
			recentSectionIds = rememberRecentId(localStorage, sectionStorageKey(projectId), sectionId);
		}
	}

	function chooseDate(value: string) {
		if (value === '__no_date__') {
			dueDate = null;
			dueTime = null;
			dueTimezone = null;
			return;
		}
		if (value.startsWith('__')) return;
		dueDate = value;
	}

	function chooseTime(value: string) {
		if (value === '__no_time__') {
			dueTime = null;
			dueTimezone = null;
			return;
		}
		if (value.startsWith('__') || !dueDate) return;
		dueTime = value;
		dueTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	}
</script>

<div
	class:compact
	class="task-property-pickers"
	role="group"
	aria-label="Task properties"
	data-timezone={dueTimezone ?? undefined}
>
	<div class="location-pickers">
		<QuickPick
			label="Project"
			buttonText={projectName}
			items={projectItems}
			value={projectId}
			select={chooseProject}
			searchable
			searchPlaceholder="Search projects"
		/>
		<QuickPick
			label="Section"
			buttonText={loadingProjectIds.includes(projectId) ? 'Loading sections…' : sectionName}
			items={sectionItems}
			value={sectionId ?? '__inbox__'}
			select={chooseSection}
			searchable
			searchPlaceholder="Search sections"
		/>
	</div>

	<div class="priority-picker" role="radiogroup" aria-label="Priority">
		{#each priorityOptions as option (option.value)}
			<button
				type="button"
				class:active={priority === option.value}
				class={`priority-${option.value}`}
				role="radio"
				aria-checked={priority === option.value}
				aria-label={`Priority: ${option.label}`}
				title={option.label}
				onclick={() => (priority = option.value)}
			>
				<span class="priority-dot" aria-hidden="true"></span>
				<span>{option.label}</span>
			</button>
		{/each}
	</div>

	<div class="schedule-pickers">
		<QuickPick
			label="Due date"
			buttonText={dateName}
			items={dateItems}
			value={dueDate ?? '__no_date__'}
			select={chooseDate}
			customInput="date"
			customValue={dueDate ?? ''}
			refresh={() => (dateReference = new Date())}
		/>
		<QuickPick
			label="Due time"
			buttonText={timeName}
			items={dueTimeItems}
			value={dueTime ?? '__no_time__'}
			select={chooseTime}
			customInput="time"
			customValue={dueTime ?? ''}
			disabled={!dueDate}
			align="end"
		/>
	</div>
</div>

<style>
	.task-property-pickers {
		display: grid;
		gap: 0.65rem;
	}
	.location-pickers,
	.schedule-pickers {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.55rem;
	}
	.priority-picker {
		display: grid;
		grid-template-columns: repeat(5, minmax(0, 1fr));
		overflow: hidden;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.65rem;
	}
	.priority-picker button {
		display: flex;
		min-width: 0;
		min-height: 2.25rem;
		align-items: center;
		justify-content: center;
		gap: 0.35rem;
		padding: 0.42rem;
		border: 0;
		border-left: 1px solid var(--theme-border, #e0e6df);
		color: #59645c;
		background: #fff;
		font: inherit;
		font-size: 0.7rem;
		font-weight: 650;
		cursor: pointer;
	}
	.priority-picker button:first-child {
		border-left: 0;
	}
	.priority-picker button:hover,
	.priority-picker button:focus-visible {
		background: var(--theme-hover, #f3f7f2);
		outline: none;
	}
	.priority-picker button.active {
		color: #1f2c23;
		background: var(--theme-hover, #edf5ed);
		box-shadow: inset 0 -2px var(--theme-accent, #477d56);
	}
	.priority-dot {
		width: 0.42rem;
		height: 0.42rem;
		flex: none;
		border: 1.5px solid currentColor;
		border-radius: 50%;
	}
	.priority-1 .priority-dot {
		color: #66836f;
	}
	.priority-2 .priority-dot {
		color: #a3822a;
	}
	.priority-3 .priority-dot {
		color: #c16a28;
	}
	.priority-4 .priority-dot {
		color: #bc3e35;
		background: currentColor;
	}
	.compact {
		padding: 0.55rem;
		border: 1px solid #e4e8e3;
		border-radius: 0.75rem;
		background: #fafbf9;
	}
	@container (min-width: 40rem) {
		.task-property-pickers {
			grid-template-columns: repeat(4, minmax(0, 1fr));
			align-items: start;
			gap: 0.45rem;
		}
		.location-pickers {
			grid-row: 1;
			grid-column: 1 / 3;
		}
		.schedule-pickers {
			grid-row: 1;
			grid-column: 3 / 5;
		}
		.priority-picker {
			grid-row: 2;
			grid-column: 1 / 3;
		}
	}
	@media (max-width: 38rem) {
		.priority-picker {
			grid-template-columns: repeat(5, minmax(2.4rem, 1fr));
		}
		.priority-picker button span:last-child {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip: rect(0, 0, 0, 0);
		}
	}
	@media (max-width: 25rem) {
		.location-pickers,
		.schedule-pickers {
			grid-template-columns: 1fr;
		}
	}
</style>
