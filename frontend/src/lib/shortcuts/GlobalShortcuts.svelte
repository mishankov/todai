<script lang="ts">
	/* Shortcut destinations are assembled from the active project and registered commands. */
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { browser } from '$app/environment';
	import { goto, invalidateAll } from '$app/navigation';
	import TaskEditorModal from '$lib/components/TaskEditorModal.svelte';
	import CommandPalette from '$lib/palette/CommandPalette.svelte';
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { listProjectSections } from '$lib/projects/client';
	import { rememberedProjectPath } from '$lib/projects/navigation';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import {
		createTask as createTaskRequest,
		updateTask as updateTaskRequest
	} from '$lib/tasks/client';
	import { onMount, tick } from 'svelte';
	import { commandPaletteRequestEvent, quickAddRequestEvent, requestChatToggle } from './events';
	import QuickAddDialog from './QuickAddDialog.svelte';
	import {
		findShortcutCommand,
		formatShortcuts,
		isApplePlatform,
		shortcutCommand,
		type ProductCommand
	} from './registry';
	import ShortcutHelp from './ShortcutHelp.svelte';

	interface Props {
		activeProject?: Project;
		projects: Project[];
		currentPath: string;
		navigate?: (href: string) => void | Promise<void>;
		refresh?: () => void | Promise<void>;
		loadSections?: (projectId: string) => Promise<ProjectSection[]>;
		createTask?: (title: string, projectId: string, sectionId: string | null) => Promise<Task>;
		updateTask?: (taskId: string, changes: TaskUpdate) => Promise<Task>;
	}

	let {
		activeProject,
		projects,
		currentPath,
		navigate = (href) => goto(href),
		refresh = () => invalidateAll(),
		loadSections = (projectId) => listProjectSections(fetch, projectId),
		createTask = (title, projectId, sectionId) =>
			createTaskRequest(fetch, title, projectId, sectionId ?? undefined),
		updateTask = (taskId, changes) => updateTaskRequest(fetch, taskId, changes)
	}: Props = $props();
	let applePlatform = $state(browser && isApplePlatform(window.navigator.platform));
	let quickAddOpen = $state(false);
	let helpOpen = $state(false);
	let paletteOpen = $state(false);
	let editingTask = $state<Task | undefined>();
	let editingSections = $state<ProjectSection[] | undefined>();
	let focusRequest = $state(0);
	let previousFocus: HTMLElement | null = null;

	onMount(() => {
		applePlatform = isApplePlatform(window.navigator.platform);
		const openQuickAdd = () => requestQuickAdd();
		const openPalette = () => requestPalette();
		const handleVisibilityChange = () => {
			if (document.hidden) hideShortcutHints();
		};
		window.addEventListener(quickAddRequestEvent, openQuickAdd);
		window.addEventListener(commandPaletteRequestEvent, openPalette);
		document.addEventListener('visibilitychange', handleVisibilityChange);
		return () => {
			window.removeEventListener(quickAddRequestEvent, openQuickAdd);
			window.removeEventListener(commandPaletteRequestEvent, openPalette);
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			hideShortcutHints();
		};
	});

	function handleWindowKeydown(event: KeyboardEvent) {
		if (isPrimaryModifierKey(event)) showShortcutHints();

		if (event.key === 'Escape' && (helpOpen || quickAddOpen)) {
			event.preventDefault();
			event.stopImmediatePropagation();
			if (helpOpen) closeHelp();
			else closeQuickAdd();
			return;
		}

		const command = findShortcutCommand(event, applePlatform);
		if (!command || (command.scope === 'project' && !activeProject)) return;
		if (paletteOpen && command.id !== 'command-palette') return;
		event.preventDefault();
		event.stopImmediatePropagation();
		void runCommand(command);
	}

	function handleWindowKeyup(event: KeyboardEvent) {
		if (isPrimaryModifierKey(event)) hideShortcutHints();
	}

	function isPrimaryModifierKey(event: KeyboardEvent): boolean {
		return event.key === (applePlatform ? 'Meta' : 'Control');
	}

	function showShortcutHints() {
		document.documentElement.dataset.shortcutHints = 'visible';
	}

	function hideShortcutHints() {
		delete document.documentElement.dataset.shortcutHints;
	}

	async function runCommand(command: ProductCommand) {
		switch (command.id) {
			case 'command-palette':
				togglePalette();
				return;
			case 'quick-add':
				requestQuickAdd();
				return;
			case 'toggle-chat':
				previousFocus = null;
				requestChatToggle();
				return;
			case 'toggle-help':
				toggleHelp();
				return;
			case 'manage-projects':
				previousFocus = null;
				await navigate('/projects');
				return;
			case 'account-settings':
				previousFocus = null;
				await navigate('/settings');
				return;
		}

		if (!activeProject) return;
		const suffix: Partial<Record<ProductCommand['id'], string>> = {
			'project-overview': '/overview',
			'project-inbox': '',
			'project-today': '/today',
			'project-tasks': '/tasks',
			'project-activity': '/activity',
			'project-settings': '/settings'
		};
		previousFocus = null;
		await navigate(`/projects/${encodeURIComponent(activeProject.id)}${suffix[command.id] ?? ''}`);
	}

	function requestPalette() {
		if (
			paletteOpen ||
			document.querySelector('[role="dialog"][aria-modal="true"]') ||
			document.querySelector('[role="listbox"]')
		)
			return;
		captureFocus();
		paletteOpen = true;
	}

	function togglePalette() {
		if (paletteOpen) closePalette();
		else requestPalette();
	}

	function closePalette(restore = true) {
		paletteOpen = false;
		if (restore) void restoreFocus();
	}

	function requestQuickAdd() {
		if (!activeProject) return;
		if (quickAddOpen) {
			focusRequest += 1;
			return;
		}
		captureFocus();
		quickAddOpen = true;
	}

	function closeQuickAdd() {
		quickAddOpen = false;
		void restoreFocus();
	}

	function toggleHelp() {
		if (helpOpen) {
			closeHelp();
			return;
		}
		captureFocus();
		helpOpen = true;
	}

	function closeHelp() {
		helpOpen = false;
		void restoreFocus();
	}

	function captureFocus() {
		if (!previousFocus) {
			previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		}
	}

	async function restoreFocus() {
		const target = previousFocus;
		previousFocus = null;
		await tick();
		if (target?.isConnected) target.focus();
	}

	async function taskSaved() {
		quickAddOpen = false;
		await refresh();
		await restoreFocus();
	}

	async function switchFromPalette(project: Project) {
		previousFocus = null;
		await navigate(rememberedProjectPath(project.id));
	}

	function openTaskFromPalette(task: Task, sections?: ProjectSection[]) {
		editingSections = sections;
		editingTask = task;
	}

	async function savePaletteTask(changes: TaskUpdate) {
		if (!editingTask) return;
		await updateTask(editingTask.id, changes);
		editingTask = undefined;
		await refresh();
		await restoreFocus();
	}

	function closePaletteTask() {
		editingTask = undefined;
		void restoreFocus();
	}

	let quickAddLabel = $derived(
		formatShortcuts(shortcutCommand('quick-add'), applePlatform).join(' / ')
	);
	let activeSectionId = $derived(sectionFromPath(currentPath));

	function sectionFromPath(path: string): string | null {
		const match = path.match(/^\/projects\/[^/]+\/sections\/([^/]+)/);
		return match ? decodeURIComponent(match[1]) : null;
	}
</script>

<svelte:window
	onkeydown={handleWindowKeydown}
	onkeyup={handleWindowKeyup}
	onblur={hideShortcutHints}
/>

{#if paletteOpen}
	<CommandPalette
		{projects}
		{activeProject}
		{applePlatform}
		{loadSections}
		close={closePalette}
		executeCommand={runCommand}
		switchProject={switchFromPalette}
		selectTask={openTaskFromPalette}
	/>
{/if}

{#if quickAddOpen && activeProject}
	<QuickAddDialog
		{projects}
		initialProjectId={activeProject.id}
		initialSectionId={activeSectionId}
		shortcutLabel={quickAddLabel}
		{focusRequest}
		{loadSections}
		{createTask}
		{updateTask}
		close={closeQuickAdd}
		saved={taskSaved}
	/>
{/if}

{#if helpOpen}
	<ShortcutHelp {applePlatform} close={closeHelp} />
{/if}

{#if editingTask}
	<TaskEditorModal
		task={editingTask}
		{projects}
		sections={editingSections}
		{loadSections}
		save={savePaletteTask}
		close={closePaletteTask}
	/>
{/if}

<style>
	:global([data-shortcut-hint]::after) {
		position: absolute;
		z-index: 2;
		top: 50%;
		right: 0.55rem;
		padding: 0.24rem 0.42rem;
		border: 1px solid color-mix(in srgb, var(--theme-accent) 24%, var(--color-border));
		border-bottom-width: 2px;
		border-radius: 0.35rem;
		color: var(--theme-accent);
		background: var(--color-surface);
		box-shadow: var(--shadow-small);
		content: attr(data-shortcut-hint);
		font-family: inherit;
		font-size: 0.64rem;
		font-weight: 780;
		line-height: 1;
		white-space: nowrap;
		opacity: 0;
		visibility: hidden;
		transform: translateY(-45%) scale(0.96);
		pointer-events: none;
		transition:
			opacity 100ms ease,
			transform 100ms ease,
			visibility 100ms ease;
	}
	:global([data-shortcut-hint-position='left']::after) {
		right: calc(100% + 0.55rem);
	}
	:global([data-shortcut-hint-position='below']::after) {
		top: calc(100% + 0.4rem);
		right: 0;
		transform: translateY(-0.2rem) scale(0.96);
	}
	:global(html[data-shortcut-hints='visible'] [data-shortcut-hint]::after) {
		opacity: 1;
		visibility: visible;
		transform: translateY(-50%) scale(1);
	}
	:global(html[data-shortcut-hints='visible'] [data-shortcut-hint-position='below']::after) {
		transform: none;
	}
	@media (prefers-reduced-motion: reduce) {
		:global([data-shortcut-hint]::after) {
			transition: none;
		}
	}
</style>
