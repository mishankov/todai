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
		loadSections
	}: Props = $props();
	let cachedSections = $state<ProjectSection[]>([]);
	let loadedProjectIds = $state<string[]>([]);
	let loadingProjectIds = $state<string[]>([]);
	let recentProjectIds = $state<string[]>([]);
	let recentSectionIds = $state<string[]>([]);
	let dateReference = $state(new Date());
	let locationOpen = $state(false);
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
	let priorityItems = $derived<QuickPickItem[]>(
		priorityOptions.map((option) => ({ id: String(option.value), label: option.label }))
	);
	let projectName = $derived(
		projects.find((project) => project.id === projectId)?.name ?? 'Choose project'
	);
	let sectionName = $derived(
		sectionId === null
			? 'No section (Inbox)'
			: (currentSections.find((section) => section.id === sectionId)?.name ?? 'Choose section')
	);
	let locationName = $derived(`${projectName} / ${sectionName}`);
	let priorityName = $derived(
		priorityOptions.find((option) => option.value === priority)?.label ?? 'None'
	);
	let dateName = $derived(dueDate ? formatDate(dueDate) : 'No date');
	let timeName = $derived(dueTime ? formatTime(dueTime) : '+ Time');

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

	function choosePriority(value: string) {
		priority = Number(value);
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
	class="property-bar"
	role="group"
	aria-label="Task properties"
	data-timezone={dueTimezone ?? undefined}
>
	<div
		class="property-location"
		onfocusout={(event) => {
			const location = event.currentTarget;
			window.setTimeout(() => {
				if (!location.contains(document.activeElement)) locationOpen = false;
			}, 0);
		}}
	>
		<button
			class="property-trigger"
			type="button"
			aria-label={`Location: ${locationName}`}
			aria-haspopup="dialog"
			aria-expanded={locationOpen}
			onkeydown={(event) => {
				if (event.key === 'Escape' && locationOpen) {
					event.preventDefault();
					locationOpen = false;
				}
			}}
			onclick={() => (locationOpen = !locationOpen)}
		>
			<svg aria-hidden="true" viewBox="0 0 24 24">
				<path d="M3.5 7.5h6l1.7 2h9.3v9h-17zM3.5 7.5V5h6l1.7 2.5" />
			</svg>
			<span>{locationName}</span>
			<svg class="chevron" aria-hidden="true" viewBox="0 0 24 24">
				<path d="m8 10 4 4 4-4" />
			</svg>
		</button>

		{#if locationOpen}
			<div class="location-popover" role="dialog" aria-label="Task location">
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
		{/if}
	</div>

	<QuickPick
		label="Priority"
		buttonText={priorityName}
		items={priorityItems}
		value={String(priority)}
		select={choosePriority}
		variant="segment"
		prefix="Priority:"
		icon="flag"
	/>
	<QuickPick
		label="Due date"
		buttonText={dateName}
		items={dateItems}
		value={dueDate ?? '__no_date__'}
		select={chooseDate}
		customInput="date"
		customValue={dueDate ?? ''}
		refresh={() => (dateReference = new Date())}
		variant="segment"
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
		variant="segment"
		icon="clock"
	/>
</div>

<style>
	.property-bar {
		display: grid;
		grid-template-columns: minmax(0, 1.45fr) minmax(9rem, 1fr) minmax(9.5rem, 1fr) minmax(
				6.75rem,
				0.7fr
			);
		min-width: 0;
		min-height: 2.85rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.75rem;
		background: #fff;
	}
	.property-location {
		position: relative;
		min-width: 0;
	}
	.property-trigger {
		display: flex;
		width: 100%;
		min-width: 0;
		min-height: 2.75rem;
		align-items: center;
		gap: 0.48rem;
		padding: 0.45rem 0.68rem;
		border: 0;
		border-radius: 0;
		color: #29332c;
		background: transparent;
		font: inherit;
		font-size: 0.78rem;
		font-weight: 600;
		text-align: left;
		cursor: pointer;
	}
	.property-trigger > span {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.property-trigger svg {
		width: 1rem;
		height: 1rem;
		flex: none;
		fill: none;
		stroke: #657269;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}
	.property-trigger .chevron {
		width: 0.85rem;
		height: 0.85rem;
	}
	.property-trigger:hover,
	.property-trigger:focus-visible {
		background: var(--theme-hover, #f3f7f2);
		outline: none;
	}
	.location-popover {
		position: absolute;
		z-index: 5;
		top: calc(100% + 0.45rem);
		left: 0;
		display: grid;
		width: min(19rem, calc(100vw - 3rem));
		gap: 0.7rem;
		padding: 0.8rem;
		border: 1px solid var(--theme-border, #ccd6ca);
		border-radius: 0.75rem;
		background: #fff;
		box-shadow: 0 0.8rem 2.2rem color-mix(in srgb, var(--theme-accent, #2d6540) 14%, transparent);
	}
	@media (max-width: 46rem) {
		.property-bar {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
			gap: 0.45rem;
			border: 0;
			background: transparent;
		}
		.property-location {
			border: 1px solid var(--theme-border, #ccd6ca);
			border-radius: 0.7rem;
			background: #fff;
		}
	}
	@media (max-width: 28rem) {
		.property-bar {
			grid-template-columns: 1fr;
		}
	}
</style>
