<script module lang="ts">
	let nextRichTitleId = 0;
	function richTitleId(): number {
		nextRichTitleId += 1;
		return nextRichTitleId;
	}
</script>

<script lang="ts">
	import type { Project, ProjectSection } from '$lib/projects/client';
	import {
		formatDate,
		formatTime,
		priorityOptions,
		relativeDateOptions,
		timeOptions
	} from '$lib/tasks/quick-picks';
	import {
		detectTitleToken,
		filterTitleOptions,
		removeTitleToken,
		type ActiveTitleToken,
		type TitleOption,
		type TitleProperty
	} from '$lib/tasks/rich-title';
	import { onMount, tick, untrack } from 'svelte';

	type PickerKind = TitleProperty | 'due-time';

	interface RichTitleOption extends TitleOption {
		projectId?: string;
		sectionId?: string | null;
	}

	interface Props {
		title: string;
		projectId: string;
		sectionId: string | null;
		priority: number;
		dueDate: string | null;
		dueTime: string | null;
		dueTimezone: string | null;
		projects?: Project[];
		sections?: ProjectSection[];
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		label?: string;
		placeholder?: string;
		disabled?: boolean;
		focusRequest?: number;
		showChips?: boolean;
		hidePresetLocationChips?: boolean;
		locationChipResetKey?: number;
	}

	let {
		title = $bindable(),
		projectId = $bindable(),
		sectionId = $bindable(),
		priority = $bindable(),
		dueDate = $bindable(),
		dueTime = $bindable(),
		dueTimezone = $bindable(),
		projects = [],
		sections = [],
		loadSections,
		label = 'Task title',
		placeholder = 'Add task',
		disabled = false,
		focusRequest = -1,
		showChips = true,
		hidePresetLocationChips = false,
		locationChipResetKey = 0
	}: Props = $props();

	const instanceId = richTitleId();
	const inputId = `rich-task-title-${instanceId}`;
	const listboxId = `rich-task-title-options-${instanceId}`;
	let input: HTMLInputElement;
	let dateInput: HTMLInputElement;
	let timeInput: HTMLInputElement;
	let cachedSections = $state<ProjectSection[]>(untrack(() => [...sections]));
	let loadedProjectIds = $state<string[]>(
		untrack(() => [...new Set(sections.map((item) => item.projectId))])
	);
	let loadingProjectIds = $state<string[]>([]);
	let activeToken = $state<ActiveTitleToken | null>(null);
	let manualPicker = $state<PickerKind | null>(null);
	let dismissedStart = $state<number | null>(null);
	let activeIndex = $state(0);
	let composing = $state(false);
	let panelStyle = $state('');
	let announcement = $state('');
	let explicitLocationTypes = $state<TitleProperty[]>([]);
	let currentSections = $derived(cachedSections.filter((item) => item.projectId === projectId));
	let pickerKind = $derived<PickerKind | null>(activeToken?.type ?? manualPicker);
	let query = $derived(activeToken?.query ?? '');
	let options = $derived(buildOptions(pickerKind));
	let filteredOptions = $derived(filterTitleOptions(options, query));
	let open = $derived(pickerKind !== null);
	let chips = $derived(showChips ? buildChips() : []);

	$effect(() => {
		const incoming = sections;
		untrack(() => mergeSections(incoming));
	});

	$effect(() => {
		const selectedProject = projectId;
		if (selectedProject) void ensureSections(selectedProject);
	});

	$effect(() => {
		const count = filteredOptions.length;
		if (activeIndex >= count) activeIndex = Math.max(0, count - 1);
		if (open) announcement = `${count} option${count === 1 ? '' : 's'} available.`;
	});

	$effect(() => {
		const request = focusRequest;
		if (request >= 0) void tick().then(() => input?.focus());
	});

	$effect(() => {
		if (!hidePresetLocationChips || locationChipResetKey < 0) return;
		untrack(() => (explicitLocationTypes = []));
	});

	onMount(() => {
		const reposition = () => positionPanel();
		document.addEventListener('scroll', reposition, true);
		return () => document.removeEventListener('scroll', reposition, true);
	});

	function mergeSections(incoming: ProjectSection[]) {
		if (incoming.length === 0) return;
		const merged = [...cachedSections];
		for (const section of incoming) {
			const index = merged.findIndex((candidate) => candidate.id === section.id);
			if (index === -1) merged.push(section);
			else merged[index] = section;
		}
		cachedSections = merged;
		loadedProjectIds = [
			...new Set([...loadedProjectIds, ...incoming.map((item) => item.projectId)])
		];
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
				...cachedSections.filter((item) => item.projectId !== selectedProjectId),
				...loaded
			];
			loadedProjectIds = [...new Set([...loadedProjectIds, selectedProjectId])];
		} catch {
			announcement = 'Sections could not be loaded. Inbox is still available.';
		} finally {
			loadingProjectIds = loadingProjectIds.filter((id) => id !== selectedProjectId);
		}
	}

	function buildOptions(kind: PickerKind | null): RichTitleOption[] {
		switch (kind) {
			case 'project':
				return projects.map((project) => ({ id: project.id, label: project.name }));
			case 'section':
				return projects.flatMap((project) => [
					{
						id: JSON.stringify([project.id, null]),
						label: 'Inbox',
						group: project.name,
						projectId: project.id,
						sectionId: null
					},
					...cachedSections
						.filter((section) => section.projectId === project.id)
						.map((section) => ({
							id: JSON.stringify([project.id, section.id]),
							label: section.name,
							group: project.name,
							projectId: project.id,
							sectionId: section.id
						}))
				]);
			case 'priority':
				return [...priorityOptions]
					.reverse()
					.map((option) => ({ id: String(option.value), label: option.label }));
			case 'due':
				return [
					...relativeDateOptions().map((option) => ({
						id: option.value,
						label: option.label,
						detail: formatDate(option.date)
					})),
					{ id: '__no_date__', label: 'No date' },
					{ id: '__custom_date__', label: 'Choose date…', custom: 'date' }
				];
			case 'due-time':
				return [
					...timeOptions.map((option) => ({
						id: option.value,
						label: option.label,
						detail: formatTime(option.value)
					})),
					{ id: '__no_time__', label: 'No time' },
					{ id: '__custom_time__', label: 'Choose time…', custom: 'time' }
				];
			default:
				return [];
		}
	}

	function buildChips(): { type: TitleProperty; label: string }[] {
		const result: { type: TitleProperty; label: string }[] = [];
		if (projectId && (!hidePresetLocationChips || explicitLocationTypes.includes('project')))
			result.push({
				type: 'project',
				label: projects.find((item) => item.id === projectId)?.name ?? projectId
			});
		if (sectionId && (!hidePresetLocationChips || explicitLocationTypes.includes('section')))
			result.push({
				type: 'section',
				label: currentSections.find((item) => item.id === sectionId)?.name ?? sectionId
			});
		if (priority > 0)
			result.push({
				type: 'priority',
				label: priorityOptions.find((item) => item.value === priority)?.label ?? String(priority)
			});
		if (dueDate)
			result.push({
				type: 'due',
				label: `${formatDate(dueDate)}${dueTime ? ` · ${formatTime(dueTime)}` : ''}`
			});
		return result;
	}

	function handleInput() {
		manualPicker = null;
		syncToken();
	}

	function syncToken() {
		const caret = input?.selectionStart ?? title.length;
		const raw = detectTitleToken(title, caret);
		if (!raw || raw.start !== dismissedStart) dismissedStart = null;
		activeToken = detectTitleToken(title, caret, dismissedStart);
		if (activeToken) {
			manualPicker = null;
			activeIndex = 0;
			if (activeToken.type === 'section') loadLocationSections();
			void tick().then(positionPanel);
		}
	}

	function loadLocationSections() {
		for (const project of projects) void ensureSections(project.id);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.isComposing || composing) return;
		if (open && event.key === 'Escape') {
			event.preventDefault();
			event.stopPropagation();
			if (activeToken) {
				dismissedStart = activeToken.start;
				announcement = 'Autocomplete closed. The token will remain literal.';
			}
			activeToken = null;
			manualPicker = null;
			return;
		}
		if (open && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
			event.preventDefault();
			moveActive(event.key === 'ArrowDown' ? 1 : -1);
			return;
		}
		if (open && (event.key === 'Enter' || event.key === 'Tab')) {
			const option = filteredOptions[activeIndex];
			if (!option) return;
			event.preventDefault();
			void chooseOption(option);
			return;
		}
		if (
			!open &&
			event.key === 'Backspace' &&
			input.selectionStart === 0 &&
			input.selectionEnd === 0 &&
			chips.length > 0
		) {
			event.preventDefault();
			removeChip(chips.at(-1)!.type);
		}
	}

	function moveActive(step: 1 | -1) {
		if (filteredOptions.length === 0) return;
		activeIndex = (activeIndex + step + filteredOptions.length) % filteredOptions.length;
		announcement = `${filteredOptions[activeIndex].label}, ${activeIndex + 1} of ${filteredOptions.length}.`;
	}

	async function chooseOption(option: RichTitleOption) {
		if (option.custom) {
			const control = option.custom === 'date' ? dateInput : timeInput;
			control?.showPicker?.();
			control?.focus();
			return;
		}
		const kind = pickerKind;
		const caret = consumeToken();
		if (kind === 'project') {
			showLocationChip('project');
			const changed = option.id !== projectId;
			projectId = option.id;
			if (changed && sectionId !== null) {
				sectionId = null;
				explicitLocationTypes = explicitLocationTypes.filter((type) => type !== 'section');
				announcement = 'The previous section was removed because the project changed.';
			}
			void ensureSections(option.id);
			closePicker();
		} else if (kind === 'section') {
			if (!option.projectId || option.sectionId === undefined) return;
			showLocationChip('project');
			projectId = option.projectId;
			sectionId = option.sectionId;
			if (option.sectionId === null) {
				explicitLocationTypes = explicitLocationTypes.filter((type) => type !== 'section');
			} else {
				showLocationChip('section');
			}
			closePicker();
		} else if (kind === 'priority') {
			priority = Number(option.id);
			closePicker();
		} else if (kind === 'due') {
			if (option.id === '__no_date__') {
				dueDate = null;
				dueTime = null;
				dueTimezone = null;
				closePicker();
			} else {
				dueDate = option.id;
				manualPicker = 'due-time';
				activeIndex = 0;
				activeToken = null;
				await tick();
				positionPanel();
			}
		} else if (kind === 'due-time') {
			if (option.id === '__no_time__') {
				dueTime = null;
				dueTimezone = null;
			} else if (dueDate) {
				dueTime = option.id;
				dueTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
			}
			closePicker();
		}
		await tick();
		input?.focus();
		input?.setSelectionRange(caret, caret);
	}

	function showLocationChip(type: 'project' | 'section') {
		if (!explicitLocationTypes.includes(type)) {
			explicitLocationTypes = [...explicitLocationTypes, type];
		}
	}

	function chooseCustom(kind: 'date' | 'time', value: string) {
		if (!value) return;
		if (kind === 'date') {
			void chooseOption({ id: value, label: value });
		} else {
			void chooseOption({ id: value, label: value });
		}
	}

	function consumeToken(): number {
		if (!activeToken) return input?.selectionStart ?? title.length;
		const result = removeTitleToken(title, activeToken);
		title = result.value;
		activeToken = null;
		dismissedStart = null;
		return result.caret;
	}

	function closePicker() {
		activeToken = null;
		manualPicker = null;
		dismissedStart = null;
	}

	function openChip(type: TitleProperty) {
		manualPicker = type;
		activeToken = null;
		activeIndex = 0;
		if (type === 'section') loadLocationSections();
		void tick().then(positionPanel);
	}

	function removeChip(type: TitleProperty) {
		if (type === 'project') {
			projectId = '';
			sectionId = null;
		} else if (type === 'section') {
			sectionId = null;
		} else if (type === 'priority') {
			priority = 0;
		} else {
			dueDate = null;
			dueTime = null;
			dueTimezone = null;
		}
		announcement = `${type} property removed.`;
		void tick().then(() => input?.focus());
	}

	function positionPanel() {
		if (!open || !input) return;
		const rect = input.getBoundingClientRect();
		const computed = getComputedStyle(input);
		const canvas = document.createElement('canvas');
		const context = canvas.getContext('2d');
		if (context) context.font = computed.font;
		const caret = input.selectionStart ?? title.length;
		const textWidth = context?.measureText(title.slice(0, caret)).width ?? 0;
		const width = Math.min(320, window.innerWidth - 16);
		const desiredLeft =
			rect.left + Number.parseFloat(computed.paddingLeft || '0') + textWidth - input.scrollLeft;
		const left = Math.max(8, Math.min(desiredLeft, window.innerWidth - width - 8));
		const roomBelow = window.innerHeight - rect.bottom - 12;
		const above = roomBelow < 220 && rect.top > roomBelow;
		const maxHeight = Math.max(160, Math.min(320, above ? rect.top - 12 : roomBelow));
		const top = above ? Math.max(8, rect.top - Math.min(280, maxHeight) - 6) : rect.bottom + 6;
		panelStyle = `left:${left}px;top:${top}px;width:${width}px;max-height:${maxHeight}px`;
	}
</script>

<svelte:window onresize={positionPanel} />

<div class="rich-title">
	<label class="sr-only" for={inputId}>{label}</label>
	<div class="composer" class:disabled data-timezone={dueTimezone ?? undefined}>
		{#each chips as chip (chip.type)}
			<span class="chip">
				<button
					type="button"
					class="chip-value"
					aria-label={`${chip.type}: ${chip.label}. Open picker`}
					{disabled}
					onclick={() => openChip(chip.type)}
					onkeydown={(event) => {
						if (event.key === 'Backspace' || event.key === 'Delete') {
							event.preventDefault();
							removeChip(chip.type);
						}
					}}>{chip.label}</button
				>
				<button
					type="button"
					class="chip-remove"
					aria-label={`Remove ${chip.type}`}
					{disabled}
					onclick={() => removeChip(chip.type)}>×</button
				>
			</span>
		{/each}
		<input
			bind:this={input}
			id={inputId}
			name="title"
			class="title-input"
			bind:value={title}
			{placeholder}
			{disabled}
			maxlength="500"
			autocomplete="off"
			role="combobox"
			aria-autocomplete="list"
			aria-expanded={open}
			aria-controls={open ? listboxId : undefined}
			aria-activedescendant={open && filteredOptions[activeIndex]
				? `${listboxId}-${activeIndex}`
				: undefined}
			oninput={handleInput}
			onclick={syncToken}
			onkeyup={(event) => {
				if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) syncToken();
			}}
			onselect={syncToken}
			onkeydown={handleKeydown}
			oncompositionstart={() => (composing = true)}
			oncompositionend={() => {
				composing = false;
				syncToken();
			}}
		/>
	</div>

	{#if open}
		<div
			id={listboxId}
			class="options"
			role="listbox"
			aria-label={pickerKind === 'section' ? 'location options' : `${pickerKind} options`}
			style={panelStyle}
		>
			{#if filteredOptions.length === 0}
				<p class="empty">No matches</p>
			{:else}
				{#each filteredOptions as option, index (option.id)}
					{#if option.group && (index === 0 || filteredOptions[index - 1]?.group !== option.group)}
						<p class="option-group" role="presentation">{option.group}</p>
					{/if}
					<button
						id={`${listboxId}-${index}`}
						type="button"
						role="option"
						aria-label={option.group ? `${option.group}: ${option.label}` : undefined}
						aria-selected={index === activeIndex}
						class:active={index === activeIndex}
						onmouseenter={() => (activeIndex = index)}
						onmousedown={(event) => event.preventDefault()}
						onclick={() => void chooseOption(option)}
					>
						<span>{option.label}</span>
						{#if option.detail}<small>{option.detail}</small>{/if}
					</button>
				{/each}
			{/if}
		</div>
	{/if}

	<input
		bind:this={dateInput}
		class="native-picker"
		type="date"
		aria-label="Choose due date"
		value={dueDate ?? ''}
		onchange={(event) => chooseCustom('date', event.currentTarget.value)}
	/>
	<input
		bind:this={timeInput}
		class="native-picker"
		type="time"
		aria-label="Choose due time"
		value={dueTime ?? ''}
		onchange={(event) => chooseCustom('time', event.currentTarget.value)}
	/>
	<span class="sr-only" aria-live="polite">{announcement}</span>
</div>

<style>
	.rich-title {
		position: relative;
		min-width: 0;
	}
	.composer {
		display: flex;
		min-height: 2.75rem;
		align-items: center;
		gap: 0.35rem;
		box-sizing: border-box;
		width: 100%;
		padding: 0.32rem 0.45rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.65rem;
		background: var(--color-surface);
		overflow-x: auto;
	}
	.composer:focus-within {
		border-color: var(--theme-accent);
		box-shadow: 0 0 0 0.2rem var(--theme-focus);
	}
	.composer.disabled {
		opacity: 0.55;
	}
	.title-input {
		min-width: 8rem;
		flex: 1;
		padding: 0.35rem 0.25rem;
		border: 0;
		color: var(--color-text);
		background: transparent;
		font: inherit;
		outline: none;
	}
	.title-input::placeholder {
		color: var(--color-text-muted);
	}
	.chip {
		display: inline-flex;
		flex: none;
		align-items: stretch;
		border: 1px solid color-mix(in srgb, var(--theme-accent) 28%, var(--theme-border));
		border-radius: 999px;
		background: var(--theme-hover);
		overflow: hidden;
	}
	.chip button {
		border: 0;
		color: var(--theme-accent);
		background: transparent;
		font: inherit;
		font-size: 0.72rem;
		font-weight: 700;
		cursor: pointer;
	}
	.chip-value {
		max-width: 10rem;
		padding: 0.3rem 0.18rem 0.3rem 0.55rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.chip-remove {
		padding: 0.25rem 0.45rem 0.25rem 0.24rem;
	}
	.chip button:focus-visible {
		outline: 2px solid var(--theme-accent);
		outline-offset: -2px;
	}
	.options {
		position: fixed;
		z-index: 120;
		box-sizing: border-box;
		padding: 0.35rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.7rem;
		background: var(--color-surface-elevated);
		box-shadow: var(--shadow-elevated);
		overflow-y: auto;
	}
	.options button {
		display: flex;
		width: 100%;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.75rem;
		padding: 0.62rem 0.7rem;
		border: 0;
		border-radius: 0.45rem;
		color: var(--color-text);
		background: transparent;
		font: inherit;
		font-size: 0.82rem;
		text-align: left;
		cursor: pointer;
	}
	.options button.active,
	.options button:hover {
		background: var(--theme-hover);
	}
	.options small {
		color: var(--color-text-muted);
	}
	.option-group {
		margin: 0.45rem 0.65rem 0.2rem;
		color: var(--color-text-muted);
		font-size: 0.68rem;
		font-weight: 750;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}
	.empty {
		margin: 0;
		padding: 0.7rem;
		color: var(--color-text-muted);
		font-size: 0.8rem;
	}
	.native-picker {
		position: fixed;
		width: 1px;
		height: 1px;
		opacity: 0;
		pointer-events: none;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
</style>
