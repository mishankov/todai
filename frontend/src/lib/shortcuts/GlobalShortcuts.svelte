<script lang="ts">
	/* Shortcut destinations are assembled from the active project and fixed registered suffixes. */
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { goto, invalidateAll } from '$app/navigation';
	import { browser } from '$app/environment';
	import type { Project, ProjectSection } from '$lib/projects/client';
	import { listProjectSections } from '$lib/projects/client';
	import type { Task, TaskUpdate } from '$lib/tasks/client';
	import {
		createTask as createTaskRequest,
		updateTask as updateTaskRequest
	} from '$lib/tasks/client';
	import { onMount, tick } from 'svelte';
	import { quickAddRequestEvent, requestChatToggle } from './events';
	import QuickAddDialog from './QuickAddDialog.svelte';
	import {
		findShortcutCommand,
		formatShortcuts,
		isApplePlatform,
		shortcutCommand,
		type ShortcutCommand
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
	let focusRequest = $state(0);
	let previousFocus: HTMLElement | null = null;

	onMount(() => {
		applePlatform = isApplePlatform(window.navigator.platform);
		const openQuickAdd = () => requestQuickAdd();
		const handleVisibilityChange = () => {
			if (document.hidden) hideShortcutHints();
		};
		window.addEventListener(quickAddRequestEvent, openQuickAdd);
		document.addEventListener('visibilitychange', handleVisibilityChange);
		return () => {
			window.removeEventListener(quickAddRequestEvent, openQuickAdd);
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

	async function runCommand(command: ShortcutCommand) {
		switch (command.id) {
			case 'quick-add':
				requestQuickAdd();
				return;
			case 'toggle-chat':
				requestChatToggle();
				return;
			case 'toggle-help':
				toggleHelp();
				return;
		}

		if (!activeProject) return;
		const suffix: Record<ShortcutCommand['id'], string> = {
			'quick-add': '',
			'toggle-chat': '',
			'project-overview': '/overview',
			'project-inbox': '',
			'project-today': '/today',
			'project-tasks': '/tasks',
			'project-activity': '/activity',
			'project-settings': '/settings',
			'toggle-help': ''
		};
		await navigate(`/projects/${encodeURIComponent(activeProject.id)}${suffix[command.id]}`);
	}

	function requestQuickAdd() {
		if (!activeProject) return;
		if (quickAddOpen) {
			focusRequest += 1;
			return;
		}
		previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
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
		previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
		helpOpen = true;
	}

	function closeHelp() {
		helpOpen = false;
		void restoreFocus();
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

<style>
	:global([data-shortcut-hint]::after) {
		position: absolute;
		z-index: 2;
		top: 50%;
		right: 0.55rem;
		padding: 0.24rem 0.42rem;
		border: 1px solid color-mix(in srgb, var(--theme-accent, #2d6540) 24%, #d7d9d5);
		border-bottom-width: 2px;
		border-radius: 0.35rem;
		color: var(--theme-accent, #2d6540);
		background: #fff;
		box-shadow: 0 0.35rem 1rem rgb(20 28 21 / 14%);
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
